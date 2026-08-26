package k8s

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// queryRecorder captures the queries one range call issues. The CPU and memory
// queries now go out together, so the slice needs a lock and their arrival
// order is not fixed.
type queryRecorder struct {
	mu      sync.Mutex
	queries []string
}

func (r *queryRecorder) record(query string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queries = append(r.queries, query)
}

func (r *queryRecorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.queries)
}

// podKeyFunc mirrors the keying parsePrometheusPodResponse uses, so the
// matrix parser is exercised through a real caller's key shape.
func podKeyFunc(labels map[string]string) (string, bool) {
	ns, pod := labels["namespace"], labels["pod"]
	if ns == "" || pod == "" {
		return "", false
	}
	return ns + "/" + pod, true
}

func TestParsePrometheusMatrix_WellFormed(t *testing.T) {
	data := []byte(`{
	  "status": "success",
	  "data": {
	    "resultType": "matrix",
	    "result": [
	      {
	        "metric": {"namespace": "default", "pod": "api-1"},
	        "values": [[1700000000, "10"], [1700000030, "20.5"], [1700000060, "30"]]
	      }
	    ]
	  }
	}`)

	got, err := parsePrometheusMatrix(data, podKeyFunc)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, []float64{10, 20.5, 30}, got["default/api-1"].Points)
}

func TestParsePrometheusMatrix_SkipsSeriesMissingLabels(t *testing.T) {
	data := []byte(`{
	  "status": "success",
	  "data": {
	    "resultType": "matrix",
	    "result": [
	      {"metric": {"pod": "no-namespace"}, "values": [[1, "1"]]},
	      {"metric": {"namespace": "default", "pod": "api-1"}, "values": [[1, "7"]]}
	    ]
	  }
	}`)

	got, err := parsePrometheusMatrix(data, podKeyFunc)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, []float64{7}, got["default/api-1"].Points)
}

// An unparseable sample must not shift the samples after it. Prometheus can
// emit "NaN" for a gap inside an otherwise present series, and collapsing the
// slice there would draw later samples at the wrong position on the time axis.
func TestParsePrometheusMatrix_UnparseableSampleBecomesNaN(t *testing.T) {
	data := []byte(`{
	  "status": "success",
	  "data": {
	    "resultType": "matrix",
	    "result": [
	      {
	        "metric": {"namespace": "default", "pod": "api-1"},
	        "values": [[1, "1"], [2, "NaN"], [3, "3"], [4, "not-a-number"]]
	      }
	    ]
	  }
	}`)

	got, err := parsePrometheusMatrix(data, podKeyFunc)
	require.NoError(t, err)
	pts := got["default/api-1"].Points
	require.Len(t, pts, 4)
	assert.Equal(t, 1.0, pts[0])
	// "NaN" parses cleanly to NaN, so it proves the gap passes through, not
	// that the parse-error branch runs. "not-a-number" is what reaches that.
	assert.True(t, math.IsNaN(pts[1]), "gap sample must be NaN, got %v", pts[1])
	assert.Equal(t, 3.0, pts[2])
	assert.True(t, math.IsNaN(pts[3]), "unparseable sample must become NaN, got %v", pts[3])
}

func TestParsePrometheusMatrix_NonSuccessStatus(t *testing.T) {
	data := []byte(`{"status": "error", "error": "boom", "data": {"resultType": "matrix", "result": []}}`)

	_, err := parsePrometheusMatrix(data, podKeyFunc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error")
}

func TestParsePrometheusMatrix_MalformedJSON(t *testing.T) {
	_, err := parsePrometheusMatrix([]byte(`not json`), podKeyFunc)
	require.Error(t, err)
}

func TestParsePrometheusMatrix_EmptyResult(t *testing.T) {
	data := []byte(`{"status": "success", "data": {"resultType": "matrix", "result": []}}`)

	got, err := parsePrometheusMatrix(data, podKeyFunc)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRunPrometheusRangeQuery_DemoModeRefuses(t *testing.T) {
	c := &Client{demo: true}

	_, err := c.runPrometheusRangeQuery(t.Context(), "ctx", "up", 15*time.Minute, 45*time.Second)
	require.Error(t, err)
	assert.ErrorIs(t, err, errPrometheusUnavailableDemo)
}

func TestRunPrometheusRangeQuery_UsesTestSeam(t *testing.T) {
	var gotQuery string
	var gotWindow, gotStep time.Duration
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, query string, window, step time.Duration) ([]byte, error) {
			gotQuery, gotWindow, gotStep = query, window, step
			return []byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`), nil
		},
	}

	body, err := c.runPrometheusRangeQuery(t.Context(), "ctx", "up", 15*time.Minute, 45*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "up", gotQuery)
	assert.Equal(t, 15*time.Minute, gotWindow)
	assert.Equal(t, 45*time.Second, gotStep)
	assert.Contains(t, string(body), "matrix")
}

// 5m/7 is fractional. A "s"-suffixed literal must be a whole number.
func TestFormatPromStep_FractionalSecondsHasNoUnitSuffix(t *testing.T) {
	step := 5 * time.Minute / 7

	assert.Equal(t, "42.857142857", formatPromStep(step))
}

func TestFormatPromStep_WholeSecondsHasNoUnitSuffix(t *testing.T) {
	assert.Equal(t, "45", formatPromStep(45*time.Second))
}

func TestGetPodMetricsRange_ReturnsCPUAndMemorySeries(t *testing.T) {
	var rec queryRecorder
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, query string, _, _ time.Duration) ([]byte, error) {
			rec.record(query)
			return []byte(`{
			  "status":"success",
			  "data":{"resultType":"matrix","result":[
			    {"metric":{"namespace":"default","pod":"api-1"},"values":[[1,"5"],[2,"10"]]}
			  ]}
			}`), nil
		},
	}

	cpu, mem, err := c.GetPodMetricsRange(t.Context(), "ctx", "default", 15*time.Minute, 45*time.Second)
	require.NoError(t, err)
	assert.Equal(t, []float64{5, 10}, cpu["default/api-1"].Points)
	assert.Equal(t, []float64{5, 10}, mem["default/api-1"].Points)
	assert.Equal(t, 45*time.Second, cpu["default/api-1"].Step)
	got := rec.all()
	require.Len(t, got, 2, "one query for cpu, one for memory")
	joined := strings.Join(got, "\n")
	assert.Contains(t, joined, "container_cpu_usage_seconds_total")
	assert.Contains(t, joined, "container_memory_working_set_bytes")
}

// A partial outage still shows what is available, matching
// getAllPodMetricsFromPrometheus, which errors only when both queries fail.
func TestGetPodMetricsRange_PartialFailureStillReturnsData(t *testing.T) {
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, query string, _, _ time.Duration) ([]byte, error) {
			if strings.Contains(query, "container_memory_working_set_bytes") {
				return nil, errors.New("memory query exploded")
			}
			return []byte(`{"status":"success","data":{"resultType":"matrix","result":[
			  {"metric":{"namespace":"default","pod":"api-1"},"values":[[1,"5"]]}]}}`), nil
		},
	}

	cpu, mem, err := c.GetPodMetricsRange(t.Context(), "ctx", "default", time.Minute, time.Second)
	require.NoError(t, err)
	assert.Equal(t, []float64{5}, cpu["default/api-1"].Points)
	assert.Empty(t, mem)
}

func TestGetPodMetricsRange_BothQueriesFail(t *testing.T) {
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, _ string, _, _ time.Duration) ([]byte, error) {
			return nil, errors.New("prometheus down")
		},
	}

	_, _, err := c.GetPodMetricsRange(t.Context(), "ctx", "default", time.Minute, time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prometheus down")
}

// buildPromPodQuery returns "" for a namespace carrying PromQL
// metacharacters. An empty query must not be sent.
func TestGetPodMetricsRange_RejectsUnsafeNamespace(t *testing.T) {
	called := false
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, _ string, _, _ time.Duration) ([]byte, error) {
			called = true
			return nil, nil
		},
	}

	_, _, err := c.GetPodMetricsRange(t.Context(), "ctx", `bad"ns`, time.Minute, time.Second)
	require.Error(t, err)
	assert.False(t, called, "an unsafe namespace must never reach the transport")
}

func TestGetNodeMetricsRange_KeysByNodeName(t *testing.T) {
	var rec queryRecorder
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, query string, _, _ time.Duration) ([]byte, error) {
			rec.record(query)
			return []byte(`{"status":"success","data":{"resultType":"matrix","result":[
			  {"metric":{"node":"node-1"},"values":[[1,"100"],[2,"200"]]}]}}`), nil
		},
	}

	cpu, mem, err := c.GetNodeMetricsRange(t.Context(), "ctx", 15*time.Minute, 45*time.Second)
	require.NoError(t, err)
	assert.Equal(t, []float64{100, 200}, cpu["node-1"].Points)
	assert.Equal(t, []float64{100, 200}, mem["node-1"].Points)
	got := rec.all()
	require.Len(t, got, 2)
	for _, q := range got {
		assert.Contains(t, q, "by (node)", "both node queries group by node")
	}
}

func TestGetNodeMetricsRange_BothQueriesFail(t *testing.T) {
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, _ string, _, _ time.Duration) ([]byte, error) {
			return nil, errors.New("prometheus down")
		},
	}

	_, _, err := c.GetNodeMetricsRange(t.Context(), "ctx", time.Minute, time.Second)
	require.Error(t, err)
}

func TestGetClusterMetricsRange_ReturnsOneSeriesEach(t *testing.T) {
	var rec queryRecorder
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, query string, _, _ time.Duration) ([]byte, error) {
			rec.record(query)
			return []byte(`{"status":"success","data":{"resultType":"matrix","result":[
			  {"metric":{},"values":[[1,"1000"],[2,"2000"]]}]}}`), nil
		},
	}

	cpu, mem, err := c.GetClusterMetricsRange(t.Context(), "ctx", 15*time.Minute, 45*time.Second)
	require.NoError(t, err)
	assert.Equal(t, []float64{1000, 2000}, cpu.Points)
	assert.Equal(t, []float64{1000, 2000}, mem.Points)
	got := rec.all()
	require.Len(t, got, 2)
	// A cluster total aggregates everything, so neither query may carry a
	// "by" clause: one would split the result into several series and the
	// parser would keep only the last.
	for _, q := range got {
		assert.NotContains(t, q, "by (")
	}
}

func TestGetClusterMetricsRange_BothQueriesFail(t *testing.T) {
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, _ string, _, _ time.Duration) ([]byte, error) {
			return nil, errors.New("prometheus down")
		},
	}

	_, _, err := c.GetClusterMetricsRange(t.Context(), "ctx", time.Minute, time.Second)
	require.Error(t, err)
}

func TestGetClusterMetricsRange_EmptyResultIsNotAnError(t *testing.T) {
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, _ string, _, _ time.Duration) ([]byte, error) {
			return []byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`), nil
		},
	}

	cpu, _, err := c.GetClusterMetricsRange(t.Context(), "ctx", time.Minute, time.Second)
	require.NoError(t, err)
	assert.Empty(t, cpu.Points, "no data is an empty series, and the caller reverts to numeric")
}

func TestBuildPromContainerRangeQuery(t *testing.T) {
	cpu := buildPromContainerRangeQuery("dev", "web-1", "cpu")
	assert.Contains(t, cpu, "container_cpu_usage_seconds_total")
	assert.Contains(t, cpu, "sum by (container)")
	assert.Contains(t, cpu, `namespace="dev"`)
	assert.Contains(t, cpu, `pod="web-1"`)
	assert.Contains(t, cpu, `container!="POD"`, "the pause container must be excluded")
	assert.Contains(t, cpu, "* 1000", "CPU converted cores->millicores")

	mem := buildPromContainerRangeQuery("dev", "web-1", "memory")
	assert.Contains(t, mem, "container_memory_working_set_bytes")
	assert.NotContains(t, mem, "rate(", "memory is a gauge")

	assert.Empty(t, buildPromContainerRangeQuery("dev", "web-1", "bogus"))
	assert.Empty(t, buildPromContainerRangeQuery(`evil"} or up{`, "web-1", "cpu"),
		"namespace with PromQL metacharacters is rejected")
	assert.Empty(t, buildPromContainerRangeQuery("dev", `evil"} or up{`, "cpu"),
		"pod with PromQL metacharacters is rejected")
}

func TestGetContainerMetricsRange_ReturnsCPUAndMemorySeries(t *testing.T) {
	var rec queryRecorder
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, query string, _, _ time.Duration) ([]byte, error) {
			rec.record(query)
			return []byte(`{
			  "status":"success",
			  "data":{"resultType":"matrix","result":[
			    {"metric":{"container":"app"},"values":[[1,"5"],[2,"10"]]}
			  ]}
			}`), nil
		},
	}

	cpu, mem, err := c.GetContainerMetricsRange(t.Context(), "ctx", "default", "web-1", 15*time.Minute, 45*time.Second)
	require.NoError(t, err)
	assert.Equal(t, []float64{5, 10}, cpu["app"].Points)
	assert.Equal(t, []float64{5, 10}, mem["app"].Points)
	assert.Equal(t, 45*time.Second, cpu["app"].Step)
	got := rec.all()
	require.Len(t, got, 2, "one query for cpu, one for memory")
	joined := strings.Join(got, "\n")
	assert.Contains(t, joined, "container_cpu_usage_seconds_total")
	assert.Contains(t, joined, "container_memory_working_set_bytes")
}

// A partial outage still shows what is available, matching GetPodMetricsRange,
// which errors only when both queries fail.
func TestGetContainerMetricsRange_PartialFailureStillReturnsData(t *testing.T) {
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, query string, _, _ time.Duration) ([]byte, error) {
			if strings.Contains(query, "container_memory_working_set_bytes") {
				return nil, errors.New("memory query exploded")
			}
			return []byte(`{"status":"success","data":{"resultType":"matrix","result":[
			  {"metric":{"container":"app"},"values":[[1,"5"]]}]}}`), nil
		},
	}

	cpu, mem, err := c.GetContainerMetricsRange(t.Context(), "ctx", "default", "web-1", time.Minute, time.Second)
	require.NoError(t, err)
	assert.Equal(t, []float64{5}, cpu["app"].Points)
	assert.Empty(t, mem)
}

func TestGetContainerMetricsRange_BothQueriesFail(t *testing.T) {
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, _ string, _, _ time.Duration) ([]byte, error) {
			return nil, errors.New("prometheus down")
		},
	}

	_, _, err := c.GetContainerMetricsRange(t.Context(), "ctx", "default", "web-1", time.Minute, time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prometheus down")
}

// buildPromContainerRangeQuery returns "" for a namespace carrying PromQL
// metacharacters. An empty query must not be sent, proving the transport is
// never reached for an unsafe namespace.
func TestGetContainerMetricsRange_RejectsUnsafeNamespace(t *testing.T) {
	called := false
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, _ string, _, _ time.Duration) ([]byte, error) {
			called = true
			return nil, nil
		},
	}

	_, _, err := c.GetContainerMetricsRange(t.Context(), "ctx", `bad"ns`, "web-1", time.Minute, time.Second)
	require.Error(t, err)
	assert.False(t, called, "an unsafe namespace must never reach the transport")
}

// Same guard, but for the pod name half of the interpolation.
func TestGetContainerMetricsRange_RejectsUnsafePodName(t *testing.T) {
	called := false
	c := &Client{
		testPromRangeQuery: func(_ context.Context, _, _ string, _, _ time.Duration) ([]byte, error) {
			called = true
			return nil, nil
		},
	}

	_, _, err := c.GetContainerMetricsRange(t.Context(), "ctx", "default", `bad"pod`, time.Minute, time.Second)
	require.Error(t, err)
	assert.False(t, called, "an unsafe pod name must never reach the transport")
}
