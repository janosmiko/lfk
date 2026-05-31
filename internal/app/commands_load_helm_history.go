package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

// helmHistoryTimeout caps how long the helm history subprocess may run before
// being killed. Prevents the loader goroutine from leaking when helm hangs on
// an unresponsive API server.
const helmHistoryTimeout = 30 * time.Second

// helmErrOutputCap bounds how much of helm's stderr/stdout we propagate into
// error messages and logs. Helm can emit multi-kilobyte NOTES.txt content and
// values snippets that would otherwise bloat logs and the UI error overlay.
const helmErrOutputCap = 512

// truncateHelmOutput trims helm subprocess output to helmErrOutputCap bytes so
// logs and UI error messages stay bounded.
func truncateHelmOutput(out []byte) string {
	s := strings.TrimSpace(string(out))
	if len(s) > helmErrOutputCap {
		return s[:helmErrOutputCap] + "...(truncated)"
	}
	return s
}

// fetchHelmHistory shells out to `helm history` and returns the parsed
// revisions (newest first). It is shared between the rollback and read-only
// history overlays so both views use the same data source. The subprocess is
// bounded by helmHistoryTimeout and its error output is truncated before
// being logged or propagated to the UI.
func fetchHelmHistory(helmPath, name, ns, kubeCtx, kubeconfigPaths string) ([]ui.HelmRevision, error) {
	ctx, cancel := context.WithTimeout(context.Background(), helmHistoryTimeout)
	defer cancel()

	args := []string{"history", name, "-n", ns, "--kube-context", kubeCtx, "-o", "json", "--max", "50"}
	cmd := exec.CommandContext(ctx, helmPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	logExecCmd("Running helm command", cmd)
	// Capture stdout and stderr separately: helm writes the JSON document to
	// stdout but may also print warnings (kubeconfig perms, deprecations) to
	// stderr on an otherwise-successful run. Merging them (CombinedOutput)
	// would interleave that text into the JSON and break Unmarshal even
	// though helm exited 0.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, cmdErr := cmd.Output()
	if cmdErr != nil {
		// Prefer stderr for the error detail; fall back to stdout when helm
		// wrote the failure there instead.
		detail := stderr.Bytes()
		if len(bytes.TrimSpace(detail)) == 0 {
			detail = output
		}
		truncated := truncateHelmOutput(detail)
		logger.Error("helm history failed", "cmd", cmd.String(), "error", cmdErr, "output", truncated)
		return nil, fmt.Errorf("%w: %s", cmdErr, truncated)
	}

	var entries []struct {
		Revision    int    `json:"revision"`
		Status      string `json:"status"`
		Chart       string `json:"chart"`
		AppVersion  string `json:"app_version"`
		Description string `json:"description"`
		Updated     string `json:"updated"`
	}
	if jsonErr := json.Unmarshal(output, &entries); jsonErr != nil {
		return nil, fmt.Errorf("failed to parse helm history: %w", jsonErr)
	}

	revisions := make([]ui.HelmRevision, len(entries))
	for i, e := range entries {
		revisions[i] = ui.HelmRevision{
			Revision:    e.Revision,
			Status:      e.Status,
			Chart:       e.Chart,
			AppVersion:  e.AppVersion,
			Description: e.Description,
			Updated:     e.Updated,
		}
	}
	// Reverse so newest revision is first.
	for i, j := 0, len(revisions)-1; i < j; i, j = i+1, j-1 {
		revisions[i], revisions[j] = revisions[j], revisions[i]
	}
	return revisions, nil
}

// loadHelmRevisions fetches the revision history for a Helm release.
func (m Model) loadHelmRevisions() tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return helmRevisionListMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPathForContext(ctx)

	return func() tea.Msg {
		revisions, err := fetchHelmHistory(helmPath, name, ns, m.kubectlContext(ctx), kubeconfigPaths)
		if err != nil {
			return helmRevisionListMsg{err: err}
		}
		return helmRevisionListMsg{revisions: revisions}
	}
}

// loadHelmHistory fetches revision history for the read-only history overlay.
// It reuses fetchHelmHistory but routes the result to helmHistoryListMsg so
// the update path opens overlayHelmHistory instead of overlayHelmRollback.
func (m Model) loadHelmHistory() tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return helmHistoryListMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPathForContext(ctx)

	return func() tea.Msg {
		revisions, err := fetchHelmHistory(helmPath, name, ns, m.kubectlContext(ctx), kubeconfigPaths)
		if err != nil {
			return helmHistoryListMsg{err: err}
		}
		return helmHistoryListMsg{revisions: revisions}
	}
}
