package nixfile

import (
	"context"
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
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/socheatsok78/buildkit-nix/nixfile/builder"
	"github.com/socheatsok78/buildkit-nix/nixfile/exporter"
	"github.com/socheatsok78/buildkit-nix/nixfile/exporter/types"
	"github.com/socheatsok78/buildkit-nix/pkg/nixllb"
)

const (
	DefaultNixImage        = "docker.io/nixos/nix:latest"
	keyNixImage            = "image"
	keyNixSecurityInsecure = "security.insecure"
	buildArgPrefix         = "build-arg:"
)

const (
	keyNixConfArgPrefix = buildArgPrefix + "nix.conf."
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

		// Create a new nix builder with the specified nix image, security mode, and build options
		nixbld := builder.NewBuilder(
			NixImage,
			builder.ShouldIgnoreCache(bc.IsNoCache("builder")),
			builder.ImageOptions(
				llb.ResolveModePreferLocal,
				llb.Platform(p),
				llb.ResolveDigest(true),
				llb.WithMetaResolver(c),
			),
			builder.NixBuildSecrets(nixBuildSecretOpts...),
			builder.NixStoreCacheKey(nixStoreCacheKey),
			builder.NixUserConfigs(nixUserConfigsStr),
			builder.SecurityMode(security),
		)

		// Build the nix derivation for the specified target using the source code from the build context
		result := nixbld.Build(bc.Target, *mainContext)

		// Export the result of the nix build
		export, err := exporter.Export(ctx, c, types.ExportConfig{
			State:       result,
			Platform:    p,
			IgnoreCache: bc.IsNoCache("exporter"),
		})
		if err != nil {
			return nil, err
		}

		def, err := export.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
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
		ref, err := res.SingleRef()
		if err != nil {
			return nil, err
		}

		// Return the final result with the layered reference and the image configuration
		return &dockerui.BuildResult{
			Reference: ref,
			Image:     &export.DockerOCIImage,
			BaseImage: &export.DockerOCIImage,
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
