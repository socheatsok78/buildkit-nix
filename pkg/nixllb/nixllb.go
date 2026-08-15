package nixllb

import (
	"github.com/moby/buildkit/client/llb"
)

var Noop = llb.WithDescription(map[string]string{})

func ShouldIgnoreCache(s bool) llb.ConstraintsOpt {
	if s {
		return llb.IgnoreCache
	}
	return Noop
}
