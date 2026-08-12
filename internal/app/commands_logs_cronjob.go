package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
)

// cronJobNoRunsSentinel is a marker line updateLogLine turns into a status
// message instead of a log line - kubectl has no selector for *v1.CronJob.
const cronJobNoRunsSentinel = "\x00lfk:cronjob-no-runs\x00"

// cronJobErrorSentinel carries a real kubectl failure (RBAC, expired creds,
// apiserver outage) the same way - it must never collapse into "no runs yet".
const cronJobErrorSentinel = "\x00lfk:cronjob-error\x00"

var errCronJobNoRuns = errors.New("no runs yet")

var errCronJobUIDMissing = errors.New("cronjob uid missing")

// runKubectlJSON runs cmd and returns its stdout, folding stderr into the
// error so an RBAC denial or apiserver message reaches the user instead of
// the bare "exit status 1" exec.Output leaves behind.
func runKubectlJSON(cmd *exec.Cmd) ([]byte, error) {
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return nil, err
}

// cronJobSentinelLine picks the marker updateLogLine turns into a status
// message, distinguishing a genuine empty result from a real kubectl failure.
func cronJobSentinelLine(cronJobName string, err error) string {
	if errors.Is(err, errCronJobNoRuns) {
		return cronJobNoRunsSentinel + cronJobName
	}
	return cronJobErrorSentinel + err.Error()
}

// resolveCronJobPodSelector returns errCronJobNoRuns only for a genuine empty
// result. Any other error is a real kubectl failure, not "no runs yet".
func resolveCronJobPodSelector(kubectlPath, kubeconfigPaths, ns, name, kctx string) (selector string, err error) {
	jobName, jobUID, err := latestOwnedJob(kubectlPath, kubeconfigPaths, ns, name, kctx)
	if err != nil {
		return "", err
	}
	// Prefixed label first, then the unprefixed pair older clusters still use.
	for _, candidate := range []string{
		"batch.kubernetes.io/job-name=" + jobName + ",batch.kubernetes.io/controller-uid=" + jobUID,
		"job-name=" + jobName + ",controller-uid=" + jobUID,
	} {
		has, err := podSelectorHasPods(kubectlPath, kubeconfigPaths, ns, kctx, candidate)
		if err != nil {
			return "", fmt.Errorf("check pods for %s: %w", candidate, err)
		}
		if has {
			return candidate, nil
		}
	}
	return "", errCronJobNoRuns
}

// latestOwnedJob matches ownership by UID - an orphaned Job keeps the OLD
// CronJob's UID, so name matching would leak its run to a same-named replacement.
func latestOwnedJob(kubectlPath, kubeconfigPaths, ns, cronJobName, kctx string) (jobName, jobUID string, err error) {
	cronJobUID, err := getCronJobUID(kubectlPath, kubeconfigPaths, ns, cronJobName, kctx)
	if err != nil {
		return "", "", fmt.Errorf("get cronjob %s: %w", cronJobName, err)
	}

	getArgs := []string{"get", "jobs", "-n", ns, "--context", kctx, "-o", "json"}
	cmd := exec.Command(kubectlPath, k8s.DemoKubectlArgs(getArgs)...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	logExecCmd("Running kubectl command", cmd)
	out, cmdErr := runKubectlJSON(cmd)
	if cmdErr != nil {
		return "", "", fmt.Errorf("list jobs: %w", cmdErr)
	}

	var list struct {
		Items []struct {
			Metadata struct {
				Name              string    `json:"name"`
				UID               string    `json:"uid"`
				CreationTimestamp time.Time `json:"creationTimestamp"`
				OwnerReferences   []struct {
					Kind string `json:"kind"`
					UID  string `json:"uid"`
				} `json:"ownerReferences"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return "", "", fmt.Errorf("parse jobs list: %w", err)
	}

	var latestTS time.Time
	found := false
	for _, item := range list.Items {
		owned := false
		for _, ref := range item.Metadata.OwnerReferences {
			if ref.Kind == "CronJob" && ref.UID == cronJobUID {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}
		// On an exact timestamp tie, the higher Job name wins so the pick
		// stays deterministic regardless of API return order.
		newer := !found ||
			item.Metadata.CreationTimestamp.After(latestTS) ||
			(item.Metadata.CreationTimestamp.Equal(latestTS) && item.Metadata.Name > jobName)
		if newer {
			jobName = item.Metadata.Name
			jobUID = item.Metadata.UID
			latestTS = item.Metadata.CreationTimestamp
			found = true
		}
	}
	if !found {
		return "", "", errCronJobNoRuns
	}
	return jobName, jobUID, nil
}

// getCronJobUID reads the live CronJob's UID via kubectl, so ownership can be
// matched by UID rather than name (see latestOwnedJob).
func getCronJobUID(kubectlPath, kubeconfigPaths, ns, name, kctx string) (string, error) {
	getArgs := []string{"get", "cronjob", name, "-n", ns, "--context", kctx, "-o", "json"}
	cmd := exec.Command(kubectlPath, k8s.DemoKubectlArgs(getArgs)...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	logExecCmd("Running kubectl command", cmd)
	out, err := runKubectlJSON(cmd)
	if err != nil {
		return "", err
	}

	var obj struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(out, &obj); err != nil {
		return "", err
	}
	if obj.Metadata.UID == "" {
		return "", errCronJobUIDMissing
	}
	return obj.Metadata.UID, nil
}

// podSelectorHasPods reports whether at least one pod currently matches
// selector, so a candidate label-key convention can be verified before it is
// handed to `kubectl logs -l`.
func podSelectorHasPods(kubectlPath, kubeconfigPaths, ns, kctx, selector string) (bool, error) {
	getArgs := []string{"get", "pods", "-l", selector, "-n", ns, "--context", kctx, "-o", "name"}
	cmd := exec.Command(kubectlPath, k8s.DemoKubectlArgs(getArgs)...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	logExecCmd("Running kubectl command", cmd)
	out, err := runKubectlJSON(cmd)
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}
