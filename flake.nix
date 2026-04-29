{
  description = "LFK is a lightning-fast, keyboard-focused, yazi-inspired terminal user interface for navigating and managing Kubernetes clusters. Built for speed and efficiency, it brings a three-column Miller columns layout with an owner-based resource hierarchy to your terminal.";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/master";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        inherit (pkgs) lib;

        # Single source of truth for the release version. Updated automatically
        # by the release-please bot on every Release PR via the marker comment
        # below; release.yml then verifies this matches the pushed tag so the
        # two can't drift. `make bump-version VERSION=X.Y.Z` remains available
        # for emergency manual bumps.
        baseVersion = "0.9.35"; # x-release-please-version
        commit = self.shortRev or self.dirtyShortRev or "unknown";
        version = "${baseVersion}-${commit}";
      in
      {
        packages = {
          default = pkgs.buildGoModule {
            pname = "lfk";
            inherit version;

            src = ./.;

            vendorHash = "sha256-nEMHImlytPq9FhN6Rb5mmBMpZ7d+II1MirD0xLLZv+A=";

            subPackages = [ "." ];

            # Matches the ldflag recipe documented in internal/version/version.go
            # so `lfk --version` reports the flake-built version instead of "dev".
            ldflags = [
              "-s"
              "-w"
              "-X github.com/janosmiko/lfk/internal/version.Version=v${baseVersion}"
              "-X github.com/janosmiko/lfk/internal/version.GitCommit=${commit}"
            ];

            enableParallelBuilding = true;

            meta = {
              description = "LFK is a lightning-fast Kubernetes navigator";
              homepage = "https://github.com/janosmiko/lfk";
              license = lib.licenses.asl20;
              mainProgram = "lfk";
            };
          };
        };

        apps = {
          default = {
            type = "app";
            program = "${self.packages.${system}.default}/bin/lfk";
          };
        };
      }
    );
}
