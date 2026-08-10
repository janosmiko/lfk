package demo

// Namespaces spanned by the seed data. NamespaceDefault and
// NamespaceKubeSystem carry no workloads but are seeded as Namespace objects
// so the namespace picker looks like a real cluster (see buildNamespaces).
const (
	NamespaceDemo       = "demo"
	NamespaceJobs       = "demo-jobs"
	NamespaceDefault    = "default"
	NamespaceKubeSystem = "kube-system"
)

// Object names, exported so tests (in this package and internal/k8s) can
// reference the seed data without hardcoding strings.
const (
	NodeControlPlane = "demo-control-plane"
	NodeWorker1      = "demo-worker-1"

	DeploymentWeb   = "web"
	ReplicaSetWeb   = "web-7d8f9c6b5"
	PodWebHealthy1  = "web-7d8f9c6b5-9k2pl"
	PodWebHealthy2  = "web-7d8f9c6b5-4m8qz"
	PodWebHealthy3  = "web-7d8f9c6b5-2rvft"
	PodWebHealthy4  = "web-7d8f9c6b5-8jdmn"
	PodWebHealthy5  = "web-7d8f9c6b5-5wqxc"
	PodWebCrashLoop = "web-7d8f9c6b5-x7bwn"

	ServiceWeb   = "web"
	ConfigMapWeb = "web-config"

	JobDBMigrate = "db-migrate-28472913"
	PodDBMigrate = "db-migrate-28472913-9f2lk"
)

// uids are fixed so ownerReferences and event involvedObject links stay
// stable across process runs instead of being regenerated each time.
const (
	uidNodeControlPlane = "a0000000-0000-4000-8000-000000000001"
	uidNodeWorker1      = "a0000000-0000-4000-8000-000000000002"

	uidDeploymentWeb  = "b0000000-0000-4000-8000-000000000001"
	uidReplicaSetWeb  = "b0000000-0000-4000-8000-000000000002"
	uidPodWebHealthy1 = "b0000000-0000-4000-8000-000000000003"
	uidPodWebHealthy2 = "b0000000-0000-4000-8000-000000000004"
	uidPodWebCrash    = "b0000000-0000-4000-8000-000000000005"
	uidPodWebHealthy3 = "b0000000-0000-4000-8000-000000000008"
	uidPodWebHealthy4 = "b0000000-0000-4000-8000-000000000009"
	uidPodWebHealthy5 = "b0000000-0000-4000-8000-00000000000a"

	uidServiceWeb   = "b0000000-0000-4000-8000-000000000006"
	uidConfigMapWeb = "b0000000-0000-4000-8000-000000000007"

	uidJobDBMigrate = "c0000000-0000-4000-8000-000000000001"
	uidPodDBMigrate = "c0000000-0000-4000-8000-000000000002"

	uidEventPodCrashBackOff   = "d0000000-0000-4000-8000-000000000001"
	uidEventPodCrashUnhealthy = "d0000000-0000-4000-8000-000000000002"
	uidEventJobBackoffLimit   = "d0000000-0000-4000-8000-000000000003"

	uidNamespaceDemo       = "f0000000-0000-4000-8000-000000000001"
	uidNamespaceJobs       = "f0000000-0000-4000-8000-000000000002"
	uidNamespaceDefault    = "f0000000-0000-4000-8000-000000000003"
	uidNamespaceKubeSystem = "f0000000-0000-4000-8000-000000000004"
)
