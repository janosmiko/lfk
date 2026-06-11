package advisor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestLivenessWithoutReadiness(t *testing.T) {
	livenessOnly := hardened("web")
	livenessOnly.ReadinessProbe = nil

	checks := fetchChecks(t,
		deployment("prod", "liveness-only", 2, map[string]string{"app": "a"}, livenessOnly),
		deployment("prod", "both-probes", 2, map[string]string{"app": "b"}, hardened("web")),
		deployment("prod", "no-probes", 2, map[string]string{"app": "c"}, corev1.Container{Name: "web"}),
		// replicas: 1 — the probe check is independent of replica count.
		statefulSet("prod", "sts-liveness-only", 1, map[string]string{"app": "d"}, livenessOnly),
	)
	assert.True(t, checks["prod/Deployment/liveness-only"]["liveness_no_readiness"])
	assert.False(t, checks["prod/Deployment/both-probes"]["liveness_no_readiness"])
	assert.False(t, checks["prod/Deployment/no-probes"]["liveness_no_readiness"],
		"no probes at all is missing_probes territory, not liveness_no_readiness")
	assert.True(t, checks["prod/StatefulSet/sts-liveness-only"]["liveness_no_readiness"],
		"the check applies to every workload kind")
}

func TestZeroGracePeriod(t *testing.T) {
	zero := deployment("prod", "zero-grace", 2, map[string]string{"app": "z"}, hardened("web"))
	zero.Spec.Template.Spec.TerminationGracePeriodSeconds = new(int64)
	thirty := deployment("prod", "thirty-grace", 2, map[string]string{"app": "t"}, hardened("web"))
	g := int64(30)
	thirty.Spec.Template.Spec.TerminationGracePeriodSeconds = &g

	checks := fetchChecks(t,
		zero,
		thirty,
		deployment("prod", "default-grace", 2, map[string]string{"app": "d"}, hardened("web")),
	)
	assert.True(t, checks["prod/Deployment/zero-grace"]["zero_grace_period"])
	assert.False(t, checks["prod/Deployment/thirty-grace"]["zero_grace_period"])
	assert.False(t, checks["prod/Deployment/default-grace"]["zero_grace_period"])
}

func TestOnDeleteUpdateStrategy(t *testing.T) {
	dsOnDelete := daemonSet("prod", "ds-ondelete", map[string]string{"app": "a"}, hardened("agent"))
	dsOnDelete.Spec.UpdateStrategy.Type = appsv1.OnDeleteDaemonSetStrategyType
	dsRolling := daemonSet("prod", "ds-rolling", map[string]string{"app": "b"}, hardened("agent"))
	dsRolling.Spec.UpdateStrategy.Type = appsv1.RollingUpdateDaemonSetStrategyType

	stsOnDelete := statefulSet("prod", "sts-ondelete", 2, map[string]string{"app": "c"}, hardened("db"))
	stsOnDelete.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType

	// System namespaces are excluded from every lifecycle check, even with
	// an explicit OnDelete strategy.
	dsSystem := daemonSet("kube-system", "ds-system", map[string]string{"app": "f"}, hardened("agent"))
	dsSystem.Spec.UpdateStrategy.Type = appsv1.OnDeleteDaemonSetStrategyType
	stsSystem := statefulSet("kube-system", "sts-system", 2, map[string]string{"app": "g"}, hardened("db"))
	stsSystem.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType

	checks := fetchChecks(t,
		dsOnDelete,
		dsRolling,
		// empty strategy type (pre-defaulting fakes) must not flag — the API
		// server defaults it to RollingUpdate.
		daemonSet("prod", "ds-empty", map[string]string{"app": "d"}, hardened("agent")),
		stsOnDelete,
		statefulSet("prod", "sts-default", 2, map[string]string{"app": "e"}, hardened("db")),
		dsSystem,
		stsSystem,
	)
	assert.True(t, checks["prod/DaemonSet/ds-ondelete"]["ondelete_update_strategy"])
	assert.False(t, checks["prod/DaemonSet/ds-rolling"]["ondelete_update_strategy"])
	assert.False(t, checks["prod/DaemonSet/ds-empty"]["ondelete_update_strategy"])
	assert.True(t, checks["prod/StatefulSet/sts-ondelete"]["ondelete_update_strategy"])
	assert.False(t, checks["prod/StatefulSet/sts-default"]["ondelete_update_strategy"])
	assert.False(t, checks["kube-system/DaemonSet/ds-system"]["ondelete_update_strategy"])
	assert.False(t, checks["kube-system/StatefulSet/sts-system"]["ondelete_update_strategy"])
}
