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

type secretOptionFunc func(*llb.SecretInfo)

func (fn secretOptionFunc) SetSecretOption(si *llb.SecretInfo) {
	fn(si)
}

func WithRunSecret(dest string, s *llb.SecretInfo) llb.RunOption {
	return llb.AddSecret(
		fmt.Sprintf("/run/secrets/%s", dest),
		llb.SecretID(dest),
		AddSecretInfo(s),
	)
}

func AddSecretInfo(s *llb.SecretInfo) llb.SecretOption {
	return secretOptionFunc(func(si *llb.SecretInfo) {
		if s.ID != "" {
			si.ID = s.ID
		}
		if s.Target != nil {
			si.Target = s.Target
		}
		if s.Env != nil {
			si.Env = s.Env
		}
		si.Optional = s.Optional
	})
}
