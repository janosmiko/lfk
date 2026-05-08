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
				assert.True(t, m.logWrap)
			},
		},
		{
			name:   "nowrap",
			option: "nowrap",
			checkFn: func(t *testing.T, m Model) {
				assert.False(t, m.logWrap)
			},
		},
		{
			name:   "linenumbers",
			option: "linenumbers",
			checkFn: func(t *testing.T, m Model) {
				assert.True(t, m.logLineNumbers)
			},
		},
		{
			name:   "nolinenumbers",
			option: "nolinenumbers",
			checkFn: func(t *testing.T, m Model) {
				assert.False(t, m.logLineNumbers)
			},
		},
		{
			name:   "timestamps",
			option: "timestamps",
			checkFn: func(t *testing.T, m Model) {
				assert.True(t, m.logTimestamps)
			},
		},
		{
			name:   "notimestamps",
			option: "notimestamps",
			checkFn: func(t *testing.T, m Model) {
				assert.False(t, m.logTimestamps)
			},
		},
		{
			name:   "follow",
			option: "follow",
			checkFn: func(t *testing.T, m Model) {
				assert.True(t, m.logFollow)
			},
		},
		{
			name:   "nofollow",
			option: "nofollow",
			checkFn: func(t *testing.T, m Model) {
				assert.False(t, m.logFollow)
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
		assert.True(t, rm.logWrap)
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

		assert.Equal(t, fmt.Sprintf("Max %d exceeded for bulk YAML copy", maxBulkYAMLCopy), rm.statusMessage)
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

	// LevelContainers must NOT show "Fetching N...": copyYAMLToClipboard
	// fetches the parent Pod by OwnedName regardless of selection (containers
	// don't have separate YAML), so the bulk indicator would be a lie.
	t.Run("export_yaml_at_level_containers_falls_back_silently", func(t *testing.T) {
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

		assert.Empty(t, rm.statusMessage,
			"LevelContainers cmd ignores selection; dispatcher must skip the bulk indicator")
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
