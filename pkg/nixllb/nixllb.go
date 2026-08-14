package nixllb

import (
	"fmt"

	"github.com/moby/buildkit/client/llb"
)

func ProgressGroup(id string, name string, weak bool) llb.ConstraintsOpt {
	return llb.ProgressGroup(id, fmt.Sprintf("[nix] %s", name), weak)
}
