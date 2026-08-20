package builder

import (
	"fmt"

	"github.com/containerd/platforms"
	"github.com/moby/buildkit/client/llb"
	"github.com/socheatsok78/buildkit-nix/builder/toolbox"
	"github.com/socheatsok78/buildkit-nix/pkg/nixllb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

func (nixbld *Builder) Build(target string, source llb.State) llb.State {
	nixbld.State, _ = toolbox.Install(
		nixbld.State,
		toolbox.MultiPlatformRequested(nixbld.NixMultiPlatformRequested),
		toolbox.ShouldIgnoreCache(nixbld.IgnoreCache),
	)

	nixbld.State = nixbld.State.Run(
		llb.AddEnv("BUILDKIT_NIX_STORE_CACHE_KEY", nixbld.NixStoreCacheKey),
		llb.AddEnv("BUILDKIT_NIX_USER_CONFIGS", nixbld.NixUserConfigs),
		llb.Shlexf(`/etc/nix/buildkit-nix-configure.sh`),
		withInternalName("configure nix.conf", nixbld.NixMultiPlatformRequested),
		nixllb.ShouldIgnoreCache(nixbld.IgnoreCache),
	).Root()

	nixbld.State = nixbld.State.Run(
		llb.Security(nixbld.NixSecurityMode),

		llb.AddEnv("BUILDKIT_NIX_SHELTER_DIR", mountShelterDir),

		llb.AddMount("/nix", nixbld.State, llb.SourcePath("/nix"), llb.AsPersistentCacheDir(nixbld.NixStoreCacheKey, llb.CacheMountLocked)),
		llb.AddMount("/build", llb.Scratch()),
		llb.AddMount(mountShelterDir, llb.Scratch()),
		llb.AddMount(mountSourceDir, source),

		llb.Dir(mountSourceDir),
		llb.AddEnv("NIX_SHOW_STATS", "1"),
		llb.Shlexf(`/etc/nix/buildkit-nix-build.sh ".#%s"`, target),

		// Special secret for GitHub token, which is used to access private repositories
		llb.AddSecret("GITHUB_TOKEN", llb.SecretID("GITHUB_TOKEN"), llb.SecretAsEnv(true), llb.SecretOptional),

		withInternalName(fmt.Sprintf("nix build .#%s", target), nixbld.NixMultiPlatformRequested),
		nixllb.ShouldIgnoreCache(nixbld.IgnoreCache),
	).GetMount(mountShelterDir)

	return nixbld.State
}

func withInternalName(name string, multiPlatformRequested bool) llb.ConstraintsOpt {
	if multiPlatformRequested {
		p := platforms.DefaultSpec()
		return nixui.WithInternalNameTag(fmt.Sprintf("builder %s/%s", p.OS, p.Architecture))(name)
	}
	return nixui.WithInternalNameTag("builder")(name)
}

func mergeSlices[T any](slices ...[]T) []T {
	totalLength := 0
	for _, s := range slices {
		totalLength += len(s)
	}
	result := make([]T, 0, totalLength)
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}
