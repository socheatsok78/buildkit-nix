package nixllb

import (
	"fmt"

	"github.com/moby/buildkit/client/llb"
)

func Shlex(str string) llb.RunOption {
	return llb.Shlexf("sh -c '%s'", str)
}

func Shlexf(str string, v ...any) llb.RunOption {
	return Shlex(fmt.Sprintf(str, v...))
}
