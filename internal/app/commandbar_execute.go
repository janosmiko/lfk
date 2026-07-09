package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// kubectlEnv returns environment variables for subprocess execution,
// including KUBECONFIG and KUBECTL_CONTEXT from the current navigation state.
func (m Model) kubectlEnv() []string {
	env := os.Environ()
	if m.client != nil {
		ctx := m.nav.Context
		if ctx == "" {
			ctx = m.client.CurrentContext()
		}
		// Use only the kubeconfig file that defines the current context so
		// kubectl doesn't trip over collapsed cluster/user names from a
		// multi-file merge — see issue #23.
		kubeconfigPath := m.client.KubeconfigPathForContext(ctx)
		if kubeconfigPath != "" {
			env = append(env, "KUBECONFIG="+kubeconfigPath)
		}
		// KUBECTL_CONTEXT is consumed by the helper that runs `:!` shell
		// commands and forwarded as kubectl's default context. It must be
		// the kubeconfig's *original* name, not lfk's potentially
		// disambiguated display name.
		if ctx != "" {
			env = append(env, "KUBECTL_CONTEXT="+m.client.OriginalContextName(ctx))
		}
	}
	return env
}

// kubectlContext maps an lfk display name back to the kubectl/helm --context
// argument it should be passed as. When two kubeconfigs declare the same
// context name, lfk disambiguates the display name (e.g. "dev (dev-envs)");
// this reverses that so subprocess invocations receive the literal name
// kubectl knows.
func (m Model) kubectlContext(displayName string) string {
	if m.client == nil {
		return displayName
	}
	return m.client.OriginalContextName(displayName)
}

// executeCommandBarInput is the main entry point for command bar execution.
// It classifies the input and dispatches to the appropriate handler.
func (m Model) executeCommandBarInput(input string) (tea.Model, tea.Cmd) {
	input = strings.TrimSpace(input)
	if input == "" {
		return m, nil
	}

	crdNames := extractCRDNames(&m)
	classified := classifyInputWithCRDs(input, crdNames)
	// Shell-out and kubectl commands inject --context from nav.Context. At the
	// union sentinel that would pass "__union__" to real kubectl and fail.
	// Block until the user drills into a specific cluster.
	if m.isUnionSentinel() && (classified == cmdShell || classified == cmdKubectl) {
		m.setStatusMessage("Shell and kubectl commands require a single cluster — drill into a resource first", true)
		return m, scheduleStatusClear()
	}
	switch classified {
	case cmdShell:
		return m, m.executeShellCommand(extractShellCommand(input))
	case cmdBuiltin:
		return m.executeBuiltinCommand(input)
	case cmdKubectl:
		// Easter egg: "kubectl explain life" or "k explain life".
		trimmed := strings.TrimSpace(input)
		trimmed = strings.TrimPrefix(trimmed, "kubectl ")
		trimmed = strings.TrimPrefix(trimmed, "k ")
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 && fields[0] == "explain" && fields[1] == "life" {
			m.mode = modeDescribe
			m.describeView.content = explainLifeContent()
			m.describeView.scroll = 0
			return m, nil
		}

		return m, m.executeKubectlCommand(input)
	case cmdResourceJump:
		return m.executeResourceJump(input)
	default:
		// cmdUnknown: show error instead of trying as kubectl.
		firstWord := strings.Fields(input)[0]
		m.setStatusMessage(fmt.Sprintf("Unknown command: %s (use :! for shell commands)", firstWord), true)
		return m, scheduleStatusClear()
	}
}

// extractShellCommand strips the "!" prefix and leading whitespace from
// a shell command input string.
func extractShellCommand(input string) string {
	s := strings.TrimPrefix(input, "!")
	return strings.TrimSpace(s)
}

// executeShellCommand runs an arbitrary shell command via sh -c.
// It sets the KUBECONFIG environment variable from the client config
// and logs the command before execution.
//
// In PTY terminal mode the command is launched inside lfk's embedded
// vt10x terminal so the output flows in-place beside the rest of the
// TUI, with line wrapping at the current pane width. In Exec mode the
// host terminal is handed over via tea.ExecProcess (legacy behavior).
func (m Model) executeShellCommand(cmd string) tea.Cmd {
	if cmd == "" {
		return nil
	}

	m.addLogEntry("DBG", fmt.Sprintf("$ sh -c %q", cmd))

	if ui.ConfigTerminalMode == ui.TerminalModePTY {
		c := exec.Command("sh", "-c", cmd)
		c.Env = m.kubectlEnv()
		logExecCmd("Running shell command", c)
		cols, rows := m.embeddedPTYSize()
		return startPTYExecCmd(c, fmtPTYTitle("$ "+cmd), cols, rows)
	}

	// Exec mode: clear the host screen, run, and wait for a keypress so
	// the output is readable before the TUI redraws.
	shellCmd := fmt.Sprintf(
		`printf '\033c' && %s; printf '\nPress any key to continue...'; read -r -n1 _`,
		cmd,
	)

	c := exec.Command("sh", "-c", shellCmd)
	c.Env = m.kubectlEnv()

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return actionResultMsg{err: err}
	})
}

// embeddedPTYSize returns the column/row dimensions to use for the
// embedded vt10x terminal when launching a command bar invocation.
// Mirrors the size policy used by execKubectlExec/Attach/Debug so all
// command-bar PTY launches share the same layout. The fallbacks (80x24)
// kick in only before the first WindowSizeMsg, which is rare in
// practice but keeps the math safe.
func (m Model) embeddedPTYSize() (cols, rows int) {
	cols = m.width
	rows = m.height - 6
	if cols < 20 {
		cols = 80
	}
	if rows < 5 {
		rows = 24
	}
	return cols, rows
}

// fmtPTYTitle returns a short title for the PTY tab bar, truncating
// long shell command strings so they don't blow out the title line.
// Operates on runes so a multi-byte codepoint (e.g. an em-dash or a
// non-ASCII pod name) never gets sliced mid-encoding.
func fmtPTYTitle(full string) string {
	const max = 60
	runes := []rune(full)
	if len(runes) <= max {
		return full
	}
	return string(runes[:max-1]) + "…"
}

// executeBuiltinCommand parses and executes a built-in command.
// It looks up the canonical command name via the builtinCommands map
// and dispatches accordingly.
func (m Model) executeBuiltinCommand(input string) (tea.Model, tea.Cmd) {
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return m, nil
	}

	canonical, ok := builtinCommands[tokens[0]]
	if !ok {
		m.setStatusMessage(fmt.Sprintf("Unknown command: %s", tokens[0]), true)
		return m, scheduleStatusClear()
	}

	arg := ""
	if len(tokens) > 1 {
		arg = strings.Join(tokens[1:], " ")
	}

	switch canonical {
	case "quit":
		// Mirror the cleanup the other quit paths (closeTabOrQuit,
		// handleQuitConfirmOverlayKey) perform before tea.Quit so that
		// kubectl log streams started from any tab don't outlive the
		// process. Without this, `:q` / `:q!` / `:quit` leaks the
		// kubectl subprocess and its reader goroutine — issue #48.
		return m.beginShutdown()

	case "namespace":
		return m.executeNamespaceCommand(tokens[1:])

	case "context":
		if m.unionMode {
			m.setStatusMessage(":ctx is disabled in union view", true)
			return m, scheduleStatusClear()
		}
		if arg == "" {
			m.setStatusMessage("Usage: :ctx <context>", true)
			return m, scheduleStatusClear()
		}
		oldCtx := m.nav.Context
		m.nav.Context = arg
		m.invalidateOrphanCacheForContext(oldCtx)
		m.recomputeReadOnly(arg)
		m.setStatusMessage(fmt.Sprintf("Context set to %s", arg), false)
		cmds := []tea.Cmd{m.loadResourceTypes(), scheduleStatusClear()}
		if cmd := m.ensureNamespaceCacheFresh(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case "set":
		return m.executeSetCommand(arg)

	case "sort":
		return m.executeSortCommand(arg)

	case "export":
		lower := strings.ToLower(arg)
		switch lower {
		case "", "yaml":
			// Share the bulk-or-cursor dispatcher with the `Y` keybinding
			// so multi-selection (cap, "Fetching N..." status, level gating)
			// behaves the same whether the user types `:export yaml` or
			// hits `Y`.
			return m.dispatchYAMLClipboardCopy()
		case "json":
			mdl, cmd := m.dispatchYAMLClipboardCopy()
			if cmd == nil {
				return mdl, nil
			}
			return mdl, wrapYAMLCmdAsJSON(cmd)
		}
		m.setStatusMessage(fmt.Sprintf("Unknown export format: %s", arg), true)
		return m, scheduleStatusClear()

	case "nyan":
		var cmd tea.Cmd
		m, cmd = m.toggleNyan()
		return m, tea.Batch(cmd, scheduleStatusClear())

	case "kubetris":
		g := newKubetrisGame()
		g.loadHighScore()
		m.kubetrisGame = g
		m.mode = modeKubetris
		return m, m.scheduleKubetrisTick()

	case "credits":
		m.mode = modeCredits
		m.creditsScroll = m.height
		return m, scheduleCreditsScroll()

	case "scheduler":
		m.overlay = overlayBackgroundTasks
		// Always open fresh in running mode with scroll at the top.
		// Tab inside the overlay switches to the completed-history view.
		m.tasksOverlayShowCompleted = false
		m.tasksOverlayScroll = 0
		return m, nil

	// Quick aliases for hotkey-only behaviors so they're discoverable via
	// command-palette autocomplete without forcing the user to memorize
	// single-letter chords.
	case "errors", "warnings":
		// Equivalent to the `!` hotkey on the Events list -- toggles the
		// "warnings only" filter. No-op (with status hint) when invoked
		// outside the Events view since there's nothing to filter.
		if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "Event" {
			m.warningEventsOnly = !m.warningEventsOnly
			m.rebuildEventsFromCache()
			if m.warningEventsOnly {
				m.setStatusMessage("Showing warnings only", false)
			} else {
				m.setStatusMessage("Showing all events", false)
			}
			return m, scheduleStatusClear()
		}
		m.setStatusMessage(":errors only applies to the Events view", true)
		return m, scheduleStatusClear()

	case "bookmarks":
		m.overlay = overlayBookmarks
		m.bookmarkSearchMode = bookmarkModeNormal
		m.bookmarkFilter.Clear()
		m.bookmarkLoadNamespace = true
		return m, nil

	case "orphans":
		return m.executeOrphansCommand(arg)

	case "reload", "refresh":
		// Force-fetch the current resource list -- equivalent to Shift+R.
		// Useful when watch mode is off or the user wants an immediate
		// refresh after a kubectl apply outside lfk.
		m.setStatusMessage("Reloading...", false)
		return m, tea.Batch(m.loadResources(false), scheduleStatusClear())

	case "session":
		return m.executeSessionCommand(tokens[1:])

	case "sessions":
		return m.openSessionsOverlay()

	default:
		m.setStatusMessage(fmt.Sprintf("Unknown command: %s", canonical), true)
		return m, scheduleStatusClear()
	}
}

func (m Model) executeNamespaceCommand(namespaces []string) (tea.Model, tea.Cmd) {
	if len(namespaces) == 0 {
		// No arguments: jump to Namespaces resource type.
		return m.executeResourceJump("namespaces")
	}
	if m.unionMode && len(namespaces) != 1 {
		m.setStatusMessage("Union mode supports exactly one namespace", true)
		return m, scheduleStatusClear()
	}
	m.allNamespaces = false
	m.selectedNamespaces = make(map[string]bool, len(namespaces))
	for _, ns := range namespaces {
		m.selectedNamespaces[ns] = true
	}
	oldNs := m.namespace
	m.namespace = namespaces[0]
	m.invalidateOrphanCacheForNamespace(m.nav.Context, oldNs)
	if len(namespaces) == 1 {
		m.setStatusMessage(fmt.Sprintf("Namespace set to %s", namespaces[0]), false)
	} else {
		m.setStatusMessage(fmt.Sprintf("Namespaces set to %s", strings.Join(namespaces, ", ")), false)
	}
	return m, tea.Batch(m.loadResources(false), scheduleStatusClear())
}

// executeSortCommand handles the :sort builtin command. At LevelClusters
// and LevelResourceTypes the sort engine early-returns, so accepting :sort
// silently would mutate sortColumnName and emit a misleading "Sort by ..."
// status while items stayed put. Surface that as an explicit error so the
// user understands why the typed command had no effect.
func (m Model) executeSortCommand(arg string) (tea.Model, tea.Cmd) {
	if arg == "" {
		m.setStatusMessage("Usage: :sort <column>", true)
		return m, scheduleStatusClear()
	}
	if !m.sortApplies() {
		m.setStatusMessage("Sort is not available at this level", true)
		return m, scheduleStatusClear()
	}
	m.sortColumnName = arg
	m.rememberSort()
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage(fmt.Sprintf("Sort by %s", arg), false)
	return m, scheduleStatusClear()
}

// executeSetCommand handles the :set builtin command.
// It toggles log viewer options: wrap/nowrap, linenumbers/nolinenumbers,
// timestamps/notimestamps, follow/nofollow, ansi/noansi.
func (m Model) executeSetCommand(option string) (tea.Model, tea.Cmd) {
	switch strings.ToLower(strings.TrimSpace(option)) {
	case "wrap":
		m.logView.wrap = true
		m.setStatusMessage("Line wrap ON", false)
	case "nowrap":
		m.logView.wrap = false
		m.setStatusMessage("Line wrap OFF", false)
	case "linenumbers":
		m.logView.lineNumbers = true
		m.setStatusMessage("Line numbers ON", false)
	case "nolinenumbers":
		m.logView.lineNumbers = false
		m.setStatusMessage("Line numbers OFF", false)
	case "timestamps":
		m.logView.timestamps = true
		m.setStatusMessage("Timestamps ON", false)
	case "notimestamps":
		m.logView.timestamps = false
		m.setStatusMessage("Timestamps OFF", false)
	case "follow":
		m.logView.follow = true
		m.setStatusMessage("Log follow ON", false)
	case "nofollow":
		m.logView.follow = false
		m.setStatusMessage("Log follow OFF", false)
	case "ansi":
		ui.ConfigLogRenderAnsi = true
		m.setStatusMessage("Log ANSI rendering ON", false)
	case "noansi":
		ui.ConfigLogRenderAnsi = false
		m.setStatusMessage("Log ANSI rendering OFF", false)
	default:
		m.setStatusMessage(fmt.Sprintf("Unknown set option: %s", option), true)
	}
	return m, scheduleStatusClear()
}

// executeResourceJump resolves a resource type name (or abbreviation) and
// moves the cursor to the matching item in the left column.
func (m Model) executeResourceJump(input string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return m, nil
	}
	name := fields[0]
	lower := strings.ToLower(name)

	// Optional namespace arguments (one or more).
	if len(fields) >= 2 {
		namespaces := fields[1:]
		if m.unionMode && len(namespaces) != 1 {
			m.setStatusMessage("Union mode supports exactly one namespace", true)
			return m, scheduleStatusClear()
		}
		m.allNamespaces = false
		m.selectedNamespaces = make(map[string]bool, len(namespaces))
		for _, ns := range namespaces {
			m.selectedNamespaces[ns] = true
		}
		oldNs := m.namespace
		m.namespace = namespaces[0]
		m.invalidateOrphanCacheForNamespace(m.nav.Context, oldNs)
	}

	// Resolve abbreviation to full resource name if possible.
	resolved := lower
	if ui.SearchAbbreviations != nil {
		if full, ok := ui.SearchAbbreviations[lower]; ok {
			resolved = strings.ToLower(full)
		}
	}

	// Navigate back to resource types level.
	for m.nav.Level > model.LevelResourceTypes {
		ret, _ := m.navigateParent()
		m = ret.(Model)
	}

	// If we're at cluster level, we can't jump to a resource type.
	if m.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage(fmt.Sprintf("Resource type not found: %s", name), true)
		return m, scheduleStatusClear()
	}

	// Find matching resource type in the middle items (which are resource types at this level).
	for i, item := range m.middleItems {
		itemResource := strings.ToLower(resourceFromExtra(item.Extra))
		itemName := strings.ToLower(item.Name)
		itemKind := strings.ToLower(item.Kind)

		itemSingular := toSingular(itemResource)
		nameSingular := toSingular(itemName)
		if itemResource == resolved || itemSingular == resolved ||
			itemName == resolved || nameSingular == resolved ||
			itemKind == resolved {
			m.setCursor(i)
			// Navigate into the resource type (loads resources).
			return m.navigateChild()
		}
	}

	m.setStatusMessage(fmt.Sprintf("Resource type not found: %s", name), true)
	return m, scheduleStatusClear()
}

// resourceFromExtra extracts the resource name (last segment) from an
// Extra field that typically looks like "group/version/resource" or "v1/resource".
func resourceFromExtra(extra string) string {
	if extra == "" {
		return ""
	}
	parts := strings.Split(extra, "/")
	return parts[len(parts)-1]
}

// findItemNamespace looks up the namespace of the first positional resource name
// found in middleItems. Returns empty string if not found.
func (m *Model) findItemNamespace(args []string) string {
	// Collect positional args after subcommand and resource type (position 2+).
	var names []string
	pos := 0
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(a, "-") {
			if a == "-n" || a == "-o" || a == "-l" || a == "-f" || a == "-c" ||
				a == "--namespace" || a == "--output" || a == "--selector" || a == "--filename" || a == "--container" {
				skipNext = true
			}
			continue
		}
		if pos >= 2 {
			names = append(names, a)
		}
		pos++
	}
	// Find the first matching item's namespace.
	for _, name := range names {
		for _, item := range m.middleItems {
			if item.Name == name && item.Namespace != "" {
				return item.Namespace
			}
		}
	}
	return ""
}

// positionalArgCount counts non-flag arguments in a kubectl command.
// e.g., ["get", "pod", "nginx"] = 3, ["get", "pod", "-n", "default"] = 2.
func positionalArgCount(args []string) int {
	count := 0
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(a, "-") {
			// Flags that take a value: skip next arg.
			if a == "-n" || a == "-o" || a == "-l" || a == "-f" || a == "-c" ||
				a == "--namespace" || a == "--output" || a == "--selector" || a == "--filename" || a == "--container" {
				skipNext = true
			}
			continue
		}
		count++
	}
	return count
}

// executeKubectlCommand runs a kubectl command. It strips the optional
// "kubectl " or "k " prefix, injects default --context and -n flags,
// sets KUBECONFIG, and runs the command.
//
// In PTY terminal mode the command runs inside lfk's embedded vt10x
// terminal so output renders inline (with line wrapping at the pane
// width) alongside the rest of the TUI. In Exec mode the host terminal
// is handed over via tea.ExecProcess and lfk is suspended until the
// command exits.
func (m Model) executeKubectlCommand(input string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	// Strip leading "kubectl " or "k " prefix if present.
	trimmed := strings.TrimSpace(input)
	trimmed = strings.TrimPrefix(trimmed, "kubectl ")
	trimmed = strings.TrimPrefix(trimmed, "k ")

	args := strings.Fields(trimmed)
	if len(args) == 0 {
		return nil
	}

	// Decide this BEFORE injectKubectlDefaults adds `--context` / `-n`
	// so the positional-arg detection sees the user's original shape.
	affectsNamespaces := commandAffectsNamespaces(args)

	args = m.injectKubectlDefaults(args)

	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl %s", strings.Join(args, " ")))

	if ui.ConfigTerminalMode == ui.TerminalModePTY {
		c := exec.Command(kubectlPath, args...)
		c.Env = m.kubectlEnv()
		logExecCmd("Running kubectl command", c)
		// PTY-mode commands don't route through tea.ExecProcess, so the
		// namespace-cache invalidation that normally arrives via
		// actionResultMsg won't fire here. That's acceptable because the
		// user sees the result inline and can refresh manually with R; a
		// future improvement could propagate invalidation through
		// execPTYExitMsg. The affectsNamespaces value is intentionally
		// dropped to keep the PTY path free of bespoke wiring.
		_ = affectsNamespaces
		cols, rows := m.embeddedPTYSize()
		return startPTYExecCmd(c, fmtPTYTitle("kubectl "+strings.Join(args, " ")), cols, rows)
	}

	// Exec mode: clear the host screen, run, and wait for a keypress
	// before returning to the TUI so the user can read the output.
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	shellCmd := fmt.Sprintf(
		`printf '\033c' && %s %s; printf '\nPress any key to continue...'; read -r -n1 _`,
		shellQuote(kubectlPath), strings.Join(quoted, " "),
	)

	c := exec.Command("sh", "-c", shellCmd)
	c.Env = m.kubectlEnv()

	logExecCmd("Running kubectl command", c)

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return actionResultMsg{err: err, invalidateNamespaceCache: affectsNamespaces}
	})
}

// wrapYAMLCmdAsJSON converts the YAML payload of an inner yamlClipboardMsg
// into JSON. A single-document payload becomes a JSON object; a multi-document
// payload (separated by `\n---\n` per copyYAMLToClipboard's joiner) becomes a
// JSON array. The bulk-fetch wiring, status messages, and error envelope are
// reused unchanged.
func wrapYAMLCmdAsJSON(cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		msg := cmd()
		yc, ok := msg.(yamlClipboardMsg)
		if !ok || yc.err != nil {
			return msg
		}
		if yc.count <= 1 {
			jsonBytes, err := yaml.YAMLToJSON([]byte(yc.content))
			if err != nil {
				yc.err = fmt.Errorf("converting YAML to JSON: %w", err)
				return yc
			}
			yc.content = string(jsonBytes) + "\n"
			yc.format = "json"
			return yc
		}
		docs := strings.Split(strings.TrimRight(yc.content, "\n"), "\n---\n")
		objects := make([]json.RawMessage, 0, len(docs))
		for _, doc := range docs {
			jsonBytes, err := yaml.YAMLToJSON([]byte(doc))
			if err != nil {
				yc.err = fmt.Errorf("converting YAML to JSON: %w", err)
				return yc
			}
			objects = append(objects, jsonBytes)
		}
		arrayBytes, err := json.Marshal(objects)
		if err != nil {
			yc.err = fmt.Errorf("marshaling JSON array: %w", err)
			return yc
		}
		yc.content = string(arrayBytes) + "\n"
		yc.format = "json"
		return yc
	}
}
