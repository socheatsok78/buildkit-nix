> [!IMPORTANT]
> This project is still in early development and may not be stable. Use at your own risk

## About
An experimental BuildKit frontend for building Nix derivations and flakes as Dockerfile.

## Usage

In the `flake.nix` add the following snippet to the top of the file:

```nix
# syntax=ghcr.io/socheatsok78/buildkit-nix:experimental
```

Then run the following command to build the flake:

```bash
docker buildx build -t flake -f flake.nix .

# or if you want to build a specific `nix build .#<installable>`,
# you can use the `--target` option:
docker buildx build -t flake -f flake.nix --target <installable> .
```

> [!NOTE]
> The image will be layered depending on the result output of the `nix build` command.
>
> Example:
> - Using `pkgs.dockerTools.buildImage` will produce a single image with a single layer for all files (and dependencies).
> - Using `pkgs.dockerTools.buildLayeredImage` will produce a single image, using multiple layers to improve sharing between images

## Example

See [socheatsok78/buildkit-nix-demo](https://github.com/socheatsok78/buildkit-nix-demo) for a working example of how to use this frontend.

## Roadmap

- [ ] Nix derivations using `pkgs.dockerTools.buildImage` & `pkgs.dockerTools.buildLayeredImage`
- [x] Nix packages using Flakes `nix build .#<package>` syntax
- [x] Multi-platform builds with `docker buildx build` and `docker buildx bake` commands
- [ ] Build cache support for Nix store and Docker layers
- [ ] Named contexts for frontend (unsupported frontend capability moby.buildkit.frontend.contexts)

## License
This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
