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

        # go.mod requires 1.26.3 (security). nixpkgs/master currently ships
        # go_1_26 = 1.26.2; override the source to the official 1.26.3 tarball
        # until nixpkgs catches up, then delete this block.
        go_1_26_3 = pkgs.go_1_26.overrideAttrs (_: rec {
          version = "1.26.3";
          src = pkgs.fetchurl {
            url = "https://go.dev/dl/go${version}.src.tar.gz";
            hash = "sha256-HGRoddCqh5kTMYTtV895/yS97+jIggRwYCqdPW2Rkrg=";
          };
        });

        # Single source of truth for the release version. Updated automatically
        # by the release-please bot on every Release PR via the marker comment
        # below; release.yml then verifies this matches the pushed tag so the
        # two can't drift. `make bump-version VERSION=X.Y.Z` remains available
        # for emergency manual bumps.
        baseVersion = "0.12.0"; # x-release-please-version
        commit = self.shortRev or self.dirtyShortRev or "unknown";
        version = "${baseVersion}-${commit}";
      in
      {
        packages = {
          default = (pkgs.buildGoModule.override { go = go_1_26_3; }) {
            pname = "lfk";
            inherit version;

            src = ./.;

            vendorHash = "sha256-zUfHbY8zyQxKOuruwi0G6J+d5o3ihU96Hg1OqPRtB9g=";

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
