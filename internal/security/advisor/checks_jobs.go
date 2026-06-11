// Batch-workload advisor checks: forgotten suspended CronJobs, standalone
// Jobs that accumulate forever, and StatefulSet volumes on StorageClasses
// that cannot be expanded.

package advisor

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/security"
)

// batchFindings runs the CronJob/Job and StatefulSet storage checks.
func (d *clusterData) batchFindings() []security.Finding {
	var out []security.Finding
	if d.cronJobsOK {
		for i := range d.cronJobs {
			cj := &d.cronJobs[i]
			if systemNamespaces[cj.Namespace] {
				continue
			}
			if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
				out = append(out, makeFinding(cj.Namespace, "CronJob", cj.Name, "cronjob_suspended", security.SeverityLow,
					"CronJob suspended",
					fmt.Sprintf("CronJob %q is suspended; it has not been scheduling and will not until suspend is removed. Delete it if it is no longer needed.", cj.Name)))
			}
		}
	}
	if d.jobsOK {
		for i := range d.jobs {
			j := &d.jobs[i]
			if systemNamespaces[j.Namespace] || len(j.OwnerReferences) > 0 {
				continue
			}
			if j.Spec.TTLSecondsAfterFinished == nil {
				out = append(out, makeFinding(j.Namespace, "Job", j.Name, "job_no_ttl", security.SeverityLow,
					"Job without TTL",
					fmt.Sprintf("Standalone Job %q sets no ttlSecondsAfterFinished; finished jobs and their pods accumulate until deleted by hand.", j.Name)))
			}
		}
	}
	out = append(out, d.storageExpansionFindings()...)
	return out
}

// storageExpansionFindings flags StatefulSets whose volumeClaimTemplates use
// a StorageClass with allowVolumeExpansion unset or false — the volumes
// cannot grow in place, and StatefulSet storage is painful to migrate.
func (d *clusterData) storageExpansionFindings() []security.Finding {
	if !d.scOK {
		return nil
	}
	expandable := make(map[string]bool, len(d.storageClasses))
	defaultClass := ""
	for i := range d.storageClasses {
		sc := &d.storageClasses[i]
		expandable[sc.Name] = sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
		if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			defaultClass = sc.Name
		}
	}
	var out []security.Finding
	for i := range d.workloads {
		w := &d.workloads[i]
		if len(w.stsVCTClasses) == 0 || systemNamespaces[w.namespace] {
			continue
		}
		var fixed []string
		for _, class := range w.stsVCTClasses {
			if class == "" {
				class = defaultClass
			}
			if class == "" {
				continue // no default class resolvable; different problem
			}
			if canExpand, known := expandable[class]; known && !canExpand {
				fixed = append(fixed, class)
			}
		}
		if len(fixed) > 0 {
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "storageclass_no_expansion", security.SeverityLow,
				"volumes cannot be expanded",
				fmt.Sprintf("StatefulSet %q claims volumes from StorageClass(es) %s without allowVolumeExpansion; growing the volumes later means migrating the StatefulSet's storage by hand.", w.name, strings.Join(fixed, ", "))))
		}
	}
	return out
}
