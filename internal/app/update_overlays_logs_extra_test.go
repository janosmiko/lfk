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

// TestBuildLogTitleSanitizesContainerNames guards the missing sink found in
// review: selectedContainers was joined raw into the title after
// resourceTitleLabel, so a hostile container name skipped the sanitizing
// kind/namespace/name get.
func TestBuildLogTitleSanitizesContainerNames(t *testing.T) {
	m := Model{
		namespace: "default",
		actionCtx: actionContext{name: "my-pod", namespace: "default"},
		logView: logViewState{
			containers:         []string{"app", "sidecar", "init"},
			selectedContainers: []string{"evil\x1b[2Japp", "sidecar"},
		},
	}
	title := m.buildLogTitle()
	assert.NotContains(t, title, "\x1b")
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
