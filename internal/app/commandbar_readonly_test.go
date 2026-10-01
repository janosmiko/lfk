package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestCommandBarReadOnly_Kubectl(t *testing.T) {
	cases := []struct {
		input   string
		blocked bool
	}{
		{"k delete pod x", true},
		{"k apply -f x.yaml", true},
		{"k edit deploy x", true},
		{"k scale deploy x --replicas=0", true},
		{"kubectl delete pod x", true},
		{"k -n ns delete pod x", true},
		{"k --namespace=ns delete pod x", true},
		{"k --context test-ctx delete pod x", true},
		{"k exec -it x -- sh", true},
		{"k config use-context other", true},
		{"k auth reconcile -f x", true},
		{"k rollout restart deploy x", true},
		{"k rollout undo deploy x", true},
		{"k -n ns rollout pause deploy x", true},
		{"k somepluginverb", true},
		{"k --as-user-extra get delete pod x", true},
		{"k --kuberc get delete pod x", true},
		{"k -v get delete pod x", true},
		{"k --v get delete pod x", true},
		{"k --vmodule get delete pod x", true},
		{"k --log-flush-frequency get delete pod x", true},
		{"k --made-up-flag get delete pod x", true},
		{"k --made-up-flag=1 get pods", true},
		{"k -v=6 get pods", false},
		{"k --v=6 get pods", false},
		{"k -v 6 get pods", false},
		{"k --insecure-skip-tls-verify get pods", false},
		{"k --as-user-extra a=b get pods", false},
		{"k get pods -o yaml", false},
		{"k get pods -l a=b", false},
		{"k get pods --all-namespaces", false},
		{"k get pods -A", false},
		{"k get pods -w", false},
		{"k get pods -n ns -o wide", false},
		{"k -n ns", true},
		{"k get pods", false},
		{"k -n ns get pods", false},
		{"k --namespace=ns describe pod x", false},
		{"k logs x", false},
		{"k top pods", false},
		{"k explain pod", false},
		{"k api-resources", false},
		{"k api-versions", false},
		{"k version", false},
		{"k cluster-info", false},
		{"k events", false},
		{"k diff -f x.yaml", false},
		{"k auth can-i get pods", false},
		{"k config view", false},
		{"k config get-contexts", false},
		{"k config current-context", false},
		{"k rollout status deploy x", false},
		{"k -n ns rollout history deploy x", false},
	}
	for _, ro := range []string{"cli", "ctx", "none"} {
		for _, tc := range cases {
			t.Run(ro+"/"+tc.input, func(t *testing.T) {
				m := fallbackModel(t, writeKubeconfigFile(t, "a", "test-ctx", "test-ctx"))
				m.nav.Context = "test-ctx"
				switch ro {
				case "cli":
					m.cliReadOnly = true
					m.readOnly = true
				case "ctx":
					m.readOnly = true
				}
				result, cmd := m.executeCommandBarInput(tc.input)
				rm := result.(Model)
				if ro != "none" && tc.blocked {
					assert.True(t, rm.statusMessageErr)
					assert.Contains(t, rm.statusMessage, "Read-only mode")
					return
				}
				assert.NotNil(t, cmd)
				assert.Empty(t, rm.statusMessage)
			})
		}
	}
}

func TestCommandBarReadOnly_ExplicitContext(t *testing.T) {
	prev := ui.ConfigClusterReadOnly
	t.Cleanup(func() { ui.ConfigClusterReadOnly = prev })
	ui.ConfigClusterReadOnly = map[string]bool{"prod": true}

	t.Run("writable_current_read_only_target", func(t *testing.T) {
		for _, in := range []string{"k --context prod delete pod x", "k --context=prod delete pod x"} {
			m := baseModelWithFakeClient()
			r, _ := m.executeCommandBarInput(in)
			assert.Contains(t, r.(Model).statusMessage, "Read-only mode", in)
		}
	})
	t.Run("read_only_current_writable_target", func(t *testing.T) {
		m := baseModelWithFakeClient()
		m.readOnly = true
		r, _ := m.executeCommandBarInput("k --context dev delete pod x")
		assert.Contains(t, r.(Model).statusMessage, "Read-only mode")
	})
	t.Run("read_only_target_read_verb_runs", func(t *testing.T) {
		m := baseModelWithFakeClient()
		r, cmd := m.executeCommandBarInput("k --context prod get pods")
		assert.NotNil(t, cmd)
		assert.Empty(t, r.(Model).statusMessage)
	})
	t.Run("any_read_only_candidate_blocks", func(t *testing.T) {
		for _, tc := range []struct {
			in      string
			blocked bool
		}{
			{"k delete pod x --context=dev --context=prod", true},
			{"k delete pod x --context dev --context prod", true},
			{"k delete pod x --context dev --context=prod", true},
			{"k delete pod x --context=prod --context=dev", true},
			{"k delete pod x --context prod --context dev", true},
			{"k delete pod x -- --context=prod", true},
			{"k --user --context prod delete pod x", true},
			{"k --context= delete pod x", true},
			{"k delete pod x --context", true},
			{"k delete pod x --context=dev --context=", true},
			{"k delete pod x --context=dev --context=stage", false},
			{"k get pods --context=prod", false},
		} {
			m := baseModelWithFakeClient()
			r, _ := m.executeCommandBarInput(tc.in)
			assert.Equal(t, tc.blocked, strings.Contains(r.(Model).statusMessage, "Read-only mode"), tc.in)
		}
	})
	t.Run("session_override_unlocks_target", func(t *testing.T) {
		m := baseModelWithFakeClient()
		m.contextROOverrides = map[string]bool{"prod": false}
		r, cmd := m.executeCommandBarInput("k --context prod delete pod x")
		assert.NotNil(t, cmd)
		assert.Empty(t, r.(Model).statusMessage)
	})
}

func TestCommandBarReadOnly_Shell(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cli, ro bool
		blocked bool
	}{
		{"cli_flag", true, true, true},
		{"context", false, true, true},
		{"writable", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := baseModelWithFakeClient()
			m.cliReadOnly, m.readOnly = tc.cli, tc.ro
			result, cmd := m.executeCommandBarInput("! kubectl delete pod x")
			rm := result.(Model)
			if tc.blocked {
				assert.True(t, rm.statusMessageErr)
				assert.Equal(t, readOnlyBlockedMessage("shell commands"), rm.statusMessage)
				return
			}
			assert.NotNil(t, cmd)
			assert.Empty(t, rm.statusMessage)
		})
	}
}

func TestInjectKubectlDefaults_BuiltinVerbPinsContextAtFront(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "my-ctx"
	for _, args := range [][]string{
		{"get", "pods"},
		{"delete", "pod", "x", "--", "--context=dev"},
		{"--user", "--context", "delete", "pod", "x"},
		{"--context=", "delete", "pod", "x"},
		{"--context", "other", "get", "pods"},
	} {
		got := m.injectKubectlDefaults(args)
		assert.Equal(t, []string{"--context", "my-ctx"}, got[:2], args)
		assert.Equal(t, args, got[2:2+len(args)], "user args follow the pin in order")
	}
}

func TestInjectKubectlDefaults_PluginVerbAppendsContext(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "my-ctx"
	for _, args := range [][]string{
		{"krew", "list"},
		{"-n", "ns", "foo", "bar"},
		{"somepluginverb"},
	} {
		got := m.injectKubectlDefaults(args)
		assert.Equal(t, args, got[:len(args)], "plugin name stays first")
		assert.Equal(t, []string{"--context", "my-ctx"}, got[len(got)-2:], args)
	}
}

func TestCommandBarReadOnly_SubcommandMustFollowVerb(t *testing.T) {
	for _, in := range []string{
		"k rollout -l status restart deployment",
		"k rollout -l status pause deployment x",
		"k rollout -l status undo deployment x",
		"k auth -f can-i reconcile",
		"k config -x view set-context foo",
		"k rollout -n ns status deploy x",
	} {
		m := baseModelWithFakeClient()
		m.readOnly = true
		r, _ := m.executeCommandBarInput(in)
		assert.Contains(t, r.(Model).statusMessage, "Read-only mode", in)
	}
}

func TestCommandBarReadOnly_EmptyNavContextUsesKubeconfigCurrent(t *testing.T) {
	prev := ui.ConfigClusterReadOnly
	t.Cleanup(func() { ui.ConfigClusterReadOnly = prev })

	t.Run("current_context_read_only_blocks", func(t *testing.T) {
		ui.ConfigClusterReadOnly = map[string]bool{"test-ctx": true}
		m := baseModelWithFakeClient()
		m.nav.Context = ""
		r, _ := m.executeCommandBarInput("k delete pod x")
		assert.Contains(t, r.(Model).statusMessage, "Read-only mode")
	})
	t.Run("current_context_writable_runs", func(t *testing.T) {
		m := fallbackModel(t, writeKubeconfigFile(t, "a", "test-ctx", "test-ctx"))
		ui.ConfigClusterReadOnly = nil
		m.nav.Context = ""
		r, cmd := m.executeCommandBarInput("k delete pod x")
		assert.NotNil(t, cmd)
		assert.Empty(t, r.(Model).statusMessage)
	})
	t.Run("read_verb_runs_even_when_read_only", func(t *testing.T) {
		ui.ConfigClusterReadOnly = map[string]bool{"test-ctx": true}
		m := baseModelWithFakeClient()
		m.nav.Context = ""
		r, cmd := m.executeCommandBarInput("k get pods")
		assert.NotNil(t, cmd)
		assert.Empty(t, r.(Model).statusMessage)
	})
	t.Run("unknown_target_blocks", func(t *testing.T) {
		ui.ConfigClusterReadOnly = nil
		m := baseModelWithFakeClient()
		m.nav.Context = ""
		m.client = nil
		assert.True(t, blockedFor(m, []string{"delete", "pod", "x"}))
	})
}

func blockedFor(m Model, args []string) bool {
	_, blocked := m.kubectlBlockedByReadOnly(args)
	return blocked
}

func TestCommandBarReadOnly_ShellAtPickerUsesKubeconfigCurrent(t *testing.T) {
	prev := ui.ConfigClusterReadOnly
	t.Cleanup(func() { ui.ConfigClusterReadOnly = prev })
	ui.ConfigClusterReadOnly = map[string]bool{"test-ctx": true}

	m := baseModelWithFakeClient()
	m.nav.Context = ""
	r, _ := m.executeCommandBarInput("! kubectl delete pod x")
	assert.Equal(t, readOnlyBlockedMessage("shell commands"), r.(Model).statusMessage)
}

func TestCommandBarReadOnly_DisplayNameAlias(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 0, 2)
	for _, f := range []string{"a", "b"} {
		p := filepath.Join(dir, f)
		require.NoError(t, os.WriteFile(p, []byte(fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: dev
clusters:
- name: c
  cluster:
    server: https://%s.example.test:6443
contexts:
- name: dev
  context:
    cluster: c
    user: u
users:
- name: u
  user:
    token: t
`, f)), 0o600))
		paths = append(paths, p)
	}
	t.Setenv("KUBECONFIG", strings.Join(paths, string(os.PathListSeparator)))
	client, err := k8s.NewClient("", nil, true, nil)
	require.NoError(t, err)

	prev := ui.ConfigClusterReadOnly
	t.Cleanup(func() { ui.ConfigClusterReadOnly = prev })
	ui.ConfigClusterReadOnly = map[string]bool{"dev (b)": true}

	m := baseModelWithFakeClient()
	m.client = client
	m.nav.Context = "other"
	assert.True(t, blockedFor(m, []string{"delete", "pod", "x", "--context", "dev"}))
	assert.True(t, blockedFor(m, []string{"delete", "pod", "x", "--context=dev"}))

	ui.ConfigClusterReadOnly = map[string]bool{"unrelated": true}
	assert.False(t, blockedFor(m, []string{"delete", "pod", "x", "--context", "dev"}))
}

func TestInjectKubectlDefaults_PluginVerbKeepsUserContext(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "my-ctx"
	for _, args := range [][]string{
		{"foo", "--context", "other"},
		{"foo", "--context=other"},
	} {
		assert.Equal(t, args, m.injectKubectlDefaults(args), args)
	}
	got := m.injectKubectlDefaults([]string{"get", "pods", "--context=other"})
	assert.Equal(t, []string{"--context", "my-ctx"}, got[:2], "built-in verbs keep the front pin")
}

func writeKubeconfigFile(t *testing.T, name, current string, contexts ...string) string {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: v1\nkind: Config\ncurrent-context: %s\nclusters:\n- name: c\n  cluster:\n    server: https://x.example.test:6443\ncontexts:\n", current)
	if len(contexts) == 0 {
		b.Reset()
		fmt.Fprintf(&b, "apiVersion: v1\nkind: Config\ncurrent-context: %s\n", current)
	}
	for _, c := range contexts {
		fmt.Fprintf(&b, "- name: %s\n  context:\n    cluster: c\n    user: u\n", c)
	}
	b.WriteString("users:\n- name: u\n  user:\n    token: t\n")
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, []byte(b.String()), 0o600))
	return p
}

func fallbackModel(t *testing.T, paths ...string) Model {
	t.Helper()
	t.Setenv("KUBECONFIG", strings.Join(paths, string(os.PathListSeparator)))
	client, err := k8s.NewClient("", nil, true, nil)
	require.NoError(t, err)
	prev := ui.ConfigClusterReadOnly
	t.Cleanup(func() { ui.ConfigClusterReadOnly = prev })
	ui.ConfigClusterReadOnly = map[string]bool{"ro": true}
	m := baseModelWithFakeClient()
	m.client = client
	return m
}

func TestCommandBarReadOnly_PluginUsesFallbackContext(t *testing.T) {
	plugins := [][]string{
		{"fake", "x"},
		{"fake", "--", "x"},
		{"fake", "--name", "--context", "rw"},
	}
	t.Run("read_only_fallback_blocks", func(t *testing.T) {
		m := fallbackModel(t, writeKubeconfigFile(t, "a", "ro", "rw", "ro"))
		m.nav.Context = "rw"
		for _, args := range plugins {
			assert.True(t, blockedFor(m, args), args)
		}
		assert.False(t, blockedFor(m, []string{"get", "pods"}))
		assert.False(t, blockedFor(m, []string{"delete", "pod", "x"}), "built-in verb is pinned to the writable nav")
	})
	t.Run("writable_fallback_allows", func(t *testing.T) {
		m := fallbackModel(t, writeKubeconfigFile(t, "a", "rw", "rw", "ro"))
		m.nav.Context = "rw"
		for _, args := range plugins {
			assert.False(t, blockedFor(m, args), args)
		}
	})
	t.Run("unreadable_fallback_blocks", func(t *testing.T) {
		m := fallbackModel(t, writeKubeconfigFile(t, "a", "rw", "rw"))
		m.nav.Context = "rw"
		require.NoError(t, os.Remove(m.client.KubeconfigPathForContext("rw")))
		assert.True(t, blockedFor(m, []string{"fake", "x"}))
	})
}

func TestCommandBarReadOnly_PickerUsesSelectedFileCurrentContext(t *testing.T) {
	// Merged current-context is "dev" (writable), but the file that defines
	// "dev" is the one kubectl reads, and its current-context is "ro".
	a := writeKubeconfigFile(t, "a", "dev")
	b := writeKubeconfigFile(t, "b", "ro", "dev", "ro")
	m := fallbackModel(t, a, b)
	m.nav.Context = ""
	require.Equal(t, "dev", m.client.CurrentContext())
	assert.True(t, blockedFor(m, []string{"delete", "pod", "x"}))
}

func TestCommandBarReadOnly_NoFallbackContextUsesGlobalFlags(t *testing.T) {
	plugin := []string{"fake", "x"}
	cases := []struct {
		name  string
		model func(t *testing.T) Model
	}{
		{"file_without_current_context", func(t *testing.T) Model {
			m := fallbackModel(t, writeKubeconfigFile(t, "a", "", "rw"))
			m.nav.Context = "rw"
			return m
		}},
		{"empty_path", func(t *testing.T) Model {
			m := baseModelWithFakeClient()
			m.client = &k8s.Client{}
			require.Empty(t, m.client.KubeconfigPathForContext("test-ctx"))
			m.nav.Context = "test-ctx"
			return m
		}},
		{"picker_without_current_context", func(t *testing.T) Model {
			m := fallbackModel(t, writeKubeconfigFile(t, "a", "", "rw"))
			require.Empty(t, m.client.CurrentContext())
			m.nav.Context = ""
			return m
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/writable", func(t *testing.T) {
			m := tc.model(t)
			assert.False(t, blockedFor(m, plugin))
			assert.False(t, blockedFor(m, []string{"delete", "pod", "x"}))
			r, cmd := m.executeCommandBarInput("! kubectl delete pod x")
			assert.NotNil(t, cmd)
			assert.Empty(t, r.(Model).statusMessage)
		})
		t.Run(tc.name+"/cli_read_only", func(t *testing.T) {
			m := tc.model(t)
			m.cliReadOnly = true
			assert.True(t, blockedFor(m, plugin))
			assert.True(t, blockedFor(m, []string{"delete", "pod", "x"}))
			r, _ := m.executeCommandBarInput("! kubectl delete pod x")
			assert.Equal(t, readOnlyBlockedMessage("shell commands"), r.(Model).statusMessage)
		})
	}
}

func TestCommandBarReadOnly_PickerRereadsKubeconfigCurrentContext(t *testing.T) {
	for _, tc := range []struct {
		current string
		blocked bool
	}{{"ro", true}, {"rw", false}} {
		t.Run(tc.current, func(t *testing.T) {
			path := writeKubeconfigFile(t, "a", "", "rw", "ro")
			m := fallbackModel(t, path)
			m.nav.Context = ""
			require.Empty(t, m.client.CurrentContext())
			// Another terminal sets current-context after lfk loaded the file.
			require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf(
				"apiVersion: v1\nkind: Config\ncurrent-context: %s\nclusters:\n- name: c\n  cluster:\n    server: https://x.example.test:6443\ncontexts:\n- name: rw\n  context:\n    cluster: c\n    user: u\n- name: ro\n  context:\n    cluster: c\n    user: u\nusers:\n- name: u\n  user:\n    token: t\n", tc.current)), 0o600))
			assert.Equal(t, tc.blocked, blockedFor(m, []string{"delete", "pod", "x"}))
			r, cmd := m.executeCommandBarInput("! kubectl delete pod x")
			if tc.blocked {
				assert.Equal(t, readOnlyBlockedMessage("shell commands"), r.(Model).statusMessage)
				return
			}
			assert.NotNil(t, cmd)
			assert.Empty(t, r.(Model).statusMessage)
		})
	}
}

func TestCommandBarReadOnly_NoFallbackContextHonorsGlobalConfig(t *testing.T) {
	prev := ui.ConfigReadOnly
	t.Cleanup(func() { ui.ConfigReadOnly = prev })
	ui.ConfigReadOnly = true

	m := baseModelWithFakeClient()
	m.client = &k8s.Client{}
	m.nav.Context = ""
	assert.True(t, blockedFor(m, []string{"delete", "pod", "x"}))
	assert.True(t, blockedFor(m, []string{"fake", "x"}))
}

func TestCommandBarReadOnly_LoadErrorStillBlocks(t *testing.T) {
	m := fallbackModel(t, writeKubeconfigFile(t, "a", "rw", "rw"))
	m.nav.Context = ""
	require.NoError(t, os.Remove(m.client.KubeconfigPathForContext("rw")))
	assert.True(t, blockedFor(m, []string{"fake", "x"}))
	assert.True(t, blockedFor(m, []string{"delete", "pod", "x"}))
}

func TestKubectlBlockedByReadOnly_HelpAndOptionsAllowed(t *testing.T) {
	m := baseModelWithFakeClient()
	m.cliReadOnly, m.readOnly = true, true
	assert.False(t, blockedFor(m, []string{"help"}))
	assert.False(t, blockedFor(m, []string{"options"}))
}
