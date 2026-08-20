package ocispec

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	dockerocispec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/socheatsok78/buildkit-nix/nixfile/exporter/plugins"
	"github.com/socheatsok78/buildkit-nix/nixfile/exporter/types"
	"github.com/socheatsok78/buildkit-nix/pkg/dockershim"
	"github.com/socheatsok78/buildkit-nix/pkg/nixllb"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

func init() {
	plugins.RegisterPlugin("ocispec", &OCIExporterPlugin{})
}

var _ plugins.Plugin = &OCIExporterPlugin{}

type OCIExporterPlugin struct{}

func (p *OCIExporterPlugin) Export(ctx context.Context, c client.Client, cfg types.ExportConfig) (types.ExportResult, error) {
	nixStoreClosure := llb.Scratch().File(
		llb.Copy(cfg.State, "/result", "/", &llb.CopyInfo{AttemptUnpack: true}),
		nixllb.ShouldIgnoreCache(cfg.IgnoreCache),
		withInternalNameW("evaluating nix store closure"),
	)

	def, err := nixStoreClosure.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
	if err != nil {
		return types.ExportResult{}, err
	}

	res, err := c.Solve(ctx, client.SolveRequest{
		Definition:   def.ToPB(),
		CacheImports: nil,
	})
	if err != nil {
		return types.ExportResult{}, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return types.ExportResult{}, err
	}

	// Parse the manifest.json file to get the list of layers and the config file
	manifestByte, err := ref.ReadFile(ctx, client.ReadRequest{Filename: "/manifest.json"})
	if err != nil {
		return types.ExportResult{}, fmt.Errorf("nix build did not produce a manifest.json file, please check the build logs for errors: %w", err)
	}
	manifest, err := dockershim.UnmarshalManifest(manifestByte)
	if err != nil {
		return types.ExportResult{}, err
	}

	// Create a new scratch state and overlay the layers from the manifest.json file to it
	layered := llb.Scratch()
	for _, layer := range manifest.Layers {
		layered = layered.File(
			llb.Copy(nixStoreClosure, layer, "/", &llb.CopyInfo{AttemptUnpack: true}),
			nixllb.ShouldIgnoreCache(cfg.IgnoreCache),
			withInternalNameW(fmt.Sprintf("copying layer '%s'...", layer)),
		)
	}

	// Read the config file from the result of the nix build and add it to the result metadata
	configByte, err := ref.ReadFile(ctx, client.ReadRequest{Filename: "/" + manifest.Config})
	if err != nil {
		return types.ExportResult{}, err
	}

	// Parse the oci config file to get the image configuration
	var config dockerocispec.DockerOCIImage
	if err := json.Unmarshal(configByte, &config); err != nil {
		return types.ExportResult{}, err
	}

	result := types.ExportResult{
		State:          layered,
		DockerOCIImage: config,
	}

	return result, nil
}

func withInternalNameW(name string) llb.ConstraintsOpt {
	return nixui.WithInternalNameTag("exporter/ocispec")(name)
}
