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

    vendorHash = "sha256-M59BZSFKSWrb6iAnXxFr+iktUcVK2AN208AfBO6XsbE=";

    ldflags = [
      "-s"
      "-w"
    ];
  });

  default = buildkit-nix;
}
