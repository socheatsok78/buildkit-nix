package types

import (
	"context"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	dockerocispec "github.com/moby/docker-image-spec/specs-go/v1"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Exporter struct {
	Ctx    context.Context
	Client client.Client
}

type ExportConfig struct {
	State       llb.State
	Platform    ocispec.Platform
	IgnoreCache bool
}

type ExportResult struct {
	llb.State
	DockerOCIImage dockerocispec.DockerOCIImage
}
