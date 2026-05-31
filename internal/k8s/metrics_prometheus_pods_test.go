package k8s

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func TestBuildPromPodQuery(t *testing.T) {
	cpuNS := buildPromPodQuery("dev", "cpu")
	assert.Contains(t, cpuNS, "container_cpu_usage_seconds_total")
	assert.Contains(t, cpuNS, "sum by (namespace, pod)")
	assert.Contains(t, cpuNS, `namespace="dev"`)
	assert.Contains(t, cpuNS, "* 1000", "CPU converted cores->millicores")

	memNS := buildPromPodQuery("dev", "memory")
	assert.Contains(t, memNS, "container_memory_working_set_bytes")
	assert.NotContains(t, memNS, "rate(", "memory is a gauge")

	allNS := buildPromPodQuery("", "cpu")
	assert.NotContains(t, allNS, "namespace=", "no namespace filter for all-namespaces query")

	assert.Empty(t, buildPromPodQuery("dev", "bogus"))
	assert.Empty(t, buildPromPodQuery(`evil"} or up{`, "cpu"), "namespace with PromQL metacharacters is rejected")
}

func TestParsePrometheusPodResponse(t *testing.T) {
	data := `{"status":"success","data":{"resultType":"vector","result":[
		{"metric":{"namespace":"dev","pod":"web-1"},"value":[1700000000,"125.5"]},
		{"metric":{"namespace":"dev","pod":"web-2"},"value":[1700000000,"40"]},
		{"metric":{"pod":"no-ns"},"value":[1700000000,"10"]},
		{"metric":{"namespace":"dev"},"value":[1700000000,"10"]}
	]}}`
	got, err := parsePrometheusPodResponse([]byte(data))
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"dev/web-1": 125.5, "dev/web-2": 40}, got)

	_, err = parsePrometheusPodResponse([]byte(`{not json`))
	assert.Error(t, err)

	_, err = parsePrometheusPodResponse([]byte(`{"status":"error","data":{}}`))
	assert.Error(t, err)
}

func TestGetAllPodMetricsFromPrometheus(t *testing.T) {
	c := &Client{}
	c.testPromQuery = func(_ context.Context, _ string, query string) ([]byte, error) {
		if strings.Contains(query, "container_cpu_usage_seconds_total") {
			return []byte(`{"status":"success","data":{"resultType":"vector","result":[
				{"metric":{"namespace":"dev","pod":"web-1"},"value":[1700000000,"250"]}]}}`), nil
		}
		return []byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"namespace":"dev","pod":"web-1"},"value":[1700000000,"209715200"]}]}}`), nil
	}

	got, err := c.getAllPodMetricsFromPrometheus(context.Background(), "ctx", "dev")
	require.NoError(t, err)
	require.Contains(t, got, "dev/web-1")
	pm := got["dev/web-1"]
	assert.Equal(t, "web-1", pm.Name)
	assert.Equal(t, "dev", pm.Namespace)
	assert.Equal(t, int64(250), pm.CPU)
	assert.Equal(t, int64(209715200), pm.Memory)
}

// GetAllPodMetrics must prefer Prometheus when it is the configured/implicit
// route, so a metrics-server-less cluster with Prometheus still shows pod usage
// instead of erroring with "metrics API unavailable".
func TestGetAllPodMetrics_PrefersPrometheusWhenConfigured(t *testing.T) {
	c := &Client{}
	c.testPromQuery = func(_ context.Context, _ string, query string) ([]byte, error) {
		val := "100"
		if !strings.Contains(query, "container_cpu_usage_seconds_total") {
			val = "1048576"
		}
		return []byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"namespace":"dev","pod":"web-1"},"value":[1700000000,"` + val + `"]}]}}`), nil
	}

	prev := model.ConfigMonitoring
	t.Cleanup(func() { model.ConfigMonitoring = prev })
	model.ConfigMonitoring = map[string]model.MonitoringConfig{
		"ctx": {Prometheus: &model.MonitoringEndpoint{
			Namespaces: []string{"monitoring"}, Services: []string{"prometheus"}, Port: "9090",
		}},
	}

	got, err := c.GetAllPodMetrics(context.Background(), "ctx", "dev")
	require.NoError(t, err)
	require.Contains(t, got, "dev/web-1")
	assert.Equal(t, int64(100), got["dev/web-1"].CPU)
}
