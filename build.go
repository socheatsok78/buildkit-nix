package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/moby/buildkit/frontend/gateway/client"
	gwpb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver/errdefs"
	"github.com/moby/buildkit/solver/pb"
	dockerocispecs "github.com/moby/docker-image-spec/specs-go/v1"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/socheatsok78/buildkit-nix/pkg/dockershim"
	"github.com/socheatsok78/buildkit-nix/pkg/nixui"
)

const (
	NixImage         = "docker.io/nixos/nix:latest"
	LocalNameContext = "context"

	buildArgPrefix = "build-arg:"
	keyTarget      = "target"
)

const (
	// Don't forget to update frontend documentation if you add
	// a new build-arg: frontend/dockerfile/docs/reference.md
	keySyntaxArg = "build-arg:BUILDKIT_SYNTAX"
)

const (
	sourceDir     = "/mnt/source"
	dockerfileDir = "/mnt/dockerfile"
	workspaceDir  = "/mnt/workspace"
)

func Build(ctx context.Context, c client.Client) (*client.Result, error) {
	c = &withResolveCache{Client: c}
	bc, err := dockerui.NewClient(c)
	if err != nil {
		return nil, err
	}
	opts := bc.BuildOpts().Opts
	allowForward, capsError := validateCaps(opts["frontend.caps"])
	if !allowForward && capsError != nil {
		return nil, capsError
	}

	// Read the flake.nix as the entrypoint for the build
	// The build process doesn't technically use this layer as build source but the local://context instead
	dockerfile, err := bc.ReadEntrypoint(ctx, "nix")
	if err != nil {
		return nil, err
	}

	if _, ok := opts["cmdline"]; !ok {
		if cmdline, ok := opts[keySyntaxArg]; ok {
			p := strings.SplitN(strings.TrimSpace(cmdline), " ", 2)
			res, err := forwardGateway(ctx, c, p[0], cmdline)
			if err != nil && len(errdefs.Sources(err)) == 0 {
				return nil, errors.Wrapf(err, "failed with %s = %s", keySyntaxArg, cmdline)
			}
			return res, err
		} else if ref, cmdline, loc, ok := parser.DetectSyntax(dockerfile.Data); ok {
			res, err := forwardGateway(ctx, c, ref, cmdline)
			if err != nil && len(errdefs.Sources(err)) == 0 {
				return nil, wrapSource(err, dockerfile.SourceMap, loc)
			}
			return res, err
		}
	}

	if capsError != nil {
		return nil, capsError
	}

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
	src := llb.Local(LocalNameContext, llb.SessionID(c.BuildOpts().SessionID), llb.SharedKeyHint("nix-src"), dockerui.WithInternalName("load source from build context"))

	// Nix Store cache key
	nixStoreCacheKey := "nix-store-cache"

	// Using the dockerui.Client to enable multi-platform builds
	// The dockerui.Client will handle the platform selection and build execution for us
	rb, err := bc.Build(ctx, func(ctx context.Context, platform *ocispecs.Platform, idx int) (*dockerui.BuildResult, error) {
		withInternalName := nixui.WithInternalName
		if platform != nil {
			nixStoreCacheKey = fmt.Sprintf("nix-store-cache-%s-%s", platform.OS, platform.Architecture)
			withInternalName = nixui.WithInternalNameTag(fmt.Sprintf("%s/%s", platform.OS, platform.Architecture))
		}

		// Nix store is used to persist the nix store between builds, so that we don't have to rebuild everything from scratch every time
		nixStore := llb.Scratch()

		// Load the nix image and set it as the base image for the build
		builderImageOpts := []llb.ImageOption{
			llb.WithMetaResolver(c),
			withInternalName(fmt.Sprintf("load builder image from %s", NixImage)),
		}
		if platform != nil {
			builderImageOpts = append(builderImageOpts, llb.Platform(*platform))
		}
		if bc.IsNoCache("") {
			builderImageOpts = append(builderImageOpts, llb.IgnoreCache)
		}
		builder := llb.Image(NixImage, builderImageOpts...)

		// Builder working directory is set to /mnt/workspace,
		// so that the nix build result can be copied to /mnt/workspace/result and extracted later
		builder = builder.With(llb.Dir(workspaceDir))

		// restore nix store cache
		nixStoreRestoreOpts := []llb.RunOption{
			llb.AddEnv("BUILDKIT_NIX_SESSIONID", c.BuildOpts().SessionID),
			llb.AddMount("/mnt/nix", nixStore, llb.AsPersistentCacheDir(nixStoreCacheKey, llb.CacheMountLocked)),
			llb.Shlex("cp -anfT /mnt/nix /nix"),
			withInternalName("configure nix store"),
		}
		if bc.IsNoCache("nix-store") {
			nixStoreRestoreOpts = append(nixStoreRestoreOpts, llb.IgnoreCache)
		}
		builderSt := builder.Run(nixStoreRestoreOpts...)

		// Nix build
		nixBuildOpts := []llb.RunOption{
			llb.AddMount(dockerfileDir, *dockerfile.State, llb.Readonly),
			llb.AddMount(sourceDir, src, llb.Readonly),
			llb.AddMount("/build", llb.Scratch()),
			llb.Args([]string{
				"nix",
				"--option", "sandbox", "false",
				"--option", "filter-syscalls", "false",
				"--extra-experimental-features", "nix-command",
				"--extra-experimental-features", "flakes",
				"--show-trace",
				"--log-format", "raw",
				"build",
				fmt.Sprintf("%s%s", sourceDir, target),
			}),
			withInternalName(fmt.Sprintf("nix build %s", prettyTarget)),
		}
		if bc.IsNoCache("nix-build") {
			nixBuildOpts = append(nixBuildOpts, llb.IgnoreCache)
		}
		builderSt = builderSt.Run(nixBuildOpts...)

		// save nix store cache
		nixStoreSaveOpts := []llb.RunOption{
			llb.AddEnv("BUILDKIT_NIX_SESSIONID", c.BuildOpts().SessionID),
			llb.AddMount("/mnt/nix", nixStore, llb.AsPersistentCacheDir(nixStoreCacheKey, llb.CacheMountLocked)),
			llb.Shlex("cp -anfT /nix /mnt/nix"),
			withInternalName("create nix store snapshot"),
		}
		if bc.IsNoCache("nix-store") {
			nixStoreSaveOpts = append(nixStoreSaveOpts, llb.IgnoreCache)
		}
		builderSt = builderSt.Run(nixStoreSaveOpts...)

		// Extract the result of the nix build to a new scratch state
		extract := llb.Scratch()
		extractFileOpts := []llb.ConstraintsOpt{
			withInternalName("extracting result layers"),
		}
		if bc.IsNoCache("extract") {
			extractFileOpts = append(extractFileOpts, llb.IgnoreCache)
		}
		extractSt := extract.File(
			llb.Copy(
				builderSt.GetMount("/"),
				fmt.Sprintf("%s/result", workspaceDir), "/", &llb.CopyInfo{
					AttemptUnpack: true,
				},
			),
			extractFileOpts...,
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

		// Create a new scratch state and overlay the layers from the manifest.json file to it
		layered := llb.Scratch()
		for _, layer := range manifest.Layers {
			layeredFileOpts := []llb.ConstraintsOpt{
				withInternalName(fmt.Sprintf("importing layer: %s", layer)),
			}
			if bc.IsNoCache("layered") {
				layeredFileOpts = append(layeredFileOpts, llb.IgnoreCache)
			}
			layered = layered.File(
				llb.Copy(extractSt, layer, "/", &llb.CopyInfo{
					AttemptUnpack: true,
				}),
				layeredFileOpts...,
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

		// Return the final result with the layered reference and the image configuration
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

func forwardGateway(ctx context.Context, c client.Client, ref string, cmdline string) (*client.Result, error) {
	opts := c.BuildOpts().Opts
	if opts == nil {
		opts = map[string]string{}
	}
	opts["cmdline"] = cmdline
	opts["source"] = ref

	gwcaps := c.BuildOpts().Caps
	var frontendInputs map[string]*pb.Definition
	if (&gwcaps).Supports(gwpb.CapFrontendInputs) == nil {
		inputs, err := c.Inputs(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get frontend inputs")
		}

		frontendInputs = make(map[string]*pb.Definition)
		for name, state := range inputs {
			def, err := state.Marshal(ctx)
			if err != nil {
				return nil, err
			}
			frontendInputs[name] = def.ToPB()
		}
	}

	return c.Solve(ctx, client.SolveRequest{
		Frontend:       "gateway.v0",
		FrontendOpt:    opts,
		FrontendInputs: frontendInputs,
	})
}

func warnOpts(r []parser.Range, detail [][]byte, url string) client.WarnOpts {
	opts := client.WarnOpts{Level: 1, Detail: detail, URL: url}
	if r == nil {
		return opts
	}
	opts.Range = []*pb.Range{}
	for _, r := range r {
		opts.Range = append(opts.Range, &pb.Range{
			Start: &pb.Position{
				Line:      int32(r.Start.Line),
				Character: int32(r.Start.Character),
			},
			End: &pb.Position{
				Line:      int32(r.End.Line),
				Character: int32(r.End.Character),
			},
		})
	}
	return opts
}

func wrapSource(err error, sm *llb.SourceMap, ranges []parser.Range) error {
	if sm == nil {
		return err
	}
	s := &errdefs.Source{
		Info: &pb.SourceInfo{
			Data:       sm.Data,
			Filename:   sm.Filename,
			Language:   sm.Language,
			Definition: sm.Definition.ToPB(),
		},
		Ranges: make([]*pb.Range, 0, len(ranges)),
	}
	for _, r := range ranges {
		s.Ranges = append(s.Ranges, &pb.Range{
			Start: &pb.Position{
				Line:      int32(r.Start.Line),
				Character: int32(r.Start.Character),
			},
			End: &pb.Position{
				Line:      int32(r.End.Line),
				Character: int32(r.End.Character),
			},
		})
	}
	return errdefs.WithSource(err, s)
}
