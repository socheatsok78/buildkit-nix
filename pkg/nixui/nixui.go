package nixui

import (
	"github.com/moby/buildkit/client/llb"
)

func WithInternalName(name string) llb.ConstraintsOpt {
	return llb.WithCustomName("[nix] " + name)
}

func WithInternalNamef(format string, args ...interface{}) llb.ConstraintsOpt {
	return llb.WithCustomNamef("[nix "+format+"]", args...)
}

func WithInternalNameTag(suffix string) func(name string) llb.ConstraintsOpt {
	return func(name string) llb.ConstraintsOpt {
		return llb.WithCustomNamef("[nix %s] %s", suffix, name)
	}
}
