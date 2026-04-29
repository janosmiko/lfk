package k8s

import (
	"github.com/janosmiko/lfk/internal/model"
)

// populateResourceDetails fills in Ready and Restarts fields for specific resource kinds.
func populateResourceDetails(ti *model.Item, obj map[string]any, kind string) {
	status, _ := obj["status"].(map[string]any)
	spec, _ := obj["spec"].(map[string]any)

	switch kind {
	case "Pod":
		populatePodDetails(ti, obj, status, spec)
	case "Deployment":
		populateDeploymentDetails(ti, status, spec)
	case "StatefulSet":
		populateStatefulSetDetails(ti, status, spec)
	case "DaemonSet":
		populateDaemonSetDetails(ti, status, spec)
	case "ReplicaSet":
		populateReplicaSetDetails(ti, status, spec)
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
	case "PersistentVolumeClaim":
		populatePVCDetails(ti, status, spec)
	case "CronJob":
		populateCronJobDetails(ti, status, spec)
	case "Job":
		populateJobDetails(ti, status, spec)
	case "HorizontalPodAutoscaler":
		populateHPADetails(ti, status, spec)
	default:
		// Extended kinds (FluxCD, cert-manager, ArgoCD, Events, storage types, etc.)
		// and unknown/CRD resources are handled in a separate file.
		populateResourceDetailsExt(ti, obj, kind, status, spec)
	}
}
