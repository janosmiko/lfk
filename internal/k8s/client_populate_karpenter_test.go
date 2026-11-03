package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/model"
)

func karpenterCond(condType, status, reason string) map[string]any {
	return map[string]any{"type": condType, "status": status, "reason": reason}
}

func buildKarpenterItem(apiVersion, kind string, conds ...map[string]any) model.Item {
	list := make([]any, 0, len(conds))
	for _, c := range conds {
		list = append(list, c)
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"name": "x"},
		"status":     map[string]any{"conditions": list},
	}}
	return (&Client{}).buildResourceItem(obj, &model.ResourceTypeEntry{Kind: kind})
}

func colValue(ti model.Item, key string) string {
	for _, kv := range ti.Columns {
		if kv.Key == key {
			return kv.Value
		}
	}
	return ""
}

func TestKarpenterNodeClaimStatus(t *testing.T) {
	ready := karpenterCond("Ready", "True", "Ready")
	tests := []struct {
		name       string
		conds      []map[string]any
		wantStatus string
		wantCol    [2]string
	}{
		{"ready", []map[string]any{ready, karpenterCond("Drifted", "False", "")}, "Ready", [2]string{}},
		{"drifted", []map[string]any{karpenterCond("Drifted", "True", "NodePoolDrifted"), ready}, "Drifted", [2]string{"Drift Reason", "NodePoolDrifted"}},
		{"disrupting beats drifted", []map[string]any{
			karpenterCond("DisruptionReason", "True", "Drifted"),
			karpenterCond("Drifted", "True", "AMIDrift"), ready,
		}, "Disrupting", [2]string{"Disruption Reason", "Drifted"}},
		{"instance terminating", []map[string]any{karpenterCond("InstanceTerminating", "True", "")}, "Terminating", [2]string{}},
		{"rebooting", []map[string]any{karpenterCond("Rebooting", "True", "RebootIssued"), ready}, "Rebooting", [2]string{}},
		{"launching", []map[string]any{
			karpenterCond("Launched", "Unknown", ""), karpenterCond("Ready", "Unknown", "NotLaunched"),
		}, "Launching", [2]string{}},
		{"registering", []map[string]any{
			karpenterCond("Launched", "True", ""), karpenterCond("Registered", "Unknown", ""),
			karpenterCond("Ready", "Unknown", "NotRegistered"),
		}, "Registering", [2]string{}},
		{"initializing", []map[string]any{
			karpenterCond("Launched", "True", ""), karpenterCond("Registered", "True", ""),
			karpenterCond("Initialized", "Unknown", ""), karpenterCond("Ready", "Unknown", "NotInitialized"),
		}, "Initializing", [2]string{}},
		{"launch failed", []map[string]any{
			karpenterCond("Launched", "False", "InsufficientCapacity"), karpenterCond("Ready", "False", "NotLaunched"),
		}, "NotReady", [2]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := buildKarpenterItem("karpenter.sh/v1", "NodeClaim", tt.conds...)
			assert.Equal(t, tt.wantStatus, ti.Status)
			if tt.wantCol[0] != "" {
				assert.Equal(t, tt.wantCol[1], colValue(ti, tt.wantCol[0]))
			}
		})
	}
}

func TestKarpenterNodePoolAndNodeClassStatus(t *testing.T) {
	tests := []struct {
		name, apiVersion, kind, wantStatus string
		conds                              []map[string]any
	}{
		{"nodepool ready", "karpenter.sh/v1", "NodePool", "Ready", []map[string]any{
			karpenterCond("NodeClassReady", "True", ""), karpenterCond("Ready", "True", ""),
		}},
		{"nodepool not ready", "karpenter.sh/v1", "NodePool", "NotReady", []map[string]any{
			karpenterCond("NodeClassReady", "False", "NodeClassNotFound"), karpenterCond("Ready", "False", "NodeClassNotReady"),
			karpenterCond("ValidationSucceeded", "True", ""),
		}},
		{"nodepool registration unhealthy", "karpenter.sh/v1", "NodePool", "RegistrationUnhealthy", []map[string]any{
			karpenterCond("NodeRegistrationHealthy", "False", "Unhealthy"), karpenterCond("Ready", "True", ""),
		}},
		{"ec2nodeclass not ready", "karpenter.k8s.aws/v1", "EC2NodeClass", "NotReady", []map[string]any{
			karpenterCond("AMIsReady", "False", "AMINotFound"), karpenterCond("Ready", "False", "AMIsNotReady"),
			karpenterCond("SubnetsReady", "True", ""),
		}},
		{"foreign NodePool kind untouched", "example.com/v1", "NodePool", "SubnetsReady", []map[string]any{
			karpenterCond("Ready", "False", ""), karpenterCond("SubnetsReady", "True", ""),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := buildKarpenterItem(tt.apiVersion, tt.kind, tt.conds...)
			assert.Equal(t, tt.wantStatus, ti.Status)
		})
	}
}
