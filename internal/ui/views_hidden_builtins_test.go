package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHiddenBuiltinsForView_HidesUnlistedBuiltins(t *testing.T) {
	resetViewsGlobals(t)

	v, err := BuildView(&ConfigView{
		Columns: []string{"Name", "NodeName:.spec.nodeName", "Age"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ConfigViews = map[string]*View{"pod": v}

	hidden := HiddenBuiltinsForView("Pod", "")
	assert.Equal(t, map[string]bool{
		"Context":   true,
		"Namespace": true,
		"Ready":     true,
		"Restarts":  true,
		"Status":    true,
	}, hidden)
	assert.False(t, hidden["Age"], "Age is listed; should not be hidden")
}

func TestHiddenBuiltinsForView_ListedBuiltinsKept(t *testing.T) {
	resetViewsGlobals(t)

	v, _ := BuildView(&ConfigView{
		Columns: []string{"Name", "Status", "Ready", "Age"},
	})
	ConfigViews = map[string]*View{"pod": v}

	hidden := HiddenBuiltinsForView("Pod", "")
	assert.False(t, hidden["Status"])
	assert.False(t, hidden["Ready"])
	assert.False(t, hidden["Age"])
	assert.True(t, hidden["Namespace"])
	assert.True(t, hidden["Restarts"])
}

func TestHiddenBuiltinsForView_NoViewReturnsNil(t *testing.T) {
	resetViewsGlobals(t)
	assert.Nil(t, HiddenBuiltinsForView("Pod", ""))
}

func TestHiddenBuiltinsForView_PerClusterWins(t *testing.T) {
	resetViewsGlobals(t)

	global, _ := BuildView(&ConfigView{Columns: []string{"Name", "Status", "Age"}})
	prod, _ := BuildView(&ConfigView{Columns: []string{"Name", "Namespace"}})
	ConfigViews = map[string]*View{"pod": global}
	ConfigClusterViews = map[string]map[string]*View{"prod": {"pod": prod}}

	hiddenProd := HiddenBuiltinsForView("Pod", "prod")
	assert.True(t, hiddenProd["Status"], "prod view doesn't list Status")
	assert.True(t, hiddenProd["Age"], "prod view doesn't list Age")
	assert.False(t, hiddenProd["Namespace"], "prod view lists Namespace")

	hiddenDev := HiddenBuiltinsForView("Pod", "dev")
	assert.True(t, hiddenDev["Namespace"], "global view doesn't list Namespace; dev falls back to global")
	assert.False(t, hiddenDev["Age"], "global view lists Age")
}

func TestHiddenBuiltinsForView_AllBuiltinsListedReturnsNil(t *testing.T) {
	resetViewsGlobals(t)
	v, _ := BuildView(&ConfigView{
		Columns: []string{"Name", "Context", "Namespace", "Ready", "Restarts", "Status", "Age"},
	})
	ConfigViews = map[string]*View{"pod": v}
	assert.Nil(t, HiddenBuiltinsForView("Pod", ""))
}
