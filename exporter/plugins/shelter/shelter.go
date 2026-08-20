package shelter

import (
	"context"

	"github.com/moby/buildkit/frontend/gateway/client"
	dockerocispec "github.com/moby/docker-image-spec/specs-go/v1"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/socheatsok78/buildkit-nix/exporter/exptypes"
	"github.com/socheatsok78/buildkit-nix/exporter/plugins"
)

func init() {
	plugins.RegisterPlugin("shelter", &ShelterExporterPlugin{})
}

var _ plugins.Plugin = &ShelterExporterPlugin{}

type ShelterExporterPlugin struct{}

func (p *ShelterExporterPlugin) Export(ctx context.Context, c client.Client, cfg exptypes.ExportConfig) (exptypes.ExportResult, error) {
	result := exptypes.ExportResult{
		State: cfg.State,
		DockerOCIImage: dockerocispec.DockerOCIImage{
			Image: ocispec.Image{Platform: cfg.Platform},
		},
	}
	return result, nil
}
