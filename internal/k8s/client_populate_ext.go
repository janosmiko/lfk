package k8s

import (
	"fmt"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

func populateResourceDetailsExt(ti *model.Item, obj map[string]any, kind string, status, spec map[string]any) {
	switch kind {
	case "Kustomization", "GitRepository", "HelmRepository", "HelmChart", "OCIRepository", "Bucket",
		"Alert", "Provider", "Receiver", "ImageRepository", "ImagePolicy", "ImageUpdateAutomation":
		populateFluxCDResource(ti, obj, status)

	case "Certificate", "CertificateRequest", "Issuer", "ClusterIssuer", "Order", "Challenge":
		populateCertManagerResource(ti, status, spec)

	case "Application", "ApplicationSet":
		populateArgoCDApplication(ti, obj, status, spec, kind)

	case "Event":
		populateEvent(ti, obj)

	case "IngressClass":
		populateIngressClass(ti, obj)

	case "StorageClass":
		populateStorageClass(ti, obj)

	case "PersistentVolume":
		populatePersistentVolume(ti, status, spec)

	case "ResourceQuota":
		populateResourceQuota(ti, status, spec)

	case "LimitRange":
		populateLimitRange(ti, spec)

	case "PodDisruptionBudget":
		populatePodDisruptionBudget(ti, status, spec)

	case "NetworkPolicy":
		populateNetworkPolicy(ti, spec)

	case "ServiceAccount":
		populateServiceAccount(ti, obj)

	case "Endpoints":
		populateEndpoints(ti, obj)

	case "EndpointSlice":
		populateEndpointSlice(ti, obj)

	case "PriorityClass":
		if val, ok := spec["globalDefault"].(bool); ok && val {
			ti.Name += " (default)"
			ti.Status = "default"
		}

	case "Workflow":
		populateArgoWorkflow(ti, status)

	default:
		populateGenericCRDResource(ti, status)
	}
}

const (
	EventColObject   = "Object"
	EventColReason   = "Reason"
	EventColMessage  = "Message"
	EventColCount    = "Count"
	EventColSource   = "Source"
	EventColLastSeen = "Last Seen"
)

func FormatAge(d time.Duration) string {
	return formatAge(d)
}

func populateGenericCRDResource(ti *model.Item, status map[string]any) {
	if status == nil {
		return
	}
	for _, key := range []string{"phase", "state", "health", "sync", "message", "reason"} {
		if v, ok := status[key]; ok {
			label := strings.ToUpper(key[:1]) + key[1:]
			switch val := v.(type) {
			case map[string]any:
				for subKey, subVal := range val {
					subLabel := label + " " + strings.ToUpper(subKey[:1]) + subKey[1:]
					ti.Columns = append(ti.Columns, model.KeyValue{Key: subLabel, Value: fmt.Sprintf("%v", subVal)})
				}
			default:
				s := fmt.Sprintf("%v", val)
				if (key == "phase" || key == "state") && s == ti.Status {
					continue
				}
				ti.Columns = append(ti.Columns, model.KeyValue{Key: label, Value: s})
			}
		}
	}

	if conditions, ok := status["conditions"].([]any); ok && len(conditions) > 0 {
		extractGenericConditions(ti, conditions)
	}
}
