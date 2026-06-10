package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A kubeconfig context name is arbitrary user/CI-controlled text — kubectl does
// not forbid shell metacharacters. The helm edit/upgrade scripts run via
// `sh -c`, so the context and kubeconfig path must reach the shell as
// environment variables, never interpolated into the script body where
// `$(...)`/backticks would execute. See issue: helm sh -c context injection.
func TestHelmEditCmdDoesNotInterpolateContext(t *testing.T) {
	const evilCtx = `prod"$(touch /tmp/lfk-pwned)"`
	const kubeconfig = `/home/u/.kube/config"$(id)"`

	cmd := buildHelmEditCmd("/usr/bin/helm", "my-release", "default", evilCtx, kubeconfig)

	require.Equal(t, []string{"sh", "-c"}, cmd.Args[:2])
	script := cmd.Args[2]
	assert.NotContains(t, script, evilCtx,
		"context name must not be interpolated into the sh -c script body")
	assert.NotContains(t, script, kubeconfig,
		"kubeconfig path must not be interpolated into the sh -c script body")

	assert.Contains(t, cmd.Env, "CTX="+evilCtx,
		"context must be passed as a discrete environment variable")
	assert.Contains(t, cmd.Env, "KUBECONFIG="+kubeconfig,
		"kubeconfig must be passed as a discrete environment variable")
}

func TestHelmUpgradeCmdDoesNotInterpolateContext(t *testing.T) {
	const evilCtx = `prod"$(touch /tmp/lfk-pwned)"`
	const kubeconfig = `/home/u/.kube/config"$(id)"`

	cmd := buildHelmUpgradeCmd("/usr/bin/helm", "my-release", "default", evilCtx, kubeconfig)

	require.Equal(t, []string{"sh", "-c"}, cmd.Args[:2])
	script := cmd.Args[2]
	assert.NotContains(t, script, evilCtx,
		"context name must not be interpolated into the sh -c script body")
	assert.NotContains(t, script, kubeconfig,
		"kubeconfig path must not be interpolated into the sh -c script body")

	assert.Contains(t, cmd.Env, "CTX="+evilCtx)
	assert.Contains(t, cmd.Env, "KUBECONFIG="+kubeconfig)
}

// The script must reference the values through shell variables so the env
// hand-off actually drives the helm invocation.
func TestHelmScriptsReferenceEnvVars(t *testing.T) {
	edit := buildHelmEditCmd("/usr/bin/helm", "r", "ns", "ctx", "/cfg").Args[2]
	upgrade := buildHelmUpgradeCmd("/usr/bin/helm", "r", "ns", "ctx", "/cfg").Args[2]
	for _, ref := range []string{"$HELM", "$RELEASE", "$NS", "$CTX"} {
		assert.Contains(t, edit, ref, "edit script must use %s", ref)
		assert.Contains(t, upgrade, ref, "upgrade script must use %s", ref)
	}
}

// Release and namespace are also handed off via env (defense in depth even
// though they are DNS-constrained in practice).
func TestHelmEnvCarriesAllValues(t *testing.T) {
	cmd := buildHelmEditCmd("/usr/bin/helm", "rel", "myns", "ctx", "/cfg")
	for _, kv := range []string{"HELM=/usr/bin/helm", "RELEASE=rel", "NS=myns", "CTX=ctx"} {
		assert.Contains(t, cmd.Env, kv)
	}
}
