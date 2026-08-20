package derivation

import (
	"context"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	dockerocispec "github.com/moby/docker-image-spec/specs-go/v1"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/socheatsok78/buildkit-nix/exporter/plugins"
	"github.com/socheatsok78/buildkit-nix/exporter/types"
	"github.com/socheatsok78/buildkit-nix/pkg/nixllb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

func init() {
	plugins.RegisterPlugin("derivation", &DerivationExporterPlugin{})
}

var _ plugins.Plugin = &DerivationExporterPlugin{}

type DerivationExporterPlugin struct{}

func (p *DerivationExporterPlugin) Export(ctx context.Context, c client.Client, cfg types.ExportConfig) (types.ExportResult, error) {
	nixStoreClosure := llb.Scratch().File(
		llb.Copy(cfg.State, "/result", "/", &llb.CopyInfo{CopyDirContentsOnly: true}),
		nixllb.ShouldIgnoreCache(cfg.IgnoreCache),
		withInternalNameW("copying nix store closure..."),
	)

	result := types.ExportResult{
		State: nixStoreClosure,
		DockerOCIImage: dockerocispec.DockerOCIImage{
			Image: ocispec.Image{Platform: cfg.Platform},
		},
	}

	return result, nil
}

func withInternalNameW(name string) llb.ConstraintsOpt {
	return nixui.WithInternalNameTag("exporter/derivation")(name)
}
