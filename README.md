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

## Example

See [socheatsok78/buildkit-nix-demo](https://github.com/socheatsok78/buildkit-nix-demo) for a working example of how to use this frontend.

## License
This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
