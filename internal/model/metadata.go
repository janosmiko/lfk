package model

// CoreCategories lists the navigation categories that are built-in to LFK
// and always appear in a fixed order at the top of the sidebar. They cannot
// be pinned or reordered by the user.
var CoreCategories = []string{
	"Dashboards",
	"Cluster",
	"Workloads",
	"Config",
	"Networking",
	"Storage",
	"Access Control",
	"Helm",
	"API and CRDs",
	"Advanced", // surfaced only when ShowRareResources is true
}

// IsCoreCategory reports whether name is one of the fixed CoreCategories.
func IsCoreCategory(name string) bool {
	for _, c := range CoreCategories {
		if c == name {
			return true
		}
	}
	return false
}

// KnownResourceNames returns the set of lowercase plural resource names
// known to LFK via BuiltInMetadata. Used by command bar parsers and
// completion when a discovered resource set is not available at the call
// site. CRDs not in BuiltInMetadata are handled at runtime via the
// discovered slice passed through higher-level helpers.
func KnownResourceNames() map[string]bool {
	out := make(map[string]bool, len(BuiltInMetadata))
	for key := range BuiltInMetadata {
		// key format: "group/resource" — take the part after the slash.
		for i := len(key) - 1; i >= 0; i-- {
			if key[i] == '/' {
				out[key[i+1:]] = true
				break
			}
		}
	}
	return out
}

// DisplayMetadata describes how a discovered API resource should appear in
// the sidebar. It carries display intent only — GVR, version, and scope come
// from runtime discovery and live on ResourceTypeEntry.
type DisplayMetadata struct {
	Category    string // e.g., "Workloads", "Storage"
	DisplayName string // e.g., "Deployments"
	Icon        Icon   // Icon variants (see icon.go)
	Rare        bool   // if true, hidden from the sidebar unless ShowRareResources is toggled on
}

// ShowRareResources, when true, causes BuildSidebarItems to surface
// BuiltInMetadata entries marked Rare=true as well as uncategorized core
// Kubernetes resources that are otherwise hidden. The app toggles this via
// a keybinding. It is a package-level var (like PinnedGroups) so
// BuildSidebarItems can read it without a signature change.
var ShowRareResources bool

// AdvancedCategory is the category name used for uncategorized core
// Kubernetes resources (TokenReview, Binding, ComponentStatus, etc.) when
// ShowRareResources is true. It is appended to CoreCategories so the
// section appears at the end of the core sidebar area.
const AdvancedCategory = "Advanced"

// CoreK8sGroups lists the API groups that belong to the core Kubernetes API
// surface. Discovered resources whose group matches this set AND whose
// group/resource key is absent from BuiltInMetadata are intentionally hidden
// from the sidebar (they are obscure built-ins like Binding, TokenReview,
// ComponentStatus that clutter navigation). Discovered resources outside
// this set are treated as CRDs and grouped under their API group.
var CoreK8sGroups = map[string]bool{
	"":                             true, // core/v1
	"apps":                         true,
	"batch":                        true,
	"autoscaling":                  true,
	"policy":                       true,
	"networking.k8s.io":            true,
	"rbac.authorization.k8s.io":    true,
	"storage.k8s.io":               true,
	"storagemigration.k8s.io":      true,
	"coordination.k8s.io":          true,
	"discovery.k8s.io":             true,
	"scheduling.k8s.io":            true,
	"node.k8s.io":                  true,
	"events.k8s.io":                true,
	"certificates.k8s.io":          true,
	"admissionregistration.k8s.io": true,
	"apiregistration.k8s.io":       true,
	"apiextensions.k8s.io":         true,
	"flowcontrol.apiserver.k8s.io": true,
	"authorization.k8s.io":         true,
	"authentication.k8s.io":        true,
	"internal.apiserver.k8s.io":    true,
	"resource.k8s.io":              true,
	"gateway.networking.k8s.io":    true,
}

// GroupCategoryFallback places discovered resources from specific core
// API groups into a predetermined sidebar category when they are not in
// BuiltInMetadata. This is the safety net for upstream additions: a new
// Kubernetes or Gateway API resource will surface in its natural
// category (with the generic CRD glyph) instead of being hidden, even
// before someone adds a curated BuiltInMetadata entry. Consulted by
// partitionDiscovered after the BuiltInMetadata lookup fails.
var GroupCategoryFallback = map[string]string{
	"networking.k8s.io":         "Networking",
	"gateway.networking.k8s.io": "Networking",
}

// GroupFallbackRank gives auto-categorized items a sort rank so they
// slot into a predictable position within their category. All items
// sharing a group use the same rank; ties fall back to alphabetical
// by display name. Consulted by itemOrderRank when a key-level lookup
// in BuiltInOrderRank misses.
var GroupFallbackRank = map[string]int{
	"networking.k8s.io":         65, // after curated Networking items, before port-forwards
	"gateway.networking.k8s.io": 65,
}

// BuiltInMetadata maps "group/resource" to display metadata for the curated
// sidebar. The key format is "<group>/<resource>" with an empty group for
// core/v1 resources (e.g., "/pods", "/services"). Resources the cluster
// serves but absent from this map fall through to GroupCategoryFallback
// (if the group is listed there), then to the CoreK8sGroups hide path,
// and finally to generic CRD entries under their API group.
//
// This map is hand-maintained. Adding an entry here makes the corresponding
// resource appear in the sidebar with the given category and icon whenever
// the cluster serves it.
var BuiltInMetadata = map[string]DisplayMetadata{
	// ---- Cluster ----
	"/nodes":      {Category: "Cluster", DisplayName: "Nodes", Icon: Icon{Unicode: "⌹", Simple: "[No]", Emoji: "🖥️", NerdFont: "\U000f048b"}},
	"/namespaces": {Category: "Cluster", DisplayName: "Namespaces", Icon: Icon{Unicode: "❐", Simple: "[NS]", Emoji: "📂", NerdFont: "\U000f03d7"}},
	"/events":     {Category: "Cluster", DisplayName: "Events", Icon: Icon{Unicode: "⌁", Simple: "[Ev]", Emoji: "🔊", NerdFont: "\U000f009c"}},

	// ---- Workloads ----
	"/pods":             {Category: "Workloads", DisplayName: "Pods", Icon: Icon{Unicode: "□", Simple: "[Po]", Emoji: "📦", NerdFont: "\U000f01a7"}},
	"apps/deployments":  {Category: "Workloads", DisplayName: "Deployments", Icon: Icon{Unicode: "■", Simple: "[De]", Emoji: "🚀", NerdFont: "\U000f01a6"}},
	"apps/replicasets":  {Category: "Workloads", DisplayName: "ReplicaSets", Icon: Icon{Unicode: "⧉", Simple: "[RS]", Emoji: "📚", NerdFont: "\U000f0f58"}},
	"apps/statefulsets": {Category: "Workloads", DisplayName: "StatefulSets", Icon: Icon{Unicode: "▥", Simple: "[SS]", Emoji: "🗄️", NerdFont: "\U000f01bc"}},
	"apps/daemonsets":   {Category: "Workloads", DisplayName: "DaemonSets", Icon: Icon{Unicode: "●", Simple: "[DS]", Emoji: "🧱", NerdFont: "\U000f04aa"}},
	"batch/jobs":        {Category: "Workloads", DisplayName: "Jobs", Icon: Icon{Unicode: "▶", Simple: "[Jo]", Emoji: "▶️", NerdFont: "\U000f040a"}},
	"batch/cronjobs":    {Category: "Workloads", DisplayName: "CronJobs", Icon: Icon{Unicode: "⟳", Simple: "[CJ]", Emoji: "⏰️", NerdFont: "\U000f00f0"}},

	// ---- Config ----
	"/configmaps":                          {Category: "Config", DisplayName: "ConfigMaps", Icon: Icon{Unicode: "≡", Simple: "[CM]", Emoji: "📋", NerdFont: "\U000f107c"}},
	"/secrets":                             {Category: "Config", DisplayName: "Secrets", Icon: Icon{Unicode: "⊡", Simple: "[Se]", Emoji: "🔒", NerdFont: "\U000f0221"}},
	"/resourcequotas":                      {Category: "Config", DisplayName: "ResourceQuotas", Icon: Icon{Unicode: "⚖", Simple: "[RQ]", Emoji: "⚖️", NerdFont: "\U000f05d1"}},
	"/limitranges":                         {Category: "Config", DisplayName: "LimitRanges", Icon: Icon{Unicode: "⎍", Simple: "[LR]", Emoji: "📏", NerdFont: "\U000f046d"}},
	"autoscaling/horizontalpodautoscalers": {Category: "Config", DisplayName: "HPA", Icon: Icon{Unicode: "⇔", Simple: "[HP]", Emoji: "↔️", NerdFont: "\U000f084e"}},
	"autoscaling.k8s.io/verticalpodautoscalers":                      {Category: "Config", DisplayName: "VPA", Icon: Icon{Unicode: "⇕", Simple: "[VP]", Emoji: "↕️", NerdFont: "\U000f084f"}},
	"policy/poddisruptionbudgets":                                    {Category: "Config", DisplayName: "PodDisruptionBudgets", Icon: Icon{Unicode: "⊘", Simple: "[PD]", Emoji: "🛡️", NerdFont: "\U000f0565"}},
	"scheduling.k8s.io/priorityclasses":                              {Category: "Config", DisplayName: "PriorityClasses", Icon: Icon{Unicode: "⇑", Simple: "[PC]", Emoji: "⬆️", NerdFont: "\U000f0603"}},
	"node.k8s.io/runtimeclasses":                                     {Category: "Config", DisplayName: "RuntimeClasses", Icon: Icon{Unicode: "⊚", Simple: "[RC]", Emoji: "⚙️", NerdFont: "\U000f08bb"}, Rare: true},
	"coordination.k8s.io/leases":                                     {Category: "Config", DisplayName: "Leases", Icon: Icon{Unicode: "⏱", Simple: "[Le]", Emoji: "⏱️", NerdFont: "\U000f051b"}, Rare: true},
	"admissionregistration.k8s.io/mutatingwebhookconfigurations":     {Category: "Config", DisplayName: "MutatingWebhookConfigurations", Icon: Icon{Unicode: "⚙", Simple: "[Wh]", Emoji: "🔧", NerdFont: "\U000f0494"}, Rare: true},
	"admissionregistration.k8s.io/validatingwebhookconfigurations":   {Category: "Config", DisplayName: "ValidatingWebhookConfigurations", Icon: Icon{Unicode: "⚙", Simple: "[Wh]", Emoji: "🔧", NerdFont: "\U000f0494"}, Rare: true},
	"admissionregistration.k8s.io/validatingadmissionpolicies":       {Category: "Config", DisplayName: "ValidatingAdmissionPolicies", Icon: Icon{Unicode: "⚙", Simple: "[Wh]", Emoji: "🔧", NerdFont: "\U000f0494"}, Rare: true},
	"admissionregistration.k8s.io/validatingadmissionpolicybindings": {Category: "Config", DisplayName: "ValidatingAdmissionPolicyBindings", Icon: Icon{Unicode: "⚙", Simple: "[Wh]", Emoji: "🔧", NerdFont: "\U000f0494"}, Rare: true},
	"flowcontrol.apiserver.k8s.io/flowschemas":                       {Category: "Config", DisplayName: "FlowSchemas", Icon: Icon{Unicode: "⚙", Simple: "[Wh]", Emoji: "🔧", NerdFont: "\U000f0494"}, Rare: true},
	"flowcontrol.apiserver.k8s.io/prioritylevelconfigurations":       {Category: "Config", DisplayName: "PriorityLevelConfigurations", Icon: Icon{Unicode: "⚙", Simple: "[Wh]", Emoji: "🔧", NerdFont: "\U000f0494"}, Rare: true},

	// ---- Networking ----
	"/services":                                    {Category: "Networking", DisplayName: "Services", Icon: Icon{Unicode: "⇌", Simple: "[Sv]", Emoji: "🔀", NerdFont: "\U000f04e1"}},
	"/endpoints":                                   {Category: "Networking", DisplayName: "Endpoints", Icon: Icon{Unicode: "→", Simple: "[EP]", Emoji: "➡️", NerdFont: "\U000f096d"}},
	"networking.k8s.io/ingresses":                  {Category: "Networking", DisplayName: "Ingresses", Icon: Icon{Unicode: "↳", Simple: "[In]", Emoji: "🌐", NerdFont: "\U000f059f"}},
	"networking.k8s.io/networkpolicies":            {Category: "Networking", DisplayName: "NetworkPolicies", Icon: Icon{Unicode: "⛊", Simple: "[NP]", Emoji: "🛡️", NerdFont: "\U000f0483"}},
	"networking.k8s.io/ingressclasses":             {Category: "Networking", DisplayName: "IngressClasses", Icon: Icon{Unicode: "⏂", Simple: "[IC]", Emoji: "🔖", NerdFont: "\U000f0832"}},
	"discovery.k8s.io/endpointslices":              {Category: "Networking", DisplayName: "EndpointSlices", Icon: Icon{Unicode: "⇶", Simple: "[Es]", Emoji: "⏭️", NerdFont: "\U000f0dbb"}},
	"gateway.networking.k8s.io/gatewayclasses":     {Category: "Networking", DisplayName: "GatewayClasses", Icon: Icon{Unicode: "⎁", Simple: "[GC]", Emoji: "🚪", NerdFont: "\U000f0299"}},
	"gateway.networking.k8s.io/gateways":           {Category: "Networking", DisplayName: "Gateways", Icon: Icon{Unicode: "⎇", Simple: "[Ga]", Emoji: "⛩️", NerdFont: "\U000f0293"}},
	"gateway.networking.k8s.io/httproutes":         {Category: "Networking", DisplayName: "HTTPRoutes", Icon: Icon{Unicode: "⟿", Simple: "[HR]", Emoji: "🛣️", NerdFont: "\U000f046a"}},
	"gateway.networking.k8s.io/tlsroutes":          {Category: "Networking", DisplayName: "TLSRoutes", Icon: Icon{Unicode: "⇆", Simple: "[TR]", Emoji: "🔐", NerdFont: "\U000f0341"}},
	"gateway.networking.k8s.io/tcproutes":          {Category: "Networking", DisplayName: "TCPRoutes", Icon: Icon{Unicode: "⤳", Simple: "[TC]", Emoji: "🧵", NerdFont: "\U000f0c46"}},
	"gateway.networking.k8s.io/udproutes":          {Category: "Networking", DisplayName: "UDPRoutes", Icon: Icon{Unicode: "⇢", Simple: "[UD]", Emoji: "📡", NerdFont: "\U000f05ab"}},
	"gateway.networking.k8s.io/grpcroutes":         {Category: "Networking", DisplayName: "GRPCRoutes", Icon: Icon{Unicode: "⤒", Simple: "[GR]", Emoji: "🔩", NerdFont: "\U000f0295"}},
	"gateway.networking.k8s.io/referencegrants":    {Category: "Networking", DisplayName: "ReferenceGrants", Icon: Icon{Unicode: "⊸", Simple: "[Rg]", Emoji: "🤝", NerdFont: "\U000f1218"}},
	"gateway.networking.k8s.io/backendtlspolicies": {Category: "Networking", DisplayName: "BackendTLSPolicies", Icon: Icon{Unicode: "⊠", Simple: "[BT]", Emoji: "🛡️", NerdFont: "\U000f0bb6"}},

	// ---- Storage ----
	"/persistentvolumeclaims":             {Category: "Storage", DisplayName: "PersistentVolumeClaims", Icon: Icon{Unicode: "⊞", Simple: "[PV]", Emoji: "💽", NerdFont: "\U000f104b"}},
	"/persistentvolumes":                  {Category: "Storage", DisplayName: "PersistentVolumes", Icon: Icon{Unicode: "⬚", Simple: "[Pv]", Emoji: "💿", NerdFont: "\U000f02ca"}},
	"storage.k8s.io/storageclasses":       {Category: "Storage", DisplayName: "StorageClasses", Icon: Icon{Unicode: "▧", Simple: "[SC]", Emoji: "💾", NerdFont: "\U000f12f7"}},
	"storage.k8s.io/csidrivers":           {Category: "Storage", DisplayName: "CSIDrivers", Icon: Icon{Unicode: "▤", Simple: "[Cs]", Emoji: "🧩", NerdFont: "\U000f0431"}, Rare: true},
	"storage.k8s.io/csinodes":             {Category: "Storage", DisplayName: "CSINodes", Icon: Icon{Unicode: "▤", Simple: "[Cs]", Emoji: "🧩", NerdFont: "\U000f0431"}, Rare: true},
	"storage.k8s.io/csistoragecapacities": {Category: "Storage", DisplayName: "CSIStorageCapacities", Icon: Icon{Unicode: "▤", Simple: "[Cs]", Emoji: "🧩", NerdFont: "\U000f0431"}, Rare: true},
	"storage.k8s.io/volumeattachments":    {Category: "Storage", DisplayName: "VolumeAttachments", Icon: Icon{Unicode: "▤", Simple: "[Cs]", Emoji: "🧩", NerdFont: "\U000f0431"}, Rare: true},

	// ---- Access Control ----
	"/serviceaccounts":                              {Category: "Access Control", DisplayName: "ServiceAccounts", Icon: Icon{Unicode: "⚇", Simple: "[SA]", Emoji: "👤", NerdFont: "\U000f0004"}},
	"rbac.authorization.k8s.io/roles":               {Category: "Access Control", DisplayName: "Roles", Icon: Icon{Unicode: "⚿", Simple: "[Ro]", Emoji: "🔑", NerdFont: "\U000f0be4"}},
	"rbac.authorization.k8s.io/rolebindings":        {Category: "Access Control", DisplayName: "RoleBindings", Icon: Icon{Unicode: "⊷", Simple: "[Rb]", Emoji: "🔗", NerdFont: "\U000f0339"}},
	"rbac.authorization.k8s.io/clusterroles":        {Category: "Access Control", DisplayName: "ClusterRoles", Icon: Icon{Unicode: "⚿", Simple: "[Ro]", Emoji: "🔑", NerdFont: "\U000f0be4"}},
	"rbac.authorization.k8s.io/clusterrolebindings": {Category: "Access Control", DisplayName: "ClusterRoleBindings", Icon: Icon{Unicode: "⊷", Simple: "[Rb]", Emoji: "🔗", NerdFont: "\U000f0339"}},

	// ---- API and CRDs ----
	"apiregistration.k8s.io/apiservices":             {Category: "API and CRDs", DisplayName: "API Services", Icon: Icon{Unicode: "⟐", Simple: "[AS]", Emoji: "🔌", NerdFont: "\U000f109b"}, Rare: true},
	"apiextensions.k8s.io/customresourcedefinitions": {Category: "API and CRDs", DisplayName: "Custom Resource Definitions", Icon: Icon{Unicode: "◆", Simple: "[CR]", Emoji: "💻", NerdFont: "\U000f109b"}},

	// ---- LFK pseudo-resources ----
	// These are not served by any Kubernetes cluster. They are injected into
	// the discovered resource set so the sidebar, command bar, and resolver
	// can treat them uniformly with real resources. The "_helm" and
	// "_portforward" API groups are LFK-only sentinels that GetResources /
	// GetResourceYAML route to helm and port-forward handlers.
	"_helm/releases":            {Category: "Helm", DisplayName: "Releases", Icon: Icon{Unicode: "⎈", Simple: "[He]", Emoji: "⛵", NerdFont: "\U000f0833"}},
	"_portforward/portforwards": {Category: "Networking", DisplayName: "Port Forwards", Icon: Icon{Unicode: "⇵", Simple: "[PF]", Emoji: "🚇", NerdFont: "\U000f07e5"}},

	// ---- Ecosystem CRDs (ported from TopLevelResourceTypes) ----
	// argoproj.io
	"argoproj.io/applications":             {Category: "argoproj.io", DisplayName: "Applications", Icon: Icon{Unicode: "⎋", Simple: "[Ap]", Emoji: "🔄", NerdFont: "\U000f04e6"}},
	"argoproj.io/applicationsets":          {Category: "argoproj.io", DisplayName: "ApplicationSets", Icon: Icon{Unicode: "⨄", Simple: "[As]", Emoji: "🏭", NerdFont: "\U000f020f"}},
	"argoproj.io/appprojects":              {Category: "argoproj.io", DisplayName: "AppProjects", Icon: Icon{Unicode: "▱", Simple: "[Aj]", Emoji: "📁", NerdFont: "\U000f0b9f"}},
	"argoproj.io/workflows":                {Category: "argoproj.io", DisplayName: "Workflows", Icon: Icon{Unicode: "⫸", Simple: "[Wf]", Emoji: "🕸️", NerdFont: "\U000f104a"}},
	"argoproj.io/workflowtemplates":        {Category: "argoproj.io", DisplayName: "WorkflowTemplates", Icon: Icon{Unicode: "⫸", Simple: "[Wf]", Emoji: "🕸️", NerdFont: "\U000f104a"}},
	"argoproj.io/clusterworkflowtemplates": {Category: "argoproj.io", DisplayName: "ClusterWorkflowTemplates", Icon: Icon{Unicode: "⫸", Simple: "[Wf]", Emoji: "🕸️", NerdFont: "\U000f104a"}},
	"argoproj.io/cronworkflows":            {Category: "argoproj.io", DisplayName: "CronWorkflows", Icon: Icon{Unicode: "⫸", Simple: "[Wf]", Emoji: "🕸️", NerdFont: "\U000f104a"}},

	// Flux CD
	"kustomize.toolkit.fluxcd.io/kustomizations":     {Category: "kustomize.toolkit.fluxcd.io", DisplayName: "Kustomizations", Icon: Icon{Unicode: "⋈", Simple: "[Ks]", Emoji: "✅", NerdFont: "\U000f012d"}},
	"helm.toolkit.fluxcd.io/helmreleases":            {Category: "helm.toolkit.fluxcd.io", DisplayName: "HelmReleases", Icon: Icon{Unicode: "⎈", Simple: "[He]", Emoji: "⛵", NerdFont: "\U000f0833"}},
	"source.toolkit.fluxcd.io/gitrepositories":       {Category: "source.toolkit.fluxcd.io", DisplayName: "GitRepositories", Icon: Icon{Unicode: "⇤", Simple: "[Fs]", Emoji: "📥", NerdFont: "\U000f0ccf"}},
	"source.toolkit.fluxcd.io/helmrepositories":      {Category: "source.toolkit.fluxcd.io", DisplayName: "HelmRepositories", Icon: Icon{Unicode: "⇤", Simple: "[Fs]", Emoji: "📥", NerdFont: "\U000f0ccf"}},
	"source.toolkit.fluxcd.io/helmcharts":            {Category: "source.toolkit.fluxcd.io", DisplayName: "HelmCharts", Icon: Icon{Unicode: "⇤", Simple: "[Fs]", Emoji: "📥", NerdFont: "\U000f0ccf"}},
	"source.toolkit.fluxcd.io/ocirepositories":       {Category: "source.toolkit.fluxcd.io", DisplayName: "OCIRepositories", Icon: Icon{Unicode: "⇤", Simple: "[Fs]", Emoji: "📥", NerdFont: "\U000f0ccf"}},
	"source.toolkit.fluxcd.io/buckets":               {Category: "source.toolkit.fluxcd.io", DisplayName: "Buckets", Icon: Icon{Unicode: "⇤", Simple: "[Fs]", Emoji: "📥", NerdFont: "\U000f0ccf"}},
	"notification.toolkit.fluxcd.io/alerts":          {Category: "notification.toolkit.fluxcd.io", DisplayName: "Alerts", Icon: Icon{Unicode: "⚐", Simple: "[Fn]", Emoji: "💬", NerdFont: "\U000f036a"}},
	"notification.toolkit.fluxcd.io/providers":       {Category: "notification.toolkit.fluxcd.io", DisplayName: "Providers", Icon: Icon{Unicode: "⚐", Simple: "[Fn]", Emoji: "💬", NerdFont: "\U000f036a"}},
	"notification.toolkit.fluxcd.io/receivers":       {Category: "notification.toolkit.fluxcd.io", DisplayName: "Receivers", Icon: Icon{Unicode: "⚐", Simple: "[Fn]", Emoji: "💬", NerdFont: "\U000f036a"}},
	"image.toolkit.fluxcd.io/imagerepositories":      {Category: "image.toolkit.fluxcd.io", DisplayName: "ImageRepositories", Icon: Icon{Unicode: "⊟", Simple: "[Fi]", Emoji: "🐳", NerdFont: "\U000f0868"}},
	"image.toolkit.fluxcd.io/imagepolicies":          {Category: "image.toolkit.fluxcd.io", DisplayName: "ImagePolicies", Icon: Icon{Unicode: "⊟", Simple: "[Fi]", Emoji: "🐳", NerdFont: "\U000f0868"}},
	"image.toolkit.fluxcd.io/imageupdateautomations": {Category: "image.toolkit.fluxcd.io", DisplayName: "ImageUpdateAutomations", Icon: Icon{Unicode: "⊟", Simple: "[Fi]", Emoji: "🐳", NerdFont: "\U000f0868"}},

	// cert-manager.io
	"cert-manager.io/certificates":        {Category: "cert-manager.io", DisplayName: "Certificates", Icon: Icon{Unicode: "⍟", Simple: "[Ce]", Emoji: "📜", NerdFont: "\U000f1188"}},
	"cert-manager.io/issuers":             {Category: "cert-manager.io", DisplayName: "Issuers", Icon: Icon{Unicode: "⚑", Simple: "[Is]", Emoji: "✒️", NerdFont: "\U000f0d13"}},
	"cert-manager.io/clusterissuers":      {Category: "cert-manager.io", DisplayName: "ClusterIssuers", Icon: Icon{Unicode: "⚑", Simple: "[Is]", Emoji: "✒️", NerdFont: "\U000f0d13"}},
	"cert-manager.io/certificaterequests": {Category: "cert-manager.io", DisplayName: "CertificateRequests", Icon: Icon{Unicode: "⋇", Simple: "[Rq]", Emoji: "🧾", NerdFont: "\U000f0996"}},
	"acme.cert-manager.io/orders":         {Category: "acme.cert-manager.io", DisplayName: "Orders", Icon: Icon{Unicode: "⋇", Simple: "[Rq]", Emoji: "🧾", NerdFont: "\U000f0996"}},
	"acme.cert-manager.io/challenges":     {Category: "acme.cert-manager.io", DisplayName: "Challenges", Icon: Icon{Unicode: "⋇", Simple: "[Rq]", Emoji: "🧾", NerdFont: "\U000f0996"}},

	// longhorn.io — uses the generic ecosystem CRD fallback glyph.
	"longhorn.io/volumes":       {Category: "longhorn.io", DisplayName: "Volumes", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"longhorn.io/engines":       {Category: "longhorn.io", DisplayName: "Engines", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"longhorn.io/replicas":      {Category: "longhorn.io", DisplayName: "Replicas", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"longhorn.io/nodes":         {Category: "longhorn.io", DisplayName: "Longhorn Nodes", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"longhorn.io/backingimages": {Category: "longhorn.io", DisplayName: "BackingImages", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"longhorn.io/backups":       {Category: "longhorn.io", DisplayName: "Backups", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"longhorn.io/recurringjobs": {Category: "longhorn.io", DisplayName: "RecurringJobs", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"longhorn.io/settings":      {Category: "longhorn.io", DisplayName: "Settings", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// Istio — ecosystem CRDs; "⎈" is now Helm-specific.
	"networking.istio.io/virtualservices":      {Category: "networking.istio.io", DisplayName: "VirtualServices", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"networking.istio.io/destinationrules":     {Category: "networking.istio.io", DisplayName: "DestinationRules", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"networking.istio.io/gateways":             {Category: "networking.istio.io", DisplayName: "Gateways", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"networking.istio.io/serviceentries":       {Category: "networking.istio.io", DisplayName: "ServiceEntries", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"networking.istio.io/sidecars":             {Category: "networking.istio.io", DisplayName: "Sidecars", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"security.istio.io/peerauthentications":    {Category: "security.istio.io", DisplayName: "PeerAuthentications", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"security.istio.io/authorizationpolicies":  {Category: "security.istio.io", DisplayName: "AuthorizationPolicies", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"security.istio.io/requestauthentications": {Category: "security.istio.io", DisplayName: "RequestAuthentications", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"telemetry.istio.io/telemetries":           {Category: "telemetry.istio.io", DisplayName: "Telemetries", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// Cloud provider — ecosystem CRDs; use the generic fallback.
	"cloud.google.com/backendconfigs":                          {Category: "cloud.google.com", DisplayName: "BackendConfigs", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"networking.gke.io/managedcertificates":                    {Category: "networking.gke.io", DisplayName: "ManagedCertificates", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"vpcresources.k8s.aws/securitygrouppolicies":               {Category: "vpcresources.k8s.aws", DisplayName: "SecurityGroupPolicies", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"crd.k8s.amazonaws.com/eniconfigs":                         {Category: "crd.k8s.amazonaws.com", DisplayName: "ENIConfigs", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"aadpodidentity.k8s.io/azureidentities":                    {Category: "aadpodidentity.k8s.io", DisplayName: "AzureIdentities", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"aadpodidentity.k8s.io/azureidentitybindings":              {Category: "aadpodidentity.k8s.io", DisplayName: "AzureIdentityBindings", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"infrastructure.cluster.x-k8s.io/azuremanagedclusters":     {Category: "infrastructure.cluster.x-k8s.io", DisplayName: "AzureManagedClusters", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"infrastructure.cluster.x-k8s.io/azuremanagedmachinepools": {Category: "infrastructure.cluster.x-k8s.io", DisplayName: "AzureManagedMachinePools", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// Karpenter — ecosystem CRDs; use the generic fallback (core Node uses "⌹").
	"karpenter.sh/nodepools":           {Category: "karpenter.sh", DisplayName: "NodePools", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"karpenter.sh/nodeclaims":          {Category: "karpenter.sh", DisplayName: "NodeClaims", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"karpenter.k8s.aws/ec2nodeclasses": {Category: "karpenter.k8s.aws", DisplayName: "EC2NodeClasses", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// Prometheus operator — ecosystem CRDs.
	"monitoring.coreos.com/servicemonitors": {Category: "monitoring.coreos.com", DisplayName: "ServiceMonitors", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"monitoring.coreos.com/podmonitors":     {Category: "monitoring.coreos.com", DisplayName: "PodMonitors", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"monitoring.coreos.com/prometheusrules": {Category: "monitoring.coreos.com", DisplayName: "PrometheusRules", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"monitoring.coreos.com/alertmanagers":   {Category: "monitoring.coreos.com", DisplayName: "Alertmanagers", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"monitoring.coreos.com/prometheuses":    {Category: "monitoring.coreos.com", DisplayName: "Prometheuses", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"monitoring.coreos.com/thanosrulers":    {Category: "monitoring.coreos.com", DisplayName: "ThanosRulers", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// keda.sh — ecosystem CRDs; "⚡" is now /events.
	"keda.sh/scaledobjects":                 {Category: "keda.sh", DisplayName: "ScaledObjects", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"keda.sh/scaledjobs":                    {Category: "keda.sh", DisplayName: "ScaledJobs", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"keda.sh/triggerauthentications":        {Category: "keda.sh", DisplayName: "TriggerAuthentications", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"keda.sh/clustertriggerauthentications": {Category: "keda.sh", DisplayName: "ClusterTriggerAuthentications", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// external-secrets.io — ecosystem CRDs; "⚿" is now RBAC roles.
	"external-secrets.io/externalsecrets":        {Category: "external-secrets.io", DisplayName: "ExternalSecrets", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"external-secrets.io/clusterexternalsecrets": {Category: "external-secrets.io", DisplayName: "ClusterExternalSecrets", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"external-secrets.io/pushsecrets":            {Category: "external-secrets.io", DisplayName: "PushSecrets", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"external-secrets.io/secretstores":           {Category: "external-secrets.io", DisplayName: "SecretStores", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"external-secrets.io/clustersecretstores":    {Category: "external-secrets.io", DisplayName: "ClusterSecretStores", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// bitnami.com — ecosystem CRD; "⚿" is now RBAC roles.
	"bitnami.com/sealedsecrets": {Category: "bitnami.com", DisplayName: "SealedSecrets", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// traefik.io — ecosystem CRDs; "⎈" is now Helm-specific.
	"traefik.io/ingressroutes":    {Category: "traefik.io", DisplayName: "IngressRoutes", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"traefik.io/middlewares":      {Category: "traefik.io", DisplayName: "Middlewares", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"traefik.io/ingressroutetcps": {Category: "traefik.io", DisplayName: "IngressRouteTCPs", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"traefik.io/tlsoptions":       {Category: "traefik.io", DisplayName: "TLSOptions", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// externaldns.k8s.io
	"externaldns.k8s.io/dnsendpoints": {Category: "externaldns.k8s.io", DisplayName: "DNSEndpoints", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// Crossplane
	"apiextensions.crossplane.io/compositions":                 {Category: "apiextensions.crossplane.io", DisplayName: "Compositions", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"apiextensions.crossplane.io/compositeresourcedefinitions": {Category: "apiextensions.crossplane.io", DisplayName: "CompositeResourceDefinitions", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"pkg.crossplane.io/providers":                              {Category: "pkg.crossplane.io", DisplayName: "Providers", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"pkg.crossplane.io/configurations":                         {Category: "pkg.crossplane.io", DisplayName: "Configurations", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// velero.io
	"velero.io/backups":                {Category: "velero.io", DisplayName: "Backups", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"velero.io/restores":               {Category: "velero.io", DisplayName: "Restores", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"velero.io/schedules":              {Category: "velero.io", DisplayName: "Schedules", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"velero.io/backupstoragelocations": {Category: "velero.io", DisplayName: "BackupStorageLocations", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// tekton.dev
	"tekton.dev/pipelines":    {Category: "tekton.dev", DisplayName: "Pipelines", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"tekton.dev/tasks":        {Category: "tekton.dev", DisplayName: "Tasks", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"tekton.dev/pipelineruns": {Category: "tekton.dev", DisplayName: "PipelineRuns", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"tekton.dev/taskruns":     {Category: "tekton.dev", DisplayName: "TaskRuns", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// kafka.strimzi.io
	"kafka.strimzi.io/kafkas":        {Category: "kafka.strimzi.io", DisplayName: "Kafkas", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"kafka.strimzi.io/kafkatopics":   {Category: "kafka.strimzi.io", DisplayName: "KafkaTopics", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"kafka.strimzi.io/kafkaconnects": {Category: "kafka.strimzi.io", DisplayName: "KafkaConnects", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"kafka.strimzi.io/kafkausers":    {Category: "kafka.strimzi.io", DisplayName: "KafkaUsers", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"kafka.strimzi.io/kafkabridges":  {Category: "kafka.strimzi.io", DisplayName: "KafkaBridges", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},

	// knative
	"serving.knative.dev/services":       {Category: "serving.knative.dev", DisplayName: "Knative Services", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"serving.knative.dev/routes":         {Category: "serving.knative.dev", DisplayName: "Routes", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"serving.knative.dev/revisions":      {Category: "serving.knative.dev", DisplayName: "Revisions", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"serving.knative.dev/configurations": {Category: "serving.knative.dev", DisplayName: "Configurations", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"eventing.knative.dev/triggers":      {Category: "eventing.knative.dev", DisplayName: "Triggers", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
	"eventing.knative.dev/brokers":       {Category: "eventing.knative.dev", DisplayName: "Brokers", Icon: Icon{Unicode: "⧫", Simple: "[CR]", Emoji: "🔷", NerdFont: "\U000f0174"}},
}

// BuiltInOrderRank maps "group/resource" keys to their display rank within
// a category. Lower rank appears first. This restores the curated display
// order of the former TopLevelResourceTypes list (e.g., Pods before
// Deployments, not alphabetical). Keys not in this map fall back to
// alphabetical ordering by display name within the category.
//
// Ranks are grouped by category in increments of 10 so new entries can be
// inserted between existing ones without renumbering.
var BuiltInOrderRank = map[string]int{
	// Cluster
	"/nodes":      10,
	"/namespaces": 11,
	"/events":     12,

	// Workloads (the old curated order: Pods first, then controllers by
	// increasing abstraction, then batch).
	"/pods":             20,
	"apps/deployments":  21,
	"apps/replicasets":  22,
	"apps/statefulsets": 23,
	"apps/daemonsets":   24,
	"batch/jobs":        25,
	"batch/cronjobs":    26,

	// Config
	"/configmaps":                          30,
	"/secrets":                             31,
	"autoscaling/horizontalpodautoscalers": 32,
	"/resourcequotas":                      33,
	"/limitranges":                         34,
	"autoscaling.k8s.io/verticalpodautoscalers":                      35,
	"policy/poddisruptionbudgets":                                    36,
	"scheduling.k8s.io/priorityclasses":                              37,
	"node.k8s.io/runtimeclasses":                                     38,
	"coordination.k8s.io/leases":                                     39,
	"admissionregistration.k8s.io/mutatingwebhookconfigurations":     40,
	"admissionregistration.k8s.io/validatingwebhookconfigurations":   41,
	"admissionregistration.k8s.io/validatingadmissionpolicies":       42,
	"admissionregistration.k8s.io/validatingadmissionpolicybindings": 43,
	"flowcontrol.apiserver.k8s.io/flowschemas":                       44,
	"flowcontrol.apiserver.k8s.io/prioritylevelconfigurations":       45,

	// Networking
	"/services":                                    50,
	"/endpoints":                                   51,
	"discovery.k8s.io/endpointslices":              52,
	"networking.k8s.io/networkpolicies":            53,
	"networking.k8s.io/ingresses":                  54,
	"networking.k8s.io/ingressclasses":             55,
	"gateway.networking.k8s.io/gateways":           56,
	"gateway.networking.k8s.io/httproutes":         57,
	"gateway.networking.k8s.io/tlsroutes":          58,
	"gateway.networking.k8s.io/grpcroutes":         59,
	"gateway.networking.k8s.io/tcproutes":          60,
	"gateway.networking.k8s.io/udproutes":          61,
	"gateway.networking.k8s.io/referencegrants":    62,
	"gateway.networking.k8s.io/backendtlspolicies": 63,
	"gateway.networking.k8s.io/gatewayclasses":     64,
	// Rank 65 is reserved for GroupFallbackRank — unknown
	// networking.k8s.io / gateway.networking.k8s.io resources slot
	// in here via itemOrderRank's group-level lookup.
	"_portforward/portforwards": 69,

	// Storage
	"/persistentvolumeclaims":             70,
	"/persistentvolumes":                  71,
	"storage.k8s.io/storageclasses":       72,
	"storage.k8s.io/csidrivers":           73,
	"storage.k8s.io/csinodes":             74,
	"storage.k8s.io/csistoragecapacities": 75,
	"storage.k8s.io/volumeattachments":    76,

	// Access Control
	"/serviceaccounts":                              80,
	"rbac.authorization.k8s.io/roles":               81,
	"rbac.authorization.k8s.io/rolebindings":        82,
	"rbac.authorization.k8s.io/clusterroles":        83,
	"rbac.authorization.k8s.io/clusterrolebindings": 84,

	// Helm
	"_helm/releases": 90,

	// API and CRDs
	"apiregistration.k8s.io/apiservices":             100,
	"apiextensions.k8s.io/customresourcedefinitions": 101,
}

// PseudoResources returns the LFK-only resource types that are not served
// by any Kubernetes cluster but need to appear in the sidebar, resolve via
// FindResourceType*, and round-trip through the command bar. The entries
// use sentinel API groups ("_helm", "_portforward") that GetResources and
// GetResourceYAML recognize and route to helm/port-forward handlers.
//
// These entries should be prepended to the discovered resource set in both
// the async discovery handler and the pre-discovery seed path so they are
// uniformly available regardless of cluster state.
func PseudoResources() []ResourceTypeEntry {
	return []ResourceTypeEntry{
		{
			DisplayName: "Releases",
			Kind:        "HelmRelease",
			APIGroup:    "_helm",
			APIVersion:  "v1",
			Resource:    "releases",
			Namespaced:  true,
		},
		{
			DisplayName: "Port Forwards",
			Kind:        "__port_forwards__",
			APIGroup:    "_portforward",
			APIVersion:  "v1",
			Resource:    "portforwards",
			Namespaced:  false,
		},
	}
}

// SeedResources returns the minimum set of core Kubernetes resources that
// should appear in the sidebar before DiscoverAPIResources completes for a
// new context, plus the LFK pseudo-resources from PseudoResources(). Every
// entry must have a matching BuiltInMetadata key.
//
// The seed is replaced wholesale (not merged) the first time discovery
// returns for a context — that replacement path also prepends
// PseudoResources() so the pseudo-entries remain present.
func SeedResources() []ResourceTypeEntry {
	seed := PseudoResources()
	seed = append(seed,
		// Workloads
		ResourceTypeEntry{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
		ResourceTypeEntry{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
		ResourceTypeEntry{Kind: "StatefulSet", APIGroup: "apps", APIVersion: "v1", Resource: "statefulsets", Namespaced: true},
		ResourceTypeEntry{Kind: "DaemonSet", APIGroup: "apps", APIVersion: "v1", Resource: "daemonsets", Namespaced: true},
		ResourceTypeEntry{Kind: "ReplicaSet", APIGroup: "apps", APIVersion: "v1", Resource: "replicasets", Namespaced: true},
		ResourceTypeEntry{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs", Namespaced: true},
		ResourceTypeEntry{Kind: "CronJob", APIGroup: "batch", APIVersion: "v1", Resource: "cronjobs", Namespaced: true},
		// Networking
		ResourceTypeEntry{Kind: "Service", APIGroup: "", APIVersion: "v1", Resource: "services", Namespaced: true},
		ResourceTypeEntry{Kind: "Ingress", APIGroup: "networking.k8s.io", APIVersion: "v1", Resource: "ingresses", Namespaced: true},
		// Config
		ResourceTypeEntry{Kind: "ConfigMap", APIGroup: "", APIVersion: "v1", Resource: "configmaps", Namespaced: true},
		ResourceTypeEntry{Kind: "Secret", APIGroup: "", APIVersion: "v1", Resource: "secrets", Namespaced: true},
		// Cluster
		ResourceTypeEntry{Kind: "Namespace", APIGroup: "", APIVersion: "v1", Resource: "namespaces", Namespaced: false},
		ResourceTypeEntry{Kind: "Node", APIGroup: "", APIVersion: "v1", Resource: "nodes", Namespaced: false},
		ResourceTypeEntry{Kind: "Event", APIGroup: "", APIVersion: "v1", Resource: "events", Namespaced: true},
		// Storage
		ResourceTypeEntry{Kind: "PersistentVolumeClaim", APIGroup: "", APIVersion: "v1", Resource: "persistentvolumeclaims", Namespaced: true},
		ResourceTypeEntry{Kind: "PersistentVolume", APIGroup: "", APIVersion: "v1", Resource: "persistentvolumes", Namespaced: false},
		// Access Control
		ResourceTypeEntry{Kind: "ServiceAccount", APIGroup: "", APIVersion: "v1", Resource: "serviceaccounts", Namespaced: true},
	)
	return seed
}
