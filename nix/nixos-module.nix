self:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.programs.kubectl-consolidation;
in
{
  options.programs.kubectl-consolidation = {
    enable = lib.mkEnableOption ''
      kubectl-consolidation, a kubectl/krew plugin that shows Karpenter
      consolidation blockers. Installing it puts the `kubectl-consolidation`
      binary on PATH, so `kubectl consolidation` becomes available system-wide'';

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.kubectl-consolidation;
      defaultText = lib.literalExpression "kubectl-consolidation.packages.\${system}.kubectl-consolidation";
      description = "The kubectl-consolidation package to install.";
    };
  };

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [ cfg.package ];
  };
}
