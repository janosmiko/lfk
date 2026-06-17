package model

// ResourceTemplate defines a predefined YAML template for creating Kubernetes resources.
type ResourceTemplate struct {
	Name        string
	Description string
	Category    string // "Workloads", "Networking", "Config", etc.
	YAML        string
}

// BuiltinTemplates returns the list of predefined resource templates.
// Templates are ordered by category: Workloads, Networking, Config, Storage,
// Access Control, Monitoring, Cluster, Custom.
func BuiltinTemplates() []ResourceTemplate {
	return []ResourceTemplate{
		// ---- Workloads ----
		{
			Name:        "Pod",
			Description: "Simple nginx pod",
			Category:    "Workloads",
			YAML: `apiVersion: v1
kind: Pod
metadata:
  name: my-pod
  namespace: NAMESPACE
  labels:
    app: my-app
spec:
  containers:
    - name: nginx
      image: nginx:latest
      ports:
        - containerPort: 80
`,
		},
		{
			Name:        "Deployment",
			Description: "Basic nginx deployment with 1 replica",
			Category:    "Workloads",
			YAML: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
  namespace: NAMESPACE
  labels:
    app: my-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: nginx
          image: nginx:latest
          ports:
            - containerPort: 80
`,
		},
		{
			Name:        "ReplicaSet",
			Description: "Basic ReplicaSet with 2 replicas",
			Category:    "Workloads",
			YAML: `apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: my-replicaset
  namespace: NAMESPACE
  labels:
    app: my-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: nginx
          image: nginx:latest
          ports:
            - containerPort: 80
`,
		},
		{
			Name:        "StatefulSet",
			Description: "Basic StatefulSet with volumeClaimTemplates",
			Category:    "Workloads",
			YAML: `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-statefulset
  namespace: NAMESPACE
  labels:
    app: my-app
spec:
  serviceName: my-statefulset
  replicas: 1
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: app
          image: nginx:latest
          ports:
            - containerPort: 80
          volumeMounts:
            - name: data
              mountPath: /data
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
`,
		},
		{
			Name:        "DaemonSet",
			Description: "Basic DaemonSet for log collection",
			Category:    "Workloads",
			YAML: `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: my-daemonset
  namespace: NAMESPACE
  labels:
    app: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: log-collector
          image: busybox:latest
          command: ["sh", "-c", "tail -f /var/log/syslog"]
          volumeMounts:
            - name: varlog
              mountPath: /var/log
              readOnly: true
      volumes:
        - name: varlog
          hostPath:
            path: /var/log
`,
		},
		{
			Name:        "Job",
			Description: "Basic job running a command",
			Category:    "Workloads",
			YAML: `apiVersion: batch/v1
kind: Job
metadata:
  name: my-job
  namespace: NAMESPACE
spec:
  template:
    spec:
      containers:
        - name: worker
          image: busybox
          command: ["echo", "Hello from Job"]
      restartPolicy: Never
  backoffLimit: 3
`,
		},
		{
			Name:        "CronJob",
			Description: "CronJob running every hour",
			Category:    "Workloads",
			YAML: `apiVersion: batch/v1
kind: CronJob
metadata:
  name: my-cronjob
  namespace: NAMESPACE
spec:
  schedule: "0 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: worker
              image: busybox
              command: ["echo", "Hello from CronJob"]
          restartPolicy: Never
`,
		},
		{
			Name:        "HorizontalPodAutoscaler",
			Description: "HPA targeting a deployment with CPU scaling",
			Category:    "Workloads",
			YAML: `apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-hpa
  namespace: NAMESPACE
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-deployment
  minReplicas: 1
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 80
`,
		},
		// ---- Networking ----
		{
			Name:        "Service",
			Description: "ClusterIP service exposing port 80",
			Category:    "Networking",
			YAML: `apiVersion: v1
kind: Service
metadata:
  name: my-service
  namespace: NAMESPACE
  labels:
    app: my-app
spec:
  type: ClusterIP
  selector:
    app: my-app
  ports:
    - name: http
      port: 80
      targetPort: 80
      protocol: TCP
`,
		},
		{
			Name:        "Ingress",
			Description: "Basic ingress rule",
			Category:    "Networking",
			YAML: `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: NAMESPACE
spec:
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 80
`,
		},
		{
			Name:        "NetworkPolicy",
			Description: "Allow ingress from same namespace",
			Category:    "Networking",
			YAML: `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: my-network-policy
  namespace: NAMESPACE
spec:
  podSelector:
    matchLabels:
      app: my-app
  policyTypes:
    - Ingress
  ingress:
    - from:
        - podSelector: {}
      ports:
        - protocol: TCP
          port: 80
`,
		},
		// ---- Config ----
		{
			Name:        "ConfigMap",
			Description: "Empty configmap with sample data",
			Category:    "Config",
			YAML: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: NAMESPACE
data:
  key: value
`,
		},
		{
			Name:        "Secret",
			Description: "Opaque secret with sample data",
			Category:    "Config",
			YAML: `apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: NAMESPACE
type: Opaque
stringData:
  username: admin
  password: changeme
`,
		},
		{
			Name:        "ResourceQuota",
			Description: "Basic resource quota for CPU and memory",
			Category:    "Config",
			YAML: `apiVersion: v1
kind: ResourceQuota
metadata:
  name: my-quota
  namespace: NAMESPACE
spec:
  hard:
    requests.cpu: "4"
    requests.memory: 8Gi
    limits.cpu: "8"
    limits.memory: 16Gi
    pods: "20"
`,
		},
		{
			Name:        "LimitRange",
			Description: "Default CPU and memory limits for containers",
			Category:    "Config",
			YAML: `apiVersion: v1
kind: LimitRange
metadata:
  name: my-limits
  namespace: NAMESPACE
spec:
  limits:
    - type: Container
      default:
        cpu: 500m
        memory: 256Mi
      defaultRequest:
        cpu: 100m
        memory: 128Mi
`,
		},
		// ---- Storage ----
		{
			Name:        "PersistentVolumeClaim",
			Description: "1Gi PVC with ReadWriteOnce access",
			Category:    "Storage",
			YAML: `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
  namespace: NAMESPACE
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
`,
		},
		{
			Name:        "PersistentVolume",
			Description: "Basic 10Gi PersistentVolume with hostPath",
			Category:    "Storage",
			YAML: `apiVersion: v1
kind: PersistentVolume
metadata:
  name: my-pv
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  hostPath:
    path: /data
`,
		},
		{
			Name:        "StorageClass",
			Description: "Basic StorageClass with default provisioner",
			Category:    "Storage",
			YAML: `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: my-storage-class
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Retain
`,
		},
		// ---- Access Control ----
		{
			Name:        "ServiceAccount",
			Description: "Basic service account",
			Category:    "Access Control",
			YAML: `apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service-account
  namespace: NAMESPACE
`,
		},
		{
			Name:        "Role",
			Description: "Read-only Role for pods and services",
			Category:    "Access Control",
			YAML: `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: my-role
  namespace: NAMESPACE
rules:
  - apiGroups: [""]
    resources: ["pods", "services"]
    verbs: ["get", "list", "watch"]
`,
		},
		{
			Name:        "RoleBinding",
			Description: "Bind a Role to a ServiceAccount",
			Category:    "Access Control",
			YAML: `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: my-rolebinding
  namespace: NAMESPACE
subjects:
  - kind: ServiceAccount
    name: my-service-account
    namespace: NAMESPACE
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: my-role
`,
		},
		{
			Name:        "ClusterRole",
			Description: "Cluster-wide read-only ClusterRole",
			Category:    "Access Control",
			YAML: `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-clusterrole
rules:
  - apiGroups: [""]
    resources: ["pods", "services", "namespaces"]
    verbs: ["get", "list", "watch"]
`,
		},
		{
			Name:        "ClusterRoleBinding",
			Description: "Bind a ClusterRole to a ServiceAccount",
			Category:    "Access Control",
			YAML: `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-clusterrolebinding
subjects:
  - kind: ServiceAccount
    name: my-service-account
    namespace: NAMESPACE
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: my-clusterrole
`,
		},
		// ---- Monitoring ----
		{
			Name:        "ServiceMonitor",
			Description: "Prometheus ServiceMonitor for metrics scraping",
			Category:    "Monitoring",
			YAML: `apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: my-service-monitor
  namespace: NAMESPACE
  labels:
    release: prometheus
spec:
  selector:
    matchLabels:
      app: my-app
  endpoints:
    - port: http
      interval: 30s
      path: /metrics
`,
		},
		// ---- Cluster ----
		{
			Name:        "Namespace",
			Description: "Basic namespace",
			Category:    "Cluster",
			YAML: `apiVersion: v1
kind: Namespace
metadata:
  name: my-namespace
`,
		},
		// ---- Custom ----
		{
			Name:        "Custom Resource",
			Description: "Empty starting point — edit apiVersion, kind, and spec",
			Category:    "Custom",
			YAML: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-resource
  namespace: NAMESPACE
`,
		},
	}
}
