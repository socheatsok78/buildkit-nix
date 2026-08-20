package builder

import (
	"fmt"

	"github.com/containerd/platforms"
	"github.com/moby/buildkit/client/llb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixllb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

func (nixbld *Builder) Build(target string, source llb.State) llb.State {
	st := nixbld.State

	st = st.Run(
		llb.AddEnv("BUILDKIT_NIX_STORE_CACHE_KEY", nixbld.NixStoreCacheKey),
		llb.AddEnv("BUILDKIT_NIX_USER_CONFIGS", nixbld.NixUserConfigs),
		llb.Shlexf(`/etc/nix/buildkit-nix-configure.sh`),
		withInternalName("configure nix.conf", nixbld.NixMultiPlatformRequested),
		nixllb.ShouldIgnoreCache(nixbld.IgnoreCache),
	).Root()

	st = st.
		Run(
			llb.Security(nixbld.NixSecurityMode),

			llb.AddEnv("BUILDKIT_NIX_SHELTER_DIR", mountShelterDir),

			llb.AddMount("/nix", st, llb.SourcePath("/nix"), llb.AsPersistentCacheDir(nixbld.NixStoreCacheKey, llb.CacheMountLocked)),
			llb.AddMount("/build", llb.Scratch()),
			llb.AddMount(mountShelterDir, llb.Scratch()),
			llb.AddMount(mountSourceDir, source),

			llb.Dir(mountSourceDir),
			llb.Shlexf(`/etc/nix/buildkit-nix-build.sh ".#%s"`, target),

			// Special secret for GitHub token, which is used to access private repositories
			llb.AddSecret("GITHUB_TOKEN", llb.SecretID("GITHUB_TOKEN"), llb.SecretAsEnv(true), llb.SecretOptional),

			withInternalName(fmt.Sprintf("nix build .#%s", target), nixbld.NixMultiPlatformRequested),
			nixllb.ShouldIgnoreCache(nixbld.IgnoreCache),
		).
		GetMount(mountShelterDir)

	return st
}

func withInternalName(name string, multiPlatformRequested bool) llb.ConstraintsOpt {
	if multiPlatformRequested {
		p := platforms.DefaultSpec()
		return nixui.WithInternalNameTag(fmt.Sprintf("builder %s/%s", p.OS, p.Architecture))(name)
	}
	return nixui.WithInternalNameTag("builder")(name)
}
