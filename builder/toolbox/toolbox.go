package toolbox

import (
	"embed"
	"fmt"

	"github.com/moby/buildkit/client/llb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

//go:embed buildkit-nix-*.sh
var toolbox embed.FS

// Install copy the buildkit-nix scripts into the given llb.State.
func Install(st llb.State, opts ...llb.ConstraintsOpt) (llb.State, error) {
	entries, err := toolbox.ReadDir(".")
	if err != nil {
		return st, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		dt, _ := toolbox.ReadFile(entry.Name())
		filepath := "/etc/nix/" + entry.Name()
		st = st.File(llb.Mkfile(filepath, 0755, dt), append([]llb.ConstraintsOpt{
			withInternalNameW(fmt.Sprintf("copying path '%s' from toolbox...", filepath)),
		}, opts...)...)
	}

	return st, nil
}

func withInternalNameW(name string) llb.ConstraintsOpt {
	return nixui.WithInternalNameTag("builder")(name)
}
