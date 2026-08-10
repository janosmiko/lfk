package app

import (
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
)

// podActionQueries maps a Pod action label to the authorization question that
// decides it. Only actions whose verb is unambiguous are listed: an action
// that is absent is always offered, which keeps the gate fail-open by
// construction.
//
// Scoped to Pods. Other kinds keep the read-only gate alone until their verb
// map is written.
var podActionQueries = map[string]k8s.PermissionQuery{
	"Delete":       {Resource: "pods", Verb: "delete"},
	"Force Delete": {Resource: "pods", Verb: "delete"},
	"Edit":         {Resource: "pods", Verb: "patch"},
	"Exec":         {Resource: "pods", Subresource: "exec", Verb: "create"},
	"Attach":       {Resource: "pods", Subresource: "attach", Verb: "create"},
	"Debug":        {Resource: "pods", Subresource: "ephemeralcontainers", Verb: "patch"},
	"Debug Pod":    {Resource: "pods", Verb: "create"},
	"Port Forward": {Resource: "pods", Subresource: "portforward", Verb: "create"},
	"Tail Logs":    {Resource: "pods", Subresource: "log", Verb: "get"},
	"Logs":         {Resource: "pods", Subresource: "log", Verb: "get"},
	"Log Top":      {Resource: "pods", Subresource: "log", Verb: "get"},
}

// permScopeKey scopes a cached verdict to the cluster and namespace it was
// asked about. Two tabs on different contexts never read each other's answer
// because the context is part of the key.
type permScopeKey struct {
	context   string
	namespace string
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

// podPermissionQueries is the set the bulk pass asks for, derived from the
// action map so a new action entry cannot be forgotten here.
func podPermissionQueries() []k8s.PermissionQuery {
	queries := make([]k8s.PermissionQuery, 0, len(podActionQueries))
	for _, q := range podActionQueries {
		queries = append(queries, q)
	}
	return queries
}

// deniedByRBAC reports whether the cluster has already told us this action
// would be refused.
//
// It answers false whenever the answer is not known: no review yet, the
// review failed, an action with no mapped verb, or a kind outside the Pod
// scope. A hidden action that would have worked is worse than an action that
// fails, and an aggregated or webhook authorizer can also overrule a denial.
func (m Model) deniedByRBAC(kind, label string) bool {
	if kind != "Pod" {
		return false
	}
	query, ok := podActionQueries[label]
	if !ok {
		return false
	}
	scope, ok := m.permScopeFor(m.actionCtx.context, m.actionCtx.namespace)
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
// cannot speak for: the union sentinel spans several clusters, and an
// all-namespaces list has no single namespace to review.
func (m Model) permScopeFor(contextName, namespace string) (permScopeKey, bool) {
	if contextName == "" || contextName == UnionContextSentinel || namespace == "" {
		return permScopeKey{}, false
	}
	return permScopeKey{context: contextName, namespace: namespace}, true
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

// rbacBlockedMessage returns the toast used when the cluster refuses an
// action. Centralised so tests can assert on the exact format.
func rbacBlockedMessage(actionLabel string) string {
	return "Not permitted: " + actionLabel + " denied by RBAC"
}
