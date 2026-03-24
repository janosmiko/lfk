package model

// ActionsForContainer returns the action menu items for a container.
func ActionsForContainer() []ActionMenuItem {
	return []ActionMenuItem{
		{Label: "Logs", Description: "View container logs", Key: "l"},
		{Label: "Exec", Description: "Execute command in container", Key: "s"},
		{Label: "Attach", Description: "Attach to running container", Key: "A"},
		{Label: "Debug", Description: "Debug container with ephemeral container", Key: "b"},
		{Label: "Describe", Description: "Describe parent pod", Key: "v"},
		{Label: "Events", Description: "Show related events", Key: "V"},
	}
}

// ActionsForBulk returns the action menu items available for bulk operations.
// Kind-specific bulk actions are appended when kind is non-empty.
func ActionsForBulk(kind string) []ActionMenuItem {
	actions := []ActionMenuItem{
		{Label: "Logs", Description: "Stream logs from selected resources", Key: "L"},
		{Label: "Delete", Description: "Delete selected resources", Key: "D"},
		{Label: "Force Delete", Description: "Force delete selected resources", Key: "X"},
		{Label: "Scale", Description: "Scale selected resources", Key: "S"},
		{Label: "Restart", Description: "Restart selected resources", Key: "r"},
		{Label: "Labels / Annotations", Description: "Edit labels and annotations", Key: "l"},
		{Label: "Diff", Description: "Compare YAML of two resources", Key: "d"},
	}
	switch kind {
	case "Application":
		actions = append(actions,
			ActionMenuItem{Label: "Sync", Description: "Sync selected applications", Key: "s"},
			ActionMenuItem{Label: "Sync (Apply Only)", Description: "Sync selected applications without hooks", Key: "a"},
			ActionMenuItem{Label: "Refresh", Description: "Hard refresh selected applications", Key: "R"},
		)
	}
	return actions
}

// ActionsForKind returns the action menu items appropriate for a given resource kind.
func ActionsForKind(kind string) []ActionMenuItem {
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
		}
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
		}
	case "ReplicaSet":
		return []ActionMenuItem{
			{Label: "Scale", Description: "Scale replica count", Key: "S"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this replicaset", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}
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
		}
	case "HorizontalPodAutoscaler":
		return []ActionMenuItem{
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this HPA", Key: "D"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}
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
		}
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
		}
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
		}
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
		}
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
		}
	case "Application":
		return []ActionMenuItem{
			{Label: "Sync", Description: "Sync application", Key: "s"},
			{Label: "Sync (Apply Only)", Description: "Sync application without hooks", Key: "a"},
			{Label: "Terminate Sync", Description: "Terminate running sync operation", Key: "T"},
			{Label: "Refresh", Description: "Hard refresh application", Key: "R"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this application", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}
	case "PersistentVolumeClaim":
		return []ActionMenuItem{
			{Label: "Go to Pod", Description: "Navigate to pod using this PVC", Key: "g"},
			{Label: "Debug Mount", Description: "Run debug pod with this PVC mounted", Key: "b"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "B"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this PVC", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}
	case "Ingress":
		return []ActionMenuItem{
			{Label: "Open in Browser", Description: "Open first host URL in browser", Key: "o"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this ingress", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}
	case "HelmRelease":
		return []ActionMenuItem{
			{Label: "Values", Description: "View user-supplied values", Key: "u"},
			{Label: "All Values", Description: "View all values (including defaults)", Key: "A"},
			{Label: "Edit Values", Description: "Edit values in $EDITOR", Key: "E"},
			{Label: "Rollback", Description: "Rollback to previous revision", Key: "R"},
			{Label: "Describe", Description: "Show release info", Key: "v"},
			{Label: "Delete", Description: "Uninstall this release", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}
	case "Kustomization":
		// FluxCD Kustomization
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
	case "GitRepository", "HelmRepository", "HelmChart", "OCIRepository", "Bucket":
		// FluxCD Source resources
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
	case "Alert", "Provider", "Receiver":
		// FluxCD Notification resources
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
	case "ImageRepository", "ImagePolicy", "ImageUpdateAutomation":
		// FluxCD Image resources
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
	case "Certificate", "CertificateRequest":
		// cert-manager Certificate resources
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}
	case "Issuer", "ClusterIssuer":
		// cert-manager Issuer resources
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}
	case "Order", "Challenge":
		// cert-manager ACME resources
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}
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
		}
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
		}
	case "NetworkPolicy":
		return []ActionMenuItem{
			{Label: "Visualize", Description: "Visualize network policy rules", Key: "N"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this network policy", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
			{Label: "Permissions", Description: "Check RBAC permissions", Key: "P"},
		}
	default:
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

	// Note: "Permissions" action is also available for all kinds — it's appended
	// by the action dispatch logic if not present in the kind-specific list.
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
// The special key "default" applies to clusters without explicit config.
var ConfigMonitoring map[string]MonitoringConfig
