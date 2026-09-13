package app

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// discoveryCacheLoadedMsg carries the result of an async startup-time
// preload of the per-host discovery snapshots. The handler in update.go
// merges entries into m.discoveredResources, layered behind any live
// apiResourceDiscoveryMsg that has already arrived for the same context.
type discoveryCacheLoadedMsg struct {
	cached map[string][]model.ResourceTypeEntry
}

// discoveryCachePreloadCmd reads the discovery cache off the main goroutine.
// loadAllDiscoveryCaches walks every kubeconfig context and calls
// clientcmd.ClientConfig() on each to extract the apiserver host — at one
// real-world scale (block-sre-operator-style kubeconfig with ~1200 contexts
// in a single file) this re-parses the kubeconfig ~1200× and serialises the
// whole startup behind a multi-second clientcmd loop. Running it as a tea.Cmd
// lets NewModel return immediately so the first frame renders with seed
// resources. The cache then lands and overlays the seed list when ready.
//
// reqCtx lets a user-initiated quit short-circuit the loop instead of
// waiting for the full walk to finish. Without this plumbing a Ctrl+C
// during preload of a 1200-context kubeconfig would block until every
// host's cache file had been read and parsed.
func discoveryCachePreloadCmd(reqCtx context.Context, client *k8s.Client) tea.Cmd {
	return func() tea.Msg {
		return discoveryCacheLoadedMsg{cached: loadAllDiscoveryCaches(reqCtx, client)}
	}
}

// discoveryCacheSchemaVersion bumps whenever the on-disk shape changes.
// loadDiscoveryCacheForHost rejects unknown versions so older binaries don't
// trip on a forward-incompat write from a newer one — the worst case is one
// extra discovery roundtrip on first launch after upgrade.
//
// v2: per-host file layout (one yaml per ~/.kube/cache/discovery/<host>/)
// replacing the v1 single-file XDG-state design — shares lifecycle with
// kubectl/k9s so `kubectl api-resources --invalidate-cache` wipes lfk's
// cache too. v1 files in the old XDG location are abandoned (the previous
// build was unreleased).
const discoveryCacheSchemaVersion = 2

// discoveryCacheFilename is the basename of lfk's enriched cache inside each
// per-host kubectl-cache dir. Distinct from kubectl's per-group/version JSON
// files so the two formats can coexist in the same directory.
const discoveryCacheFilename = "lfk-enriched.yaml"

// DiscoveryCacheHostState is the on-disk shape: one file per cluster API
// host. Multiple kubeconfig contexts pointing at the same host share one
// file because cluster-level discovery output is host-keyed, not
// context-keyed.
type DiscoveryCacheHostState struct {
	SchemaVersion int                       `json:"schema_version"`
	Host          string                    `json:"host"`
	UpdatedAt     time.Time                 `json:"updated_at"`
	Entries       []model.ResourceTypeEntry `json:"entries"`
}

// discoveryCacheFilePathForHost returns the per-host cache file path:
// $KUBECACHEDIR/discovery/<host>/lfk-enriched.yaml (with $KUBECACHEDIR
// defaulting to ~/.kube/cache). Returns "" for an empty host or an
// unresolvable home dir. Callers treat that as "skip caching".
func discoveryCacheFilePathForHost(host string) string {
	if host == "" {
		return ""
	}
	base := k8s.DiscoveryCacheBaseDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "discovery", k8s.CacheHostDir(host), discoveryCacheFilename)
}

// loadDiscoveryCacheForHost reads one host's enriched cache. Returns nil on
// any failure — missing file, corrupt YAML, schema mismatch — so callers
// can treat a nil result as "fall through to live discovery".
func loadDiscoveryCacheForHost(host string) *DiscoveryCacheHostState {
	path := discoveryCacheFilePathForHost(host)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("Discovery cache read failed", "host", host, "error", err)
		}
		return nil
	}
	var s DiscoveryCacheHostState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Discovery cache is corrupt; ignoring", "host", host, "error", err)
		return nil
	}
	if s.SchemaVersion != discoveryCacheSchemaVersion {
		logger.Info("Discovery cache schema version mismatch; ignoring",
			"host", host, "got", s.SchemaVersion, "want", discoveryCacheSchemaVersion)
		return nil
	}
	return &s
}

// saveDiscoveryCacheForHost writes one host's enriched cache atomically
// (sibling .tmp + rename) so a crash mid-write can't leave a half-written
// file that loadDiscoveryCacheForHost would discard.
func saveDiscoveryCacheForHost(host string, entries []model.ResourceTypeEntry) error {
	path := discoveryCacheFilePathForHost(host)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	state := DiscoveryCacheHostState{
		SchemaVersion: discoveryCacheSchemaVersion,
		Host:          host,
		UpdatedAt:     time.Now().UTC(),
		Entries:       entries,
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// loadAllDiscoveryCaches enumerates every kubeconfig context known to client,
// resolves each to a host, reads each unique host's cache file once, and
// returns a map from context display name to cached resource entries ready
// to plug into m.discoveredResources. Contexts with unresolvable hosts or
// missing cache files are silently skipped — they fall through to live
// discovery on first interaction.
//
// reqCtx is checked at the top of every iteration so a cancellation during
// the walk (e.g. user quit) returns whatever was loaded so far instead of
// processing the rest. With a kubeconfig of ~1200 contexts the loop is
// minutes long under cold disk, and a quit must not block on it.
func loadAllDiscoveryCaches(reqCtx context.Context, client *k8s.Client) map[string][]model.ResourceTypeEntry {
	if client == nil {
		return nil
	}
	contexts, err := client.GetContexts()
	if err != nil {
		return nil
	}
	out := make(map[string][]model.ResourceTypeEntry)
	hostCache := make(map[string]*DiscoveryCacheHostState)
	for _, ctx := range contexts {
		if reqCtx != nil && reqCtx.Err() != nil {
			return out
		}
		host := client.HostForContext(ctx.Name)
		if host == "" {
			continue
		}
		snap, ok := hostCache[host]
		if !ok {
			snap = loadDiscoveryCacheForHost(host)
			hostCache[host] = snap
		}
		if snap == nil {
			continue
		}
		out[ctx.Name] = snap.Entries
	}
	return out
}

// sessionRestoreContext names the one context a saved session opens on: the
// active tab's for a multi-tab session, the legacy single-tab field otherwise.
func sessionRestoreContext(sess *SessionState) string {
	if sess == nil {
		return ""
	}
	if len(sess.Tabs) == 0 {
		return sess.Context
	}
	i := sess.ActiveTab
	if i < 0 || i >= len(sess.Tabs) {
		i = 0
	}
	return sess.Tabs[i].Context
}

// loadDiscoveryCacheForContext reads the on-disk snapshot for one context and
// returns it in the same shape updateAPIResourceDiscovery produces, pseudo
// resources first. Returns nil when the host cannot be resolved or no usable
// snapshot exists, which leaves the caller on the built-in seed list.
//
// One context, one clientcmd.ClientConfig() call. loadAllDiscoveryCaches pays
// that per kubeconfig context, which is why it runs off the main goroutine;
// this is the single-context read a session restore can afford up front.
func loadDiscoveryCacheForContext(client *k8s.Client, contextName string) []model.ResourceTypeEntry {
	if client == nil || contextName == "" {
		return nil
	}
	host := client.HostForContext(contextName)
	if host == "" {
		return nil
	}
	snap := loadDiscoveryCacheForHost(host)
	if snap == nil || len(snap.Entries) == 0 {
		return nil
	}
	return append(model.PseudoResources(), snap.Entries...)
}

// updateDiscoveryCacheForContext is the single mutator used by the discovery
// success path: resolve the context to its host, write the host's enriched
// snapshot. No-op when the host can't be resolved — the live data is still
// authoritative for this session.
func updateDiscoveryCacheForContext(client *k8s.Client, contextName string, entries []model.ResourceTypeEntry) error {
	if client == nil || contextName == "" {
		return nil
	}
	host := client.HostForContext(contextName)
	if host == "" {
		return nil
	}
	return saveDiscoveryCacheForHost(host, entries)
}

// shouldFireDiscoveryFor reports whether a fresh discovery call should be
// kicked off for contextName. Returns false when (a) a discovery is already
// in flight for that context — deduplication — or (b) discovery has already
// completed during this session, in which case re-fetching would just hit
// the cluster API for nothing.
//
// This is the gate that makes stale-while-revalidate work: NewModel prefills
// m.discoveredResources from disk, but the first hover/navigate of a context
// returns true here so a live refresh kicks off behind the cached view.
func (m *Model) shouldFireDiscoveryFor(contextName string) bool {
	if contextName == "" {
		return false
	}
	if m.discoveringContexts[contextName] {
		return false
	}
	if m.discoveryRefreshedContexts[contextName] {
		return false
	}
	return true
}

// markDiscoveryStarted records that a discovery call is now in flight for
// contextName so subsequent shouldFireDiscoveryFor calls deduplicate. Safe
// to call when discoveringContexts is nil — the function is the canonical
// place to lazily allocate the map.
func (m *Model) markDiscoveryStarted(contextName string) {
	if contextName == "" {
		return
	}
	if m.discoveringContexts == nil {
		m.discoveringContexts = make(map[string]bool)
	}
	m.discoveringContexts[contextName] = true
}
