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

	source := ""
	if v, ok := opts[keySource]; ok {
		source = v
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
		nixStoreCacheKey := path.Clean(fmt.Sprintf("%s/%s/%s/%s/nix/store", bc.CacheIDNamespace, nixImageDigest, source, bc.Target))

		withInternalName := nixui.WithInternalName
		if bc.MultiPlatformRequested {
			withInternalName = nixui.WithInternalNameTag(fmt.Sprintf("%s/%s", p.OS, p.Architecture))
		}

		// Nix store is used to persist the nix store between builds, so that we don't have to rebuild everything from scratch every time
		nixStore := llb.Scratch()

		// Load the nix image and set it as the base image for the build
		builderImageOpts := []llb.ImageOption{
			llb.Platform(p),
			llb.WithExportCache(),
			llb.WithMetaResolver(c),
			withInternalName(fmt.Sprintf("load builder image from %s", NixImage)),
		}
		if bc.IsNoCache("builder") {
			builderImageOpts = append(builderImageOpts, llb.IgnoreCache)
		}
		builder := llb.Image(NixImage, builderImageOpts...)

		// Builder working directory is set to /mnt/workspace,
		// so that the nix build result can be copied to /mnt/workspace/result and extracted later
		builder = builder.With(llb.Dir(mountWorkspaceDir))

		// setup build environment
		builder = builder.Run(
			llb.AddMount(mountSourceDir, *mainContext, llb.Readonly),
			nixllb.Shlexf(`
				nix --version
				{
					echo "auto-optimise-store = true"
					echo "binary-caches-parallel-connections = 15"
					echo "extra-experimental-features = flakes"
					echo "extra-experimental-features = nix-command"
					echo "filter-syscalls = false"
					# echo "sandbox = false" -- already set by default from nixos/nix image
				} >> /etc/nix/nix.conf
				cat /etc/nix/nix.conf

				# This is a fake config for debugging purposes,
				# it will be printed in the build logs, but it will not be used by nix
				echo "nix-store-cache-key = %s"
			`, nixStoreCacheKey),
			withInternalName("setup build environment"),
		).Root()

		// restore nix store cache
		nixStoreRestoreOpts := []llb.RunOption{
			llb.AddMount(mountSourceDir, *mainContext, llb.Readonly),
			llb.AddMount(mountNixStoreCacheDir, nixStore, llb.AsPersistentCacheDir(nixStoreCacheKey, llb.CacheMountLocked)),
			nixllb.Shlexf(`
				export NIX_STORE_CACHE_DIR="%s"
				ls /nix/store > /tmp/nix-store-before.txt
				if [ -d "${NIX_STORE_CACHE_DIR}/var" ]; then
					cp -afT "${NIX_STORE_CACHE_DIR}/var" /nix/var
				fi
				if [ -d "${NIX_STORE_CACHE_DIR}/store" ]; then
					for f in "${NIX_STORE_CACHE_DIR}/store"/*; do
						if ! grep -q "$(basename "$f")" /tmp/nix-store-before.txt; then
							echo "copying path \"$f\" from nix store snapshot..."
							cp -afT "$f" "/nix/store/$(basename "$f")"
						fi
					done
					ls /nix/store > /tmp/nix-store-before.txt
				fi
			`, mountNixStoreCacheDir),
			withInternalName("configure nix store"),
		}
		if bc.IsNoCache("nix-store") {
			nixStoreRestoreOpts = append(nixStoreRestoreOpts, llb.IgnoreCache)
		}
		builder = builder.Run(nixStoreRestoreOpts...).Root()

		// Nix build
		nixBuildOpts := []llb.RunOption{
			llb.AddMount(mountSourceDir, *mainContext, llb.Readonly),
			llb.AddMount(mountNixStoreCacheDir, nixStore, llb.AsPersistentCacheDir(nixStoreCacheKey, llb.CacheMountLocked)),
			llb.AddMount("/build", llb.Scratch()),
			nixllb.Shlexf(`
				set -euo pipefail

				nixopts=()
				installable="%s#%s"

				echo "Prepare build environment..."
				echo "extra-experimental-features = configurable-impure-env" >> /etc/nix/nix.conf

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
				nix "${nixopts[@]}" --show-trace --log-format raw build "$installable"
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
		builder = builder.Run(nixBuildOpts...).Root()

		// save nix store cache
		nixStoreSaveOpts := []llb.RunOption{
			llb.AddMount(mountSourceDir, *mainContext, llb.Readonly),
			llb.AddMount(mountNixStoreCacheDir, nixStore, llb.AsPersistentCacheDir(nixStoreCacheKey, llb.CacheMountLocked)),
			nixllb.Shlexf(`
				export NIX_STORE_CACHE_DIR="%s"
				mkdir -p "${NIX_STORE_CACHE_DIR}/store" "${NIX_STORE_CACHE_DIR}/var"
				cp -afT /nix/var "${NIX_STORE_CACHE_DIR}/var"
				for f in /nix/store/*; do
					if ! grep -q "$(basename "$f")" /tmp/nix-store-before.txt; then
						echo "copying path \"$f\" to nix store snapshot..."
						cp -afT "$f" "${NIX_STORE_CACHE_DIR}/store/$(basename "$f")"
					fi
				done
			`, mountNixStoreCacheDir),
			withInternalName("create nix store snapshot"),
		}
		if bc.IsNoCache("nix-store") {
			nixStoreSaveOpts = append(nixStoreSaveOpts, llb.IgnoreCache)
		}
		builder = builder.Run(nixStoreSaveOpts...).Root()

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
				builder,
				fmt.Sprintf("%s/result", mountWorkspaceDir), "/", &llb.CopyInfo{
					AttemptUnpack: true,
				},
			),
			extractFileOpts...,
		)

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

		// Prepare the final definition from final state
		layeredDef, err := layered.Marshal(ctx)
		if err != nil {
			return nil, err
		}
		layeredRes, err := c.Solve(ctx, client.SolveRequest{
			Definition:   layeredDef.ToPB(),
			CacheImports: bc.CacheImports,
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
