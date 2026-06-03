package k8s

import (
	"github.com/janosmiko/lfk/internal/model"
)

// populateResourceDetails fills in Ready and Restarts fields for specific resource kinds.
func populateResourceDetails(ti *model.Item, obj map[string]any, kind string) {
	ti.Raw = obj
	status, _ := obj["status"].(map[string]any)
	spec, _ := obj["spec"].(map[string]any)

	// Populate the per-condition detail section once, for every kind, from the
	// standard status.conditions array. Kind-specific handlers below only add
	// the compact at-a-glance summary to ti.Columns.
	if status != nil {
		if conditions, ok := status["conditions"].([]any); ok {
			appendAllConditions(ti, conditions)
		}
	}

	switch kind {
	case "Pod":
		populatePodDetails(ti, obj, status, spec)
	case "Deployment":
		populateDeploymentDetails(ti, obj, status, spec)
	case "StatefulSet":
		populateStatefulSetDetails(ti, obj, status, spec)
	case "DaemonSet":
		populateDaemonSetDetails(ti, obj, status, spec)
	case "ReplicaSet":
		populateReplicaSetDetails(ti, obj, status, spec)
	case "Service":
		populateServiceDetails(ti, status, spec)
	case "Ingress":
		populateIngressDetails(ti, status, spec)
	case "ConfigMap":
		populateConfigMapDetails(ti, obj)
	case "Secret":
		populateSecretDetails(ti, obj)
	case "Node":
		populateNodeDetails(ti, obj, status, spec)
	case "Namespace":
		populateNamespaceDetails(ti, status)
	case "PersistentVolumeClaim":
		populatePVCDetails(ti, status, spec)
	case "CronJob":
		populateCronJobDetails(ti, obj, status, spec)
	case "Job":
		populateJobDetails(ti, obj, status, spec)
	case "HorizontalPodAutoscaler":
		populateHPADetails(ti, status, spec)
	default:
		// Extended kinds (FluxCD, cert-manager, ArgoCD, Events, storage types, etc.)
		// and unknown/CRD resources are handled in a separate file.
		populateResourceDetailsExt(ti, obj, kind, status, spec)
	}
}
