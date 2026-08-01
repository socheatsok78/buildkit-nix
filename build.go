package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/moby/buildkit/frontend/gateway/client"
	dockerocispecs "github.com/moby/docker-image-spec/specs-go/v1"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
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
	c = &withResolveCache{Client: c}
	bc, err := dockerui.NewClient(c)
	if err != nil {
		return nil, err
	}
	opts := bc.BuildOpts().Opts

	// also accept build args from Moby
	for k, v := range opts {
		if strings.HasPrefix(k, buildArgPrefix) {
			opts[strings.TrimPrefix(k, buildArgPrefix)] = v
		}
	}

	// Check if a target is specified in the build options, Use it for nix build installables
	target := ""
	prettyTarget := ""
	if v, ok := opts[keyTarget]; ok {
		target = fmt.Sprintf("#%v", v)
		prettyTarget = fmt.Sprintf(".%v", target)
	}

	// Load the source code from the build context
	src := llb.Local(LocalNameContext, llb.SessionID(c.BuildOpts().SessionID), llb.SharedKeyHint("nix-src"), nixui.WithInternalName("load source from build context"))

	// Using the dockerui.Client to enable multi-platform builds
	// The dockerui.Client will handle the platform selection and build execution for us
	rb, err := bc.Build(ctx, func(ctx context.Context, platform *ocispecs.Platform, idx int) (*dockerui.BuildResult, error) {
		withInternalName := nixui.WithInternalName
		if platform != nil {
			withInternalName = nixui.WithInternalNameTag(fmt.Sprintf("%s/%s", platform.OS, platform.Architecture))
		}

		// Load the nix image and set it as the base image for the build
		builderOpts := []llb.ImageOption{
			llb.WithMetaResolver(c),
			withInternalName(fmt.Sprintf("load builder image from %s", NixImage)),
		}
		if platform != nil {
			builderOpts = append(builderOpts, llb.Platform(*platform))
		}
		builder := llb.Image(NixImage, builderOpts...)

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
				"--option", "filter-syscalls", "false",
				"--extra-experimental-features", "nix-command",
				"--extra-experimental-features", "flakes",
				"build",
				"--option", "build-users-group", "",
				fmt.Sprintf("/src%s", target),
			}),
			withInternalName(fmt.Sprintf("nix build %s", prettyTarget)),
		)

		// Extract the result of the nix build to a new scratch state
		extract := llb.Scratch()
		extractSt := extract.File(
			llb.Copy(builderSt.GetMount("/"), "/workspace/result", "/", &llb.CopyInfo{
				AttemptUnpack: true,
			}),
			withInternalName("extracting result layers"),
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
				withInternalName(fmt.Sprintf("importing layer: %s", layer)),
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

		layeredRef, err := layeredRes.SingleRef()
		if err != nil {
			return nil, err
		}

		// Read the config file from the result of the nix build and add it to the result metadata
		configByte, err := extractRef.ReadFile(ctx, client.ReadRequest{Filename: "/" + manifest.Config})
		if err != nil {
			return nil, err
		}

		// Parse the oci config file to get the image configuration
		var config dockerocispecs.DockerOCIImage
		if err := json.Unmarshal(configByte, &config); err != nil {
			return nil, err
		}

		return &dockerui.BuildResult{
			Reference: layeredRef,
			Image:     &config,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	return rb.Finalize()
}
