package exporter

import (
	"github.com/moby/buildkit/client/llb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

func withInternalNameW(name string) llb.ConstraintsOpt {
	return nixui.WithInternalName(name)
}
