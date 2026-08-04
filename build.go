package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moby/buildkit/client/llb"
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
)

const (
	DefaultNixImage = "docker.io/nixos/nix:latest"
	keyNixImage     = "BUILDKIT_NIX_IMAGE"
	keyNixSessionID = "BUILDKIT_NIX_SESSIONID"
)

const (
	mountDockerfileDir    = "/mnt/dockerfile"
	mountNixStoreCacheDir = "/mnt/nix"
	mountSourceDir        = "/mnt/source"
	mountWorkspaceDir     = "/mnt/workspace"
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

	// TODO: Implement support for build-args:
	//       - substituters
	//       - trusted-substituters

	// Default secrets for nix build, which are optional and can be provided by the user
	nixBuildSecrets := []llb.RunOption{
		llb.AddSecret("/run/secrets/access-tokens", llb.SecretID("access-tokens"), llb.SecretOptional),
		llb.AddSecret("/run/secrets/impure-env", llb.SecretID("impure-env"), llb.SecretOptional),
		llb.AddSecret("/run/secrets/netrc-file", llb.SecretID("netrc-file"), llb.SecretOptional),

		// Special secret for GitHub token, which is used to access private repositories
		llb.AddSecret("GITHUB_TOKEN", llb.SecretID("GITHUB_TOKEN"), llb.SecretAsEnv(true), llb.SecretOptional),
	}

	// Nix Store cache key
	nixStoreCacheKey := "nix-store-cache"

	// Using the dockerui.Client to enable multi-platform builds
	// The dockerui.Client will handle the platform selection and build execution for us
	rb, err := bc.Build(ctx, func(ctx context.Context, platform *ocispecs.Platform, idx int) (*dockerui.BuildResult, error) {
		withInternalName := nixui.WithInternalName
		if bc.MultiPlatformRequested {
			nixStoreCacheKey = fmt.Sprintf("nix-store-cache-%s-%s", platform.OS, platform.Architecture)
			withInternalName = nixui.WithInternalNameTag(fmt.Sprintf("%s/%s", platform.OS, platform.Architecture))
		}

		// Nix store is used to persist the nix store between builds, so that we don't have to rebuild everything from scratch every time
		nixStore := llb.Scratch()

		// Load the nix image and set it as the base image for the build
		builderImageOpts := []llb.ImageOption{
			llb.WithMetaResolver(c),
			withInternalName(fmt.Sprintf("load builder image from %s", NixImage)),
		}
		if bc.MultiPlatformRequested {
			builderImageOpts = append(builderImageOpts, llb.Platform(*platform))
		}
		if bc.IsNoCache("builder") {
			builderImageOpts = append(builderImageOpts, llb.IgnoreCache)
		}
		builder := llb.Image(NixImage, builderImageOpts...)

		// Builder working directory is set to /mnt/workspace,
		// so that the nix build result can be copied to /mnt/workspace/result and extracted later
		builder = builder.With(llb.Dir(mountWorkspaceDir))

		// restore nix store cache
		nixStoreRestoreOpts := []llb.RunOption{
			llb.AddEnv(keyNixSessionID, c.BuildOpts().SessionID),
			llb.AddMount(mountNixStoreCacheDir, nixStore, llb.AsPersistentCacheDir(nixStoreCacheKey, llb.CacheMountLocked)),
			llb.Shlexf("cp -anT %s /nix", mountNixStoreCacheDir),
			withInternalName("configure nix store"),
		}
		if bc.IsNoCache("nix-store") {
			nixStoreRestoreOpts = append(nixStoreRestoreOpts, llb.IgnoreCache)
		}
		builderSt := builder.Run(nixStoreRestoreOpts...)

		// Nix build
		nixBuildOpts := []llb.RunOption{
			llb.AddMount(mountSourceDir, *mainContext, llb.Readonly),
			llb.AddMount("/build", llb.Scratch()),
			nixllb.Shlexf(`
				set -euo pipefail
				nixopts=()

				echo "Prepare build environment..."
				# If GITHUB_TOKEN is not empty, then add it to the nix options as a secret
				if [ -n "${GITHUB_TOKEN:-}" ]; then
					echo "Detected GITHUB_TOKEN secret, adding to nix options"
					nixopts+=("--option" "access-tokens" "github.com=${GITHUB_TOKEN}")
				fi

				# If there are any secrets in /run/secrets, then add them to nix options,
				# the secret name is the nix option name and the secret value is the nix option value
				for f in /run/secrets/*; do
					if [ -f "$f" ]; then
						echo "Detected secret for nix option: $(basename "$f")"
						nixopts+=("--option" "$(basename "$f")" "$(cat "$f")")
					fi
				done

				echo -e "\nBuild log data will stream in below:"
				nix "${nixopts[@]}" \
					--option sandbox false \
					--option filter-syscalls false \
					--option auto-optimise-store true \
					--option binary-caches-parallel-connections 15 \
					--extra-experimental-features configurable-impure-env \
					--extra-experimental-features nix-command \
					--extra-experimental-features flakes \
					--show-trace \
					--log-format raw \
				build %s#%s
				echo -e "\nBuild finished!"

				echo -e "\nPerforming post-build checks:\n"
				if [ -d "$(readlink -f result/)" ]; then
					echo "- It appears that the build produced a directory instead of a Docker-compatible repository tarball."
					echo "  This is not supported by buildkit-nix, please check the build logs for errors."
					echo ""
					echo "  See https://nix.dev/tutorials/nixos/building-and-running-docker-images.html for more information on how to build Docker images with Nix."
					exit 1
				elif ! tar -tf result | grep -q "manifest.json"; then
					echo "- It appears that the build did not produce a Docker-compatible repository tarball."
					echo "  This is not supported by buildkit-nix, please check the build logs for errors."
					echo ""
					echo "  See https://nix.dev/tutorials/nixos/building-and-running-docker-images.html for more information on how to build Docker images with Nix."
					exit 1
				fi
				echo "-  Found Docker-compatible repository tarball, proceeding with extraction of layers and config file."
				echo ""
				exit 0
			`, mountSourceDir, bc.Target),
			withInternalName(fmt.Sprintf("nix build .#%s", bc.Target)),
		}
		for _, secret := range nixBuildSecrets {
			nixBuildOpts = append(nixBuildOpts, secret)
		}
		if bc.IsNoCache("nix-build") {
			nixBuildOpts = append(nixBuildOpts, llb.IgnoreCache)
		}
		builderSt = builderSt.Run(nixBuildOpts...)

		// save nix store cache
		nixStoreSaveOpts := []llb.RunOption{
			llb.AddEnv(keyNixSessionID, c.BuildOpts().SessionID),
			llb.AddMount(mountNixStoreCacheDir, nixStore, llb.AsPersistentCacheDir(nixStoreCacheKey, llb.CacheMountLocked)),
			llb.Shlexf("cp -afT /nix %s", mountNixStoreCacheDir),
			withInternalName("create nix store snapshot"),
		}
		if bc.IsNoCache("nix-store") {
			nixStoreSaveOpts = append(nixStoreSaveOpts, llb.IgnoreCache)
		}
		builderSt = builderSt.Run(nixStoreSaveOpts...)

		// Extract the result of the nix build to a new scratch state
		extract := llb.Scratch()
		extractFileOpts := []llb.ConstraintsOpt{
			withInternalName("extracting result layers"),
		}
		if bc.IsNoCache("extract") {
			extractFileOpts = append(extractFileOpts, llb.IgnoreCache)
		}
		extractSt := extract.File(
			llb.Copy(
				builderSt.GetMount("/"),
				fmt.Sprintf("%s/result", mountWorkspaceDir), "/", &llb.CopyInfo{
					AttemptUnpack: true,
				},
			),
			extractFileOpts...,
		)

		// Prepare the builder state to extract the manifest.json file
		extractDef, err := extractSt.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
		if err != nil {
			return nil, err
		}
		extractRes, err := c.Solve(ctx, client.SolveRequest{
			Definition: extractDef.ToPB(),
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
				llb.Copy(extractSt, layer, "/", &llb.CopyInfo{
					AttemptUnpack: true,
				}),
				layeredFileOpts...,
			)
		}

		// Prepare the final definition from final state
		layeredDef, err := layered.Marshal(ctx)
		if err != nil {
			return nil, err
		}
		layeredRes, err := c.Solve(ctx, client.SolveRequest{
			Definition: layeredDef.ToPB(),
		})
		if err != nil {
			return nil, err
		}
		layeredRef, err := layeredRes.SingleRef()
		if err != nil {
			return nil, err
		}

		// Read the config file from the result of the nix build and add it to the result metadata
		configByte, err := extractRef.ReadFile(ctx, client.ReadRequest{Filename: "/" + manifest.Config})
		if err != nil {
			return nil, err
		}
		// Parse the oci config file to get the image configuration
		var config dockerocispecs.DockerOCIImage
		if err := json.Unmarshal(configByte, &config); err != nil {
			return nil, err
		}

		// Return the final result with the layered reference and the image configuration
		return &dockerui.BuildResult{
			Reference: layeredRef,
			Image:     &config,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	return rb.Finalize()
}
