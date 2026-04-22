package k8s

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// convertAPIResourceLists walks the output of ServerPreferredResources and
// returns one ResourceTypeEntry per non-subresource API resource. Subresources
// (names containing '/', e.g. "pods/log") are skipped. Review APIs like
// tokenreviews are kept so they remain resolvable even though they are not
// displayed in the sidebar.
func convertAPIResourceLists(lists []*metav1.APIResourceList) []model.ResourceTypeEntry {
	entries := make([]model.ResourceTypeEntry, 0, len(lists)*8)
	for _, list := range lists {
		if list == nil {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			logger.Warn("Discovery: unparseable GroupVersion", "groupVersion", list.GroupVersion, "error", err)
			continue
		}
		for _, r := range list.APIResources {
			if strings.Contains(r.Name, "/") {
				continue // subresource
			}
			entries = append(entries, model.ResourceTypeEntry{
				Kind:       r.Kind,
				APIGroup:   gv.Group,
				APIVersion: gv.Version,
				Resource:   r.Name,
				Namespaced: r.Namespaced,
				Verbs:      append([]string(nil), r.Verbs...),
			})
		}
	}
	return entries
}

// DiscoverAPIResources returns every API resource the cluster serves for the
// given context, including core Kubernetes resources and installed CRDs. Each
// resource carries its group, version, plural name, kind, and namespaced
// scope as reported by the discovery API. Subresources are filtered out.
//
// PrinterColumns for CRDs are populated by a secondary pass that reads the
// CustomResourceDefinition objects directly; core resources have empty
// PrinterColumns because the discovery API does not expose them.
//
// Partial discovery errors (one API group failing to resolve while others
// succeed) are logged and the successful subset is returned. A complete
// failure to reach the discovery endpoint returns an error.
func (c *Client) DiscoverAPIResources(ctx context.Context, contextName string) ([]model.ResourceTypeEntry, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, fmt.Errorf("getting clientset: %w", err)
	}
	dc := cs.Discovery()

	lists, err := discovery.ServerPreferredResources(dc)
	if err != nil {
		var gdf *discovery.ErrGroupDiscoveryFailed
		if errors.As(err, &gdf) {
			for gv, gerr := range gdf.Groups {
				logger.Warn("Discovery: group failed", "groupVersion", gv.String(), "error", gerr)
			}
			// fall through with whatever lists we got
		} else if apierrors.IsForbidden(err) {
			logger.Warn("Discovery: forbidden on discovery endpoint, returning empty result", "error", err)
			return nil, nil
		} else {
			return nil, fmt.Errorf("server preferred resources: %w", err)
		}
	}

	entries := convertAPIResourceLists(lists)

	// Secondary pass: fetch printer columns from the CRD spec objects and
	// merge them into matching entries by group/resource key.
	if printer, perr := c.fetchCRDPrinterColumns(ctx, contextName); perr != nil {
		logger.Warn("Discovery: printer column fetch failed", "error", perr)
	} else {
		for i := range entries {
			key := entries[i].APIGroup + "/" + entries[i].Resource
			if cols, ok := printer[key]; ok && len(cols) > 0 {
				entries[i].PrinterColumns = cols
			}
		}
	}

	// Apply deprecation metadata (reuses existing helper from deprecations.go).
	for i := range entries {
		if dep, found := CheckDeprecation(entries[i].APIGroup, entries[i].APIVersion, entries[i].Resource); found {
			entries[i].Deprecated = true
			entries[i].DeprecationMsg = dep.Message
		}
	}

	// Stable sort: group, then kind.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].APIGroup != entries[j].APIGroup {
			return entries[i].APIGroup < entries[j].APIGroup
		}
		return entries[i].Kind < entries[j].Kind
	})
	return entries, nil
}

// fetchCRDPrinterColumns returns a map from "group/resource" to the
// additionalPrinterColumns declared in the CRD spec for the preferred
// version. It reuses the existing preferredCRDVersion and
// extractCRDPrinterColumns helpers from client_crd.go.
func (c *Client) fetchCRDPrinterColumns(ctx context.Context, contextName string) (map[string][]model.PrinterColumn, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, fmt.Errorf("getting dynamic client: %w", err)
	}
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
	list, err := dynClient.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing CRDs: %w", err)
	}

	out := make(map[string][]model.PrinterColumn, len(list.Items))
	for _, item := range list.Items {
		spec, ok := item.Object["spec"].(map[string]interface{})
		if !ok {
			continue
		}
		group, _ := spec["group"].(string)
		names, ok := spec["names"].(map[string]interface{})
		if !ok {
			continue
		}
		plural, _ := names["plural"].(string)
		if plural == "" {
			continue
		}
		version := preferredCRDVersion(spec, item.Object)
		cols := extractCRDPrinterColumns(spec, version)
		if len(cols) > 0 {
			out[group+"/"+plural] = cols
		}
	}
	return out, nil
}
