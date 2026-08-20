package builder

import (
	"github.com/containerd/platforms"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/socheatsok78/buildkit-nix/builder/toolbox"
	"github.com/socheatsok78/buildkit-nix/pkg/nixllb"
)

const (
	mountNixStoreCacheDir = "/mnt/nix"
	mountSourceDir        = "/mnt/source"
	mountShelterDir       = "/shelter"
)

type Builder struct {
	llb.State

	NixBuildSecrets           []llb.RunOption
	NixBuildTargetPlatform    ocispec.Platform
	NixIgnoreCache            bool
	NixImageOpts              []llb.ImageOption
	NixMultiPlatformRequested bool
	NixSecurityMode           pb.SecurityMode
	NixStoreCacheKey          string
	NixUserConfigs            string
}

func NewBuilder(ref string, opts ...BuilderOption) *Builder {
	nixbld := &Builder{
		NixBuildTargetPlatform:    platforms.DefaultSpec(),
		NixIgnoreCache:            false,
		NixMultiPlatformRequested: false,
		NixSecurityMode:           llb.SecurityModeSandbox,
		NixStoreCacheKey:          ref,
		NixUserConfigs:            "",
	}
	for _, opt := range opts {
		opt.SetBuilderOption(nixbld)
	}

	st := llb.Image(ref, append(nixbld.NixImageOpts, nixllb.ShouldIgnoreCache(nixbld.NixIgnoreCache))...)

	st = toolbox.Install(
		st,
		toolbox.MultiPlatformRequested(nixbld.NixMultiPlatformRequested),
		toolbox.ShouldIgnoreCache(nixbld.NixIgnoreCache),
		toolbox.Platform(nixbld.NixBuildTargetPlatform),
	)

	nixbld.State = st

	return nixbld
}

type BuilderOption interface {
	SetBuilderOption(*Builder)
}

type buildOptionFunc func(*Builder)

func (f buildOptionFunc) SetBuilderOption(b *Builder) {
	f(b)
}

func NixBuildSecrets(opts ...llb.RunOption) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.NixBuildSecrets = append(b.NixBuildSecrets, opts...)
	})
}

func NixBuildTargetPlatform(platform ocispec.Platform) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.NixBuildTargetPlatform = platform
		b.NixImageOpts = append(b.NixImageOpts, llb.Platform(platform))
	})
}

func NixShouldIgnoreCache(ignore bool) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.NixIgnoreCache = ignore
	})
}

func ImageOptions(opts ...llb.ImageOption) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.NixImageOpts = append(b.NixImageOpts, opts...)
	})
}

func NixMultiPlatformRequested(requested bool) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.NixMultiPlatformRequested = requested
	})
}

func NixStoreCacheKey(key string) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.NixStoreCacheKey = key
	})
}

func NixUserConfigs(configs string) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.NixUserConfigs = configs
	})
}

func NixSecurityMode(mode pb.SecurityMode) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.NixSecurityMode = mode
	})
}
