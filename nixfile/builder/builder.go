package builder

import (
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
)

const (
	mountNixStoreCacheDir = "/mnt/nix"
	mountSourceDir        = "/mnt/source"
	mountShelterDir       = "/shelter"
)

type Builder struct {
	llb.State
	IgnoreCache      bool
	ImageOpts        []llb.ImageOption
	NixBuildSecrets  []llb.RunOption
	NixStoreCacheKey string
	NixUserConfigs   string
	SecurityMode     pb.SecurityMode
}

func NewBuilder(ref string, opts ...BuilderOption) *Builder {
	nixbld := &Builder{NixStoreCacheKey: ref, NixUserConfigs: ""}
	for _, opt := range opts {
		opt.SetBuilderOption(nixbld)
	}
	nixbld.State = llb.Image(ref, nixbld.ImageOpts...)
	return nixbld
}

type BuilderOption interface {
	SetBuilderOption(*Builder)
}

type buildOptionFunc func(*Builder)

func (f buildOptionFunc) SetBuilderOption(b *Builder) {
	f(b)
}

func ShouldIgnoreCache(ignore bool) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.IgnoreCache = ignore
	})
}

func ImageOptions(opts ...llb.ImageOption) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.ImageOpts = append(b.ImageOpts, opts...)
	})
}

func NixBuildSecrets(opts ...llb.RunOption) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.NixBuildSecrets = append(b.NixBuildSecrets, opts...)
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

func SecurityMode(mode pb.SecurityMode) BuilderOption {
	return buildOptionFunc(func(b *Builder) {
		b.SecurityMode = mode
	})
}
