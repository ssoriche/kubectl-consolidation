{
  lib,
  buildGoModule,
  go_1_26,
  versionCheckHook,
}:

let
  version = "0.1.0";
in
# Pin the Go toolchain to 1.26 to satisfy the `go 1.26.0` directive in go.mod;
# a sandboxed Nix build cannot download a newer toolchain on demand.
(buildGoModule.override { go = go_1_26; }) {
  pname = "kubectl-consolidation";
  inherit version;

  # Only the files needed to build the binary, so unrelated changes
  # (docs, CI config, the flox env) don't invalidate the build.
  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../cmd
      ../internal
    ];
  };

  vendorHash = "sha256-JyrEXuI9B1dPhhAtzpiz4s7+T4yUaRBH74PKky9Q4JI=";

  subPackages = [ "cmd/kubectl-consolidation" ];

  env.CGO_ENABLED = 0;

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];

  # Sanity-check that the built binary runs and reports its version.
  nativeBuildInputs = [ versionCheckHook ];
  doInstallCheck = true;

  meta = {
    description = "kubectl/krew plugin that shows Karpenter consolidation blockers for nodes";
    longDescription = ''
      A kubectl plugin (distributed via krew as "consolidation") that shows
      nodes with Karpenter consolidation blocker information: nodepool/
      provisioner, capacity type, CPU/memory utilization, and the reasons a
      node cannot be consolidated. It auto-detects the Karpenter API version
      (v1alpha5, v1beta1, v1) and supports mixed-version clusters.

      Because kubectl discovers `kubectl-*` executables on PATH as plugins,
      installing this package makes `kubectl consolidation` available.
    '';
    homepage = "https://github.com/ssoriche/kubectl-consolidation";
    license = lib.licenses.mit;
    mainProgram = "kubectl-consolidation";
    maintainers = [ ];
  };
}
