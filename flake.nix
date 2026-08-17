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

        # go.mod requires 1.26.6 (security). nixpkgs/master still ships an
        # older go_1_26, so point the source at the official 1.26.6 tarball.
        # Delete this block once nixpkgs catches up.
        go_1_26_6 = pkgs.go_1_26.overrideAttrs (_: rec {
          version = "1.26.6";
          src = pkgs.fetchurl {
            url = "https://go.dev/dl/go${version}.src.tar.gz";
            hash = "sha256-oHIcVMaIkBRI13rZs+x+p8R0cwdV/4kTgukuy5P/LLE=";
          };
        });

        # Single source of truth for the release version. Updated automatically
        # by the release-please bot on every Release PR via the marker comment
        # below; release.yml then verifies this matches the pushed tag so the
        # two can't drift. `make bump-version VERSION=X.Y.Z` remains available
        # for emergency manual bumps.
        baseVersion = "0.17.3"; # x-release-please-version
        commit = self.shortRev or self.dirtyShortRev or "unknown";
        version = "${baseVersion}-${commit}";
      in
      {
        packages = {
          default = (pkgs.buildGoModule.override { go = go_1_26_6; }) {
            pname = "lfk";
            inherit version;

            src = ./.;

            vendorHash = "sha256-wmM3qWzNnb4zis5JhZNd2iXV6gzy1dMagADYh/hzKuc=";

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
