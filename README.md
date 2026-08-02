> [!IMPORTANT]
> This project is still in early development and may not be stable. Use at your own risk

## About
An experimental BuildKit frontend for building Nix Flakes as Dockerfile.

## Image

| Registry                    | Image                                 |
| --------------------------- | ------------------------------------- |
| [Docker Hub]                | socheatsok78/nixfile-frontend         |
| [GitHub Container Registry] | ghcr.io/socheatsok78/nixfile-frontend |

## Usage

In the `flake.nix` add the following snippet to the top of the file:

```nix
# syntax=socheatsok78/nixfile-frontend:experimental
```
> [!NOTE]
> The package must be built using either `pkgs.dockerTools.buildImage` or `pkgs.dockerTools.buildLayeredImage` to produce a Docker image.

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

- [x] Nix packages using Flakes `nix build .#<package>` syntax
- [x] Build cache support for Nix store and Docker layers
- [x] Multi-platform builds with `docker buildx build` and `docker buildx bake` commands

## Caveats

- Nix derivations using `pkgs.dockerTools.buildImage` & `pkgs.dockerTools.buildLayeredImage`, there is no plan to support building Nix derivation directly yet. Maybe in the future.

- Named contexts for frontend `(unsupported frontend capability moby.buildkit.frontend.contexts)`, there is no way to reference a named context in the `flake.nix` file, so the implementation is to remove pesky warning and errors about named contexts.

## License
This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
