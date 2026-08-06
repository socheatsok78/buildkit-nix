package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/moby/buildkit/frontend/gateway/client"
	dockerocispecs "github.com/moby/docker-image-spec/specs-go/v1"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
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
	keyNixSessionID = "BUILDKIT_NIX_SESSIONID"

	DefaultNixImage                 = "docker.io/nixos/nix:latest"
	keyNixImage                     = "BUILDKIT_NIX_IMAGE"
	keyNixOptionSubstituters        = "BUILDKIT_NIX_OPTION_SUBSTITUTERS"
	keyNixOptionTrustedPublicKeys   = "BUILDKIT_NIX_OPTION_TRUSTED_PUBLIC_KEYS"
	keyNixOptionTrustedSubstituters = "BUILDKIT_NIX_OPTION_TRUSTED_SUBSTITUTERS"
)

const (
	mountDockerfileDir    = "/mnt/dockerfile"
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

	// Load the source code from the build context
	mainContext, err := bc.MainContext(ctx, llb.SessionID(c.BuildOpts().SessionID), llb.SharedKeyHint("nix-src"))
	if err != nil {
		return nil, err
	}

	// Load the nix option substituters and trusted substituters from the build options, if provided
	NixOptionSubstituters := ""
	if v, ok := opts[keyNixOptionSubstituters]; ok {
		NixOptionSubstituters = v
	}
	NixOptionTrustedSubstituters := ""
	if v, ok := opts[keyNixOptionTrustedSubstituters]; ok {
		NixOptionTrustedSubstituters = v
	}
	NixOptionTrustedPublicKeys := ""
	if v, ok := opts[keyNixOptionTrustedPublicKeys]; ok {
		NixOptionTrustedPublicKeys = v
	}

	// Load secrets for the nix build, including access tokens, impure environment variables, and netrc files
	// The secrets are on-demand and will be provided by the buildkit session, if available
	nixBuildSecrets := []llb.RunOption{
		llb.AddSecret("/run/secrets/access-tokens", llb.SecretID("access-tokens"), llb.SecretOptional),
		llb.AddSecret("/run/secrets/impure-env", llb.SecretID("impure-env"), llb.SecretOptional),
		llb.AddSecret("/run/secrets/netrc-file", llb.SecretID("netrc-file"), llb.SecretOptional),

		// Special secret for GitHub token, which is used to access private repositories
		llb.AddSecret("GITHUB_TOKEN", llb.SecretID("GITHUB_TOKEN"), llb.SecretAsEnv(true), llb.SecretOptional),
	}

	// Using the dockerui.Client to enable multi-platform builds
	// The dockerui.Client will handle the platform selection and build execution for us
	rb, err := bc.Build(ctx, func(ctx context.Context, platform *ocispecs.Platform, idx int) (*dockerui.BuildResult, error) {
		var p ocispecs.Platform
		if platform != nil {
			p = *platform
		} else {
			p = platforms.DefaultSpec()
		}

		nixImageDigest, err := resolveNixImageDigest(ctx, c, NixImage, &p)
		if err != nil {
			return nil, err
		}
		nixStoreCacheKey := path.Clean(fmt.Sprintf("%s/%s/%s/%s/nix", bc.CacheIDNamespace, nixImageDigest, p.OS, p.Architecture))

		withInternalName := nixui.WithInternalName
		if bc.MultiPlatformRequested {
			withInternalName = nixui.WithInternalNameTag(fmt.Sprintf("%s/%s", p.OS, p.Architecture))
		}

		// Shelter
		shelter := llb.Scratch()

		// Load the nix image and set it as the base image for the build
		builderImageOpts := []llb.ImageOption{
			llb.Platform(p),
			llb.ResolveDigest(true),
			llb.ResolveModePreferLocal, // Prefer local cache for the nix image, to avoid pulling it from the registry if it's already available
			llb.WithExportCache(),
			llb.WithMetaResolver(c),
			withInternalName(fmt.Sprintf("load builder image from %s", NixImage)),
		}
		if bc.IsNoCache("builder") {
			builderImageOpts = append(builderImageOpts, llb.IgnoreCache)
		}
		builder := llb.Image(NixImage, builderImageOpts...)

		// Nix build
		nixBuildOpts := []llb.RunOption{
			llb.AddEnv("BUILDKIT_NIX_BUILD_SHELTER", mountShelterDir),
			llb.AddEnv("BUILDKIT_NIX_BUILD_TARGET", bc.Target),
			llb.AddEnv("BUILDKIT_NIX_OPTION_SUBSTITUTERS", NixOptionSubstituters),
			llb.AddEnv("BUILDKIT_NIX_OPTION_TRUSTED_PUBLIC_KEYS", NixOptionTrustedPublicKeys),
			llb.AddEnv("BUILDKIT_NIX_OPTION_TRUSTED_SUBSTITUTERS", NixOptionTrustedSubstituters),
			llb.AddEnv("BUILDKIT_NIX_STORE_CACHE_KEY", nixStoreCacheKey),
			llb.AddMount("/nix", builder, llb.SourcePath("/nix"), llb.AsPersistentCacheDir(nixStoreCacheKey, llb.CacheMountLocked)),
			llb.AddMount("/build", llb.Scratch()),
			llb.AddMount(mountShelterDir, shelter),
			llb.AddMount(mountSourceDir, *mainContext),
			llb.Dir(mountSourceDir),
			nixllb.Shlexf(`
				set -euo pipefail

				nix-config-get() {
					nix --extra-experimental-features nix-command config show | grep "$1" | cut -d"=" -f2 | xargs
				}

				BUILDKIT_NIX_BUILD_SHELTER=${BUILDKIT_NIX_BUILD_SHELTER:-/shelter}
				BUILDKIT_NIX_BUILD_TARGET=${BUILDKIT_NIX_BUILD_TARGET:-default}
				BUILDKIT_NIX_OPTION_SUBSTITUTERS=${BUILDKIT_NIX_OPTION_SUBSTITUTERS:-}
				BUILDKIT_NIX_OPTION_TRUSTED_PUBLIC_KEYS=${BUILDKIT_NIX_OPTION_TRUSTED_PUBLIC_KEYS:-}
				BUILDKIT_NIX_OPTION_TRUSTED_SUBSTITUTERS=${BUILDKIT_NIX_OPTION_TRUSTED_SUBSTITUTERS:-}
				BUILDKIT_NIX_STORE_CACHE_KEY=${BUILDKIT_NIX_STORE_CACHE_KEY:-}

				echo "Setup build environment"
				nix --version
				{
					echo "binary-caches-parallel-connections = 15"
					echo "extra-experimental-features = configurable-impure-env"
					echo "extra-experimental-features = flakes"
					echo "extra-experimental-features = nix-command"
					echo "filter-syscalls = false"
					echo -ne "${BUILDKIT_NIX_OPTION_SUBSTITUTERS:+substituters = ${BUILDKIT_NIX_OPTION_SUBSTITUTERS} $(nix-config-get substituters)\n}"
					echo -ne "${BUILDKIT_NIX_OPTION_TRUSTED_PUBLIC_KEYS:+trusted-public-keys = ${BUILDKIT_NIX_OPTION_TRUSTED_PUBLIC_KEYS} $(nix-config-get trusted-public-keys)\n}"
					echo -ne "${BUILDKIT_NIX_OPTION_TRUSTED_SUBSTITUTERS:+trusted-substituters = ${BUILDKIT_NIX_OPTION_TRUSTED_SUBSTITUTERS}\n}"
					# echo "sandbox = false" -- already set by default from nixos/nix image
				} | tee -a /etc/nix/nix.conf

				# This is a fake config for debugging purposes,
				# it will be printed in the build logs, but it will not be used by nix
				echo "nix-store-cache-key = ${BUILDKIT_NIX_STORE_CACHE_KEY}"
				echo ""

				nixopts=()

				# If GITHUB_TOKEN is not empty, then add it to the nix options as a secret
				if [ -n "${GITHUB_TOKEN:-}" ]; then
					echo "- Detected GITHUB_TOKEN secret, adding to nix options"
					nixopts+=("--option" "access-tokens" "github.com=${GITHUB_TOKEN}")
				fi

				# If there are any secrets in /run/secrets, then add them to nix options,
				# the secret name is the nix option name and the secret value is the nix option value
				for f in /run/secrets/*; do
					if [ -f "$f" ]; then
						echo "- Detected secret for nix option: $(basename "$f")"
						nixopts+=("--option" "$(basename "$f")" "$(cat "$f")")
					fi
				done

				echo -e "\nBuild log data will stream in below:"
				nix "${nixopts[@]}" --show-trace --log-format raw build ".#${BUILDKIT_NIX_BUILD_TARGET}"
				echo -e "\nBuild finished!"

				if [ -d "$(readlink -f result)" ]; then
					echo -n "derivation" > "${BUILDKIT_NIX_BUILD_SHELTER}/type"
					mkdir -p "${BUILDKIT_NIX_BUILD_SHELTER}/result/nix/store"
					cp -af result/* "${BUILDKIT_NIX_BUILD_SHELTER}/result"
					cp -af $(nix-store -qR result/) "${BUILDKIT_NIX_BUILD_SHELTER}/result/nix/store"
				else
					if tar -tf result | grep -q manifest.json; then
						echo -n "ocispec" > "${BUILDKIT_NIX_BUILD_SHELTER}/type"
						cp $(nix-store -qR result/) "${BUILDKIT_NIX_BUILD_SHELTER}/result"
					else
						echo "ERROR: nix build did not produce a valid result"
						exit 1
					fi
				fi

				echo ""
				exit 0
			`),
			withInternalName(fmt.Sprintf("nix build .#%s", bc.Target)),
		}
		for _, secret := range nixBuildSecrets {
			nixBuildOpts = append(nixBuildOpts, secret)
		}
		if bc.IsNoCache("nix-build") {
			nixBuildOpts = append(nixBuildOpts, llb.IgnoreCache)
		}
		builderSt := builder.Run(nixBuildOpts...)

		// Re-assign the shelter state to the result of the nix build, so that we can read the result type and extract the result from it
		shelter = builderSt.GetMount(mountShelterDir)

		// Marshal the shelter state to a definition and solve it to get a reference to the result of the nix build
		shelterDef, err := shelter.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
		if err != nil {
			return nil, err
		}
		shelterRes, err := c.Solve(ctx, client.SolveRequest{
			Definition:   shelterDef.ToPB(),
			CacheImports: bc.CacheImports,
		})
		if err != nil {
			return nil, err
		}
		shelterRef, err := shelterRes.SingleRef()
		if err != nil {
			return nil, err
		}

		// Read the result type from the shelter state to determine how to handle the result of the nix build
		resultTypeByte, err := shelterRef.ReadFile(ctx, client.ReadRequest{Filename: "/type"})
		if err != nil {
			return nil, err
		}
		resultType := strings.TrimSpace(string(resultTypeByte))

		// Extract the result of the nix build to a new scratch state
		extract := llb.Scratch()
		extractFileOpts := []llb.ConstraintsOpt{
			withInternalName("extracting result layers"),
		}
		if bc.IsNoCache("extract") {
			extractFileOpts = append(extractFileOpts, llb.IgnoreCache)
		}
		extract = extract.File(
			llb.Copy(
				shelter, "/result", "/", &llb.CopyInfo{
					AttemptUnpack:       true,
					CopyDirContentsOnly: true,
				},
			),
			extractFileOpts...,
		)

		var st llb.State
		var ref client.Reference
		var config dockerocispecs.DockerOCIImage

		if resultType == "derivation" {
			config = dockerocispecs.DockerOCIImage{
				Image: ocispecs.Image{Platform: p},
			}
			st = extract
		} else if resultType == "ocispec" {
			// Prepare the builder state to extract the manifest.json file
			extractDef, err := extract.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
			if err != nil {
				return nil, err
			}
			extractRes, err := c.Solve(ctx, client.SolveRequest{
				Definition:   extractDef.ToPB(),
				CacheImports: bc.CacheImports,
			})
			if err != nil {
				return nil, err
			}
			extractRef, err := extractRes.SingleRef()
			if err != nil {
				return nil, err
			}

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
				layeredFileOpts := []llb.ConstraintsOpt{
					withInternalName(fmt.Sprintf("importing layer: %s", layer)),
				}
				if bc.IsNoCache("layered") {
					layeredFileOpts = append(layeredFileOpts, llb.IgnoreCache)
				}
				layered = layered.File(
					llb.Copy(extract, layer, "/", &llb.CopyInfo{
						AttemptUnpack: true,
					}),
					layeredFileOpts...,
				)
			}

			// Assin the layered state to the final state to be returned
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
		res, err := c.Solve(ctx, client.SolveRequest{
			Definition:   def.ToPB(),
			CacheImports: bc.CacheImports,
		})
		if err != nil {
			return nil, err
		}
		ref, err = res.SingleRef()
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

func resolveNixImageDigest(ctx context.Context, c client.Client, nixImage string, platform *ocispecs.Platform) (string, error) {
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
