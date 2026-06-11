// Missing-reference check: pods whose spec references a ConfigMap or Secret
// that does not exist. Such pods run until their next restart, then fail
// with CreateContainerConfigError — catching it while they still run is the
// whole point. References marked optional are skipped, as are reference
// kinds whose backing list failed (or was disabled) — absence can only be
// asserted against a complete list.

package heuristic

import (
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// refCollector accumulates unresolved ConfigMap/Secret references for one
// pod. Each kind is only consulted when its list loaded completely.
type refCollector struct {
	ns          string
	cmNames     map[string]bool
	cmOK        bool
	secretNames map[string]bool
	secretsOK   bool
	missing     map[string]bool
}

func (rc *refCollector) addCM(name string, optional *bool) {
	if rc.cmOK && name != "" && (optional == nil || !*optional) && !rc.cmNames[rc.ns+"/"+name] {
		rc.missing["ConfigMap "+name] = true
	}
}

func (rc *refCollector) addSecret(name string, optional *bool) {
	if rc.secretsOK && name != "" && (optional == nil || !*optional) && !rc.secretNames[rc.ns+"/"+name] {
		rc.missing["Secret "+name] = true
	}
}

func (rc *refCollector) scanVolumes(volumes []corev1.Volume) {
	for i := range volumes {
		v := &volumes[i]
		if v.ConfigMap != nil {
			rc.addCM(v.ConfigMap.Name, v.ConfigMap.Optional)
		}
		if v.Secret != nil {
			rc.addSecret(v.Secret.SecretName, v.Secret.Optional)
		}
		if v.Projected != nil {
			for j := range v.Projected.Sources {
				src := &v.Projected.Sources[j]
				if src.ConfigMap != nil {
					rc.addCM(src.ConfigMap.Name, src.ConfigMap.Optional)
				}
				if src.Secret != nil {
					rc.addSecret(src.Secret.Name, src.Secret.Optional)
				}
			}
		}
	}
}

func (rc *refCollector) scanContainer(c *corev1.Container) {
	for i := range c.EnvFrom {
		ef := &c.EnvFrom[i]
		if ef.ConfigMapRef != nil {
			rc.addCM(ef.ConfigMapRef.Name, ef.ConfigMapRef.Optional)
		}
		if ef.SecretRef != nil {
			rc.addSecret(ef.SecretRef.Name, ef.SecretRef.Optional)
		}
	}
	for i := range c.Env {
		vf := c.Env[i].ValueFrom
		if vf == nil {
			continue
		}
		if vf.ConfigMapKeyRef != nil {
			rc.addCM(vf.ConfigMapKeyRef.Name, vf.ConfigMapKeyRef.Optional)
		}
		if vf.SecretKeyRef != nil {
			rc.addSecret(vf.SecretKeyRef.Name, vf.SecretKeyRef.Optional)
		}
	}
}

// checkMissingRefs returns at most one finding per pod, listing every
// non-optional ConfigMap/Secret reference that resolves to nothing.
// Emitted as CategoryReliability — like bare_pod, it is an operational
// problem, not a hardening gap, and stays off the SEC badge.
func checkMissingRefs(pod *corev1.Pod, cmNames map[string]bool, cmOK bool, secretNames map[string]bool, secretsOK bool) []security.Finding {
	if !cmOK && !secretsOK {
		return nil
	}
	rc := &refCollector{
		ns:          pod.Namespace,
		cmNames:     cmNames,
		cmOK:        cmOK,
		secretNames: secretNames,
		secretsOK:   secretsOK,
		missing:     map[string]bool{},
	}
	rc.scanVolumes(pod.Spec.Volumes)
	for i := range pod.Spec.InitContainers {
		rc.scanContainer(&pod.Spec.InitContainers[i])
	}
	for i := range pod.Spec.Containers {
		rc.scanContainer(&pod.Spec.Containers[i])
	}
	for i := range pod.Spec.EphemeralContainers {
		c := corev1.Container(pod.Spec.EphemeralContainers[i].EphemeralContainerCommon)
		rc.scanContainer(&c)
	}
	if len(rc.missing) == 0 {
		return nil
	}
	names := make([]string, 0, len(rc.missing))
	for n := range rc.missing {
		names = append(names, n)
	}
	slices.Sort(names)
	verb := "does"
	if len(names) > 1 {
		verb = "do"
	}
	f := security.Finding{
		ID:       fmt.Sprintf("heuristic/%s/Pod/%s/missing_config_ref", pod.Namespace, pod.Name),
		Source:   "heuristic",
		Category: security.CategoryReliability,
		Severity: security.SeverityHigh,
		Title:    "references missing ConfigMap/Secret",
		Resource: security.ResourceRef{Namespace: pod.Namespace, Kind: "Pod", Name: pod.Name},
		Summary:  fmt.Sprintf("Pod references %s, which %s not exist; the pod runs now but fails with CreateContainerConfigError on its next restart.", strings.Join(names, ", "), verb),
		Labels:   map[string]string{"check": "missing_config_ref"},
	}
	return []security.Finding{f}
}
