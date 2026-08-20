package oci

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/containerd/platforms"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	dockerocispec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/socheatsok78/buildkit-nix/exporter/exptypes"
	"github.com/socheatsok78/buildkit-nix/exporter/plugins"
	"github.com/socheatsok78/buildkit-nix/pkg/dockershim"
	"github.com/socheatsok78/buildkit-nix/pkg/nixllb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

func init() {
	plugins.RegisterPlugin("oci", &OCIExporterPlugin{})
}

var _ plugins.Plugin = &OCIExporterPlugin{}

type OCIExporterPlugin struct{}

func (p *OCIExporterPlugin) Export(ctx context.Context, c client.Client, cfg exptypes.ExportConfig) (exptypes.ExportResult, error) {
	nixStoreClosure := llb.Scratch().File(
		llb.Copy(cfg.State, "/result", "/", &llb.CopyInfo{AttemptUnpack: true}),
		nixllb.ShouldIgnoreCache(cfg.IgnoreCache),
		withInternalName("evaluating nix store closure", cfg.MultiPlatformRequested),
	)

	def, err := nixStoreClosure.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
	if err != nil {
		return exptypes.ExportResult{}, err
	}

	res, err := c.Solve(ctx, client.SolveRequest{
		Definition:   def.ToPB(),
		CacheImports: nil,
	})
	if err != nil {
		return exptypes.ExportResult{}, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return exptypes.ExportResult{}, err
	}

	// Parse the manifest.json file to get the list of layers and the config file
	manifestByte, err := ref.ReadFile(ctx, client.ReadRequest{Filename: "/manifest.json"})
	if err != nil {
		return exptypes.ExportResult{}, fmt.Errorf("nix build did not produce a manifest.json file, please check the build logs for errors: %w", err)
	}
	manifest, err := dockershim.UnmarshalManifest(manifestByte)
	if err != nil {
		return exptypes.ExportResult{}, err
	}

	// Create a new scratch state and overlay the layers from the manifest.json file to it
	layered := llb.Scratch()
	for _, layer := range manifest.Layers {
		layered = layered.File(
			llb.Copy(nixStoreClosure, layer, "/", &llb.CopyInfo{AttemptUnpack: true}),
			nixllb.ShouldIgnoreCache(cfg.IgnoreCache),
			withInternalName(fmt.Sprintf("copying layer '%s'...", layer), cfg.MultiPlatformRequested),
		)
	}

	// Read the config file from the result of the nix build and add it to the result metadata
	configByte, err := ref.ReadFile(ctx, client.ReadRequest{Filename: "/" + manifest.Config})
	if err != nil {
		return exptypes.ExportResult{}, err
	}

	// Parse the oci config file to get the image configuration
	var config dockerocispec.DockerOCIImage
	if err := json.Unmarshal(configByte, &config); err != nil {
		return exptypes.ExportResult{}, err
	}

	result := exptypes.ExportResult{
		State:          layered,
		DockerOCIImage: config,
	}

	return result, nil
}

func withInternalName(name string, multiPlatformRequested bool) llb.ConstraintsOpt {
	if multiPlatformRequested {
		p := platforms.DefaultSpec()
		return nixui.WithInternalNameTag(fmt.Sprintf("builder %s/%s", p.OS, p.Architecture))(name)
	}
	return nixui.WithInternalNameTag("exporter")(name)
}
