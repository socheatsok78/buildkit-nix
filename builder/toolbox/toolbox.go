package toolbox

import (
	"embed"
	"fmt"

	"github.com/moby/buildkit/client/llb"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

//go:embed buildkit-nix-*.sh
var toolbox embed.FS

// Install copy the buildkit-nix scripts into the given llb.State.
func Install(st llb.State, opts ...ToolboxOptions) llb.State {
	var cfg ToolboxConfig
	for _, opt := range opts {
		opt.SetToolboxOptions(&cfg)
	}

	// Copy the buildkit-nix scripts from the embedded filesystem into the llb.State
	entries, _ := toolbox.ReadDir(".")
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		dt, _ := toolbox.ReadFile(entry.Name())
		filepath := "/etc/nix/" + entry.Name()
		st = st.File(
			llb.Mkfile(filepath, 0755, dt),
			withInternalName(fmt.Sprintf("copying path '%s' from toolbox...", filepath), cfg.Platform, cfg.MultiPlatformRequested),
		)
	}

	return st
}

func withInternalName(name string, p ocispec.Platform, multiPlatformRequested bool) llb.ConstraintsOpt {
	if multiPlatformRequested {
		return nixui.WithInternalNameTag(fmt.Sprintf("builder %s/%s", p.OS, p.Architecture))(name)
	}
	return nixui.WithInternalNameTag("builder")(name)
}

type ToolboxConfig struct {
	IgnoreCache            bool
	MultiPlatformRequested bool
	Platform               ocispec.Platform
}

type ToolboxOptions interface {
	SetToolboxOptions(cfg *ToolboxConfig)
}

type toolboxOptionFunc func(cfg *ToolboxConfig)

func (fn toolboxOptionFunc) SetToolboxOptions(cfg *ToolboxConfig) {
	fn(cfg)
}

func ShouldIgnoreCache(ignore bool) ToolboxOptions {
	return toolboxOptionFunc(func(cfg *ToolboxConfig) {
		cfg.IgnoreCache = ignore
	})
}

func MultiPlatformRequested(requested bool) ToolboxOptions {
	return toolboxOptionFunc(func(cfg *ToolboxConfig) {
		cfg.MultiPlatformRequested = requested
	})
}

func Platform(platform ocispec.Platform) ToolboxOptions {
	return toolboxOptionFunc(func(cfg *ToolboxConfig) {
		cfg.Platform = platform
	})
}
