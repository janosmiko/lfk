package app

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// baseOverlayModel returns a minimal Model for overlay rendering tests.
func baseOverlayModel() Model {
	return Model{
		width:         120,
		height:        40,
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		yamlCollapsed: make(map[string]bool),
	}
}

func TestRenderOverlayNone(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayNone
	bg := "background\n"
	result := m.renderOverlay(bg)
	assert.Equal(t, bg, result)
}

func TestRenderOverlayNamespace(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayNamespace
	m.overlayItems = []model.Item{
		{Name: "default"},
		{Name: "kube-system"},
	}
	m.namespace = "default"
	m.selectedNamespaces = make(map[string]bool)
	bg := strings.Repeat("x", 120) + "\n"
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
	assert.NotEqual(t, bg, result)
}

func TestRenderOverlayAction(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayAction
	m.overlayItems = []model.Item{
		{Name: "Delete"},
		{Name: "Restart"},
		{Name: "Scale"},
	}
	bg := strings.Repeat("bg-line\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
	assert.NotEqual(t, bg, result)
}

func TestRenderOverlayQuitConfirm(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayQuitConfirm
	bg := strings.Repeat("content\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
	assert.NotEqual(t, bg, result)
}

func TestRenderOverlayConfirm(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayConfirm
	m.confirmAction = "Delete pod/nginx"
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayConfirmType(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayConfirmType
	m.confirmAction = "Delete namespace production"
	m.confirmTitle = "Confirm Delete"
	m.confirmQuestion = "Delete namespace production?"
	m.confirmTypeInput = TextInput{Value: "DELE"}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayScaleInput(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayScaleInput
	m.scaleInput = TextInput{Value: "3"}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayPortForward(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayPortForward
	m.portForwardInput = TextInput{Value: "8080"}
	m.pfAvailablePorts = []ui.PortInfo{
		{Port: "80", Name: "http", Protocol: "TCP"},
		{Port: "443", Name: "https", Protocol: "TCP"},
	}
	m.actionCtx = actionContext{name: "my-pod"}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayContainerSelect(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayContainerSelect
	m.overlayItems = []model.Item{
		{Name: "nginx"},
		{Name: "sidecar"},
	}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayPodSelect(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayPodSelect
	m.overlayItems = []model.Item{
		{Name: "pod-1", Status: "Running"},
		{Name: "pod-2", Status: "Running"},
	}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayLogPodSelect(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayLogPodSelect
	m.overlayItems = []model.Item{
		{Name: "log-pod-1"},
	}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayLogContainerSelect(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayLogContainerSelect
	m.overlayItems = []model.Item{
		{Name: "app"},
		{Name: "init-container"},
	}
	m.logSelectedContainers = []string{"app"}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayBookmarks(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayBookmarks
	m.bookmarks = []model.Bookmark{
		{Name: "bookmark-1", Context: "ctx-1"},
		{Name: "bookmark-2", Context: "ctx-2"},
	}
	m.bookmarkFilter = TextInput{}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayTemplates(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayTemplates
	m.templateItems = []model.ResourceTemplate{
		{Name: "Deployment", YAML: "apiVersion: apps/v1"},
	}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayColorscheme(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayColorscheme
	m.schemeEntries = []ui.SchemeEntry{
		{Name: "dark"},
		{Name: "light"},
	}
	m.schemeFilter = TextInput{}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayFilterPreset(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayFilterPreset
	m.filterPresets = []FilterPreset{
		{Name: "Failing", Description: "Show failing pods", Key: "f"},
		{Name: "Not Ready", Description: "Show not-ready pods", Key: "n"},
	}
	m.activeFilterPreset = &FilterPreset{Name: "Failing"}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayRBAC(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayRBAC
	m.rbacResults = []k8s.RBACCheck{
		{Verb: "get", Allowed: true},
		{Verb: "delete", Allowed: false},
	}
	m.rbacKind = "pods"
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayBatchLabel(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayBatchLabel
	m.batchLabelInput = TextInput{Value: "env=prod"}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayPodStartup(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayPodStartup
	m.podStartupData = &k8s.PodStartupInfo{
		PodName:   "my-pod",
		Namespace: "default",
		TotalTime: 5200 * time.Millisecond,
		Phases: []k8s.StartupPhase{
			{Name: "Scheduled", Duration: 100 * time.Millisecond, Status: "completed"},
			{Name: "Initialized", Duration: 1 * time.Second, Status: "completed"},
			{Name: "Ready", Duration: 4100 * time.Millisecond, Status: "completed"},
		},
	}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayPodStartupNilData(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayPodStartup
	m.podStartupData = nil
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayQuotaDashboard(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayQuotaDashboard
	m.quotaData = []k8s.QuotaInfo{
		{
			Name:      "my-quota",
			Namespace: "default",
			Resources: []k8s.QuotaResource{
				{Name: "cpu", Hard: "4", Used: "2", Percent: 50},
				{Name: "memory", Hard: "8Gi", Used: "4Gi", Percent: 50},
			},
		},
	}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayEventTimeline(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayEventTimeline
	m.eventTimelineData = []k8s.EventInfo{
		{
			Timestamp:    time.Now(),
			Type:         "Normal",
			Reason:       "Created",
			Message:      "Created container nginx",
			Source:       "kubelet",
			Count:        1,
			InvolvedName: "my-pod",
			InvolvedKind: "Pod",
		},
	}
	m.actionCtx = actionContext{name: "my-pod"}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayAlerts(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayAlerts
	m.alertsData = []k8s.AlertInfo{
		{
			Name:     "HighCPU",
			State:    "firing",
			Severity: "critical",
			Summary:  "CPU usage is too high",
		},
	}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayExplainSearch(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayExplainSearch
	m.explainRecursiveFilter = TextInput{}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlaySecretEditor(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlaySecretEditor
	m.secretData = &model.SecretData{
		Keys: []string{"password"},
		Data: map[string]string{"password": "c2VjcmV0"},
	}
	m.secretEditKey = TextInput{}
	m.secretEditValue = TextInput{}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayConfigMapEditor(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayConfigMapEditor
	m.configMapData = &model.ConfigMapData{
		Keys: []string{"config.yaml"},
		Data: map[string]string{"config.yaml": "key: value"},
	}
	m.configMapEditKey = TextInput{}
	m.configMapEditValue = TextInput{}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayRollback(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayRollback
	m.rollbackRevisions = []k8s.DeploymentRevision{
		{Revision: 1, Name: "rev-1"},
		{Revision: 2, Name: "rev-2"},
	}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayHelmRollback(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayHelmRollback
	m.helmRollbackRevisions = []ui.HelmRevision{
		{Revision: 1, Chart: "nginx-1.0.0", Status: "deployed"},
	}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayLabelEditor(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayLabelEditor
	m.labelData = &model.LabelAnnotationData{
		Labels:    map[string]string{"app": "nginx", "env": "prod"},
		LabelKeys: []string{"app", "env"},
	}
	m.labelEditKey = TextInput{}
	m.labelEditValue = TextInput{}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayNetworkPolicy(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayNetworkPolicy
	m.netpolData = &k8s.NetworkPolicyInfo{
		Name:        "allow-web",
		Namespace:   "default",
		PodSelector: map[string]string{"app": "web"},
		PolicyTypes: []string{"Ingress"},
		IngressRules: []k8s.NetpolRule{
			{
				Ports: []k8s.NetpolPort{{Protocol: "TCP", Port: "80"}},
				Peers: []k8s.NetpolPeer{{Type: "PodSelector", Selector: map[string]string{"role": "frontend"}}},
			},
		},
		EgressRules: []k8s.NetpolRule{
			{
				Ports: []k8s.NetpolPort{{Protocol: "TCP", Port: "443"}},
				Peers: []k8s.NetpolPeer{{Type: "IPBlock", CIDR: "10.0.0.0/8"}},
			},
		},
	}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayNetworkPolicyNilData(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayNetworkPolicy
	m.netpolData = nil
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayCanI(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayCanI
	m.canIGroups = []model.CanIGroup{
		{
			Name: "core",
			Resources: []model.CanIResource{
				{Resource: "pods", Verbs: map[string]bool{"get": true, "list": true}},
			},
		},
	}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayCanISubject(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayCanISubject
	m.canIGroups = []model.CanIGroup{
		{
			Name: "core",
			Resources: []model.CanIResource{
				{Resource: "pods", Verbs: map[string]bool{"get": true}},
			},
		},
	}
	m.overlayItems = []model.Item{
		{Name: "admin"},
		{Name: "viewer"},
	}
	m.overlayFilter = TextInput{}
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlaySmallDimensions(t *testing.T) {
	m := baseOverlayModel()
	m.width = 20
	m.height = 10
	m.overlay = overlayAction
	m.overlayItems = []model.Item{{Name: "Delete"}}
	bg := "bg\n"
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderOverlayFilterPresetNoActive(t *testing.T) {
	m := baseOverlayModel()
	m.overlay = overlayFilterPreset
	m.filterPresets = []FilterPreset{
		{Name: "Failing", Description: "failing pods", Key: "f"},
	}
	m.activeFilterPreset = nil
	bg := strings.Repeat("bg\n", 10)
	result := m.renderOverlay(bg)
	assert.NotEmpty(t, result)
}
