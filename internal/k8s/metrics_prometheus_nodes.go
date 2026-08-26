package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strconv"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
)

// nodeUptimeQueryEnabled reports whether Prometheus is the configured
// monitoring source for contextName. On a metrics-server-only cluster there
// is no Prometheus service to find, and an unconditional fetch would re-run
// the full namespace x service discovery sweep on every watch tick.
func nodeUptimeQueryEnabled(contextName string) bool {
	nodeMetrics, hasPrometheus := resolveNodeMetricsConfig(contextName)
	return nodeMetrics == "prometheus" || hasPrometheus
}

// nodeUptimeNameKeys returns the Kubernetes-node-name label values a
// node_exporter series carries. Kept separate from nodeUptimeAddrKeys (see
// NodeUptimes) so a node named the same string as another node's IP or OS
// hostname can't collide in a single keyspace.
func nodeUptimeNameKeys(labels map[string]string) []string {
	var keys []string
	for _, label := range []string{"node", "kubernetes_node"} {
		if v := labels[label]; v != "" {
			keys = append(keys, v)
		}
	}
	return keys
}

// nodeUptimeAddrKeys returns the IP/hostname label values a node_exporter
// series carries. node_exporter series don't reliably carry a "node" label,
// and their "instance" label is a host:port ("10.0.1.5:9100"), never a
// Kubernetes node name.
func nodeUptimeAddrKeys(labels map[string]string) []string {
	var keys []string
	for _, label := range []string{"nodename", "host"} {
		if v := labels[label]; v != "" {
			keys = append(keys, v)
		}
	}
	if instance := labels["instance"]; instance != "" {
		keys = append(keys, stripInstancePort(instance))
	}
	return keys
}

// stripInstancePort trims the ":<port>" suffix off a node_exporter "instance"
// label (e.g. "10.0.1.5:9100" -> "10.0.1.5", "[::1]:9100" -> "::1"). Falls
// back to the raw value when it carries no port.
func stripInstancePort(instance string) string {
	host, _, err := net.SplitHostPort(instance)
	if err != nil {
		return instance
	}
	return host
}

// nodeUptimeSeconds is the float64-seconds counterpart to NodeUptimes,
// produced by parseNodeUptimeVector before GetNodeUptimes converts each
// value to a time.Duration.
type nodeUptimeSeconds struct {
	byName map[string]float64
	byAddr map[string]float64
}

// parseNodeUptimeVector parses a `time() - node_boot_time_seconds` instant-
// vector response into name-keyed and address-keyed seconds (see
// nodeUptimeNameKeys / nodeUptimeAddrKeys). The two keyspaces are kept
// separate so a node's name colliding with another node's IP/hostname can't
// silently overwrite either value. Samples with an unparseable, negative,
// NaN, or infinite value are skipped -- a negative/NaN/Inf uptime is
// nonsense and would come from clock skew or a malformed sample.
func parseNodeUptimeVector(data []byte) (nodeUptimeSeconds, error) {
	var resp prometheusQueryResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nodeUptimeSeconds{}, fmt.Errorf("parsing prometheus response: %w", err)
	}
	if resp.Status != "success" {
		return nodeUptimeSeconds{}, fmt.Errorf("prometheus query returned status: %s", resp.Status)
	}

	result := nodeUptimeSeconds{
		byName: make(map[string]float64, len(resp.Data.Result)),
		byAddr: make(map[string]float64, len(resp.Data.Result)),
	}
	for _, r := range resp.Data.Result {
		if len(r.Value) < 2 {
			continue
		}
		var valStr string
		if err := json.Unmarshal(r.Value[1], &valStr); err != nil {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		// NaN and +Inf both pass a plain "val < 0" check (IEEE-754), and
		// would otherwise render as "0s" (just rebooted) or a ~292y uptime --
		// both actively misleading. -Inf is already caught by val < 0.
		if err != nil || val < 0 || math.IsNaN(val) || math.IsInf(val, 0) {
			continue
		}

		for _, key := range nodeUptimeNameKeys(r.Metric) {
			result.byName[key] = val
		}
		for _, key := range nodeUptimeAddrKeys(r.Metric) {
			result.byAddr[key] = val
		}
	}
	return result, nil
}

// NodeUptimes separates node uptimes keyed by Kubernetes node name (ByName)
// from those keyed by node IP or OS hostname (ByAddr). node_exporter series
// don't reliably carry a Kubernetes node name, so callers commonly need to
// fall back to an IP/hostname column (see lookupNodeUptime in the app
// package) -- but folding both into one flat map let one node's name
// collide with another node's IP/hostname, silently overwriting one node's
// uptime with another's.
type NodeUptimes struct {
	ByName map[string]time.Duration
	ByAddr map[string]time.Duration
}

// Empty reports whether neither map holds any data: Prometheus isn't the
// configured monitoring source, node_exporter isn't installed, or the query
// has never once succeeded.
func (n NodeUptimes) Empty() bool {
	return len(n.ByName) == 0 && len(n.ByAddr) == 0
}

// GetNodeUptimes queries Prometheus for how long each node has been
// running, keyed separately by node name and by node IP/hostname (see
// NodeUptimes). Returns a zero-value NodeUptimes (Empty() == true) with a
// nil error when Prometheus isn't the configured monitoring source -- uptime
// is simply unavailable on a metrics-server-only cluster, not an error.
func (c *Client) GetNodeUptimes(ctx context.Context, contextName string) (NodeUptimes, error) {
	if !nodeUptimeQueryEnabled(contextName) {
		return NodeUptimes{}, nil
	}

	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return NodeUptimes{}, fmt.Errorf("failed to get clientset: %w", err)
	}

	// Bounded like the sibling node CPU/mem queries, so a hung proxy can't
	// block the caller forever.
	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	promTargets, _ := monitoringTargetsFor(qctx, clientset, contextName)

	// Subtracted server-side so the result is independent of clock skew
	// between this machine and the cluster.
	const query = `time() - node_boot_time_seconds`
	seconds, err := queryPrometheusMetric(qctx, contextName, clientset, c.demo, promTargets, query, parseNodeUptimeVector)
	if err != nil {
		logger.Debug("Prometheus node uptime query failed", "context", contextName, "error", err)
		return NodeUptimes{}, fmt.Errorf("prometheus node uptime query failed: %w", err)
	}

	result := NodeUptimes{
		ByName: make(map[string]time.Duration, len(seconds.byName)),
		ByAddr: make(map[string]time.Duration, len(seconds.byAddr)),
	}
	for key, s := range seconds.byName {
		result.ByName[key] = time.Duration(s * float64(time.Second))
	}
	for key, s := range seconds.byAddr {
		result.ByAddr[key] = time.Duration(s * float64(time.Second))
	}
	return result, nil
}
