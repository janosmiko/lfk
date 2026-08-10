package app

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- wrappedLineCount ---

func TestWrappedLineCount(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		width    int
		expected int
	}{
		{
			name:     "empty line returns 1",
			line:     "",
			width:    80,
			expected: 1,
		},
		{
			name:     "short line fits in one row",
			line:     "hello",
			width:    80,
			expected: 1,
		},
		{
			name:     "line exactly fills width",
			line:     strings.Repeat("a", 80),
			width:    80,
			expected: 1,
		},
		{
			name:     "line wraps to two rows",
			line:     strings.Repeat("a", 81),
			width:    80,
			expected: 2,
		},
		{
			name:     "line wraps to three rows",
			line:     strings.Repeat("a", 161),
			width:    80,
			expected: 3,
		},
		{
			name:     "zero width returns 1",
			line:     "hello",
			width:    0,
			expected: 1,
		},
		{
			name:     "negative width returns 1",
			line:     "hello",
			width:    -5,
			expected: 1,
		},
		{
			name:     "single char width",
			line:     "abc",
			width:    1,
			expected: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, wrappedLineCount(tt.line, tt.width))
		})
	}
}

// --- clampLogScroll ---

func TestClampLogScrollNoWrap(t *testing.T) {
	t.Run("clamps scroll past end", func(t *testing.T) {
		m := Model{
			height: 20,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:  make([]string, 10),
				scroll: 100,
			},
		}
		m.clampLogScroll()
		assert.LessOrEqual(t, m.logView.scroll, len(m.logView.lines))
		assert.GreaterOrEqual(t, m.logView.scroll, 0)
	})

	t.Run("zero scroll stays zero", func(t *testing.T) {
		m := Model{
			height: 20,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:  make([]string, 10),
				scroll: 0,
			},
		}
		m.clampLogScroll()
		assert.Equal(t, 0, m.logView.scroll)
	})

	t.Run("negative scroll clamped to zero", func(t *testing.T) {
		m := Model{
			height: 20,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:  make([]string, 10),
				scroll: -5,
			},
		}
		m.clampLogScroll()
		assert.Equal(t, 0, m.logView.scroll)
	})

	t.Run("fewer lines than viewport keeps scroll at zero", func(t *testing.T) {
		m := Model{
			height: 100,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:  make([]string, 5),
				scroll: 3,
			},
		}
		m.clampLogScroll()
		assert.Equal(t, 0, m.logView.scroll)
	})
}

func TestClampLogScrollWithWrap(t *testing.T) {
	t.Run("wrapping clamps scroll correctly", func(t *testing.T) {
		lines := make([]string, 5)
		for i := range lines {
			lines[i] = strings.Repeat("x", 10) // short lines
		}
		m := Model{
			height: 100, // tall viewport
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:  lines,
				wrap:   true,
				scroll: 50,
			},
		}
		m.clampLogScroll()
		assert.GreaterOrEqual(t, m.logView.scroll, 0)
		assert.LessOrEqual(t, m.logView.scroll, len(lines))
	})

	t.Run("wrap with long last line pins tail to bottom via topSkip", func(t *testing.T) {
		// Regression: when following and the most recent log line wraps
		// to more visual rows than fit in viewH, maxScroll alone (source
		// line index) couldn't position the bottom of the wrapped output
		// at the bottom of the viewport — the renderer filled top-down
		// and the tail of the line dropped off the bottom. Verify
		// logMaxScrollAndSkip returns a non-zero topSkip in that case.
		// width=40 → contentWidth=36, availWidth=35.
		// 350-char line wraps to 10 sub-lines.
		longLine := strings.Repeat("x", 350)
		m := Model{
			height: 12, // viewH ≈ 12-5 = 7
			width:  40,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:      []string{longLine},
				wrap:       true,
				timestamps: true, // no stripping
			},
		}
		viewH := m.logContentHeight()
		ms, topSkip := m.logMaxScrollAndSkip()
		assert.Equal(t, 0, ms, "scroll stays on the only source line")
		assert.Greater(t, topSkip, 0, "topSkip must drop wrapped sub-lines so tail fits")
		// 350 / 35 = 10 wrapped sub-lines; viewH=7 → topSkip=3.
		assert.Equal(t, 10-viewH, topSkip)
	})

	t.Run("wrap-aware maxScroll uses display line, not raw line with timestamp", func(t *testing.T) {
		// Regression: clampLogScroll/logMaxScroll counted wraps on the
		// raw log lines (timestamps included), but the renderer strips
		// timestamps before wrapping. Overestimating wraps shrank
		// maxScroll, which pushed the tail off the bottom when following.
		//
		// Construct a case where each raw line wraps to 2 visual lines
		// but stripped down to message-only content fits in 1.
		lines := make([]string, 0, 100)
		for i := range 100 {
			lines = append(lines, fmt.Sprintf("2024-01-15T10:30:%02d.000000000Z msg %d", i%60, i))
		}
		// width=60: contentWidth=56, availWidth=55 (no line numbers).
		// Raw line "2024-01-15T10:30:00.000000000Z msg 0" is ~36 chars
		// (fits in 1 wrap). With a longer message we can force it.
		// Use a long-enough message that the raw line wraps but the
		// stripped one doesn't.
		for i := range lines {
			lines[i] += " " + strings.Repeat("y", 30) // raw ~66, stripped ~34
		}
		m := Model{
			height: 20, // viewport height includes overhead
			width:  60, // content width 56, avail ~55
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:      lines,
				wrap:       true,
				timestamps: false, // timestamps stripped at render
			},
		}
		ms := m.logMaxScroll()
		// With stripped lines fitting on one visual line each and
		// viewH around 14, maxScroll should be near len(lines)-viewH.
		// Raw-line counting would wildly underestimate this (because
		// raw lines wrap to 2 each, halving the source-line capacity).
		viewH := m.logContentHeight()
		expected := len(lines) - viewH
		assert.Equal(t, expected, ms,
			"maxScroll should match the no-wrap value when stripped lines fit on one row each")
	})
}

// --- ensureLogCursorVisible ---

func TestEnsureLogCursorVisible(t *testing.T) {
	t.Run("cursor above viewport scrolls up with scrolloff", func(t *testing.T) {
		m := Model{
			height: 20,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:  make([]string, 100),
				scroll: 50,
				cursor: 10,
			},
		}
		m.ensureLogCursorVisible()
		// Scroll should position cursor with scrolloff margin above it.
		so := ui.ConfigScrollOff
		assert.Equal(t, 10-so, m.logView.scroll)
	})

	t.Run("cursor below viewport scrolls down", func(t *testing.T) {
		m := Model{
			height: 20,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:  make([]string, 100),
				scroll: 0,
				cursor: 50,
			},
		}
		m.ensureLogCursorVisible()
		viewH := m.logContentHeight()
		assert.GreaterOrEqual(t, m.logView.scroll, 50-viewH)
	})

	t.Run("negative cursor is no-op", func(t *testing.T) {
		m := Model{
			height: 20,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:  make([]string, 100),
				scroll: 10,
				cursor: -1,
			},
		}
		m.ensureLogCursorVisible()
		assert.Equal(t, 10, m.logView.scroll)
	})

	t.Run("cursor past end is clamped", func(t *testing.T) {
		m := Model{
			height: 20,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines:  make([]string, 10),
				scroll: 0,
				cursor: 100,
			},
		}
		m.ensureLogCursorVisible()
		assert.Equal(t, 9, m.logView.cursor)
	})
}

// --- logMaxScroll ---

func TestLogMaxScroll(t *testing.T) {
	t.Run("fewer lines than viewport returns zero", func(t *testing.T) {
		m := Model{
			height: 100,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines: make([]string, 5),
			},
		}
		assert.Equal(t, 0, m.logMaxScroll())
	})

	t.Run("more lines than viewport returns positive", func(t *testing.T) {
		m := Model{
			height: 20,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines: make([]string, 100),
			},
		}
		ms := m.logMaxScroll()
		assert.Greater(t, ms, 0)
		viewH := m.logContentHeight()
		assert.Equal(t, len(m.logView.lines)-viewH, ms)
	})

	t.Run("wrap mode returns valid max scroll", func(t *testing.T) {
		lines := make([]string, 10)
		for i := range lines {
			lines[i] = "short"
		}
		m := Model{
			height: 100,
			width:  80,
			tabs:   []TabState{{}},
			logView: logViewState{
				lines: lines,
				wrap:  true,
			},
		}
		ms := m.logMaxScroll()
		assert.GreaterOrEqual(t, ms, 0)
	})

	t.Run("empty log returns zero", func(t *testing.T) {
		m := Model{
			height: 20,
			width:  80,
			tabs:   []TabState{{}},
		}
		assert.Equal(t, 0, m.logMaxScroll())
	})
}

// --- viewDescribe ---

func TestViewDescribe(t *testing.T) {
	t.Run("renders title and content", func(t *testing.T) {
		m := Model{
			width:  120,
			height: 30,
			describeView: describeViewState{
				title:   "Describe: my-pod",
				content: "Name:         my-pod\nNamespace:    default\nStatus:       Running",
			},
		}
		output := m.viewDescribe()
		stripped := stripANSI(output)
		assert.Contains(t, stripped, "Describe: my-pod")
		assert.Contains(t, stripped, "my-pod")
		assert.Contains(t, stripped, "navigate")
		assert.Contains(t, stripped, "back")
	})

	t.Run("respects scroll offset", func(t *testing.T) {
		lines := make([]string, 50)
		for i := range lines {
			lines[i] = strings.Repeat("x", 10)
		}
		m := Model{
			width:  80,
			height: 30,
			describeView: describeViewState{
				title:   "Test",
				content: strings.Join(lines, "\n"),
				scroll:  10,
			},
		}
		output := m.viewDescribe()
		assert.NotEmpty(t, output)
	})

	t.Run("small height renders correctly", func(t *testing.T) {
		m := Model{
			width:  80,
			height: 5,
			describeView: describeViewState{
				title:   "Test",
				content: "line1\nline2\nline3",
			},
		}
		output := m.viewDescribe()
		assert.NotEmpty(t, output)
	})
}

// --- viewDescribe: sanitizing ---
//
// describeView.content is fed by kubectl describe, helm values, command-bar
// output, trivy scans, and gitops watch — none of it lfk-controlled. This
// sink must run the ANSI-aware sanitizer (ui.SanitizeLogBody), not the
// name/title one, or SGR colour and tab alignment break.

func TestViewDescribe_StripsInjectedEscapesFromContent(t *testing.T) {
	m := Model{
		width:  120,
		height: 30,
		describeView: describeViewState{
			title:   "Command Output",
			content: "before‮after\x1b[2Jgone\x1b]52;c;ZXZpbA==\x07tail",
		},
	}
	output := m.viewDescribe()
	assert.NotContains(t, output, "‮", "bidi override must not survive")
	assert.NotContains(t, output, "\x1b[2J", "raw CSI (screen erase) must not survive")
	assert.NotContains(t, output, "\x1b]52", "OSC-52 clipboard sequence must not survive")
	assert.NotContains(t, output, "\x07")
}

func TestViewDescribe_StripsInjectedEscapesInWrapMode(t *testing.T) {
	m := Model{
		width:  120,
		height: 30,
		describeView: describeViewState{
			title:   "Command Output",
			content: "before‮after\x1b[2Jgone",
			wrap:    true,
		},
	}
	output := m.viewDescribe()
	assert.NotContains(t, output, "‮")
	assert.NotContains(t, output, "\x1b[2J")
}

func TestViewDescribe_PreservesSGRColour(t *testing.T) {
	// Realistic trivy-style coloured table output. SGR must survive so the
	// severity colouring the scanner intended still renders — the guard
	// against fixing the injection issue by breaking the feature.
	trivyLine := "\x1b[31mCRITICAL\x1b[0m CVE-2024-1234  openssl  1.1.1  1.1.1w"
	m := Model{
		width:  120,
		height: 30,
		describeView: describeViewState{
			title:   "Vuln Scan: nginx:latest",
			content: trivyLine,
		},
	}
	output := m.viewDescribe()
	assert.Contains(t, output, "\x1b[31m", "SGR red-severity colour must survive the describe path")
}

func TestViewDescribe_PreservesTabAlignment(t *testing.T) {
	// kubectl describe output is tab-and-space aligned; tabs must expand to
	// the same column stops the log viewer uses, not vanish or collapse.
	m := Model{
		width:  120,
		height: 30,
		describeView: describeViewState{
			title:   "Describe: my-pod",
			content: "Name:\tmy-pod\nNamespace:\tdefault",
		},
	}
	output := m.viewDescribe()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "Name:   my-pod")
	assert.Contains(t, stripped, "Namespace:      default")
}

func TestViewDescribe_OrdinaryContentUnaffected(t *testing.T) {
	m := Model{
		width:  120,
		height: 30,
		describeView: describeViewState{
			title:   "Describe: my-pod",
			content: "Name:         my-pod\nNamespace:    default\nStatus:       Running",
		},
	}
	output := m.viewDescribe()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "Name:         my-pod")
	assert.Contains(t, stripped, "Namespace:    default")
	assert.Contains(t, stripped, "Status:       Running")
}

// --- viewDiff ---

func TestViewDiff(t *testing.T) {
	t.Run("unified mode calls unified renderer", func(t *testing.T) {
		m := Model{
			width:  80,
			height: 30,
			diffView: diffViewState{
				left:      "line1\nline2\n",
				right:     "line1\nline3\n",
				leftName:  "old.yaml",
				rightName: "new.yaml",
				unified:   true,
			},
		}
		output := m.viewDiff()
		assert.NotEmpty(t, output)
	})

	t.Run("side-by-side mode", func(t *testing.T) {
		m := Model{
			width:  80,
			height: 30,
			diffView: diffViewState{
				left:      "same\nold\n",
				right:     "same\nnew\n",
				leftName:  "before",
				rightName: "after",
				unified:   false,
			},
		}
		output := m.viewDiff()
		assert.NotEmpty(t, output)
	})
}

// --- viewLogs ---

func TestViewLogs(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		tabs:   []TabState{{}},
		logView: logViewState{
			lines: []string{"line 1", "line 2", "line 3"},
			title: "Logs: my-pod",
		},
	}
	output := m.viewLogs()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "Logs: my-pod")
}

// --- View with different modes ---

func TestViewDescribeMode(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		mode:   modeDescribe,
		describeView: describeViewState{
			title:   "Describe: test",
			content: "Name: test-pod\nStatus: Running",
		},
		tabs: []TabState{{}},
	}
	output := m.View().Content
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "Describe: test")
}

func TestViewLogsMode(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		mode:   modeLogs,
		logView: logViewState{
			lines: []string{"log line 1"},
			title: "Logs: my-pod",
		},
		tabs: []TabState{{}},
	}
	output := m.View().Content
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "Logs: my-pod")
}

func TestViewYAMLMode(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		mode:   modeYAML,
		nav: model.NavigationState{
			Level: model.LevelResources,
		},
		namespace:   "default",
		middleItems: []model.Item{{Name: "my-configmap"}},
		yamlView: yamlViewState{
			content:   "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: my-configmap",
			collapsed: make(map[string]bool),
		},
		tabs: []TabState{{}},
	}
	output := m.View().Content
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "YAML")
}

func TestViewDiffMode(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		mode:   modeDiff,
		diffView: diffViewState{
			left:      "old content",
			right:     "new content",
			leftName:  "before",
			rightName: "after",
		},
		tabs: []TabState{{}},
	}
	output := m.View().Content
	assert.NotEmpty(t, output)
}

func TestViewExplainMode(t *testing.T) {
	m := Model{
		width:        120,
		height:       30,
		mode:         modeExplain,
		explainTitle: "Explain: Deployment",
		explainDesc:  "A Deployment provides declarative updates for Pods.",
		explainFields: []model.ExplainField{
			{Name: "apiVersion", Type: "<string>", Description: "API version"},
			{Name: "kind", Type: "<string>", Description: "Kind of resource"},
			{Name: "spec", Type: "<DeploymentSpec>", Description: "Desired state"},
		},
		tabs: []TabState{{}},
	}
	output := m.View().Content
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "Explain: Deployment") // title line is back
	assert.Contains(t, stripped, "apiVersion")
}

func TestViewHelpMode(t *testing.T) {
	m := Model{
		width:  120,
		height: 40,
		mode:   modeHelp,
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
			},
		},
		middleItems:   []model.Item{{Name: "test-pod"}},
		namespace:     "default",
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
	}
	output := m.View().Content
	stripped := stripANSI(output)
	// Help mode renders an overlay on top of explorer view.
	assert.NotEmpty(t, stripped)
}

func TestViewWithTabs(t *testing.T) {
	m := Model{
		width:  120,
		height: 30,
		mode:   modeDescribe,
		describeView: describeViewState{
			title:   "Describe: test",
			content: "Name: test\n",
		},
		nav: model.NavigationState{
			Context: "active-ctx",
		},
		tabs: []TabState{
			{nav: model.NavigationState{Context: "ctx-1"}},
			{nav: model.NavigationState{Context: "ctx-2"}},
		},
		activeTab: 0,
	}
	output := m.View().Content
	stripped := stripANSI(output)
	// Tab labels are derived from context names.
	assert.Contains(t, stripped, "active-ctx")
}

// --- vt10xColorToLipgloss ---

func TestVt10xColorToLipgloss(t *testing.T) {
	color := vt10xColorToLipgloss(vt10x.Color(1))
	assert.NotNil(t, color)

	color2 := vt10xColorToLipgloss(vt10x.Color(255))
	assert.NotNil(t, color2)
}

func TestPush3ViewHelpNotEmpty(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeHelp
	result := m.View().Content
	assert.NotEmpty(t, result)
}

func TestPush3ViewExplorerNotEmpty(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeExplorer
	result := m.View().Content
	assert.NotEmpty(t, result)
}

func TestPush3ViewYAMLNotEmpty(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeYAML
	m.yamlView.content = "apiVersion: v1"
	result := m.View().Content
	assert.NotEmpty(t, result)
}

func TestPush3ViewDiffNotEmpty(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeDiff
	m.diffView.left = "left"
	m.diffView.right = "right"
	result := m.View().Content
	assert.NotEmpty(t, result)
}

func TestPush3ViewDescribeNotEmpty(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeDescribe
	m.describeView.content = "Name: pod-1\nStatus: Running"
	result := m.View().Content
	assert.NotEmpty(t, result)
}

func TestPush3ViewExplainNotEmpty(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeExplain
	m.explainTitle = "pods"
	m.explainFields = []model.ExplainField{
		{Name: "apiVersion", Type: "string", Description: "API version"},
	}
	result := m.View().Content
	assert.NotEmpty(t, result)
}

func TestPush3ViewFullscreenDashboard(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeExplorer
	m.fullscreenDashboard = true
	m.dashboardPreview = "CLUSTER DASHBOARD\n..."
	result := m.View().Content
	assert.NotEmpty(t, result)
}

func TestPush3ViewWithTabs(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeExplorer
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0
	result := m.View().Content
	assert.NotEmpty(t, result)
}

func TestPush3ViewWithError(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeExplorer
	m.err = assert.AnError
	result := m.View().Content
	assert.NotEmpty(t, result)
}

func TestPush3ViewWithSelection(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeExplorer
	m.selectedItems = map[string]bool{"default/pod-1": true}
	result := m.View().Content
	assert.NotEmpty(t, result)
}

func TestPush3ViewFullscreenMiddle(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeExplorer
	m.fullscreenMiddle = true
	result := m.View().Content
	assert.NotEmpty(t, result)
}

func TestCovViewExecTerminalSmall(t *testing.T) {
	m := Model{
		width: 15, height: 8, mode: modeExec, execTitle: "Exec",
		tabs: []TabState{{}}, execMu: &sync.Mutex{},
	}
	assert.NotEmpty(t, m.viewExecTerminal())
}

func TestCovViewLogsBasic(t *testing.T) {
	m := Model{
		width: 80, height: 30, tabs: []TabState{{}},
		logView: logViewState{
			lines: []string{"line 1", "line 2"}, title: "Logs: pod",
			follow: true, searchInput: TextInput{},
		},
		actionCtx: actionContext{kind: "Pod"},
	}
	assert.NotEmpty(t, m.viewLogs())
}

func TestCovViewLogsSearch(t *testing.T) {
	m := Model{
		width: 80, height: 30, tabs: []TabState{{}},
		logView: logViewState{
			lines: []string{"line"}, title: "Logs",
			searchActive: true, searchInput: TextInput{Value: "err"},
		},
		actionCtx: actionContext{kind: "Pod"},
	}
	assert.NotEmpty(t, m.viewLogs())
}

func TestCovViewLogsStatus(t *testing.T) {
	m := Model{
		width: 80, height: 30, tabs: []TabState{{}},
		logView: logViewState{
			lines: []string{"line"}, title: "Logs",
			searchInput: TextInput{},
		},
		statusMessage: "Copied", actionCtx: actionContext{kind: "Pod"},
	}
	assert.NotEmpty(t, m.viewLogs())
}

func TestCovLoadPreviewYAMLNil(t *testing.T) {
	m := baseModelWithFakeClient()
	m.middleItems = nil
	cmd := m.loadPreviewYAML()
	assert.Nil(t, cmd)
}

func TestCovEventViewerModeKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeEventViewer
	m.eventTimelineLines = []string{"line1"}
	result, _ := m.handleEventViewerModeKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovEventViewerModeKeyF(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeEventViewer
	m.eventTimelineFullscreen = true
	m.eventTimelineLines = []string{"line1"}
	result, _ := m.handleEventViewerModeKey(keyMsg("F"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
	assert.Equal(t, overlayEventTimeline, rm.overlay)
}

func TestCovHighlightDescribeSearchLineEmpty(t *testing.T) {
	assert.Equal(t, "hello world", highlightDescribeSearchLine("hello world", ""))
}

func TestCovHighlightDescribeSearchLineMatch(t *testing.T) {
	// The function expects a pre-lowered query.
	result := highlightDescribeSearchLine("Hello World", "hello")
	// Result contains "Hello" (possibly styled), always non-empty.
	assert.Contains(t, result, "World")
}

func TestCovHighlightDescribeSearchLineNoMatch(t *testing.T) {
	result := highlightDescribeSearchLine("Hello World", "xyz")
	assert.Equal(t, "Hello World", result)
}

func TestCovHighlightDescribeSearchLineMultiple(t *testing.T) {
	result := highlightDescribeSearchLine("the the the", "the")
	// Result should contain all three occurrences (possibly with styling).
	assert.NotEmpty(t, result)
}

func TestCovViewEventViewer(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
		mode:   modeEventViewer,
		eventTimelineLines: []string{
			"10:00 Normal pod-1 Started",
			"10:01 Warning pod-2 OOMKilled",
		},
		eventTimelineSearchInput: TextInput{},
		actionCtx:                actionContext{name: "my-pod"},
	}
	result := m.viewEventViewer()
	assert.NotEmpty(t, result)
}

func TestCovViewEventViewerWithSearch(t *testing.T) {
	m := Model{
		width:                     80,
		height:                    30,
		tabs:                      []TabState{{}},
		execMu:                    &sync.Mutex{},
		mode:                      modeEventViewer,
		eventTimelineLines:        []string{"line1"},
		eventTimelineSearchActive: true,
		eventTimelineSearchInput:  TextInput{Value: "err"},
	}
	result := m.viewEventViewer()
	assert.NotEmpty(t, result)
}

func TestCovViewEventViewerWithVisualMode(t *testing.T) {
	m := Model{
		width:                    80,
		height:                   30,
		tabs:                     []TabState{{}},
		execMu:                   &sync.Mutex{},
		mode:                     modeEventViewer,
		eventTimelineLines:       []string{"line1"},
		eventTimelineVisualMode:  'V',
		eventTimelineSearchInput: TextInput{},
	}
	result := m.viewEventViewer()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "VISUAL LINE")
}

func TestCovViewEventViewerWrap(t *testing.T) {
	m := Model{
		width:                    80,
		height:                   30,
		tabs:                     []TabState{{}},
		execMu:                   &sync.Mutex{},
		mode:                     modeEventViewer,
		eventTimelineLines:       []string{"line1"},
		eventTimelineWrap:        true,
		eventTimelineSearchInput: TextInput{},
	}
	result := m.viewEventViewer()
	assert.Contains(t, result, "WRAP")
}

func TestCovViewExplain(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
		mode:   modeExplain,
		explainFields: []model.ExplainField{
			{Name: "spec", Type: "Object"},
			{Name: "status", Type: "Object"},
		},
		explainTitle:       "pods",
		explainSearchInput: TextInput{},
	}
	result := m.viewExplain()
	assert.NotEmpty(t, result)
}

func TestCovViewExplainWithSearch(t *testing.T) {
	m := Model{
		width:               80,
		height:              30,
		tabs:                []TabState{{}},
		execMu:              &sync.Mutex{},
		mode:                modeExplain,
		explainSearchActive: true,
		explainSearchInput:  TextInput{Value: "spec"},
		explainFields:       []model.ExplainField{{Name: "spec"}},
		explainTitle:        "pods",
	}
	result := m.viewExplain()
	assert.NotEmpty(t, result)
}

func TestCovViewExplainWithQuery(t *testing.T) {
	m := Model{
		width:              80,
		height:             30,
		tabs:               []TabState{{}},
		execMu:             &sync.Mutex{},
		mode:               modeExplain,
		explainSearchQuery: "status",
		explainSearchInput: TextInput{},
		explainFields:      []model.ExplainField{{Name: "spec"}, {Name: "status"}},
		explainTitle:       "pods",
	}
	result := m.viewExplain()
	assert.NotEmpty(t, result)
}

func TestCovLogViewHeight(t *testing.T) {
	m := Model{height: 30}
	assert.Equal(t, 28, m.logViewHeight())

	m.height = 3
	assert.Equal(t, 3, m.logViewHeight())
}

func TestCovLogContentHeight(t *testing.T) {
	m := Model{height: 30}
	h := m.logContentHeight()
	assert.Greater(t, h, 0)
}

func TestCovRenderSplitPreview(t *testing.T) {
	m := Model{
		width:        120,
		height:       30,
		tabs:         []TabState{{}},
		execMu:       &sync.Mutex{},
		nav:          model.NavigationState{Level: model.LevelResources, ResourceType: model.ResourceTypeEntry{Kind: "Pod"}},
		splitPreview: true,
		previewYAML:  "apiVersion: v1\nkind: Pod",
		yamlView: yamlViewState{
			content: "apiVersion: v1\nkind: Pod",
		},
		middleItems: []model.Item{{Name: "pod-1", Namespace: "ns1", Status: "Running"}},
		cursors:     [5]int{},
	}
	result := m.renderSplitPreview(60, 25)
	assert.NotEmpty(t, result)
}

func TestCovViewDescribeWithContent(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
		mode:   modeDescribe,
		describeView: describeViewState{
			content:     "Name: nginx\nNamespace: default\nStatus: Running",
			title:       "Describe: pods/nginx",
			searchInput: TextInput{},
		},
	}
	result := m.viewDescribe()
	assert.NotEmpty(t, result)
}

func TestCovViewDescribeSearchActive(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
		mode:   modeDescribe,
		describeView: describeViewState{
			content:      "Name: nginx",
			title:        "Describe",
			searchActive: true,
			searchInput:  TextInput{Value: "Name"},
		},
	}
	result := m.viewDescribe()
	assert.NotEmpty(t, result)
}

func TestCovViewDescribeWithSearchQuery(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
		mode:   modeDescribe,
		describeView: describeViewState{
			content:     "Name: nginx\nStatus: Running",
			title:       "Describe",
			searchQuery: "Status",
			searchInput: TextInput{},
		},
	}
	result := m.viewDescribe()
	assert.NotEmpty(t, result)
}

func TestCovViewDescribeVisualMode(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
		mode:   modeDescribe,
		describeView: describeViewState{
			content:     "Name: nginx\nStatus: Running",
			title:       "Describe",
			visualMode:  'V',
			searchInput: TextInput{},
		},
	}
	result := m.viewDescribe()
	assert.Contains(t, result, "VISUAL LINE")
}

func TestCovViewDiff(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
		mode:   modeDiff,
		diffView: diffViewState{
			left:       "key: default\n",
			right:      "key: custom\n",
			leftName:   "Default Values",
			rightName:  "User Values",
			searchText: TextInput{},
		},
	}
	result := m.viewDiff()
	assert.NotEmpty(t, result)
}

func TestCovViewDiffUnified(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		tabs:   []TabState{{}},
		execMu: &sync.Mutex{},
		mode:   modeDiff,
		diffView: diffViewState{
			left:       "key: default\n",
			right:      "key: custom\n",
			leftName:   "Default",
			rightName:  "User",
			unified:    true,
			searchText: TextInput{},
		},
	}
	result := m.viewDiff()
	assert.NotEmpty(t, result)
}
