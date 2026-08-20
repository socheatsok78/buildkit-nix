package export

import (
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	dockerocispec "github.com/moby/docker-image-spec/specs-go/v1"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ExportResult struct {
	llb.State
	DockerOCIImage dockerocispec.DockerOCIImage
}

type ExportConfig struct {
	State llb.State

	CacheImports           []client.CacheOptionsEntry
	IgnoreCache            bool
	MultiPlatformRequested bool
	Platform               ocispec.Platform
}

type ExportOption interface {
	SetExporterOption(info ExportConfig)
}

type ExporterOptionFunc func(info ExportConfig)

func (f ExporterOptionFunc) SetExporterOption(info ExportConfig) {
	f(info)
}
