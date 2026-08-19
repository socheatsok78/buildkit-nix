package nixfile

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/moby/buildkit/frontend/gateway/client"
	dockerocispec "github.com/moby/docker-image-spec/specs-go/v1"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/socheatsok78/buildkit-nix/nixfile/builder"
	"github.com/socheatsok78/buildkit-nix/pkg/dockershim"
	"github.com/socheatsok78/buildkit-nix/pkg/nixllb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

const (
	buildArgPrefix      = "build-arg:"
	keyLocalNameContext = "context"
	keyTarget           = "target"
	keySource           = "source"
)

const (
	DefaultNixImage = "docker.io/nixos/nix:latest"

	// build-args keys
	keyNixImage            = "image"
	keyNixSecurityInsecure = "security.insecure"

	// build-args prefixes
	keyNixConfArgPrefix = buildArgPrefix + "nix.conf."
)

const (
	mountNixStoreCacheDir = "/mnt/nix"
	mountSourceDir        = "/mnt/source"
	mountShelterDir       = "/shelter"
)

func Build(ctx context.Context, c client.Client) (*client.Result, error) {
	c = &withResolveCache{Client: c}
	bc, err := dockerui.NewClient(c)
	if err != nil {
		return nil, err
	}
	opts := bc.BuildOpts().Opts

	// also accept build args from Moby
	for k, v := range opts {
		if strings.HasPrefix(k, buildArgPrefix) {
			opts[strings.TrimPrefix(k, buildArgPrefix)] = v
		}
	}

	// Accept a custom nix image from the build options, otherwise use the default one
	NixImage := DefaultNixImage
	if v, ok := opts[keyNixImage]; ok {
		NixImage = v
	}

	// If no target is specified, set to "default"
	if bc.Target == "" {
		bc.Target = "default"
	}

	// Load the security mode from the build options, if provided. default (sandbox)
	security := llb.SecurityModeSandbox
	if v, ok := opts[keyNixSecurityInsecure]; ok {
		if enable, _ := strconv.ParseBool(v); enable {
			security = llb.SecurityModeInsecure
		} else {
			security = llb.SecurityModeSandbox
		}
	}

	// Load the nix option substituters and trusted substituters from the build options, if provided
	nixExtraConfigOpt := map[string]string{}
	for k, v := range opts {
		if strings.HasPrefix(k, keyNixConfArgPrefix) {
			nixExtraConfigOpt[strings.TrimPrefix(k, keyNixConfArgPrefix)] = v
		}
	}
	nixUserConfigsStr := ""
	for k, v := range nixExtraConfigOpt {
		nixUserConfigsStr += fmt.Sprintf("%s = %s\n", k, v)
	}

	// Load the nix option secrets from the build options, if provided
	nixBuildSecretOpts, err := NixSecretRunOptions(opts)
	if err != nil {
		return nil, err
	}

	// Load the source code from the build context
	mainContext, err := bc.MainContext(ctx, llb.SessionID(c.BuildOpts().SessionID), llb.SharedKeyHint("nix-src"), nixllb.ResetExcludePatterns())
	if err != nil {
		return nil, err
	}

	// Using the dockerui.Client to enable multi-platform builds
	// The dockerui.Client will handle the platform selection and build execution for us
	rb, err := bc.Build(ctx, func(ctx context.Context, platform *ocispec.Platform, idx int) (*dockerui.BuildResult, error) {
		var p ocispec.Platform
		if platform != nil {
			p = *platform
		} else {
			p = platforms.DefaultSpec()
		}

		nixImageDigest, err := resolveNixImageDigest(ctx, c, NixImage, &p)
		if err != nil {
			return nil, err
		}
		nixStoreCacheKey := path.Clean(fmt.Sprintf("%s/%s/%s/%s", bc.CacheIDNamespace, nixImageDigest, p.OS, p.Architecture))

		withInternalNameW := nixui.WithInternalName
		if bc.MultiPlatformRequested {
			withInternalNameW = nixui.WithInternalNameTag(fmt.Sprintf("%s/%s", p.OS, p.Architecture))
		}

		nixbld := builder.NewBuilder(
			NixImage,
			builder.ShouldIgnoreCache(bc.IsNoCache("builder")),
			builder.ImageOptions(
				llb.ResolveModePreferLocal,
				llb.Platform(p),
				llb.ResolveDigest(true),
				llb.WithMetaResolver(c),
				llb.WithExportCache(),
			),
			builder.NixBuildSecrets(nixBuildSecretOpts...),
			builder.NixStoreCacheKey(nixStoreCacheKey),
			builder.NixUserConfigs(nixUserConfigsStr),
			builder.SecurityMode(security),
		)

		result := nixbld.Build(bc.Target, *mainContext)

		// Marshal the shelter state to a definition and solve it to get a reference to the result of the nix build
		buildResultDef, err := result.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
		if err != nil {
			return nil, err
		}
		_, buildResultRef, err := solveResultReference(ctx, c, client.SolveRequest{
			Definition:   buildResultDef.ToPB(),
			CacheImports: bc.CacheImports,
		})

		// Read the result type from the shelter state to determine how to handle the result of the nix build
		resultTypeByte, err := buildResultRef.ReadFile(ctx, client.ReadRequest{Filename: "/type"})
		if err != nil {
			return nil, err
		}
		resultType := strings.TrimSpace(string(resultTypeByte))

		// Create a new scratch state and copy the result of the nix build from the shelter state to it, depending on the result type
		//
		// - If the result type is "derivation", we will copy the result of the nix build to the scratch state and return it as a reference
		// - If the result type is "ocispec", we will copy the result of the nix build to the scratch state, parse the manifest.json file,
		//   and return the layers and config file as a reference
		var st llb.State
		var config dockerocispec.DockerOCIImage

		if resultType == "derivation" {
			config = dockerocispec.DockerOCIImage{
				Image: ocispec.Image{Platform: p},
			}

			nixStoreClosure := llb.Scratch().File(
				llb.Copy(result, "/result", "/", &llb.CopyInfo{CopyDirContentsOnly: true}),
				nixllb.ShouldIgnoreCache(bc.IsNoCache("extract")),
				withInternalNameW("copying nix store closure..."),
			)

			st = nixStoreClosure
		} else if resultType == "ocispec" {
			nixStoreClosure := llb.Scratch().File(
				llb.Copy(result, "/result", "/", &llb.CopyInfo{AttemptUnpack: true}),
				nixllb.ShouldIgnoreCache(bc.IsNoCache("extract")),
				withInternalNameW("evaluating nix store closure"),
			)

			// Prepare the builder state to extract the manifest.json file
			extractDef, err := nixStoreClosure.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
			if err != nil {
				return nil, err
			}
			_, extractRef, err := solveResultReference(ctx, c, client.SolveRequest{
				Definition:   extractDef.ToPB(),
				CacheImports: bc.CacheImports,
			})

			// Parse the manifest.json file to get the list of layers and the config file
			manifestByte, err := extractRef.ReadFile(ctx, client.ReadRequest{Filename: "/manifest.json"})
			if err != nil {
				return nil, fmt.Errorf("nix build did not produce a manifest.json file, please check the build logs for errors: %w", err)
			}
			manifest, err := dockershim.UnmarshalManifest(manifestByte)
			if err != nil {
				return nil, err
			}

			// Create a new scratch state and overlay the layers from the manifest.json file to it
			layered := llb.Scratch()
			for _, layer := range manifest.Layers {
				layered = layered.File(
					llb.Copy(nixStoreClosure, layer, "/", &llb.CopyInfo{AttemptUnpack: true}),
					nixllb.ShouldIgnoreCache(bc.IsNoCache("layered")),
					withInternalNameW(fmt.Sprintf("copying layer '%s'...", layer)),
				)
			}

			// Set the final state to the layered state, which contains the layers from the manifest.json file
			st = layered

			// Read the config file from the result of the nix build and add it to the result metadata
			configByte, err := extractRef.ReadFile(ctx, client.ReadRequest{Filename: "/" + manifest.Config})
			if err != nil {
				return nil, err
			}

			// Parse the oci config file to get the image configuration
			if err := json.Unmarshal(configByte, &config); err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("unsupported result type: %s", resultType)
		}

		def, err := st.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
		if err != nil {
			return nil, err
		}
		_, ref, err := solveResultReference(ctx, c, client.SolveRequest{
			Definition:   def.ToPB(),
			CacheImports: bc.CacheImports,
		})
		if err != nil {
			return nil, err
		}

		// Return the final result with the layered reference and the image configuration
		return &dockerui.BuildResult{
			Reference: ref,
			Image:     &config,
			BaseImage: &config,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	return rb.Finalize()
}

func resolveNixImageDigest(ctx context.Context, c client.Client, nixImage string, platform *ocispec.Platform) (string, error) {
	opt := &sourceresolver.ResolveImageOpt{
		Platform: platform,
	}
	if _, nixImageDigest, _, err := c.ResolveImageConfig(ctx, nixImage, sourceresolver.Opt{ImageOpt: opt}); err != nil {
		return "", err
	} else if nixImageDigest != "" {
		nixImageRef, err := reference.ParseNormalizedNamed(nixImage)
		if err != nil {
			return "", err
		}
		nixImageRefWithDigest, err := reference.WithDigest(nixImageRef, nixImageDigest)
		if err != nil {
			return "", err
		}
		return nixImageRefWithDigest.String(), nil
	}
	return nixImage, nil
}
