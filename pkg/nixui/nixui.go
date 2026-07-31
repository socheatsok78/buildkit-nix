package nixui

import "github.com/moby/buildkit/client/llb"

func WithInternalName(name string) llb.ConstraintsOpt {
	return llb.WithCustomName("[nix] " + name)
}
