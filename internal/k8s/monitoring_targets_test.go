package k8s

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/model"
)

func monitoringSvc(ns, name, appName string, ports ...corev1.ServicePort) *corev1.Service {
	return &corev1.Service{
		Name:      name,
		Namespace: ns,
		Labels:    map[string]string{appNameLabel: appName},
		Spec:      corev1.ServiceSpec{Ports: ports},
	}
}

func port(name string, num int32) corev1.ServicePort {
	return corev1.ServicePort{Name: name, Port: num, Protocol: corev1.ProtocolTCP}
}

// --- servicePort ---

func TestServicePort(t *testing.T) {
	tests := []struct {
		name  string
		ports []corev1.ServicePort
		want  string
	}{
		{"no ports", nil, ""},
		{"single port", []corev1.ServicePort{port("http", 8481)}, "8481"},
		{"prefers http over the first port", []corev1.ServicePort{port("grpc", 8482), port("http", 8481)}, "8481"},
		{"prefers web when http is absent", []corev1.ServicePort{port("tcp-mesh", 9094), port("web", 9093)}, "9093"},
		{"falls back to the first TCP port", []corev1.ServicePort{port("mesh", 9094), port("other", 9095)}, "9094"},
		{"skips a UDP port", []corev1.ServicePort{
			{Name: "udp-mesh", Port: 9094, Protocol: corev1.ProtocolUDP},
			{Name: "tcp-mesh", Port: 9093, Protocol: corev1.ProtocolTCP},
		}, "9093"},
		{"skips a UDP port named http", []corev1.ServicePort{
			{Name: "http", Port: 9094, Protocol: corev1.ProtocolUDP},
			{Name: "web", Port: 9093, Protocol: corev1.ProtocolTCP},
		}, "9093"},
		{"skips a UDP port named web and takes the plain TCP port", []corev1.ServicePort{
			{Name: "web", Port: 9094, Protocol: corev1.ProtocolUDP},
			{Name: "mesh", Port: 9095, Protocol: corev1.ProtocolTCP},
		}, "9095"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := monitoringSvc("monitoring", "svc", "prometheus", tt.ports...)
			assert.Equal(t, tt.want, servicePort(svc))
		})
	}
}

// --- targetsFromServices ---

func TestTargetsFromServices(t *testing.T) {
	t.Run("maps vmselect to the tenant query prefixes", func(t *testing.T) {
		svcs := []corev1.Service{*monitoringSvc("monitoring", "vmselect-vmks", "vmselect", port("http", 8481))}

		prom, am := targetsFromServices(svcs)

		assert.Empty(t, am)
		require.Len(t, prom, 2)
		assert.Equal(t, monitoringTarget{Namespace: "monitoring", Service: "vmselect-vmks", Port: "8481", Prefix: "/select/0/prometheus"}, prom[0])
		assert.Equal(t, "/select/multitenant/prometheus", prom[1].Prefix)
	})

	t.Run("maps vmsingle to the root query path", func(t *testing.T) {
		svcs := []corev1.Service{*monitoringSvc("vm", "vmsingle-vmks", "vmsingle", port("http", 8428))}

		prom, _ := targetsFromServices(svcs)

		require.Len(t, prom, 1)
		assert.Equal(t, monitoringTarget{Namespace: "vm", Service: "vmsingle-vmks", Port: "8428", Prefix: ""}, prom[0])
	})

	t.Run("maps vmalertmanager to the alertmanager role", func(t *testing.T) {
		svcs := []corev1.Service{*monitoringSvc("monitoring", "vmalertmanager-vmks", "vmalertmanager",
			port("web", 9093), port("tcp-mesh", 9094))}

		prom, am := targetsFromServices(svcs)

		assert.Empty(t, prom)
		require.Len(t, am, 1)
		assert.Equal(t, monitoringTarget{Namespace: "monitoring", Service: "vmalertmanager-vmks", Port: "9093", Prefix: ""}, am[0])
	})

	t.Run("maps plain prometheus and alertmanager services", func(t *testing.T) {
		svcs := []corev1.Service{
			*monitoringSvc("monitoring", "kps-prometheus", "prometheus", port("http-web", 9090)),
			*monitoringSvc("monitoring", "kps-alertmanager", "alertmanager", port("http-web", 9093)),
		}

		prom, am := targetsFromServices(svcs)

		require.Len(t, prom, 1)
		require.Len(t, am, 1)
		assert.Equal(t, "kps-prometheus", prom[0].Service)
		assert.Equal(t, "9090", prom[0].Port)
		assert.Equal(t, "kps-alertmanager", am[0].Service)
	})

	t.Run("skips a service with an unknown label and one with no port", func(t *testing.T) {
		svcs := []corev1.Service{
			*monitoringSvc("monitoring", "grafana", "grafana", port("http", 3000)),
			*monitoringSvc("monitoring", "vmselect-broken", "vmselect"),
		}

		prom, am := targetsFromServices(svcs)

		assert.Empty(t, prom)
		assert.Empty(t, am)
	})
}

// --- discoverMonitoringServices ---

func TestDiscoverMonitoringServices(t *testing.T) {
	t.Run("finds services cluster-wide", func(t *testing.T) {
		cs := k8sfake.NewClientset(
			monitoringSvc("vm-system", "vmselect-vmks", "vmselect", port("http", 8481)),
			monitoringSvc("default", "web", "nginx", port("http", 80)),
		)

		prom, am := discoverMonitoringServices(t.Context(), cs, []string{"monitoring"})

		assert.Empty(t, am)
		require.Len(t, prom, 2)
		assert.Equal(t, "vm-system", prom[0].Namespace)
	})

	t.Run("falls back to the candidate namespaces when a cluster-wide list is denied", func(t *testing.T) {
		cs := k8sfake.NewClientset(monitoringSvc("monitoring", "vmsingle-vmks", "vmsingle", port("http", 8428)))
		cs.PrependReactor("list", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
			if action.GetNamespace() == "" {
				return true, nil, assert.AnError
			}
			return false, nil, nil
		})

		prom, _ := discoverMonitoringServices(t.Context(), cs, []string{"monitoring"})

		require.Len(t, prom, 1)
		assert.Equal(t, "vmsingle-vmks", prom[0].Service)
	})

	t.Run("returns nothing when every list fails", func(t *testing.T) {
		cs := k8sfake.NewClientset()
		cs.PrependReactor("list", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, assert.AnError
		})

		prom, am := discoverMonitoringServices(t.Context(), cs, []string{"monitoring"})

		assert.Empty(t, prom)
		assert.Empty(t, am)
	})
}

// --- monitoringTargetsFor ---

func TestMonitoringTargetsFor(t *testing.T) {
	t.Run("puts discovered targets ahead of the name guesses", func(t *testing.T) {
		resetMonitoringDiscoveryCache()
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = nil
		defer func() { model.ConfigMonitoring = origCfg }()

		cs := k8sfake.NewClientset(monitoringSvc("monitoring", "vmselect-vmks", "vmselect", port("http", 8481)))

		prom, am := monitoringTargetsFor(t.Context(), cs, "ctx-a")

		require.NotEmpty(t, prom)
		assert.Equal(t, "vmselect-vmks", prom[0].Service)
		assert.Equal(t, "8481", prom[0].Port)
		// The built-in guesses still follow, on the Prometheus default port.
		assert.Contains(t, targetServices(prom), "prometheus")
		assert.Contains(t, targetServices(am), "alertmanager")
	})

	t.Run("reuses the cached discovery for a second call", func(t *testing.T) {
		resetMonitoringDiscoveryCache()
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = nil
		defer func() { model.ConfigMonitoring = origCfg }()

		cs := k8sfake.NewClientset(monitoringSvc("monitoring", "vmsingle-vmks", "vmsingle", port("http", 8428)))
		lists := 0
		cs.PrependReactor("list", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
			lists++
			return false, nil, nil
		})

		monitoringTargetsFor(t.Context(), cs, "ctx-b")
		afterFirst := lists
		monitoringTargetsFor(t.Context(), cs, "ctx-b")

		assert.Equal(t, len(monitoringSelectors), afterFirst, "one list per selector")
		assert.Equal(t, afterFirst, lists, "the second call must list nothing")
	})

	t.Run("skips discovery when the config names services explicitly", func(t *testing.T) {
		resetMonitoringDiscoveryCache()
		origCfg := model.ConfigMonitoring
		model.ConfigMonitoring = map[string]model.MonitoringConfig{
			"_global": {
				Prometheus:   &model.MonitoringEndpoint{Services: []string{"my-prom"}},
				Alertmanager: &model.MonitoringEndpoint{Services: []string{"my-am"}},
			},
		}
		defer func() { model.ConfigMonitoring = origCfg }()

		cs := k8sfake.NewClientset(monitoringSvc("monitoring", "vmselect-vmks", "vmselect", port("http", 8481)))
		lists := 0
		cs.PrependReactor("list", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
			lists++
			return false, nil, nil
		})

		prom, am := monitoringTargetsFor(t.Context(), cs, "ctx-c")

		assert.Zero(t, lists)
		assert.NotEmpty(t, prom)
		assert.NotEmpty(t, am)
		for _, svc := range targetServices(prom) {
			assert.Equal(t, "my-prom", svc)
		}
		for _, svc := range targetServices(am) {
			assert.Equal(t, "my-am", svc)
		}
	})
}

func targetServices(targets []monitoringTarget) []string {
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		out = append(out, t.Service)
	}
	return out
}

// --- config path_prefix ---

func TestResolveMonitoringEndpointsPathPrefix(t *testing.T) {
	origCfg := model.ConfigMonitoring
	model.ConfigMonitoring = map[string]model.MonitoringConfig{
		"_global": {
			Prometheus: &model.MonitoringEndpoint{
				Namespaces: []string{"vm"},
				Services:   []string{"vmselect-vmks"},
				Port:       "8481",
				PathPrefix: "/select/0/prometheus",
			},
		},
	}
	defer func() { model.ConfigMonitoring = origCfg }()

	prom, _ := resolveMonitoringEndpoints("any-ctx")

	require.Len(t, prom, 1)
	assert.Equal(t, monitoringTarget{Namespace: "vm", Service: "vmselect-vmks", Port: "8481", Prefix: "/select/0/prometheus"}, prom[0])
	assert.Equal(t, "/select/0/prometheus/api/v1/query", prom[0].path("/api/v1/query"))
}

// --- resolveMonitoringEndpoints ---

func TestResolveMonitoringEndpoints(t *testing.T) {
	setConfig := func(t *testing.T, cfg map[string]model.MonitoringConfig) {
		t.Helper()
		orig := model.ConfigMonitoring
		model.ConfigMonitoring = cfg
		t.Cleanup(func() { model.ConfigMonitoring = orig })
	}

	assertDefaults := func(t *testing.T, prom, am []monitoringTarget) {
		t.Helper()
		assert.Contains(t, targetNamespaces(prom), "monitoring")
		assert.Contains(t, targetServices(prom), "prometheus")
		assert.Equal(t, "9090", prom[0].Port)
		assert.Contains(t, targetNamespaces(am), "monitoring")
		assert.Contains(t, targetServices(am), "alertmanager")
		assert.Equal(t, "9093", am[0].Port)
	}

	t.Run("returns defaults when ConfigMonitoring is nil", func(t *testing.T) {
		setConfig(t, nil)

		prom, am := resolveMonitoringEndpoints("test-ctx")

		assertDefaults(t, prom, am)
	})

	t.Run("uses context-specific config when available", func(t *testing.T) {
		setConfig(t, map[string]model.MonitoringConfig{
			"my-ctx": {
				Prometheus:   &model.MonitoringEndpoint{Namespaces: []string{"custom-ns"}, Services: []string{"custom-prom"}, Port: "8080"},
				Alertmanager: &model.MonitoringEndpoint{Namespaces: []string{"alerting"}, Services: []string{"custom-am"}, Port: "9999"},
			},
		})

		prom, am := resolveMonitoringEndpoints("my-ctx")

		assert.Equal(t, []monitoringTarget{{Namespace: "custom-ns", Service: "custom-prom", Port: "8080"}}, prom)
		assert.Equal(t, []monitoringTarget{{Namespace: "alerting", Service: "custom-am", Port: "9999"}}, am)
	})

	t.Run("falls back to _global config when context not found", func(t *testing.T) {
		setConfig(t, map[string]model.MonitoringConfig{
			"_global": {
				Prometheus: &model.MonitoringEndpoint{Namespaces: []string{"default-mon"}, Services: []string{"default-prom"}, Port: "7070"},
			},
		})

		prom, _ := resolveMonitoringEndpoints("unknown-ctx")

		assert.Equal(t, []monitoringTarget{{Namespace: "default-mon", Service: "default-prom", Port: "7070"}}, prom)
	})

	t.Run("returns defaults when neither context nor default config exists", func(t *testing.T) {
		setConfig(t, map[string]model.MonitoringConfig{"other-ctx": {}})

		prom, am := resolveMonitoringEndpoints("missing-ctx")

		assertDefaults(t, prom, am)
	})

	t.Run("partial config only overrides specified fields", func(t *testing.T) {
		setConfig(t, map[string]model.MonitoringConfig{
			"partial-ctx": {Prometheus: &model.MonitoringEndpoint{Port: "1234"}},
		})

		prom, am := resolveMonitoringEndpoints("partial-ctx")

		assert.Equal(t, "1234", prom[0].Port)
		assert.Contains(t, targetNamespaces(prom), "monitoring")
		assert.Contains(t, targetServices(prom), "prometheus")
		assert.Contains(t, targetNamespaces(am), "monitoring")
		assert.Contains(t, targetServices(am), "alertmanager")
		assert.Equal(t, "9093", am[0].Port)
	})
}

func targetNamespaces(targets []monitoringTarget) []string {
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		out = append(out, t.Namespace)
	}
	return out
}

func resetMonitoringDiscoveryCache() {
	monitoringDiscoveryCache.Range(func(k, _ any) bool {
		monitoringDiscoveryCache.Delete(k)
		return true
	})
	promSvcCache.Range(func(k, _ any) bool {
		promSvcCache.Delete(k)
		return true
	})
}

func TestMonitoringTargetsForDoesNotMutateTheCache(t *testing.T) {
	resetMonitoringDiscoveryCache()
	orig := model.ConfigMonitoring
	model.ConfigMonitoring = nil
	t.Cleanup(func() { model.ConfigMonitoring = orig })

	cs := k8sfake.NewClientset(monitoringSvc("monitoring", "vmselect-vmks", "vmselect", port("http", 8481)))

	first, _ := monitoringTargetsFor(t.Context(), cs, "ctx-d")
	monitoringTargetsFor(t.Context(), cs, "ctx-d")
	second, _ := monitoringTargetsFor(t.Context(), cs, "ctx-d")

	assert.Equal(t, first, second)
	assert.Equal(t, "vmselect-vmks", second[0].Service)
	assert.Equal(t, "vmselect-vmks", second[1].Service)
}

// --- MonitoringSearchHint ---

func TestMonitoringSearchHint(t *testing.T) {
	setConfig := func(t *testing.T, cfg map[string]model.MonitoringConfig) {
		t.Helper()
		orig := model.ConfigMonitoring
		model.ConfigMonitoring = cfg
		t.Cleanup(func() { model.ConfigMonitoring = orig })
	}

	t.Run("names the labels when discovery runs", func(t *testing.T) {
		setConfig(t, nil)

		hint := strings.Join(MonitoringSearchHint("ctx"), " ")

		assert.Contains(t, hint, "vmselect")
		assert.Contains(t, hint, "kube-prometheus-stack")
	})

	t.Run("names the configured namespaces when discovery runs", func(t *testing.T) {
		setConfig(t, map[string]model.MonitoringConfig{
			"_global": {
				Prometheus:   &model.MonitoringEndpoint{Namespaces: []string{"obs-ns"}},
				Alertmanager: &model.MonitoringEndpoint{Namespaces: []string{"obs-ns"}},
			},
		})

		hint := strings.Join(MonitoringSearchHint("ctx"), " ")

		assert.Contains(t, hint, "obs-ns")
		assert.NotContains(t, hint, "kube-prometheus-stack")
	})

	t.Run("names every namespace it searched when only one role is configured", func(t *testing.T) {
		setConfig(t, map[string]model.MonitoringConfig{
			"_global": {Prometheus: &model.MonitoringEndpoint{Namespaces: []string{"obs-ns"}}},
		})

		hint := strings.Join(MonitoringSearchHint("ctx"), " ")

		assert.Contains(t, hint, "obs-ns")
		assert.Contains(t, hint, "kube-prometheus-stack")
	})

	t.Run("ties the namespace list to the name probing, not to discovery", func(t *testing.T) {
		setConfig(t, nil)

		hint := MonitoringSearchHint("ctx")

		assert.Contains(t, strings.Join(hint, " "), "Probed those names in: monitoring")
		// The label search covers the cluster, so it must not claim the list bounds it.
		assert.Contains(t, hint[0], "Searched the cluster")
	})

	t.Run("names a service configured for one role while discovery still runs", func(t *testing.T) {
		setConfig(t, map[string]model.MonitoringConfig{
			"_global": {Prometheus: &model.MonitoringEndpoint{Services: []string{"thanos-query"}}},
		})

		hint := strings.Join(MonitoringSearchHint("ctx"), " ")

		// The configured name is one lfk actually tried, so the hint must say it.
		assert.Contains(t, hint, "thanos-query")
		// Discovery still runs for the alertmanager role.
		assert.Contains(t, hint, "vmselect")
	})

	t.Run("says discovery is off when the config names both roles", func(t *testing.T) {
		setConfig(t, map[string]model.MonitoringConfig{
			"_global": {
				Prometheus:   &model.MonitoringEndpoint{Services: []string{"my-prom"}},
				Alertmanager: &model.MonitoringEndpoint{Services: []string{"my-am"}},
			},
		})

		hint := strings.Join(MonitoringSearchHint("ctx"), " ")

		assert.Contains(t, hint, "my-prom")
		assert.Contains(t, hint, "my-am")
		assert.NotContains(t, hint, "vmselect")
	})
}

// --- serviceRole ---

func TestServiceRole(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   monitoringRole
	}{
		{"vm operator select", map[string]string{appNameLabel: "vmselect"}, roleVMSelect},
		{"vm operator single", map[string]string{appNameLabel: "vmsingle"}, rolePrometheus},
		{"vm operator alertmanager", map[string]string{appNameLabel: "vmalertmanager"}, roleAlertmanager},
		{"chart name label", map[string]string{appNameLabel: "prometheus"}, rolePrometheus},
		{"operator governing prometheus", map[string]string{"operated-prometheus": "true"}, rolePrometheus},
		{"operator governing alertmanager", map[string]string{"operated-alertmanager": "true"}, roleAlertmanager},
		{"operator label set to false", map[string]string{"operated-prometheus": "false"}, roleNone},
		{"unrelated service", map[string]string{"app": "nginx"}, roleNone},
		{"no labels", nil, roleNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &corev1.Service{Labels: tt.labels}
			assert.Equal(t, tt.want, serviceRole(svc))
		})
	}
}

// TestDiscoverKubePrometheusStack covers the labels a real kube-prometheus-stack
// install carries. Its Prometheus Service has no app.kubernetes.io/name at all,
// so only the operator's own governing-service label finds it.
func TestDiscoverKubePrometheusStack(t *testing.T) {
	promOperated := monitoringSvc("monitoring", "prometheus-operated", "", port("web", 9090))
	promOperated.Labels = map[string]string{"operated-prometheus": "true"}
	amOperated := monitoringSvc("monitoring", "alertmanager-operated", "", port("web", 9093))
	amOperated.Labels = map[string]string{"operated-alertmanager": "true"}
	// The chart's own Service, which carries no label discovery matches on.
	chartProm := monitoringSvc("monitoring", "monitoring-kube-prometheus-prometheus", "", port("http-web", 9090))
	chartProm.Labels = map[string]string{"app": "kube-prometheus-stack-prometheus"}

	cs := k8sfake.NewClientset(promOperated, amOperated, chartProm)

	prom, am := discoverMonitoringServices(t.Context(), cs, []string{"monitoring"})

	require.Len(t, prom, 1)
	assert.Equal(t, "prometheus-operated", prom[0].Service)
	assert.Equal(t, "9090", prom[0].Port)
	require.Len(t, am, 1)
	assert.Equal(t, "alertmanager-operated", am[0].Service)
	assert.Equal(t, "9093", am[0].Port)
}

func TestDiscoverMonitoringServicesDedupes(t *testing.T) {
	// A Service that matches two selectors must yield one target, not two.
	svc := monitoringSvc("monitoring", "prometheus-operated", "prometheus", port("web", 9090))
	svc.Labels["operated-prometheus"] = "true"

	prom, _ := discoverMonitoringServices(t.Context(), k8sfake.NewClientset(svc), []string{"monitoring"})

	assert.Len(t, prom, 1)
}

// TestDiscoveryPrefersWellKnownNamespaces guards the order discovery hands
// back. Any user who can create a Service can carry the labels discovery
// matches, so a Service in a namespace an operator normally owns is probed
// before one that merely appeared somewhere else.
func TestDiscoveryPrefersWellKnownNamespaces(t *testing.T) {
	cs := k8sfake.NewClientset(
		monitoringSvc("apps", "vmsingle-squatter", "vmsingle", port("http", 8428)),
		monitoringSvc("monitoring", "vmsingle-real", "vmsingle", port("http", 8428)),
	)

	prom, _ := discoverMonitoringServices(t.Context(), cs, defaultMonitoringNamespaces)

	require.Len(t, prom, 2)
	assert.Equal(t, "vmsingle-real", prom[0].Service, "the well-known namespace is probed first")
	assert.Equal(t, "vmsingle-squatter", prom[1].Service)
}

func TestProbePrometheusTargetsNamesEveryTargetOnFailure(t *testing.T) {
	targets := []monitoringTarget{
		{Namespace: "monitoring", Service: "vmselect-vmks", Port: "8481", Prefix: vmSelectTenantPrefix},
		{Namespace: "monitoring", Service: "prometheus-operated", Port: "9090"},
	}
	calls := 0
	_, _, err := probePrometheusTargets(targets, func(monitoringTarget) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("404 page not found")
		}
		return nil, errors.New("the server could not find the requested resource")
	})
	require.Error(t, err)
	assert.Equal(t, 2, calls)
	// The user reads this line in the log, so it has to name what was
	// tried and what each answered, not only the last guess.
	assert.Contains(t, err.Error(), "monitoring/vmselect-vmks:8481/select/0/prometheus: 404 page not found")
	assert.Contains(t, err.Error(), "monitoring/prometheus-operated:9090: the server could not find the requested resource")
	assert.Contains(t, err.Error(), "tried 2 targets")
}

func TestProbePrometheusTargetsReturnsTheFirstHit(t *testing.T) {
	targets := []monitoringTarget{
		{Namespace: "monitoring", Service: "a", Port: "9090"},
		{Namespace: "monitoring", Service: "b", Port: "9090"},
	}
	data, hit, err := probePrometheusTargets(targets, func(t monitoringTarget) ([]byte, error) {
		if t.Service == "b" {
			return []byte("ok"), nil
		}
		return nil, errors.New("nope")
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", string(data))
	assert.Equal(t, "b", hit.Service)
}

func TestProbePrometheusTargetsWithNothingToTry(t *testing.T) {
	_, _, err := probePrometheusTargets(nil, func(monitoringTarget) ([]byte, error) {
		t.Fatal("must not be called")
		return nil, nil
	})
	require.ErrorContains(t, err, "no prometheus service found")
}
