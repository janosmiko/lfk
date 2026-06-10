// Package app — logshared.go
// Pure helpers shared by the fullscreen log viewer (m.logView) and the
// right-pane live-log preview (m.previewLog). These hold no state of their own
// so the two surfaces keep independent buffers and scroll models while reusing
// the same argument-building and history-merge logic.
package app

import (
	"fmt"
	"slices"
)

// kubectlPodLogArgs builds the kubectl args to tail all containers of a single
// pod. When follow is true the stream stays open (-f) with the flags needed to
// survive init-container transitions (--max-log-requests, --ignore-errors);
// when false it is a one-shot snapshot used for lazy history fetches. A tail of
// 0 means "no backlog", a positive tail caps the backlog, and a negative tail
// omits --tail entirely.
func kubectlPodLogArgs(name, namespace, kctx string, follow bool, tail int) []string {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, name, "-n", namespace, "--context", kctx, "--all-containers=true", "--prefix")
	if follow {
		args = append(args, "--max-log-requests=20", "--ignore-errors")
	}
	if tail >= 0 {
		args = append(args, fmt.Sprintf("--tail=%d", tail))
	}
	return append(args, "--timestamps")
}

// mergeOlderLogLines computes the genuinely-older lines to prepend to current
// given a freshly-fetched batch (the last N lines of the pod). The current
// buffer's oldest lines appear at the END of the fetched batch, so the overlap
// is found by searching backwards for a 3-line match of current's head (with a
// single-line fallback for short buffers). Returns the prefix of fetched that
// precedes the overlap: empty when there is nothing older, or the whole batch
// when no overlap is found (logs rotated).
func mergeOlderLogLines(current, fetched []string) []string {
	if len(fetched) == 0 {
		return nil
	}

	overlapIdx := -1
	if len(current) >= 3 && len(fetched) > 3 {
		first3 := current[:3]
		for i := len(fetched) - 3; i >= 0; i-- {
			if fetched[i] == first3[0] && fetched[i+1] == first3[1] && fetched[i+2] == first3[2] {
				overlapIdx = i
				break
			}
		}
	} else if len(current) > 0 {
		// Single-line fallback for short buffers.
		for i, line := range slices.Backward(fetched) {
			if line == current[0] {
				overlapIdx = i
				break
			}
		}
	}

	switch {
	case overlapIdx > 0:
		return fetched[:overlapIdx]
	case overlapIdx == -1:
		// No overlap (logs may have rotated) — everything is older.
		return fetched
	default:
		// overlapIdx == 0: first fetched line IS the oldest current line.
		return nil
	}
}
