package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- expandSearchQuery ---

func TestExpandSearchQuery(t *testing.T) {
	t.Run("plain query returns itself", func(t *testing.T) {
		queries := expandSearchQuery("nginx")
		assert.Contains(t, queries, "nginx")
		assert.Len(t, queries, 1)
	})

	t.Run("preserves original case", func(t *testing.T) {
		queries := expandSearchQuery("NGINX")
		assert.Contains(t, queries, "NGINX")
	})
}

// --- searchMatches ---

func TestSearchMatches(t *testing.T) {
	m := Model{}

	assert.True(t, m.searchMatches("nginx-pod", []string{"nginx"}))
	assert.True(t, m.searchMatches("NGINX-Pod", []string{"nginx"}))
	assert.False(t, m.searchMatches("redis-pod", []string{"nginx"}))
	assert.True(t, m.searchMatches("test", []string{"te"}))
	assert.False(t, m.searchMatches("test", []string{"xyz"}))
}

// --- searchMatchesItem ---

func TestSearchMatchesItem(t *testing.T) {
	t.Run("matches by name", func(t *testing.T) {
		m := Model{nav: model.NavigationState{Level: model.LevelResources}}
		item := model.Item{Name: "nginx-deployment"}
		assert.True(t, m.searchMatchesItem(item, []string{"nginx"}))
	})

	t.Run("matches by category at LevelResourceTypes", func(t *testing.T) {
		// Categories render as visible bars at the resource-types
		// level (Workloads, Argo CD, ...). The renderer highlights
		// matching text in those bars, so n/N jumping to items under
		// a matching category makes sense — the user sees the marked
		// bar and lands on the matching group.
		m := Model{nav: model.NavigationState{Level: model.LevelResourceTypes}}
		item := model.Item{Name: "Applications", Category: "Argo CD"}
		assert.True(t, m.searchMatchesItem(item, []string{"argo"}),
			"search at resource-types level must reach the visible category bar")
	})

	t.Run("does NOT match by category at deeper levels", func(t *testing.T) {
		// At LevelResources items still carry a Category from the
		// data model, but the bar isn't rendered there — matching it
		// would jump n/N to a row with no visible highlight.
		m := Model{nav: model.NavigationState{Level: model.LevelResources}}
		item := model.Item{Name: "my-pod", Category: "Workloads"}
		assert.False(t, m.searchMatchesItem(item, []string{"workloads"}),
			"category match outside LevelResourceTypes would jump to non-highlighted rows")
	})

	t.Run("does not match by namespace alone", func(t *testing.T) {
		m := Model{nav: model.NavigationState{Level: model.LevelResources}}
		item := model.Item{Name: "my-pod", Namespace: "production"}
		assert.False(t, m.searchMatchesItem(item, []string{"production"}))
	})

	t.Run("no match", func(t *testing.T) {
		m := Model{nav: model.NavigationState{Level: model.LevelResources}}
		item := model.Item{Name: "nginx"}
		assert.False(t, m.searchMatchesItem(item, []string{"redis"}))
	})
}

// --- resourceNames ---

func TestResourceNames(t *testing.T) {
	m := Model{
		nav: model.NavigationState{Level: model.LevelResources},
		middleItems: []model.Item{
			{Name: "pod-a"},
			{Name: "pod-b"},
			{Name: "pod-a"}, // duplicate
			{Name: ""},      // empty
		},
	}

	names := resourceNames(&m)
	assert.Equal(t, []string{"pod-a", "pod-b"}, names)
}

func TestResourceNamesEmpty(t *testing.T) {
	m := Model{}
	names := resourceNames(&m)
	assert.Empty(t, names)
}

// --- jumpToSearchMatch ---

func TestJumpToSearchMatch(t *testing.T) {
	t.Run("finds matching item forward", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "alpha-pod"},
				{Name: "beta-pod"},
				{Name: "nginx-pod"},
				{Name: "gamma-pod"},
			},
			searchInput: TextInput{Value: "nginx"},
		}
		m.setCursor(0)
		m.jumpToSearchMatch(0)
		assert.Equal(t, 2, m.cursor())
	})

	t.Run("wraps around to start", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "nginx-pod"},
				{Name: "alpha-pod"},
				{Name: "beta-pod"},
			},
			searchInput: TextInput{Value: "nginx"},
		}
		m.setCursor(1)
		m.jumpToSearchMatch(1)
		assert.Equal(t, 0, m.cursor())
	})

	t.Run("empty query does nothing", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "nginx-pod"},
			},
			searchInput: TextInput{},
		}
		m.setCursor(0)
		m.jumpToSearchMatch(0)
		assert.Equal(t, 0, m.cursor())
	})

	t.Run("no match keeps cursor", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "alpha-pod"},
				{Name: "beta-pod"},
			},
			searchInput: TextInput{Value: "nonexistent"},
		}
		m.setCursor(0)
		m.jumpToSearchMatch(0)
		assert.Equal(t, 0, m.cursor())
	})
}

// --- jumpToPrevSearchMatch ---

func TestJumpToPrevSearchMatch(t *testing.T) {
	t.Run("finds matching item backward", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "nginx-1"},
				{Name: "alpha-pod"},
				{Name: "nginx-2"},
				{Name: "beta-pod"},
			},
			searchInput: TextInput{Value: "nginx"},
		}
		m.setCursor(3)
		m.jumpToPrevSearchMatch(3)
		assert.Equal(t, 2, m.cursor())
	})

	t.Run("wraps around to end", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "alpha-pod"},
				{Name: "beta-pod"},
				{Name: "nginx-pod"},
			},
			searchInput: TextInput{Value: "nginx"},
		}
		m.setCursor(0)
		m.jumpToPrevSearchMatch(0)
		assert.Equal(t, 2, m.cursor())
	})

	t.Run("empty query does nothing", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResources},
			middleItems: []model.Item{
				{Name: "nginx-pod"},
			},
			searchInput: TextInput{},
		}
		m.setCursor(0)
		m.jumpToPrevSearchMatch(0)
		assert.Equal(t, 0, m.cursor())
	})
}

// --- searchAllItems ---

func TestSearchAllItems(t *testing.T) {
	t.Run("forward search expands collapsed group", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResourceTypes},
			middleItems: []model.Item{
				{Name: "Pods", Category: "Workloads"},
				{Name: "Deployments", Category: "Workloads"},
				{Name: "Services", Category: "Networking"},
				{Name: "Ingresses", Category: "Networking"},
			},
			expandedGroup: "Workloads",
			searchInput:   TextInput{Value: "services"},
		}
		m.setCursor(0)
		m.searchAllItems([]string{"services"}, 0, true)
		assert.Equal(t, "Networking", m.expandedGroup)
	})

	t.Run("backward search finds match", func(t *testing.T) {
		m := Model{
			nav: model.NavigationState{Level: model.LevelResourceTypes},
			middleItems: []model.Item{
				{Name: "Pods", Category: "Workloads"},
				{Name: "Services", Category: "Networking"},
				{Name: "Ingresses", Category: "Networking"},
			},
			expandedGroup: "Networking",
			searchInput:   TextInput{Value: "pods"},
		}
		m.setCursor(1)
		m.searchAllItems([]string{"pods"}, 1, false)
		assert.Equal(t, "Workloads", m.expandedGroup)
	})
}

// --- commandBarApplySuggestion ---

func TestCommandBarApplySuggestion(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		suggestion string
		expected   string
	}{
		{
			name:       "empty input appends suggestion",
			input:      "",
			suggestion: "get",
			expected:   "get",
		},
		{
			name:       "input ending with space appends",
			input:      "kubectl ",
			suggestion: "get",
			expected:   "kubectl get",
		},
		{
			name:       "replaces last partial word",
			input:      "kubectl ge",
			suggestion: "get",
			expected:   "kubectl get",
		},
		{
			name:       "single partial word replaces",
			input:      "ge",
			suggestion: "get",
			expected:   "get",
		},
		{
			name:       "replaces last word of multi-word input",
			input:      "kubectl get po",
			suggestion: "pod",
			expected:   "kubectl get pod",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				commandBarInput: TextInput{Value: tt.input},
			}
			result := m.commandBarApplySuggestion(tt.suggestion)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- generateCommandBarSuggestions ---

func TestGenerateCommandBarSuggestions(t *testing.T) {
	t.Run("empty input returns default suggestions", func(t *testing.T) {
		m := Model{
			commandBarInput: TextInput{Value: ""},
		}
		suggestions := m.generateCommandBarSuggestions()
		// Empty input gives default suggestions (builtin commands + resources).
		assert.NotEmpty(t, suggestions)
	})

	t.Run("kubectl prefix with partial subcommand", func(t *testing.T) {
		m := Model{
			commandBarInput: TextInput{Value: "kubectl ge"},
		}
		suggestions := m.generateCommandBarSuggestions()
		texts := make([]string, 0, len(suggestions))
		for _, s := range suggestions {
			texts = append(texts, s.Text)
		}
		assert.Contains(t, texts, "get")
	})

	t.Run("shell command returns nil", func(t *testing.T) {
		m := Model{
			commandBarInput: TextInput{Value: "!echo hello"},
		}
		suggestions := m.generateCommandBarSuggestions()
		assert.Nil(t, suggestions)
	})

	t.Run("flag prefix suggests flags", func(t *testing.T) {
		m := Model{
			commandBarInput: TextInput{Value: "kubectl get pods -"},
		}
		suggestions := m.generateCommandBarSuggestions()
		assert.NotEmpty(t, suggestions)
	})
}

// --- completeResourceJump ---

func TestCompleteResourceJump(t *testing.T) {
	t.Run("returns built-in resource types", func(t *testing.T) {
		suggestions := completeResourceJump("", testLeftItems())
		assert.NotEmpty(t, suggestions)
		// Extract text values for comparison.
		texts := make([]string, 0, len(suggestions))
		for _, s := range suggestions {
			texts = append(texts, s.Text)
		}
		assert.Contains(t, texts, "pod")
		assert.Contains(t, texts, "deployment")
	})

	t.Run("prefix filter for services", func(t *testing.T) {
		suggestions := completeResourceJump("serv", testLeftItems())
		texts := make([]string, 0, len(suggestions))
		for _, s := range suggestions {
			texts = append(texts, s.Text)
		}
		assert.Contains(t, texts, "service")
	})

	t.Run("includes CRD names", func(t *testing.T) {
		items := append(testLeftItems(), model.Item{
			Name:  "MyCustomResource",
			Extra: "example.io/v1/mycustomresource",
		})
		suggestions := completeResourceJump("myc", items)
		texts := make([]string, 0, len(suggestions))
		for _, s := range suggestions {
			texts = append(texts, s.Text)
		}
		assert.Contains(t, texts, "mycustomresource")
	})

	t.Run("filters by prefix", func(t *testing.T) {
		suggestions := completeResourceJump("pod", testLeftItems())
		for _, s := range suggestions {
			assert.True(t, len(s.Text) > 0)
		}
	})

	t.Run("no duplicates", func(t *testing.T) {
		suggestions := completeResourceJump("", testLeftItems())
		seen := make(map[string]bool)
		for _, s := range suggestions {
			assert.False(t, seen[s.Text], "duplicate suggestion: %s", s.Text)
			seen[s.Text] = true
		}
	})
}

func TestPush4HandleFilterKeyEsc(t *testing.T) {
	m := basePush4Model()
	m.filterActive = true
	result, _ := m.handleFilterKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.filterActive)
}

func TestPush4HandleFilterKeyEnter(t *testing.T) {
	m := basePush4Model()
	m.filterActive = true
	result, _ := m.handleFilterKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.filterActive)
}

// TestHandleFilterKeyEnterInvalidatesPreview verifies that confirming a
// filter (Enter) invalidates the right pane: rightItems cleared,
// previewLoading armed, and requestGen bumped. Without this, the cursor
// jumps to the first filter match but the right pane keeps rendering the
// previous selection's children for several seconds until the new
// preview fetch returns — the regression the user reported as "search
// for pvc, jump on it, but it still shows services for 3-4 seconds".
func TestHandleFilterKeyEnterInvalidatesPreview(t *testing.T) {
	m := basePush4Model()
	m.filterActive = true
	m.requestGen = 5
	m.rightItems = []model.Item{{Name: "services-preview"}}
	m.previewLoading = false

	result, _ := m.handleFilterKey(keyMsg("enter"))
	rm := result.(Model)

	assert.Nil(t, rm.rightItems, "stale preview items from the prior cursor must be cleared")
	assert.True(t, rm.previewLoading, "previewLoading must be armed so the spinner shows during the new fetch")
	assert.Greater(t, rm.requestGen, uint64(5), "requestGen must bump so any in-flight pre-filter preview is discarded")
}

// TestHandleFilterKeyEscInvalidatesPreview mirrors the Enter case for the
// Esc path: clearing the filter resets the cursor, and the preview must
// refresh for the new cursor position.
func TestHandleFilterKeyEscInvalidatesPreview(t *testing.T) {
	m := basePush4Model()
	m.filterActive = true
	m.requestGen = 3
	m.rightItems = []model.Item{{Name: "old-preview"}}

	result, _ := m.handleFilterKey(keyMsg("esc"))
	rm := result.(Model)

	assert.Nil(t, rm.rightItems, "rightItems must clear when esc resets the cursor")
	assert.True(t, rm.previewLoading, "previewLoading must be armed")
	assert.Greater(t, rm.requestGen, uint64(3))
}

// TestHandleSearchKeyEnterInvalidatesPreview covers the search-mode
// (slash) analogue of the filter Enter case.
func TestHandleSearchKeyEnterInvalidatesPreview(t *testing.T) {
	m := basePush4Model()
	m.searchActive = true
	m.requestGen = 7
	m.rightItems = []model.Item{{Name: "prev-preview"}}

	result, _ := m.handleSearchKey(keyMsg("enter"))
	rm := result.(Model)

	assert.Nil(t, rm.rightItems, "confirming search must drop stale preview")
	assert.True(t, rm.previewLoading)
	assert.Greater(t, rm.requestGen, uint64(7))
}

func TestPush4HandleSearchKeyEsc(t *testing.T) {
	m := basePush4Model()
	m.searchActive = true
	result, _ := m.handleSearchKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.searchActive)
}

func TestPush4HandleSearchKeyEnter(t *testing.T) {
	m := basePush4Model()
	m.searchActive = true
	result, _ := m.handleSearchKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.searchActive)
}

func TestCovHandleFilterKeyEnter(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.filterActive = true
	m.filterInput = TextInput{Value: "test"}

	r, _ := m.handleFilterKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, "test", r.(Model).filterText)
	assert.False(t, r.(Model).filterActive)
}

func TestCovHandleFilterKeyEsc(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.filterActive = true
	m.filterInput = TextInput{Value: "test"}
	m.filterText = "test"

	r, _ := m.handleFilterKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, r.(Model).filterActive)
	assert.Empty(t, r.(Model).filterText)
}

func TestCovHandleFilterKeyBackspace(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.filterActive = true
	m.filterInput = TextInput{Value: "test", Cursor: 4}

	r, _ := m.handleFilterKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "tes", r.(Model).filterInput.Value)
}

func TestCovHandleFilterKeyCtrlW(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.filterInput = TextInput{Value: "hello world", Cursor: 11}

	r, _ := m.handleFilterKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Equal(t, "hello ", r.(Model).filterInput.Value)
}

func TestCovHandleFilterKeyCursorMovement(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.filterInput = TextInput{Value: "hello", Cursor: 3}

	r, _ := m.handleFilterKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	assert.Equal(t, 0, r.(Model).filterInput.Cursor)

	r, _ = m.handleFilterKey(tea.KeyMsg{Type: tea.KeyCtrlE})
	assert.Equal(t, 5, r.(Model).filterInput.Cursor)

	m.filterInput.Cursor = 3
	r, _ = m.handleFilterKey(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 2, r.(Model).filterInput.Cursor)

	m.filterInput.Cursor = 3
	r, _ = m.handleFilterKey(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 4, r.(Model).filterInput.Cursor)
}

func TestCovHandleFilterKeyInsert(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.filterInput = TextInput{Value: "", Cursor: 0}

	r, _ := m.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Equal(t, "x", r.(Model).filterInput.Value)
}

func TestCovHelpKeySearchEsc(t *testing.T) {
	// Under the new search-vs-filter split: Esc inside the / search
	// input cancels the search (clears helpSearchQuery) and exits the
	// input. The filter is independent and untouched.
	m := baseModelSearch()
	m.helpSearchActive = true
	m.helpSearchQuery = "test"
	m.helpSearchInput.SetValue("test")
	result, _ := m.handleHelpKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.helpSearchActive)
	assert.Empty(t, rm.helpSearchQuery)
}

func TestCovHelpKeySearchEnter(t *testing.T) {
	m := baseModelSearch()
	m.helpSearchActive = true
	m.helpFilter.Insert("test")
	result, _ := m.handleHelpKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.helpSearchActive)
}

func TestCovHelpKeySearchTyping(t *testing.T) {
	m := baseModelSearch()
	m.helpSearchActive = true
	result, cmd := m.handleHelpKey(keyMsg("a"))
	rm := result.(Model)
	_ = rm
	_ = cmd
}

func TestCovHelpKeyQuit(t *testing.T) {
	m := baseModelSearch()
	m.mode = modeHelp
	m.helpPreviousMode = modeExplorer
	result, _ := m.handleHelpKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovHelpKeyDown(t *testing.T) {
	m := baseModelSearch()
	m.helpScroll = 0
	result, _ := m.handleHelpKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.helpScroll)
}

func TestCovHelpKeyUp(t *testing.T) {
	m := baseModelSearch()
	m.helpScroll = 5
	result, _ := m.handleHelpKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 4, rm.helpScroll)
}

func TestCovHelpKeyUpAtZero(t *testing.T) {
	m := baseModelSearch()
	m.helpScroll = 0
	result, _ := m.handleHelpKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.helpScroll)
}

func TestCovHelpKeyGG(t *testing.T) {
	m := baseModelSearch()
	m.helpScroll = 10
	result, _ := m.handleHelpKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)

	result, _ = rm.handleHelpKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.helpScroll)
	assert.False(t, rm.pendingG)
}

func TestCovHelpKeyBigG(t *testing.T) {
	// G clamps to the actual max scroll (totalLines - viewport) so a
	// follow-up ctrl+u responds on the first press. The old impl parked
	// the model at the 9999 sentinel and required dozens of ctrl+u
	// keystrokes to undo phantom scroll.
	m := baseModelSearch()
	result, _ := m.handleHelpKey(keyMsg("G"))
	rm := result.(Model)
	assert.Less(t, rm.helpScroll, 9999, "G must clamp the sentinel to actual max")
	assert.Greater(t, rm.helpScroll, 0, "G should still scroll past the top")
}

func TestCovHelpKeyCtrlD(t *testing.T) {
	m := baseModelSearch()
	m.helpScroll = 0
	result, _ := m.handleHelpKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Equal(t, 20, rm.helpScroll) // height/2 = 40/2
}

func TestCovHelpKeyCtrlU(t *testing.T) {
	m := baseModelSearch()
	m.helpScroll = 30
	result, _ := m.handleHelpKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Equal(t, 10, rm.helpScroll) // 30 - 20
}

func TestCovHelpKeyCtrlUClamp(t *testing.T) {
	m := baseModelSearch()
	m.helpScroll = 5
	result, _ := m.handleHelpKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.helpScroll)
}

func TestCovHelpKeyCtrlF(t *testing.T) {
	m := baseModelSearch()
	m.helpScroll = 0
	result, _ := m.handleHelpKey(keyMsg("ctrl+f"))
	rm := result.(Model)
	assert.Equal(t, 40, rm.helpScroll) // height
}

func TestCovHelpKeyCtrlB(t *testing.T) {
	m := baseModelSearch()
	m.helpScroll = 50
	result, _ := m.handleHelpKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.Equal(t, 10, rm.helpScroll) // 50 - 40
}

func TestCovHelpKeyCtrlBClamp(t *testing.T) {
	m := baseModelSearch()
	m.helpScroll = 10
	result, _ := m.handleHelpKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.helpScroll)
}

func TestCovHelpKeySlash(t *testing.T) {
	m := baseModelSearch()
	_, cmd := m.handleHelpKey(keyMsg("/"))
	assert.NotNil(t, cmd)
}

func TestCovHelpKeyDefault(t *testing.T) {
	m := baseModelSearch()
	result, _ := m.handleHelpKey(keyMsg("x"))
	_ = result.(Model)
}

func TestCovSearchKeyEnter(t *testing.T) {
	m := baseModelSearch()
	m.searchActive = true
	result, _ := m.handleSearchKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.searchActive)
}

func TestCovSearchKeyEsc(t *testing.T) {
	m := baseModelSearch()
	m.searchActive = true
	m.searchInput.Insert("test")
	m.searchPrevCursor = 3
	result, _ := m.handleSearchKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.searchActive)
	assert.Empty(t, rm.searchInput.Value)
}

func TestCovSearchKeyBackspace(t *testing.T) {
	m := baseModelSearch()
	m.searchActive = true
	m.searchInput.Insert("ab")
	result, _ := m.handleSearchKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "a", rm.searchInput.Value)
}

func TestCovSearchKeyCtrlW(t *testing.T) {
	m := baseModelSearch()
	m.searchActive = true
	m.searchInput.Insert("foo bar")
	result, _ := m.handleSearchKey(keyMsg("ctrl+w"))
	rm := result.(Model)
	assert.NotEqual(t, "foo bar", rm.searchInput.Value)
}

func TestCovSearchKeyCtrlA(t *testing.T) {
	m := baseModelSearch()
	m.searchActive = true
	result, _ := m.handleSearchKey(keyMsg("ctrl+a"))
	_ = result.(Model)
}

func TestCovSearchKeyCtrlE(t *testing.T) {
	m := baseModelSearch()
	m.searchActive = true
	result, _ := m.handleSearchKey(keyMsg("ctrl+e"))
	_ = result.(Model)
}

func TestCovSearchKeyLeftRight(t *testing.T) {
	m := baseModelSearch()
	m.searchActive = true
	m.searchInput.Insert("abc")
	result, _ := m.handleSearchKey(keyMsg("left"))
	rm := result.(Model)
	result, _ = rm.handleSearchKey(keyMsg("right"))
	_ = result.(Model)
}

func TestCovSearchKeyCtrlN(t *testing.T) {
	m := baseModelSearch()
	m.searchActive = true
	m.searchInput.Insert("line")
	m.middleItems = []model.Item{{Name: "line1"}, {Name: "line2"}}
	result, _ := m.handleSearchKey(keyMsg("ctrl+n"))
	_ = result.(Model)
}

func TestCovSearchKeyCtrlP(t *testing.T) {
	m := baseModelSearch()
	m.searchActive = true
	m.searchInput.Insert("line")
	m.middleItems = []model.Item{{Name: "line1"}, {Name: "line2"}}
	result, _ := m.handleSearchKey(keyMsg("ctrl+p"))
	_ = result.(Model)
}

func TestCovSearchKeyTyping(t *testing.T) {
	m := baseModelSearch()
	m.searchActive = true
	result, _ := m.handleSearchKey(keyMsg("x"))
	rm := result.(Model)
	assert.Equal(t, "x", rm.searchInput.Value)
}

func TestCovSearchKeyBackspaceEmpty(t *testing.T) {
	m := baseModelSearch()
	m.searchActive = true
	result, _ := m.handleSearchKey(keyMsg("backspace"))
	_ = result.(Model)
}

func TestCovFilterKeyEnter(t *testing.T) {
	m := baseModelSearch()
	m.filterActive = true
	m.filterInput.Insert("test")
	result, _ := m.handleFilterKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.filterActive)
	assert.Equal(t, "test", rm.filterText)
}

func TestCovFilterKeyEsc(t *testing.T) {
	m := baseModelSearch()
	m.filterActive = true
	m.filterInput.Insert("test")
	result, _ := m.handleFilterKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.filterActive)
	assert.Empty(t, rm.filterText)
}

func TestCovFilterKeyBackspace(t *testing.T) {
	m := baseModelSearch()
	m.filterActive = true
	m.filterInput.Insert("ab")
	result, _ := m.handleFilterKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "a", rm.filterText)
}

func TestCovFilterKeyBackspaceEmpty(t *testing.T) {
	m := baseModelSearch()
	m.filterActive = true
	result, _ := m.handleFilterKey(keyMsg("backspace"))
	_ = result.(Model)
}

func TestCovFilterKeyCtrlW(t *testing.T) {
	m := baseModelSearch()
	m.filterActive = true
	m.filterInput.Insert("foo bar")
	result, _ := m.handleFilterKey(keyMsg("ctrl+w"))
	rm := result.(Model)
	assert.NotEqual(t, "foo bar", rm.filterText)
}

func TestCovFilterKeyCtrlA(t *testing.T) {
	m := baseModelSearch()
	m.filterActive = true
	result, _ := m.handleFilterKey(keyMsg("ctrl+a"))
	_ = result.(Model)
}

func TestCovFilterKeyCtrlE(t *testing.T) {
	m := baseModelSearch()
	m.filterActive = true
	result, _ := m.handleFilterKey(keyMsg("ctrl+e"))
	_ = result.(Model)
}

func TestCovFilterKeyLeftRight(t *testing.T) {
	m := baseModelSearch()
	m.filterActive = true
	m.filterInput.Insert("abc")
	result, _ := m.handleFilterKey(keyMsg("left"))
	rm := result.(Model)
	result, _ = rm.handleFilterKey(keyMsg("right"))
	_ = result.(Model)
}

func TestCovFilterKeyTyping(t *testing.T) {
	m := baseModelSearch()
	m.filterActive = true
	result, _ := m.handleFilterKey(keyMsg("x"))
	rm := result.(Model)
	assert.Equal(t, "x", rm.filterText)
}

func TestCovCommandBarKeyHelpSearchActive(t *testing.T) {
	m := baseModelSearch()
	m.helpSearchActive = true
	result, _ := m.handleHelpKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	_ = result
}

func TestCovHandleFilterKeyAllActions(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		expect filterAction
	}{
		{"escape", "esc", filterEscape},
		{"enter", "enter", filterAccept},
		{"ctrl+c", "ctrl+c", filterClose},
		{"backspace", "backspace", filterContinue},
		{"ctrl+w", "ctrl+w", filterContinue},
		{"ctrl+a home", "ctrl+a", filterNavigate},
		{"ctrl+e end", "ctrl+e", filterNavigate},
		{"left", "left", filterNavigate},
		{"right", "right", filterNavigate},
		{"printable char", "a", filterContinue},
		{"multi-char key ignored", "f1", filterIgnored},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &TextInput{Value: "hello", Cursor: 3}
			result := handleFilterKey(ti, tt.key)
			assert.Equal(t, tt.expect, result)
		})
	}
}
