package toolbox

import (
	"embed"

	"github.com/moby/buildkit/client/llb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixllb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

//go:embed buildkit-nix-*.sh
var toolbox embed.FS

// Install copy the buildkit-nix scripts into the given llb.State.
func Install(st llb.State, ignoreCache bool) (llb.State, error) {
	entries, err := toolbox.ReadDir(".")
	if err != nil {
		return st, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		dt, _ := toolbox.ReadFile(entry.Name())
		st = st.File(
			llb.Mkfile("/etc/nix/"+entry.Name(), 0755, dt),
			nixllb.ShouldIgnoreCache(ignoreCache),
			nixui.WithInternalNameTag("toolbox")("copying path "+entry.Name()),
		)
	}

	return st, nil
}
