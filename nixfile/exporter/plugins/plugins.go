package plugins

import (
	"context"

	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/socheatsok78/buildkit-nix/nixfile/exporter/types"
)

var registeredPlugins = make(map[string]Plugin)

type Plugin interface {
	Export(ctx context.Context, c client.Client, cfg types.ExportConfig) (types.ExportResult, error)
}

func RegisterPlugin(name string, plugin Plugin) {
	registeredPlugins[name] = plugin
}

func GetPlugin(name string) (Plugin, bool) {
	plugin, ok := registeredPlugins[name]
	return plugin, ok
}
