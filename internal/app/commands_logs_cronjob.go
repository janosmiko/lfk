package app

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
)

// cronJobNoRunsSentinel is a marker line updateLogLine turns into a status
// message instead of a log line - kubectl has no selector for *v1.CronJob.
const cronJobNoRunsSentinel = "\x00lfk:cronjob-no-runs\x00"

var errCronJobNoRuns = errors.New("no runs yet")

var errCronJobUIDMissing = errors.New("cronjob uid missing")

// resolveCronJobPodSelector resolves a CronJob to the pod selector for its
// newest owned Job. *v1.CronJob has no pod selector of its own. ok is false
// when the CronJob has never run or its newest Job's pods are already gone.
func resolveCronJobPodSelector(kubectlPath, kubeconfigPaths, ns, name, kctx string) (selector string, ok bool) {
	jobName, jobUID, found := latestOwnedJob(kubectlPath, kubeconfigPaths, ns, name, kctx)
	if !found {
		return "", false
	}
	// Prefixed label first, then the unprefixed pair older clusters still use.
	for _, candidate := range []string{
		"batch.kubernetes.io/job-name=" + jobName + ",batch.kubernetes.io/controller-uid=" + jobUID,
		"job-name=" + jobName + ",controller-uid=" + jobUID,
	} {
		if podSelectorHasPods(kubectlPath, kubeconfigPaths, ns, kctx, candidate) {
			return candidate, true
		}
	}
	return "", false
}

// latestOwnedJob matches ownership by UID - an orphaned Job keeps the OLD
// CronJob's UID, so name matching would leak its run to a same-named replacement.
func latestOwnedJob(kubectlPath, kubeconfigPaths, ns, cronJobName, kctx string) (jobName, jobUID string, found bool) {
	cronJobUID, err := getCronJobUID(kubectlPath, kubeconfigPaths, ns, cronJobName, kctx)
	if err != nil {
		logger.Error("Failed to read CronJob UID", "cronjob", cronJobName, "error", err)
		return "", "", false
	}

	getArgs := []string{"get", "jobs", "-n", ns, "--context", kctx, "-o", "json"}
	cmd := exec.Command(kubectlPath, k8s.DemoKubectlArgs(getArgs)...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	logExecCmd("Running kubectl command", cmd)
	out, err := cmd.Output()
	if err != nil {
		logger.Error("Failed to list jobs for CronJob", "cronjob", cronJobName, "error", err)
		return "", "", false
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
		logger.Error("Failed to parse kubectl jobs output", "error", err)
		return "", "", false
	}

	var latestTS time.Time
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
	return jobName, jobUID, found
}

// getCronJobUID reads the live CronJob's UID via kubectl, so ownership can be
// matched by UID rather than name (see latestOwnedJob).
func getCronJobUID(kubectlPath, kubeconfigPaths, ns, name, kctx string) (string, error) {
	getArgs := []string{"get", "cronjob", name, "-n", ns, "--context", kctx, "-o", "json"}
	cmd := exec.Command(kubectlPath, k8s.DemoKubectlArgs(getArgs)...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	logExecCmd("Running kubectl command", cmd)
	out, err := cmd.Output()
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
func podSelectorHasPods(kubectlPath, kubeconfigPaths, ns, kctx, selector string) bool {
	getArgs := []string{"get", "pods", "-l", selector, "-n", ns, "--context", kctx, "-o", "name"}
	cmd := exec.Command(kubectlPath, k8s.DemoKubectlArgs(getArgs)...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	logExecCmd("Running kubectl command", cmd)
	out, err := cmd.Output()
	if err != nil {
		logger.Error("Failed to check pods for selector", "selector", selector, "error", err)
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}
