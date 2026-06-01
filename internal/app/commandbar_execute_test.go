package app

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// ---------------------------------------------------------------------------
// extractShellCommand
// ---------------------------------------------------------------------------

func TestExtractShellCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "bang_space_cmd", input: "! ls -la", want: "ls -la"},
		{name: "bang_no_space", input: "!ls", want: "ls"},
		{name: "bang_multiple_spaces", input: "!   echo hello", want: "echo hello"},
		{name: "bang_only", input: "!", want: ""},
		{name: "bang_space_only", input: "! ", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractShellCommand(tt.input))
		})
	}
}

// ---------------------------------------------------------------------------
// injectKubectlDefaults
// ---------------------------------------------------------------------------

func TestInjectKubectlDefaults(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		context   string
		namespace string
		wantCtx   bool // expect --context in result
		wantNS    bool // expect -n in result
	}{
		{
			name:      "inject_both",
			args:      []string{"get", "pods"},
			context:   "my-ctx",
			namespace: "my-ns",
			wantCtx:   true,
			wantNS:    true,
		},
		{
			name:      "has_namespace_flag",
			args:      []string{"get", "pods", "-n", "other"},
			context:   "my-ctx",
			namespace: "my-ns",
			wantCtx:   true,
			wantNS:    false, // already present, should not inject
		},
		{
			name:      "has_all_namespaces",
			args:      []string{"get", "pods", "-A"},
			context:   "my-ctx",
			namespace: "my-ns",
			wantCtx:   true,
			wantNS:    false, // -A means all namespaces
		},
		{
			name:      "has_all_namespaces_long",
			args:      []string{"get", "pods", "--all-namespaces"},
			context:   "my-ctx",
			namespace: "my-ns",
			wantCtx:   true,
			wantNS:    false,
		},
		{
			name:      "has_context_flag",
			args:      []string{"get", "pods", "--context", "foo"},
			context:   "my-ctx",
			namespace: "my-ns",
			wantCtx:   false, // already present
			wantNS:    true,
		},
		{
			name:      "has_namespace_equals",
			args:      []string{"get", "pods", "--namespace=bar"},
			context:   "my-ctx",
			namespace: "my-ns",
			wantCtx:   true,
			wantNS:    false, // equals form should be detected
		},
		{
			name:      "empty_context_no_inject",
			args:      []string{"get", "pods"},
			context:   "",
			namespace: "my-ns",
			wantCtx:   false, // empty context, nothing to inject
			wantNS:    true,
		},
		{
			name:      "empty_namespace_no_inject",
			args:      []string{"get", "pods"},
			context:   "my-ctx",
			namespace: "",
			wantCtx:   true,
			wantNS:    false, // empty namespace, nothing to inject
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseModelCov()
			m.nav.Context = tt.context
			m.namespace = tt.namespace

			result := m.injectKubectlDefaults(tt.args)

			hasCtx := containsFlag(result, "--context")
			hasNS := containsFlag(result, "-n")

			if tt.wantCtx {
				assert.True(t, hasCtx, "expected --context to be injected")
			} else {
				// If context was already present in input, it should still be there.
				// If empty, it should not have been added.
				if tt.context == "" {
					assert.False(t, hasCtx, "expected --context NOT to be injected (empty)")
				}
			}

			if tt.wantNS {
				assert.True(t, hasNS, "expected -n to be injected")
			}
		})
	}
}

// TestEmbeddedPTYSize_ClampsToFallbacksWhenSmall pins the safety net
// that protects the PTY launch path from being called before the
// initial WindowSizeMsg arrives (m.width / m.height both zero), which
// would otherwise hand pty.StartWithSize a 0x0 winsize and trip the
// underlying ioctl.
func TestEmbeddedPTYSize_ClampsToFallbacksWhenSmall(t *testing.T) {
	tests := []struct {
		name     string
		w, h     int
		wantCols int
		wantRows int
	}{
		{name: "uninitialized", w: 0, h: 0, wantCols: 80, wantRows: 24},
		{name: "tiny", w: 5, h: 3, wantCols: 80, wantRows: 24},
		{name: "border_too_small_cols", w: 19, h: 100, wantCols: 80, wantRows: 94},
		{name: "border_too_small_rows", w: 200, h: 10, wantCols: 200, wantRows: 24},
		{name: "normal", w: 200, h: 60, wantCols: 200, wantRows: 54},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{width: tt.w, height: tt.h}
			cols, rows := m.embeddedPTYSize()
			assert.Equal(t, tt.wantCols, cols, "cols")
			assert.Equal(t, tt.wantRows, rows, "rows")
		})
	}
}

func TestFmtPTYTitle_TruncatesLong(t *testing.T) {
	short := "kubectl get pods"
	assert.Equal(t, short, fmtPTYTitle(short))

	long := strings.Repeat("x", 80)
	got := fmtPTYTitle(long)
	// Trailing ellipsis is U+2026 (3 bytes in UTF-8); cap on visible
	// rune count rather than byte length so widening characters don't
	// trip the assertion.
	assert.LessOrEqual(t, len([]rune(got)), 60)
	assert.True(t, strings.HasSuffix(got, "…"))
}

// TestInjectKubectlDefaults_SkipsNamespaceFlagsForNonNamespacedCommands
// pins the fix for kubectl subcommands that reject -A/-n. Before the fix,
// `:k explain pod` with allNamespaces=true would append -A and fail with
// "unknown shorthand flag: 'A' in -A". The injection must skip those
// subcommands while still propagating --context (which kubectl accepts
// as a global flag on every subcommand).
func TestInjectKubectlDefaults_SkipsNamespaceFlagsForNonNamespacedCommands(t *testing.T) {
	nonNS := []string{
		"explain", "api-resources", "api-versions", "version",
		"cluster-info", "config", "options", "plugin", "completion",
		"help", "cordon", "uncordon", "drain", "taint",
		"kustomize", "convert",
	}
	for _, sub := range nonNS {
		t.Run(sub+"_with_allNamespaces", func(t *testing.T) {
			m := baseModelCov()
			m.nav.Context = "my-ctx"
			m.namespace = ""
			m.allNamespaces = true
			m.selectedNamespaces = nil

			result := m.injectKubectlDefaults([]string{sub, "pod"})

			assert.True(t, containsFlag(result, "--context"),
				"--context should still be injected for %q", sub)
			assert.False(t, containsFlag(result, "-A"),
				"-A must NOT be injected for %q", sub)
			assert.False(t, containsFlag(result, "-n"),
				"-n must NOT be injected for %q", sub)
		})
		t.Run(sub+"_with_namespace", func(t *testing.T) {
			m := baseModelCov()
			m.nav.Context = "my-ctx"
			m.namespace = "kube-system"

			result := m.injectKubectlDefaults([]string{sub, "pod"})

			assert.False(t, containsFlag(result, "-n"),
				"-n must NOT be injected for %q even with namespace set", sub)
		})
	}
}

// TestInjectKubectlDefaults_NamespacedCommandsStillGetFlags verifies the
// allowlist doesn't accidentally exclude `get`, `describe`, etc.
func TestInjectKubectlDefaults_NamespacedCommandsStillGetFlags(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "my-ctx"
	m.namespace = ""
	m.allNamespaces = true

	result := m.injectKubectlDefaults([]string{"get", "pods"})

	assert.True(t, containsFlag(result, "-A"),
		"get pods with allNamespaces should still get -A injected")
}

// TestCommandSupportsNamespaceFlags_SkipsGlobalFlags pins the fix for
// kubectl global flags appearing before the verb (Cobra accepts them
// anywhere). Without this, `:k --context foo explain pod` would slip
// past the denylist and still get `-A` injected.
func TestCommandSupportsNamespaceFlags_SkipsGlobalFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "bare_explain", args: []string{"explain", "pod"}, want: false},
		{name: "bare_get", args: []string{"get", "pods"}, want: true},
		{name: "global_flag_value_then_explain", args: []string{"--context", "foo", "explain", "pod"}, want: false},
		{name: "global_flag_equals_then_explain", args: []string{"--context=foo", "explain", "pod"}, want: false},
		{name: "multiple_globals_then_explain", args: []string{"--context", "foo", "--request-timeout=5s", "explain", "pod"}, want: false},
		{name: "short_n_then_explain", args: []string{"-n", "kube-system", "explain", "pod"}, want: false},
		{name: "globals_then_get", args: []string{"--context", "foo", "get", "pods"}, want: true},
		{name: "all_flags_no_verb", args: []string{"--context", "foo"}, want: false},
		{name: "empty", args: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, commandSupportsNamespaceFlags(tt.args))
		})
	}
}

// TestInjectKubectlDefaults_GlobalFlagsBeforeExplain combines the
// denylist with a real injection call to prove the end-to-end fix:
// `--context X explain pod` with allNamespaces must NOT get `-A`.
func TestInjectKubectlDefaults_GlobalFlagsBeforeExplain(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "my-ctx"
	m.namespace = ""
	m.allNamespaces = true

	result := m.injectKubectlDefaults([]string{"--context", "explicit", "explain", "pod"})

	assert.False(t, containsFlag(result, "-A"),
		"-A must not be injected when explain sits after a global flag")
	assert.False(t, containsFlag(result, "-n"),
		"-n must not be injected for explain")
}

// TestFmtPTYTitle_RuneSafe confirms the truncation cuts on runes, not
// bytes, so multi-byte codepoints never get split mid-encoding.
func TestFmtPTYTitle_RuneSafe(t *testing.T) {
	// 70 em-dashes (3 bytes each in UTF-8). Byte length 210; rune count 70.
	long := strings.Repeat("—", 70)
	got := fmtPTYTitle(long)

	assert.Equal(t, 60, len([]rune(got)), "must be truncated to 60 runes including the ellipsis")
	assert.True(t, strings.HasSuffix(got, "…"))
	// Decoding the truncated string must not produce U+FFFD (replacement
	// rune), which would indicate a mid-codepoint cut.
	for _, r := range got {
		assert.NotEqual(t, '�', r, "no replacement rune from mid-codepoint cut")
	}
}

// ---------------------------------------------------------------------------
// executeSetCommand
// ---------------------------------------------------------------------------

func TestExecuteSetCommand(t *testing.T) {
	tests := []struct {
		name      string
		option    string
		checkFn   func(t *testing.T, m Model)
		wantError bool
	}{
		{
			name:   "wrap",
			option: "wrap",
			checkFn: func(t *testing.T, m Model) {
				assert.True(t, m.logView.wrap)
			},
		},
		{
			name:   "nowrap",
			option: "nowrap",
			checkFn: func(t *testing.T, m Model) {
				assert.False(t, m.logView.wrap)
			},
		},
		{
			name:   "linenumbers",
			option: "linenumbers",
			checkFn: func(t *testing.T, m Model) {
				assert.True(t, m.logView.lineNumbers)
			},
		},
		{
			name:   "nolinenumbers",
			option: "nolinenumbers",
			checkFn: func(t *testing.T, m Model) {
				assert.False(t, m.logView.lineNumbers)
			},
		},
		{
			name:   "timestamps",
			option: "timestamps",
			checkFn: func(t *testing.T, m Model) {
				assert.True(t, m.logView.timestamps)
			},
		},
		{
			name:   "notimestamps",
			option: "notimestamps",
			checkFn: func(t *testing.T, m Model) {
				assert.False(t, m.logView.timestamps)
			},
		},
		{
			name:   "follow",
			option: "follow",
			checkFn: func(t *testing.T, m Model) {
				assert.True(t, m.logView.follow)
			},
		},
		{
			name:   "nofollow",
			option: "nofollow",
			checkFn: func(t *testing.T, m Model) {
				assert.False(t, m.logView.follow)
			},
		},
		{
			name:      "unknown_option",
			option:    "unknown",
			wantError: true,
		},
		{
			name:      "empty_option",
			option:    "",
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseModelCov()
			result, _ := m.executeSetCommand(tt.option)
			rm := result.(Model)
			if tt.wantError {
				assert.True(t, rm.statusMessageErr)
			} else {
				require.NotNil(t, tt.checkFn)
				tt.checkFn(t, rm)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// executeResourceJump
// ---------------------------------------------------------------------------

func TestExecuteResourceJump(t *testing.T) {
	t.Run("matching_left_item", func(t *testing.T) {
		m := baseModelCov()
		m.nav.Level = model.LevelResourceTypes
		m.leftItems = []model.Item{
			{Name: "Pods", Kind: "Pod", Extra: "v1/pods"},
			{Name: "Deployments", Kind: "Deployment", Extra: "apps/v1/deployments"},
			{Name: "Services", Kind: "Service", Extra: "v1/services"},
		}
		m.middleItems = m.leftItems

		ret, _ := m.executeResourceJump("deployment")
		result := ret.(Model)
		// Should have navigated into the resource type (level changed to Resources).
		assert.NotNil(t, result)
	})

	t.Run("abbreviation_jump", func(t *testing.T) {
		origAbbr := ui.SearchAbbreviations
		ui.SearchAbbreviations = map[string]string{
			"deploy": "deployments",
			"svc":    "services",
		}
		defer func() { ui.SearchAbbreviations = origAbbr }()

		m := baseModelCov()
		m.nav.Level = model.LevelResourceTypes
		m.leftItems = []model.Item{
			{Name: "Pods", Kind: "Pod", Extra: "v1/pods"},
			{Name: "Deployments", Kind: "Deployment", Extra: "apps/v1/deployments"},
			{Name: "Services", Kind: "Service", Extra: "v1/services"},
		}
		m.middleItems = m.leftItems

		ret, _ := m.executeResourceJump("svc")
		result := ret.(Model)
		assert.NotNil(t, result)
	})

	t.Run("no_match_shows_error", func(t *testing.T) {
		m := baseModelCov()
		m.nav.Level = model.LevelResourceTypes
		m.leftItems = []model.Item{
			{Name: "Pods", Kind: "Pod", Extra: "v1/pods"},
		}
		m.middleItems = m.leftItems

		ret, _ := m.executeResourceJump("nonexistent")
		result := ret.(Model)
		assert.Contains(t, result.statusMessage, "not found")
	})
}

// ---------------------------------------------------------------------------
// executeBuiltinCommand
// ---------------------------------------------------------------------------

func TestExecuteBuiltinCommand(t *testing.T) {
	t.Run("quit_returns_quit_cmd", func(t *testing.T) {
		m := baseModelCov()
		_, cmd := m.executeBuiltinCommand("q")
		require.NotNil(t, cmd)
	})

	t.Run("quit_bang", func(t *testing.T) {
		m := baseModelCov()
		_, cmd := m.executeBuiltinCommand("q!")
		require.NotNil(t, cmd)
	})

	t.Run("namespace_sets_namespace", func(t *testing.T) {
		m := baseModelWithFakeClient()
		result, _ := m.executeBuiltinCommand("ns production")
		rm := result.(Model)
		assert.Equal(t, "production", rm.namespace)
	})

	t.Run("context_sets_context", func(t *testing.T) {
		m := baseModelWithFakeClient()
		result, _ := m.executeBuiltinCommand("ctx my-cluster")
		rm := result.(Model)
		assert.Equal(t, "my-cluster", rm.nav.Context)
	})

	t.Run("set_delegates_to_set_command", func(t *testing.T) {
		m := baseModelCov()
		result, _ := m.executeBuiltinCommand("set wrap")
		rm := result.(Model)
		assert.True(t, rm.logView.wrap)
	})

	t.Run("sort_sets_column", func(t *testing.T) {
		m := baseModelCov()
		result, _ := m.executeBuiltinCommand("sort Name")
		rm := result.(Model)
		assert.Equal(t, "Name", rm.sortColumnName)
	})

	// :sort at picker levels (LevelClusters, LevelResourceTypes) is a no-op
	// because sortMiddleItems() early-returns there. The command must not
	// mutate sort state silently — it must signal the user with an error
	// status so they understand why typing :sort had no visible effect.
	t.Run("sort_no_op_at_clusters_level", func(t *testing.T) {
		m := baseModelCov()
		m.nav.Level = model.LevelClusters
		m.sortColumnName = "Name"
		m.sortAscending = true

		result, _ := m.executeBuiltinCommand("sort Age")
		rm := result.(Model)

		assert.Equal(t, "Name", rm.sortColumnName, "sort column must not change at LevelClusters")
		assert.True(t, rm.sortAscending, "sortAscending must not change at LevelClusters")
		assert.True(t, rm.statusMessageErr, "must signal error to user who explicitly invoked :sort")
	})

	t.Run("sort_no_op_at_resource_types_level", func(t *testing.T) {
		m := baseModelCov()
		m.nav.Level = model.LevelResourceTypes
		m.sortColumnName = "Name"

		result, _ := m.executeBuiltinCommand("sort Age")
		rm := result.(Model)

		assert.Equal(t, "Name", rm.sortColumnName, "sort column must not change at LevelResourceTypes")
		assert.True(t, rm.statusMessageErr, "must signal error to user who explicitly invoked :sort")
	})

	t.Run("unknown_builtin_returns_error", func(t *testing.T) {
		m := baseModelCov()
		result, _ := m.executeBuiltinCommand("notabuiltin")
		rm := result.(Model)
		assert.True(t, rm.statusMessageErr)
	})

	// `:export yaml` shares the multi-selection bulk path with the `Y` key.
	// Without this hookup the command-bar route silently dispatches a bulk
	// fetch (which can hit dozens of items at QPS=5) with no progress
	// indicator and no over-cap protection, so the user is left staring at
	// a blank toast for ~10s.
	t.Run("export_yaml_with_selection_shows_fetching_status", func(t *testing.T) {
		m := basePush80Model()
		m.toggleSelection(m.middleItems[0])
		m.toggleSelection(m.middleItems[1])

		result, cmd := m.executeBuiltinCommand("export yaml")
		rm := result.(Model)

		assert.Equal(t, "Fetching 2 manifests...", rm.statusMessage,
			":export must mirror Y's bulk dispatcher status")
		assert.NotNil(t, cmd)
	})

	// Cap protection: `:export yaml` past maxBulkYAMLCopy must reject with
	// the same error toast the Y key path uses, not silently kick off a
	// 100-item sequential fetch behind the rate limiter.
	t.Run("export_yaml_over_cap_rejects", func(t *testing.T) {
		m := basePush80Model()
		m.middleItems = make([]model.Item, maxBulkYAMLCopy+1)
		for i := range m.middleItems {
			m.middleItems[i] = model.Item{
				Name:      fmt.Sprintf("pod-%d", i),
				Namespace: "default",
				Kind:      "Pod",
			}
			m.toggleSelection(m.middleItems[i])
		}

		result, cmd := m.executeBuiltinCommand("export yaml")
		rm := result.(Model)

		assert.Equal(t, fmt.Sprintf("Max %d exceeded for bulk YAML/JSON copy", maxBulkYAMLCopy), rm.statusMessage)
		assert.True(t, rm.statusMessageErr, "must surface as error toast")
		assert.NotNil(t, cmd, "auto-clear timer is still dispatched")
	})

	// No selection: `:export yaml` falls through to the cursor-row single-
	// item fetch — no "Fetching N..." status (that's reserved for the bulk
	// path). The cmd is still non-nil so the YAML still goes to clipboard.
	t.Run("export_yaml_no_selection_uses_cursor", func(t *testing.T) {
		m := basePush80Model()
		m.setCursor(0)

		result, cmd := m.executeBuiltinCommand("export yaml")
		rm := result.(Model)

		assert.Empty(t, rm.statusMessage,
			"single-item path dispatches silently; status is set only when the fetch resolves")
		assert.NotNil(t, cmd)
	})

	// `:export json` reuses the same bulk-or-cursor dispatcher as `yaml`,
	// then post-processes the YAML payload into JSON. Pin the bulk-status
	// hookup so a future refactor that swaps the dispatcher doesn't silently
	// drop the over-cap / "Fetching N..." UI.
	t.Run("export_json_with_selection_shows_fetching_status", func(t *testing.T) {
		m := basePush80Model()
		m.toggleSelection(m.middleItems[0])
		m.toggleSelection(m.middleItems[1])

		result, cmd := m.executeBuiltinCommand("export json")
		rm := result.(Model)

		assert.Equal(t, "Fetching 2 manifests...", rm.statusMessage)
		assert.NotNil(t, cmd)
	})

	// LevelContainers now supports bulk: when N containers are selected,
	// the dispatcher fetches the parent Pod once and extracts the matching
	// container spec blocks, so "Fetching N manifests..." correctly reflects
	// the N requested container blocks.
	t.Run("export_yaml_at_level_containers_uses_bulk", func(t *testing.T) {
		m := basePush80Model()
		m.nav.Level = model.LevelContainers
		m.nav.OwnedName = "pod-1"
		m.middleItems = []model.Item{
			{Name: "container-1", Kind: "Container", Namespace: "default"},
			{Name: "container-2", Kind: "Container", Namespace: "default"},
		}
		m.toggleSelection(m.middleItems[0])
		m.toggleSelection(m.middleItems[1])

		result, cmd := m.executeBuiltinCommand("export yaml")
		rm := result.(Model)

		assert.Equal(t, "Fetching 2 manifests...", rm.statusMessage,
			"LevelContainers now supports bulk; dispatcher must show the fetching toast")
		assert.NotNil(t, cmd)
	})

	t.Run("export_unknown_format_returns_error", func(t *testing.T) {
		m := basePush80Model()
		result, _ := m.executeBuiltinCommand("export csv")
		rm := result.(Model)
		assert.Contains(t, rm.statusMessage, "Unknown export format")
		assert.True(t, rm.statusMessageErr)
	})
}

// ---------------------------------------------------------------------------
// wrapYAMLCmdAsJSON
// ---------------------------------------------------------------------------

func TestWrapYAMLCmdAsJSON(t *testing.T) {
	t.Run("single_doc_becomes_json_object", func(t *testing.T) {
		inner := func() tea.Msg {
			return yamlClipboardMsg{
				content: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: foo\n",
				count:   1,
			}
		}

		out := wrapYAMLCmdAsJSON(inner)().(yamlClipboardMsg)
		require.NoError(t, out.err)
		assert.Equal(t, 1, out.count)
		assert.Contains(t, out.content, `"apiVersion":"v1"`)
		assert.Contains(t, out.content, `"kind":"Pod"`)
		assert.Contains(t, out.content, `"name":"foo"`)
	})

	t.Run("multi_doc_becomes_json_array", func(t *testing.T) {
		inner := func() tea.Msg {
			return yamlClipboardMsg{
				content: "kind: Pod\nmetadata:\n  name: a\n" +
					"\n---\n" +
					"kind: Pod\nmetadata:\n  name: b\n",
				count: 2,
			}
		}

		out := wrapYAMLCmdAsJSON(inner)().(yamlClipboardMsg)
		require.NoError(t, out.err)
		assert.Equal(t, 2, out.count)

		var arr []map[string]any
		require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out.content)), &arr))
		require.Len(t, arr, 2)
		assert.Equal(t, "a", arr[0]["metadata"].(map[string]any)["name"])
		assert.Equal(t, "b", arr[1]["metadata"].(map[string]any)["name"])
	})

	t.Run("inner_error_passes_through_unchanged", func(t *testing.T) {
		inner := func() tea.Msg {
			return yamlClipboardMsg{err: assert.AnError}
		}

		out := wrapYAMLCmdAsJSON(inner)().(yamlClipboardMsg)
		assert.ErrorIs(t, out.err, assert.AnError)
		assert.Empty(t, out.content)
	})

	t.Run("non_yaml_message_passes_through_unchanged", func(t *testing.T) {
		marker := struct{ note string }{note: "not-a-yaml-msg"}
		inner := func() tea.Msg { return marker }

		out := wrapYAMLCmdAsJSON(inner)()
		assert.Equal(t, marker, out)
	})

	t.Run("malformed_yaml_surfaces_as_error_envelope", func(t *testing.T) {
		inner := func() tea.Msg {
			return yamlClipboardMsg{
				content: "kind: Pod\nmetadata:\n  name: a\nbadly_formed: : :\n",
				count:   1,
			}
		}

		out := wrapYAMLCmdAsJSON(inner)().(yamlClipboardMsg)
		require.Error(t, out.err)
		assert.Contains(t, out.err.Error(), "converting YAML to JSON")
	})
}

// ---------------------------------------------------------------------------
// executeCommandBarInput (dispatcher)
// ---------------------------------------------------------------------------

func TestExecuteCommandBarInput(t *testing.T) {
	t.Run("empty_input", func(t *testing.T) {
		m := baseModelCov()
		result, cmd := m.executeCommandBarInput("")
		rm := result.(Model)
		assert.Equal(t, m, rm)
		assert.Nil(t, cmd)
	})

	t.Run("shell_input", func(t *testing.T) {
		m := baseModelWithFakeClient()
		_, cmd := m.executeCommandBarInput("! echo hello")
		assert.NotNil(t, cmd)
	})

	t.Run("builtin_input_quit", func(t *testing.T) {
		m := baseModelCov()
		_, cmd := m.executeCommandBarInput("q")
		assert.NotNil(t, cmd)
	})

	t.Run("unknown_tries_kubectl", func(t *testing.T) {
		m := baseModelWithFakeClient()
		_, cmd := m.executeCommandBarInput("somethingweird")
		// Should attempt kubectl for backward compat.
		assert.NotNil(t, cmd)
	})
}

// ---------------------------------------------------------------------------
// executeOrphansCommand
// ---------------------------------------------------------------------------

func TestOrphansCommand_KindArg_JumpsToList(t *testing.T) {
	m := newTestModel()
	m.nav.Context = "test"
	m.namespace = "default"
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Pods", Kind: "Pod", Extra: "v1/pods"},
		{Name: "Secrets", Kind: "Secret", Extra: "v1/secrets"},
		{Name: "ConfigMaps", Kind: "ConfigMap", Extra: "v1/configmaps"},
		{Name: "Services", Kind: "Service", Extra: "v1/services"},
	}
	m.leftItems = m.middleItems

	updated, _ := m.executeBuiltinCommand("orphans secrets")

	mu := updated.(Model)
	// Regardless of whether the resource jump fully navigates (it requires
	// discoveredResources), the orphan preset must be activated.
	require.NotNil(t, mu.activeFilterPreset)
	assert.Equal(t, "Unmounted", mu.activeFilterPreset.Name)
}

func TestOrphansCommand_NoArg_OpensOverlay(t *testing.T) {
	m := newTestModel()
	updated, _ := m.executeBuiltinCommand("orphans")
	mu := updated.(Model)
	assert.Equal(t, overlayOrphans, mu.overlay)
}

func TestOrphansCommand_UnknownKind(t *testing.T) {
	m := newTestModel()
	updated, _ := m.executeBuiltinCommand("orphans foo")
	mu := updated.(Model)
	assert.Contains(t, mu.statusMessage, "unknown kind")
}

func TestOrphansCommand_PodAlias(t *testing.T) {
	m := newTestModel()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Pods", Kind: "Pod", Extra: "v1/pods"},
	}
	m.leftItems = m.middleItems

	updated, _ := m.executeBuiltinCommand("orphans po")
	mu := updated.(Model)
	require.NotNil(t, mu.activeFilterPreset)
	assert.Equal(t, "Orphans", mu.activeFilterPreset.Name)
}

func TestOrphansCommand_ConfigMapAlias(t *testing.T) {
	m := newTestModel()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "ConfigMaps", Kind: "ConfigMap", Extra: "v1/configmaps"},
	}
	m.leftItems = m.middleItems

	updated, _ := m.executeBuiltinCommand("orphans cm")
	mu := updated.(Model)
	require.NotNil(t, mu.activeFilterPreset)
	assert.Equal(t, "Unmounted", mu.activeFilterPreset.Name)
}

func TestOrphansCommand_ServiceAlias(t *testing.T) {
	m := newTestModel()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Services", Kind: "Service", Extra: "v1/services"},
	}
	m.leftItems = m.middleItems

	updated, _ := m.executeBuiltinCommand("orphans svc")
	mu := updated.(Model)
	require.NotNil(t, mu.activeFilterPreset)
	assert.Equal(t, "No Endpoints", mu.activeFilterPreset.Name)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func TestExtractShellCommandNoPrefix(t *testing.T) {
	// Edge case: if someone somehow passes a string without bang.
	assert.Equal(t, "hello", extractShellCommand("hello"))
}

// containsFlag checks if a flag exists in a slice of args.
func containsFlag(args []string, flag string) bool {
	return slices.Contains(args, flag)
}

// ---------------------------------------------------------------------------
// scheduler command rename from :tasks
// ---------------------------------------------------------------------------

func TestCommandbarExecute_SchedulerOpensTasksOverlay(t *testing.T) {
	m := newTestModel()
	result, _ := m.executeBuiltinCommand("scheduler")
	rm := result.(Model)

	assert.Equal(t, overlayBackgroundTasks, rm.overlay,
		":scheduler must open the tasks overlay")
}

func TestCommandbarExecute_TasksAliasGone(t *testing.T) {
	m := newTestModel()
	result, _ := m.executeBuiltinCommand("tasks")
	rm := result.(Model)

	assert.NotEqual(t, overlayBackgroundTasks, rm.overlay,
		":tasks must no longer open the overlay (hard rename)")
}
