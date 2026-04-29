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
// Pattern is copied verbatim from kubectl's overlyCautiousIllegalFileCharacters:
// the literal '(' and ')' inside the character class are intentional and kept
// for on-disk-cache compatibility — they're filename-safe anyway, so allowing
// them through is harmless.
var toCacheHostDir = regexp.MustCompile(`[^(\w/.)]`)

// CacheHostDir returns the directory name kubectl/k9s/lfk all use to namespace
// per-host cache data under ~/.kube/cache/discovery/<dir>/. Exposed so other
// packages (e.g. lfk's enriched discovery cache) can write into the same
// per-host layout and inherit the same `kubectl api-resources --invalidate-cache`
// lifecycle.
func CacheHostDir(host string) string {
	return cacheHostDir(host)
}

func cacheHostDir(host string) string {
	h := strings.Replace(host, "https://", "", 1)
	h = strings.Replace(h, "http://", "", 1)
	return toCacheHostDir.ReplaceAllString(h, "_")
}

// DiscoveryCacheBaseDir returns the on-disk base for per-host discovery
// caches: $KUBECACHEDIR or ~/.kube/cache. Returns "" if the home directory
// can't be resolved (extremely rare; the same fallback the standard
// disk-cached discovery client uses internally). lfk's enriched cache layers
// on top of this same root.
func DiscoveryCacheBaseDir() string {
	if base := os.Getenv("KUBECACHEDIR"); base != "" {
		return base
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "cache")
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
//
// Note: this only flips the in-memory client's invalidated flag. A *fresh*
// client created later (e.g. first-time discovery for a context) won't see
// the flag and will read from disk if the cache is still warm. That's fine
// in practice because shift+r at LevelResourceTypes can only be reached
// after an initial discovery has already created the client — see the
// caller in app/update_actions.go's refreshCurrentLevel.
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
