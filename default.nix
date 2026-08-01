{
  pkgs ? import <nixpkgs> { },
}:
rec {
  buildkit-nix = pkgs.buildGoModule (finalAttrs: {
    pname = "buildkit-nix";
    version = "experimental";

    src = ./.;

    subPackages = [
      "cmd/nix-frontend"
    ];

    vendorHash = "sha256-4doFp5UonyTpQlpMewEbaJXy9m94NAlQDVTH7YnBpvA=";

    ldflags = [
      "-s"
      "-w"
    ];
  });

  buildkit-nix-image = pkgs.dockerTools.buildLayeredImage {
    name = "buildkit-nix";
    tag = "experimental";
    contents = [ buildkit-nix ];
    config = {
      Cmd = [ "${buildkit-nix}/bin/nix-frontend" ];
      Labels = {
        "moby.buildkit.frontend.network.none" = "true";

        # nix-frontend isn't technically support these capabilities,
        # This is a workaround for the following error:
        # - buildx bake failed with: ERROR: current frontend does not support defining additional contexts for targets.
        #   Named contexts are supported since Dockerfile v1.4. Use #syntax directive in Dockerfile or update to latest BuildKit.
        "moby.buildkit.frontend.caps" = "moby.buildkit.frontend.inputs,moby.buildkit.frontend.contexts";
      };
    };
  };

  default = buildkit-nix;
}
