package exporter

import (
	"context"
	"fmt"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/socheatsok78/buildkit-nix/nixfile/exporter/plugins"
	"github.com/socheatsok78/buildkit-nix/nixfile/exporter/types"

	// Register the available plugins by importing their packages
	_ "github.com/socheatsok78/buildkit-nix/nixfile/exporter/plugins/derivation"
	_ "github.com/socheatsok78/buildkit-nix/nixfile/exporter/plugins/ocispec"
)

func Export(ctx context.Context, c client.Client, cfg types.ExportConfig) (types.ExportResult, error) {
	def, err := cfg.State.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
	if err != nil {
		return types.ExportResult{}, err
	}

	res, err := c.Solve(ctx, client.SolveRequest{
		Definition: def.ToPB(),
	})
	if err != nil {
		return types.ExportResult{}, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return types.ExportResult{}, err
	}

	resultTypeByte, err := ref.ReadFile(ctx, client.ReadRequest{Filename: "/type"})
	if err != nil {
		return types.ExportResult{}, err
	}
	resultType := strings.TrimSpace(string(resultTypeByte))

	// Use the result type to select the appropriate plugin for exporting
	plugin, ok := plugins.GetPlugin(resultType)
	if !ok {
		return types.ExportResult{}, fmt.Errorf("no plugin found for result type: %s", resultType)
	}

	// Call the plugin's Export method to handle the export process
	return plugin.Export(ctx, c, cfg)
}
