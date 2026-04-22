package model

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltInMetadata_EntriesAreWellFormed(t *testing.T) {
	require.NotEmpty(t, BuiltInMetadata)
	for key, meta := range BuiltInMetadata {
		assert.NotEmpty(t, meta.Category, "key=%s missing category", key)
		assert.NotEmpty(t, meta.DisplayName, "key=%s missing display name", key)
		assert.NotEmpty(t, meta.Icon, "key=%s missing icon", key)
		// key format: "group/resource" where group may be empty.
		parts := strings.SplitN(key, "/", 2)
		require.Len(t, parts, 2, "key=%s must contain exactly one '/'", key)
		assert.NotEmpty(t, parts[1], "key=%s missing resource part", key)
	}
}

func TestBuiltInMetadata_CoversCoreK8sResources(t *testing.T) {
	// Minimum set that must be present for the curated sidebar to work.
	required := []string{
		"/pods",
		"/services",
		"/configmaps",
		"/secrets",
		"/namespaces",
		"/nodes",
		"/persistentvolumes",
		"/persistentvolumeclaims",
		"/serviceaccounts",
		"/events",
		"apps/deployments",
		"apps/statefulsets",
		"apps/daemonsets",
		"apps/replicasets",
		"batch/jobs",
		"batch/cronjobs",
		"networking.k8s.io/ingresses",
		"networking.k8s.io/networkpolicies",
		"storage.k8s.io/storageclasses",
		"storage.k8s.io/csidrivers",
		"storage.k8s.io/csinodes",
		"rbac.authorization.k8s.io/roles",
		"rbac.authorization.k8s.io/rolebindings",
		"rbac.authorization.k8s.io/clusterroles",
		"rbac.authorization.k8s.io/clusterrolebindings",
	}
	for _, key := range required {
		_, ok := BuiltInMetadata[key]
		assert.True(t, ok, "BuiltInMetadata must contain %s", key)
	}
}

// TestBuiltInMetadata_CoversGatewayAPI asserts that the full set of
// Gateway API resources (gateway.networking.k8s.io/*) surfaced by LFK
// are present in BuiltInMetadata and carry the Networking category, so
// users can navigate to Gateways, every route kind (HTTP, TLS, TCP,
// gRPC), and the supporting policy/grant resources
// (BackendTLSPolicies, ReferenceGrants) from the curated sidebar.
func TestBuiltInMetadata_CoversGatewayAPI(t *testing.T) {
	required := []string{
		"gateway.networking.k8s.io/gatewayclasses",
		"gateway.networking.k8s.io/gateways",
		"gateway.networking.k8s.io/httproutes",
		"gateway.networking.k8s.io/tlsroutes",
		"gateway.networking.k8s.io/tcproutes",
		"gateway.networking.k8s.io/udproutes",
		"gateway.networking.k8s.io/grpcroutes",
		"gateway.networking.k8s.io/referencegrants",
		"gateway.networking.k8s.io/backendtlspolicies",
	}
	for _, key := range required {
		meta, ok := BuiltInMetadata[key]
		if assert.True(t, ok, "BuiltInMetadata must contain %s", key) {
			assert.Equal(t, "Networking", meta.Category,
				"%s must be in the Networking category", key)
		}
		_, ranked := BuiltInOrderRank[key]
		assert.True(t, ranked, "BuiltInOrderRank must contain %s", key)
	}
}

// TestBuiltInMetadataCatalogIntegrity enforces that every entry in
// BuiltInMetadata has all four curated Icon variants populated: the
// canonical Unicode glyph, the ASCII Simple label, the Emoji glyph,
// and the NerdFont MDI codepoint.
//
// The test also enforces that the (Unicode, Simple, Emoji, NerdFont)
// quadruple is consistent across the catalog: any two entries that
// share the same Unicode glyph must also share the same Simple, Emoji,
// and NerdFont values. This prevents drift between the catalog and the
// reference mapping table.
func TestBuiltInMetadataCatalogIntegrity(t *testing.T) {
	require.NotEmpty(t, BuiltInMetadata)

	// Per-glyph consistency: track the first (Simple, Emoji, NerdFont)
	// triple seen for each Unicode glyph and fail if a later entry
	// disagrees.
	type variants struct {
		simple   string
		emoji    string
		nerdFont string
		first    string // first key seen with this glyph, for error messages
	}
	seen := make(map[string]variants)

	for key, meta := range BuiltInMetadata {
		t.Run(key, func(t *testing.T) {
			assert.NotEmpty(t, meta.Icon.Unicode, "key=%s missing Icon.Unicode", key)
			assert.NotEmpty(t, meta.Icon.Simple, "key=%s missing Icon.Simple", key)
			assert.NotEmpty(t, meta.Icon.Emoji, "key=%s missing Icon.Emoji", key)
			assert.NotEmpty(t, meta.Icon.NerdFont, "key=%s missing Icon.NerdFont", key)
		})

		if meta.Icon.Unicode == "" {
			continue
		}
		if prev, ok := seen[meta.Icon.Unicode]; ok {
			assert.Equal(t, prev.simple, meta.Icon.Simple,
				"key=%s uses Unicode=%q with Simple=%q but %s uses Simple=%q",
				key, meta.Icon.Unicode, meta.Icon.Simple, prev.first, prev.simple)
			assert.Equal(t, prev.emoji, meta.Icon.Emoji,
				"key=%s uses Unicode=%q with Emoji=%q but %s uses Emoji=%q",
				key, meta.Icon.Unicode, meta.Icon.Emoji, prev.first, prev.emoji)
			assert.Equal(t, prev.nerdFont, meta.Icon.NerdFont,
				"key=%s uses Unicode=%q with NerdFont=%q but %s uses NerdFont=%q",
				key, meta.Icon.Unicode, meta.Icon.NerdFont, prev.first, prev.nerdFont)
			continue
		}
		seen[meta.Icon.Unicode] = variants{
			simple:   meta.Icon.Simple,
			emoji:    meta.Icon.Emoji,
			nerdFont: meta.Icon.NerdFont,
			first:    key,
		}
	}
}

// TestBuiltInMetadataUnicodeSingleCell ensures every Unicode glyph
// renders within a single terminal cell. Wide glyphs would misalign
// the sidebar and break narrow-column layouts.
func TestBuiltInMetadataUnicodeSingleCell(t *testing.T) {
	for key, meta := range BuiltInMetadata {
		t.Run(key, func(t *testing.T) {
			if w := lipgloss.Width(meta.Icon.Unicode); w > 1 {
				t.Errorf("%s: Unicode glyph %q has width %d (want <= 1)",
					key, meta.Icon.Unicode, w)
			}
		})
	}
}

// TestBuiltInMetadataNerdFontSingleCell ensures every NerdFont codepoint
// renders within a single terminal cell. A width > 1 typically indicates
// the codepoint string was constructed incorrectly (e.g., \u escape used
// for a 5-hex-digit codepoint, which produces a 2-rune string).
func TestBuiltInMetadataNerdFontSingleCell(t *testing.T) {
	for key, meta := range BuiltInMetadata {
		if meta.Icon.NerdFont == "" {
			continue
		}
		t.Run(key, func(t *testing.T) {
			if w := lipgloss.Width(meta.Icon.NerdFont); w > 1 {
				t.Errorf("%s: NerdFont codepoint %q has width %d (want <= 1)",
					key, meta.Icon.NerdFont, w)
			}
		})
	}
}

// TestBuiltInMetadataSimpleConsistentWidth ensures all ASCII Simple
// labels have the same display width. The sidebar pads icons to a
// fixed column, so inconsistent widths would cause column drift.
func TestBuiltInMetadataSimpleConsistentWidth(t *testing.T) {
	var wantWidth int
	var wantKey string
	for key, meta := range BuiltInMetadata {
		if meta.Icon.Simple == "" {
			continue
		}
		w := lipgloss.Width(meta.Icon.Simple)
		if wantWidth == 0 {
			wantWidth = w
			wantKey = key
			continue
		}
		if w != wantWidth {
			t.Errorf("%s: Simple label %q width %d, want %d (matching %s)",
				key, meta.Icon.Simple, w, wantWidth, wantKey)
		}
	}
}

// allowListedUnicodeCollisions lists (entry key → shared-glyph-group) pairs
// where two or more entries intentionally share the same unicode glyph.
// Adding an entry here makes it exempt from the collision check.
var allowListedUnicodeCollisions = map[string]string{
	// Helm + Flux HelmReleases both deploy helm charts — same concept.
	"_helm/releases":                      "helm-release",
	"helm.toolkit.fluxcd.io/helmreleases": "helm-release",

	// CSI internals share a glyph (rare, internal).
	"storage.k8s.io/csidrivers":           "csi-internal",
	"storage.k8s.io/csinodes":             "csi-internal",
	"storage.k8s.io/csistoragecapacities": "csi-internal",
	"storage.k8s.io/volumeattachments":    "csi-internal",

	// RBAC cluster-scoped siblings share with namespaced versions.
	"rbac.authorization.k8s.io/roles":               "rbac-role",
	"rbac.authorization.k8s.io/clusterroles":        "rbac-role",
	"rbac.authorization.k8s.io/rolebindings":        "rbac-rolebinding",
	"rbac.authorization.k8s.io/clusterrolebindings": "rbac-rolebinding",

	// Cert-manager Issuer + ClusterIssuer share.
	"cert-manager.io/issuers":        "cert-issuer",
	"cert-manager.io/clusterissuers": "cert-issuer",

	// Cert-manager ACME flow shares (CertificateRequests, Orders, Challenges).
	"cert-manager.io/certificaterequests": "cert-acme-flow",
	"acme.cert-manager.io/orders":         "cert-acme-flow",
	"acme.cert-manager.io/challenges":     "cert-acme-flow",

	// Admission + FlowControl rare batch shares.
	"admissionregistration.k8s.io/mutatingwebhookconfigurations":     "admission-flow",
	"admissionregistration.k8s.io/validatingwebhookconfigurations":   "admission-flow",
	"admissionregistration.k8s.io/validatingadmissionpolicies":       "admission-flow",
	"admissionregistration.k8s.io/validatingadmissionpolicybindings": "admission-flow",
	"flowcontrol.apiserver.k8s.io/flowschemas":                       "admission-flow",
	"flowcontrol.apiserver.k8s.io/prioritylevelconfigurations":       "admission-flow",

	// Argo Workflows family shares.
	"argoproj.io/workflows":                "argo-workflow",
	"argoproj.io/workflowtemplates":        "argo-workflow",
	"argoproj.io/clusterworkflowtemplates": "argo-workflow",
	"argoproj.io/cronworkflows":            "argo-workflow",

	// Flux sources share.
	"source.toolkit.fluxcd.io/gitrepositories":  "flux-source",
	"source.toolkit.fluxcd.io/helmrepositories": "flux-source",
	"source.toolkit.fluxcd.io/helmcharts":       "flux-source",
	"source.toolkit.fluxcd.io/ocirepositories":  "flux-source",
	"source.toolkit.fluxcd.io/buckets":          "flux-source",

	// Flux notifications share.
	"notification.toolkit.fluxcd.io/alerts":    "flux-notification",
	"notification.toolkit.fluxcd.io/providers": "flux-notification",
	"notification.toolkit.fluxcd.io/receivers": "flux-notification",

	// Flux images share.
	"image.toolkit.fluxcd.io/imagerepositories":      "flux-image",
	"image.toolkit.fluxcd.io/imagepolicies":          "flux-image",
	"image.toolkit.fluxcd.io/imageupdateautomations": "flux-image",

	// API extensions share NerdFont (nf-md-api) — both are API extension mechanisms.
	"apiregistration.k8s.io/apiservices":             "api-extensions",
	"apiextensions.k8s.io/customresourcedefinitions": "api-extensions",

	// Gateway API currently has distinct glyphs per spec — no allow-list needed.
}

func TestBuiltInMetadataNoCollisions(t *testing.T) {
	// Group all entries (including ecosystem CRDs) by Unicode glyph.
	byUnicode := map[string][]string{}
	for key, meta := range BuiltInMetadata {
		if meta.Icon.Unicode == "" {
			continue
		}
		byUnicode[meta.Icon.Unicode] = append(byUnicode[meta.Icon.Unicode], key)
	}

	// Ecosystem CRDs routinely share the generic CRD glyph "⧫". That's by
	// design — rather than inventing unique icons for every CRD in every
	// ecosystem, the generic fallback is used. Don't flag it.
	const ecosystemFallback = "⧫"

	for glyph, keys := range byUnicode {
		if len(keys) < 2 {
			continue
		}
		if glyph == ecosystemFallback {
			// Generic ecosystem CRD fallback — always allowed.
			continue
		}
		// All members of a shared-glyph group must belong to the same
		// allow-list group.
		groups := map[string]bool{}
		var missing []string
		for _, k := range keys {
			g, ok := allowListedUnicodeCollisions[k]
			if !ok {
				missing = append(missing, k)
				continue
			}
			groups[g] = true
		}
		if len(missing) > 0 {
			t.Errorf("glyph %q is shared by %v but not in allow-list: %v",
				glyph, keys, missing)
		}
		if len(groups) > 1 {
			t.Errorf("glyph %q is shared across allow-list groups %v (keys %v)",
				glyph, groupKeys(groups), keys)
		}
	}
}

// TestBuiltInMetadataNoNerdFontCollisions enforces the same collision
// policy on NerdFont codepoints as TestBuiltInMetadataNoCollisions does
// on Unicode glyphs. The generic CRD fallback codepoint (nf-md-code-tags)
// is exempted because every ecosystem CRD entry deliberately reuses it.
func TestBuiltInMetadataNoNerdFontCollisions(t *testing.T) {
	byNF := map[string][]string{}
	for key, meta := range BuiltInMetadata {
		if meta.Icon.NerdFont == "" {
			continue
		}
		byNF[meta.Icon.NerdFont] = append(byNF[meta.Icon.NerdFont], key)
	}

	// Same ecosystem fallback exception as the Unicode test:
	// nf-md-code-tags is intentionally shared by the curated CRD entry
	// and every ecosystem CRD that uses the generic "⧫" Unicode glyph.
	const ecosystemFallback = "\U000f0174" // nf-md-code-tags

	for cp, keys := range byNF {
		if len(keys) < 2 {
			continue
		}
		if cp == ecosystemFallback {
			continue
		}
		groups := map[string]bool{}
		var missing []string
		for _, k := range keys {
			g, ok := allowListedUnicodeCollisions[k]
			if !ok {
				missing = append(missing, k)
				continue
			}
			groups[g] = true
		}
		if len(missing) > 0 {
			t.Errorf("NerdFont codepoint %q is shared by %v but not in allow-list: %v",
				cp, keys, missing)
		}
		if len(groups) > 1 {
			t.Errorf("NerdFont codepoint %q is shared across allow-list groups %v (keys %v)",
				cp, groupKeys(groups), keys)
		}
	}
}

func groupKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestSeedResources_CoversCoreNavigation(t *testing.T) {
	seed := SeedResources()
	require.NotEmpty(t, seed)

	// Every seed entry must resolve to a BuiltInMetadata entry so
	// BuildSidebarItems produces a visible, categorized sidebar even before
	// discovery completes.
	for _, e := range seed {
		key := e.APIGroup + "/" + e.Resource
		_, ok := BuiltInMetadata[key]
		assert.True(t, ok, "seed entry %s has no BuiltInMetadata", key)
	}

	// Minimum set: the sidebar must be usable for day-to-day navigation
	// before discovery finishes.
	kinds := make(map[string]bool, len(seed))
	for _, e := range seed {
		kinds[e.Kind] = true
	}
	for _, must := range []string{"Pod", "Deployment", "Service", "ConfigMap", "Secret", "Namespace", "Node"} {
		assert.True(t, kinds[must], "seed missing %s", must)
	}
}
