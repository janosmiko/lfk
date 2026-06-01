package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildLogTitleBasic(t *testing.T) {
	m := Model{
		namespace: "default",
		actionCtx: actionContext{name: "my-pod", namespace: "default"},
	}
	title := m.buildLogTitle()
	assert.Contains(t, title, "my-pod")
	assert.Contains(t, title, "default")
}

func TestBuildLogTitleWithContainerFilter(t *testing.T) {
	m := Model{
		namespace: "default",
		actionCtx: actionContext{name: "my-pod", namespace: "default"},
		logView: logViewState{
			containers:         []string{"app", "sidecar", "init"},
			selectedContainers: []string{"app", "sidecar"},
		},
	}
	title := m.buildLogTitle()
	assert.Contains(t, title, "app")
	assert.Contains(t, title, "sidecar")
}

func TestBuildLogTitleAllContainersSelected(t *testing.T) {
	m := Model{
		namespace: "default",
		actionCtx: actionContext{name: "my-pod", namespace: "default"},
		logView: logViewState{
			containers:         []string{"app", "sidecar"},
			selectedContainers: []string{"app", "sidecar"}, // all selected
		},
	}
	title := m.buildLogTitle()
	// When all containers are selected, no bracket filter is shown.
	assert.NotContains(t, title, "[")
}
