package builder

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixllb"
)

const (
	keyNixSecretArgPrefix = buildArgPrefix + "nix.secret."
)

// NixSecretRunOptions parses the nix option secrets from the build options and returns a list of llb.RunOption to be used in the nix build command.
func NixSecretRunOptions(opts map[string]string) ([]llb.RunOption, error) {
	runOpts := []llb.RunOption{}
	for k, v := range opts {
		if strings.HasPrefix(k, keyNixSecretArgPrefix) {
			dest := strings.TrimPrefix(k, keyNixSecretArgPrefix)
			s, err := parseSecretArg(v)
			if err != nil {
				return nil, err
			}
			runOpts = append(runOpts, nixllb.WithRunSecret(dest, s))
		}
	}
	return runOpts, nil
}

// parseSecretArg parses a secret argument string into a llb.SecretInfo struct.
// The secret argument string can be in the following formats:
// - id=<secret-id> (to pass the secret as a file) (default: /run/secrets/<secret-id>)
// - id=<secret-id>,target=<file-path> (to pass the secret as a file) (default: /run/secrets/<secret-id>)
// - id=<secret-id>,env=<env-var-name> (to pass the secret as an environment variable)
// the `optional` or `optional=<bool>` flag can be added to the secret definition to make it optional (default: false)
func parseSecretArg(v string) (*llb.SecretInfo, error) {
	s := &llb.SecretInfo{Optional: false}

	_parts := strings.Split(v, ",")
	for _, part := range _parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := kv[0]
		value := kv[1]

		if value == "" {
			switch key {
			case "optional":
				s.Optional = true
			default:
				return nil, fmt.Errorf("secret option %s requires a value", key)
			}
		}

		switch key {
		case "id":
			s.ID = value
		case "target":
			s.Target = &value
		case "env":
			s.Env = &value
		case "optional":
			if optional, err := strconv.ParseBool(value); err == nil {
				s.Optional = optional
			}
		default:
			return nil, fmt.Errorf("unknown secret option: %s", key)
		}
	}

	return s, nil
}
