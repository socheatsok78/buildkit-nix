package toolbox

import (
	"embed"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/identity"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

//go:embed buildkit-nix-*.sh
var fs embed.FS

// Install copy the buildkit-nix scripts into the given llb.State.
func Install(st llb.State) (llb.State, error) {
	entries, err := fs.ReadDir(".")
	if err != nil {
		return st, err
	}
	pgId := identity.NewID()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		dt, _ := fs.ReadFile(entry.Name())
		st = st.File(
			llb.Mkfile("/etc/nix/"+entry.Name(), 0755, dt),
			llb.ProgressGroup(pgId, "[nix toolbox] install", false),
			nixui.WithInternalNameTag("toolbox")("install: "+entry.Name()),
		)
	}

	return st, nil
}
