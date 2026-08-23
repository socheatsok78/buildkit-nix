package nixllb

import (
	"fmt"

	"github.com/moby/buildkit/client/llb"
)

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

func WithAccessTokenSecret(dest string, s *llb.SecretInfo) llb.RunOption {
	return llb.AddSecret(
		fmt.Sprintf("/run/access-tokens/%s", dest),
		llb.SecretID(dest),
		AddSecretInfo(s),
	)
}

func WithImpureEnvSecret(dest string, s *llb.SecretInfo) llb.RunOption {
	return llb.AddSecret(
		dest,
		llb.SecretID(dest),
		llb.SecretAsEnv(true),
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
