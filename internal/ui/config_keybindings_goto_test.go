package ui

import "testing"

func TestDefaultKeybindings_GotoChords(t *testing.T) {
	kb := DefaultKeybindings()
	cases := map[string]string{
		"GotoPods": "gp", "GotoDeployments": "gd", "GotoServices": "gs",
		"GotoNodes": "gn", "GotoNamespaces": "gN", "GotoIngresses": "gi",
		"GotoJobs": "gj", "GotoCronJobs": "gc", "GotoReplicaSets": "gr",
		"GotoDaemonSets": "gD", "GotoStatefulSets": "gt", "GotoConfigMaps": "gC",
		"GotoSecrets": "gS", "GotoArgoApplications": "ga",
	}
	got := map[string]string{
		"GotoPods": kb.GotoPods, "GotoDeployments": kb.GotoDeployments,
		"GotoServices": kb.GotoServices, "GotoNodes": kb.GotoNodes,
		"GotoNamespaces": kb.GotoNamespaces, "GotoIngresses": kb.GotoIngresses,
		"GotoJobs": kb.GotoJobs, "GotoCronJobs": kb.GotoCronJobs,
		"GotoReplicaSets": kb.GotoReplicaSets, "GotoDaemonSets": kb.GotoDaemonSets,
		"GotoStatefulSets": kb.GotoStatefulSets, "GotoConfigMaps": kb.GotoConfigMaps,
		"GotoSecrets": kb.GotoSecrets, "GotoArgoApplications": kb.GotoArgoApplications,
	}
	for field, want := range cases {
		if got[field] != want {
			t.Errorf("%s = %q, want %q", field, got[field], want)
		}
	}
}
