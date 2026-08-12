package democli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/k8s/demo"
)

func TestRun_GetCronJob_ReturnsUID(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"get", "cronjob", demo.CronJobNightlyBackup, "-n", demo.NamespaceJobs, "--context", "demo", "-o", "json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var obj struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &obj); err != nil {
		t.Fatalf("unmarshalling stdout %q: %v", stdout.String(), err)
	}
	if obj.Metadata.UID != demo.UIDCronJobNightlyBackup {
		t.Errorf("uid = %q, want %q", obj.Metadata.UID, demo.UIDCronJobNightlyBackup)
	}
}

func TestRun_GetCronJob_UnknownName_ReturnsEmptyUID(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"get", "cronjob", "no-such-cronjob", "-n", demo.NamespaceJobs, "--context", "demo", "-o", "json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(stdout.String(), demo.UIDCronJobNightlyBackup) {
		t.Errorf("stdout = %q, want no uid for an unseeded CronJob name", stdout.String())
	}
}

func TestRun_GetJobs_ReturnsJobOwnedByCronJob(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"get", "jobs", "-n", demo.NamespaceJobs, "--context", "demo", "-o", "json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var list struct {
		Items []struct {
			Metadata struct {
				Name            string `json:"name"`
				UID             string `json:"uid"`
				OwnerReferences []struct {
					Kind string `json:"kind"`
					UID  string `json:"uid"`
				} `json:"ownerReferences"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		t.Fatalf("unmarshalling stdout %q: %v", stdout.String(), err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(list.Items))
	}
	item := list.Items[0]
	if item.Metadata.Name != demo.JobNightlyBackupRun || item.Metadata.UID != demo.UIDJobNightlyBackupRun {
		t.Errorf("job = %+v, want name=%q uid=%q", item.Metadata, demo.JobNightlyBackupRun, demo.UIDJobNightlyBackupRun)
	}
	if len(item.Metadata.OwnerReferences) != 1 ||
		item.Metadata.OwnerReferences[0].Kind != "CronJob" ||
		item.Metadata.OwnerReferences[0].UID != demo.UIDCronJobNightlyBackup {
		t.Errorf("ownerReferences = %+v, want a CronJob owner with uid %q",
			item.Metadata.OwnerReferences, demo.UIDCronJobNightlyBackup)
	}
}

func TestRun_GetPods_MatchingSelector_ReturnsPodName(t *testing.T) {
	selector := fmt.Sprintf("batch.kubernetes.io/job-name=%s,batch.kubernetes.io/controller-uid=%s",
		demo.JobNightlyBackupRun, demo.UIDJobNightlyBackupRun)

	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"get", "pods", "-l", selector, "-n", demo.NamespaceJobs, "--context", "demo", "-o", "name"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "pod/" + demo.PodNightlyBackupRun
	if strings.TrimSpace(stdout.String()) != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRun_GetPods_LegacySelector_ReturnsEmpty(t *testing.T) {
	selector := fmt.Sprintf("job-name=%s,controller-uid=%s", demo.JobNightlyBackupRun, demo.UIDJobNightlyBackupRun)

	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"get", "pods", "-l", selector, "-n", demo.NamespaceJobs, "--context", "demo", "-o", "name"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Errorf("stdout = %q, want empty - the demo pod does not carry legacy labels", stdout.String())
	}
}

// TestCronJobLogResolution_FullChain replays resolveCronJobPodSelector's
// three-call sequence through Run, proving the chain resolves end to end.
func TestCronJobLogResolution_FullChain(t *testing.T) {
	ctx := t.Context()

	var cronJobOut, jobsOut bytes.Buffer
	if err := Run(ctx, []string{"get", "cronjob", demo.CronJobNightlyBackup, "-n", demo.NamespaceJobs, "--context", "demo", "-o", "json"}, &cronJobOut, &bytes.Buffer{}); err != nil {
		t.Fatalf("get cronjob: %v", err)
	}
	var cronJobObj struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(cronJobOut.Bytes(), &cronJobObj); err != nil {
		t.Fatalf("unmarshalling cronjob: %v", err)
	}

	if err := Run(ctx, []string{"get", "jobs", "-n", demo.NamespaceJobs, "--context", "demo", "-o", "json"}, &jobsOut, &bytes.Buffer{}); err != nil {
		t.Fatalf("get jobs: %v", err)
	}
	var jobsList struct {
		Items []struct {
			Metadata struct {
				Name            string `json:"name"`
				UID             string `json:"uid"`
				OwnerReferences []struct {
					Kind string `json:"kind"`
					UID  string `json:"uid"`
				} `json:"ownerReferences"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(jobsOut.Bytes(), &jobsList); err != nil {
		t.Fatalf("unmarshalling jobs: %v", err)
	}

	var jobName, jobUID string
	for _, item := range jobsList.Items {
		for _, ref := range item.Metadata.OwnerReferences {
			if ref.Kind == "CronJob" && ref.UID == cronJobObj.Metadata.UID {
				jobName, jobUID = item.Metadata.Name, item.Metadata.UID
			}
		}
	}
	if jobName == "" {
		t.Fatalf("no Job owned by CronJob uid %q", cronJobObj.Metadata.UID)
	}

	selector := fmt.Sprintf("batch.kubernetes.io/job-name=%s,batch.kubernetes.io/controller-uid=%s", jobName, jobUID)
	var podsOut bytes.Buffer
	if err := Run(ctx, []string{"get", "pods", "-l", selector, "-n", demo.NamespaceJobs, "--context", "demo", "-o", "name"}, &podsOut, &bytes.Buffer{}); err != nil {
		t.Fatalf("get pods: %v", err)
	}
	if strings.TrimSpace(podsOut.String()) == "" {
		t.Fatalf("no pods matched selector %q, resolved from a real ownership chain walk", selector)
	}
}
