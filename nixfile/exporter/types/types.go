package types

import (
	"github.com/moby/buildkit/client/llb"
	dockerocispec "github.com/moby/docker-image-spec/specs-go/v1"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ExportConfig struct {
	State       llb.State
	Platform    ocispec.Platform
	IgnoreCache bool
}

type ExportResult struct {
	llb.State
	DockerOCIImage dockerocispec.DockerOCIImage
}
