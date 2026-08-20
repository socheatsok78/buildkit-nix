package toolbox

import (
	"embed"
	"fmt"

	"github.com/containerd/platforms"
	"github.com/moby/buildkit/client/llb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

//go:embed buildkit-nix-*.sh
var toolbox embed.FS

// Install copy the buildkit-nix scripts into the given llb.State.
func Install(st llb.State, opts ...ToolboxOptions) (llb.State, error) {
	var cfg ToolboxInfo
	for _, opt := range opts {
		opt.SetToolboxOptions(cfg)
	}

	// Copy the buildkit-nix scripts from the embedded filesystem into the llb.State
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
		st = st.File(
			llb.Mkfile(filepath, 0755, dt),
			withInternalName(fmt.Sprintf("copying path '%s' from toolbox...", filepath), cfg.MultiPlatformRequested),
		)
	}

	return st, nil
}

func withInternalName(name string, multiPlatformRequested bool) llb.ConstraintsOpt {
	if multiPlatformRequested {
		p := platforms.DefaultSpec()
		return nixui.WithInternalNameTag(fmt.Sprintf("builder %s/%s", p.OS, p.Architecture))(name)
	}
	return nixui.WithInternalNameTag("builder")(name)
}

type ToolboxInfo struct {
	IgnoreCache            bool
	MultiPlatformRequested bool
}

type ToolboxOptions interface {
	SetToolboxOptions(cfg ToolboxInfo)
}

type toolboxOptionFunc func(cfg *ToolboxInfo)

func (f toolboxOptionFunc) SetToolboxOptions(cfg ToolboxInfo) {
	f(&cfg)
}

func ShouldIgnoreCache(ignore bool) ToolboxOptions {
	return toolboxOptionFunc(func(cfg *ToolboxInfo) {
		cfg.IgnoreCache = ignore
	})
}

func MultiPlatformRequested(requested bool) ToolboxOptions {
	return toolboxOptionFunc(func(cfg *ToolboxInfo) {
		cfg.MultiPlatformRequested = requested
	})
}
