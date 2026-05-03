package k8s

import "time"

// RBACCheck represents a single permission check result.
type RBACCheck struct {
	Verb    string
	Allowed bool
}

// AccessRule represents a single access rule from SelfSubjectRulesReview.
type AccessRule struct {
	Verbs         []string
	APIGroups     []string
	Resources     []string
	ResourceNames []string // empty means all names
}

// QuotaInfo holds resource quota data for a single ResourceQuota object.
type QuotaInfo struct {
	Name      string
	Namespace string
	Resources []QuotaResource
}

// QuotaResource holds usage data for a single resource within a quota.
type QuotaResource struct {
	Name    string  // e.g. "cpu", "memory", "pods", "services"
	Hard    string  // limit
	Used    string  // current usage
	Percent float64 // usage percentage (0-100)
}

// RBACSubject represents a unique subject (User, Group, or ServiceAccount) found
// in ClusterRoleBindings or RoleBindings.
type RBACSubject struct {
	Kind      string // "User", "Group", or "ServiceAccount"
	Name      string
	Namespace string // only populated for ServiceAccount
}

// DeploymentRevision represents a deployment revision history entry.
type DeploymentRevision struct {
	Revision  int64
	Name      string
	Replicas  int32
	Images    []string
	CreatedAt time.Time
}
