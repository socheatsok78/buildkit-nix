{
  pkgs ? import <nixpkgs> { },
  maintainers ? import <nixpkgs> { }.lib.maintainers,
}:
rec {
  default = nixfile-frontend;

  nixfile-frontend = pkgs.callPackage ./nixfile-frontend.nix {
    inherit maintainers;
  };

  nixfile-frontend-image = pkgs.callPackage ./nixfile-frontend-image.nix {
    inherit maintainers nixfile-frontend;
  };
}
