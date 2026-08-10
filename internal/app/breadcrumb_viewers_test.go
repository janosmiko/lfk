package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// resourceTitleLabel is the shared "Kind namespace/name" formatter that keeps
// every viewer sub-title consistent for the same resource.
func TestResourceTitleLabel(t *testing.T) {
	assert.Equal(t, "Pod argo-cd/argocd-redis-ha-server-2",
		resourceTitleLabel("Pod", "argo-cd", "argocd-redis-ha-server-2"))
	// Cluster-scoped: no namespace segment.
	assert.Equal(t, "Node ip-10-0-0-1", resourceTitleLabel("Node", "", "ip-10-0-0-1"))
	// Unknown kind: just namespace/name.
	assert.Equal(t, "default/my-pod", resourceTitleLabel("", "default", "my-pod"))
	// Bare name.
	assert.Equal(t, "x", resourceTitleLabel("", "", "x"))
}

// resourceTitleLabel feeds seven fullscreen view titles (logs, describe,
// exec, object explorer, YAML, event viewer, logtop) with kind/namespace/name
// taken raw from cluster-controlled resources. A hostile name must not be
// able to reorder or hijack the title bar; fixing it once here covers every
// call site.
func TestResourceTitleLabel_SanitizesTerminalEscapes(t *testing.T) {
	label := resourceTitleLabel("Pod", "default", "evil\x1b[2Jpod")
	assert.NotContains(t, label, "\x1b")
	assert.Equal(t, "Pod default/evil[2Jpod", label)

	label = resourceTitleLabel("Pod", "default", "evil\u202epod")
	assert.NotContains(t, label, "\u202e", "bidi override must not survive")

	label = resourceTitleLabel("Pod", "default", "evil\x1b]52;c;ZXZpbA==\x07pod")
	assert.NotContains(t, label, "\x1b")
	assert.NotContains(t, label, "\x07")

	// Ordinary names render identically.
	assert.Equal(t, "Pod argo-cd/argocd-redis-ha-server-2",
		resourceTitleLabel("Pod", "argo-cd", "argocd-redis-ha-server-2"))
}

// The fullscreen viewers (logs, exec, describe, diff, events) must surface the
// object they show in the top breadcrumb, the same way the YAML viewer and the
// Object Explorer do. Each case is exercised through explorerDrillPath, which
// builds the trailing breadcrumb segment(s).

func TestDrillPathLogsShowsPodAndContainer(t *testing.T) {
	m := Model{mode: modeLogs, nav: model.NavigationState{Level: model.LevelResources}}
	m.actionCtx.name = "my-pod"
	assert.Equal(t, []string{"my-pod"}, m.explorerDrillPath())

	m.actionCtx.containerName = "web"
	assert.Equal(t, []string{"my-pod", "web"}, m.explorerDrillPath())
}

func TestDrillPathExecShowsPodAndContainer(t *testing.T) {
	m := Model{mode: modeExec, nav: model.NavigationState{Level: model.LevelResources}}
	m.actionCtx.name = "my-pod"
	m.actionCtx.containerName = "web"
	assert.Equal(t, []string{"my-pod", "web"}, m.explorerDrillPath())
}

func TestDrillPathDescribeShowsResource(t *testing.T) {
	m := Model{mode: modeDescribe, nav: model.NavigationState{Level: model.LevelResources}}
	m.actionCtx.name = "my-deploy"
	assert.Equal(t, []string{"my-deploy"}, m.explorerDrillPath())
}

func TestDrillPathEventViewerShowsResourceOrNothing(t *testing.T) {
	m := Model{mode: modeEventViewer, nav: model.NavigationState{Level: model.LevelResources}}
	m.actionCtx.name = "my-pod"
	assert.Equal(t, []string{"my-pod"}, m.explorerDrillPath())

	// Cluster-wide events (no target) add nothing.
	m.actionCtx.name = ""
	assert.Nil(t, m.explorerDrillPath())
}

func TestDrillPathDiffShowsBothNames(t *testing.T) {
	m := Model{mode: modeDiff, nav: model.NavigationState{Level: model.LevelResources}}
	m.diffView.leftName = "pod-a"
	m.diffView.rightName = "pod-b"
	assert.Equal(t, []string{"pod-a ↔ pod-b"}, m.explorerDrillPath())
}

// At the containers level the nav breadcrumb already ends with the pod
// (OwnedName), so the drill path must not repeat it — only the container.
func TestDrillPathLogsDedupsPodAtContainerLevel(t *testing.T) {
	m := Model{
		mode: modeLogs,
		nav: model.NavigationState{
			Level:     model.LevelContainers,
			OwnedName: "my-pod",
		},
	}
	m.actionCtx.name = "my-pod" // same pod the nav breadcrumb already shows
	m.actionCtx.containerName = "web"
	assert.Equal(t, []string{"web"}, m.explorerDrillPath())
}
