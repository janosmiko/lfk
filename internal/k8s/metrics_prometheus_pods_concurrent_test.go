package k8s

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Both pod queries go through the API server's proxy to Prometheus, and each
// round trip costs a second or two on a remote cluster. Run one after the other
// they add up past the watch interval, so every refresh cancelled the fetch
// before it finished and the CPU/MEM columns never filled in. They must go out
// together.
func TestGetAllPodMetricsFromPrometheus_RunsBothQueriesConcurrently(t *testing.T) {
	c := &Client{}

	var mu sync.Mutex
	inFlight := 0
	maxInFlight := 0
	bothArrived := make(chan struct{})
	var once sync.Once

	c.testPromQuery = func(_ context.Context, _ string, query string) ([]byte, error) {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		reached := inFlight == 2
		mu.Unlock()
		if reached {
			once.Do(func() { close(bothArrived) })
		}
		// Sequential queries never reach two in flight, so this wait is what
		// makes the test fail instead of hang forever.
		select {
		case <-bothArrived:
		case <-time.After(2 * time.Second):
		}
		mu.Lock()
		inFlight--
		mu.Unlock()

		val := "250"
		if !strings.Contains(query, "container_cpu_usage_seconds_total") {
			val = "209715200"
		}
		return []byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"namespace":"dev","pod":"web-1"},"value":[1700000000,"` + val + `"]}]}}`), nil
	}

	got, err := c.getAllPodMetricsFromPrometheus(t.Context(), "ctx", "dev")
	require.NoError(t, err)

	mu.Lock()
	peak := maxInFlight
	mu.Unlock()
	assert.Equal(t, 2, peak, "the CPU and memory queries must be on the wire together")

	// The concurrent path must still merge both results into one entry.
	pm := got["dev/web-1"]
	assert.Equal(t, int64(250), pm.CPU)
	assert.Equal(t, int64(209715200), pm.Memory)
}
