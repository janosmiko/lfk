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

// rangeConcurrencyProbe returns a range-query stub that reports the peak number
// of queries in flight. A sequential caller never reaches two, so the wait ends
// on its timeout and the assertion fails rather than the test hanging.
func rangeConcurrencyProbe() (func(context.Context, string, string, time.Duration, time.Duration) ([]byte, error), func() int) {
	var mu sync.Mutex
	inFlight, peak := 0, 0
	bothArrived := make(chan struct{})
	var once sync.Once

	stub := func(_ context.Context, _ string, query string, _, _ time.Duration) ([]byte, error) {
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		reached := inFlight == 2
		mu.Unlock()
		if reached {
			once.Do(func() { close(bothArrived) })
		}
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
		return []byte(`{"status":"success","data":{"resultType":"matrix","result":[
			{"metric":{"namespace":"dev","pod":"web-1","node":"node-1"},
			 "values":[[1700000000,"` + val + `"]]}]}}`), nil
	}
	return stub, func() int {
		mu.Lock()
		defer mu.Unlock()
		return peak
	}
}

// Each range query goes through the API server's proxy to Prometheus, and a
// matrix response is heavier than the vector the instant queries return. Run
// one after the other the two round trips add up past the watch interval, the
// same way they did for the instant pod queries before they were made
// concurrent, and the fetch is cancelled before it finishes.
func TestGetMetricsRange_RunsBothQueriesConcurrently(t *testing.T) {
	const window, step = 15 * time.Minute, time.Minute

	tests := map[string]func(*Client) error{
		"pod": func(c *Client) error {
			_, _, err := c.GetPodMetricsRange(t.Context(), "ctx", "dev", window, step)
			return err
		},
		"node": func(c *Client) error {
			_, _, err := c.GetNodeMetricsRange(t.Context(), "ctx", window, step)
			return err
		},
		"cluster": func(c *Client) error {
			_, _, err := c.GetClusterMetricsRange(t.Context(), "ctx", window, step)
			return err
		},
		"container": func(c *Client) error {
			_, _, err := c.GetContainerMetricsRange(t.Context(), "ctx", "dev", "web-1", window, step)
			return err
		},
	}
	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			stub, peak := rangeConcurrencyProbe()
			c := &Client{testPromRangeQuery: stub}

			require.NoError(t, call(c))

			assert.Equal(t, 2, peak(), "the CPU and memory range queries must be on the wire together")
		})
	}
}
