package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/logger"
)

// fieldDocDebounce is how long the cursor must rest before the schema is asked
// about the field under it. One kubectl explain costs a process spawn and a
// round trip, so a key held down must not start one per line.
const fieldDocDebounce = 250 * time.Millisecond

// fieldDocKeyForPath addresses the field under the cursor in the schema of the
// resource the viewer is showing. It reports false when the navigation state
// carries no resource type, which leaves nothing to ask the schema about.
func (m Model) fieldDocKeyForPath(objPath []string) (fieldDocKey, bool) {
	rt := m.nav.ResourceType
	resource, apiVersion := buildExplainResourceFromType(rt)
	if resource == "" && rt.Kind != "" {
		resource = strings.ToLower(rt.Kind) + "s"
	}
	if resource == "" {
		return fieldDocKey{}, false
	}
	return fieldDocKey{
		context:    m.effectiveContext(),
		apiVersion: apiVersion,
		resource:   resource,
		path:       fieldDocPath(objPath),
	}, true
}

// scheduleFieldDocFetch waits out the debounce and then reports back with the
// request number, which updateFieldDocDebounce checks before it spends a fetch.
func scheduleFieldDocFetch(req uint64) tea.Cmd {
	return tea.Tick(fieldDocDebounce, func(time.Time) tea.Msg {
		return fieldDocDebounceMsg{req: req}
	})
}

// execKubectlExplainField reads one field description from the schema of the
// connected cluster. It shells out to kubectl explain rather than reading a
// bundled copy, so the text matches the cluster version and covers CRDs.
func (m Model) execKubectlExplainField(req uint64, key fieldDocKey) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return fieldDocLoadedMsg{req: req, key: key, err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	kubeconfigPaths := m.client.KubeconfigPathForContext(key.context)
	target := key.resource
	if key.path != "" {
		target = key.resource + "." + key.path
	}
	kubectlContext := m.kubectlContext(key.context)

	return m.trackBgTask(scheduler.KindSubprocess, "Field doc: "+target, key.context, func() tea.Msg {
		args := []string{"explain", target, "--context", kubectlContext}
		if key.apiVersion != "" {
			args = append(args, "--api-version", key.apiVersion)
		}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logExecCmd("Running kubectl command", cmd)
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			logger.Error("kubectl explain failed", "cmd", cmd.String(), "error", cmdErr, "output", string(output))
			return fieldDocLoadedMsg{
				req: req, key: key,
				err: fmt.Errorf("%w: %s", cmdErr, strings.TrimSpace(string(output))),
			}
		}

		desc, _ := parseExplainOutput(string(output), key.path)
		_, fieldType := parseExplainFieldHeader(string(output))
		return fieldDocLoadedMsg{
			req: req, key: key,
			entry: fieldDocEntry{fieldType: fieldType, desc: desc},
		}
	})
}
