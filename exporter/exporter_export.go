package exporter

import (
	"context"
	"fmt"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/socheatsok78/buildkit-nix/exporter/exptypes"
	"github.com/socheatsok78/buildkit-nix/exporter/plugins"

	// Register the available plugins by importing their packages
	_ "github.com/socheatsok78/buildkit-nix/exporter/plugins/derivation"
	_ "github.com/socheatsok78/buildkit-nix/exporter/plugins/oci"
	_ "github.com/socheatsok78/buildkit-nix/exporter/plugins/shelter"
)

func Export(ctx context.Context, c client.Client, st llb.State, opts ...exptypes.ExportOption) (exptypes.ExportResult, error) {
	cfg := exptypes.ExportConfig{State: st}
	for _, opt := range opts {
		opt.SetExporterOption(cfg)
	}

	def, err := cfg.State.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
	if err != nil {
		return exptypes.ExportResult{}, err
	}

	res, err := c.Solve(ctx, client.SolveRequest{
		Definition:   def.ToPB(),
		CacheImports: cfg.CacheImports,
	})
	if err != nil {
		return exptypes.ExportResult{}, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return exptypes.ExportResult{}, err
	}

	resultTypeByte, err := ref.ReadFile(ctx, client.ReadRequest{Filename: "/type"})
	if err != nil {
		return exptypes.ExportResult{}, err
	}
	resultType := strings.TrimSpace(string(resultTypeByte))

	// Use the result type to select the appropriate plugin for exporting
	plugin, ok := plugins.GetPlugin(resultType)
	if !ok {
		return exptypes.ExportResult{}, fmt.Errorf("no plugin found for result type: %s", resultType)
	}

	// Call the plugin's Export method to handle the export process
	return plugin.Export(ctx, c, cfg)
}
