package advisor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func cronJob(ns, name string, suspend bool) *batchv1.CronJob {
	return &batchv1.CronJob{
		Namespace: ns, Name: name,
		Spec: batchv1.CronJobSpec{Suspend: &suspend},
	}
}

func job(ns, name string, ttl *int32, owned bool) *batchv1.Job {
	j := &batchv1.Job{
		Namespace: ns, Name: name,
		Spec: batchv1.JobSpec{TTLSecondsAfterFinished: ttl},
	}
	if owned {
		j.OwnerReferences = []metav1.OwnerReference{{Kind: "CronJob", Name: "parent"}}
	}
	return j
}

// vct builds one volumeClaimTemplate; class "" leaves storageClassName nil
// (cluster default).
func vct(class string) []corev1.PersistentVolumeClaim {
	pvc := corev1.PersistentVolumeClaim{Name: "data"}
	if class != "" {
		pvc.Spec.StorageClassName = &class
	}
	return []corev1.PersistentVolumeClaim{pvc}
}

func storageClass(name string, expandable *bool, isDefault bool) *storagev1.StorageClass {
	sc := &storagev1.StorageClass{
		Name:                 name,
		AllowVolumeExpansion: expandable,
	}
	if isDefault {
		sc.Annotations = map[string]string{"storageclass.kubernetes.io/is-default-class": "true"}
	}
	return sc
}

func TestCronJobAndJobChecks(t *testing.T) {
	ttl := int32(3600)
	checks := fetchChecks(t,
		cronJob("prod", "paused", true),
		cronJob("prod", "active", false),
		job("prod", "standalone-no-ttl", nil, false),
		job("prod", "standalone-ttl", &ttl, false),
		job("prod", "cron-owned", nil, true),
	)
	assert.True(t, checks["prod/CronJob/paused"]["cronjob_suspended"])
	assert.False(t, checks["prod/CronJob/active"]["cronjob_suspended"])
	assert.True(t, checks["prod/Job/standalone-no-ttl"]["job_no_ttl"])
	assert.False(t, checks["prod/Job/standalone-ttl"]["job_no_ttl"])
	assert.False(t, checks["prod/Job/cron-owned"]["job_no_ttl"],
		"CronJob-owned jobs are pruned by history limits, not TTL")
}

func TestStorageExpansionChecks(t *testing.T) {
	fixedClass := "fixed"
	growClass := "grow"
	missingClass := "missing"

	stsFixed := statefulSet("prod", "on-fixed", 2, map[string]string{"app": "a"}, hardened("db"))
	stsFixed.Spec.VolumeClaimTemplates = vct(fixedClass)
	stsGrow := statefulSet("prod", "on-grow", 2, map[string]string{"app": "b"}, hardened("db"))
	stsGrow.Spec.VolumeClaimTemplates = vct(growClass)
	stsDefault := statefulSet("prod", "on-default", 2, map[string]string{"app": "c"}, hardened("db"))
	stsDefault.Spec.VolumeClaimTemplates = vct("")
	stsUnknown := statefulSet("prod", "on-missing", 2, map[string]string{"app": "d"}, hardened("db"))
	stsUnknown.Spec.VolumeClaimTemplates = vct(missingClass)

	checks := fetchChecks(t,
		stsFixed, stsGrow, stsDefault, stsUnknown,
		storageClass("fixed", nil, true), // nil expansion = not expandable; also the default class
		storageClass("grow", new(true), false),
	)
	assert.True(t, checks["prod/StatefulSet/on-fixed"]["storageclass_no_expansion"])
	assert.False(t, checks["prod/StatefulSet/on-grow"]["storageclass_no_expansion"])
	assert.True(t, checks["prod/StatefulSet/on-default"]["storageclass_no_expansion"],
		"an empty storageClassName resolves to the default class")
	assert.False(t, checks["prod/StatefulSet/on-missing"]["storageclass_no_expansion"],
		"an unknown class is a different problem, not an expansion finding")
}
