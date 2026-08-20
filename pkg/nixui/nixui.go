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

func WithInternalNameTag(tag string) func(name string) llb.ConstraintsOpt {
	return func(name string) llb.ConstraintsOpt {
		return llb.WithCustomNamef("[nix %s] %s", tag, name)
	}
}
