package model

// ActionsForContainer returns the action menu items for a container.
func ActionsForContainer() []ActionMenuItem {
	return []ActionMenuItem{
		{Label: "Tail Logs", Description: "Tail the last 10 lines and follow", Key: "l"},
		{Label: "Logs", Description: "View container logs", Key: "L"},
		{Label: "Log Top", Description: "Aggregate logs by method/host/path/status", Key: "T"},
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
//
// Kind collision note: Knative's serving.knative.dev/Service has Kind="Service",
// which matches the core /v1/Service handler in actionsForCoreKind. Because the
// core Service menu (Tail Logs, Port Forward, etc.) is largely applicable to
// Knative Services too — they expose pods underneath — we deliberately do NOT
// route Knative Services through actionsForKnativeKind. Knative-specific verbs
// (traffic split editing, scale-min/max via autoscaling.knative.dev annotations)
// will land on a follow-up that distinguishes Service-by-APIGroup. Revision /
// Configuration / Route are Knative-only kinds and route here.
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
	if actions, ok := actionsForKarpenterKind(kind); ok {
		return actions
	}
	if actions, ok := actionsForKnativeKind(kind); ok {
		return actions
	}
	return actionsDefault()
}

// actionsForCoreKind returns actions for core Kubernetes resource kinds.
func actionsForCoreKind(kind string) ([]ActionMenuItem, bool) {
	switch kind {
	case "Pod":
		return []ActionMenuItem{
			{Label: "Tail Logs", Description: "Tail the last 10 lines and follow", Key: "l"},
			{Label: "Logs", Description: "View pod logs", Key: "L"},
			{Label: "Log Top", Description: "Aggregate logs by method/host/path/status", Key: "T"},
			{Label: "Exec", Description: "Execute command in container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Debug", Description: "Debug pod with ephemeral container", Key: "B"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Port Forward", Description: "Forward local port to pod", Key: "p"},
			{Label: "Startup Analysis", Description: "Analyze pod startup timing", Key: "S"},
			{Label: "Crash Investigator", Description: "Investigate crash loop / failing pod", Key: "I"},
			{Label: "Capture Traffic", Description: "Capture network packets to pcap", Key: "c"},
			{Label: "Network Policies", Description: "Show network policies affecting this pod", Key: "N"},
			{Label: "Go to Node", Description: "Navigate to the node hosting this pod", Key: "g"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Right-sizing", Description: "Per-container CPU/Mem recommendations", Key: "z"},
			{Label: "Security Findings", Description: "List security findings for this resource", Key: "y"},
			{Label: "Delete", Description: "Delete this pod", Key: "D"},
			{Label: "Force Delete", Description: "Force delete this pod", Key: "X"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Node":
		return []ActionMenuItem{
			{Label: "Cordon", Description: "Mark node as unschedulable", Key: "c"},
			{Label: "Uncordon", Description: "Mark node as schedulable", Key: "u"},
			{Label: "Drain", Description: "Drain node (evict pods)", Key: "n"},
			{Label: "Taints", Description: "Edit node taints (add / remove)", Key: "t"},
			{Label: "Shell", Description: "Open shell on node via debug pod", Key: "s"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in current namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Service":
		return []ActionMenuItem{
			{Label: "Tail Logs", Description: "Tail the last 10 lines and follow", Key: "l"},
			{Label: "Logs", Description: "View aggregated pod logs", Key: "L"},
			{Label: "Log Top", Description: "Aggregate logs by method/host/path/status", Key: "T"},
			{Label: "Exec", Description: "Exec into pod behind service", Key: "s"},
			{Label: "Attach", Description: "Attach to pod behind service", Key: "A"},
			{Label: "Port Forward", Description: "Forward local port to service", Key: "p"},
			{Label: "Port Forward & Open", Description: "Forward a local port and open it in the browser", Key: "O"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this service", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Capture Traffic", Description: "Capture packets on a backing pod", Key: "c"},
			{Label: "Network Policies", Description: "Show network policies affecting this service's pods", Key: "N"},
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
	case "NetworkPolicy", "CiliumNetworkPolicy", "CiliumClusterwideNetworkPolicy":
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
			{Label: "Tail Logs", Description: "Tail the last 10 lines and follow", Key: "l"},
			{Label: "Logs", Description: "View aggregated pod logs", Key: "L"},
			{Label: "Log Top", Description: "Aggregate logs by method/host/path/status", Key: "T"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Scale", Description: "Scale replica count", Key: "S"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Rollback", Description: "Pick from revision history to apply rollback", Key: "R"},
			{Label: "Port Forward", Description: "Forward local port to deployment pod", Key: "p"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Right-sizing", Description: "Per-container CPU/Mem recommendations", Key: "z"},
			{Label: "Security Findings", Description: "List security findings for this resource", Key: "y"},
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
			{Label: "Scale", Description: "Edit min/max bounds & target replicas", Key: "S"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this HPA", Key: "D"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "StatefulSet":
		return []ActionMenuItem{
			{Label: "Tail Logs", Description: "Tail the last 10 lines and follow", Key: "l"},
			{Label: "Logs", Description: "View aggregated pod logs", Key: "L"},
			{Label: "Log Top", Description: "Aggregate logs by method/host/path/status", Key: "T"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Scale", Description: "Scale replica count", Key: "S"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Port Forward", Description: "Forward local port to statefulset pod", Key: "p"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Right-sizing", Description: "Per-container CPU/Mem recommendations", Key: "z"},
			{Label: "Security Findings", Description: "List security findings for this resource", Key: "y"},
			{Label: "Delete", Description: "Delete this statefulset", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "DaemonSet":
		return []ActionMenuItem{
			{Label: "Tail Logs", Description: "Tail the last 10 lines and follow", Key: "l"},
			{Label: "Logs", Description: "View aggregated pod logs", Key: "L"},
			{Label: "Log Top", Description: "Aggregate logs by method/host/path/status", Key: "T"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Port Forward", Description: "Forward local port to daemonset pod", Key: "p"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Right-sizing", Description: "Per-container CPU/Mem recommendations", Key: "z"},
			{Label: "Security Findings", Description: "List security findings for this resource", Key: "y"},
			{Label: "Delete", Description: "Delete this daemonset", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Job":
		return []ActionMenuItem{
			{Label: "Tail Logs", Description: "Tail the last 10 lines and follow", Key: "l"},
			{Label: "Logs", Description: "View job logs", Key: "L"},
			{Label: "Log Top", Description: "Aggregate logs by method/host/path/status", Key: "T"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Right-sizing", Description: "Per-container CPU/Mem recommendations", Key: "z"},
			{Label: "Security Findings", Description: "List security findings for this resource", Key: "y"},
			{Label: "Delete", Description: "Delete this job", Key: "D"},
			{Label: "Force Delete", Description: "Force delete this job", Key: "X"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "CronJob":
		return []ActionMenuItem{
			{Label: "Tail Logs", Description: "Tail the last 10 lines and follow", Key: "l"},
			{Label: "Logs", Description: "View cronjob logs", Key: "L"},
			{Label: "Log Top", Description: "Aggregate logs by method/host/path/status", Key: "T"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "s"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Trigger", Description: "Create a Job from this CronJob", Key: "t"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Right-sizing", Description: "Per-container CPU/Mem recommendations", Key: "z"},
			{Label: "Security Findings", Description: "List security findings for this resource", Key: "y"},
			{Label: "Delete", Description: "Delete this cronjob", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "HelmRelease":
		return []ActionMenuItem{
			{Label: "Values", Description: "View user-supplied values", Key: "u"},
			{Label: "All Values", Description: "View all values (including defaults)", Key: "A"},
			{Label: "Edit Values", Description: "Edit values in $KUBE_EDITOR or $EDITOR", Key: "E"},
			{Label: "Diff", Description: "Compare default vs user-supplied values", Key: "d"},
			{Label: "Upgrade", Description: "Upgrade release to latest chart version", Key: "U"},
			{Label: "Rollback", Description: "Pick from revision history to apply rollback", Key: "R"},
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
			{Label: "Tail Logs", Description: "Tail the last 10 lines and follow", Key: "l"},
			{Label: "Logs", Description: "View workflow pod logs", Key: "L"},
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
			{Label: "Sync Wave Timeline", Description: "Visualize sync wave order and status", Key: "W"},
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

// actionsForKarpenterKind returns actions for Karpenter resource kinds.
// NodeClaim represents a single provisioned node — Disrupt deletes the
// claim (Karpenter then terminates the underlying instance), while
// Cordon / Uncordon / Drain Node resolve the claim's status.nodeName
// and forward to the standard kubectl helpers so the same UX as a
// plain Node row works from the NodeClaim view. NodePool and
// EC2NodeClass stick to the generic Describe/Edit/Delete/Events surface
// for now — per-pool disruption controls (spec.disruption.budgets)
// overlap with user-managed config and are deferred to a follow-up.
func actionsForKarpenterKind(kind string) ([]ActionMenuItem, bool) {
	switch kind {
	case "NodeClaim":
		return []ActionMenuItem{
			{Label: "Disrupt", Description: "Delete claim; Karpenter terminates the node", Key: "X"},
			{Label: "Cordon Node", Description: "Cordon the bound node", Key: "c"},
			{Label: "Uncordon Node", Description: "Uncordon the bound node", Key: "u"},
			{Label: "Drain Node", Description: "Drain the bound node (evict pods)", Key: "n"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this NodeClaim", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "NodePool":
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this NodePool", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "EC2NodeClass":
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this EC2NodeClass", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	}
	return nil, false
}

// actionsForKnativeKind returns actions for Knative Serving resource
// kinds. Revision exposes Activate — patching the parent Service's
// spec.traffic to send 100% of traffic to the selected revision, the
// canonical Knative rollback / promotion gesture. Configuration and
// Route stay on the standard surface; their CRD printer columns (URL,
// LatestReady, Ready) carry the operationally useful state and the
// generic Describe/Edit/Delete/Events covers the rest.
//
// Knative Service (Kind="Service") is NOT routed here — see the
// comment on ActionsForKind. The core Service menu (Tail Logs, Port
// Forward, etc.) applies to Knative Services through the pods they
// expose, and a Knative-specific Service overlay (traffic split, scale
// min/max) is deferred to a follow-up.
func actionsForKnativeKind(kind string) ([]ActionMenuItem, bool) {
	switch kind {
	case "Revision":
		return []ActionMenuItem{
			{Label: "Activate", Description: "Send 100% traffic to this revision (rolls the parent Service)", Key: "a"},
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this Revision", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Configuration":
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this Configuration", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	case "Route":
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "v"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this Route", Key: "D"},
			{Label: "Debug Pod", Description: "Run standalone alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "V"},
		}, true
	}
	return nil, false
}

// ActionsForLonghornNode returns the action menu for longhorn.io Node CRDs.
// Distinct from the core-node menu (Cordon / Drain / Taint / Shell are
// kubectl node verbs that do not apply here): a longhorn.io node is managed
// through its spec. Evict Replicas / Cancel Eviction toggle
// spec.evictionRequested so Longhorn rebuilds replicas elsewhere before
// removing them; Force Delete disables scheduling then deletes past the
// validating webhook (which rejects deletion of a still-schedulable node).
func ActionsForLonghornNode() []ActionMenuItem {
	return []ActionMenuItem{
		{Label: "Evict Replicas", Description: "Drain replicas off this node (rebuilds them elsewhere first)", Key: "e"},
		{Label: "Cancel Eviction", Description: "Stop an in-progress replica eviction", Key: "C"},
		{Label: "Describe", Description: "Describe resource", Key: "v"},
		{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
		{Label: "Delete", Description: "Delete this Longhorn node", Key: "D"},
		{Label: "Force Delete", Description: "Force delete this Longhorn node (disable scheduling, then delete)", Key: "X"},
		{Label: "Events", Description: "Show related events", Key: "V"},
		{Label: "Permissions", Description: "Check RBAC permissions", Key: "P"},
	}
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

// Labels for the cluster-picker action menu. Kept as exported
// constants so the app-layer dispatcher can switch on them without
// duplicating the literal strings.
const (
	ActionLabelSetColor            = "Set color"
	ActionLabelManageLocalClusters = "Local clusters"
)

// ClusterPickerKeys carries the user-configurable shortcuts for the
// cluster-picker actions so the model layer can render in-menu key
// hints without importing the ui package. The app layer populates
// the struct from ui.ActiveKeybindings before calling
// ActionsForClusterPicker.
//
// Only SetColor is user-configurable. The Local clusters action's
// chip is hardcoded as `[n]` in the constructor: action-menu chips
// are single-letter activators and the global LocalClusterManager
// binding is a chord (ctrl+n) that can't reach the action menu's
// keypress dispatcher anyway.
type ClusterPickerKeys struct {
	SetColor string
}

// localClustersMenuChip is the in-menu single-letter activator for
// the Local clusters action. Hardcoded because (a) action-menu
// dispatch routes single keypresses, not chords, and (b) the global
// LocalClusterManager binding (ctrl+n) opens the manager from
// anywhere; the action-menu chip is a separate discoverability
// channel and need not echo it.
const localClustersMenuChip = "n"

// ActionsForClusterPicker returns the action menu items appropriate
// when the user is at LevelClusters (the kubeconfig-context picker).
// Each entry's Key carries the bare-keypress shortcut so the in-menu
// hint stays in sync with the user's keybinding configuration.
func ActionsForClusterPicker(keys ClusterPickerKeys) []ActionMenuItem {
	return []ActionMenuItem{
		{
			Label:       ActionLabelSetColor,
			Description: "Assign a background tint to this context",
			Key:         keys.SetColor,
		},
		{
			Label:       ActionLabelManageLocalClusters,
			Description: "Manage kind/k3d/minikube clusters",
			Key:         localClustersMenuChip,
		},
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

// ActionsForCapture returns the action menu items for a capture entry.
func ActionsForCapture() []ActionMenuItem {
	return []ActionMenuItem{
		{Label: "Open", Description: "Re-open the capture overlay attached to this entry", Key: "o"},
		{Label: "Stop", Description: "Stop a running capture", Key: "s"},
		{Label: "Delete File", Description: "Delete the on-disk pcap file", Key: "D"},
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
