package heuristic

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/janosmiko/lfk/internal/security"
)

func baseRef(pod *corev1.Pod, container corev1.Container) security.ResourceRef {
	return security.ResourceRef{
		Namespace: pod.Namespace,
		Kind:      "Pod",
		Name:      pod.Name,
		Container: container.Name,
	}
}

func makeFinding(pod *corev1.Pod, container corev1.Container, check string, sev security.Severity, title, summary string) security.Finding {
	return security.Finding{
		ID:       fmt.Sprintf("heuristic/%s/%s/%s/%s", pod.Namespace, pod.Name, container.Name, check),
		Source:   "heuristic",
		Category: security.CategoryMisconfig,
		Severity: sev,
		Title:    title,
		Resource: baseRef(pod, container),
		Summary:  summary,
		Labels:   map[string]string{"check": check, "container": container.Name},
	}
}

// checkPrivileged flags containers running with Privileged: true.
func checkPrivileged(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if c.SecurityContext == nil || c.SecurityContext.Privileged == nil {
		return nil
	}
	if !*c.SecurityContext.Privileged {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "privileged", security.SeverityCritical,
		"privileged container",
		fmt.Sprintf("Container %q runs in privileged mode, enabling full host access and container escape risk.", c.Name))}
}

// checkHostNamespaces flags pods that share the host's PID/network/IPC namespaces.
// Only emits for the first container (bound to first container) to avoid duplication.
func checkHostNamespaces(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if pod.Spec.Containers[0].Name != c.Name {
		return nil
	}
	var findings []security.Finding
	if pod.Spec.HostPID {
		findings = append(findings, makeFinding(pod, c, "host_pid", security.SeverityHigh,
			"hostPID enabled", "Pod shares the host PID namespace, exposing all host processes."))
	}
	if pod.Spec.HostNetwork {
		findings = append(findings, makeFinding(pod, c, "host_network", security.SeverityHigh,
			"hostNetwork enabled", "Pod shares the host network namespace, bypassing NetworkPolicies."))
	}
	if pod.Spec.HostIPC {
		findings = append(findings, makeFinding(pod, c, "host_ipc", security.SeverityHigh,
			"hostIPC enabled", "Pod shares the host IPC namespace, exposing host SYSV IPC objects."))
	}
	return findings
}

// checkHostPath flags pods that mount hostPath volumes.
// Only emits for the first container — volumes are pod-level.
func checkHostPath(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if pod.Spec.Containers[0].Name != c.Name {
		return nil
	}
	var findings []security.Finding
	for _, v := range pod.Spec.Volumes {
		if v.HostPath == nil {
			continue
		}
		findings = append(findings, makeFinding(pod, c, "host_path", security.SeverityHigh,
			"hostPath volume mount",
			fmt.Sprintf("Volume %q mounts host path %q, granting node filesystem access.", v.Name, v.HostPath.Path)))
	}
	return findings
}

// checkReadOnlyRootFilesystem flags containers with a writable root filesystem.
func checkReadOnlyRootFilesystem(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if c.SecurityContext != nil && c.SecurityContext.ReadOnlyRootFilesystem != nil && *c.SecurityContext.ReadOnlyRootFilesystem {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "readonly_root_fs", security.SeverityLow,
		"writable root filesystem",
		fmt.Sprintf("Container %q has a writable root filesystem. Prefer readOnlyRootFilesystem: true with emptyDir for temp.", c.Name))}
}

// checkRunAsRoot flags containers that may run as uid 0.
func checkRunAsRoot(pod *corev1.Pod, c corev1.Container) []security.Finding {
	// If either pod-level or container-level context guarantees non-root, clean.
	if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsNonRoot != nil && *pod.Spec.SecurityContext.RunAsNonRoot {
		return nil
	}
	if c.SecurityContext != nil && c.SecurityContext.RunAsNonRoot != nil && *c.SecurityContext.RunAsNonRoot {
		return nil
	}
	// Non-zero runAsUser on the container is fine.
	if c.SecurityContext != nil && c.SecurityContext.RunAsUser != nil && *c.SecurityContext.RunAsUser != 0 {
		return nil
	}
	// Pod-level non-zero is also fine if the container does not override.
	if (c.SecurityContext == nil || c.SecurityContext.RunAsUser == nil) &&
		pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsUser != nil && *pod.Spec.SecurityContext.RunAsUser != 0 {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "run_as_root", security.SeverityMedium,
		"container may run as root",
		fmt.Sprintf("Container %q has no explicit non-root guarantee (runAsNonRoot: true or runAsUser > 0).", c.Name))}
}

// checkAllowPrivilegeEscalation flags containers that can escalate privileges.
func checkAllowPrivilegeEscalation(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if c.SecurityContext != nil && c.SecurityContext.AllowPrivilegeEscalation != nil && !*c.SecurityContext.AllowPrivilegeEscalation {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "allow_priv_esc", security.SeverityMedium,
		"privilege escalation allowed",
		fmt.Sprintf("Container %q does not set allowPrivilegeEscalation: false. Setuid binaries can elevate.", c.Name))}
}

var dangerousCapabilities = map[corev1.Capability]bool{
	"SYS_ADMIN":  true,
	"NET_ADMIN":  true,
	"SYS_PTRACE": true,
	"SYS_MODULE": true,
	"NET_RAW":    true,
	"ALL":        true,
}

// checkDangerousCapabilities flags containers adding known-dangerous capabilities.
func checkDangerousCapabilities(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if c.SecurityContext == nil || c.SecurityContext.Capabilities == nil {
		return nil
	}
	var findings []security.Finding
	for _, cap := range c.SecurityContext.Capabilities.Add {
		if !dangerousCapabilities[cap] {
			continue
		}
		findings = append(findings, makeFinding(pod, c, "dangerous_caps_"+string(cap), security.SeverityHigh,
			"dangerous capability added",
			fmt.Sprintf("Container %q adds capability %q. Drop it unless strictly required.", c.Name, cap)))
	}
	return findings
}

// checkResourceLimits flags containers missing CPU or memory limits.
func checkResourceLimits(pod *corev1.Pod, c corev1.Container) []security.Finding {
	cpu, hasCPU := c.Resources.Limits[corev1.ResourceCPU]
	mem, hasMem := c.Resources.Limits[corev1.ResourceMemory]
	if hasCPU && !cpu.IsZero() && hasMem && !mem.IsZero() {
		return nil
	}
	var missing []string
	if !hasCPU || cpu.IsZero() {
		missing = append(missing, "cpu")
	}
	if !hasMem || mem.IsZero() {
		missing = append(missing, "memory")
	}
	return []security.Finding{makeFinding(pod, c, "missing_resource_limits", security.SeverityLow,
		"missing resource limits",
		fmt.Sprintf("Container %q is missing resource limits (%v). Unbounded containers can DoS the node.", c.Name, missing))}
}

// checkDefaultServiceAccount flags pods using the namespace's default service account.
// Emits once per pod (bound to first container).
func checkDefaultServiceAccount(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if pod.Spec.Containers[0].Name != c.Name {
		return nil
	}
	sa := pod.Spec.ServiceAccountName
	if sa != "" && sa != "default" {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "default_sa", security.SeverityLow,
		"uses default ServiceAccount",
		"Pod uses the default ServiceAccount. Create a dedicated SA with minimal RBAC.")}
}

// checkLatestImageTag flags containers without a pinned tag.
func checkLatestImageTag(pod *corev1.Pod, c corev1.Container) []security.Finding {
	img := c.Image
	if strings.Contains(img, "@sha256:") {
		return nil
	}
	last := strings.LastIndex(img, "/")
	rest := img
	if last >= 0 {
		rest = img[last+1:]
	}
	colon := strings.Index(rest, ":")
	tag := ""
	if colon >= 0 {
		tag = rest[colon+1:]
	}
	if tag != "" && tag != "latest" {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "latest_tag", security.SeverityLow,
		"unpinned image tag",
		fmt.Sprintf("Container %q uses image %q without a pinned version. Pin by tag or digest.", c.Name, img))}
}

// allChecks is the ordered list of checks the Source runs against each container.
var allChecks = []checkFn{
	checkPrivileged,
	checkHostNamespaces,
	checkHostPath,
	checkReadOnlyRootFilesystem,
	checkRunAsRoot,
	checkAllowPrivilegeEscalation,
	checkDangerousCapabilities,
	checkResourceLimits,
	checkDefaultServiceAccount,
	checkLatestImageTag,
}
