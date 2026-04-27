package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/kubernetes"
)

const discoveryCacheTTL = 5 * time.Minute

// Layout matches kubectl/k9s so all three tools share ~/.kube/cache/discovery.
var toCacheHostDir = regexp.MustCompile(`[^(\w/.)]`)

func cacheHostDir(host string) string {
	h := strings.Replace(host, "https://", "", 1)
	h = strings.Replace(h, "http://", "", 1)
	return toCacheHostDir.ReplaceAllString(h, "_")
}

// discoveryForContext returns a per-context disk-cached discovery client,
// memoized in-process. testClientset bypasses the disk path so unit tests
// don't touch the filesystem.
func (c *Client) discoveryForContext(displayName string) (discovery.DiscoveryInterface, error) {
	if c.testClientset != nil {
		if cs, ok := c.testClientset.(kubernetes.Interface); ok {
			return cs.Discovery(), nil
		}
	}

	c.discoveryMu.Lock()
	defer c.discoveryMu.Unlock()

	if existing, ok := c.discoveryClients[displayName]; ok {
		return existing, nil
	}

	cfg, err := c.restConfigForContext(displayName)
	if err != nil {
		return nil, err
	}

	base := os.Getenv("KUBECACHEDIR")
	if base == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return nil, fmt.Errorf("resolving home dir for discovery cache: %w", herr)
		}
		base = filepath.Join(home, ".kube", "cache")
	}

	discDir := filepath.Join(base, "discovery", cacheHostDir(cfg.Host))
	httpDir := filepath.Join(base, "http")

	client, err := disk.NewCachedDiscoveryClientForConfig(cfg, discDir, httpDir, discoveryCacheTTL)
	if err != nil {
		return nil, fmt.Errorf("creating cached discovery client: %w", err)
	}

	if c.discoveryClients == nil {
		c.discoveryClients = make(map[string]*disk.CachedDiscoveryClient)
	}
	c.discoveryClients[displayName] = client
	return client, nil
}

// InvalidateDiscoveryCache forces the next discovery call to re-fetch.
// Nil-safe so test models without a real client can call it.
func (c *Client) InvalidateDiscoveryCache(displayName string) {
	if c == nil {
		return
	}
	c.discoveryMu.Lock()
	defer c.discoveryMu.Unlock()
	if cli, ok := c.discoveryClients[displayName]; ok {
		cli.Invalidate()
	}
}
