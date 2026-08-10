package app

import (
	"maps"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
)

// actionQueries maps a resource kind, then an action label, to the
// authorization question that decides it. Only actions whose verb is
// unambiguous are listed: a label that is absent is always offered, which
// keeps the gate fail-open by construction. A kind that is absent keeps the
// read-only gate alone.
//
// The verbs mirror what the action actually sends: Scale calls UpdateScale
// (update on the scale subresource), Restart and Rollback patch the object,
// Edit runs kubectl edit, which patches. Scale reads the scale subresource
// before it writes, but only the write is asked about: a role holding update
// without get is rare, and asking for both could hide an action the user can
// in fact run.
var actionQueries = map[string]map[string]k8s.PermissionQuery{
	"Pod": mergeQueries(podRuntimeQueries(), map[string]k8s.PermissionQuery{
		"Delete":       {Resource: "pods", Verb: "delete"},
		"Force Delete": {Resource: "pods", Verb: "delete"},
		"Edit":         {Resource: "pods", Verb: "patch"},
		"Debug":        {Resource: "pods", Subresource: "ephemeralcontainers", Verb: "patch"},
		// Generated when a row is already deleting; see openResourceActionMenu.
		"Force Finalize": {Resource: "pods", Verb: "patch"},
	}),
	"Deployment": mergeQueries(workloadQueries("deployments"), podRuntimeQueries(), map[string]k8s.PermissionQuery{
		"Scale":    {Group: "apps", Resource: "deployments", Subresource: "scale", Verb: "update"},
		"Rollback": {Group: "apps", Resource: "deployments", Verb: "patch"},
	}),
	"StatefulSet": mergeQueries(workloadQueries("statefulsets"), podRuntimeQueries(), map[string]k8s.PermissionQuery{
		"Scale": {Group: "apps", Resource: "statefulsets", Subresource: "scale", Verb: "update"},
	}),
	"DaemonSet": mergeQueries(workloadQueries("daemonsets"), podRuntimeQueries()),
	// The ReplicaSet menu offers no pod-level action except Debug Pod, so it
	// asks nothing about logs, exec or port-forward.
	"ReplicaSet": mergeQueries(workloadQueries("replicasets"), map[string]k8s.PermissionQuery{
		"Scale":     {Group: "apps", Resource: "replicasets", Subresource: "scale", Verb: "update"},
		"Debug Pod": {Resource: "pods", Verb: "create"},
	}),
}

// podRuntimeQueries are the actions that reach the pods themselves. A
// workload menu offers them too, and there they still ask about pods: lfk
// resolves the workload to a pod and acts on that.
func podRuntimeQueries() map[string]k8s.PermissionQuery {
	return map[string]k8s.PermissionQuery{
		"Tail Logs":    {Resource: "pods", Subresource: "log", Verb: "get"},
		"Logs":         {Resource: "pods", Subresource: "log", Verb: "get"},
		"Log Top":      {Resource: "pods", Subresource: "log", Verb: "get"},
		"Exec":         {Resource: "pods", Subresource: "exec", Verb: "create"},
		"Attach":       {Resource: "pods", Subresource: "attach", Verb: "create"},
		"Port Forward": {Resource: "pods", Subresource: "portforward", Verb: "create"},
		"Debug Pod":    {Resource: "pods", Verb: "create"},
	}
}

// workloadQueries builds the verbs every apps/v1 workload menu carries. Force
// Finalize is generated when a row is already deleting (see
// openResourceActionMenu), and removing finalizers is a patch, so it costs no
// extra review: it shares the key Edit and Restart already ask for.
func workloadQueries(resource string) map[string]k8s.PermissionQuery {
	return map[string]k8s.PermissionQuery{
		"Delete":         {Group: "apps", Resource: resource, Verb: "delete"},
		"Edit":           {Group: "apps", Resource: resource, Verb: "patch"},
		"Restart":        {Group: "apps", Resource: resource, Verb: "patch"},
		"Force Finalize": {Group: "apps", Resource: resource, Verb: "patch"},
	}
}

// mergeQueries folds the maps into the first one, later entries winning.
func mergeQueries(into map[string]k8s.PermissionQuery, rest ...map[string]k8s.PermissionQuery) map[string]k8s.PermissionQuery {
	for _, m := range rest {
		maps.Copy(into, m)
	}
	return into
}

// permScopeKey scopes a cached verdict to the cluster, namespace and kind it
// was asked about. Two tabs on different contexts never read each other's
// answer because the context is part of the key; the kind is part of it
// because each kind asks a different set of questions.
type permScopeKey struct {
	context   string
	namespace string
	kind      string
}

// permissionState caches the bulk review per scope for the life of the
// process. A user's rights inside one context do not change between tabs, so
// the cache is shared; the key keeps contexts and namespaces apart.
type permissionState struct {
	allowed  map[permScopeKey]map[string]bool
	inflight map[permScopeKey]time.Time
}

// newPermissionState allocates the maps up front. The Model travels by value
// through the update loop, so a map allocated later inside a method would be
// written to a copy and thrown away with it.
func newPermissionState() permissionState {
	return permissionState{
		allowed:  make(map[permScopeKey]map[string]bool),
		inflight: make(map[permScopeKey]time.Time),
	}
}

// permissionRetryAfter is how long a started pass is taken to be still
// running. The marker has to expire: the review is scheduled at low priority,
// so leaving the namespace supersedes it and the scheduler drops it without a
// reply. A marker that only a reply could clear would then block every later
// pass for that namespace, and the menu would stop hiding anything.
const permissionRetryAfter = 30 * time.Second

// permNow is time.Now, replaced in tests that drive the retry window.
var permNow = time.Now

// record stores a completed bulk pass and retires the in-flight marker.
func (s *permissionState) record(key permScopeKey, allowed map[string]bool) {
	if s.allowed == nil {
		s.allowed = make(map[permScopeKey]map[string]bool)
	}
	s.allowed[key] = allowed
	delete(s.inflight, key)
}

// clear drops every verdict without replacing the maps. The Model travels by
// value, so a fresh map assigned here would be written to a copy; emptying
// the maps in place reaches every copy that shares them.
func (s *permissionState) clear() {
	for key := range s.allowed {
		delete(s.allowed, key)
	}
	for key := range s.inflight {
		delete(s.inflight, key)
	}
}

// fail drops the in-flight marker without storing a verdict, so the scope
// stays unanswered and every action keeps showing.
func (s *permissionState) fail(key permScopeKey) {
	delete(s.inflight, key)
}

// begin marks a scope as being reviewed and reports whether the caller should
// start the pass. It returns false when the answer is already cached or a
// pass started less than permissionRetryAfter ago, which is what keeps a
// namespace to one bulk pass.
func (s *permissionState) begin(key permScopeKey) bool {
	if _, ok := s.allowed[key]; ok {
		return false
	}
	now := permNow()
	if started, ok := s.inflight[key]; ok && now.Sub(started) < permissionRetryAfter {
		return false
	}
	if s.inflight == nil {
		s.inflight = make(map[permScopeKey]time.Time)
	}
	s.inflight[key] = now
	return true
}

// permissionQueriesFor is the set the bulk pass asks for, derived from the
// action map so a new action entry cannot be forgotten here.
func permissionQueriesFor(kind string) []k8s.PermissionQuery {
	byLabel, ok := actionQueries[kind]
	if !ok {
		return nil
	}
	queries := make([]k8s.PermissionQuery, 0, len(byLabel))
	for _, q := range byLabel {
		queries = append(queries, q)
	}
	return queries
}

// deniedByRBAC reports whether the cluster has already told us this action
// would be refused.
//
// It answers false whenever the answer is not known: no review yet, the
// review failed, an action with no mapped verb, or a kind with no verb map. A
// hidden action that would have worked is worse than an action that fails,
// and an aggregated or webhook authorizer can also overrule a denial.
func (m Model) deniedByRBAC(kind, label string) bool {
	query, ok := actionQueries[kind][label]
	if !ok {
		return false
	}
	scope, ok := m.permScopeFor(m.actionCtx.context, m.actionCtx.namespace, kind)
	if !ok {
		return false
	}
	allowed, ok := m.perms.allowed[scope]
	if !ok {
		return false
	}
	verdict, ok := allowed[query.Key()]
	if !ok {
		return false
	}
	return !verdict
}

// permScopeFor builds the cache key for a target, refusing scopes the review
// cannot speak for: the union sentinel spans several clusters, an
// all-namespaces list has no single namespace to review, and a kind with no
// verb map has nothing to ask.
func (m Model) permScopeFor(contextName, namespace, kind string) (permScopeKey, bool) {
	if contextName == "" || contextName == UnionContextSentinel || namespace == "" {
		return permScopeKey{}, false
	}
	if _, ok := actionQueries[kind]; !ok {
		return permScopeKey{}, false
	}
	return permScopeKey{context: contextName, namespace: namespace, kind: kind}, true
}

// actionBlockedReason is the single gate in front of every action entry.
// Read-only mode and RBAC both answer here, so the menu that hides an entry
// and the dispatcher that refuses it can never disagree. The returned string
// is the toast the dispatcher shows.
func (m Model) actionBlockedReason(kind, label string) (string, bool) {
	if isMutatingActionForKind(kind, label) && m.readOnlyForContext(m.actionCtx.context) {
		return readOnlyBlockedMessage(label), true
	}
	if m.deniedByRBAC(kind, label) {
		return rbacBlockedMessage(label), true
	}
	return "", false
}

// actionBlocked is the menu-side form of actionBlockedReason: the entry is
// dropped and no toast is needed.
func (m Model) actionBlocked(kind, label string) bool {
	_, blocked := m.actionBlockedReason(kind, label)
	return blocked
}

// bulkActionBlockedReason is the RBAC half of the gate for a whole selection.
// It answers only when every selected row sits in the context and namespace
// the action targets; a mixed selection has no single verdict to apply, so the
// gate stands aside and the API server decides row by row.
//
// Read-only is not repeated here: executeBulkAction checks it first, and it
// treats a mixed selection as all-or-nothing, which RBAC deliberately does not.
func (m Model) bulkActionBlockedReason(kind, label string) (string, bool) {
	if len(m.bulkItems) == 0 || !m.bulkTargetsOneScope() {
		return "", false
	}
	if m.deniedByRBAC(kind, label) {
		return rbacBlockedMessage(label), true
	}
	return "", false
}

// bulkTargetsOneScope reports whether every selected row sits in the context
// and namespace named by the action context.
func (m Model) bulkTargetsOneScope() bool {
	for _, item := range m.bulkItems {
		if item.Namespace != "" && item.Namespace != m.actionCtx.namespace {
			return false
		}
		if item.ClusterName != "" && item.ClusterName != m.actionCtx.context {
			return false
		}
	}
	return true
}

// rbacBlockedMessage returns the toast used when the cluster refuses an
// action. Centralised so tests can assert on the exact format.
func rbacBlockedMessage(actionLabel string) string {
	return "Not permitted: " + actionLabel + " denied by RBAC"
}
