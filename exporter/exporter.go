package exporter

import (
	"github.com/moby/buildkit/frontend/gateway/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/socheatsok78/buildkit-nix/exporter/exptypes"
)

func CacheImports(imports []client.CacheOptionsEntry) exptypes.ExportOption {
	return exptypes.ExporterOptionFunc(func(info exptypes.ExportConfig) {
		info.CacheImports = imports
	})
}

func ShouldIgnoreCache(ignoreCache bool) exptypes.ExportOption {
	return exptypes.ExporterOptionFunc(func(info exptypes.ExportConfig) {
		info.IgnoreCache = ignoreCache
	})
}

func MultiPlatformRequested(multiPlatformRequested bool) exptypes.ExportOption {
	return exptypes.ExporterOptionFunc(func(info exptypes.ExportConfig) {
		info.MultiPlatformRequested = multiPlatformRequested
	})
}

func Platform(p ocispec.Platform) exptypes.ExportOption {
	return exptypes.ExporterOptionFunc(func(info exptypes.ExportConfig) {
		info.Platform = p
	})
}
