# syntax=ghcr.io/socheatsok78/nixfile-frontend:experimental
{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
  };
  outputs =
    {
      self,
      nixpkgs,
    }:
    let
      forAllSystems = nixpkgs.lib.genAttrs nixpkgs.lib.systems.flakeExposed;
    in
    {
      legacyPackages = forAllSystems (
        system:
        import ./default.nix {
          pkgs = import nixpkgs { inherit system; };
        }
      );

      packages = forAllSystems (
        system:
        import ./default.nix {
          pkgs = import nixpkgs { inherit system; };
        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
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
