package heuristic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/security"
)

func TestExtendedChecksRegistered(t *testing.T) {
	// 21, not 22: checkSecretEnv takes per-source config, so Fetch dispatches
	// it directly instead of through allChecks.
	assert.Len(t, allChecks, 21, "all extended checks must be wired into allChecks")
}

func TestCheckSecretEnvWith(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
	cases := []struct {
		name    string
		env     []corev1.EnvVar
		include []string
		exclude []string
		want    int
	}{
		{
			"include glob flags non-keyword name",
			[]corev1.EnvVar{{Name: "MY_CONN_STR", Value: "Server=db;Pwd=x"}},
			[]string{"*_CONN_STR"},
			nil, 1,
		},
		{
			"include without match stays clean",
			[]corev1.EnvVar{{Name: "LOG_LEVEL", Value: "debug"}},
			[]string{"*_CONN_STR"},
			nil, 0,
		},
		{
			"exclude glob suppresses keyword name",
			[]corev1.EnvVar{{Name: "LEGACY_PASSWORD", Value: "x"}},
			nil,
			[]string{"LEGACY_*"},
			0,
		},
		{
			"exclude wins over include",
			[]corev1.EnvVar{{Name: "LEGACY_PASSWORD", Value: "x"}},
			[]string{"LEGACY_*"},
			[]string{"LEGACY_*"},
			0,
		},
		{
			"include overrides built-in exemption",
			[]corev1.EnvVar{{Name: "TOKEN_PATH", Value: "literal-token"}},
			[]string{"TOKEN_PATH"},
			nil, 1,
		},
		{
			"matching is case-insensitive",
			[]corev1.EnvVar{{Name: "my_conn_str", Value: "x"}},
			[]string{"*_conn_str"},
			nil, 1,
		},
		{
			"include never flags non-literal values",
			[]corev1.EnvVar{{Name: "MY_CONN_STR"}},
			[]string{"*_CONN_STR"},
			nil, 0,
		},
		{
			"malformed pattern is a no-match, not a panic",
			[]corev1.EnvVar{{Name: "DB_PASSWORD", Value: "x"}},
			nil,
			[]string{"[invalid"},
			1,
		},
		{
			"nil patterns keep built-in behavior",
			[]corev1.EnvVar{{Name: "DB_PASSWORD", Value: "x"}},
			nil, nil, 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := corev1.Container{Name: "c", Env: tc.env}
			findings := checkSecretEnvWith(pod, c, tc.include, tc.exclude)
			assert.Len(t, findings, tc.want)
		})
	}
}

// TestExtendedFirstContainerChecks_NoRegularContainers mirrors
// TestFirstContainerChecks_NoRegularContainers for the extended pod-level
// checks: a pod with only init containers must not panic or emit.
func TestExtendedFirstContainerChecks_NoRegularContainers(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
		Spec: corev1.PodSpec{
			InitContainers:        []corev1.Container{{Name: "init"}},
			ShareProcessNamespace: new(true),
			SecurityContext: &corev1.PodSecurityContext{
				Sysctls: []corev1.Sysctl{{Name: "kernel.msgmax", Value: "1"}},
			},
			Volumes: []corev1.Volume{{Name: "sock", VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/run/docker.sock"},
			}}},
		},
	}
	init := pod.Spec.InitContainers[0]
	assert.Nil(t, checkRuntimeSocket(pod, init))
	assert.Nil(t, checkUnsafeSysctls(pod, init))
	assert.Nil(t, checkShareProcessNamespace(pod, init))
	assert.Nil(t, checkSATokenAutomount(pod, init))
}

func TestCheckRuntimeSocket(t *testing.T) {
	hostPathVol := func(name, path string) corev1.Volume {
		return corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: path},
		}}
	}
	cases := []struct {
		name    string
		volumes []corev1.Volume
		want    int
	}{
		{"docker socket", []corev1.Volume{hostPathVol("sock", "/var/run/docker.sock")}, 1},
		{"docker socket short", []corev1.Volume{hostPathVol("sock", "/run/docker.sock")}, 1},
		{"containerd socket", []corev1.Volume{hostPathVol("sock", "/run/containerd/containerd.sock")}, 1},
		{"crio socket", []corev1.Volume{hostPathVol("sock", "/var/run/crio/crio.sock")}, 1},
		{"cri-dockerd socket", []corev1.Volume{hostPathVol("sock", "/run/cri-dockerd.sock")}, 1},
		{"podman socket", []corev1.Volume{hostPathVol("sock", "/run/podman/podman.sock")}, 1},
		{"trailing slash", []corev1.Volume{hostPathVol("sock", "/var/run/docker.sock/")}, 1},
		{"generic hostPath", []corev1.Volume{hostPathVol("etc", "/etc")}, 0},
		{"emptyDir", []corev1.Volume{{Name: "d", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}, 0},
		{"no volumes", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}, Volumes: tc.volumes},
			}
			findings := checkRuntimeSocket(pod, pod.Spec.Containers[0])
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, security.SeverityCritical, findings[0].Severity)
				assert.Equal(t, "runtime_socket", findings[0].Labels["check"])
			}
		})
	}
	t.Run("non-first container is skipped", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "c1"}, {Name: "c2"}},
				Volumes:    []corev1.Volume{hostPathVol("sock", "/var/run/docker.sock")},
			},
		}
		assert.Nil(t, checkRuntimeSocket(pod, pod.Spec.Containers[1]))
	})
}

func TestCheckUnsafeSysctls(t *testing.T) {
	cases := []struct {
		name    string
		sysctls []corev1.Sysctl
		want    int
		wantIn  []string
	}{
		{"nil", nil, 0, nil},
		{"safe only", []corev1.Sysctl{
			{Name: "net.ipv4.ip_local_port_range", Value: "1024 65535"},
			{Name: "kernel.shm_rmid_forced", Value: "1"},
			{Name: "net.ipv4.tcp_notsent_lowat", Value: "16384"},
		}, 0, nil},
		{"unsafe", []corev1.Sysctl{{Name: "net.core.somaxconn", Value: "1024"}}, 1, []string{"net.core.somaxconn"}},
		{"mixed", []corev1.Sysctl{
			{Name: "net.ipv4.tcp_syncookies", Value: "1"},
			{Name: "kernel.msgmax", Value: "65536"},
		}, 1, []string{"kernel.msgmax"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}},
			}
			if tc.sysctls != nil {
				pod.Spec.SecurityContext = &corev1.PodSecurityContext{Sysctls: tc.sysctls}
			}
			findings := checkUnsafeSysctls(pod, pod.Spec.Containers[0])
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, security.SeverityHigh, findings[0].Severity)
				assert.Equal(t, "unsafe_sysctls", findings[0].Labels["check"])
				for _, s := range tc.wantIn {
					assert.Contains(t, findings[0].Summary, s)
				}
			}
		})
	}
	t.Run("non-first container is skipped", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
			Spec: corev1.PodSpec{
				Containers:      []corev1.Container{{Name: "c1"}, {Name: "c2"}},
				SecurityContext: &corev1.PodSecurityContext{Sysctls: []corev1.Sysctl{{Name: "kernel.msgmax", Value: "1"}}},
			},
		}
		assert.Nil(t, checkUnsafeSysctls(pod, pod.Spec.Containers[1]))
	})
}

func TestCheckProcMount(t *testing.T) {
	unmasked := corev1.UnmaskedProcMount
	def := corev1.DefaultProcMount
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
	cases := []struct {
		name string
		sc   *corev1.SecurityContext
		want int
	}{
		{"nil context", nil, 0},
		{"unset", &corev1.SecurityContext{}, 0},
		{"default", &corev1.SecurityContext{ProcMount: &def}, 0},
		{"unmasked", &corev1.SecurityContext{ProcMount: &unmasked}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := corev1.Container{Name: "c", SecurityContext: tc.sc}
			findings := checkProcMount(pod, c)
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, security.SeverityHigh, findings[0].Severity)
				assert.Equal(t, "proc_mount", findings[0].Labels["check"])
			}
		})
	}
}

func TestCheckHostPort(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
	cases := []struct {
		name   string
		ports  []corev1.ContainerPort
		want   int
		wantIn []string
	}{
		{"no ports", nil, 0, nil},
		{"container port only", []corev1.ContainerPort{{ContainerPort: 8080}}, 0, nil},
		{"one host port", []corev1.ContainerPort{{ContainerPort: 8080, HostPort: 8080}}, 1, []string{"8080"}},
		{"two host ports one finding", []corev1.ContainerPort{
			{ContainerPort: 8080, HostPort: 8080},
			{ContainerPort: 9090, HostPort: 9091},
		}, 1, []string{"8080", "9091"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := corev1.Container{Name: "c", Ports: tc.ports}
			findings := checkHostPort(pod, c)
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, security.SeverityMedium, findings[0].Severity)
				assert.Equal(t, "host_port", findings[0].Labels["check"])
				for _, s := range tc.wantIn {
					assert.Contains(t, findings[0].Summary, s)
				}
			}
		})
	}
}

func TestCheckSeccompUnconfined(t *testing.T) {
	unconfined := &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined}
	runtimeDefault := &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	cases := []struct {
		name     string
		podProf  *corev1.SeccompProfile
		contProf *corev1.SeccompProfile
		want     int
	}{
		{"no profiles", nil, nil, 0},
		{"container runtime default", nil, runtimeDefault, 0},
		{"container unconfined", nil, unconfined, 1},
		{"pod unconfined", unconfined, nil, 1},
		{"pod unconfined container overrides", unconfined, runtimeDefault, 0},
		{"pod runtime default", runtimeDefault, nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
			if tc.podProf != nil {
				pod.Spec.SecurityContext = &corev1.PodSecurityContext{SeccompProfile: tc.podProf}
			}
			c := corev1.Container{Name: "c"}
			if tc.contProf != nil {
				c.SecurityContext = &corev1.SecurityContext{SeccompProfile: tc.contProf}
			}
			pod.Spec.Containers = []corev1.Container{c}
			findings := checkSeccompUnconfined(pod, c)
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, security.SeverityMedium, findings[0].Severity)
				assert.Equal(t, "seccomp_unconfined", findings[0].Labels["check"])
			}
		})
	}
}

func TestCheckShareProcessNamespace(t *testing.T) {
	cases := []struct {
		name  string
		share *bool
		want  int
	}{
		{"nil", nil, 0},
		{"false", new(false), 0},
		{"true", new(true), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
				Spec: corev1.PodSpec{
					ShareProcessNamespace: tc.share,
					Containers:            []corev1.Container{{Name: "c"}},
				},
			}
			findings := checkShareProcessNamespace(pod, pod.Spec.Containers[0])
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, security.SeverityMedium, findings[0].Severity)
				assert.Equal(t, "share_process_ns", findings[0].Labels["check"])
			}
		})
	}
	t.Run("non-first container is skipped", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
			Spec: corev1.PodSpec{
				ShareProcessNamespace: new(true),
				Containers:            []corev1.Container{{Name: "c1"}, {Name: "c2"}},
			},
		}
		assert.Nil(t, checkShareProcessNamespace(pod, pod.Spec.Containers[1]))
	})
}

func TestCheckSecretEnv(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
	fromSecret := &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "s"}, Key: "k",
		},
	}
	cases := []struct {
		name    string
		env     []corev1.EnvVar
		want    int
		wantIn  []string
		wantOut []string
	}{
		{"no env", nil, 0, nil, nil},
		{"plain var", []corev1.EnvVar{{Name: "LOG_LEVEL", Value: "debug"}}, 0, nil, nil},
		{
			"literal password",
			[]corev1.EnvVar{{Name: "DB_PASSWORD", Value: "hunter2"}},
			1,
			[]string{"DB_PASSWORD"},
			[]string{"hunter2"},
		},
		{"secretKeyRef is clean", []corev1.EnvVar{{Name: "API_TOKEN", ValueFrom: fromSecret}}, 0, nil, nil},
		{"path exempt", []corev1.EnvVar{{Name: "TOKEN_PATH", Value: "/var/run/secrets/token"}}, 0, nil, nil},
		{"file exempt", []corev1.EnvVar{{Name: "SECRET_FILE", Value: "/etc/secret"}}, 0, nil, nil},
		{"name exempt", []corev1.EnvVar{{Name: "SECRET_NAME", Value: "my-secret"}}, 0, nil, nil},
		{"public exempt", []corev1.EnvVar{{Name: "PUBLIC_KEY", Value: "ssh-rsa AAA"}}, 0, nil, nil},
		{"url exempt", []corev1.EnvVar{{Name: "GF_AUTH_AZUREAD_TOKEN_URL", Value: "https://login.example.com/oauth2/token"}}, 0, nil, nil},
		{"endpoint exempt", []corev1.EnvVar{{Name: "OIDC_TOKEN_ENDPOINT", Value: "https://idp.example.com/token"}}, 0, nil, nil},
		{"reloader hash exempt", []corev1.EnvVar{{Name: "STAKATER_KUBE_PROMETHEUS_STACK_SECRETS_SECRET", Value: "da39a3ee5e6b4b0d"}}, 0, nil, nil},
		{"empty value", []corev1.EnvVar{{Name: "DB_PASSWORD", Value: ""}}, 0, nil, nil},
		{"two secrets one finding", []corev1.EnvVar{
			{Name: "DB_PASSWORD", Value: "hunter2value"},
			{Name: "AWS_SECRET_ACCESS_KEY", Value: "wJalrXUtnFEMI"},
		}, 1, []string{"DB_PASSWORD", "AWS_SECRET_ACCESS_KEY"}, []string{"hunter2value", "wJalrXUtnFEMI"}},
		{
			"lowercase name matches",
			[]corev1.EnvVar{{Name: "db_password", Value: "x"}},
			1,
			[]string{"db_password"},
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := corev1.Container{Name: "c", Env: tc.env}
			findings := checkSecretEnv(pod, c)
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, security.SeverityMedium, findings[0].Severity)
				assert.Equal(t, "secret_env", findings[0].Labels["check"])
				for _, s := range tc.wantIn {
					assert.Contains(t, findings[0].Summary, s)
				}
				for _, s := range tc.wantOut {
					assert.NotContains(t, findings[0].Summary, s, "finding must never echo the env value")
				}
			}
		})
	}
}

func TestCheckSATokenAutomount(t *testing.T) {
	cases := []struct {
		name      string
		sa        string
		automount *bool
		want      int
	}{
		{"default SA, automount unset", "default", nil, 1},
		{"empty SA, automount unset", "", nil, 1},
		{"default SA, automount true", "default", new(true), 1},
		{"default SA, automount false", "default", new(false), 0},
		{"dedicated SA", "api-sa", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
				Spec: corev1.PodSpec{
					ServiceAccountName:           tc.sa,
					AutomountServiceAccountToken: tc.automount,
					Containers:                   []corev1.Container{{Name: "c"}},
				},
			}
			findings := checkSATokenAutomount(pod, pod.Spec.Containers[0])
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, security.SeverityLow, findings[0].Severity)
				assert.Equal(t, "sa_token_automount", findings[0].Labels["check"])
			}
		})
	}
	t.Run("non-first container is skipped", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c1"}, {Name: "c2"}}},
		}
		assert.Nil(t, checkSATokenAutomount(pod, pod.Spec.Containers[1]))
	})
}
