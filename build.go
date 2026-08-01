package builder

import (
	"context"
	"fmt"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/socheatsok78/buildkit-nix/pkg/dockershim"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

const (
	NixImage         = "docker.io/nixos/nix:latest"
	LocalNameContext = "context"

	buildArgPrefix = "build-arg:"
	keyTarget      = "target"
)

func Build(ctx context.Context, c client.Client) (*client.Result, error) {
	opts := c.BuildOpts().Opts

	// also accept build args from Moby
	for k, v := range opts {
		if strings.HasPrefix(k, buildArgPrefix) {
			opts[strings.TrimPrefix(k, buildArgPrefix)] = v
		}
	}

	// Check if a target is specified in the build options, Use it for nix build installables
	target := ""
	if v, ok := opts[keyTarget]; ok {
		target = fmt.Sprintf("#%v", v)
	}

	// Load the source code from the build context
	src := llb.Local(LocalNameContext, llb.SessionID(c.BuildOpts().SessionID), llb.SharedKeyHint("nix-src"), nixui.WithInternalName("load source from build context"))

	// Load the nix image and set it as the base image for the build
	builder := llb.Image(NixImage, llb.WithMetaResolver(c), nixui.WithInternalName(fmt.Sprintf("load builder image from %s", NixImage)))

	// Configure builder
	builder = builder.With(
		llb.Dir("/workspace"),
	)

	// Run the nix build command inside the nix image
	builderSt := builder.Run(
		llb.AddMount("/src", src, llb.Readonly),
		llb.AddMount("/build", llb.Scratch()),
		llb.Args([]string{
			"nix",
			"--option", "sandbox", "false",
			"--extra-experimental-features", "nix-command",
			"--extra-experimental-features", "flakes",
			"build",
			"--option", "build-users-group", "",
			fmt.Sprintf("/src%s", target),
		}),
	)

	// Extract the result of the nix build to a new scratch state
	extract := llb.Scratch()
	extractSt := extract.File(
		llb.Copy(builderSt.GetMount("/"), "/workspace/result", "/", &llb.CopyInfo{
			AttemptUnpack: true,
		}),
		nixui.WithInternalName("extracting result layers"),
	)

	// Prepare the builder state to extract the manifest.json file
	extractDef, err := extractSt.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
	if err != nil {
		return nil, err
	}
	extractRes, err := c.Solve(ctx, client.SolveRequest{
		Definition: extractDef.ToPB(),
	})
	if err != nil {
		return nil, err
	}
	extractRef, err := extractRes.SingleRef()
	if err != nil {
		return nil, err
	}

	// Parse the manifest.json file to get the list of layers and the config file
	manifestByte, err := extractRef.ReadFile(ctx, client.ReadRequest{Filename: "/manifest.json"})
	if err != nil {
		return nil, err
	}
	manifest, err := dockershim.UnmarshalManifest(manifestByte)
	if err != nil {
		return nil, err
	}

	// Create a new scratch state to hold the extracted layers
	layered := llb.Scratch()

	// Import each layer from the result of the nix build into the new scratch state
	for _, layer := range manifest.Layers {
		layered = layered.File(
			llb.Copy(extractSt, layer, "/", &llb.CopyInfo{
				AttemptUnpack: true,
			}),
			nixui.WithInternalName(fmt.Sprintf("importing layer: %s", layer)),
		)
	}

	// Prepare the final definition from final state
	layeredDef, err := layered.Marshal(ctx)
	if err != nil {
		return nil, err
	}
	layeredRes, err := c.Solve(ctx, client.SolveRequest{
		Definition: layeredDef.ToPB(),
	})
	if err != nil {
		return nil, err
	}

	// Read the config file from the result of the nix build and add it to the result metadata
	configByte, err := extractRef.ReadFile(ctx, client.ReadRequest{Filename: "/" + manifest.Config})
	if err != nil {
		return nil, err
	}
	layeredRes.AddMeta(exptypes.ExporterImageConfigKey, configByte)

	return layeredRes, nil
}
