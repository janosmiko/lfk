package model

// TaintPreset is a well-known taint offered by the editor's picker, with a
// short note on what applying it does.
type TaintPreset struct {
	Taint Taint
	Desc  string
}

// CommonTaints are widely used taints a user applies by hand, offered as
// starting points in the taint editor — the keys are long and easy to
// mistype, and a typo silently produces a taint nothing tolerates.
//
// The node-controller-managed condition taints (not-ready, unreachable,
// memory-pressure, disk-pressure, pid-pressure, network-unavailable) are
// deliberately absent: the controller adds and removes them from live node
// conditions, so a hand-applied copy is either redundant or fought over.
//
// Every entry is a real taint documented by Kubernetes or by the platform
// that applies it; values stay editable after picking.
var CommonTaints = []TaintPreset{
	{
		Taint{Key: "node-role.kubernetes.io/control-plane", Effect: "NoSchedule"},
		"Reserve control-plane nodes",
	},
	{
		Taint{Key: "CriticalAddonsOnly", Value: "true", Effect: "NoSchedule"},
		"System addons only (AKS, GKE, kubeadm)",
	},
	{
		Taint{Key: "dedicated", Value: "", Effect: "NoSchedule"},
		"Dedicate nodes to one workload — set the value",
	},
	{
		Taint{Key: "node.kubernetes.io/unschedulable", Effect: "NoSchedule"},
		"Cordon: block new pods, keep running ones",
	},
	{
		Taint{Key: "node.kubernetes.io/out-of-service", Value: "nodeshutdown", Effect: "NoExecute"},
		"Shut-down node: force pod eviction and volume detach",
	},
	{
		Taint{Key: "nvidia.com/gpu", Value: "present", Effect: "NoSchedule"},
		"Reserve GPU nodes for GPU workloads",
	},
	{
		Taint{Key: "node.cilium.io/agent-not-ready", Effect: "NoSchedule"},
		"Hold pods until the Cilium agent is ready",
	},
	{
		Taint{Key: "node.cloudprovider.kubernetes.io/uninitialized", Effect: "NoSchedule"},
		"Hold pods until the cloud controller initializes the node",
	},
	{
		Taint{Key: "karpenter.sh/disrupted", Effect: "NoSchedule"},
		"Karpenter is disrupting this node",
	},
	{
		Taint{Key: "cloud.google.com/gke-spot", Value: "true", Effect: "NoSchedule"},
		"GKE Spot nodes: opt-in workloads only",
	},
	{
		Taint{Key: "kubernetes.azure.com/scalesetpriority", Value: "spot", Effect: "NoSchedule"},
		"AKS Spot nodes: opt-in workloads only",
	},
	{
		Taint{Key: "eks.amazonaws.com/compute-type", Value: "fargate", Effect: "NoSchedule"},
		"EKS Fargate nodes",
	},
}
