package heuristic

import (
	"fmt"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// firstContainer reports whether c is the first regular container, the
// convention pod-level checks use to emit a single finding per pod.
func firstContainer(pod *corev1.Pod, c corev1.Container) bool {
	return len(pod.Spec.Containers) > 0 && pod.Spec.Containers[0].Name == c.Name
}

// runtimeSocketPaths are container-runtime control sockets; mounting one is
// equivalent to root on the node. /var/run is a symlink to /run on modern
// distros, so both spellings are listed.
var runtimeSocketPaths = map[string]bool{
	"/var/run/docker.sock":                true,
	"/run/docker.sock":                    true,
	"/var/run/containerd/containerd.sock": true,
	"/run/containerd/containerd.sock":     true,
	"/var/run/crio/crio.sock":             true,
	"/run/crio/crio.sock":                 true,
	"/var/run/dockershim.sock":            true,
	"/run/dockershim.sock":                true,
	"/var/run/cri-dockerd.sock":           true,
	"/run/cri-dockerd.sock":               true,
	"/var/run/podman/podman.sock":         true,
	"/run/podman/podman.sock":             true,
}

// checkRuntimeSocket flags hostPath mounts of container-runtime sockets.
// Only emits for the first container — volumes are pod-level. Distinct from
// the generic host_path check so it can carry Critical severity and be
// ignored independently.
func checkRuntimeSocket(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if !firstContainer(pod, c) {
		return nil
	}
	var findings []security.Finding
	for _, v := range pod.Spec.Volumes {
		if v.HostPath == nil {
			continue
		}
		path := strings.TrimSuffix(v.HostPath.Path, "/")
		if !runtimeSocketPaths[path] {
			continue
		}
		findings = append(findings, makeFinding(pod, c, "runtime_socket", security.SeverityCritical,
			"container runtime socket mounted",
			fmt.Sprintf("Volume %q mounts the container runtime socket %q, equivalent to root on the node.", v.Name, v.HostPath.Path)))
	}
	return findings
}

// safeSysctls is the kubelet's allowed-by-default set (Pod Security Standards
// baseline), mirroring upstream pkg/kubelet/sysctl/safe_sysctls.go. It tracks
// the newest Kubernetes release; some entries are version-gated upstream, so
// on older clusters a listed sysctl may still need the kubelet allowlist.
var safeSysctls = map[string]bool{
	"kernel.shm_rmid_forced":              true,
	"net.ipv4.ip_local_port_range":        true,
	"net.ipv4.ip_unprivileged_port_start": true,
	"net.ipv4.tcp_syncookies":             true,
	"net.ipv4.ping_group_range":           true,
	"net.ipv4.ip_local_reserved_ports":    true,
	"net.ipv4.tcp_keepalive_time":         true,
	"net.ipv4.tcp_fin_timeout":            true,
	"net.ipv4.tcp_keepalive_intvl":        true,
	"net.ipv4.tcp_keepalive_probes":       true,
	"net.ipv4.tcp_rmem":                   true,
	"net.ipv4.tcp_wmem":                   true,
	"net.ipv4.tcp_slow_start_after_idle":  true,
	"net.ipv4.tcp_notsent_lowat":          true,
}

// checkUnsafeSysctls flags pods setting sysctls outside the kubelet safe set.
// Only emits for the first container — sysctls are pod-level.
func checkUnsafeSysctls(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if !firstContainer(pod, c) {
		return nil
	}
	if pod.Spec.SecurityContext == nil {
		return nil
	}
	var unsafe []string
	for _, s := range pod.Spec.SecurityContext.Sysctls {
		if !safeSysctls[s.Name] {
			unsafe = append(unsafe, s.Name)
		}
	}
	if len(unsafe) == 0 {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "unsafe_sysctls", security.SeverityHigh,
		"unsafe sysctls set",
		fmt.Sprintf("Pod sets sysctls outside the kubelet safe set: %s.", strings.Join(unsafe, ", ")))}
}

// checkProcMount flags containers with procMount: Unmasked, which exposes
// host paths normally masked inside /proc.
func checkProcMount(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if c.SecurityContext == nil || c.SecurityContext.ProcMount == nil {
		return nil
	}
	if *c.SecurityContext.ProcMount != corev1.UnmaskedProcMount {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "proc_mount", security.SeverityHigh,
		"unmasked /proc mount",
		fmt.Sprintf("Container %q sets procMount: Unmasked, exposing masked /proc paths from the host.", c.Name))}
}

// checkHostPort flags containers binding ports directly on the node.
func checkHostPort(pod *corev1.Pod, c corev1.Container) []security.Finding {
	var ports []string
	for _, p := range c.Ports {
		if p.HostPort != 0 {
			ports = append(ports, fmt.Sprintf("%d", p.HostPort))
		}
	}
	if len(ports) == 0 {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "host_port", security.SeverityMedium,
		"hostPort binding",
		fmt.Sprintf("Container %q binds host port(s) %s, bypassing Services and NetworkPolicies.", c.Name, strings.Join(ports, ", ")))}
}

// checkSeccompUnconfined flags containers whose effective seccomp profile is
// explicitly Unconfined (container-level overrides pod-level). The absence of
// a profile is deliberately not flagged — it would fire on nearly every pod.
func checkSeccompUnconfined(pod *corev1.Pod, c corev1.Container) []security.Finding {
	var prof *corev1.SeccompProfile
	switch {
	case c.SecurityContext != nil && c.SecurityContext.SeccompProfile != nil:
		prof = c.SecurityContext.SeccompProfile
	case pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.SeccompProfile != nil:
		prof = pod.Spec.SecurityContext.SeccompProfile
	}
	if prof == nil || prof.Type != corev1.SeccompProfileTypeUnconfined {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "seccomp_unconfined", security.SeverityMedium,
		"seccomp disabled",
		fmt.Sprintf("Container %q runs with seccompProfile: Unconfined, disabling syscall filtering.", c.Name))}
}

// checkShareProcessNamespace flags pods sharing one PID namespace across
// containers, letting them signal each other and read /proc/<pid>/environ.
// Only emits for the first container — the field is pod-level.
func checkShareProcessNamespace(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if !firstContainer(pod, c) {
		return nil
	}
	if pod.Spec.ShareProcessNamespace == nil || !*pod.Spec.ShareProcessNamespace {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "share_process_ns", security.SeverityMedium,
		"shared process namespace",
		"Pod sets shareProcessNamespace: true; containers can signal each other and read each other's /proc/<pid>/environ.")}
}

// secretEnvKeywords match env var names that conventionally hold credentials.
var secretEnvKeywords = []string{
	"PASSWORD", "PASSWD", "SECRET", "TOKEN",
	"API_KEY", "APIKEY", "ACCESS_KEY", "PRIVATE_KEY", "CREDENTIAL",
}

// secretEnvExempt suppress matches that reference credentials indirectly
// (file paths, object names, endpoints) or are not sensitive (public keys).
var secretEnvExempt = []string{
	"_PATH", "_FILE", "_DIR", "_NAME", "PUBLIC",
	"_URL", "_URI", "_ENDPOINT", "_HOST",
}

// secretEnvExemptPrefixes suppress env vars injected by known tools whose
// values are not credentials. STAKATER_*: Stakater Reloader injects
// STAKATER_<resource>_SECRET vars holding a hash of the secret, used only to
// trigger rolling restarts.
var secretEnvExemptPrefixes = []string{"STAKATER_"}

// checkSecretEnv is the default-configured, test-only variant. Fetch
// dispatches checkSecretEnvWith with the source's configured patterns
// instead of going through allChecks.
func checkSecretEnv(pod *corev1.Pod, c corev1.Container) []security.Finding {
	return checkSecretEnvWith(pod, c, nil, nil)
}

// checkSecretEnvWith flags env vars whose name looks like a credential but
// whose value is a literal in the pod spec instead of a secretKeyRef. The
// summary lists names only — never the values. Include/exclude are
// case-insensitive name globs from config: include adds names to flag (and,
// matched explicitly, overrides a built-in exemption); exclude is never
// flagged and wins over include.
func checkSecretEnvWith(pod *corev1.Pod, c corev1.Container, include, exclude []string) []security.Finding {
	var names []string
	for _, e := range c.Env {
		if e.Value == "" {
			continue
		}
		upper := strings.ToUpper(e.Name)
		if matchesAnyGlob(upper, exclude) {
			continue
		}
		builtin := containsAny(upper, secretEnvKeywords) &&
			!containsAny(upper, secretEnvExempt) &&
			!hasAnyPrefix(upper, secretEnvExemptPrefixes)
		if !builtin && !matchesAnyGlob(upper, include) {
			continue
		}
		names = append(names, e.Name)
	}
	if len(names) == 0 {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "secret_env", security.SeverityMedium,
		"plaintext secret in env",
		fmt.Sprintf("Container %q sets credential-looking env var(s) %s as plaintext literals. Use secretKeyRef instead.", c.Name, strings.Join(names, ", ")))}
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// matchesAnyGlob reports whether s matches any of the shell-style globs
// (`*`, `?`), compared case-insensitively. A malformed pattern is treated as
// a non-match rather than an error — config typos must not hide or invent
// findings for unrelated names.
func matchesAnyGlob(s string, patterns []string) bool {
	for _, p := range patterns {
		if ok, err := path.Match(strings.ToUpper(p), s); err == nil && ok {
			return true
		}
	}
	return false
}

// checkSATokenAutomount flags pods on the default ServiceAccount that still
// auto-mount its API token — the combination that makes default_sa an actual
// exposure. Only emits for the first container — the fields are pod-level.
// A ServiceAccount-level automountServiceAccountToken: false is not visible
// from the pod spec, so such pods are still flagged (static-analysis limit).
func checkSATokenAutomount(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if !firstContainer(pod, c) {
		return nil
	}
	sa := pod.Spec.ServiceAccountName
	if sa != "" && sa != "default" {
		return nil
	}
	if pod.Spec.AutomountServiceAccountToken != nil && !*pod.Spec.AutomountServiceAccountToken {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "sa_token_automount", security.SeverityLow,
		"default SA token auto-mounted",
		"Pod auto-mounts the default ServiceAccount token. Set automountServiceAccountToken: false or use a dedicated SA.")}
}
