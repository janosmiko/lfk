package k8s

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/janosmiko/lfk/internal/logger"
)

// pathPrefixFlags are the flags the Prometheus operator and the
// VictoriaMetrics operator use to move the query API below a URL prefix.
var pathPrefixFlags = []string{"--web.route-prefix", "-http.pathPrefix", "--http.pathPrefix"}

// discoveredPathPrefix reads the URL prefix off the pods a Service selects.
// It returns "" when the Service has no selector, no pod carries a prefix
// flag, or the list is denied.
func discoveredPathPrefix(ctx context.Context, cs kubernetes.Interface, svc *corev1.Service) string {
	if len(svc.Spec.Selector) == 0 {
		return ""
	}
	pods, err := cs.CoreV1().Pods(svc.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(svc.Spec.Selector).String(),
	})
	if err != nil {
		logger.Debug("path prefix discovery failed", "namespace", svc.Namespace, "service", svc.Name, "error", logger.Redact(err.Error()))
		return ""
	}
	for i := range pods.Items {
		for _, c := range pods.Items[i].Spec.Containers {
			if p := pathPrefixFromArgs(append(append([]string{}, c.Command...), c.Args...)); p != "" {
				return p
			}
		}
	}
	return ""
}

// pathPrefixFromArgs finds a prefix flag in a container's arguments, in
// either --flag=value or --flag value form, and normalises it to /name.
func pathPrefixFromArgs(args []string) string {
	for i, arg := range args {
		for _, flag := range pathPrefixFlags {
			var value string
			switch {
			case strings.HasPrefix(arg, flag+"="):
				value = strings.TrimPrefix(arg, flag+"=")
			case arg == flag && i+1 < len(args):
				value = args[i+1]
			default:
				continue
			}
			value = strings.Trim(value, "/")
			if value == "" {
				return ""
			}
			return "/" + value
		}
	}
	return ""
}
