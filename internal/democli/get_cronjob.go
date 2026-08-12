package democli

import (
	"fmt"
	"io"

	"github.com/janosmiko/lfk/internal/k8s/demo"
)

// cronJobLogSelector is the only selector writePodNamesForSelector answers -
// the prefixed pair buildCronJobOwnedPod (internal/k8s/demo/jobs.go) sets.
// The legacy fallback needs a real apiserver to exercise.
func cronJobLogSelector() string {
	return fmt.Sprintf("batch.kubernetes.io/job-name=%s,batch.kubernetes.io/controller-uid=%s",
		demo.JobNightlyBackupRun, demo.UIDJobNightlyBackupRun)
}

// writeCronJobUID answers `get cronjob <name> -o json`. An unrecognized name
// gets an empty uid, which getCronJobUID turns into a real error rather
// than "no runs yet".
func writeCronJobUID(stdout io.Writer, name string) error {
	uid := ""
	if name == demo.CronJobNightlyBackup {
		uid = demo.UIDCronJobNightlyBackup
	}
	_, err := fmt.Fprintf(stdout, `{"metadata":{"uid":%q}}`+"\n", uid)
	return err
}

// writeOwnedJobsList answers `kubectl get jobs -o json`: the demo cluster
// has exactly one Job, owned by the one seeded CronJob.
func writeOwnedJobsList(stdout io.Writer) error {
	body := fmt.Sprintf(
		`{"items":[{"metadata":{"name":%q,"uid":%q,"creationTimestamp":"2024-01-01T00:00:00Z",`+
			`"ownerReferences":[{"kind":"CronJob","uid":%q}]}}]}`,
		demo.JobNightlyBackupRun, demo.UIDJobNightlyBackupRun, demo.UIDCronJobNightlyBackup)
	_, err := fmt.Fprintln(stdout, body)
	return err
}

// writePodNamesForSelector answers `kubectl get pods -l <selector> -o name`.
func writePodNamesForSelector(stdout io.Writer, selector string) error {
	if selector != cronJobLogSelector() {
		return nil // no match: an empty result, same as a real cluster's
	}
	_, err := fmt.Fprintf(stdout, "pod/%s\n", demo.PodNightlyBackupRun)
	return err
}
