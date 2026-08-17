## About
An experimental [BuildKit] frontend for building Nix Flakes as Dockerfile.

[Source] | [Docker Hub] | [GitHub Container Registry]

> [!IMPORTANT]
> This project is still in early development.
>
> The frontend is designed to be used with BuildKit and Docker Buildx, which allows for building Nix Flakes as Docker images. It supports multi-platform builds, build cache for Nix store and Docker layers, and customization through build arguments and secrets.

## Why?

The main goal of this project is to simplify the process of building Docker images from Nix Flakes, leveraging the power of BuildKit and Docker Buildx.

There are several other implementations that leverage BuildKit but most often they are either not maintained or re-implementing the wheel which further complicates the adoption.

This frontend is designed to be simple, easy to use, and maintainable. Instead of re-implementing the wheel, it leverages the existing Nix tools and commands to build the Flakes and produce Docker images.

## Image

[![Docker](https://github.com/socheatsok78/buildkit-nix/actions/workflows/docker.yml/badge.svg)](https://github.com/socheatsok78/buildkit-nix/actions/workflows/docker.yml)

| Registry                    | Image                                 |
| --------------------------- | ------------------------------------- |
| [Docker Hub]                | socheatsok78/nixfile-frontend         |
| [GitHub Container Registry] | ghcr.io/socheatsok78/nixfile-frontend |

## Usage

In the `flake.nix` add the following snippet to the top of the file:

```nix
# syntax=socheatsok78/nixfile-frontend:experimental
```

Example:

```nix
# syntax=socheatsok78/nixfile-frontend:experimental
{
  description = "A very basic flake";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs = inputs: {
    packages = builtins.mapAttrs (system: pkgs: {
      hello = pkgs.hello;

      default = inputs.self.packages.${system}.hello;
    }) inputs.nixpkgs.legacyPackages;
  };
}
```

Then run the following command to build the flake:

```bash
# Build the default package in the flake
docker buildx build -t flake -f flake.nix .

# or if you want to build a specific `nix build .#<installable>`,
# you can use the `--target` option:
docker buildx build -t flake -f flake.nix --target <installable> .
```

> [!NOTE]
> The image will be layered depending on the result output of the `nix build` command.
>
> If the package is a standard `derivation`, the image will produce a single layer with all the files in the result output and the Nix store dependencies.
>
> If the package is a `dockerTools.buildImage` or `dockerTools.buildLayeredImage`, the image will produce multiple layers depending on the result output and the Nix store dependencies.
>
> See https://nix.dev/tutorials/nixos/building-and-running-docker-images.html and https://nixos.org/manual/nixpkgs/stable/#sec-pkgs-dockerTools.

## Customization

The frontend can be customized using the following build arguments, which can be passed to the `docker buildx build` command using the `--build-arg` option:

- `nix.conf.substituters`: (default: `""`) A list of URLs of Nix stores to be used as substituters, separated by whitespace. A substituter is an additional store from which Nix can obtain store objects instead of building them.  
  See https://nix.dev/manual/nix/2.24/command-ref/conf-file#conf-substituters

- `nix.conf.trusted-public-keys`: (default: `""`) A whitespace-separated list of public keys.  
  See https://nix.dev/manual/nix/2.24/command-ref/conf-file#conf-trusted-public-keys
  
- `nix.conf.trusted-substituters`: (default: `""`) A list of Nix store URLs, separated by whitespace.
  See https://nix.dev/manual/nix/2.24/command-ref/conf-file#conf-trusted-substituters

**Advanced Options**:

- `image`: The Nix image to use for the builder. (default: `docker.io/nixos/nix:latest`)

- `security.insecure`: (default: `false`), The default security mode is sandbox. With `security.insecure=true`, the builder runs the command without sandbox in insecure mode, which allows to run flows requiring elevated privileges.  
  See https://docs.docker.com/reference/dockerfile/#run---security


### Build Secrets

You can provide secrets to the frontend using the `--secret` option of `docker buildx build`. The following secrets are supported:
- `nix.secret.access-tokens`: A file containing access tokens used to access protected GitHub, GitLab, or other locations requiring token-based authentication.  
    See https://nix.dev/manual/nix/2.24/command-ref/conf-file#conf-access-tokens.

- `nix.secret.impure-env`: A file containing environment variables that are considered impure and should be passed to the Nix build process.  
    See https://nix.dev/manual/nix/2.24/command-ref/conf-file#conf-impure-env.

- `nix.secret.netrc-file`: If set to an absolute path to a netrc file, Nix will use the HTTP authentication credentials in this file when trying to download from a remote host through HTTP or HTTPS.  
    See https://nix.dev/manual/nix/2.24/command-ref/conf-file#conf-netrc-file.

> [!NOTE]
> The build secrets are on-demand and optional, so you can choose to provide them or not. During the `nix build` phase, the frontend will check if the secrets are provided and pass them to the `nix build` command as `--option` arguments. If the secrets are not provided, the frontend will skip them and continue with the build.


## Example

See [socheatsok78/buildkit-nix-demo](https://github.com/socheatsok78/buildkit-nix-demo) for a working example of how to use this frontend.

## Roadmap

- [x] Nix packages using Flakes `nix build .#<package>` syntax
- [x] Build cache support for Nix store and Docker layers
- [x] Multi-platform builds with `docker buildx build` and `docker buildx bake` commands

## Caveats

> [!CAUTION]
> Not all packages can be built with this frontend, some may fail due to various reasons.

> [!CAUTION]
> **Privileged Build**
> 
> The privileged build is required for some packages that require access to the host system, such as `dockerTools.buildImage` or `dockerTools.buildLayeredImage`. If you are building a package that requires privileged access.
>
> To enable privileged build, you'll need to add the following build argument to the `docker buildx build` command:
> ```bash
> --build-arg "security.insecure=true" --allow "security.insecure"
> ```
> or, if you are using `docker buildx bake`, add the following to the `docker-bake.hcl` file:
> ```hcl
> targets "<target_name>" {
>   args = {
>     "security.insecure" = "true";
>   }
>   entitlements = [
>     "security.insecure"
>   ]
> }

- Building `.nix` file is currently not supported, only `flake.nix` is supported.

- Named contexts for frontend `(unsupported frontend capability moby.buildkit.frontend.contexts)`, there is no way to reference a named context in the `flake.nix` file, so the implementation is to remove pesky warning and errors about named contexts.

## License
This project is licensed under the Apache License 2.0 - see the [LICENSE] file for details.

[Source]: https://github.com/socheatsok78/buildkit-nix
[LICENSE]: https://github.com/socheatsok78/buildkit-nix/blob/main/LICENSE
[BuildKit]: https://github.com/moby/buildkit
[Docker Hub]: https://hub.docker.com/r/socheatsok78/nixfile-frontend
[GitHub Container Registry]: https://github.com/socheatsok78/nixfile-frontend/pkgs/container/nixfile-frontend
