// Package k8s — ratelimit.go
// Client-side REST rate-limit knobs. client-go's built-in default (QPS=5,
// Burst=10) throttles foreground list calls whenever background work runs
// concurrently — a multi-source security scan or metrics enrichment drains the
// 10-token bucket and the next pod list queues behind it. These defaults give
// an interactive multi-resource TUI enough headroom; the config layer can
// override them globally and per-cluster via RateLimitForContext.
package k8s

const (
	// DefaultClientQPS / DefaultClientBurst are the foreground rest.Config
	// rate limits used when the config layer supplies no override.
	DefaultClientQPS   float32 = 50
	DefaultClientBurst int     = 100

	// SecurityClientQPS / SecurityClientBurst are the dedicated, smaller
	// budget for security-source clients. Giving background finding scans
	// their own low rate is the practical "low priority" lever — the
	// bg-task wrapper's scheduler priority is advisory only, so a separate
	// token bucket is what actually keeps scans from starving the
	// foreground's shared budget.
	SecurityClientQPS   float32 = 10
	SecurityClientBurst int     = 20
)

// RateLimitForContext, when set by the config layer, returns the effective
// foreground QPS/Burst for a kube context (per-cluster override > global >
// default). A nil hook, or a non-positive return, falls back to the
// Default* constants. Modeled as a function hook so the k8s package keeps no
// dependency on the ui/config package.
//
// Concurrency contract: written exactly once, from ui/config's init() at
// package load, before any worker goroutine or REST client is created;
// read-only thereafter. That single-write-before-use ordering is why no
// mutex guards it. Tests that reassign it run sequentially (no t.Parallel).
var RateLimitForContext func(context string) (qps float32, burst int)

// RateLimitOverridesEnabled gates whether lfk applies ANY client-side
// QPS/Burst override (the raised foreground default and the lower security
// budget). When false, every client uses client-go's stock default (QPS 5 /
// Burst 10) — the pre-feature baseline. Enabled now that the scheduler
// lost-wakeup jam is fixed; set to false to fall back to stock client-go
// limits while diagnosing any throttling regression.
var RateLimitOverridesEnabled = true

// foregroundRate resolves the foreground rate limit for a context, applying
// the config hook when present and falling back to the compiled defaults.
func foregroundRate(context string) (float32, int) {
	if RateLimitForContext != nil {
		if qps, burst := RateLimitForContext(context); qps > 0 && burst > 0 {
			return qps, burst
		}
	}
	return DefaultClientQPS, DefaultClientBurst
}
