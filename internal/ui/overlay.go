package ui

import (
	"time"

	"github.com/janosmiko/lfk/internal/tainted"
)

// OverlayNsScroll is the persistent scroll position for the namespace
// overlay, read by mouse click resolution and written by the OverlayList
// helper on every render.
var OverlayNsScroll int

// ErrorLogEntry stores a single application log entry with its timestamp and severity level.
type ErrorLogEntry struct {
	Time    time.Time
	Message string
	Level   string // "ERR", "WRN", "INF", "DBG"
}

// PortInfo represents a discovered port for the port forward overlay.
type PortInfo struct {
	Port     string
	Name     string
	Protocol string
}

// RBACCheckEntry holds RBAC check data for rendering in the overlay.
type RBACCheckEntry struct {
	Verb    string
	Allowed bool
}

// PodStartupEntry holds pod startup data for rendering, avoiding k8s package import.
type PodStartupEntry struct {
	PodName   string
	Namespace string
	TotalTime time.Duration
	Phases    []StartupPhaseEntry
}

// StartupPhaseEntry represents a single phase in the pod startup sequence for rendering.
type StartupPhaseEntry struct {
	Name     string
	Duration time.Duration
	Status   string // "completed", "in-progress", "unknown"
}

// QuotaEntry holds quota data for rendering in the overlay (avoids importing k8s).
type QuotaEntry struct {
	Name      string
	Namespace string
	Resources []QuotaResourceEntry
}

// QuotaResourceEntry holds usage data for a single resource in the quota overlay.
type QuotaResourceEntry struct {
	Name    string
	Hard    string
	Used    string
	Percent float64
}

// EventTimelineEntry holds event data for rendering in the timeline overlay.
// It mirrors k8s.EventInfo so internal/ui does not depend on the k8s package,
// and keeps the same tainted fields - see k8s.EventInfo for why.
type EventTimelineEntry struct {
	Timestamp time.Time
	Type      tainted.String // "Normal" or "Warning"
	Reason    tainted.String
	Message   tainted.String
	Source    tainted.String
	Count     int32
}

// AlertEntry holds alert data for rendering in the overlay, decoupled from the k8s package.
type AlertEntry struct {
	Name        string
	State       string // "firing", "pending"
	Severity    string // "critical", "warning", "info"
	Summary     string
	Description string
	Since       time.Time
	GrafanaURL  string
}

// NetworkPolicyEntry holds network policy data for rendering, decoupled from the k8s package.
type NetworkPolicyEntry struct {
	Name         string
	Namespace    string
	Kind         string // "" = NetworkPolicy; or CiliumNetworkPolicy / CiliumClusterwideNetworkPolicy
	NodePolicy   bool   // Cilium: selects nodes, not pods
	PodSelector  map[string]string
	PolicyTypes  []string
	IngressRules []NetpolRuleEntry
	EgressRules  []NetpolRuleEntry
	AffectedPods []string
	MatchedPods  []string // multi-policy view from a Service: backing pods this policy selects
}

// ResourceNetpolsEntry holds the policies affecting a pod or service for the
// multi-policy view opened from the resource's action menu.
type ResourceNetpolsEntry struct {
	Kind        string // "Pod" or "Service"
	Name        string
	Namespace   string
	NoSelector  bool     // Service only: the service defines no pod selector
	BackingPods []string // Service only: pods backing the service
	Policies    []NetworkPolicyEntry
}

// NetpolRuleEntry holds a single ingress/egress rule for rendering.
type NetpolRuleEntry struct {
	Ports []NetpolPortEntry
	Peers []NetpolPeerEntry
	Deny  bool   // Cilium: deny rule
	L7    string // Cilium: L7 protocol summary
}

// NetpolPortEntry holds port information for a network policy rule.
type NetpolPortEntry struct {
	Protocol string
	Port     string
}

// NetpolPeerEntry holds peer information for a network policy rule.
type NetpolPeerEntry struct {
	Type      string
	Selector  map[string]string
	CIDR      string
	Except    []string
	Namespace string
	Value     string // Entity name, FQDN pattern, or Service reference
}
