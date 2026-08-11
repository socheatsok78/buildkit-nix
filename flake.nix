# syntax=socheatsok78/nixfile-frontend:experimental
{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    maintainers = {
      url = "github:socheatsok78/maintainers.nix";
      flake = false;
    };
  };
  outputs =
    {
      self,
      nixpkgs,
      maintainers,
    }:
    let
      forAllSystems = nixpkgs.lib.genAttrs nixpkgs.lib.systems.flakeExposed;
    in
    {
      legacyPackages = forAllSystems (
        system:
        import ./packages.nix {
          inherit maintainers;
          pkgs = nixpkgs.legacyPackages.${system};
        }
      );

      packages = forAllSystems (
        system: nixpkgs.lib.filterAttrs (_: v: nixpkgs.lib.isDerivation v) self.legacyPackages.${system}
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          # It is recommended to pin Go version to avoid issues with breaking changes in the future.
          # You can uncomment the following lines to pin a specific version of Go and its tools.
          # go = pkgs.go_1_25;
          # gotools = pkgs.gotools.override { go = go; };
        in
        {
          default = pkgs.mkShellNoCC {
            packages = with pkgs; [
              delve
              go
              go-tools
              gopls
              gotools
            ];
          };
        }
      );

      # nix fmt (experimental)
      formatter = forAllSystems (system: nixpkgs.legacyPackages.${system}.nixfmt-tree);
    };
}
