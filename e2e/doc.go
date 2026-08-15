// Package e2e holds the cluster-backed tests, all behind the `e2e` build tag.
// This file carries no tag so `go test ./e2e` skips instead of failing to
// build, which packagers hit when they expand packages one by one (#627).
package e2e
