// Package version holds build-time version information for lfk.
// These variables are intended to be set via -ldflags at build time, e.g.:
//
//	go build -ldflags "-X github.com/janosmiko/lfk/internal/version.Version=v1.0.0 \
//	  -X github.com/janosmiko/lfk/internal/version.GitCommit=$(git rev-parse --short HEAD) \
//	  -X github.com/janosmiko/lfk/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package version

import "fmt"

var (
	// Version is the semantic version of the binary (e.g., "v1.2.3").
	Version = "dev"

	// GitCommit is the short SHA of the git commit used to build the binary.
	GitCommit = "unknown"

	// BuildDate is the UTC date/time when the binary was built.
	BuildDate = "unknown"
)

// Full returns a human-readable multi-line version string.
func Full() string {
	return fmt.Sprintf("lfk %s (commit: %s, built: %s)", Version, GitCommit, BuildDate)
}

// Short returns just the version tag (e.g., "dev" or "v1.2.3").
func Short() string {
	return Version
}
