// Package app — tabs_portforward.go
// Port-forward list helpers split out of tabs.go to keep that file under
// the 800-line cap. Builds the Port Forwards resource view and its rows.
package app

import (
	"fmt"
	"strconv"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

// portForwardItems returns the list of active port forwards as model.Items for display.
func (m *Model) portForwardItems() []model.Item {
	entries := m.portForwardMgr.Entries()
	items := make([]model.Item, 0, len(entries))
	for _, e := range entries {
		displayLocalPort := e.LocalPort
		if displayLocalPort == "0" {
			displayLocalPort = "..."
		}
		name := fmt.Sprintf("%s/%s  %s:%s", e.ResourceKind, e.ResourceName, displayLocalPort, e.RemotePort)
		extra := fmt.Sprintf("%s/%s", e.Namespace, e.Context)
		status := string(e.Status)
		age := time.Since(e.StartedAt).Truncate(time.Second).String()

		items = append(items, model.Item{
			Name:      name,
			Namespace: e.Namespace,
			Status:    status,
			Kind:      "__port_forward_entry__",
			Extra:     extra,
			Age:       age,
			CreatedAt: e.StartedAt,
			Columns: []model.KeyValue{
				{Key: "ID", Value: fmt.Sprintf("%d", e.ID)},
				{Key: "Context", Value: e.Context},
				{Key: "Local", Value: displayLocalPort},
				{Key: "Remote", Value: e.RemotePort},
				{Key: "Resource", Value: e.ResourceKind + "/" + e.ResourceName},
				{Key: "Status", Value: status},
			},
		})
	}
	return items
}

// navigateToPortForwards switches the view to the Port Forwards resource list.
// If pfLastCreatedID is set, the cursor is placed on the matching entry.
func (m *Model) navigateToPortForwards() {
	// Build the correct left column state for LevelResources. Fetch the
	// contexts before mutating any state so a failure aborts the teleport
	// cleanly instead of leaving navigation half-updated.
	contexts, err := m.client.GetContexts()
	if err != nil {
		m.setErrorFromErr("Failed to load contexts: ", err)
		return
	}

	// Record the origin so jump_back can return here after this teleport.
	m.pushJumpHistory()

	var resourceTypes []model.Item
	discoveryCtx := m.nav.Context
	if m.isUnionSentinel() && len(m.unionContexts) > 0 {
		discoveryCtx = m.unionContexts[0]
	}
	if discovered := m.discoveredResources[discoveryCtx]; len(discovered) > 0 {
		resourceTypes = model.BuildSidebarItems(discovered)
	} else {
		resourceTypes = model.BuildSidebarItems(model.SeedResources())
	}

	m.nav.ResourceType = model.ResourceTypeEntry{
		DisplayName: "Port Forwards",
		Kind:        "__port_forwards__",
		APIGroup:    "_portforward",
		APIVersion:  "v1",
		Resource:    "portforwards",
		Namespaced:  false,
	}
	m.nav.Level = model.LevelResources
	m.leftItemsHistory = [][]model.Item{contexts}
	m.leftItems = resourceTypes
	m.clearRight()
	m.setMiddleItems(m.portForwardItems())
	m.setCursor(0)
	// Try to position cursor on the newly created port forward.
	if m.pfLastCreatedID > 0 {
		for i, item := range m.middleItems {
			if m.getPortForwardID(item.Columns) == m.pfLastCreatedID {
				m.setCursor(i)
				break
			}
		}
	}
	m.clampCursor()
	m.saveCurrentSession()
}

// getPortForwardID extracts the port forward ID from item columns.
func (m *Model) getPortForwardID(columns []model.KeyValue) int {
	for _, kv := range columns {
		if kv.Key == "ID" {
			id, err := strconv.Atoi(kv.Value)
			if err == nil {
				return id
			}
		}
	}
	return 0
}
