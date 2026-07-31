package builder

import (
	"context"
	"fmt"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/dockerui"
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
	src := llb.Local(LocalNameContext, llb.SessionID(c.BuildOpts().SessionID), llb.SharedKeyHint("nix-src"))

	// Load the nix image and set it as the base image for the build
	builder := llb.Image(NixImage, llb.WithMetaResolver(c), dockerui.WithInternalName(fmt.Sprintf("load builder image from %s", NixImage)))

	// builder = builder.Security(llb.SecurityModeSandbox)

	// Run the nix build command inside the nix image
	builderSt := builder.Dir("/workspace").Run(
		llb.AddMount("/src", src),
		llb.Args([]string{
			"nix",
			"--option", "sandbox", "true",
			"--extra-experimental-features", "nix-command",
			"--extra-experimental-features", "flakes",
			"build",
			"--option", "build-users-group", "",
			fmt.Sprintf("/src%s", target),
		}),
	)

	// Extract the result of the nix build to a new scratch state
	layered := llb.Scratch()
	layeredSt := layered.File(
		llb.Copy(builderSt.GetMount("/"), "/workspace/result", "/", &llb.CopyInfo{
			AttemptUnpack: true,
		}),
		nixui.WithInternalName("extracting result layers"),
	)

	// Prepare the builder state to extract the manifest.json file
	builderDef, err := layeredSt.Marshal(ctx, llb.WithCaps(c.BuildOpts().LLBCaps))
	if err != nil {
		return nil, err
	}

	builderRes, err := c.Solve(ctx, client.SolveRequest{
		Definition: builderDef.ToPB(),
	})
	if err != nil {
		return nil, err
	}

	builderRes.EachRef(func(r client.Reference) error {
		builderRef := r

		// Parse the manifest.json file to get the list of layers and the config file
		manifestByte, err := builderRef.ReadFile(ctx, client.ReadRequest{Filename: "/manifest.json"})
		if err != nil {
			return err
		}
		manifest, err := dockershim.UnmarshalManifest(manifestByte)
		if err != nil {
			return err
		}

		// Create a new scratch state to hold the extracted layers
		st := llb.Scratch()

		// Import each layer from the result of the nix build into the new scratch state
		for _, layer := range manifest.Layers {
			st = st.File(
				llb.Copy(layeredSt, layer, "/", &llb.CopyInfo{
					AttemptUnpack: true,
				}),
				nixui.WithInternalName(fmt.Sprintf("importing layer: %s", layer)),
			)
		}

		// Prepare the final definition from final state
		def, err := st.Marshal(ctx)
		if err != nil {
			return err
		}
		res, err := c.Solve(ctx, client.SolveRequest{
			Definition: def.ToPB(),
		})
		if err != nil {
			return err
		}

		// Read the config file from the result of the nix build and add it to the result metadata
		configByte, err := builderRef.ReadFile(ctx, client.ReadRequest{Filename: "/" + manifest.Config})
		if err != nil {
			return err
		}

		res.AddMeta(exptypes.ExporterImageConfigKey, configByte)

		return nil
	})

	return builderRes, nil
}
