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

        # Single source of truth for the release version. Bumped by
        # `make bump-version VERSION=X.Y.Z` before tagging; the release
        # workflow verifies that this matches the pushed tag so the two
        # can't drift. See docs/RELEASE.md for the full flow.
        baseVersion = "0.9.28";
        commit = self.shortRev or self.dirtyShortRev or "unknown";
        version = "${baseVersion}-${commit}";
      in
      {
        packages = {
          default = pkgs.buildGoModule {
            pname = "lfk";
            inherit version;

            src = ./.;

            vendorHash = "sha256-mx5IuJLGtNx2WZUfF/TdubwOGCr0Wjy7s2zvzOXqyO0=";

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
