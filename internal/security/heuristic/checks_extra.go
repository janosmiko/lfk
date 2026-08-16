package heuristic

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// checkEnvFromSecret flags containers importing entire Secrets via envFrom.
// Unlike secret_env (name-based, doesn't see envFrom), this exposes every
// Secret key to the whole process environment, leaking via
// /proc/<pid>/environ, crash dumps, and child processes. The summary lists
// Secret names only — never values.
func checkEnvFromSecret(pod *corev1.Pod, c corev1.Container) []security.Finding {
	var names []string
	for _, ef := range c.EnvFrom {
		if ef.SecretRef != nil {
			names = append(names, ef.SecretRef.Name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "envfrom_secret", security.SeverityLow,
		"entire Secret in environment",
		fmt.Sprintf("Container %q imports every key of Secret(s) %s into its environment via envFrom. Mount as files or reference individual keys with secretKeyRef.", c.Name, strings.Join(names, ", ")))}
}

// checkEphemeralContainers flags pods with ephemeral (debug) containers still
// attached — usually a leftover `kubectl debug` session. They cannot be
// removed without recreating the pod and keep their tooling reachable.
// Only emits for the first container — the field is pod-level.
func checkEphemeralContainers(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if !firstContainer(pod, c) {
		return nil
	}
	if len(pod.Spec.EphemeralContainers) == 0 {
		return nil
	}
	names := make([]string, 0, len(pod.Spec.EphemeralContainers))
	for i := range pod.Spec.EphemeralContainers {
		names = append(names, pod.Spec.EphemeralContainers[i].Name)
	}
	return []security.Finding{makeFinding(pod, c, "ephemeral_containers", security.SeverityLow,
		"ephemeral debug container attached",
		fmt.Sprintf("Pod carries ephemeral container(s) %s, likely a leftover kubectl debug session. Recreate the pod to remove them.", strings.Join(names, ", ")))}
}

// checkBarePod flags pods with no ownerReferences — not managed by any
// controller, so never rescheduled on node failure, excluded from rollouts,
// and invisible to PDBs. Emitted as CategoryReliability: it is an
// operational recommendation, kept off the per-resource SEC badge like the
// advisor's findings. Only emits for the first container — pod-level.
func checkBarePod(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if !firstContainer(pod, c) {
		return nil
	}
	if len(pod.OwnerReferences) > 0 {
		return nil
	}
	f := makeFinding(pod, c, "bare_pod", security.SeverityLow,
		"bare pod without a controller",
		"Pod has no owning controller; it is never rescheduled on node failure and no rollout or PodDisruptionBudget covers it. Manage it with a Deployment, StatefulSet, or Job.")
	f.Category = security.CategoryReliability
	return []security.Finding{f}
}

// checkHostProcess flags Windows containers running as a HostProcess —
// the Windows equivalent of privileged: the container runs directly on the
// host with the pod's identity. Container-level windowsOptions override
// pod-level, mirroring the seccomp precedence.
func checkHostProcess(pod *corev1.Pod, c corev1.Container) []security.Finding {
	var hp *bool
	if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.WindowsOptions != nil {
		hp = pod.Spec.SecurityContext.WindowsOptions.HostProcess
	}
	if c.SecurityContext != nil && c.SecurityContext.WindowsOptions != nil && c.SecurityContext.WindowsOptions.HostProcess != nil {
		hp = c.SecurityContext.WindowsOptions.HostProcess
	}
	if hp == nil || !*hp {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "host_process", security.SeverityCritical,
		"Windows HostProcess container",
		fmt.Sprintf("Container %q runs as a Windows HostProcess, executing directly on the node host — equivalent to privileged.", c.Name))}
}
