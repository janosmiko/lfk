package k8s

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	        "values": [[1, "1"], [2, "NaN"], [3, "3"]]
	      }
	    ]
	  }
	}`)

	got, err := parsePrometheusMatrix(data, podKeyFunc)
	require.NoError(t, err)
	pts := got["default/api-1"].Points
	require.Len(t, pts, 3)
	assert.Equal(t, 1.0, pts[0])
	assert.True(t, math.IsNaN(pts[1]), "gap sample must be NaN, got %v", pts[1])
	assert.Equal(t, 3.0, pts[2])
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
