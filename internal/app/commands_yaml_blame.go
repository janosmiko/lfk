package app

import (
	"context"
	"errors"
	"hash/fnv"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
)

// errNoBlameTarget means the viewer is showing something with no API object
// behind it, so there are no field managers to read.
var errNoBlameTarget = errors.New("field owners are not available for this view")

// builtinPodResourceType is the built-in type for the levels that show a Pod without
// carrying a ResourceTypeEntry (owned Pods and the container level).
var builtinPodResourceType = model.ResourceTypeEntry{
	Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
}

// yamlBlameTarget names the object the YAML viewer is showing. It mirrors the
// selection logic of loadYAML rather than sharing it: loadYAML branches on
// three levels with two different fetch calls, and blame only needs the
// address of the object.
func (m Model) yamlBlameTarget() (kctx, ns string, rt model.ResourceTypeEntry, name string, ok bool) {
	kctx, ns = m.effectiveContext(), m.resolveNamespace()

	if m.nav.Level == model.LevelContainers {
		return kctx, ns, builtinPodResourceType, m.nav.OwnedName, m.nav.OwnedName != ""
	}

	sel := m.selectedMiddleItem()
	if sel == nil {
		return "", "", model.ResourceTypeEntry{}, "", false
	}
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	if sel.ClusterName != "" {
		kctx = sel.ClusterName
	}

	switch m.nav.Level {
	case model.LevelResources:
		return kctx, ns, m.nav.ResourceType, sel.Name, true
	case model.LevelOwned:
		if sel.Kind == "Pod" {
			return kctx, ns, builtinPodResourceType, sel.Name, true
		}
		owned, found := m.resolveOwnedResourceType(sel)
		return kctx, ns, owned, sel.Name, found
	}
	return "", "", model.ResourceTypeEntry{}, "", false
}

// loadYAMLBlame fetches the field managers of the object on screen. It is a
// second GET on purpose: the YAML fetch strips managedFields so the rendered
// document stays readable, and this runs only when the user asks for blame.
func (m Model) loadYAMLBlame() tea.Cmd {
	kctx, ns, rt, name, ok := m.yamlBlameTarget()
	if !ok {
		return func() tea.Msg {
			return yamlBlameLoadedMsg{err: errNoBlameTarget}
		}
	}
	client := m.client
	content := m.yamlView.content
	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindYAMLFetch,
		"Field owners: "+name,
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			owners, err := client.GetFieldOwners(ctx, kctx, ns, rt, name)
			if err != nil {
				return yamlBlameLoadedMsg{err: err}
			}
			return yamlBlameLoadedMsg{
				blame:       computeYAMLBlame(content, owners),
				contentHash: yamlContentHash(content),
				contentLen:  len(content),
			}
		},
	)
}

// yamlContentHash identifies a rendered document cheaply, so a late reply can
// be matched against what is on screen now.
func yamlContentHash(content string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(content))
	return h.Sum64()
}

// yamlBlameMinInlineWidth is the space the trailing blame note needs before it
// is worth printing. Below this the note would be truncated to nothing useful,
// so the line is left alone.
const yamlBlameMinInlineWidth = 14
