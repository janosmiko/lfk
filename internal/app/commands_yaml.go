package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
)

// copyYAMLToClipboard fetches the YAML for the selected resource and sends it for clipboard copy.
func (m Model) copyYAMLToClipboard() tea.Cmd {
	kctx := m.nav.Context
	ns := m.resolveNamespace()

	switch m.nav.Level {
	case model.LevelResources:
		rt := m.nav.ResourceType
		// Gate on visible selection, not raw hasSelection() — selected rows
		// can be filtered out of view, in which case we want the single-item
		// (cursor) path, not an empty multi-doc fetch.
		if items := m.selectedItemsList(); len(items) > 0 {
			type fetchTarget struct {
				ns, name string
			}
			targets := make([]fetchTarget, len(items))
			for i, it := range items {
				itemNs := ns
				if it.Namespace != "" {
					itemNs = it.Namespace
				}
				targets[i] = fetchTarget{ns: itemNs, name: it.Name}
			}
			return func() tea.Msg {
				docs := make([]string, 0, len(targets))
				for _, t := range targets {
					content, err := m.client.GetResourceYAML(context.Background(), kctx, t.ns, rt, t.name)
					if err != nil {
						return yamlClipboardMsg{err: fmt.Errorf("%s/%s: %w", t.ns, t.name, err)}
					}
					docs = append(docs, strings.TrimRight(content, "\n"))
				}
				return yamlClipboardMsg{
					content: strings.Join(docs, "\n---\n") + "\n",
					count:   len(docs),
				}
			}
		}
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		return func() tea.Msg {
			content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
			return yamlClipboardMsg{content: content, count: 1, err: err}
		}
	case model.LevelOwned:
		// Bulk path mirrors LevelResources: gate on visible selection so a
		// selection that's been filtered out of view falls through to the
		// cursor branch instead of dispatching an empty fetch. Per-item Kind
		// dispatch (Pod -> GetPodYAML; others -> resolveOwnedResourceType +
		// GetResourceYAML) is resolved before the closure runs to keep the
		// goroutine off the model.
		if items := m.selectedItemsList(); len(items) > 0 {
			type fetchTarget struct {
				ns, name string
				isPod    bool
				rt       model.ResourceTypeEntry
				resolved bool
				kind     string
			}
			targets := make([]fetchTarget, len(items))
			for i, it := range items {
				itemNs := ns
				if it.Namespace != "" {
					itemNs = it.Namespace
				}
				t := fetchTarget{ns: itemNs, name: it.Name, kind: it.Kind, isPod: it.Kind == "Pod"}
				if !t.isPod {
					t.rt, t.resolved = m.resolveOwnedResourceType(&items[i])
				}
				targets[i] = t
			}
			return func() tea.Msg {
				docs := make([]string, 0, len(targets))
				for _, t := range targets {
					var (
						content string
						err     error
					)
					switch {
					case t.isPod:
						content, err = m.client.GetPodYAML(context.Background(), kctx, t.ns, t.name)
					case t.resolved:
						content, err = m.client.GetResourceYAML(context.Background(), kctx, t.ns, t.rt, t.name)
					default:
						err = fmt.Errorf("unknown resource type: %s", t.kind)
					}
					if err != nil {
						return yamlClipboardMsg{err: fmt.Errorf("%s/%s: %w", t.ns, t.name, err)}
					}
					docs = append(docs, strings.TrimRight(content, "\n"))
				}
				return yamlClipboardMsg{
					content: strings.Join(docs, "\n---\n") + "\n",
					count:   len(docs),
				}
			}
		}
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		if sel.Kind == "Pod" {
			return func() tea.Msg {
				content, err := m.client.GetPodYAML(context.Background(), kctx, itemNs, name)
				return yamlClipboardMsg{content: content, count: 1, err: err}
			}
		}
		rt, ok := m.resolveOwnedResourceType(sel)
		if !ok {
			return func() tea.Msg {
				return yamlClipboardMsg{err: fmt.Errorf("unknown resource type: %s", sel.Kind)}
			}
		}
		return func() tea.Msg {
			content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
			return yamlClipboardMsg{content: content, count: 1, err: err}
		}
	case model.LevelContainers:
		podName := m.nav.OwnedName
		return func() tea.Msg {
			content, err := m.client.GetPodYAML(context.Background(), kctx, ns, podName)
			return yamlClipboardMsg{content: content, count: 1, err: err}
		}
	}
	return nil
}

// exportResourceToFile saves the selected resource YAML to a file.
func (m Model) exportResourceToFile() tea.Cmd {
	kctx := m.nav.Context
	ns := m.resolveNamespace()

	var fetchYAML func() (string, string, error) // returns (yaml, kindForFilename, error)

	switch m.nav.Level {
	case model.LevelResources:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		rt := m.nav.ResourceType
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		kind := strings.ToLower(rt.Kind)
		fetchYAML = func() (string, string, error) {
			content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
			return content, kind, err
		}
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		if sel.Kind == "Pod" {
			fetchYAML = func() (string, string, error) {
				content, err := m.client.GetPodYAML(context.Background(), kctx, itemNs, name)
				return content, "pod", err
			}
		} else {
			rt, ok := m.resolveOwnedResourceType(sel)
			if !ok {
				return func() tea.Msg {
					return exportDoneMsg{err: fmt.Errorf("unknown resource type: %s", sel.Kind)}
				}
			}
			kind := strings.ToLower(rt.Kind)
			fetchYAML = func() (string, string, error) {
				content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
				return content, kind, err
			}
		}
	case model.LevelContainers:
		podName := m.nav.OwnedName
		fetchYAML = func() (string, string, error) {
			content, err := m.client.GetPodYAML(context.Background(), kctx, ns, podName)
			return content, "pod", err
		}
	default:
		return nil
	}

	return func() tea.Msg {
		yaml, kind, err := fetchYAML()
		if err != nil {
			return exportDoneMsg{err: fmt.Errorf("fetching resource: %w", err)}
		}

		// Build filename: <kind>_<name>.yaml
		var name string
		switch m.nav.Level {
		case model.LevelContainers:
			name = m.nav.OwnedName
		default:
			sel := m.selectedMiddleItem()
			if sel != nil {
				name = sel.Name
			}
		}
		sanitized := strings.ReplaceAll(name, "/", "_")
		filename := fmt.Sprintf("%s_%s.yaml", kind, sanitized)

		if err := os.WriteFile(filename, []byte(yaml), 0o644); err != nil {
			return exportDoneMsg{err: fmt.Errorf("writing file: %w", err)}
		}

		abs, _ := filepath.Abs(filename)
		if abs == "" {
			abs = filename
		}
		return exportDoneMsg{path: abs}
	}
}
