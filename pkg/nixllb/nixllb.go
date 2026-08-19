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

// ResetExcludePatterns returns an llb.LocalOption that resets the exclude patterns for the local context to an empty list.
func ResetExcludePatterns() llb.LocalOption {
	return llb.ExcludePatterns([]string{})
}
