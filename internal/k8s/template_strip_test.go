package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	sigsyaml "sigs.k8s.io/yaml"
)

// hasPath walks a decoded YAML document by map keys and reports whether the
// path resolves. List indices are not supported — the assertions below only
// need map traversal.
func hasPath(t *testing.T, doc string, path ...string) bool {
	t.Helper()
	var obj map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(doc), &obj))
	var cur any = obj
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		cur, ok = m[key]
		if !ok {
			return false
		}
	}
	return true
}

const livePodYAML = `apiVersion: v1
kind: Pod
metadata:
  name: web-0
  namespace: prod
  uid: 8c3b1f52-0000-4000-8000-000000000001
  resourceVersion: "918273"
  generation: 3
  creationTimestamp: "2026-01-02T03:04:05Z"
  selfLink: /api/v1/namespaces/prod/pods/web-0
  deletionTimestamp: "2026-01-02T04:04:05Z"
  deletionGracePeriodSeconds: 30
  labels:
    app: web
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: '{"apiVersion":"v1"}'
    team: payments
  ownerReferences:
    - apiVersion: apps/v1
      kind: ReplicaSet
      name: web
      uid: 8c3b1f52-0000-4000-8000-000000000002
  managedFields:
    - manager: kubelet
      operation: Update
spec:
  nodeName: ip-10-0-1-7
  serviceAccount: web-sa
  serviceAccountName: web-sa
  containers:
    - name: app
      image: nginx:1.27
      volumeMounts:
        - name: config
          mountPath: /etc/config
        - name: kube-api-access-x7f2q
          readOnly: true
          mountPath: /var/run/secrets/kubernetes.io/serviceaccount
  volumes:
    - name: config
      configMap:
        name: web-config
    - name: kube-api-access-x7f2q
      projected:
        sources:
          - serviceAccountToken:
              path: token
status:
  phase: Running
`

const liveServiceYAML = `apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: prod
  uid: 8c3b1f52-0000-4000-8000-000000000003
spec:
  clusterIP: 10.96.4.11
  clusterIPs:
    - 10.96.4.11
  ipFamilies:
    - IPv4
  ipFamilyPolicy: SingleStack
  type: ClusterIP
  selector:
    app: web
  ports:
    - name: http
      port: 80
      targetPort: 8080
status:
  loadBalancer: {}
`

const livePVCYAML = `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-web-0
  namespace: prod
  annotations:
    pv.kubernetes.io/bind-completed: "yes"
    pv.kubernetes.io/bound-by-controller: "yes"
    volume.beta.kubernetes.io/storage-provisioner: ebs.csi.aws.com
    volume.kubernetes.io/storage-provisioner: ebs.csi.aws.com
    volume.kubernetes.io/selected-node: ip-10-0-1-7
    team: payments
spec:
  volumeName: pvc-8c3b1f52-0000-4000-8000-000000000004
  storageClassName: gp3
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
status:
  phase: Bound
`

const liveJobYAML = `apiVersion: batch/v1
kind: Job
metadata:
  name: report-29384
  namespace: prod
  labels:
    batch.kubernetes.io/controller-uid: 8c3b1f52-0000-4000-8000-000000000005
    batch.kubernetes.io/job-name: report-29384
    controller-uid: 8c3b1f52-0000-4000-8000-000000000005
    job-name: report-29384
    team: payments
spec:
  completions: 1
  selector:
    matchLabels:
      batch.kubernetes.io/controller-uid: 8c3b1f52-0000-4000-8000-000000000005
  template:
    metadata:
      creationTimestamp: null
      labels:
        batch.kubernetes.io/controller-uid: 8c3b1f52-0000-4000-8000-000000000005
        batch.kubernetes.io/job-name: report-29384
        controller-uid: 8c3b1f52-0000-4000-8000-000000000005
        job-name: report-29384
        team: payments
    spec:
      restartPolicy: Never
      containers:
        - name: report
          image: reporter:2.1
status:
  succeeded: 1
`

const liveDeploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: prod
  generation: 7
  annotations:
    deployment.kubernetes.io/revision: "4"
    team: payments
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: web
    spec:
      containers:
        - name: app
          image: nginx:1.27
status:
  readyReplicas: 3
`

const liveCronJobYAML = `apiVersion: batch/v1
kind: CronJob
metadata:
  name: report
  namespace: prod
  uid: 8c3b1f52-0000-4000-8000-000000000006
  creationTimestamp: "2026-01-02T03:04:05Z"
  labels:
    team: payments
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    metadata:
      creationTimestamp: null
    spec:
      template:
        metadata:
          creationTimestamp: null
        spec:
          restartPolicy: OnFailure
          containers:
            - name: report
              image: reporter:2.1
status:
  lastScheduleTime: "2026-01-02T03:05:00Z"
`

const liveSecretYAML = `apiVersion: v1
kind: Secret
metadata:
  name: db-creds
  namespace: prod
  uid: 8c3b1f52-0000-4000-8000-000000000007
  labels:
    team: payments
type: kubernetes.io/basic-auth
data:
  username: YWRtaW4=
  password: aHVudGVyMi1zdXBlci1zZWNyZXQ=
stringData:
  token: plaintext-bearer-token
`

// TestStripToTemplate_PerKind asserts, for each kind that gets specific
// handling, that the server-set fields are gone AND that a meaningful,
// user-authored field survived. The "kept" half is what catches an
// over-broad strip.
func TestStripToTemplate_PerKind(t *testing.T) {
	tests := []struct {
		name string
		in   string
		gone [][]string
		kept [][]string
	}{
		{
			name: "Pod",
			in:   livePodYAML,
			gone: [][]string{
				{"status"},
				{"metadata", "uid"},
				{"metadata", "resourceVersion"},
				{"metadata", "generation"},
				{"metadata", "creationTimestamp"},
				{"metadata", "managedFields"},
				{"metadata", "selfLink"},
				{"metadata", "ownerReferences"},
				{"metadata", "deletionTimestamp"},
				{"metadata", "deletionGracePeriodSeconds"},
				{"metadata", "namespace"},
				{"metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration"},
				{"spec", "nodeName"},
				{"spec", "serviceAccount"},
			},
			kept: [][]string{
				{"metadata", "name"},
				{"metadata", "labels", "app"},
				{"metadata", "annotations", "team"},
				{"spec", "serviceAccountName"},
			},
		},
		{
			name: "Service",
			in:   liveServiceYAML,
			gone: [][]string{
				{"status"},
				{"spec", "clusterIP"},
				{"spec", "clusterIPs"},
				{"spec", "ipFamilies"},
				{"spec", "ipFamilyPolicy"},
			},
			kept: [][]string{
				{"spec", "type"},
				{"spec", "selector", "app"},
				{"spec", "ports"},
			},
		},
		{
			name: "PersistentVolumeClaim",
			in:   livePVCYAML,
			gone: [][]string{
				{"status"},
				{"spec", "volumeName"},
				{"metadata", "annotations", "pv.kubernetes.io/bind-completed"},
				{"metadata", "annotations", "pv.kubernetes.io/bound-by-controller"},
				{"metadata", "annotations", "volume.beta.kubernetes.io/storage-provisioner"},
				{"metadata", "annotations", "volume.kubernetes.io/storage-provisioner"},
				{"metadata", "annotations", "volume.kubernetes.io/selected-node"},
			},
			kept: [][]string{
				{"spec", "storageClassName"},
				{"spec", "accessModes"},
				{"spec", "resources", "requests", "storage"},
				{"metadata", "annotations", "team"},
			},
		},
		{
			name: "Job",
			in:   liveJobYAML,
			gone: [][]string{
				{"status"},
				{"spec", "selector"},
				{"metadata", "labels", "controller-uid"},
				{"metadata", "labels", "batch.kubernetes.io/controller-uid"},
				{"metadata", "labels", "job-name"},
				{"metadata", "labels", "batch.kubernetes.io/job-name"},
				{"spec", "template", "metadata", "labels", "controller-uid"},
				{"spec", "template", "metadata", "labels", "batch.kubernetes.io/controller-uid"},
				{"spec", "template", "metadata", "creationTimestamp"},
			},
			kept: [][]string{
				{"spec", "completions"},
				{"metadata", "labels", "team"},
				{"spec", "template", "metadata", "labels", "team"},
				{"spec", "template", "spec", "containers"},
			},
		},
		{
			name: "Secret",
			in:   liveSecretYAML,
			gone: [][]string{
				{"status"},
				{"metadata", "uid"},
				{"metadata", "namespace"},
			},
			kept: [][]string{
				// The shape of a Secret is its keys and its type. Both survive;
				// only the values are blanked — see the redaction test below.
				{"type"},
				{"data", "username"},
				{"data", "password"},
				{"stringData", "token"},
				{"metadata", "labels", "team"},
			},
		},
		{
			name: "CronJob",
			in:   liveCronJobYAML,
			gone: [][]string{
				{"status"},
				{"metadata", "uid"},
				{"metadata", "creationTimestamp"},
				{"metadata", "namespace"},
				// Both ObjectMetas the serializer fills in: the JobTemplateSpec's
				// own, and the pod template's inside it.
				{"spec", "jobTemplate", "metadata", "creationTimestamp"},
				{"spec", "jobTemplate", "spec", "template", "metadata", "creationTimestamp"},
			},
			kept: [][]string{
				{"spec", "schedule"},
				{"metadata", "labels", "team"},
				{"spec", "jobTemplate", "spec", "template", "spec", "containers"},
			},
		},
		{
			name: "Deployment falls back to the generic strip",
			in:   liveDeploymentYAML,
			gone: [][]string{
				{"status"},
				{"metadata", "generation"},
				{"metadata", "annotations", "deployment.kubernetes.io/revision"},
				{"spec", "template", "metadata", "creationTimestamp"},
			},
			kept: [][]string{
				{"spec", "replicas"},
				{"spec", "selector", "matchLabels", "app"},
				{"spec", "template", "spec", "containers"},
				{"metadata", "annotations", "team"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := StripToTemplateWith(tc.in, DefaultTemplateStripSet())
			require.NoError(t, err)
			for _, p := range tc.gone {
				assert.False(t, hasPath(t, out, p...), "expected %v to be stripped", p)
			}
			for _, p := range tc.kept {
				assert.True(t, hasPath(t, out, p...), "expected %v to survive", p)
			}
		})
	}
}

// TestStripToTemplate_DropsInjectedServiceAccountVolume asserts the projected
// token volume the API server injects into every Pod is removed together with
// the volumeMount that references it — leaving one without the other produces
// a manifest the API server rejects.
func TestStripToTemplate_DropsInjectedServiceAccountVolume(t *testing.T) {
	out, err := StripToTemplateWith(livePodYAML, DefaultTemplateStripSet())
	require.NoError(t, err)

	var pod corev1.Pod
	require.NoError(t, sigsyaml.UnmarshalStrict([]byte(out), &pod))

	require.Len(t, pod.Spec.Volumes, 1)
	assert.Equal(t, "config", pod.Spec.Volumes[0].Name)
	require.Len(t, pod.Spec.Containers, 1)
	require.Len(t, pod.Spec.Containers[0].VolumeMounts, 1)
	assert.Equal(t, "config", pod.Spec.Containers[0].VolumeMounts[0].Name)
}

// TestStripToTemplate_NoServerAssignedIdentifiersSurvive is the AC #4 guard.
// Two halves:
//
//   - No server-assigned identifier value survives anywhere in the document.
//     Matching on values rather than keys means the sweep cannot be fooled by
//     a field that moves, and cannot false-positive on a user-authored key
//     that happens to share a name with a server-set one.
//   - The result still round-trips into the typed API struct with
//     unknown-field checking on, so a strip that mangles the document shape
//     fails here rather than at the API server.
func TestStripToTemplate_NoServerAssignedIdentifiersSurvive(t *testing.T) {
	// Identifiers only an API server, scheduler, or controller can assign:
	// UIDs, resourceVersions, allocated cluster IPs, the scheduled node, the
	// bound PV name, and the injected token volume name.
	serverAssigned := []string{
		"8c3b1f52-",
		"918273",
		"10.96.4.11",
		"ip-10-0-1-7",
		"pvc-8c3b1f52",
		"kube-api-access-x7f2q",
		"last-applied-configuration",
	}
	tests := []struct {
		name string
		in   string
		into any
	}{
		{"Pod", livePodYAML, &corev1.Pod{}},
		{"Service", liveServiceYAML, &corev1.Service{}},
		{"PersistentVolumeClaim", livePVCYAML, &corev1.PersistentVolumeClaim{}},
		{"Job", liveJobYAML, &batchv1.Job{}},
		{"CronJob", liveCronJobYAML, &batchv1.CronJob{}},
		{"Secret", liveSecretYAML, &corev1.Secret{}},
		{"Deployment", liveDeploymentYAML, &appsv1.Deployment{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := StripToTemplateWith(tc.in, DefaultTemplateStripSet())
			require.NoError(t, err)
			for _, id := range serverAssigned {
				assert.NotContains(t, out, id, "server-assigned identifier survived the strip")
			}
			require.NoError(t, sigsyaml.UnmarshalStrict([]byte(out), tc.into))
		})
	}
}

// TestStripToTemplate_DropsEmptiedAnnotationMap asserts that a metadata map
// left empty by the strip is removed rather than emitted as `annotations: {}`.
func TestStripToTemplate_DropsEmptiedAnnotationMap(t *testing.T) {
	in := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: '{}'
data:
  key: value
`
	out, err := StripToTemplateWith(in, DefaultTemplateStripSet())
	require.NoError(t, err)
	assert.False(t, hasPath(t, out, "metadata", "annotations"), "an annotation map emptied by the strip must be dropped")
	assert.True(t, hasPath(t, out, "data", "key"))
}

// TestStripToTemplate_RejectsNonObject guards the boundary: the export action
// hands cluster-sourced text to this function, and a document that is not a
// mapping must produce an error rather than a nil-map panic.
func TestStripToTemplate_RejectsNonObject(t *testing.T) {
	_, err := StripToTemplateWith("- just\n- a\n- list\n", DefaultTemplateStripSet())
	assert.Error(t, err)
}

// TestStripToTemplate_SecretKeepsKeysDropsValues: a template is a shape, not a
// payload. All three export destinations persist, so the live credential must
// not travel with the keys. The type is authored, not server-set, and stays —
// a kubernetes.io/tls Secret is a different template from an Opaque one.
func TestStripToTemplate_SecretKeepsKeysDropsValues(t *testing.T) {
	out, err := StripToTemplateWith(liveSecretYAML, DefaultTemplateStripSet())
	require.NoError(t, err)

	// Assert on the payload itself, not only on the decoded value: a value
	// that reads as empty while the ciphertext lingers elsewhere in the
	// document is a different failure from a value that was never blanked.
	for _, secret := range []string{"YWRtaW4=", "aHVudGVyMi1zdXBlci1zZWNyZXQ=", "plaintext-bearer-token"} {
		assert.NotContains(t, out, secret, "a live Secret value survived the strip")
	}

	var decoded corev1.Secret
	require.NoError(t, sigsyaml.UnmarshalStrict([]byte(out), &decoded))

	assert.Equal(t, corev1.SecretTypeBasicAuth, decoded.Type)
	require.Len(t, decoded.Data, 2)
	for key, value := range decoded.Data {
		assert.Empty(t, value, "data[%s] must be blank", key)
	}
	require.Len(t, decoded.StringData, 1)
	assert.Empty(t, decoded.StringData["token"])
}

// TestTemplateRedactsValues ties the caller-facing predicate to the strip: the
// export tells the user its values were redacted based on this answer.
func TestTemplateRedactsValues(t *testing.T) {
	assert.True(t, TemplateRedactsValues("Secret", DefaultTemplateStripSet()))
	assert.False(t, TemplateRedactsValues("ConfigMap", DefaultTemplateStripSet()))
	assert.False(t, TemplateRedactsValues("Pod", DefaultTemplateStripSet()))
}
