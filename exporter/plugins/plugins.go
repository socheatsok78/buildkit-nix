package plugins

import (
	"context"

	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/socheatsok78/buildkit-nix/exporter/export"
)

var registeredPlugins = make(map[string]Plugin)

type Plugin interface {
	Export(ctx context.Context, c client.Client, cfg export.ExportConfig) (export.ExportResult, error)
}

func RegisterPlugin(name string, plugin Plugin) {
	registeredPlugins[name] = plugin
}

func GetPlugin(name string) (Plugin, bool) {
	plugin, ok := registeredPlugins[name]
	return plugin, ok
}
