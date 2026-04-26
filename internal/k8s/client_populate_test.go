package k8s

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- populateResourceDetails: Pod ---

func TestPopulateResourceDetails_Pod(t *testing.T) {
	tests := []struct {
		name       string
		obj        map[string]interface{}
		wantReady  string
		wantRestr  string
		wantStatus string
		wantCols   map[string]string
	}{
		{
			name: "running pod with all fields",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "app",
							"image": "nginx:1.25",
							"resources": map[string]interface{}{
								"requests": map[string]interface{}{"cpu": "100m", "memory": "128Mi"},
								"limits":   map[string]interface{}{"cpu": "500m", "memory": "256Mi"},
							},
						},
					},
					"nodeName":           "node-1",
					"serviceAccountName": "default",
					"priorityClassName":  "high-priority",
				},
				"status": map[string]interface{}{
					"phase":    "Running",
					"qosClass": "Burstable",
					"podIP":    "10.0.0.5",
					"containerStatuses": []interface{}{
						map[string]interface{}{
							"name":         "app",
							"ready":        true,
							"restartCount": float64(0),
						},
					},
				},
			},
			wantReady: "1/1",
			wantRestr: "0",
			wantCols: map[string]string{
				"CPU Req":         "100m",
				"CPU Lim":         "500m",
				"Mem Req":         "128Mi",
				"Mem Lim":         "256Mi",
				"QoS":             "Burstable",
				"Service Account": "default",
				"Pod IP":          "10.0.0.5",
				"Images":          "nginx:1.25",
				"Priority Class":  "high-priority",
				"Node":            "node-1",
			},
		},
		{
			name: "pod with nil status returns early",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "app"},
					},
				},
			},
			wantReady: "",
			wantRestr: "",
		},
		{
			name: "pod with multiple containers, partial readiness",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "app", "image": "app:v1"},
						map[string]interface{}{"name": "sidecar", "image": "envoy:v1"},
					},
				},
				"status": map[string]interface{}{
					"containerStatuses": []interface{}{
						map[string]interface{}{
							"name":         "app",
							"ready":        true,
							"restartCount": float64(2),
						},
						map[string]interface{}{
							"name":         "sidecar",
							"ready":        false,
							"restartCount": float64(1),
							"state": map[string]interface{}{
								"waiting": map[string]interface{}{
									"reason": "CrashLoopBackOff",
								},
							},
						},
					},
				},
			},
			wantReady:  "1/2",
			wantRestr:  "3",
			wantStatus: "CrashLoopBackOff",
		},
		{
			name: "pod with int64 restartCount",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "app"},
					},
				},
				"status": map[string]interface{}{
					"containerStatuses": []interface{}{
						map[string]interface{}{
							"name":         "app",
							"ready":        true,
							"restartCount": int64(7),
						},
					},
				},
			},
			wantReady: "1/1",
			wantRestr: "7",
		},
		{
			name: "succeeded pod stays succeeded even with unready containers",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "job"},
					},
				},
				"status": map[string]interface{}{
					"containerStatuses": []interface{}{
						map[string]interface{}{
							"name":         "job",
							"ready":        false,
							"restartCount": float64(0),
						},
					},
				},
			},
			wantReady: "0/1",
			wantRestr: "0",
		},
		{
			name: "pod with init container failure overrides status",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "app"},
					},
				},
				"status": map[string]interface{}{
					"containerStatuses": []interface{}{
						map[string]interface{}{
							"name":         "app",
							"ready":        false,
							"restartCount": float64(0),
							"state": map[string]interface{}{
								"waiting": map[string]interface{}{
									"reason": "PodInitializing",
								},
							},
						},
					},
					"initContainerStatuses": []interface{}{
						map[string]interface{}{
							"name":  "init-db",
							"ready": false,
							"state": map[string]interface{}{
								"terminated": map[string]interface{}{
									"reason": "Error",
								},
							},
						},
					},
				},
			},
			wantReady:  "0/1",
			wantRestr:  "0",
			wantStatus: "Error",
		},
		{
			name: "running pod with unready container and no reason gets NotReady",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "app"},
					},
				},
				"status": map[string]interface{}{
					"containerStatuses": []interface{}{
						map[string]interface{}{
							"name":         "app",
							"ready":        false,
							"restartCount": float64(0),
						},
					},
				},
			},
			wantReady: "0/1",
			wantRestr: "0",
		},
		{
			name: "non-map containerStatuses entries are skipped",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "app"},
					},
				},
				"status": map[string]interface{}{
					"containerStatuses": []interface{}{
						"not-a-map",
						map[string]interface{}{
							"name":         "app",
							"ready":        true,
							"restartCount": float64(0),
						},
					},
				},
			},
			wantReady: "1/1",
			wantRestr: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{Status: "Running"}
			populateResourceDetails(ti, tt.obj, "Pod")

			if tt.wantReady != "" {
				assert.Equal(t, tt.wantReady, ti.Ready)
			}
			if tt.wantRestr != "" {
				assert.Equal(t, tt.wantRestr, ti.Restarts)
			}
			if tt.wantStatus != "" {
				assert.Equal(t, tt.wantStatus, ti.Status)
			}
			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateResourceDetails: Deployment ---

func TestPopulateResourceDetails_Deployment(t *testing.T) {
	tests := []struct {
		name      string
		obj       map[string]interface{}
		wantReady string
		wantCols  map[string]string
	}{
		{
			name: "fully available deployment",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": float64(3),
					"strategy": map[string]interface{}{
						"type": "RollingUpdate",
					},
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"image": "myapp:v1",
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{"cpu": "100m"},
									},
								},
							},
						},
					},
				},
				"status": map[string]interface{}{
					"readyReplicas":     float64(3),
					"updatedReplicas":   float64(3),
					"availableReplicas": float64(3),
				},
			},
			wantReady: "3/3",
			wantCols: map[string]string{
				"Replicas":   "3",
				"Strategy":   "RollingUpdate",
				"Up-to-date": "3",
				"Available":  "3",
				"CPU Req":    "100m",
				"Images":     "myapp:v1",
			},
		},
		{
			name: "deployment with int64 replicas",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": int64(2),
				},
				"status": map[string]interface{}{
					"readyReplicas": int64(1),
				},
			},
			wantReady: "1/2",
		},
		{
			name: "nil status returns early",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": float64(3),
				},
			},
			wantReady: "",
		},
		{
			name: "nil spec returns early",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"readyReplicas": float64(1),
				},
			},
			wantReady: "",
		},
		{
			name: "deployment defaults to 1 replica when not specified",
			obj: map[string]interface{}{
				"spec":   map[string]interface{}{},
				"status": map[string]interface{}{},
			},
			wantReady: "0/1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "Deployment")

			if tt.wantReady != "" {
				assert.Equal(t, tt.wantReady, ti.Ready)
			}
			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateResourceDetails: StatefulSet ---

func TestPopulateResourceDetails_StatefulSet(t *testing.T) {
	tests := []struct {
		name      string
		obj       map[string]interface{}
		wantReady string
		wantCols  map[string]string
	}{
		{
			name: "ready statefulset",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": float64(3),
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{"image": "redis:7"},
							},
						},
					},
				},
				"status": map[string]interface{}{
					"readyReplicas": float64(3),
				},
			},
			wantReady: "3/3",
			wantCols: map[string]string{
				"Replicas": "3",
				"Images":   "redis:7",
			},
		},
		{
			name: "statefulset with int64 replicas",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": int64(5),
				},
				"status": map[string]interface{}{
					"readyReplicas": int64(3),
				},
			},
			wantReady: "3/5",
		},
		{
			name: "nil status returns early",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{"replicas": float64(1)},
			},
			wantReady: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "StatefulSet")

			if tt.wantReady != "" {
				assert.Equal(t, tt.wantReady, ti.Ready)
			}
			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateResourceDetails: DaemonSet ---

func TestPopulateResourceDetails_DaemonSet(t *testing.T) {
	tests := []struct {
		name      string
		obj       map[string]interface{}
		wantReady string
		wantCols  map[string]string
	}{
		{
			name: "fully scheduled daemonset",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{"cpu": "50m"},
									},
								},
							},
						},
					},
				},
				"status": map[string]interface{}{
					"desiredNumberScheduled": float64(5),
					"numberReady":            float64(5),
				},
			},
			wantReady: "5/5",
			wantCols: map[string]string{
				"Desired": "5",
				"CPU Req": "50m",
			},
		},
		{
			name: "daemonset with int64 values",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"desiredNumberScheduled": int64(3),
					"numberReady":            int64(2),
				},
			},
			wantReady: "2/3",
		},
		{
			name: "nil status returns early",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			wantReady: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "DaemonSet")

			if tt.wantReady != "" {
				assert.Equal(t, tt.wantReady, ti.Ready)
			}
			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateResourceDetails: ReplicaSet ---

func TestPopulateResourceDetails_ReplicaSet(t *testing.T) {
	tests := []struct {
		name      string
		obj       map[string]interface{}
		wantReady string
	}{
		{
			name: "replicaset with float64 values",
			obj: map[string]interface{}{
				"spec":   map[string]interface{}{"replicas": float64(3)},
				"status": map[string]interface{}{"readyReplicas": float64(2)},
			},
			wantReady: "2/3",
		},
		{
			name: "replicaset with int64 values",
			obj: map[string]interface{}{
				"spec":   map[string]interface{}{"replicas": int64(4)},
				"status": map[string]interface{}{"readyReplicas": int64(4)},
			},
			wantReady: "4/4",
		},
		{
			name: "nil status returns early",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{"replicas": float64(1)},
			},
			wantReady: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "ReplicaSet")

			if tt.wantReady != "" {
				assert.Equal(t, tt.wantReady, ti.Ready)
			}
		})
	}
}

// --- populateResourceDetails: Service ---

func TestPopulateResourceDetails_Service(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "ClusterIP service with ports and selector",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type":      "ClusterIP",
					"clusterIP": "10.96.0.1",
					"ports": []interface{}{
						map[string]interface{}{
							"port":     int64(80),
							"protocol": "TCP",
						},
					},
					"selector": map[string]interface{}{
						"app": "nginx",
					},
				},
			},
			wantCols: map[string]string{
				"Type":       "ClusterIP",
				"Cluster IP": "10.96.0.1",
				"Ports":      "80/TCP",
				"Selector":   "app=nginx",
			},
		},
		{
			name: "service with targetPort different from port",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type": "ClusterIP",
					"ports": []interface{}{
						map[string]interface{}{
							"port":       float64(80),
							"targetPort": float64(8080),
							"protocol":   "TCP",
						},
					},
				},
			},
			wantCols: map[string]string{
				"Ports": "80\u21928080/TCP",
			},
		},
		{
			name: "NodePort service shows nodePort like kubectl",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type": "NodePort",
					"ports": []interface{}{
						map[string]interface{}{
							"port":     int64(80),
							"nodePort": int64(30001),
							"protocol": "TCP",
						},
					},
				},
			},
			wantCols: map[string]string{
				"Ports": "80:30001/TCP",
			},
		},
		{
			name: "NodePort service with targetPort combines both formats",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type": "NodePort",
					"ports": []interface{}{
						map[string]interface{}{
							"port":       float64(80),
							"targetPort": float64(8080),
							"nodePort":   float64(30001),
							"protocol":   "TCP",
						},
					},
				},
			},
			wantCols: map[string]string{
				"Ports": "80:30001\u21928080/TCP",
			},
		},
		{
			name: "LoadBalancer service with nodePort shows nodePort",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type": "LoadBalancer",
					"ports": []interface{}{
						map[string]interface{}{
							"port":     int64(443),
							"nodePort": int64(31443),
							"protocol": "TCP",
						},
					},
				},
			},
			wantCols: map[string]string{
				"Ports": "443:31443/TCP",
			},
		},
		{
			name: "multiple ports mix with and without nodePort",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type": "NodePort",
					"ports": []interface{}{
						map[string]interface{}{
							"port":     int64(80),
							"nodePort": int64(30080),
							"protocol": "TCP",
						},
						map[string]interface{}{
							"port":     int64(443),
							"protocol": "TCP",
						},
					},
				},
			},
			wantCols: map[string]string{
				"Ports": "80:30080/TCP, 443/TCP",
			},
		},
		{
			name: "LoadBalancer service with external address",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type": "LoadBalancer",
				},
				"status": map[string]interface{}{
					"loadBalancer": map[string]interface{}{
						"ingress": []interface{}{
							map[string]interface{}{"ip": "1.2.3.4"},
						},
					},
				},
			},
			wantCols: map[string]string{
				"Type":             "LoadBalancer",
				"External Address": "1.2.3.4",
			},
		},
		{
			name: "LoadBalancer with hostname",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type": "LoadBalancer",
				},
				"status": map[string]interface{}{
					"loadBalancer": map[string]interface{}{
						"ingress": []interface{}{
							map[string]interface{}{"hostname": "elb.example.com"},
						},
					},
				},
			},
			wantCols: map[string]string{
				"External Address": "elb.example.com",
			},
		},
		{
			name: "service with externalIPs",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type":        "ClusterIP",
					"externalIPs": []interface{}{"5.6.7.8"},
				},
			},
			wantCols: map[string]string{
				"External IPs": "5.6.7.8",
			},
		},
		{
			name: "service with sessionAffinity",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type":            "ClusterIP",
					"sessionAffinity": "ClientIP",
				},
			},
			wantCols: map[string]string{
				"Session Affinity": "ClientIP",
			},
		},
		{
			name: "service with sessionAffinity None is omitted",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"type":            "ClusterIP",
					"sessionAffinity": "None",
				},
			},
			wantCols: map[string]string{
				"Type": "ClusterIP",
			},
		},
		{
			name: "nil spec returns early",
			obj:  map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "Service")

			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateResourceDetails: Ingress ---

func TestPopulateResourceDetails_Ingress(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "ingress with rules and TLS",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"ingressClassName": "nginx",
					"rules": []interface{}{
						map[string]interface{}{
							"host": "example.com",
							"http": map[string]interface{}{
								"paths": []interface{}{
									map[string]interface{}{
										"path": "/api",
									},
								},
							},
						},
					},
					"tls": []interface{}{
						map[string]interface{}{
							"hosts": []interface{}{"example.com"},
						},
					},
				},
			},
			wantCols: map[string]string{
				"Ingress Class": "nginx",
				"Rules":         "1",
				"Hosts":         "example.com",
				"TLS Hosts":     "example.com",
				"__ingress_url": "https://example.com/api",
			},
		},
		{
			name: "ingress without TLS uses http scheme",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"rules": []interface{}{
						map[string]interface{}{
							"host": "example.com",
						},
					},
				},
			},
			wantCols: map[string]string{
				"__ingress_url": "http://example.com",
			},
		},
		{
			name: "ingress with default backend (numeric port)",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"defaultBackend": map[string]interface{}{
						"service": map[string]interface{}{
							"name": "my-svc",
							"port": map[string]interface{}{
								"number": float64(8080),
							},
						},
					},
				},
			},
			wantCols: map[string]string{
				"Default Backend": "my-svc:8080",
			},
		},
		{
			name: "ingress with default backend (named port)",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"defaultBackend": map[string]interface{}{
						"service": map[string]interface{}{
							"name": "my-svc",
							"port": map[string]interface{}{
								"name": "http",
							},
						},
					},
				},
			},
			wantCols: map[string]string{
				"Default Backend": "my-svc:http",
			},
		},
		{
			name: "ingress with LB address from status",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{},
				"status": map[string]interface{}{
					"loadBalancer": map[string]interface{}{
						"ingress": []interface{}{
							map[string]interface{}{"ip": "10.0.0.1"},
						},
					},
				},
			},
			wantCols: map[string]string{
				"Address": "10.0.0.1",
			},
		},
		{
			name: "nil spec returns early",
			obj:  map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "Ingress")

			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateResourceDetails: ConfigMap ---

func TestPopulateResourceDetails_ConfigMap(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "configmap with data keys",
			obj: map[string]interface{}{
				"data": map[string]interface{}{
					"config.yaml": "key: value",
					"app.conf":    "port=8080",
				},
			},
			wantCols: map[string]string{
				"data:app.conf":    "port=8080",
				"data:config.yaml": "key: value",
			},
		},
		{
			name: "configmap with no data",
			obj:  map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "ConfigMap")

			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			} else {
				assert.Empty(t, ti.Columns)
			}
		})
	}
}

// --- populateResourceDetails: Secret ---

func TestPopulateResourceDetails_Secret(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("my-password"))

	tests := []struct {
		name     string
		obj      map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "secret with decoded data and type",
			obj: map[string]interface{}{
				"data": map[string]interface{}{
					"password": encoded,
				},
				"type": "Opaque",
			},
			wantCols: map[string]string{
				"secret:password": "my-password",
				"Type":            "Opaque",
			},
		},
		{
			name: "secret with invalid base64 skips that key",
			obj: map[string]interface{}{
				"data": map[string]interface{}{
					"bad": "!!!not-base64!!!",
				},
				"type": "Opaque",
			},
			wantCols: map[string]string{
				"Type": "Opaque",
			},
		},
		{
			name: "secret with no data",
			obj: map[string]interface{}{
				"type": "kubernetes.io/service-account-token",
			},
			wantCols: map[string]string{
				"Type": "kubernetes.io/service-account-token",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "Secret")

			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateResourceDetails: Node ---

func TestPopulateResourceDetails_Node(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "node with roles, addresses, nodeInfo, and taints",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"node-role.kubernetes.io/control-plane": "",
						"node-role.kubernetes.io/worker":        "",
					},
				},
				"spec": map[string]interface{}{
					"taints": []interface{}{
						map[string]interface{}{
							"key":    "node-role.kubernetes.io/control-plane",
							"effect": "NoSchedule",
						},
					},
				},
				"status": map[string]interface{}{
					"addresses": []interface{}{
						map[string]interface{}{
							"type":    "InternalIP",
							"address": "192.168.1.10",
						},
					},
					"allocatable": map[string]interface{}{
						"cpu":    "4",
						"memory": "16Gi",
					},
					"nodeInfo": map[string]interface{}{
						"kubeletVersion":          "v1.29.0",
						"osImage":                 "Ubuntu 22.04 LTS",
						"containerRuntimeVersion": "containerd://1.7.0",
					},
				},
			},
			wantCols: map[string]string{
				"Role":       "control-plane,worker",
				"InternalIP": "192.168.1.10",
				"CPU Alloc":  "4",
				"Mem Alloc":  "16Gi",
				"Version":    "v1.29.0",
				"OS":         "Ubuntu 22.04 LTS",
				"Runtime":    "containerd://1.7.0",
				"Taints":     "node-role.kubernetes.io/control-plane:NoSchedule",
			},
		},
		{
			name: "node with taint that has a value",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{},
				"spec": map[string]interface{}{
					"taints": []interface{}{
						map[string]interface{}{
							"key":    "special",
							"value":  "true",
							"effect": "NoExecute",
						},
					},
				},
				"status": map[string]interface{}{},
			},
			wantCols: map[string]string{
				"Taints": "special=true:NoExecute",
			},
		},
		{
			name: "node without roles produces no role column",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"kubernetes.io/hostname": "node-1",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "Node")

			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateResourceDetails: PersistentVolumeClaim ---

func TestPopulateResourceDetails_PVC(t *testing.T) {
	tests := []struct {
		name       string
		obj        map[string]interface{}
		wantStatus string
		wantCols   map[string]string
	}{
		{
			name: "bound PVC with all fields",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"storage": "10Gi",
						},
					},
					"volumeName":       "pv-abc",
					"accessModes":      []interface{}{"ReadWriteOnce"},
					"storageClassName": "gp3",
					"volumeMode":       "Filesystem",
				},
				"status": map[string]interface{}{
					"phase": "Bound",
					"capacity": map[string]interface{}{
						"storage": "10Gi",
					},
				},
			},
			wantStatus: "Bound",
			wantCols: map[string]string{
				"Capacity":      "10Gi",
				"Request":       "10Gi",
				"Volume":        "pv-abc",
				"Access Modes":  "ReadWriteOnce",
				"Storage Class": "gp3",
				"Volume Mode":   "Filesystem",
			},
		},
		{
			name: "pending PVC with no status capacity",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"storage": "5Gi",
						},
					},
				},
				"status": map[string]interface{}{
					"phase": "Pending",
				},
			},
			wantStatus: "Pending",
			wantCols: map[string]string{
				"Request": "5Gi",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "PersistentVolumeClaim")

			if tt.wantStatus != "" {
				assert.Equal(t, tt.wantStatus, ti.Status)
			}
			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateResourceDetails: CronJob ---

func TestPopulateResourceDetails_CronJob(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "cronjob with schedule and last schedule",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"schedule": "*/5 * * * *",
					"suspend":  false,
				},
				"status": map[string]interface{}{
					"lastScheduleTime": "2025-01-15T10:30:00Z",
				},
			},
			wantCols: map[string]string{
				"Schedule":      "*/5 * * * *",
				"Suspend":       "false",
				"Last Schedule": "2025-01-15T10:30:00Z",
			},
		},
		{
			name: "suspended cronjob",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"schedule": "0 0 * * *",
					"suspend":  true,
				},
			},
			wantCols: map[string]string{
				"Schedule": "0 0 * * *",
				"Suspend":  "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "CronJob")

			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

func TestPopulateResourceDetails_CronJob_Next(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		wantNext bool
	}{
		{
			name: "valid schedule, not suspended",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"schedule": "*/5 * * * *",
					"suspend":  false,
				},
			},
			wantNext: true,
		},
		{
			name: "suspended cronjob skips next",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"schedule": "*/5 * * * *",
					"suspend":  true,
				},
			},
			wantNext: false,
		},
		{
			name: "invalid schedule skips next",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"schedule": "not a cron expression",
					"suspend":  false,
				},
			},
			wantNext: false,
		},
		{
			name: "valid schedule with valid timezone",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"schedule": "0 0 * * *",
					"timeZone": "America/New_York",
					"suspend":  false,
				},
			},
			wantNext: true,
		},
		{
			name: "invalid timezone skips next",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"schedule": "0 0 * * *",
					"timeZone": "Not/A_Real_Zone",
					"suspend":  false,
				},
			},
			wantNext: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "CronJob")
			val, hasNext := columnsToMap(ti.Columns)["Next"]
			assert.Equal(t, tt.wantNext, hasNext, "Next column presence mismatch")
			if hasNext {
				// Verify the value is a formatAge-shaped duration string
				// (digits followed by s/m/h/d/y) so a regression that
				// drops formatAge or changes the key would surface here,
				// not just at runtime.
				assert.Regexp(t, `^\d+[smhdy]$`, val,
					"Next column value must be a formatAge duration like 4m, 2h, 3d")
			}
		})
	}
}

// --- populateResourceDetails: Job ---

func TestPopulateResourceDetails_Job(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "completed job",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"completions": float64(3),
				},
				"status": map[string]interface{}{
					"succeeded": float64(3),
				},
			},
			wantCols: map[string]string{
				"Succeeded":   "3",
				"Completions": "3",
			},
		},
		{
			name: "failed job",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"completions": float64(1),
				},
				"status": map[string]interface{}{
					"succeeded": float64(0),
					"failed":    float64(3),
				},
			},
			wantCols: map[string]string{
				"Succeeded":   "0",
				"Failed":      "3",
				"Completions": "1",
			},
		},
		{
			name: "job with zero failures omits failed column",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"succeeded": float64(1),
					"failed":    float64(0),
				},
			},
			wantCols: map[string]string{
				"Succeeded": "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "Job")

			colMap := columnsToMap(ti.Columns)
			for k, v := range tt.wantCols {
				assert.Equal(t, v, colMap[k], "column %q mismatch", k)
			}
			// Verify failed column is absent when expected.
			if tt.name == "job with zero failures omits failed column" {
				_, hasFailed := colMap["Failed"]
				assert.False(t, hasFailed, "Failed column should not be present for zero failures")
			}
		})
	}
}

// --- populateResourceDetails: HPA ---

func TestPopulateResourceDetails_HPA(t *testing.T) {
	tests := []struct {
		name      string
		obj       map[string]interface{}
		wantReady string
		wantCols  map[string]string
	}{
		{
			name: "HPA with resource metrics and conditions",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"minReplicas": float64(2),
					"maxReplicas": float64(10),
					"scaleTargetRef": map[string]interface{}{
						"kind": "Deployment",
						"name": "my-app",
					},
					"metrics": []interface{}{
						map[string]interface{}{
							"type": "Resource",
							"resource": map[string]interface{}{
								"name": "cpu",
								"target": map[string]interface{}{
									"type":               "Utilization",
									"averageUtilization": float64(80),
								},
							},
						},
					},
				},
				"status": map[string]interface{}{
					"currentReplicas": float64(3),
					"desiredReplicas": float64(4),
					"currentMetrics": []interface{}{
						map[string]interface{}{
							"type": "Resource",
							"resource": map[string]interface{}{
								"name": "cpu",
								"current": map[string]interface{}{
									"averageUtilization": float64(75),
								},
							},
						},
					},
				},
			},
			wantReady: "3/4 (2-10)",
			wantCols: map[string]string{
				"Target":           "Deployment/my-app",
				"Min Replicas":     "2",
				"Max Replicas":     "10",
				"Target Cpu":       "80%",
				"Current Replicas": "3",
				"Desired Replicas": "4",
				"Current Cpu":      "75%",
			},
		},
		{
			name: "HPA with Pods metric type",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"minReplicas": float64(1),
					"maxReplicas": float64(5),
					"metrics": []interface{}{
						map[string]interface{}{
							"type": "Pods",
							"pods": map[string]interface{}{
								"metric": map[string]interface{}{
									"name": "requests_per_second",
								},
								"target": map[string]interface{}{
									"averageValue": "100",
								},
							},
						},
					},
				},
				"status": map[string]interface{}{
					"currentReplicas": float64(2),
					"desiredReplicas": float64(2),
					"currentMetrics": []interface{}{
						map[string]interface{}{
							"type": "Pods",
							"pods": map[string]interface{}{
								"metric": map[string]interface{}{
									"name": "requests_per_second",
								},
								"current": map[string]interface{}{
									"averageValue": "85",
								},
							},
						},
					},
				},
			},
			wantCols: map[string]string{
				"Target requests_per_second":  "100",
				"Current requests_per_second": "85",
			},
		},
		{
			name: "HPA with Object metric type",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"maxReplicas": float64(10),
					"metrics": []interface{}{
						map[string]interface{}{
							"type": "Object",
							"object": map[string]interface{}{
								"metric": map[string]interface{}{
									"name": "queue_depth",
								},
								"target": map[string]interface{}{
									"value": "50",
								},
							},
						},
					},
				},
				"status": map[string]interface{}{
					"currentReplicas": float64(1),
					"desiredReplicas": float64(1),
				},
			},
			wantCols: map[string]string{
				"Target queue_depth": "50",
			},
		},
		{
			name: "HPA with AverageValue resource metric",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"maxReplicas": float64(5),
					"metrics": []interface{}{
						map[string]interface{}{
							"type": "Resource",
							"resource": map[string]interface{}{
								"name": "memory",
								"target": map[string]interface{}{
									"type":         "AverageValue",
									"averageValue": "500Mi",
								},
							},
						},
					},
				},
				"status": map[string]interface{}{
					"currentReplicas": float64(2),
					"desiredReplicas": float64(2),
					"currentMetrics": []interface{}{
						map[string]interface{}{
							"type": "Resource",
							"resource": map[string]interface{}{
								"name": "memory",
								"current": map[string]interface{}{
									"averageValue": "450Mi",
								},
							},
						},
					},
				},
			},
			wantCols: map[string]string{
				"Target Memory":  "500Mi",
				"Current Memory": "450Mi",
			},
		},
		{
			name: "HPA with ScalingLimited condition",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"maxReplicas": float64(3),
				},
				"status": map[string]interface{}{
					"currentReplicas": float64(3),
					"desiredReplicas": float64(3),
					"conditions": []interface{}{
						map[string]interface{}{
							"type":    "ScalingLimited",
							"status":  "True",
							"message": "desired replica count limited to 3",
						},
					},
				},
			},
			wantCols: map[string]string{
				"Scaling Limited": "desired replica count limited to 3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "HorizontalPodAutoscaler")

			if tt.wantReady != "" {
				assert.Equal(t, tt.wantReady, ti.Ready)
			}
			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateResourceDetails: default dispatches to ext ---

func TestPopulateResourceDetails_DefaultDispatchesToExt(t *testing.T) {
	// Unknown kinds should dispatch to populateResourceDetailsExt,
	// which handles generic CRD resources with status fields.
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Active",
		},
	}
	ti := &model.Item{}
	populateResourceDetails(ti, obj, "UnknownCustomResource")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "Active", colMap["Phase"])
}
