{
  pkgs ? import <nixpkgs> { },
}:
rec {
  nixfile-frontend = pkgs.buildGoModule (finalAttrs: {
    pname = "nixfile-frontend";
    version = "experimental";

    src = ./.;

    subPackages = [
      "cmd/nixfile-frontend"
    ];

    vendorHash = "sha256-4doFp5UonyTpQlpMewEbaJXy9m94NAlQDVTH7YnBpvA=";

    ldflags = [
      "-s"
      "-w"
    ];
  });

  nixfile-frontend-image = pkgs.dockerTools.buildLayeredImage {
    name = "nixfile-frontend";
    tag = "experimental";
    contents = [ nixfile-frontend ];
    config = {
      Cmd = [ "${nixfile-frontend}/bin/nixfile-frontend" ];
      Labels = {
        "moby.buildkit.frontend.network.none" = "true";

        # nixfile-frontend isn't technically support these capabilities,
        # This is a workaround for the following error:
        # - buildx bake failed with: ERROR: current frontend does not support defining additional contexts for targets.
        #   Named contexts are supported since Dockerfile v1.4. Use #syntax directive in Dockerfile or update to latest BuildKit.
        "moby.buildkit.frontend.caps" = "moby.buildkit.frontend.inputs,moby.buildkit.frontend.contexts,moby.buildkit.frontend.gitquerystring";
      };
    };
  };

  default = nixfile-frontend;
}
