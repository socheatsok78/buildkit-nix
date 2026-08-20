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

func ShouldIgnoreCache(ignore bool) export.ExportOption {
	return export.ExporterOptionFunc(func(cfg export.ExportConfig) {
		cfg.IgnoreCache = ignore
	})
}

func MultiPlatformRequested(requested bool) export.ExportOption {
	return export.ExporterOptionFunc(func(cfg export.ExportConfig) {
		cfg.MultiPlatformRequested = requested
	})
}

func Platform(p ocispec.Platform) export.ExportOption {
	return export.ExporterOptionFunc(func(cfg export.ExportConfig) {
		cfg.Platform = p
	})
}
