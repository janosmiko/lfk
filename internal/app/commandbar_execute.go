package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
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
		// Use only the kubeconfig file that defines the current context.
		kubeconfigPath := m.client.KubeconfigPathForContext(ctx)
		if kubeconfigPath != "" {
			env = append(env, "KUBECONFIG="+kubeconfigPath)
		}
		if ctx != "" {
			env = append(env, "KUBECTL_CONTEXT="+ctx)
		}
	}
	return env
}

// executeCommandBarInput is the main entry point for command bar execution.
// It classifies the input and dispatches to the appropriate handler.
func (m Model) executeCommandBarInput(input string) (tea.Model, tea.Cmd) {
	input = strings.TrimSpace(input)
	if input == "" {
		return m, nil
	}

	crdNames := extractCRDNames(&m)
	switch classifyInputWithCRDs(input, crdNames) {
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
			m.describeContent = explainLifeContent()
			m.describeScroll = 0
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
func (m Model) executeShellCommand(cmd string) tea.Cmd {
	if cmd == "" {
		return nil
	}

	m.addLogEntry("DBG", fmt.Sprintf("$ sh -c %q", cmd))

	// Wrap to clear screen, run, and wait for keypress.
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
		if m.portForwardMgr != nil {
			m.portForwardMgr.StopAll()
		}
		m.cancelAllTabLogStreams()
		m.saveCurrentSession()
		return m, tea.Quit

	case "namespace":
		namespaces := tokens[1:]
		if len(namespaces) == 0 {
			// No arguments: jump to Namespaces resource type.
			return m.executeResourceJump("namespaces")
		}
		m.allNamespaces = false
		m.selectedNamespaces = make(map[string]bool, len(namespaces))
		for _, ns := range namespaces {
			m.selectedNamespaces[ns] = true
		}
		m.namespace = namespaces[0]
		if len(namespaces) == 1 {
			m.setStatusMessage(fmt.Sprintf("Namespace set to %s", namespaces[0]), false)
		} else {
			m.setStatusMessage(fmt.Sprintf("Namespaces set to %s", strings.Join(namespaces, ", ")), false)
		}
		return m, tea.Batch(m.loadResources(false), scheduleStatusClear())

	case "context":
		if arg == "" {
			m.setStatusMessage("Usage: :ctx <context>", true)
			return m, scheduleStatusClear()
		}
		m.nav.Context = arg
		m.setStatusMessage(fmt.Sprintf("Context set to %s", arg), false)
		cmds := []tea.Cmd{m.loadResourceTypes(), scheduleStatusClear()}
		if cmd := m.ensureNamespaceCacheFresh(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case "set":
		return m.executeSetCommand(arg)

	case "sort":
		if arg == "" {
			m.setStatusMessage("Usage: :sort <column>", true)
			return m, scheduleStatusClear()
		}
		m.sortColumnName = arg
		m.sortMiddleItems()
		m.clampCursor()
		m.setStatusMessage(fmt.Sprintf("Sort by %s", arg), false)
		return m, scheduleStatusClear()

	case "export":
		lower := strings.ToLower(arg)
		if lower == "yaml" || lower == "json" || lower == "" {
			return m, m.copyYAMLToClipboard()
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

	case "tasks":
		m.overlay = overlayBackgroundTasks
		// Always open fresh in running mode with scroll at the top.
		// Tab inside the overlay switches to the completed-history view.
		m.tasksOverlayShowCompleted = false
		m.tasksOverlayScroll = 0
		return m, nil

	default:
		m.setStatusMessage(fmt.Sprintf("Unknown command: %s", canonical), true)
		return m, scheduleStatusClear()
	}
}

// executeSetCommand handles the :set builtin command.
// It toggles log viewer options: wrap/nowrap, linenumbers/nolinenumbers,
// timestamps/notimestamps, follow/nofollow, ansi/noansi.
func (m Model) executeSetCommand(option string) (tea.Model, tea.Cmd) {
	switch strings.ToLower(strings.TrimSpace(option)) {
	case "wrap":
		m.logWrap = true
		m.setStatusMessage("Line wrap ON", false)
	case "nowrap":
		m.logWrap = false
		m.setStatusMessage("Line wrap OFF", false)
	case "linenumbers":
		m.logLineNumbers = true
		m.setStatusMessage("Line numbers ON", false)
	case "nolinenumbers":
		m.logLineNumbers = false
		m.setStatusMessage("Line numbers OFF", false)
	case "timestamps":
		m.logTimestamps = true
		m.setStatusMessage("Timestamps ON", false)
	case "notimestamps":
		m.logTimestamps = false
		m.setStatusMessage("Timestamps OFF", false)
	case "follow":
		m.logFollow = true
		m.setStatusMessage("Log follow ON", false)
	case "nofollow":
		m.logFollow = false
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
		m.allNamespaces = false
		m.selectedNamespaces = make(map[string]bool, len(namespaces))
		for _, ns := range namespaces {
			m.selectedNamespaces[ns] = true
		}
		m.namespace = namespaces[0]
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
// sets KUBECONFIG, and runs interactively via tea.ExecProcess.
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

	// Wrap command to clear screen, run, and wait for keypress before returning to TUI.
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

	logger.Info("Running kubectl command", "cmd", shellCmd)

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return actionResultMsg{err: err, invalidateNamespaceCache: affectsNamespaces}
	})
}

// commandAffectsNamespaces reports whether a kubectl command (args
// without the "kubectl"/"k" prefix) looks like it mutates the cluster's
// Namespace list. Matches `create|delete|replace (ns|namespace|namespaces) ...`.
// Read-only verbs (`get`, `describe`) are excluded so they don't force
// an unnecessary cache refresh, and `apply -f <file>` is not inspected
// (template applies invalidate unconditionally instead). False
// positives only cost one extra GetNamespaces call, so the heuristic
// favours breadth.
func commandAffectsNamespaces(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[0] {
	case "create", "delete", "replace":
	default:
		return false
	}
	for _, a := range args[1:] {
		switch a {
		case "ns", "namespace", "namespaces":
			return true
		}
	}
	return false
}

// injectKubectlDefaults scans the args for --context, -n/--namespace, and
// -A/--all-namespaces flags. If they are not present, it injects the current
// context and namespace from the model.
func (m *Model) injectKubectlDefaults(args []string) []string {
	hasContext := false
	hasNamespace := false
	hasAllNamespaces := false

	for _, a := range args {
		switch {
		case a == "--context" || strings.HasPrefix(a, "--context="):
			hasContext = true
		case a == "-n" || a == "--namespace" || strings.HasPrefix(a, "--namespace="):
			hasNamespace = true
		case a == "-A" || a == "--all-namespaces":
			hasAllNamespaces = true
		}
	}

	result := make([]string, len(args))
	copy(result, args)

	if !hasContext && m.nav.Context != "" {
		result = append(result, "--context", m.nav.Context)
	}

	if !hasNamespace && !hasAllNamespaces {
		hasResourceNames := positionalArgCount(args) > 2
		ns := m.effectiveNamespace()
		if ns != "" {
			result = append(result, "-n", ns)
		} else if m.allNamespaces {
			if hasResourceNames {
				// kubectl can't use -A with resource names. Look up the namespace
				// of the first named resource from the currently loaded items.
				if foundNS := m.findItemNamespace(args); foundNS != "" {
					result = append(result, "-n", foundNS)
				}
				// If not found, run without -n (kubectl uses kubeconfig default).
			} else {
				result = append(result, "-A")
			}
		}
	}

	return result
}
