package nixllb

import (
	"fmt"

	"github.com/moby/buildkit/client/llb"
)

func ProgressGroup(id string, name string, weak bool) llb.ConstraintsOpt {
	return llb.ProgressGroup(id, fmt.Sprintf("[nix] %s", name), weak)
}

func Shlex(str string) llb.RunOption {
	return llb.Shlexf("sh -c '%s'", str)
}

func Shlexf(str string, v ...any) llb.RunOption {
	return Shlex(fmt.Sprintf(str, v...))
}
