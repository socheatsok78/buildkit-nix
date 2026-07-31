## About
An experimental BuildKit frontend for building Nix derivations and flakes as Dockerfile.

## Usage

In the `flake.nix` add the following snippet to the top of the file:

```nix
# syntax=ghcr.io/socheatsok78/buildkit-nix:dev
```

Then run the following command to build the flake:

```bash
export DOCKER_BUILDKIT=1
docker build -t nginx-nix -f flake.nix .
```
