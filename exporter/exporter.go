package exporter

import (
	"github.com/moby/buildkit/frontend/gateway/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/socheatsok78/buildkit-nix/exporter/export"
)

func CacheImports(imports []client.CacheOptionsEntry) export.ExportOption {
	return export.ExporterOptionFunc(func(cfg export.ExportConfig) {
		cfg.CacheImports = imports
	})
}

func ShouldIgnoreCache(ignoreCache bool) export.ExportOption {
	return export.ExporterOptionFunc(func(cfg export.ExportConfig) {
		cfg.IgnoreCache = ignoreCache
	})
}

func MultiPlatformRequested(multiPlatformRequested bool) export.ExportOption {
	return export.ExporterOptionFunc(func(cfg export.ExportConfig) {
		cfg.MultiPlatformRequested = multiPlatformRequested
	})
}

func Platform(p ocispec.Platform) export.ExportOption {
	return export.ExporterOptionFunc(func(cfg export.ExportConfig) {
		cfg.Platform = p
	})
}
