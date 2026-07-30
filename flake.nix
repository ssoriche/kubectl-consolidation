{
  description = "kubectl-consolidation: show Karpenter consolidation blockers for nodes";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs =
    { self, nixpkgs }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems =
        f: nixpkgs.lib.genAttrs supportedSystems (system: f system nixpkgs.legacyPackages.${system});
    in
    {
      # Overlay that adds `kubectl-consolidation` to a nixpkgs instance,
      # for consumers who prefer composing it into their own package set.
      overlays.default = final: _prev: {
        kubectl-consolidation = final.callPackage ./nix/package.nix { };
      };

      # `nix build`, `nix build .#kubectl-consolidation`, and the input for
      # the NixOS / home-manager modules below.
      packages = forAllSystems (
        _system: pkgs: rec {
          kubectl-consolidation = pkgs.callPackage ./nix/package.nix { };
          default = kubectl-consolidation;
        }
      );

      # `nix run` -> `kubectl-consolidation` (also usable as `kubectl consolidation`).
      apps = forAllSystems (
        system: _pkgs: rec {
          kubectl-consolidation = {
            type = "app";
            program = nixpkgs.lib.getExe self.packages.${system}.kubectl-consolidation;
            meta.description = "Show Karpenter consolidation blockers for nodes";
          };
          default = kubectl-consolidation;
        }
      );

      # Matches the flox dev environment: Go 1.26 plus the release tooling.
      devShells = forAllSystems (
        _system: pkgs: {
          default = pkgs.mkShellNoCC {
            packages = [
              pkgs.go_1_26
              pkgs.golangci-lint
              pkgs.goreleaser
              pkgs.kubectl
            ];
          };
        }
      );

      formatter = forAllSystems (_system: pkgs: pkgs.nixfmt);

      # Declarative installation of the krew/kubectl plugin on NixOS or via
      # home-manager. Enabling either puts `kubectl-consolidation` on PATH,
      # which is all kubectl needs to expose `kubectl consolidation`.
      nixosModules.default = import ./nix/nixos-module.nix self;
      homeManagerModules.default = import ./nix/home-manager-module.nix self;
    };
}
