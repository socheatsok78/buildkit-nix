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
    };
  };

  default = buildkit-nix;
}
