package model

// ResourceTemplate defines a predefined YAML template for creating Kubernetes resources.
type ResourceTemplate struct {
	Name        string
	Description string
	Category    string // "Workloads", "Networking", "Config", etc.
	YAML        string
}

// BuiltinTemplates returns the list of predefined resource templates.
func BuiltinTemplates() []ResourceTemplate {
	return []ResourceTemplate{
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
    app: my-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-deployment
  template:
    metadata:
      labels:
        app: my-deployment
    spec:
      containers:
        - name: nginx
          image: nginx:latest
          ports:
            - containerPort: 80
`,
		},
		{
			Name:        "Service",
			Description: "ClusterIP service exposing port 80",
			Category:    "Networking",
			YAML: `apiVersion: v1
kind: Service
metadata:
  name: my-service
  namespace: NAMESPACE
spec:
  type: ClusterIP
  selector:
    app: my-app
  ports:
    - port: 80
      targetPort: 80
      protocol: TCP
`,
		},
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
			Name:        "Namespace",
			Description: "Basic namespace",
			Category:    "Cluster",
			YAML: `apiVersion: v1
kind: Namespace
metadata:
  name: my-namespace
`,
		},
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
	}
}

// TemplateCategories returns the unique ordered categories from the templates.
func TemplateCategories() []string {
	seen := make(map[string]bool)
	var cats []string
	for _, t := range BuiltinTemplates() {
		if !seen[t.Category] {
			seen[t.Category] = true
			cats = append(cats, t.Category)
		}
	}
	return cats
}
