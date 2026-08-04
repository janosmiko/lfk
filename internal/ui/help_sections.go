package ui

// helpSections returns all help sections with their keybindings.
//
// Every entry is a real binding: the help screen renders one hotkey per
// line, so a row with no key would render as a wall of prose. Longer
// explanations belong in docs/keybindings.md, which stays exhaustive.
//
// Lives in its own file so the static catalog can grow without
// pushing help.go past the repo's 800-line file cap.
func helpSections() []helpSection {
	kb := ActiveKeybindings
	sections := explorerHelpSections(kb)
	return append(sections, viewerHelpSections(kb)...)
}

// HelpRow is one rendered help row reduced to the two facts a cross-surface
// consistency check needs: the section that drew it and the raw key string
// the row advertises.
type HelpRow struct {
	Section string
	Key     string
}

// ExplorerHelpRows returns every context-free (explorer) help row, in catalog
// order. Exported for the app layer's group-alignment guard: the which-key
// registry lives in package app and cannot reach into this package's
// unexported catalog, so the two surfaces would otherwise have no way to be
// compared in a test. Per-view sections are omitted — they carry a context
// and have no which-key counterpart by design.
func ExplorerHelpRows() []HelpRow {
	var rows []HelpRow
	for _, s := range helpSections() {
		if s.context != "" {
			continue
		}
		for _, b := range s.bindings {
			rows = append(rows, HelpRow{Section: s.title, Key: b.key})
		}
	}
	return rows
}

// explorerHelpSections lists the sections shown in the explorer (main)
// view — the ones with an empty context — plus the context-free tail
// (tabs, mouse, help, general).
func explorerHelpSections(kb Keybindings) []helpSection {
	return []helpSection{
		{
			title: "Navigation",
			bindings: []helpEntry{
				{kb.Left + "/Left", "Go to parent"},
				{kb.Right + "/Right", "Go to child"},
				{kb.Down + "/Down", "Move down"},
				{kb.Up + "/Up", "Move up"},
				{kb.JumpTop + kb.JumpTop + "/Home", "Jump to top"},
				{kb.JumpBottom + "/End", "Jump to bottom"},
				// "{key}" is vim's own notation for a required placeholder
				// (":help g{char}"), and this is a vim-modeled TUI. A bare
				// "g" would read as ambiguous next to the "gg" row above it.
				{kb.JumpTop + "{key}", "Goto resource type"},
				{kb.PageDown + "/" + kb.PageUp, "Scroll half page down/up"},
				{kb.PageForward + "/" + kb.PageBack, "Scroll full page down/up"},
				{kb.Enter, "Open YAML view / navigate into"},
				{kb.LevelCluster + "/" + kb.LevelTypes + "/" + kb.LevelResources, "Jump to cluster/type/resource level"},
				{kb.JumpOwner, "Jump to owner/controller"},
				{kb.JumpBack, "Jump back through teleport history"},
				{kb.PreviewDown + "/" + kb.PreviewUp, "Scroll preview pane down/up"},
				{kb.ExpandCollapse, "Expand/collapse all groups"},
			},
		},
		{
			// Namespace scope lives here, not under Actions: selecting a
			// namespace narrows what the list shows, the same way a filter
			// does. Matches the which-key panel, which groups all three
			// under Filter (whichkey_registry.go).
			title: "Search & Filter",
			bindings: []helpEntry{
				{kb.Filter, "Filter items (~fuzzy, regex auto, \\literal)"},
				{kb.Search, "Search and jump to match"},
				{kb.NextMatch, "Next search match"},
				{kb.PrevMatch, "Previous search match"},
				{kb.FilterPresets, "Quick filter presets"},
				{"Up/Down", "Recall previous query (in filter or search)"},
				{kb.NamespaceSelector, "Select namespace"},
				{kb.AllNamespaces, "Toggle all-namespaces mode"},
				{kb.PreviousNamespace, "Jump to previous namespace"},
			},
		},
		{
			title: "Views & Tools",
			bindings: []helpEntry{
				{kb.ExplorerHelpKey(), "Toggle help screen (" + kb.Help + " outside the explorer)"},
				{kb.WhichKeyLeader, "Which-key panel: hotkeys actionable now"},
				{kb.TogglePreview, "Toggle details / YAML preview"},
				{kb.TogglePreviewLogs, "Toggle live-log preview pane"},
				{kb.Fullscreen, "Cycle layout: sidebar, fullscreen, restore"},
				{kb.ResourceMap, "Toggle resource relationship map"},
				{kb.ColumnToggle, "Show/hide and reorder columns"},
				{kb.PinGroup, "Pin/unpin resource type"},
				{kb.ToggleRare, "Toggle rare + hidden resource types"},
				{kb.APIExplorer, "API Explorer (resource structure)"},
				{kb.ObjectExplorer, "Object Explorer (live object tree)"},
				{kb.RBACBrowser, "RBAC permissions browser (can-i)"},
				{kb.OrphanOverlay, "Cluster-wide Orphan overview"},
				{kb.SessionManager, "Session manager"},
				{kb.FinalizerSearch, "Finalizer search and remove"},
				{kb.ErrorLog, "Error log"},
				{kb.Monitoring, "Cycle Cluster / Monitoring dashboard"},
				{kb.QuotaDashboard, "Namespace resource quota dashboard"},
				{kb.TasksOverlay, "Scheduler / task queue"},
			},
		},
		{
			title: "Sorting",
			bindings: []helpEntry{
				{kb.SortNext, "Sort by next column"},
				{kb.SortPrev, "Sort by previous column"},
				{kb.SortFlip, "Toggle sort direction"},
				{kb.SortReset, "Reset sort to default (Name ascending)"},
			},
		},
		{
			title: "Multi-Selection",
			bindings: []helpEntry{
				{kb.ToggleSelect, "Toggle selection on current item"},
				{kb.SelectRange, "Select range from anchor to cursor"},
				{kb.SelectAll, "Select/deselect all visible items"},
				{"esc", "Clear selection"},
				{kb.ActionMenu, "Bulk action menu"},
				{kb.Diff, "Diff YAML of two selected resources"},
			},
		},
		{
			title: "Actions",
			bindings: []helpEntry{
				{kb.ActionMenu, "Action menu: logs, exec, debug, edit, scale, ..."},
				{kb.Logs, "Open fullscreen log viewer"},
				{kb.Describe, "Describe selected resource"},
				{kb.Edit, "Edit in $KUBE_EDITOR or $EDITOR"},
				{kb.SecretEditor, "Secret/ConfigMap editor"},
				{kb.LabelEditor, "Edit labels/annotations"},
				{kb.Refresh, "Refresh current view"},
				{kb.Delete, "Delete (Tab cycles the cascade policy)"},
				{kb.ForceDelete, "Force delete (Pod/Job only)"},
				{kb.SaveResource, "Save resource to file"},
				{kb.CopyName, "Copy resource name"},
				{kb.CopyYAML, "Copy-as picker (YAML / JSON / Table)"},
				{kb.CopyField, "Copy a single field (Tab: all manifest fields)"},
				{kb.PasteApply, "Apply resource from clipboard"},
				{kb.OpenBrowser, "Open in browser (Ingress / port-forward)"},
			},
		},
		{
			title: "Modes & Settings",
			bindings: []helpEntry{
				{kb.WatchMode, "Toggle watch mode (auto-refresh)"},
				{kb.ReadOnlyToggle, "Toggle read-only mode"},
				{kb.ClusterColorPicker, "Cluster color picker (cluster picker only)"},
				{kb.ThemeSelector, "Switch color scheme"},
				{kb.TerminalToggle, "Cycle terminal mode (pty / exec / mux)"},
				{kb.SecretToggle, "Toggle secret value visibility"},
				{kb.SecurityBadgeToggle, "Show/hide the SEC severity badge"},
				{kb.SecurityIgnoreToggle, "Show/hide ignored security findings"},
			},
		},
		{
			title: "Command Bar (" + kb.CommandBar + ")",
			bindings: []helpEntry{
				{kb.CommandBar, "Open command bar"},
				{":pod, :dep, :svc", "Jump to resource type (add a namespace to filter)"},
				{":ns, :ctx, :set", "Namespace, context, option"},
				{":sort, :export", "Sort by column, export in a format"},
				{":session", "Save or delete a named session"},
				{":sessions", "Open the session picker"},
				{":k, :kubectl", "Run a kubectl command"},
				{":!", "Run a shell command"},
				{"tab", "Cycle suggestions forward (auto-fill on 1 match)"},
				{"shift+tab", "Cycle suggestions backward"},
				{"ctrl+n/ctrl+p", "Cycle suggestions forward/backward"},
				{"ctrl+d/ctrl+u", "Scroll suggestions (half page)"},
				{"ctrl+f/ctrl+b", "Scroll suggestions (full page)"},
				{"ctrl+space", "Open/refresh suggestions"},
				{"space/Right", "Accept ghost text preview"},
				{"enter", "Accept suggestion or execute command"},
				{"esc", "Close suggestions, or close command bar"},
				{"Up/Down", "Browse command history"},
				{"ctrl+w", "Delete word backwards"},
				{"ctrl+a/ctrl+e", "Home / End"},
			},
		},
		{
			title: "Bookmarks",
			bindings: []helpEntry{
				{kb.SetMark + "<a-z/0-9>", "Set context-aware mark (switches cluster on jump)"},
				{kb.SetMark + "<A-Z>", "Set context-free mark (stays in cluster)"},
				{kb.OpenMarks, "Open bookmarks list"},
				{"a-z/A-Z/0-9", "Jump to named mark"},
				{"j/k", "Navigate bookmarks"},
				{"/", "Filter bookmarks by name"},
				{"enter", "Jump to selected bookmark"},
				{"ctrl+x", "Delete selected bookmark"},
				{"alt+x", "Delete all bookmarks"},
			},
		},
		{
			title: "Tabs",
			bindings: []helpEntry{
				{kb.NewTab, "New tab (clone current)"},
				{kb.PrevTab, "Previous tab"},
				{kb.NextTab, "Next tab"},
				{kb.MoveTabLeft, "Move tab left"},
				{kb.MoveTabRight, "Move tab right"},
				{"ctrl+c", "Close tab (quit if last)"},
			},
		},
		{
			title: "Mouse",
			bindings: []helpEntry{
				{"Click left pane", "Drill out one level"},
				{"Click middle pane", "Select row; click again to drill in"},
				{"Click right pane", "Drill into the selected item"},
				{"Right-click", "Open action menu for the clicked item"},
				{"Click header", "Sort by that column"},
				{"Click ns badge", "Open the namespace selector"},
				{"Click outside", "Dismiss the overlay (same as Esc)"},
				{"Wheel", "Scroll the list or pane under the pointer"},
				{"shift+Drag", "Select text (terminal native)"},
				{kb.MouseToggle, "Toggle mouse capture"},
			},
		},
		{
			title: "Help View",
			bindings: []helpEntry{
				{"/", "Search — highlights matches inline"},
				{"ctrl+n/ctrl+p", "Next / previous match while typing"},
				{"enter", "Apply search (keep highlights, arm n/N)"},
				{"n/N", "Jump to next / previous match"},
				{"f", "Filter — narrows the list to matching lines"},
				{"esc", "Clear search, then filter, then close"},
			},
		},
		{
			title: "General",
			bindings: []helpEntry{
				{"q", "Quit (with confirmation)"},
				{"esc", "Go back / quit"},
			},
		},
	}
}

// viewerHelpSections lists the per-view sections. Each carries a context
// so the help overlay shows only the sections that belong to the view the
// user opened it from.
func viewerHelpSections(kb Keybindings) []helpSection {
	return append([]helpSection{
		{
			title: "Error Log", context: "Error Log",
			bindings: append(scrollHelpEntries(), []helpEntry{
				{"v/V", "Character / line visual selection"},
				{"y", "Copy selection (visual) or all entries"},
				{kb.Fullscreen, "Toggle fullscreen / overlay mode"},
				{"d", "Toggle debug log visibility"},
				{"esc", "Cancel visual selection, or close"},
				{"q", "Close overlay"},
			}...),
		},
		{
			title: "YAML View", context: "YAML View",
			bindings: append(textViewHelpEntries(kb), []helpEntry{
				{kb.ToggleFold, "Toggle fold on section under cursor"},
				{kb.ToggleFoldAll, "Toggle all folds"},
				{kb.ToggleWrap, "Toggle line wrapping"},
				{kb.Refresh, "Re-fetch and refresh (keeps position)"},
				{"ctrl+e", "Edit resource in editor"},
				{"O", "Open Object Explorer at the attribute under cursor"},
				{"I", "Open API Explorer at the schema of the attribute"},
				{"q/esc", "Back to explorer"},
			}...),
		},
		{
			title: "Describe View", context: "Describe View",
			bindings: append(textViewHelpEntries(kb), []helpEntry{
				{kb.ToggleWrap, "Toggle line wrapping"},
				{"q/esc", "Back to explorer"},
			}...),
		},
		{
			title: "Diff View", context: "Diff View",
			bindings: append(textViewHelpEntries(kb), []helpEntry{
				{"tab", "Switch cursor side (side-by-side mode)"},
				{kb.ToggleFold, "Toggle fold unchanged section at cursor"},
				{kb.ToggleFoldAll, "Toggle all folds"},
				{kb.ToggleLineNumbers, "Toggle line numbers"},
				{kb.ToggleWrap, "Toggle line wrapping"},
				{kb.ToggleUnified, "Toggle unified / side-by-side view"},
				{"q/esc", "Back to explorer"},
			}...),
		},
		{
			title: "API Explorer", context: "API Explorer",
			bindings: append([]helpEntry{
				{"j/k", "Navigate fields"},
				{"l/enter", "Drill into field (Object/array types)"},
				{"h/Backspace", "Go back one level"},
				{"/", "Search fields"},
				{"n/N", "Next / previous match"},
				{"r", "Recursive field browser"},
				{kb.TreeView, "Toggle tree view (whole field subtree)"},
				{"space/" + kb.ToggleFold, "Fold/unfold the subtree under the cursor"},
			}, append(scrollHelpEntries(), []helpEntry{
				{"q", "Close API explorer"},
				{"esc", "Go back one level / close at root"},
			}...)...),
		},
		{
			title: "Object Explorer", context: "Object Explorer",
			bindings: append([]helpEntry{
				{"j/k", "Navigate fields"},
				{"l/enter/Right", "Drill into object/array field"},
				{"h/Backspace/Left", "Go back one level"},
				{"J/K", "Scroll the YAML preview pane"},
				{"/", "Filter the current level by key"},
				{"r", "Recursive find across the whole object"},
				{kb.TreeView, "Toggle tree view (whole subtree)"},
				{"space/" + kb.ToggleFold, "Fold/unfold the subtree under the cursor"},
				{kb.Refresh, "Refresh the browsed object now"},
				{kb.WatchMode, "Toggle live refresh on/off"},
				{"y", "Yank the selected node's path"},
				{"Y", "Yank the selected node's full YAML"},
				{"P", "Open the whole resource in the YAML viewer"},
				{"I", "Open API Explorer at the schema of the item"},
			}, append(scrollHelpEntries(), helpEntry{"q/esc", "Back one level / close at root"})...),
		},
		{
			title: "Can-I Browser", context: "Can-I Browser",
			bindings: append([]helpEntry{
				{"j/k", "Navigate groups"},
				{"J/K", "Scroll resource list down/up"},
				{"/", "Search/filter groups by name"},
				{"a", "Toggle all / allowed-only permissions"},
				{"s", "Switch subject (User/Group/SA)"},
			}, append(scrollHelpEntries(), helpEntry{"q/esc", "Clear search / close"})...),
		},
		{
			title: "Can-I Subject Selector", context: "Can-I Browser",
			bindings: append([]helpEntry{
				{"j/k", "Navigate subjects"},
				{"/", "Filter subjects by name"},
				{"enter", "Select subject"},
			}, append(scrollHelpEntries(), helpEntry{"esc", "Clear filter / close"})...),
		},
		{
			title: "Network Policy Visualizer", context: "Network Policy / Pod / Service",
			bindings: append(scrollHelpEntries(), []helpEntry{
				{"/", "Search (highlights matches)"},
				{"n/N", "Next / previous match"},
				{"q/esc", "Close visualizer (Esc clears search first)"},
			}...),
		},
		{
			title: "Log Viewer", context: "Log Viewer",
			bindings: append(textViewHelpEntries(kb), []helpEntry{
				{kb.ToggleFollow, "Toggle follow mode (auto-scroll)"},
				{kb.ToggleWrap, "Toggle line wrapping"},
				{kb.ToggleLineNumbers, "Toggle line numbers"},
				{kb.ToggleTimestamps, "Toggle timestamps"},
				{kb.TogglePrefixes, "Toggle pod/container prefixes"},
				{kb.TogglePreview, "Toggle structured preview side panel"},
				{"J/K", "Scroll preview side panel down/up"},
				{"c", "Toggle previous container logs"},
				{kb.Filter, "Filter log lines live"},
				{kb.SeverityDown + "/" + kb.SeverityUp, "Lower/raise min severity (off/INFO/WARN/ERROR)"},
				{kb.LogTop, "Open Log Top aggregation"},
				{"S", "Save loaded logs to file"},
				{"ctrl+s", "Save all logs to file"},
				{"\\", "Switch pod / filter containers"},
				{"q/esc", "Close log viewer"},
			}...),
		},
		{
			title: "Log Top", context: "Log Top",
			bindings: []helpEntry{
				{"j/k", "Navigate rows"},
				{"g/G", "Jump to top / bottom"},
				{".", "Group-by field picker (multi-select)"},
				{"p", "Log format profile picker"},
				{",", "Show/hide and reorder columns"},
				{">/<", "Cycle sort column / flip direction"},
				{"=/-", "Reset sort"},
				{"enter", "Drill into selected group"},
				{"esc/q", "Pop drill level or return to log viewer"},
			},
		},
		{
			title: "Event Timeline", context: "Event Timeline",
			bindings: append(textViewHelpEntries(kb), []helpEntry{
				{kb.Fullscreen, "Toggle fullscreen event viewer"},
				{kb.ToggleWrap, "Toggle line wrapping"},
				{kb.ExpandCollapse, "Toggle event grouping"},
				{kb.SaveResource, "Toggle warnings-only"},
				{"q/esc", "Close overlay (or exit fullscreen)"},
			}...),
		},
		{
			title: "Sync Wave Timeline", context: "Sync Wave Timeline",
			bindings: append([]helpEntry{
				{"W", "Open Sync Wave Timeline (Application action menu)"},
				{"R", "Refresh"},
				{"tab/shift+tab", "Switch focus between sidebar and body"},
				{"enter/space", "Collapse / expand phase or wave under focus"},
			}, append(scrollHelpEntries(), helpEntry{"q/esc", "Close overlay"})...),
		},
		{
			title: "Exec Mode (embedded terminal)", context: "Exec Mode",
			bindings: []helpEntry{
				{"ctrl+]", "Prefix key (like tmux Ctrl+b)"},
				{"ctrl+] ctrl+]", "Exit terminal and return to explorer"},
				{"ctrl+] " + kb.NextTab, "Next tab (PTY keeps running)"},
				{"ctrl+] " + kb.PrevTab, "Previous tab (PTY keeps running)"},
				{"ctrl+] " + kb.NewTab, "New tab (clone current context)"},
				{"ctrl+] ctrl+u/ctrl+d", "Scroll back / forward by half a viewport"},
				{"ctrl+] ctrl+b/ctrl+f", "Scroll back / forward by a full viewport"},
				{"ctrl+] g/G", "Jump to oldest captured line / back to live"},
				{"Mouse Scroll", "Scroll the PTY scrollback"},
				{"shift+Drag", "Select text (host terminal)"},
			},
		},
	}, trafficCaptureHelpSections()...)
}

// trafficCaptureHelpSections splits the capture overlay into its three
// phases. Each phase gets its own section header rather than a prose row,
// so every rendered line stays a real binding.
func trafficCaptureHelpSections() []helpSection {
	const ctx = "Traffic Capture"
	return []helpSection{
		{
			title: "Traffic Capture: config", context: ctx,
			bindings: []helpEntry{
				{"tab/shift+tab", "Cycle focus between fields"},
				{"j/k", "Next / previous field"},
				{"h/l", "Cycle value of focused field"},
				{"(text)", "Type BPF filter (when focused)"},
				{"enter", "Start capture (or kubeshark hand-off)"},
				{"esc", "Close overlay"},
			},
		},
		{
			title: "Traffic Capture: live", context: ctx,
			bindings: []helpEntry{
				{"s", "Stop capture (overlay stays)"},
				{"t", "Toggle live table vs status-only view"},
				{"Y", "Copy pcap path (marks capture saved)"},
				{"/", "Search within live table"},
				{"j/k", "Scroll older / newer"},
				{"ctrl+d/ctrl+u", "Half-page scroll older / newer"},
				{"ctrl+f/ctrl+b", "Full-page scroll older / newer"},
				{"g/G", "Jump to oldest / return to live"},
				{"esc/q", "Stop capture; second Esc dismisses"},
			},
		},
		{
			title: "Traffic Capture: stopped", context: ctx,
			bindings: []helpEntry{
				{"enter", "Restart with the same params"},
				{"e", "Edit filter (re-opens config phase)"},
				{"esc", "Dismiss; deletes the pcap unless Y was pressed"},
			},
		},
	}
}

// scrollHelpEntries is the scroll/jump block every scrollable view shares.
// One definition keeps the wording identical across sections and stops the
// same four rows being re-typed a dozen times.
func scrollHelpEntries() []helpEntry {
	return []helpEntry{
		{"j/k", "Scroll down/up"},
		{"gg/G", "Top / bottom"},
		{"ctrl+d/ctrl+u", "Half-page down/up"},
		{"ctrl+f/ctrl+b", "Full-page down/up"},
	}
}

// textViewHelpEntries is the shared vim motion / visual / search block used
// by the full text viewers (YAML, Describe, Diff, Log, Events).
func textViewHelpEntries(kb Keybindings) []helpEntry {
	return []helpEntry{
		{"j/k", "Move cursor down/up"},
		{"h/l", "Move cursor column left/right"},
		{"0/$/^", "Line start / end / first non-blank"},
		{"w/b/e", "Word motions (W/B/E for WORD)"},
		{"gg/G", "Top / bottom"},
		{"123G", "Jump to line number"},
		{"ctrl+d/ctrl+u", "Half-page down/up"},
		{"ctrl+f/ctrl+b", "Full-page down/up"},
		{"123<motion>", "Repeat any motion N times"},
		{kb.Search, "Search in content"},
		{kb.NextMatch + "/" + kb.PrevMatch, "Next / previous match"},
		{"v/V/ctrl+v", "Character / line / block visual selection"},
		{"viw/vaw", "Select inner / around word (visual mode)"},
		{"y", "Copy line (or selection in visual mode)"},
		{"123y", "Copy N lines from cursor"},
	}
}
