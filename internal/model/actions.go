package model

// ActionsForContainer returns the action menu items for a container.
func ActionsForContainer() []ActionMenuItem {
	return []ActionMenuItem{
		{Label: "Logs", Description: "View container logs", Key: "l"},
		{Label: "Exec", Description: "Execute command in container", Key: "s"},
		{Label: "Attach", Description: "Attach to running container", Key: "A"},
		{Label: "Vuln Scan", Description: "Scan container image for vulnerabilities", Key: "V"},
		{Label: "Debug", Description: "Debug container with ephemeral container", Key: "b"},
		{Label: "Describe", Description: "Describe parent pod", Key: "v"},
		{Label: "Events", Description: "Show related events", Key: "e"},
	}
}

// ActionsForBulk returns the action menu items available for bulk operations.
// Kind-specific bulk actions are prepended when kind is non-empty, matching the
// single-resource action menu order where kind-specific actions appear first.
func ActionsForBulk(kind string) []ActionMenuItem {
	var kindActions []ActionMenuItem //nolint:prealloc // size depends on kind
	switch kind {
	case "Application":
		kindActions = []ActionMenuItem{
			{Label: "Sync", Description: "Sync selected applications", Key: "s"},
			{Label: "Sync (Apply Only)", Description: "Sync selected applications without hooks", Key: "a"},
			{Label: "Refresh", Description: "Hard refresh selected applications", Key: "R"},
		}
	}
	generic := []ActionMenuItem{
		{Label: "Logs", Description: "Stream logs from selected resources", Key: "L"},
		{Label: "Delete", Description: "Delete selected resources", Key: "D"},
		{Label: "Force Delete", Description: "Force delete selected resources", Key: "X"},
		{Label: "Scale", Description: "Scale selected resources", Key: "S"},
		{Label: "Restart", Description: "Restart selected resources", Key: "r"},
		{Label: "Labels / Annotations", Description: "Edit labels and annotations", Key: "l"},
		{Label: "Diff", Description: "Compare YAML of two resources", Key: "d"},
	}
	return append(kindActions, generic...)
}

// ActionsForKind returns the action menu items appropriate for a given resource kind.
func ActionsForKind(kind string) []ActionMenuItem {
	if actions, ok := actionsForCoreKind(kind); ok {
		return actions
	}
	if actions, ok := actionsForWorkloadKind(kind); ok {
		return actions
	}
	if actions, ok := actionsForGitOpsKind(kind); ok {
		return actions
	}
	if actions, ok := actionsForCertManagerKind(kind); ok {
		return actions
	}
	if actions, ok := actionsForOperatorKind(kind); ok {
		return actions
	}
	return actionsDefault()
}

// actionsForCoreKind returns actions for core Kubernetes resource kinds.
func actionsForCoreKind(kind string) ([]ActionMenuItem, bool) {
	switch kind {
	case "Pod":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View pod logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Debug", Description: "Debug pod with ephemeral container", Key: "B"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Port Forward", Description: "Forward local port to pod", Key: "p"},
			{Label: "Startup Analysis", Description: "Analyze pod startup timing", Key: "S"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this pod", Key: "D"},
			{Label: "Force Delete", Description: "Force delete this pod (grace-period=0)", Key: "X"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Node":
		return []ActionMenuItem{
			{Label: "Cordon", Description: "Mark node as unschedulable", Key: "c"},
			{Label: "Uncordon", Description: "Mark node as schedulable", Key: "u"},
			{Label: "Drain", Description: "Drain node (evict pods)", Key: "n"},
			{Label: "Taint", Description: "Add taint to node", Key: "t"},
			{Label: "Untaint", Description: "Remove taint from node", Key: "T"},
			{Label: "Shell", Description: "Open shell on node via debug pod", Key: "s"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in current namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Service":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View aggregated pod logs", Key: "l"},
			{Label: "Exec", Description: "Exec into pod behind service", Key: "s"},
			{Label: "Attach", Description: "Attach to pod behind service", Key: "A"},
			{Label: "Port Forward", Description: "Forward local port to service", Key: "p"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this service", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Secret":
		return []ActionMenuItem{
			{Label: "Secret Editor", Description: "Edit secret values (decode/encode base64)", Key: "e"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this secret", Key: "D"},
			{Label: "Labels / Annotations", Description: "Edit labels and annotations", Key: "l"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
			{Label: "Permissions", Description: "Check RBAC permissions", Key: "P"},
		}, true
	case "ConfigMap":
		return []ActionMenuItem{
			{Label: "ConfigMap Editor", Description: "Edit configmap key-value data", Key: "e"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this configmap", Key: "D"},
			{Label: "Labels / Annotations", Description: "Edit labels and annotations", Key: "l"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
			{Label: "Permissions", Description: "Check RBAC permissions", Key: "P"},
		}, true
	case "NetworkPolicy":
		return []ActionMenuItem{
			{Label: "Visualize", Description: "Visualize network policy rules", Key: "N"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this network policy", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
			{Label: "Permissions", Description: "Check RBAC permissions", Key: "P"},
		}, true
	case "PersistentVolumeClaim":
		return []ActionMenuItem{
			{Label: "Resize", Description: "Expand PVC storage size", Key: "r"},
			{Label: "Go to Pod", Description: "Navigate to pod using this PVC", Key: "g"},
			{Label: "Debug Mount", Description: "Run debug pod with this PVC mounted", Key: "b"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "B"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this PVC", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Ingress":
		return []ActionMenuItem{
			{Label: "Open in Browser", Description: "Open first host URL in browser", Key: "o"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this ingress", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	}
	return nil, false
}

// actionsForWorkloadKind returns actions for workload resource kinds
// (Deployment, StatefulSet, DaemonSet, Job, CronJob, etc.).
func actionsForWorkloadKind(kind string) ([]ActionMenuItem, bool) {
	switch kind {
	case "Deployment":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View aggregated pod logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Scale", Description: "Scale replica count", Key: "S"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Rollback", Description: "Rollback to previous revision", Key: "R"},
			{Label: "Port Forward", Description: "Forward local port to deployment pod", Key: "p"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this deployment", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "ReplicaSet":
		return []ActionMenuItem{
			{Label: "Scale", Description: "Scale replica count", Key: "S"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this replicaset", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "HorizontalPodAutoscaler":
		return []ActionMenuItem{
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this HPA", Key: "D"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "StatefulSet":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View aggregated pod logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Scale", Description: "Scale replica count", Key: "S"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Port Forward", Description: "Forward local port to statefulset pod", Key: "p"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this statefulset", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "DaemonSet":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View aggregated pod logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Port Forward", Description: "Forward local port to daemonset pod", Key: "p"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this daemonset", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Job":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View job logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this job", Key: "D"},
			{Label: "Force Delete", Description: "Force delete this job (grace-period=0)", Key: "X"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "CronJob":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View cronjob logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Trigger", Description: "Create a Job from this CronJob", Key: "t"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this cronjob", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "HelmRelease":
		return []ActionMenuItem{
			{Label: "Values", Description: "View user-supplied values", Key: "u"},
			{Label: "All Values", Description: "View all values (including defaults)", Key: "A"},
			{Label: "Edit Values", Description: "Edit values in $EDITOR", Key: "E"},
			{Label: "Diff", Description: "Compare default vs user-supplied values", Key: "d"},
			{Label: "Upgrade", Description: "Upgrade release to latest chart version", Key: "U"},
			{Label: "Rollback", Description: "Rollback to previous revision", Key: "R"},
			{Label: "History", Description: "Show release revision history", Key: "h"},
			{Label: "Describe", Description: "Show release info", Key: "v"},
			{Label: "Delete", Description: "Uninstall this release", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	}
	return nil, false
}

// actionsForGitOpsKind returns actions for Argo and FluxCD resource kinds.
func actionsForGitOpsKind(kind string) ([]ActionMenuItem, bool) {
	switch kind {
	case "Workflow":
		return []ActionMenuItem{
			{Label: "Watch Workflow", Description: "Live status of workflow nodes", Key: "w"},
			{Label: "Suspend Workflow", Description: "Pause workflow execution", Key: "s"},
			{Label: "Resume Workflow", Description: "Resume paused workflow", Key: "r"},
			{Label: "Stop Workflow", Description: "Stop workflow (allow exit handlers)", Key: "S"},
			{Label: "Terminate Workflow", Description: "Immediately terminate workflow", Key: "T"},
			{Label: "Resubmit Workflow", Description: "Create new workflow from this spec", Key: "R"},
			{Label: "Logs", Description: "View workflow pod logs", Key: "l"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this workflow", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "WorkflowTemplate":
		return []ActionMenuItem{
			{Label: "Submit Workflow", Description: "Create workflow from this template", Key: "s"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this template", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "ClusterWorkflowTemplate":
		return []ActionMenuItem{
			{Label: "Submit Workflow", Description: "Create workflow from this template", Key: "s"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this template", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "CronWorkflow":
		return []ActionMenuItem{
			{Label: "Suspend CronWorkflow", Description: "Suspend scheduled execution", Key: "s"},
			{Label: "Resume CronWorkflow", Description: "Resume scheduled execution", Key: "r"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this cron workflow", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Application":
		return []ActionMenuItem{
			{Label: "Configure AutoSync", Description: "Toggle autosync, self-heal, prune", Key: "A"},
			{Label: "Sync", Description: "Sync application", Key: "s"},
			{Label: "Sync (Apply Only)", Description: "Sync application without hooks", Key: "a"},
			{Label: "Terminate Sync", Description: "Terminate running sync operation", Key: "T"},
			{Label: "Refresh", Description: "Hard refresh application", Key: "R"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this application", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "ApplicationSet":
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this ApplicationSet", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Kustomization":
		return actionsFluxReconcilable("Kustomization"), true
	case "GitRepository", "HelmRepository", "HelmChart", "OCIRepository", "Bucket":
		return actionsFluxReconcilable(kind), true
	case "Alert", "Provider", "Receiver":
		return actionsFluxReconcilable(kind), true
	case "ImageRepository", "ImagePolicy", "ImageUpdateAutomation":
		return actionsFluxReconcilable(kind), true
	}
	return nil, false
}

// actionsFluxReconcilable returns the standard action set for FluxCD reconcilable resources.
func actionsFluxReconcilable(_ string) []ActionMenuItem {
	return []ActionMenuItem{
		{Label: "Reconcile", Description: "Trigger reconciliation", Key: "r"},
		{Label: "Suspend", Description: "Suspend reconciliation", Key: "s"},
		{Label: "Resume", Description: "Resume reconciliation", Key: "R"},
		{Label: "Describe", Description: "Describe resource", Key: "v"},
		{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
		{Label: "Delete", Description: "Delete this resource", Key: "D"},
		{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
		{Label: "Events", Description: "Show related events", Key: "V"},
	}
}

// actionsForCertManagerKind returns actions for cert-manager resource kinds.
func actionsForCertManagerKind(kind string) ([]ActionMenuItem, bool) {
	switch kind {
	case "Certificate":
		return []ActionMenuItem{
			{Label: "Force Renew", Description: "Trigger certificate re-issuance", Key: "r"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "CertificateRequest":
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Issuer", "ClusterIssuer":
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Order", "Challenge":
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	}
	return nil, false
}

// actionsForOperatorKind returns actions for operator-managed resource kinds
// (KEDA, External Secrets, etc.).
func actionsForOperatorKind(kind string) ([]ActionMenuItem, bool) {
	switch kind {
	case "ScaledObject", "ScaledJob":
		return []ActionMenuItem{
			{Label: "Pause", Description: "Pause autoscaling", Key: "p"},
			{Label: "Unpause", Description: "Resume autoscaling", Key: "u"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "ExternalSecret", "ClusterExternalSecret", "PushSecret":
		return []ActionMenuItem{
			{Label: "Force Refresh", Description: "Force sync external secret", Key: "r"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	}
	return nil, false
}

// actionsDefault returns the generic action menu for unrecognized resource kinds.
func actionsDefault() []ActionMenuItem {
	return []ActionMenuItem{
		{Label: "Describe", Description: "Describe resource", Key: "v"},
		{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
		{Label: "Delete", Description: "Delete this resource", Key: "D"},
		{Label: "Labels / Annotations", Description: "Edit labels and annotations", Key: "l"},
		{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
		{Label: "Events", Description: "Show related events", Key: "V"},
		{Label: "Permissions", Description: "Check RBAC permissions", Key: "P"},
	}
}

// ActionsForPortForward returns the action menu items for a port forward entry.
func ActionsForPortForward() []ActionMenuItem {
	return []ActionMenuItem{
		{Label: "Stop", Description: "Stop this port forward", Key: "s"},
		{Label: "Restart", Description: "Restart this port forward", Key: "r"},
		{Label: "Remove", Description: "Remove this entry", Key: "D"},
		{Label: "Open in Browser", Description: "Open localhost port in browser", Key: "O"},
	}
}

// MonitoringEndpoint defines a custom monitoring service endpoint.
type MonitoringEndpoint struct {
	Namespaces []string `json:"namespaces" yaml:"namespaces"` // monitoring namespaces to search
	Services   []string `json:"services" yaml:"services"`     // service names to try
	Port       string   `json:"port" yaml:"port"`             // port number (default: "9090" for prometheus, "9093" for alertmanager)
}

// MonitoringConfig defines per-cluster monitoring endpoints.
type MonitoringConfig struct {
	Prometheus   *MonitoringEndpoint `json:"prometheus" yaml:"prometheus"`
	Alertmanager *MonitoringEndpoint `json:"alertmanager" yaml:"alertmanager"`
	NodeMetrics  string              `json:"node_metrics" yaml:"node_metrics"` // "prometheus" or "metrics-api" (default: auto-detect)
}

// ConfigMonitoring maps cluster context names to monitoring config.
// The special key "_global" applies to clusters without explicit config.
var ConfigMonitoring map[string]MonitoringConfig

// SecurityConfig is per-cluster security configuration. The dashboard-
// specific fields (per_resource_indicators, per_resource_action,
// refresh_ttl, availability_ttl) from the Phase 1 design are removed;
// TTLs are code defaults (30s fetch / 60s availability). The
// per_resource_indicators field was renamed to sec_column for clarity.
type SecurityConfig struct {
	Enabled           bool                         `json:"enabled" yaml:"enabled"`
	SecColumn         bool                         `json:"sec_column" yaml:"sec_column"`
	IgnoredNamespaces []string                     `json:"ignored_namespaces,omitempty" yaml:"ignored_namespaces,omitempty"`
	Sources           map[string]SecuritySourceCfg `json:"sources" yaml:"sources"`
}

// SecuritySourceCfg is the per-source config shared across adapters.
type SecuritySourceCfg struct {
	Enabled           bool     `json:"enabled" yaml:"enabled"`
	Checks            []string `json:"checks,omitempty" yaml:"checks,omitempty"`
	IgnoredNamespaces []string `json:"ignored_namespaces,omitempty" yaml:"ignored_namespaces,omitempty"`
}
