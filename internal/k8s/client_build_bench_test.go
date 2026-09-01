package k8s

import (
	"os"
	"path/filepath"
	"testing"
)

// benchKubeconfig is a minimal two-context kubeconfig: "plain" uses a static
// token; "exec" uses an exec credential provider (the EKS aws / kubelogin
// shape) so we can measure the extra cost client-go's exec plugin machinery
// adds to client construction. The exec command is "true" so it never blocks
// — we are measuring lfk's build path, not the plugin's runtime.
const benchKubeconfigPlain = `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://127.0.0.1:6443
    insecure-skip-tls-verify: true
contexts:
- name: plain
  context:
    cluster: c
    user: u
current-context: plain
users:
- name: u
  user:
    token: deadbeef
`

const benchKubeconfigExec = `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://127.0.0.1:6443
    insecure-skip-tls-verify: true
contexts:
- name: exec
  context:
    cluster: c
    user: u
current-context: exec
users:
- name: u
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: "true"
      args: []
`

// newBenchClient writes the given kubeconfig to a temp file and builds a real
// Client against it (no fakes), so the *ForContext builders exercise the full
// rest.Config + transport + clientset construction path.
func newBenchClient(b *testing.B, kubeconfig string) *Client {
	b.Helper()
	dir := b.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(kubeconfig), 0o600); err != nil {
		b.Fatalf("write kubeconfig: %v", err)
	}
	c, err := NewClient(path, nil, true, nil)
	if err != nil {
		b.Fatalf("NewClient: %v", err)
	}
	return c
}

// BenchmarkClientsetForContext_Plain measures a full per-call clientset build
// for a static-token context (the current per-call behavior — no cache).
func BenchmarkClientsetForContext_Plain(b *testing.B) {
	c := newBenchClient(b, benchKubeconfigPlain)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		cs, err := c.clientsetForContext("plain")
		if err != nil || cs == nil {
			b.Fatalf("clientsetForContext: %v", err)
		}
	}
}

// BenchmarkClientsetForContext_Exec measures the same for an exec-credential
// (EKS aws / kubelogin) context, where client-go wires up the exec plugin
// transport on every build.
func BenchmarkClientsetForContext_Exec(b *testing.B) {
	c := newBenchClient(b, benchKubeconfigExec)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		cs, err := c.clientsetForContext("exec")
		if err != nil || cs == nil {
			b.Fatalf("clientsetForContext: %v", err)
		}
	}
}

// BenchmarkRestConfigForContext isolates just the kubeconfig parse + rest.Config
// build (the part a config-only cache would eliminate), excluding transport and
// clientset construction.
func BenchmarkRestConfigForContext(b *testing.B) {
	c := newBenchClient(b, benchKubeconfigPlain)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		cfg, err := c.restConfigForContext("plain")
		if err != nil || cfg == nil {
			b.Fatalf("restConfigForContext: %v", err)
		}
	}
}

// BenchmarkDynamicForContext_Plain measures the dynamic-client build path.
func BenchmarkDynamicForContext_Plain(b *testing.B) {
	c := newBenchClient(b, benchKubeconfigPlain)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		dc, err := c.dynamicForContext("plain")
		if err != nil || dc == nil {
			b.Fatalf("dynamicForContext: %v", err)
		}
	}
}

// BenchmarkCachedClientsetReuse is the lower bound: build once, then reuse the
// same clientset N times. The delta between this and BenchmarkClientsetForContext_*
// is exactly what a per-context client cache would save per call.
func BenchmarkCachedClientsetReuse(b *testing.B) {
	c := newBenchClient(b, benchKubeconfigPlain)
	cs, err := c.clientsetForContext("plain")
	if err != nil || cs == nil {
		b.Fatalf("warmup clientsetForContext: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = cs.Discovery()
	}
}
