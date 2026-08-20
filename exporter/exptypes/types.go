package exptypes

import (
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	dockerocispec "github.com/moby/docker-image-spec/specs-go/v1"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ExportConfig struct {
	State        llb.State
	Platform     ocispec.Platform
	CacheImports []client.CacheOptionsEntry
}

type ExportResult struct {
	llb.State
	DockerOCIImage dockerocispec.DockerOCIImage
}

type ExporterInfo struct {
	IgnoreCache            bool
	MultiPlatformRequested bool
}

type ExporterOption interface {
	SetExporterOption(info ExporterInfo)
}

type ExporterOptionFunc func(info ExporterInfo)

func (f ExporterOptionFunc) SetExporterOption(info ExporterInfo) {
	f(info)
}
