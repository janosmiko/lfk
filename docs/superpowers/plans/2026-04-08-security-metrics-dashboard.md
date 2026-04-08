# Security Metrics Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Phase 1 of the security metrics dashboard from `docs/superpowers/specs/2026-04-08-security-metrics-design.md`: a `#` hotkey that opens a dependency-gated Security dashboard as a pseudo-resource in the middle column, aggregating findings from a built-in heuristic source and the Trivy Operator CRDs, with per-resource indicators and an action-menu/hotkey entry.

**Architecture:** A new `internal/security` package defines a 3-method `SecuritySource` interface and a `Manager` that fans out `Fetch` calls concurrently. Two source adapters (heuristic, trivyop) live in sub-packages. The UI layer adds a `__security__` pseudo-resource registered in `internal/model/resource_lookup.go` and a pure-function renderer in `internal/ui/securityview.go`. The `internal/app` package adds a key handler, async `tea.Cmd`, message types, and a preview-pane dispatcher that mirrors the existing `__monitoring__` wiring but with interactive `SecurityViewState` on the Model (closer to the log viewer than the string-based monitoring preview).

**Tech Stack:** Go 1.26.2, Bubbletea 1.3.10, Lipgloss 1.1.0, client-go v0.35.3 (dynamic client, fake dynamic client for tests), stretchr/testify for assertions, existing `internal/logger` for structured logs.

**Worktree:** Implementation should run in a dedicated git worktree branched from `main`. Create one with `git worktree add ../lfk-security-dashboard feat/security-dashboard` before starting Task A1.

**Reference documents:**
- Spec: `docs/superpowers/specs/2026-04-08-security-metrics-design.md`
- Existing monitoring pattern for reference: `internal/app/commands_dashboard.go:638` (`loadMonitoringDashboard`), `internal/app/view_right.go:258` (`__monitoring__` render), `internal/app/update_keys_actions.go:597` (`handleExplorerActionKeyMonitoring`)
- Existing `__monitoring__` pseudo-resource: `internal/model/resource_lookup.go:30-36`
- Existing keybindings: `internal/ui/config.go:58` (field) and `internal/ui/config.go:123` (default)

---

## File Structure

### New files

| Path | Responsibility |
|---|---|
| `internal/security/source.go` | Core types (`Category`, `Severity`, `ResourceRef`, `Finding`) and the `SecuritySource` interface |
| `internal/security/source_test.go` | Unit tests for type behavior |
| `internal/security/manager.go` | `Manager` with source registration, concurrent fetch, caching, and `FindingIndex` |
| `internal/security/manager_test.go` | Manager tests using fake sources |
| `internal/security/testing.go` | Shared `FakeSource` helper for cross-package tests (only exported within the module) |
| `internal/security/heuristic/heuristic.go` | `heuristic.Source` adapter — always available, walks pod specs |
| `internal/security/heuristic/checks.go` | Pure check functions over `*corev1.Pod` |
| `internal/security/heuristic/heuristic_test.go` | Table-driven tests per check |
| `internal/security/heuristic/fetch_test.go` | Integration test for `Fetch()` using the fake k8s client |
| `internal/security/trivyop/trivyop.go` | `trivyop.Source` adapter reading Trivy Operator CRDs |
| `internal/security/trivyop/parse.go` | Parsers for `VulnerabilityReport` and `ConfigAuditReport` |
| `internal/security/trivyop/trivyop_test.go` | Tests using `dynamic/fake.NewSimpleDynamicClient` |
| `internal/ui/securityview.go` | Pure-function rendering for the dashboard |
| `internal/ui/securityview_test.go` | Golden-style rendering tests |
| `internal/app/messages_security.go` | `tea.Msg` types for security data |
| `internal/app/commands_security.go` | `loadSecurityDashboard` and related `tea.Cmd` wrappers |
| `internal/app/commands_security_test.go` | Command wrapper tests |
| `internal/app/update_security.go` | Key handling for the security preview and handler wiring |
| `internal/app/update_security_test.go` | Key handler tests |

### Modified files

| Path | Change |
|---|---|
| `internal/model/resource_lookup.go:30-36` | Add `__security__` pseudo-item after Monitoring |
| `internal/ui/config.go:58` | Add `Security` and `SecurityResource` fields to `Keybindings`; add `Security` map to the per-cluster config struct |
| `internal/ui/config.go:123` | Add `Security: "#", SecurityResource: "H"` defaults |
| `internal/app/app.go` | Add `SecurityViewState`, `securityPreview`, `securityManager`, `securityFindingIndex` fields to `Model` |
| `internal/app/update_keys_actions.go:70` | Add `case kb.Security` routing to `handleExplorerActionKeySecurity` |
| `internal/app/update_keys_actions.go:597` | Add `handleExplorerActionKeySecurity` next to the monitoring one; add `handleExplorerActionKeySecurityResource` for `H` |
| `internal/app/commands_load_preview.go:47` | Add `__security__` case dispatching `loadSecurityDashboard` |
| `internal/app/view_right.go:258` | Add `__security__` render branch calling `ui.RenderSecurityDashboard` |
| `internal/app/update_navigation.go:243` | Reset `securityViewState` on context switch |
| `internal/model/actions.go` | Add `Security Findings` entry (gated on availability) |
| `internal/app/update_actions.go` | Add handler for the `Security Findings` action |
| `internal/ui/explorer_table.go` + `internal/ui/explorer_format.go` | Add `SEC` column (auto-gated) |
| `internal/ui/help.go` | New Security section in the help screen |
| `internal/ui/hintbar.go` | Add `#` to the explorer hint bar |
| `internal/app/commands.go:42` | Add startup tips for `#` and `H` |
| `README.md` | Update Features and Integrations sections |
| `docs/keybindings.md` | Add `#`, `H`, and the security sub-mode keybindings |
| `docs/config-reference.md` | Add `security:` section next to `monitoring:` |
| `docs/config-example.yaml` | Add commented `security:` block |

---

## Task Index

1. **Phase A — Foundation** (Tasks A1–A4)
2. **Phase B — Heuristic source** (Tasks B1–B8)
3. **Phase C — Trivy Operator source** (Tasks C1–C4)
4. **Phase D — Config & keybindings** (Tasks D1–D2)
5. **Phase E — Model registration** (Task E1)
6. **Phase F — UI rendering** (Tasks F1–F7)
7. **Phase G — App wiring** (Tasks G1–G5)
8. **Phase H — Per-resource integration** (Tasks H1–H5)
9. **Phase I — Docs & polish** (Tasks I1–I3)
10. **Phase J — End-to-end smoke** (Task J1)

Each task is independent once its dependencies are committed. Frequent commits are required — every task ends with a commit.

---

## Phase A — Foundation

### Task A1: Core types and `SecuritySource` interface

**Files:**
- Create: `internal/security/source.go`
- Create: `internal/security/source_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/security/source_test.go
package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeverityOrdering(t *testing.T) {
	assert.True(t, SeverityCritical > SeverityHigh)
	assert.True(t, SeverityHigh > SeverityMedium)
	assert.True(t, SeverityMedium > SeverityLow)
	assert.True(t, SeverityLow > SeverityUnknown)
}

func TestResourceRefKey(t *testing.T) {
	r := ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}
	assert.Equal(t, "prod/Deployment/api", r.Key())
}

func TestResourceRefKeyWithoutContainer(t *testing.T) {
	r := ResourceRef{Namespace: "prod", Kind: "Pod", Name: "api-abc", Container: "main"}
	// Key() intentionally omits container for per-resource aggregation.
	assert.Equal(t, "prod/Pod/api-abc", r.Key())
}

func TestFindingZeroValueSafe(t *testing.T) {
	f := Finding{}
	assert.Equal(t, SeverityUnknown, f.Severity)
	assert.Empty(t, f.Labels)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/security/...`
Expected: FAIL — `package github.com/janosmiko/lfk/internal/security is not in std`

- [ ] **Step 3: Write minimal implementation**

```go
// internal/security/source.go
// Package security defines the abstraction and shared types for pluggable
// security-finding sources. Adapters live under sub-packages (heuristic,
// trivyop, ...) and register with the Manager at startup.
package security

import "context"

// Category groups a finding by kind so the UI can tab/filter on it.
type Category string

const (
	CategoryVuln       Category = "vuln"
	CategoryMisconfig  Category = "misconfig"
	CategoryPolicy     Category = "policy"
	CategoryCompliance Category = "compliance"
)

// Severity is a 4-level scale; sources map their own scales onto these.
type Severity int

const (
	SeverityUnknown Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// ResourceRef identifies the Kubernetes object a finding is attached to.
// Empty ResourceRef means the finding is cluster-scoped.
type ResourceRef struct {
	Namespace string
	Kind      string
	Name      string
	Container string
}

// Key returns a stable identifier used by FindingIndex aggregation.
// Container is intentionally omitted so findings on multiple containers of
// a pod aggregate into a single row in the explorer SEC column.
func (r ResourceRef) Key() string {
	return r.Namespace + "/" + r.Kind + "/" + r.Name
}

// Finding is the denormalized row the UI renders.
type Finding struct {
	ID         string
	Source     string
	Category   Category
	Severity   Severity
	Title      string
	Resource   ResourceRef
	Summary    string
	Details    string
	References []string
	Labels     map[string]string
}

// SecuritySource is the contract every adapter implements.
// Implementations must be goroutine-safe and honor ctx cancellation.
type SecuritySource interface {
	// Name returns a stable, lowercase identifier used for config keys,
	// logging, and the source column in the UI. E.g., "trivy-operator".
	Name() string

	// Categories returns the set of categories this source produces.
	Categories() []Category

	// IsAvailable returns true if the source's dependencies are reachable.
	// Results are cached by the Manager (default TTL 60s). Must return
	// quickly or honor ctx timeout. Errors are treated as "not available"
	// but logged for diagnostics.
	IsAvailable(ctx context.Context, kubeCtx string) (bool, error)

	// Fetch collects findings for the given cluster context and optional
	// namespace filter. Empty namespace means all-namespaces. Must honor
	// ctx cancellation.
	Fetch(ctx context.Context, kubeCtx, namespace string) ([]Finding, error)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/security/...`
Expected: `ok  github.com/janosmiko/lfk/internal/security`

- [ ] **Step 5: Commit**

```bash
git add internal/security/source.go internal/security/source_test.go
git commit -m "feat(security): add SecuritySource interface and core types"
```

---

### Task A2: `Manager` with source registration and concurrent fetch

**Files:**
- Create: `internal/security/manager.go`
- Create: `internal/security/manager_test.go`
- Create: `internal/security/testing.go`

- [ ] **Step 1: Write the fake source helper**

```go
// internal/security/testing.go
package security

import (
	"context"
	"sync/atomic"
	"time"
)

// FakeSource is a test helper implementing SecuritySource with configurable
// responses. Exported for use by test packages within the module.
type FakeSource struct {
	NameStr       string
	CategoriesVal []Category
	Available     bool
	AvailableErr  error
	Findings      []Finding
	FetchErr      error
	FetchDelay    time.Duration
	FetchCalls    atomic.Int32
	AvailCalls    atomic.Int32
}

func (f *FakeSource) Name() string         { return f.NameStr }
func (f *FakeSource) Categories() []Category { return f.CategoriesVal }

func (f *FakeSource) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	f.AvailCalls.Add(1)
	return f.Available, f.AvailableErr
}

func (f *FakeSource) Fetch(ctx context.Context, kubeCtx, namespace string) ([]Finding, error) {
	f.FetchCalls.Add(1)
	if f.FetchDelay > 0 {
		select {
		case <-time.After(f.FetchDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.FetchErr != nil {
		return nil, f.FetchErr
	}
	out := make([]Finding, len(f.Findings))
	copy(out, f.Findings)
	return out, nil
}
```

- [ ] **Step 2: Write the failing Manager tests**

```go
// internal/security/manager_test.go
package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerRegisterAndFetchAll(t *testing.T) {
	m := NewManager()
	s1 := &FakeSource{NameStr: "s1", Available: true, CategoriesVal: []Category{CategoryVuln},
		Findings: []Finding{{ID: "1", Source: "s1", Severity: SeverityHigh}}}
	s2 := &FakeSource{NameStr: "s2", Available: true, CategoriesVal: []Category{CategoryMisconfig},
		Findings: []Finding{{ID: "2", Source: "s2", Severity: SeverityLow}}}
	m.Register(s1)
	m.Register(s2)

	res, err := m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err)
	assert.Len(t, res.Findings, 2)
	assert.Empty(t, res.Errors)
	assert.Equal(t, int32(1), s1.FetchCalls.Load())
	assert.Equal(t, int32(1), s2.FetchCalls.Load())
}

func TestManagerFetchAllParallel(t *testing.T) {
	m := NewManager()
	s1 := &FakeSource{NameStr: "s1", Available: true, FetchDelay: 80 * time.Millisecond}
	s2 := &FakeSource{NameStr: "s2", Available: true, FetchDelay: 80 * time.Millisecond}
	m.Register(s1)
	m.Register(s2)

	start := time.Now()
	_, err := m.FetchAll(context.Background(), "ctx", "")
	elapsed := time.Since(start)

	require.NoError(t, err)
	// If serial, elapsed would be >= 160ms. Parallel should be ~80ms + overhead.
	assert.Less(t, elapsed, 150*time.Millisecond, "sources should fetch in parallel")
}

func TestManagerFetchAllPartialFailure(t *testing.T) {
	m := NewManager()
	good := &FakeSource{NameStr: "good", Available: true,
		Findings: []Finding{{ID: "ok", Source: "good"}}}
	bad := &FakeSource{NameStr: "bad", Available: true, FetchErr: errors.New("boom")}
	m.Register(good)
	m.Register(bad)

	res, err := m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err, "partial failures must not return error")
	assert.Len(t, res.Findings, 1)
	assert.Contains(t, res.Errors, "bad")
	assert.EqualError(t, res.Errors["bad"], "boom")
}

func TestManagerSkipsUnavailableSources(t *testing.T) {
	m := NewManager()
	avail := &FakeSource{NameStr: "avail", Available: true,
		Findings: []Finding{{ID: "ok"}}}
	gone := &FakeSource{NameStr: "gone", Available: false,
		Findings: []Finding{{ID: "should-not-appear"}}}
	m.Register(avail)
	m.Register(gone)

	res, err := m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err)
	assert.Len(t, res.Findings, 1)
	assert.Equal(t, "ok", res.Findings[0].ID)
	assert.Equal(t, int32(0), gone.FetchCalls.Load(),
		"unavailable sources must not be fetched")
}

func TestManagerAnyAvailable(t *testing.T) {
	m := NewManager()
	m.Register(&FakeSource{NameStr: "a", Available: false})
	m.Register(&FakeSource{NameStr: "b", Available: true})
	ok, err := m.AnyAvailable(context.Background(), "ctx")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestManagerCancellation(t *testing.T) {
	m := NewManager()
	m.Register(&FakeSource{NameStr: "slow", Available: true, FetchDelay: 500 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before call
	_, err := m.FetchAll(ctx, "ctx", "")
	// Error is allowed to be nil if cancellation happened cleanly — what matters
	// is we return quickly.
	_ = err
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/security/... -run TestManager`
Expected: FAIL — `undefined: NewManager`

- [ ] **Step 4: Write the Manager implementation**

```go
// internal/security/manager.go
package security

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// FetchResult is the aggregated output of Manager.FetchAll.
type FetchResult struct {
	Findings []Finding
	Errors   map[string]error // source name -> error (nil-safe)
	Sources  []SourceStatus
}

// SourceStatus captures the last known state of a registered source.
type SourceStatus struct {
	Name      string
	Available bool
	Count     int
	LastError error
}

// Manager aggregates SecuritySource instances, runs IsAvailable and Fetch
// concurrently, and exposes an aggregate result. Caching is added in Task A3.
type Manager struct {
	mu      sync.RWMutex
	sources []SecuritySource
}

// NewManager returns an empty Manager. Sources are registered via Register.
func NewManager() *Manager {
	return &Manager{}
}

// Register appends a source. Not safe to call concurrently with FetchAll.
func (m *Manager) Register(s SecuritySource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources = append(m.sources, s)
}

// Sources returns a snapshot of currently registered sources.
func (m *Manager) Sources() []SecuritySource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]SecuritySource, len(m.sources))
	copy(out, m.sources)
	return out
}

// AnyAvailable returns true if at least one registered source reports
// IsAvailable(ctx, kubeCtx) == true.
func (m *Manager) AnyAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	for _, s := range m.Sources() {
		ok, err := s.IsAvailable(ctx, kubeCtx)
		if err != nil {
			continue // treat errors as "not available"
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// FetchAll runs Fetch concurrently across all available sources. Per-source
// errors do not cancel other sources; they are collected in result.Errors.
func (m *Manager) FetchAll(ctx context.Context, kubeCtx, namespace string) (FetchResult, error) {
	sources := m.Sources()

	type sourceResult struct {
		name     string
		findings []Finding
		err      error
	}
	results := make(chan sourceResult, len(sources))

	g, gctx := errgroup.WithContext(ctx)
	for _, s := range sources {
		s := s
		g.Go(func() error {
			ok, err := s.IsAvailable(gctx, kubeCtx)
			if err != nil || !ok {
				results <- sourceResult{name: s.Name()}
				return nil
			}
			findings, ferr := s.Fetch(gctx, kubeCtx, namespace)
			results <- sourceResult{name: s.Name(), findings: findings, err: ferr}
			return nil
		})
	}

	_ = g.Wait() // errgroup is used only for scope; per-source errors are captured in results.
	close(results)

	res := FetchResult{
		Errors: map[string]error{},
	}
	for r := range results {
		if r.err != nil {
			res.Errors[r.name] = r.err
			res.Sources = append(res.Sources, SourceStatus{Name: r.name, LastError: r.err})
			continue
		}
		res.Findings = append(res.Findings, r.findings...)
		res.Sources = append(res.Sources, SourceStatus{
			Name: r.name, Available: true, Count: len(r.findings),
		})
	}
	return res, nil
}
```

- [ ] **Step 5: Add `golang.org/x/sync` to go.mod if missing**

Run: `go mod tidy`
Expected: `go.sum` updated with `golang.org/x/sync` if not already present.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/security/... -run TestManager -race`
Expected: `PASS`

- [ ] **Step 7: Commit**

```bash
git add internal/security/manager.go internal/security/manager_test.go internal/security/testing.go go.mod go.sum
git commit -m "feat(security): add Manager with concurrent fetch across sources"
```

---

### Task A3: Manager caching and context invalidation

**Files:**
- Modify: `internal/security/manager.go`
- Modify: `internal/security/manager_test.go`

- [ ] **Step 1: Write failing cache tests**

Append to `internal/security/manager_test.go`:

```go
func TestManagerCachedFetch(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(200 * time.Millisecond)
	s := &FakeSource{NameStr: "s", Available: true,
		Findings: []Finding{{ID: "x"}}}
	m.Register(s)

	_, err := m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err)
	_, err = m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err)
	assert.Equal(t, int32(1), s.FetchCalls.Load(),
		"second call within TTL should hit cache")
}

func TestManagerForceRefresh(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(1 * time.Hour)
	s := &FakeSource{NameStr: "s", Available: true,
		Findings: []Finding{{ID: "x"}}}
	m.Register(s)

	_, _ = m.FetchAll(context.Background(), "ctx", "")
	_, _ = m.Refresh(context.Background(), "ctx", "")
	assert.Equal(t, int32(2), s.FetchCalls.Load(),
		"Refresh must bypass the cache")
}

func TestManagerInvalidateOnContextChange(t *testing.T) {
	m := NewManager()
	m.SetRefreshTTL(1 * time.Hour)
	s := &FakeSource{NameStr: "s", Available: true,
		Findings: []Finding{{ID: "x"}}}
	m.Register(s)

	_, _ = m.FetchAll(context.Background(), "ctxA", "")
	_, _ = m.FetchAll(context.Background(), "ctxB", "")
	assert.Equal(t, int32(2), s.FetchCalls.Load(),
		"different kubeCtx should bypass cache")
}

func TestManagerAvailabilityCached(t *testing.T) {
	m := NewManager()
	m.SetAvailabilityTTL(200 * time.Millisecond)
	s := &FakeSource{NameStr: "s", Available: true}
	m.Register(s)

	_, _ = m.AnyAvailable(context.Background(), "ctx")
	_, _ = m.AnyAvailable(context.Background(), "ctx")
	assert.Equal(t, int32(1), s.AvailCalls.Load(),
		"availability should be cached within TTL")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/security/... -run TestManagerCached -run TestManagerForce -run TestManagerInvalidate -run TestManagerAvailabilityCached`
Expected: FAIL — undefined `SetRefreshTTL`, `SetAvailabilityTTL`, `Refresh`.

- [ ] **Step 3: Extend Manager with caching**

Replace the `Manager` struct and add cache helpers in `internal/security/manager.go`:

```go
type Manager struct {
	mu      sync.RWMutex
	sources []SecuritySource

	refreshTTL     time.Duration
	availabilityTTL time.Duration

	cacheKey     string // lastCtx + "|" + lastNamespace
	cachedResult FetchResult
	cachedAt     time.Time

	availCache map[string]availEntry // key = kubeCtx
}

type availEntry struct {
	available bool
	at        time.Time
}

// NewManager returns a Manager with sensible cache defaults (30s fetch, 60s availability).
func NewManager() *Manager {
	return &Manager{
		refreshTTL:     30 * time.Second,
		availabilityTTL: 60 * time.Second,
		availCache:     make(map[string]availEntry),
	}
}

// SetRefreshTTL overrides the FetchAll cache TTL. Zero disables caching.
func (m *Manager) SetRefreshTTL(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshTTL = d
}

// SetAvailabilityTTL overrides the AnyAvailable cache TTL.
func (m *Manager) SetAvailabilityTTL(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.availabilityTTL = d
}

// Refresh is FetchAll that always bypasses the cache.
func (m *Manager) Refresh(ctx context.Context, kubeCtx, namespace string) (FetchResult, error) {
	m.mu.Lock()
	m.cacheKey = ""
	m.availCache = make(map[string]availEntry)
	m.mu.Unlock()
	return m.FetchAll(ctx, kubeCtx, namespace)
}
```

Update `FetchAll` to consult and populate the cache at the top of the function:

```go
func (m *Manager) FetchAll(ctx context.Context, kubeCtx, namespace string) (FetchResult, error) {
	cacheKey := kubeCtx + "|" + namespace

	m.mu.RLock()
	if cacheKey == m.cacheKey && m.refreshTTL > 0 && time.Since(m.cachedAt) < m.refreshTTL {
		cached := m.cachedResult
		m.mu.RUnlock()
		return cached, nil
	}
	m.mu.RUnlock()

	// ... existing fetch logic ...

	m.mu.Lock()
	m.cacheKey = cacheKey
	m.cachedResult = res
	m.cachedAt = time.Now()
	m.mu.Unlock()
	return res, nil
}
```

Update `AnyAvailable` to consult and populate its cache:

```go
func (m *Manager) AnyAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	m.mu.RLock()
	if entry, ok := m.availCache[kubeCtx]; ok && time.Since(entry.at) < m.availabilityTTL {
		m.mu.RUnlock()
		return entry.available, nil
	}
	m.mu.RUnlock()

	for _, s := range m.Sources() {
		ok, err := s.IsAvailable(ctx, kubeCtx)
		if err != nil {
			continue
		}
		if ok {
			m.mu.Lock()
			m.availCache[kubeCtx] = availEntry{available: true, at: time.Now()}
			m.mu.Unlock()
			return true, nil
		}
	}
	m.mu.Lock()
	m.availCache[kubeCtx] = availEntry{available: false, at: time.Now()}
	m.mu.Unlock()
	return false, nil
}
```

Add `"time"` to the imports.

- [ ] **Step 4: Run all Manager tests with race detector**

Run: `go test ./internal/security/... -run TestManager -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/manager.go internal/security/manager_test.go
git commit -m "feat(security): add Manager caching for fetch and availability"
```

---

### Task A4: `FindingIndex` for per-resource lookup

**Files:**
- Modify: `internal/security/manager.go`
- Modify: `internal/security/manager_test.go`

- [ ] **Step 1: Write failing index test**

Append to `internal/security/manager_test.go`:

```go
func TestFindingIndexCountsAndLookup(t *testing.T) {
	m := NewManager()
	s := &FakeSource{NameStr: "s", Available: true, Findings: []Finding{
		{ID: "1", Severity: SeverityCritical,
			Resource: ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		{ID: "2", Severity: SeverityHigh,
			Resource: ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		{ID: "3", Severity: SeverityLow,
			Resource: ResourceRef{Namespace: "prod", Kind: "Pod", Name: "db-0"}},
	}}
	m.Register(s)

	_, err := m.FetchAll(context.Background(), "ctx", "")
	require.NoError(t, err)

	idx := m.Index()
	api := idx.For(ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"})
	assert.Equal(t, 1, api.Critical)
	assert.Equal(t, 1, api.High)
	assert.Equal(t, 0, api.Medium)
	assert.Equal(t, 0, api.Low)
	assert.Equal(t, 2, api.Total())
	assert.Equal(t, SeverityCritical, api.Highest())

	empty := idx.For(ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "nope"})
	assert.Equal(t, 0, empty.Total())
	assert.Equal(t, SeverityUnknown, empty.Highest())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/security/... -run TestFindingIndex`
Expected: FAIL — `m.Index undefined`.

- [ ] **Step 3: Add `FindingIndex` types and builder**

Append to `internal/security/manager.go`:

```go
// SeverityCounts holds severity breakdown for a single resource.
type SeverityCounts struct {
	Critical, High, Medium, Low int
}

// Total returns the sum of all severity buckets.
func (c SeverityCounts) Total() int {
	return c.Critical + c.High + c.Medium + c.Low
}

// Highest returns the highest severity present, or SeverityUnknown if none.
func (c SeverityCounts) Highest() Severity {
	switch {
	case c.Critical > 0:
		return SeverityCritical
	case c.High > 0:
		return SeverityHigh
	case c.Medium > 0:
		return SeverityMedium
	case c.Low > 0:
		return SeverityLow
	default:
		return SeverityUnknown
	}
}

// FindingIndex aggregates findings by resource for O(1) per-row lookup.
type FindingIndex struct {
	counts map[string]SeverityCounts
}

// For returns the aggregated counts for the given resource. Zero value when absent.
func (i *FindingIndex) For(ref ResourceRef) SeverityCounts {
	if i == nil {
		return SeverityCounts{}
	}
	return i.counts[ref.Key()]
}

// BuildFindingIndex constructs an index from a slice of findings.
func BuildFindingIndex(findings []Finding) *FindingIndex {
	idx := &FindingIndex{counts: make(map[string]SeverityCounts)}
	for _, f := range findings {
		key := f.Resource.Key()
		c := idx.counts[key]
		switch f.Severity {
		case SeverityCritical:
			c.Critical++
		case SeverityHigh:
			c.High++
		case SeverityMedium:
			c.Medium++
		case SeverityLow:
			c.Low++
		}
		idx.counts[key] = c
	}
	return idx
}
```

Add a cached index field to `Manager`:

```go
type Manager struct {
	// ... existing fields ...
	cachedIndex *FindingIndex
}

// Index returns the FindingIndex for the most recent FetchAll result.
// Returns an empty index if FetchAll has not been called yet.
func (m *Manager) Index() *FindingIndex {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cachedIndex == nil {
		return &FindingIndex{counts: map[string]SeverityCounts{}}
	}
	return m.cachedIndex
}
```

Update `FetchAll` to build the index after a successful fetch:

```go
	m.mu.Lock()
	m.cacheKey = cacheKey
	m.cachedResult = res
	m.cachedAt = time.Now()
	m.cachedIndex = BuildFindingIndex(res.Findings)
	m.mu.Unlock()
```

- [ ] **Step 4: Run the index test**

Run: `go test ./internal/security/... -run TestFindingIndex -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/manager.go internal/security/manager_test.go
git commit -m "feat(security): add FindingIndex for per-resource aggregation"
```

---

## Phase B — Heuristic source

### Task B1: Heuristic scaffold, Source type, and `privileged` check

**Files:**
- Create: `internal/security/heuristic/heuristic.go`
- Create: `internal/security/heuristic/checks.go`
- Create: `internal/security/heuristic/heuristic_test.go`

- [ ] **Step 1: Write the failing privileged-check test**

```go
// internal/security/heuristic/heuristic_test.go
package heuristic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/security"
)

func podWith(container corev1.Container) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "api-abc"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{container}},
	}
}

func boolPtr(b bool) *bool { return &b }

func TestCheckPrivileged(t *testing.T) {
	cases := []struct {
		name      string
		container corev1.Container
		want      int // number of findings produced by checkPrivileged
		wantSev   security.Severity
	}{
		{"privileged true", corev1.Container{
			Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(true)},
		}, 1, security.SeverityCritical},
		{"privileged false", corev1.Container{
			Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(false)},
		}, 0, security.SeverityUnknown},
		{"no security context", corev1.Container{Name: "c"}, 0, security.SeverityUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := podWith(tc.container)
			findings := checkPrivileged(pod, tc.container)
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, tc.wantSev, findings[0].Severity)
				assert.Equal(t, security.CategoryMisconfig, findings[0].Category)
				assert.Equal(t, "privileged", findings[0].Labels["check"])
			}
		})
	}
}

func TestSourceMetadata(t *testing.T) {
	s := New()
	assert.Equal(t, "heuristic", s.Name())
	assert.Equal(t, []security.Category{security.CategoryMisconfig}, s.Categories())
	ok, err := s.IsAvailable(nil, "")
	assert.NoError(t, err)
	assert.True(t, ok, "heuristic is always available")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/security/heuristic/...`
Expected: FAIL — `undefined: New`, `undefined: checkPrivileged`.

- [ ] **Step 3: Write the package scaffold and privileged check**

```go
// internal/security/heuristic/heuristic.go
// Package heuristic implements a zero-dependency security.SecuritySource that
// walks Pod specs and produces findings for common workload hardening issues.
package heuristic

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// Source is the heuristic SecuritySource implementation.
type Source struct{}

// New returns a new heuristic source.
func New() *Source { return &Source{} }

// Name returns the stable identifier.
func (s *Source) Name() string { return "heuristic" }

// Categories returns the categories this source contributes to.
func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryMisconfig}
}

// IsAvailable is always true for heuristic — it has no external dependencies.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	return true, nil
}

// checkFn is the signature all heuristic checks implement.
type checkFn func(pod *corev1.Pod, c corev1.Container) []security.Finding

// allChecks is populated by check files via the init pattern below.
var allChecks = []checkFn{
	checkPrivileged,
}
```

```go
// internal/security/heuristic/checks.go
package heuristic

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/janosmiko/lfk/internal/security"
)

func baseRef(pod *corev1.Pod, container corev1.Container) security.ResourceRef {
	return security.ResourceRef{
		Namespace: pod.Namespace,
		Kind:      "Pod",
		Name:      pod.Name,
		Container: container.Name,
	}
}

func makeFinding(pod *corev1.Pod, container corev1.Container, check string, sev security.Severity, title, summary string) security.Finding {
	return security.Finding{
		ID:       fmt.Sprintf("heuristic/%s/%s/%s/%s", pod.Namespace, pod.Name, container.Name, check),
		Source:   "heuristic",
		Category: security.CategoryMisconfig,
		Severity: sev,
		Title:    title,
		Resource: baseRef(pod, container),
		Summary:  summary,
		Labels:   map[string]string{"check": check, "container": container.Name},
	}
}

// checkPrivileged flags containers running with Privileged: true.
func checkPrivileged(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if c.SecurityContext == nil || c.SecurityContext.Privileged == nil {
		return nil
	}
	if !*c.SecurityContext.Privileged {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "privileged", security.SeverityCritical,
		"privileged container",
		fmt.Sprintf("Container %q runs in privileged mode, enabling full host access and container escape risk.", c.Name))}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/security/heuristic/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/heuristic/
git commit -m "feat(security/heuristic): add source scaffold and privileged check"
```

---

### Task B2: Host namespace checks (hostPID/hostNetwork/hostIPC)

**Files:**
- Modify: `internal/security/heuristic/checks.go`
- Modify: `internal/security/heuristic/heuristic.go`
- Modify: `internal/security/heuristic/heuristic_test.go`

- [ ] **Step 1: Write failing tests**

Append to `heuristic_test.go`:

```go
func TestCheckHostNamespaces(t *testing.T) {
	cases := []struct {
		name    string
		spec    corev1.PodSpec
		wantIDs []string // values of the "check" label
	}{
		{"hostPID", corev1.PodSpec{HostPID: true, Containers: []corev1.Container{{Name: "c"}}}, []string{"host_pid"}},
		{"hostNetwork", corev1.PodSpec{HostNetwork: true, Containers: []corev1.Container{{Name: "c"}}}, []string{"host_network"}},
		{"hostIPC", corev1.PodSpec{HostIPC: true, Containers: []corev1.Container{{Name: "c"}}}, []string{"host_ipc"}},
		{"all three", corev1.PodSpec{
			HostPID: true, HostNetwork: true, HostIPC: true,
			Containers: []corev1.Container{{Name: "c"}},
		}, []string{"host_pid", "host_network", "host_ipc"}},
		{"none", corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
				Spec:       tc.spec,
			}
			findings := checkHostNamespaces(pod, pod.Spec.Containers[0])
			gotIDs := make([]string, 0, len(findings))
			for _, f := range findings {
				gotIDs = append(gotIDs, f.Labels["check"])
				assert.Equal(t, security.SeverityHigh, f.Severity)
			}
			assert.ElementsMatch(t, tc.wantIDs, gotIDs)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/security/heuristic/... -run TestCheckHostNamespaces`
Expected: FAIL — `undefined: checkHostNamespaces`.

- [ ] **Step 3: Add the check**

Append to `checks.go`:

```go
// checkHostNamespaces flags pods that share the host's PID/network/IPC namespaces.
// Only runs once per pod — emits its findings on the first container it sees.
func checkHostNamespaces(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if pod.Spec.Containers[0].Name != c.Name {
		return nil // only emit for the first container to avoid duplication
	}
	var findings []security.Finding
	if pod.Spec.HostPID {
		findings = append(findings, makeFinding(pod, c, "host_pid", security.SeverityHigh,
			"hostPID enabled", "Pod shares the host PID namespace, exposing all host processes."))
	}
	if pod.Spec.HostNetwork {
		findings = append(findings, makeFinding(pod, c, "host_network", security.SeverityHigh,
			"hostNetwork enabled", "Pod shares the host network namespace, bypassing NetworkPolicies."))
	}
	if pod.Spec.HostIPC {
		findings = append(findings, makeFinding(pod, c, "host_ipc", security.SeverityHigh,
			"hostIPC enabled", "Pod shares the host IPC namespace, exposing host SYSV IPC objects."))
	}
	return findings
}
```

Register the check in `heuristic.go`:

```go
var allChecks = []checkFn{
	checkPrivileged,
	checkHostNamespaces,
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/security/heuristic/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/heuristic/
git commit -m "feat(security/heuristic): add host namespace sharing checks"
```

---

### Task B3: `hostPath` and `readOnlyRootFilesystem` checks

**Files:**
- Modify: `internal/security/heuristic/checks.go`
- Modify: `internal/security/heuristic/heuristic.go`
- Modify: `internal/security/heuristic/heuristic_test.go`

- [ ] **Step 1: Write failing tests**

Append to `heuristic_test.go`:

```go
func TestCheckHostPath(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c"}},
			Volumes: []corev1.Volume{
				{Name: "etc", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/etc"},
				}},
				{Name: "data", VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			},
		},
	}
	findings := checkHostPath(pod, pod.Spec.Containers[0])
	assert.Len(t, findings, 1)
	assert.Equal(t, security.SeverityHigh, findings[0].Severity)
	assert.Equal(t, "host_path", findings[0].Labels["check"])
	assert.Contains(t, findings[0].Summary, "/etc")
}

func TestCheckReadOnlyRootFilesystem(t *testing.T) {
	writable := corev1.Container{Name: "c"}
	explicitFalse := corev1.Container{Name: "c", SecurityContext: &corev1.SecurityContext{
		ReadOnlyRootFilesystem: boolPtr(false),
	}}
	readOnly := corev1.Container{Name: "c", SecurityContext: &corev1.SecurityContext{
		ReadOnlyRootFilesystem: boolPtr(true),
	}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}

	assert.Len(t, checkReadOnlyRootFilesystem(pod, writable), 1)
	assert.Len(t, checkReadOnlyRootFilesystem(pod, explicitFalse), 1)
	assert.Len(t, checkReadOnlyRootFilesystem(pod, readOnly), 0)
}
```

- [ ] **Step 2: Run tests — expect fail**

Run: `go test ./internal/security/heuristic/... -run "TestCheckHostPath|TestCheckReadOnly"`
Expected: FAIL — undefined functions.

- [ ] **Step 3: Add checks**

Append to `checks.go`:

```go
// checkHostPath flags pods that mount hostPath volumes. Only emits once per pod
// (bound to the first container) because volumes are pod-level.
func checkHostPath(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if pod.Spec.Containers[0].Name != c.Name {
		return nil
	}
	var findings []security.Finding
	for _, v := range pod.Spec.Volumes {
		if v.HostPath == nil {
			continue
		}
		findings = append(findings, makeFinding(pod, c, "host_path", security.SeverityHigh,
			"hostPath volume mount",
			fmt.Sprintf("Volume %q mounts host path %q, granting node filesystem access.", v.Name, v.HostPath.Path)))
	}
	return findings
}

// checkReadOnlyRootFilesystem flags containers with a writable root filesystem.
func checkReadOnlyRootFilesystem(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if c.SecurityContext != nil && c.SecurityContext.ReadOnlyRootFilesystem != nil && *c.SecurityContext.ReadOnlyRootFilesystem {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "readonly_root_fs", security.SeverityLow,
		"writable root filesystem",
		fmt.Sprintf("Container %q has a writable root filesystem. Prefer readOnlyRootFilesystem: true with emptyDir for temp.", c.Name))}
}
```

Register in `heuristic.go`:

```go
var allChecks = []checkFn{
	checkPrivileged,
	checkHostNamespaces,
	checkHostPath,
	checkReadOnlyRootFilesystem,
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/security/heuristic/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/heuristic/
git commit -m "feat(security/heuristic): add hostPath and readOnlyRootFilesystem checks"
```

---

### Task B4: `runAsRoot` and `allowPrivilegeEscalation` checks

**Files:**
- Modify: `internal/security/heuristic/checks.go`
- Modify: `internal/security/heuristic/heuristic.go`
- Modify: `internal/security/heuristic/heuristic_test.go`

- [ ] **Step 1: Write failing tests**

Append to `heuristic_test.go`:

```go
func int64Ptr(i int64) *int64 { return &i }

func TestCheckRunAsRoot(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
	cases := []struct {
		name      string
		pod       corev1.PodSecurityContext
		container corev1.SecurityContext
		want      int
	}{
		{"no context -> flag", corev1.PodSecurityContext{}, corev1.SecurityContext{}, 1},
		{"runAsUser:0 -> flag", corev1.PodSecurityContext{}, corev1.SecurityContext{RunAsUser: int64Ptr(0)}, 1},
		{"runAsUser:1000 -> clean", corev1.PodSecurityContext{}, corev1.SecurityContext{RunAsUser: int64Ptr(1000)}, 0},
		{"pod runAsNonRoot:true -> clean", corev1.PodSecurityContext{RunAsNonRoot: boolPtr(true)}, corev1.SecurityContext{}, 0},
		{"container runAsNonRoot:true -> clean", corev1.PodSecurityContext{}, corev1.SecurityContext{RunAsNonRoot: boolPtr(true)}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := pod.DeepCopy()
			p.Spec.SecurityContext = &tc.pod
			c := corev1.Container{Name: "c", SecurityContext: &tc.container}
			p.Spec.Containers = []corev1.Container{c}
			findings := checkRunAsRoot(p, c)
			assert.Len(t, findings, tc.want)
		})
	}
}

func TestCheckAllowPrivilegeEscalation(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
	cases := []struct {
		name string
		sc   *corev1.SecurityContext
		want int
	}{
		{"nil context -> flag", nil, 1},
		{"unset -> flag", &corev1.SecurityContext{}, 1},
		{"true -> flag", &corev1.SecurityContext{AllowPrivilegeEscalation: boolPtr(true)}, 1},
		{"false -> clean", &corev1.SecurityContext{AllowPrivilegeEscalation: boolPtr(false)}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := corev1.Container{Name: "c", SecurityContext: tc.sc}
			findings := checkAllowPrivilegeEscalation(pod, c)
			assert.Len(t, findings, tc.want)
		})
	}
}
```

- [ ] **Step 2: Run tests — expect fail**

Run: `go test ./internal/security/heuristic/... -run "TestCheckRunAsRoot|TestCheckAllowPrivilege"`
Expected: FAIL — undefined.

- [ ] **Step 3: Add checks**

Append to `checks.go`:

```go
// checkRunAsRoot flags containers that may run as uid 0.
func checkRunAsRoot(pod *corev1.Pod, c corev1.Container) []security.Finding {
	// If either pod-level or container-level context guarantees non-root, clean.
	if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsNonRoot != nil && *pod.Spec.SecurityContext.RunAsNonRoot {
		return nil
	}
	if c.SecurityContext != nil && c.SecurityContext.RunAsNonRoot != nil && *c.SecurityContext.RunAsNonRoot {
		return nil
	}
	// Non-zero runAsUser on the container is fine.
	if c.SecurityContext != nil && c.SecurityContext.RunAsUser != nil && *c.SecurityContext.RunAsUser != 0 {
		return nil
	}
	// Pod-level non-zero is also fine if the container does not override.
	if (c.SecurityContext == nil || c.SecurityContext.RunAsUser == nil) &&
		pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsUser != nil && *pod.Spec.SecurityContext.RunAsUser != 0 {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "run_as_root", security.SeverityMedium,
		"container may run as root",
		fmt.Sprintf("Container %q has no explicit non-root guarantee (runAsNonRoot: true or runAsUser > 0).", c.Name))}
}

// checkAllowPrivilegeEscalation flags containers that can escalate privileges.
func checkAllowPrivilegeEscalation(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if c.SecurityContext != nil && c.SecurityContext.AllowPrivilegeEscalation != nil && !*c.SecurityContext.AllowPrivilegeEscalation {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "allow_priv_esc", security.SeverityMedium,
		"privilege escalation allowed",
		fmt.Sprintf("Container %q does not set allowPrivilegeEscalation: false. Setuid binaries can elevate.", c.Name))}
}
```

Register in `heuristic.go`:

```go
var allChecks = []checkFn{
	checkPrivileged,
	checkHostNamespaces,
	checkHostPath,
	checkReadOnlyRootFilesystem,
	checkRunAsRoot,
	checkAllowPrivilegeEscalation,
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/security/heuristic/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/heuristic/
git commit -m "feat(security/heuristic): add runAsRoot and allowPrivilegeEscalation checks"
```

---

### Task B5: Dangerous capability checks

**Files:**
- Modify: `internal/security/heuristic/checks.go`
- Modify: `internal/security/heuristic/heuristic.go`
- Modify: `internal/security/heuristic/heuristic_test.go`

- [ ] **Step 1: Write failing test**

Append to `heuristic_test.go`:

```go
func TestCheckDangerousCapabilities(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
	cases := []struct {
		name     string
		caps     *corev1.Capabilities
		want     int
		wantCaps []string
	}{
		{"nil -> clean", nil, 0, nil},
		{"safe caps -> clean", &corev1.Capabilities{Add: []corev1.Capability{"NET_BIND_SERVICE"}}, 0, nil},
		{"SYS_ADMIN -> flag", &corev1.Capabilities{Add: []corev1.Capability{"SYS_ADMIN"}}, 1, []string{"SYS_ADMIN"}},
		{"NET_ADMIN -> flag", &corev1.Capabilities{Add: []corev1.Capability{"NET_ADMIN"}}, 1, []string{"NET_ADMIN"}},
		{"ALL -> flag", &corev1.Capabilities{Add: []corev1.Capability{"ALL"}}, 1, []string{"ALL"}},
		{"multiple dangerous -> flag", &corev1.Capabilities{Add: []corev1.Capability{"SYS_ADMIN", "NET_ADMIN"}}, 2, []string{"SYS_ADMIN", "NET_ADMIN"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := corev1.Container{Name: "c", SecurityContext: &corev1.SecurityContext{Capabilities: tc.caps}}
			findings := checkDangerousCapabilities(pod, c)
			assert.Len(t, findings, tc.want)
			for i, f := range findings {
				assert.Equal(t, security.SeverityHigh, f.Severity)
				assert.Contains(t, f.Summary, tc.wantCaps[i])
			}
		})
	}
}
```

- [ ] **Step 2: Run test — expect fail**

Run: `go test ./internal/security/heuristic/... -run TestCheckDangerous`
Expected: FAIL — `undefined: checkDangerousCapabilities`.

- [ ] **Step 3: Add the check**

Append to `checks.go`:

```go
var dangerousCapabilities = map[corev1.Capability]bool{
	"SYS_ADMIN":  true,
	"NET_ADMIN":  true,
	"SYS_PTRACE": true,
	"SYS_MODULE": true,
	"NET_RAW":    true,
	"ALL":        true,
}

// checkDangerousCapabilities flags containers adding known-dangerous capabilities.
func checkDangerousCapabilities(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if c.SecurityContext == nil || c.SecurityContext.Capabilities == nil {
		return nil
	}
	var findings []security.Finding
	for _, cap := range c.SecurityContext.Capabilities.Add {
		if !dangerousCapabilities[cap] {
			continue
		}
		findings = append(findings, makeFinding(pod, c, "dangerous_caps_"+string(cap), security.SeverityHigh,
			"dangerous capability added",
			fmt.Sprintf("Container %q adds capability %q. Drop it unless strictly required.", c.Name, cap)))
	}
	return findings
}
```

Register in `heuristic.go`:

```go
var allChecks = []checkFn{
	checkPrivileged,
	checkHostNamespaces,
	checkHostPath,
	checkReadOnlyRootFilesystem,
	checkRunAsRoot,
	checkAllowPrivilegeEscalation,
	checkDangerousCapabilities,
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/security/heuristic/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/heuristic/
git commit -m "feat(security/heuristic): add dangerous capabilities check"
```

---

### Task B6: Resource limit check

**Files:**
- Modify: `internal/security/heuristic/checks.go`
- Modify: `internal/security/heuristic/heuristic.go`
- Modify: `internal/security/heuristic/heuristic_test.go`

- [ ] **Step 1: Write failing test**

Append to `heuristic_test.go`:

```go
func TestCheckResourceLimits(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
	resCPU := resource.MustParse("100m")
	resMem := resource.MustParse("128Mi")

	cases := []struct {
		name string
		res  corev1.ResourceRequirements
		want int
	}{
		{"no limits", corev1.ResourceRequirements{}, 1},
		{"cpu only", corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resCPU}}, 1},
		{"memory only", corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resMem}}, 1},
		{"both set", corev1.ResourceRequirements{Limits: corev1.ResourceList{
			corev1.ResourceCPU: resCPU, corev1.ResourceMemory: resMem,
		}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := corev1.Container{Name: "c", Resources: tc.res}
			findings := checkResourceLimits(pod, c)
			assert.Len(t, findings, tc.want)
		})
	}
}
```

Add import to the test file if not already present:

```go
import "k8s.io/apimachinery/pkg/api/resource"
```

- [ ] **Step 2: Run test — expect fail**

Run: `go test ./internal/security/heuristic/... -run TestCheckResourceLimits`
Expected: FAIL — undefined.

- [ ] **Step 3: Add the check**

Append to `checks.go`:

```go
// checkResourceLimits flags containers missing CPU or memory limits.
func checkResourceLimits(pod *corev1.Pod, c corev1.Container) []security.Finding {
	cpu, hasCPU := c.Resources.Limits[corev1.ResourceCPU]
	mem, hasMem := c.Resources.Limits[corev1.ResourceMemory]
	if hasCPU && !cpu.IsZero() && hasMem && !mem.IsZero() {
		return nil
	}
	var missing []string
	if !hasCPU || cpu.IsZero() {
		missing = append(missing, "cpu")
	}
	if !hasMem || mem.IsZero() {
		missing = append(missing, "memory")
	}
	return []security.Finding{makeFinding(pod, c, "missing_resource_limits", security.SeverityLow,
		"missing resource limits",
		fmt.Sprintf("Container %q is missing resource limits (%v). Unbounded containers can DoS the node.", c.Name, missing))}
}
```

Register in `heuristic.go`:

```go
var allChecks = []checkFn{
	checkPrivileged,
	checkHostNamespaces,
	checkHostPath,
	checkReadOnlyRootFilesystem,
	checkRunAsRoot,
	checkAllowPrivilegeEscalation,
	checkDangerousCapabilities,
	checkResourceLimits,
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/security/heuristic/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/heuristic/
git commit -m "feat(security/heuristic): add resource limits check"
```

---

### Task B7: Default ServiceAccount and latest-tag checks

**Files:**
- Modify: `internal/security/heuristic/checks.go`
- Modify: `internal/security/heuristic/heuristic.go`
- Modify: `internal/security/heuristic/heuristic_test.go`

- [ ] **Step 1: Write failing tests**

Append to `heuristic_test.go`:

```go
func TestCheckDefaultServiceAccount(t *testing.T) {
	cases := []struct {
		name string
		sa   string
		want int
	}{
		{"empty (defaults to default) -> flag", "", 1},
		{"explicit default -> flag", "default", 1},
		{"custom -> clean", "api-sa", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
				Spec:       corev1.PodSpec{ServiceAccountName: tc.sa, Containers: []corev1.Container{{Name: "c"}}},
			}
			findings := checkDefaultServiceAccount(pod, pod.Spec.Containers[0])
			assert.Len(t, findings, tc.want)
		})
	}
}

func TestCheckLatestImageTag(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"}}
	cases := []struct {
		name, image string
		want        int
	}{
		{"latest tag", "nginx:latest", 1},
		{"no tag", "nginx", 1},
		{"specific tag", "nginx:1.25.3", 0},
		{"digest", "nginx@sha256:abcdef", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := corev1.Container{Name: "c", Image: tc.image}
			findings := checkLatestImageTag(pod, c)
			assert.Len(t, findings, tc.want)
		})
	}
}
```

- [ ] **Step 2: Run tests — expect fail**

Run: `go test ./internal/security/heuristic/... -run "TestCheckDefault|TestCheckLatestImage"`
Expected: FAIL — undefined.

- [ ] **Step 3: Add checks**

Append to `checks.go`:

```go
import "strings"

// checkDefaultServiceAccount flags pods using the namespace's default service account.
// Emits once per pod (bound to first container).
func checkDefaultServiceAccount(pod *corev1.Pod, c corev1.Container) []security.Finding {
	if pod.Spec.Containers[0].Name != c.Name {
		return nil
	}
	sa := pod.Spec.ServiceAccountName
	if sa != "" && sa != "default" {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "default_sa", security.SeverityLow,
		"uses default ServiceAccount",
		"Pod uses the default ServiceAccount. Create a dedicated SA with minimal RBAC.")}
}

// checkLatestImageTag flags containers without a pinned tag.
func checkLatestImageTag(pod *corev1.Pod, c corev1.Container) []security.Finding {
	img := c.Image
	// Digest-pinned images are fine.
	if strings.Contains(img, "@sha256:") {
		return nil
	}
	// Extract tag portion (the last colon after the final slash).
	last := strings.LastIndex(img, "/")
	rest := img
	if last >= 0 {
		rest = img[last+1:]
	}
	colon := strings.Index(rest, ":")
	tag := ""
	if colon >= 0 {
		tag = rest[colon+1:]
	}
	if tag != "" && tag != "latest" {
		return nil
	}
	return []security.Finding{makeFinding(pod, c, "latest_tag", security.SeverityLow,
		"unpinned image tag",
		fmt.Sprintf("Container %q uses image %q without a pinned version. Pin by tag or digest.", c.Name, img))}
}
```

Add `"strings"` to the file's top imports (merge with the existing `fmt` import group).

Register in `heuristic.go`:

```go
var allChecks = []checkFn{
	checkPrivileged,
	checkHostNamespaces,
	checkHostPath,
	checkReadOnlyRootFilesystem,
	checkRunAsRoot,
	checkAllowPrivilegeEscalation,
	checkDangerousCapabilities,
	checkResourceLimits,
	checkDefaultServiceAccount,
	checkLatestImageTag,
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/security/heuristic/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/heuristic/
git commit -m "feat(security/heuristic): add default SA and latest-tag checks"
```

---

### Task B8: `Fetch()` — walk pod list and run all checks

**Files:**
- Modify: `internal/security/heuristic/heuristic.go`
- Create: `internal/security/heuristic/fetch_test.go`

- [ ] **Step 1: Write failing fetch test**

```go
// internal/security/heuristic/fetch_test.go
package heuristic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/security"
)

func TestSourceFetch(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "bad"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "c", Image: "nginx:latest",
					SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(true)},
				}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "clean"},
			Spec: corev1.PodSpec{
				ServiceAccountName: "api-sa",
				Containers: []corev1.Container{{
					Name: "c", Image: "nginx:1.25.3@sha256:abcdef",
					SecurityContext: &corev1.SecurityContext{
						Privileged:               boolPtr(false),
						AllowPrivilegeEscalation: boolPtr(false),
						ReadOnlyRootFilesystem:   boolPtr(true),
						RunAsNonRoot:             boolPtr(true),
					},
					Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resourceQuantity("100m"),
						corev1.ResourceMemory: resourceQuantity("128Mi"),
					}},
				}},
			},
		},
	)

	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)

	// "bad" pod produces multiple findings, "clean" produces none.
	badCount := 0
	cleanCount := 0
	for _, f := range findings {
		switch f.Resource.Name {
		case "bad":
			badCount++
		case "clean":
			cleanCount++
		}
	}
	assert.Greater(t, badCount, 0)
	assert.Equal(t, 0, cleanCount)

	// All findings should come from our source.
	for _, f := range findings {
		assert.Equal(t, "heuristic", f.Source)
		assert.Equal(t, security.CategoryMisconfig, f.Category)
	}
}

func TestSourceFetchNamespaceFilter(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p1"},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(true)}}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "staging", Name: "p2"},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(true)}}}},
		},
	)

	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "prod")
	require.NoError(t, err)
	for _, f := range findings {
		assert.Equal(t, "prod", f.Resource.Namespace)
	}
}
```

Add helper to `heuristic_test.go` (or a shared test util file):

```go
import "k8s.io/apimachinery/pkg/api/resource"

func resourceQuantity(s string) resource.Quantity { return resource.MustParse(s) }
```

- [ ] **Step 2: Run test — expect fail**

Run: `go test ./internal/security/heuristic/... -run TestSourceFetch`
Expected: FAIL — `undefined: NewWithClient`.

- [ ] **Step 3: Extend the Source to accept a kubernetes.Interface**

Replace the package body of `heuristic.go`:

```go
// internal/security/heuristic/heuristic.go
package heuristic

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/janosmiko/lfk/internal/security"
)

// Source is the heuristic SecuritySource implementation.
type Source struct {
	client kubernetes.Interface
}

// New returns a heuristic source with no client. Fetch returns an empty slice.
// Callers must prefer NewWithClient when they have a kubernetes client.
func New() *Source { return &Source{} }

// NewWithClient returns a heuristic source that lists pods via the given client.
func NewWithClient(client kubernetes.Interface) *Source {
	return &Source{client: client}
}

func (s *Source) Name() string { return "heuristic" }

func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryMisconfig}
}

func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	return s.client != nil, nil
}

// Fetch lists pods in the given namespace (empty = all namespaces) and runs
// every registered check against every container.
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.client == nil {
		return nil, nil
	}
	list, err := s.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var findings []security.Finding
	for i := range list.Items {
		pod := &list.Items[i]
		for _, c := range pod.Spec.Containers {
			for _, check := range allChecks {
				findings = append(findings, check(pod, c)...)
			}
		}
	}
	return findings, nil
}

type checkFn func(pod *corev1.Pod, c corev1.Container) []security.Finding
```

Note: `checkFn` is redeclared here; remove its previous declaration. The `allChecks` slice stays in `checks.go`.

Move the `allChecks` declaration from `heuristic.go` to the bottom of `checks.go`:

```go
// allChecks is the ordered list of checks the Source runs against each container.
var allChecks = []checkFn{
	checkPrivileged,
	checkHostNamespaces,
	checkHostPath,
	checkReadOnlyRootFilesystem,
	checkRunAsRoot,
	checkAllowPrivilegeEscalation,
	checkDangerousCapabilities,
	checkResourceLimits,
	checkDefaultServiceAccount,
	checkLatestImageTag,
}
```

Also fix the `TestSourceMetadata` test in `heuristic_test.go`: `New()` now returns a source with a nil client, so `IsAvailable` returns `false`. Update the test to use `NewWithClient(fake.NewSimpleClientset())` and import the fake client.

- [ ] **Step 4: Run all heuristic tests**

Run: `go test ./internal/security/heuristic/... -race -cover`
Expected: `PASS`, coverage > 90% for the package.

- [ ] **Step 5: Commit**

```bash
git add internal/security/heuristic/
git commit -m "feat(security/heuristic): add Fetch that walks pods and runs all checks"
```

---

## Phase C — Trivy Operator source

### Task C1: TrivyOp scaffold, IsAvailable via CRD GVR lookup

**Files:**
- Create: `internal/security/trivyop/trivyop.go`
- Create: `internal/security/trivyop/trivyop_test.go`

- [ ] **Step 1: Write failing availability test**

```go
// internal/security/trivyop/trivyop_test.go
package trivyop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

var vulnReportGVR = schema.GroupVersionResource{
	Group: "aquasecurity.github.io", Version: "v1alpha1", Resource: "vulnerabilityreports",
}

var configAuditGVR = schema.GroupVersionResource{
	Group: "aquasecurity.github.io", Version: "v1alpha1", Resource: "configauditreports",
}

func newFakeDyn(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			vulnReportGVR:    "VulnerabilityReportList",
			configAuditGVR:   "ConfigAuditReportList",
		},
		objects...,
	)
}

func TestIsAvailableCRDPresent(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "aquasecurity.github.io", Version: "v1alpha1", Kind: "VulnerabilityReport",
	})
	obj.SetNamespace("prod")
	obj.SetName("probe")

	client := newFakeDyn(obj)
	s := NewWithDynamic(client)
	ok, err := s.IsAvailable(context.Background(), "")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestIsAvailableCRDAbsent(t *testing.T) {
	// Empty fake client — list will succeed but return no items; we treat
	// "list works without 'no matches for kind' error" as available. An empty
	// cluster with the CRD installed is still available.
	client := newFakeDyn()
	s := NewWithDynamic(client)
	ok, err := s.IsAvailable(context.Background(), "")
	require.NoError(t, err)
	assert.True(t, ok, "empty list means the CRD exists; source is available")
	_ = metav1.ObjectMeta{} // keep import
}
```

- [ ] **Step 2: Run test — expect fail**

Run: `go test ./internal/security/trivyop/...`
Expected: FAIL — `package github.com/janosmiko/lfk/internal/security/trivyop is not in std`.

- [ ] **Step 3: Scaffold the package**

```go
// internal/security/trivyop/trivyop.go
// Package trivyop reads Trivy Operator CRDs (VulnerabilityReport and
// ConfigAuditReport) and exposes them as security.Findings.
package trivyop

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/security"
)

// GVRs for the Trivy Operator CRDs Phase 1 consumes.
var (
	VulnerabilityReportGVR = schema.GroupVersionResource{
		Group: "aquasecurity.github.io", Version: "v1alpha1", Resource: "vulnerabilityreports",
	}
	ConfigAuditReportGVR = schema.GroupVersionResource{
		Group: "aquasecurity.github.io", Version: "v1alpha1", Resource: "configauditreports",
	}
)

// Source is the trivy-operator SecuritySource implementation.
type Source struct {
	client dynamic.Interface
}

// New returns a Source with no client (Fetch returns nil, IsAvailable false).
func New() *Source { return &Source{} }

// NewWithDynamic returns a Source using the given dynamic client.
func NewWithDynamic(client dynamic.Interface) *Source {
	return &Source{client: client}
}

func (s *Source) Name() string { return "trivy-operator" }

func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryVuln, security.CategoryMisconfig}
}

// IsAvailable checks that the VulnerabilityReport CRD is served by the API.
// A successful List call means the CRD is registered — even if empty.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	if s.client == nil {
		return false, nil
	}
	_, err := s.client.Resource(VulnerabilityReportGVR).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return false, fmt.Errorf("trivy-operator availability probe: %w", err)
	}
	return true, nil
}

// Fetch is added in Task C4. Return nil until then.
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	return nil, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/security/trivyop/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/trivyop/
git commit -m "feat(security/trivyop): add source scaffold and availability probe"
```

---

### Task C2: Parse `VulnerabilityReport` CRDs into findings

**Files:**
- Create: `internal/security/trivyop/parse.go`
- Modify: `internal/security/trivyop/trivyop_test.go`

- [ ] **Step 1: Write failing parser test**

Append to `trivyop_test.go`:

```go
func TestParseVulnerabilityReport(t *testing.T) {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aquasecurity.github.io/v1alpha1",
			"kind":       "VulnerabilityReport",
			"metadata": map[string]interface{}{
				"namespace": "prod",
				"name":      "deployment-api-container-app",
				"labels": map[string]interface{}{
					"trivy-operator.resource.kind":      "Deployment",
					"trivy-operator.resource.name":      "api",
					"trivy-operator.container.name":     "app",
				},
			},
			"report": map[string]interface{}{
				"vulnerabilities": []interface{}{
					map[string]interface{}{
						"vulnerabilityID":  "CVE-2024-0001",
						"severity":         "CRITICAL",
						"resource":         "openssl",
						"installedVersion": "3.0.7",
						"fixedVersion":     "3.0.13",
						"title":            "Remote code execution in openssl",
						"description":      "A flaw was found...",
						"primaryLink":      "https://nvd.nist.gov/vuln/detail/CVE-2024-0001",
					},
					map[string]interface{}{
						"vulnerabilityID":  "CVE-2024-0002",
						"severity":         "HIGH",
						"resource":         "glibc",
						"installedVersion": "2.36",
					},
				},
			},
		},
	}

	findings := parseVulnerabilityReport(u)
	require.Len(t, findings, 2)

	crit := findings[0]
	assert.Equal(t, security.CategoryVuln, crit.Category)
	assert.Equal(t, security.SeverityCritical, crit.Severity)
	assert.Equal(t, "trivy-operator", crit.Source)
	assert.Equal(t, "Deployment", crit.Resource.Kind)
	assert.Equal(t, "api", crit.Resource.Name)
	assert.Equal(t, "app", crit.Resource.Container)
	assert.Contains(t, crit.ID, "CVE-2024-0001")
	assert.Contains(t, crit.References, "https://nvd.nist.gov/vuln/detail/CVE-2024-0001")
	assert.Equal(t, "openssl", crit.Labels["package"])
	assert.Equal(t, "3.0.13", crit.Labels["fixed_version"])

	assert.Equal(t, security.SeverityHigh, findings[1].Severity)
}

func TestParseVulnerabilityReportEmpty(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"report": map[string]interface{}{"vulnerabilities": []interface{}{}},
	}}
	findings := parseVulnerabilityReport(u)
	assert.Empty(t, findings)
}

func TestParseVulnerabilityReportMalformed(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{"weird": 123}}
	findings := parseVulnerabilityReport(u)
	assert.Empty(t, findings, "malformed report must not panic and must return empty")
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/security/trivyop/... -run TestParseVuln`
Expected: FAIL — `undefined: parseVulnerabilityReport`.

- [ ] **Step 3: Add parser**

```go
// internal/security/trivyop/parse.go
package trivyop

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/security"
)

// parseSeverity converts a Trivy severity string to our scale.
func parseSeverity(s string) security.Severity {
	switch strings.ToUpper(s) {
	case "CRITICAL":
		return security.SeverityCritical
	case "HIGH":
		return security.SeverityHigh
	case "MEDIUM":
		return security.SeverityMedium
	case "LOW":
		return security.SeverityLow
	}
	return security.SeverityUnknown
}

// extractResourceRef reads the Trivy Operator labels that encode the owning workload.
func extractResourceRef(u *unstructured.Unstructured) security.ResourceRef {
	labels := u.GetLabels()
	return security.ResourceRef{
		Namespace: u.GetNamespace(),
		Kind:      labels["trivy-operator.resource.kind"],
		Name:      labels["trivy-operator.resource.name"],
		Container: labels["trivy-operator.container.name"],
	}
}

// parseVulnerabilityReport extracts findings from a single VulnerabilityReport.
// Malformed or missing fields are skipped silently (never panic).
func parseVulnerabilityReport(u *unstructured.Unstructured) []security.Finding {
	ref := extractResourceRef(u)
	vulns, ok, _ := unstructured.NestedSlice(u.Object, "report", "vulnerabilities")
	if !ok || len(vulns) == 0 {
		return nil
	}
	findings := make([]security.Finding, 0, len(vulns))
	for _, v := range vulns {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		cve, _ := m["vulnerabilityID"].(string)
		sev, _ := m["severity"].(string)
		pkg, _ := m["resource"].(string)
		installed, _ := m["installedVersion"].(string)
		fixed, _ := m["fixedVersion"].(string)
		title, _ := m["title"].(string)
		desc, _ := m["description"].(string)
		link, _ := m["primaryLink"].(string)

		if cve == "" {
			continue
		}
		summary := fmt.Sprintf("%s %s (installed: %s)", cve, pkg, installed)
		if title == "" {
			title = cve
		}
		details := desc
		if fixed != "" {
			details = fmt.Sprintf("Fixed in %s\n\n%s", fixed, details)
		}

		f := security.Finding{
			ID:       fmt.Sprintf("trivy-operator/%s/%s/%s/%s", ref.Namespace, ref.Kind, ref.Name, cve),
			Source:   "trivy-operator",
			Category: security.CategoryVuln,
			Severity: parseSeverity(sev),
			Title:    title,
			Resource: ref,
			Summary:  summary,
			Details:  details,
			Labels: map[string]string{
				"cve":           cve,
				"package":       pkg,
				"installed":     installed,
				"fixed_version": fixed,
			},
		}
		if link != "" {
			f.References = []string{link}
		}
		findings = append(findings, f)
	}
	return findings
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/security/trivyop/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/trivyop/
git commit -m "feat(security/trivyop): parse VulnerabilityReport CRDs"
```

---

### Task C3: Parse `ConfigAuditReport` CRDs

**Files:**
- Modify: `internal/security/trivyop/parse.go`
- Modify: `internal/security/trivyop/trivyop_test.go`

- [ ] **Step 1: Write failing test**

Append to `trivyop_test.go`:

```go
func TestParseConfigAuditReport(t *testing.T) {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aquasecurity.github.io/v1alpha1",
			"kind":       "ConfigAuditReport",
			"metadata": map[string]interface{}{
				"namespace": "prod",
				"name":      "daemonset-agent",
				"labels": map[string]interface{}{
					"trivy-operator.resource.kind": "DaemonSet",
					"trivy-operator.resource.name": "agent",
				},
			},
			"report": map[string]interface{}{
				"checks": []interface{}{
					map[string]interface{}{
						"checkID":     "KSV001",
						"severity":    "HIGH",
						"title":       "Process can elevate its own privileges",
						"description": "A program inside the container can elevate its own privileges...",
						"success":     false,
					},
					map[string]interface{}{
						"checkID":  "KSV002",
						"severity": "LOW",
						"title":    "Default AppArmor profile not set",
						"success":  true, // passing check, must be skipped
					},
				},
			},
		},
	}

	findings := parseConfigAuditReport(u)
	require.Len(t, findings, 1, "only failing checks should produce findings")
	f := findings[0]
	assert.Equal(t, security.CategoryMisconfig, f.Category)
	assert.Equal(t, security.SeverityHigh, f.Severity)
	assert.Equal(t, "DaemonSet", f.Resource.Kind)
	assert.Equal(t, "KSV001", f.Labels["check_id"])
	assert.Contains(t, f.ID, "KSV001")
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/security/trivyop/... -run TestParseConfigAudit`
Expected: FAIL — undefined.

- [ ] **Step 3: Add parser**

Append to `parse.go`:

```go
// parseConfigAuditReport extracts failing misconfig checks from a ConfigAuditReport.
func parseConfigAuditReport(u *unstructured.Unstructured) []security.Finding {
	ref := extractResourceRef(u)
	checks, ok, _ := unstructured.NestedSlice(u.Object, "report", "checks")
	if !ok || len(checks) == 0 {
		return nil
	}
	var findings []security.Finding
	for _, c := range checks {
		m, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		passed, _ := m["success"].(bool)
		if passed {
			continue
		}
		checkID, _ := m["checkID"].(string)
		sev, _ := m["severity"].(string)
		title, _ := m["title"].(string)
		desc, _ := m["description"].(string)
		if checkID == "" {
			continue
		}
		findings = append(findings, security.Finding{
			ID:       fmt.Sprintf("trivy-operator/%s/%s/%s/%s", ref.Namespace, ref.Kind, ref.Name, checkID),
			Source:   "trivy-operator",
			Category: security.CategoryMisconfig,
			Severity: parseSeverity(sev),
			Title:    title,
			Resource: ref,
			Summary:  title,
			Details:  desc,
			Labels:   map[string]string{"check_id": checkID},
		})
	}
	return findings
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/security/trivyop/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/security/trivyop/
git commit -m "feat(security/trivyop): parse ConfigAuditReport CRDs"
```

---

### Task C4: `Fetch()` integration — list and parse both CRDs

**Files:**
- Modify: `internal/security/trivyop/trivyop.go`
- Modify: `internal/security/trivyop/trivyop_test.go`

- [ ] **Step 1: Write failing integration test**

Append to `trivyop_test.go`:

```go
func TestFetchAggregatesBothCRDs(t *testing.T) {
	vuln := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aquasecurity.github.io/v1alpha1",
			"kind":       "VulnerabilityReport",
			"metadata": map[string]interface{}{
				"namespace": "prod", "name": "v1",
				"labels": map[string]interface{}{
					"trivy-operator.resource.kind": "Deployment",
					"trivy-operator.resource.name": "api",
					"trivy-operator.container.name": "app",
				},
			},
			"report": map[string]interface{}{
				"vulnerabilities": []interface{}{
					map[string]interface{}{
						"vulnerabilityID": "CVE-1", "severity": "CRITICAL", "resource": "openssl",
					},
				},
			},
		},
	}
	audit := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aquasecurity.github.io/v1alpha1",
			"kind":       "ConfigAuditReport",
			"metadata": map[string]interface{}{
				"namespace": "prod", "name": "a1",
				"labels": map[string]interface{}{
					"trivy-operator.resource.kind": "Deployment",
					"trivy-operator.resource.name": "api",
				},
			},
			"report": map[string]interface{}{
				"checks": []interface{}{
					map[string]interface{}{
						"checkID": "KSV001", "severity": "HIGH", "title": "Priv esc", "success": false,
					},
				},
			},
		},
	}

	client := newFakeDyn(vuln, audit)
	s := NewWithDynamic(client)
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	assert.Len(t, findings, 2)

	var gotVuln, gotAudit bool
	for _, f := range findings {
		switch f.Category {
		case security.CategoryVuln:
			gotVuln = true
		case security.CategoryMisconfig:
			gotAudit = true
		}
	}
	assert.True(t, gotVuln)
	assert.True(t, gotAudit)
}

func TestFetchNamespaceFilter(t *testing.T) {
	make := func(ns, name string) *unstructured.Unstructured {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "aquasecurity.github.io/v1alpha1",
				"kind":       "VulnerabilityReport",
				"metadata": map[string]interface{}{
					"namespace": ns, "name": name,
					"labels": map[string]interface{}{
						"trivy-operator.resource.kind": "Deployment",
						"trivy-operator.resource.name": name,
					},
				},
				"report": map[string]interface{}{
					"vulnerabilities": []interface{}{
						map[string]interface{}{
							"vulnerabilityID": "CVE-X", "severity": "HIGH",
						},
					},
				},
			},
		}
	}

	client := newFakeDyn(make("prod", "a"), make("staging", "b"))
	s := NewWithDynamic(client)
	findings, err := s.Fetch(context.Background(), "", "prod")
	require.NoError(t, err)
	assert.Len(t, findings, 1)
	assert.Equal(t, "prod", findings[0].Resource.Namespace)
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/security/trivyop/... -run TestFetch`
Expected: FAIL — current `Fetch` returns nil.

- [ ] **Step 3: Implement Fetch**

Replace the `Fetch` function in `trivyop.go`:

```go
// Fetch lists VulnerabilityReport and ConfigAuditReport CRDs and returns
// them as findings. Malformed items are skipped (logged once per batch
// elsewhere — see Task G2 for logging integration).
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.client == nil {
		return nil, nil
	}

	var findings []security.Finding

	vulnList, err := s.client.Resource(VulnerabilityReportGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list vulnerability reports: %w", err)
	}
	for i := range vulnList.Items {
		findings = append(findings, parseVulnerabilityReport(&vulnList.Items[i])...)
	}

	auditList, err := s.client.Resource(ConfigAuditReportGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return findings, fmt.Errorf("list config audit reports: %w", err)
	}
	for i := range auditList.Items {
		findings = append(findings, parseConfigAuditReport(&auditList.Items[i])...)
	}

	return findings, nil
}
```

- [ ] **Step 4: Run all trivyop tests**

Run: `go test ./internal/security/trivyop/... -race -cover`
Expected: `PASS`, coverage > 85%.

- [ ] **Step 5: Commit**

```bash
git add internal/security/trivyop/
git commit -m "feat(security/trivyop): Fetch lists and parses both CRDs"
```

---

## Phase D — Config & keybindings

### Task D1: Add `Security` and `SecurityResource` keybindings

**Files:**
- Modify: `internal/ui/config.go`
- Modify: `internal/ui/config_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/ui/config_test.go`:

```go
func TestDefaultKeybindingsIncludeSecurity(t *testing.T) {
	kb := DefaultKeybindings()
	assert.Equal(t, "#", kb.Security)
	assert.Equal(t, "H", kb.SecurityResource)
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/ui/... -run TestDefaultKeybindingsIncludeSecurity`
Expected: FAIL — `kb.Security undefined`.

- [ ] **Step 3: Extend the Keybindings struct and defaults**

In `internal/ui/config.go` around line 58, add fields to the `Keybindings` struct:

```go
	Monitoring       string `json:"monitoring" yaml:"monitoring"`
	Security         string `json:"security" yaml:"security"`
	SecurityResource string `json:"security_resource" yaml:"security_resource"`
```

Around line 123 in `DefaultKeybindings()`, add:

```go
		SaveResource: "W", Monitoring: "@",
		Security: "#", SecurityResource: "H",
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/ui/... -run TestDefaultKeybindingsIncludeSecurity`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/config.go internal/ui/config_test.go
git commit -m "feat(ui): add Security and SecurityResource keybindings"
```

---

### Task D2: Add `security:` per-cluster config block

**Files:**
- Modify: `internal/ui/config.go`
- Modify: `internal/ui/config_test.go`
- Modify: `internal/model/types.go`

- [ ] **Step 1: Write failing test**

Append to `internal/ui/config_test.go`:

```go
func TestParseSecurityConfig(t *testing.T) {
	yaml := `
security:
  default:
    enabled: true
    per_resource_indicators: true
    per_resource_action: true
    refresh_ttl: 30s
    availability_ttl: 60s
    sources:
      heuristic:
        enabled: true
        checks:
          - privileged
          - host_namespaces
      trivy_operator:
        enabled: true
`
	cfg, err := parseConfigYAML([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Security)
	def := cfg.Security["default"]
	assert.True(t, def.Enabled)
	assert.True(t, def.PerResourceIndicators)
	assert.Equal(t, "30s", def.RefreshTTL)
	assert.True(t, def.Sources["heuristic"].Enabled)
	assert.Equal(t, []string{"privileged", "host_namespaces"}, def.Sources["heuristic"].Checks)
	assert.True(t, def.Sources["trivy_operator"].Enabled)
}

func TestDefaultSecurityConfig(t *testing.T) {
	def := DefaultSecurityConfig()
	assert.True(t, def.Enabled)
	assert.True(t, def.PerResourceIndicators)
	assert.Equal(t, "30s", def.RefreshTTL)
	assert.True(t, def.Sources["heuristic"].Enabled)
	assert.True(t, def.Sources["trivy_operator"].Enabled)
}
```

Note: `parseConfigYAML` is assumed to exist — if the config package uses a differently-named parser, adjust accordingly. Check `internal/ui/config.go` for the real function name before running. If no helper exists, add one as part of this task.

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/ui/... -run "TestParseSecurityConfig|TestDefaultSecurityConfig"`
Expected: FAIL.

- [ ] **Step 3: Add types and default**

Add to `internal/model/types.go`:

```go
// SecurityConfig is per-cluster security dashboard configuration.
type SecurityConfig struct {
	Enabled               bool                          `json:"enabled" yaml:"enabled"`
	PerResourceIndicators bool                          `json:"per_resource_indicators" yaml:"per_resource_indicators"`
	PerResourceAction     bool                          `json:"per_resource_action" yaml:"per_resource_action"`
	RefreshTTL            string                        `json:"refresh_ttl" yaml:"refresh_ttl"`
	AvailabilityTTL       string                        `json:"availability_ttl" yaml:"availability_ttl"`
	Sources               map[string]SecuritySourceCfg  `json:"sources" yaml:"sources"`
}

// SecuritySourceCfg is the per-source config shared across adapters.
type SecuritySourceCfg struct {
	Enabled bool     `json:"enabled" yaml:"enabled"`
	Checks  []string `json:"checks,omitempty" yaml:"checks,omitempty"`
}
```

In `internal/ui/config.go`, add the `Security` map to the top-level config (around line 290 where `Monitoring` lives):

```go
	Security map[string]model.SecurityConfig `json:"security" yaml:"security"`
```

Add a default:

```go
// DefaultSecurityConfig returns the default security configuration applied when
// no override is present.
func DefaultSecurityConfig() model.SecurityConfig {
	return model.SecurityConfig{
		Enabled:               true,
		PerResourceIndicators: true,
		PerResourceAction:     true,
		RefreshTTL:            "30s",
		AvailabilityTTL:       "60s",
		Sources: map[string]model.SecuritySourceCfg{
			"heuristic": {Enabled: true, Checks: []string{
				"privileged", "host_namespaces", "host_path", "readonly_root_fs",
				"run_as_root", "allow_priv_esc", "dangerous_caps",
				"missing_resource_limits", "default_sa", "latest_tag",
			}},
			"trivy_operator": {Enabled: true},
		},
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ui/... ./internal/model/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/config.go internal/ui/config_test.go internal/model/types.go
git commit -m "feat(ui): add per-cluster security config block with defaults"
```

---

## Phase E — Model registration

### Task E1: Register `__security__` pseudo-resource in the middle column

**Files:**
- Modify: `internal/model/resource_lookup.go`
- Modify: `internal/model/types_test.go` (or add new test file)

- [ ] **Step 1: Write failing test**

Append to `internal/model/types_test.go`:

```go
func TestFlattenedResourceTypesIncludesSecurity(t *testing.T) {
	items := FlattenedResourceTypes()
	var found bool
	var securityIdx, monitoringIdx int
	for i, it := range items {
		if it.Extra == "__security__" {
			found = true
			securityIdx = i
			assert.Equal(t, "Security", it.Name)
			assert.Equal(t, "Dashboards", it.Category)
		}
		if it.Extra == "__monitoring__" {
			monitoringIdx = i
		}
	}
	assert.True(t, found, "security pseudo-item must be present")
	assert.Equal(t, monitoringIdx+1, securityIdx,
		"security must appear immediately after monitoring")
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/model/... -run TestFlattenedResourceTypesIncludesSecurity`
Expected: FAIL.

- [ ] **Step 3: Register the pseudo-item**

In `internal/model/resource_lookup.go` around line 30-36, insert the security item after the monitoring block:

```go
	items = append(items, Item{
		Name:     "Monitoring",
		Kind:     "__monitoring__",
		Extra:    "__monitoring__",
		Category: "Dashboards",
		Icon:     "⊙",
	})
	items = append(items, Item{
		Name:     "Security",
		Kind:     "__security__",
		Extra:    "__security__",
		Category: "Dashboards",
		Icon:     "◈",
	})
```

Also update anywhere else that special-cases `__monitoring__` and would need to treat `__security__` the same way. Search the codebase:

Run: `grep -rn "__monitoring__" internal/`

For each file that checks `item.Extra == "__monitoring__"` (commandbar_complete.go, update_explain.go, view_status.go, update_keys.go, view_right.go), inspect whether the same gating should apply to `__security__`. For this task, only update **commandbar_complete.go** and **update_explain.go** which treat monitoring as a skip entry — the security item behaves the same in these contexts.

In `internal/app/commandbar_complete.go:47`, change:

```go
if item.Extra == "" || item.Extra == "__overview__" || item.Extra == "__monitoring__" {
```

to:

```go
if item.Extra == "" || item.Extra == "__overview__" || item.Extra == "__monitoring__" || item.Extra == "__security__" {
```

Apply the same change at lines 76 and 527.

In `internal/app/update_explain.go:27-28`, extend the conditional:

```go
sel.Kind == "__monitoring__" || sel.Extra == "__overview__" ||
    sel.Extra == "__monitoring__" || sel.Kind == "__security__" || sel.Extra == "__security__" {
```

- [ ] **Step 4: Run all affected package tests**

Run: `go test ./internal/model/... ./internal/app/... -race`
Expected: `PASS` (some tests may need minor updates — see next step).

- [ ] **Step 5: Fix any existing tests that count items**

If tests like `view_extra_test.go`, `view_right_coverage_test.go`, or `view_coverage_test.go` hard-code the number of middle-column items, bump those counts by 1.

Run: `go test ./internal/app/... 2>&1 | grep -E "FAIL|--- FAIL" | head -20`

For each failure, open the file and adjust fixtures.

- [ ] **Step 6: Commit**

```bash
git add internal/model/resource_lookup.go internal/model/types_test.go internal/app/commandbar_complete.go internal/app/update_explain.go
git commit -m "feat(model): register __security__ pseudo-resource in Dashboards group"
```

---

## Phase F — UI rendering

### Task F1: `SecurityViewState` type and state helpers

**Files:**
- Create: `internal/ui/securityview.go`
- Create: `internal/ui/securityview_test.go`

- [ ] **Step 1: Write failing state-helper tests**

```go
// internal/ui/securityview_test.go
package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/security"
)

func TestSecurityViewStateSeverityCounts(t *testing.T) {
	state := SecurityViewState{
		Findings: []security.Finding{
			{Severity: security.SeverityCritical, Category: security.CategoryVuln},
			{Severity: security.SeverityCritical, Category: security.CategoryVuln},
			{Severity: security.SeverityHigh, Category: security.CategoryVuln},
			{Severity: security.SeverityMedium, Category: security.CategoryMisconfig},
			{Severity: security.SeverityLow, Category: security.CategoryMisconfig},
		},
	}
	counts := state.severityCounts()
	assert.Equal(t, 2, counts[security.SeverityCritical])
	assert.Equal(t, 1, counts[security.SeverityHigh])
	assert.Equal(t, 1, counts[security.SeverityMedium])
	assert.Equal(t, 1, counts[security.SeverityLow])
}

func TestSecurityViewStateFilterByCategory(t *testing.T) {
	state := SecurityViewState{
		ActiveCategory: security.CategoryVuln,
		Findings: []security.Finding{
			{ID: "1", Category: security.CategoryVuln},
			{ID: "2", Category: security.CategoryMisconfig},
			{ID: "3", Category: security.CategoryVuln},
		},
	}
	visible := state.visibleFindings()
	assert.Len(t, visible, 2)
	assert.Equal(t, "1", visible[0].ID)
	assert.Equal(t, "3", visible[1].ID)
}

func TestSecurityViewStateEmpty(t *testing.T) {
	state := SecurityViewState{}
	assert.Empty(t, state.visibleFindings())
	assert.Equal(t, 0, state.severityCounts()[security.SeverityCritical])
}

func TestSecurityViewStateUpdatedAgo(t *testing.T) {
	state := SecurityViewState{LastFetch: time.Now().Add(-5 * time.Second)}
	s := state.updatedAgo()
	assert.Contains(t, s, "5s")
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/ui/... -run TestSecurityViewState`
Expected: FAIL — `undefined: SecurityViewState`.

- [ ] **Step 3: Create the state type and helpers**

```go
// internal/ui/securityview.go
// Package ui security view: pure-function rendering for the Security dashboard.
// The caller (internal/app) owns the state; this file only produces strings.
package ui

import (
	"fmt"
	"time"

	"github.com/janosmiko/lfk/internal/security"
)

// SecurityViewState is the complete state needed to render the dashboard.
// It is populated by internal/app from security.Manager results and key events.
type SecurityViewState struct {
	// Data.
	Findings            []security.Finding   // all findings (from the last successful fetch)
	AvailableCategories []security.Category  // controls which tabs are shown
	ActiveCategory      security.Category

	// Interaction.
	Cursor         int
	Scroll         int
	ShowDetail     bool
	Filter         string
	FilterFocus    bool
	ResourceFilter *security.ResourceRef

	// Status.
	Loading   bool
	LastError error
	LastFetch time.Time
}

// severityCounts returns severity bucket counts across all findings (not just
// the active category).
func (s SecurityViewState) severityCounts() map[security.Severity]int {
	m := map[security.Severity]int{}
	for _, f := range s.Findings {
		m[f.Severity]++
	}
	return m
}

// visibleFindings returns findings filtered to the active category and,
// if set, the resource filter. Text filter is also applied if non-empty.
func (s SecurityViewState) visibleFindings() []security.Finding {
	if len(s.Findings) == 0 {
		return nil
	}
	out := make([]security.Finding, 0, len(s.Findings))
	for _, f := range s.Findings {
		if s.ActiveCategory != "" && f.Category != s.ActiveCategory {
			continue
		}
		if s.ResourceFilter != nil && f.Resource.Key() != s.ResourceFilter.Key() {
			continue
		}
		if s.Filter != "" && !matchFilter(f, s.Filter) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// matchFilter performs a case-insensitive substring match against the finding's
// most important fields.
func matchFilter(f security.Finding, filter string) bool {
	needle := lowerAscii(filter)
	haystacks := []string{f.Title, f.Summary, f.Source, f.Resource.Name, f.Resource.Namespace}
	for _, h := range haystacks {
		if containsLower(h, needle) {
			return true
		}
	}
	return false
}

// lowerAscii lowercases ASCII only — avoids importing unicode.
func lowerAscii(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func containsLower(h, needleLower string) bool {
	if needleLower == "" {
		return true
	}
	hl := lowerAscii(h)
	for i := 0; i+len(needleLower) <= len(hl); i++ {
		if hl[i:i+len(needleLower)] == needleLower {
			return true
		}
	}
	return false
}

// updatedAgo returns a human-readable "12s ago" style string.
func (s SecurityViewState) updatedAgo() string {
	if s.LastFetch.IsZero() {
		return "never"
	}
	d := time.Since(s.LastFetch).Round(time.Second)
	return fmt.Sprintf("%s ago", d)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ui/... -run TestSecurityViewState -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/securityview.go internal/ui/securityview_test.go
git commit -m "feat(ui): add SecurityViewState and filtering helpers"
```

---

### Task F2: Render severity tiles

**Files:**
- Modify: `internal/ui/securityview.go`
- Modify: `internal/ui/securityview_test.go`

- [ ] **Step 1: Write failing test**

Append to `securityview_test.go`:

```go
func TestRenderSeverityTiles(t *testing.T) {
	state := SecurityViewState{
		Findings: []security.Finding{
			{Severity: security.SeverityCritical},
			{Severity: security.SeverityCritical},
			{Severity: security.SeverityHigh},
			{Severity: security.SeverityLow},
		},
	}
	out := renderSeverityTiles(state, 80)
	assert.Contains(t, out, "CRIT")
	assert.Contains(t, out, "2")
	assert.Contains(t, out, "HIGH")
	assert.Contains(t, out, "1")
	assert.Contains(t, out, "LOW")
	assert.Contains(t, out, "MED")
	assert.Contains(t, out, "0")
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/ui/... -run TestRenderSeverityTiles`
Expected: FAIL.

- [ ] **Step 3: Add tile renderer**

Append to `securityview.go`:

```go
import "strings"

// renderSeverityTiles returns a single-line tile row with severity counts.
// The tiles use the existing StatusFailed/StatusWarning/StatusProgressing/StatusRunning
// theme styles so colors respect the user's theme. When width < 40 the function
// falls back to a compact single-line list.
func renderSeverityTiles(state SecurityViewState, width int) string {
	counts := state.severityCounts()
	critical := counts[security.SeverityCritical]
	high := counts[security.SeverityHigh]
	medium := counts[security.SeverityMedium]
	low := counts[security.SeverityLow]

	if width < 40 {
		return fmt.Sprintf("CRIT %d  HIGH %d  MED %d  LOW %d", critical, high, medium, low)
	}

	tile := func(label string, n int, style lipglossStyleLike) string {
		return style.Render(fmt.Sprintf(" %s %3d ", label, n))
	}
	parts := []string{
		tile("CRIT", critical, StatusFailed),
		tile("HIGH", high, StatusWarning),
		tile("MED ", medium, StatusProgressing),
		tile("LOW ", low, StatusRunning),
	}
	return strings.Join(parts, "  ")
}
```

Note: `lipglossStyleLike` is a placeholder for whatever style type lfk already uses. Replace it with the actual type used by `StatusFailed` et al — inspect `internal/ui/styles.go` or `theme.go` before implementing. If the styles are `lipgloss.Style`, import `github.com/charmbracelet/lipgloss` and use `lipgloss.Style` directly.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ui/... -run TestRenderSeverityTiles -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/securityview.go internal/ui/securityview_test.go
git commit -m "feat(ui): add severity tile renderer for security dashboard"
```

---

### Task F3: Render category tab strip

**Files:**
- Modify: `internal/ui/securityview.go`
- Modify: `internal/ui/securityview_test.go`

- [ ] **Step 1: Write failing tests**

Append to `securityview_test.go`:

```go
func TestRenderTabStripShowsAllAvailable(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln, security.CategoryMisconfig},
		ActiveCategory:      security.CategoryVuln,
		Findings: []security.Finding{
			{Category: security.CategoryVuln},
			{Category: security.CategoryMisconfig},
			{Category: security.CategoryMisconfig},
		},
	}
	out := renderTabStrip(state)
	assert.Contains(t, out, "Vulns 1")
	assert.Contains(t, out, "Misconf 2")
	assert.NotContains(t, out, "Policies", "policies tab must be hidden when not available")
}

func TestRenderTabStripEmptyWhenSingleCategory(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryMisconfig},
		ActiveCategory:      security.CategoryMisconfig,
	}
	out := renderTabStrip(state)
	assert.Empty(t, out, "single-category dashboards skip the tab strip")
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/ui/... -run TestRenderTabStrip`
Expected: FAIL.

- [ ] **Step 3: Add renderer**

Append to `securityview.go`:

```go
// categoryLabel returns the short display label for a category.
func categoryLabel(c security.Category) string {
	switch c {
	case security.CategoryVuln:
		return "Vulns"
	case security.CategoryMisconfig:
		return "Misconf"
	case security.CategoryPolicy:
		return "Policies"
	case security.CategoryCompliance:
		return "CIS"
	}
	return string(c)
}

// renderTabStrip returns the category tab row, or empty if there's one or zero
// available categories.
func renderTabStrip(state SecurityViewState) string {
	if len(state.AvailableCategories) <= 1 {
		return ""
	}
	perCat := map[security.Category]int{}
	for _, f := range state.Findings {
		perCat[f.Category]++
	}
	var parts []string
	for _, c := range state.AvailableCategories {
		label := fmt.Sprintf("%s %d", categoryLabel(c), perCat[c])
		if c == state.ActiveCategory {
			parts = append(parts, "["+label+"]")
		} else {
			parts = append(parts, " "+label+" ")
		}
	}
	return strings.Join(parts, "  ")
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ui/... -run TestRenderTabStrip -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/securityview.go internal/ui/securityview_test.go
git commit -m "feat(ui): add category tab strip with per-category counts"
```

---

### Task F4: Render findings table

**Files:**
- Modify: `internal/ui/securityview.go`
- Modify: `internal/ui/securityview_test.go`

- [ ] **Step 1: Write failing tests**

Append to `securityview_test.go`:

```go
func TestRenderFindingsTable(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln},
		ActiveCategory:      security.CategoryVuln,
		Cursor:              1,
		Findings: []security.Finding{
			{ID: "1", Severity: security.SeverityCritical, Source: "trivy-op",
				Title: "CVE-2024-1234", Category: security.CategoryVuln,
				Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}},
			{ID: "2", Severity: security.SeverityHigh, Source: "trivy-op",
				Title: "CVE-2024-5678", Category: security.CategoryVuln,
				Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "web"}},
		},
	}
	out := renderFindingsTable(state, 80, 10)
	assert.Contains(t, out, "deploy/api")
	assert.Contains(t, out, "deploy/web")
	assert.Contains(t, out, "CVE-2024-1234")
	assert.Contains(t, out, "> ", "cursor should highlight row 1")
}

func TestRenderFindingsTableEmpty(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln},
		ActiveCategory:      security.CategoryVuln,
	}
	out := renderFindingsTable(state, 80, 10)
	assert.Contains(t, out, "No security findings")
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/ui/... -run TestRenderFindingsTable`
Expected: FAIL.

- [ ] **Step 3: Add renderer**

Append to `securityview.go`:

```go
// kindShort returns the abbreviated kind name used in the table.
func kindShort(kind string) string {
	switch kind {
	case "Deployment":
		return "deploy"
	case "StatefulSet":
		return "sts"
	case "DaemonSet":
		return "ds"
	case "ReplicaSet":
		return "rs"
	case "Job":
		return "job"
	case "CronJob":
		return "cron"
	case "Pod":
		return "pod"
	}
	return kind
}

// renderFindingsTable produces the body table. maxRows is the viewport height
// available for findings rows (excluding header).
func renderFindingsTable(state SecurityViewState, width, maxRows int) string {
	visible := state.visibleFindings()
	if len(visible) == 0 {
		return DimStyle.Render("  No security findings")
	}

	var b strings.Builder
	b.WriteString("  SEV   RESOURCE                 SOURCE        TITLE\n")

	start := state.Scroll
	end := start + maxRows
	if end > len(visible) {
		end = len(visible)
	}

	for i := start; i < end; i++ {
		f := visible[i]
		marker := "  "
		if i == state.Cursor {
			marker = "> "
		}
		resource := fmt.Sprintf("%s/%s  (%s)", kindShort(f.Resource.Kind), f.Resource.Name, f.Resource.Namespace)
		row := fmt.Sprintf("%s%-5s %-24s %-12s %s",
			marker, severityShort(f.Severity), truncate(resource, 24), truncate(f.Source, 12), truncate(f.Title, 40))
		b.WriteString(row)
		b.WriteString("\n")
	}
	return b.String()
}

func severityShort(s security.Severity) string {
	switch s {
	case security.SeverityCritical:
		return "CRIT"
	case security.SeverityHigh:
		return "HIGH"
	case security.SeverityMedium:
		return "MED"
	case security.SeverityLow:
		return "LOW"
	}
	return "?"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
```

Note: `DimStyle` already exists in the `ui` package; if not, replace with a no-op `func(s string) string { return s }`-style call.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ui/... -run TestRenderFindingsTable -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/securityview.go internal/ui/securityview_test.go
git commit -m "feat(ui): add findings table with cursor highlight"
```

---

### Task F5: Render details pane and empty/loading/error states

**Files:**
- Modify: `internal/ui/securityview.go`
- Modify: `internal/ui/securityview_test.go`

- [ ] **Step 1: Write failing tests**

Append to `securityview_test.go`:

```go
func TestRenderDetailsPane(t *testing.T) {
	state := SecurityViewState{
		ActiveCategory: security.CategoryVuln,
		ShowDetail:     true,
		Findings: []security.Finding{
			{ID: "1", Title: "CVE-2024-1234", Severity: security.SeverityCritical,
				Summary: "openssl 3.0.7", Details: "Fixed in 3.0.13\n\nA flaw was found...",
				References: []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-1234"},
				Category:   security.CategoryVuln,
				Resource:   security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		},
	}
	out := renderDetailsPane(state, 80)
	assert.Contains(t, out, "CVE-2024-1234")
	assert.Contains(t, out, "Fixed in 3.0.13")
	assert.Contains(t, out, "https://nvd.nist.gov/vuln/detail/CVE-2024-1234")
}

func TestRenderDetailsPaneHidden(t *testing.T) {
	state := SecurityViewState{ShowDetail: false}
	assert.Empty(t, renderDetailsPane(state, 80))
}

func TestRenderLoadingState(t *testing.T) {
	state := SecurityViewState{Loading: true}
	out := renderSecurityStatusOverlay(state)
	assert.Contains(t, out, "Loading security findings")
}

func TestRenderErrorState(t *testing.T) {
	state := SecurityViewState{LastError: fmt.Errorf("probe failed")}
	out := renderSecurityStatusOverlay(state)
	assert.Contains(t, out, "probe failed")
	assert.Contains(t, out, "Press r to retry")
}

func TestRenderNoSourcesAvailable(t *testing.T) {
	state := SecurityViewState{AvailableCategories: nil, Loading: false, LastError: nil}
	out := renderSecurityStatusOverlay(state)
	assert.Contains(t, out, "No security sources available")
}
```

Add `"fmt"` to test imports if not already present.

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/ui/... -run "TestRenderDetails|TestRenderLoading|TestRenderError|TestRenderNoSources"`
Expected: FAIL.

- [ ] **Step 3: Add renderers**

Append to `securityview.go`:

```go
// renderDetailsPane returns the detail panel for the currently selected finding,
// or an empty string if ShowDetail is false.
func renderDetailsPane(state SecurityViewState, width int) string {
	if !state.ShowDetail {
		return ""
	}
	visible := state.visibleFindings()
	if len(visible) == 0 || state.Cursor >= len(visible) {
		return ""
	}
	f := visible[state.Cursor]

	var b strings.Builder
	b.WriteString("─ Details ")
	b.WriteString(strings.Repeat("─", max(0, width-11)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf(" %s    %s\n", f.Title, severityShort(f.Severity)))
	b.WriteString(fmt.Sprintf(" Source:   %s\n", f.Source))
	if f.Resource.Name != "" {
		b.WriteString(fmt.Sprintf(" Resource: %s/%s/%s\n",
			f.Resource.Namespace, kindShort(f.Resource.Kind), f.Resource.Name))
	}
	if f.Summary != "" {
		b.WriteString(" Summary:  ")
		b.WriteString(f.Summary)
		b.WriteString("\n")
	}
	if f.Details != "" {
		b.WriteString("\n")
		b.WriteString(f.Details)
		b.WriteString("\n")
	}
	if len(f.References) > 0 {
		b.WriteString("\n References:\n")
		for _, r := range f.References {
			b.WriteString("  - ")
			b.WriteString(r)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// renderSecurityStatusOverlay returns a status message for loading, error, and
// no-sources states. Returns empty string when the dashboard should render normally.
func renderSecurityStatusOverlay(state SecurityViewState) string {
	if state.Loading {
		return DimStyle.Render("  Loading security findings...")
	}
	if state.LastError != nil {
		return fmt.Sprintf("  Error: %s\n  Press r to retry", state.LastError.Error())
	}
	if len(state.AvailableCategories) == 0 {
		return DimStyle.Render("  No security sources available.\n  Install Trivy Operator to enable vulnerability scanning.")
	}
	return ""
}
```

Add `max` helper if Go version lacks it — 1.26 has the builtin.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ui/... -run "TestRenderDetails|TestRenderLoading|TestRenderError|TestRenderNoSources" -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/securityview.go internal/ui/securityview_test.go
git commit -m "feat(ui): add details pane and status overlay renderers"
```

---

### Task F6: Main `RenderSecurityDashboard` composition

**Files:**
- Modify: `internal/ui/securityview.go`
- Modify: `internal/ui/securityview_test.go`

- [ ] **Step 1: Write failing tests**

Append to `securityview_test.go`:

```go
func TestRenderSecurityDashboardComposition(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln, security.CategoryMisconfig},
		ActiveCategory:      security.CategoryVuln,
		LastFetch:           time.Now().Add(-12 * time.Second),
		Findings: []security.Finding{
			{ID: "1", Category: security.CategoryVuln, Severity: security.SeverityCritical,
				Title: "CVE-2024-1", Source: "trivy-op",
				Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		},
	}
	out := RenderSecurityDashboard(state, 80, 20)
	assert.Contains(t, out, "Security Dashboard")
	assert.Contains(t, out, "CRIT")
	assert.Contains(t, out, "Vulns 1")
	assert.Contains(t, out, "CVE-2024-1")
	assert.Contains(t, out, "ago") // updatedAgo rendering
}

func TestRenderSecurityDashboardLoading(t *testing.T) {
	state := SecurityViewState{Loading: true}
	out := RenderSecurityDashboard(state, 80, 20)
	assert.Contains(t, out, "Loading security findings")
}

func TestRenderSecurityDashboardWithFilter(t *testing.T) {
	ref := &security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln},
		ActiveCategory:      security.CategoryVuln,
		ResourceFilter:      ref,
	}
	out := RenderSecurityDashboard(state, 80, 20)
	assert.Contains(t, out, "Security: prod/Deployment/api")
	assert.Contains(t, out, "clear filter")
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/ui/... -run TestRenderSecurityDashboard`
Expected: FAIL — `undefined: RenderSecurityDashboard`.

- [ ] **Step 3: Add the main renderer**

Append to `securityview.go`:

```go
// RenderSecurityDashboard composes the header, tiles, tab strip, findings
// table, and details pane into a single string sized to (width, height).
// This is the only exported render function for the security view.
func RenderSecurityDashboard(state SecurityViewState, width, height int) string {
	if overlay := renderSecurityStatusOverlay(state); overlay != "" && len(state.Findings) == 0 {
		return overlay
	}

	var b strings.Builder

	// Header.
	header := "Security Dashboard"
	if state.ResourceFilter != nil {
		header = "Security: " + state.ResourceFilter.Key() + "    [C] clear filter"
	}
	b.WriteString(header)
	b.WriteString("    ")
	b.WriteString(DimStyle.Render("updated "))
	b.WriteString(DimStyle.Render(state.updatedAgo()))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", width))
	b.WriteString("\n")

	// Tiles.
	b.WriteString(renderSeverityTiles(state, width))
	b.WriteString("\n\n")

	// Tab strip.
	if tabs := renderTabStrip(state); tabs != "" {
		b.WriteString(tabs)
		b.WriteString("\n")
	}
	b.WriteString(strings.Repeat("─", width))
	b.WriteString("\n")

	// Table — reserve rows for header/details/hint.
	reserved := 8 // tile row + header + separators
	if state.ShowDetail {
		reserved += 8
	}
	maxRows := max(1, height-reserved)
	b.WriteString(renderFindingsTable(state, width, maxRows))

	// Details pane.
	if state.ShowDetail {
		b.WriteString("\n")
		b.WriteString(renderDetailsPane(state, width))
	}

	// Hint bar.
	b.WriteString("\n")
	hint := "[/]filter [Tab]tab [Enter]det [r]refresh [J/K]scroll"
	if state.ResourceFilter != nil {
		hint = "[/]filter [Tab]tab [Enter]det [r]refresh [C]clear [J/K]scroll"
	}
	b.WriteString(DimStyle.Render(hint))

	return b.String()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ui/... -run TestRenderSecurity -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/securityview.go internal/ui/securityview_test.go
git commit -m "feat(ui): compose security dashboard render pipeline"
```

---

### Task F7: Width adaptation tests and fix

**Files:**
- Modify: `internal/ui/securityview.go`
- Modify: `internal/ui/securityview_test.go`

- [ ] **Step 1: Write width-adaptation tests**

Append to `securityview_test.go`:

```go
func TestRenderSecurityDashboardCompactWidth(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln},
		ActiveCategory:      security.CategoryVuln,
		Findings: []security.Finding{
			{Severity: security.SeverityCritical, Title: "X",
				Resource: security.ResourceRef{Namespace: "p", Kind: "Deployment", Name: "api"}},
		},
	}
	for _, w := range []int{30, 50, 70, 120} {
		out := RenderSecurityDashboard(state, w, 20)
		assert.NotEmpty(t, out, "width %d should still render", w)
		assert.Contains(t, out, "CRIT", "width %d should include severity", w)
	}
}
```

- [ ] **Step 2: Run — expect pass or fail depending on existing behavior**

Run: `go test ./internal/ui/... -run TestRenderSecurityDashboardCompactWidth`

If passing: no code change needed. If failing at any width: fix `renderFindingsTable` to clamp columns when `width < 80` (drop source column) and `width < 60` (drop namespace).

- [ ] **Step 3: Add width-aware column shrinking (if tests fail)**

Update `renderFindingsTable` header and row builders:

```go
func renderFindingsTable(state SecurityViewState, width, maxRows int) string {
	visible := state.visibleFindings()
	if len(visible) == 0 {
		return DimStyle.Render("  No security findings")
	}

	showSource := width >= 80
	showNamespace := width >= 60

	var b strings.Builder
	if showSource {
		b.WriteString("  SEV   RESOURCE                 SOURCE        TITLE\n")
	} else {
		b.WriteString("  SEV   RESOURCE                 TITLE\n")
	}

	start := state.Scroll
	end := start + maxRows
	if end > len(visible) {
		end = len(visible)
	}

	for i := start; i < end; i++ {
		f := visible[i]
		marker := "  "
		if i == state.Cursor {
			marker = "> "
		}
		resource := fmt.Sprintf("%s/%s", kindShort(f.Resource.Kind), f.Resource.Name)
		if showNamespace {
			resource += fmt.Sprintf("  (%s)", f.Resource.Namespace)
		}
		var row string
		if showSource {
			row = fmt.Sprintf("%s%-5s %-24s %-12s %s",
				marker, severityShort(f.Severity), truncate(resource, 24),
				truncate(f.Source, 12), truncate(f.Title, 40))
		} else {
			row = fmt.Sprintf("%s%-5s %-24s %s",
				marker, severityShort(f.Severity), truncate(resource, 24),
				truncate(f.Title, max(10, width-32)))
		}
		b.WriteString(row)
		b.WriteString("\n")
	}
	return b.String()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/ui/... -race -cover`
Expected: `PASS`, coverage on `securityview.go` > 85%.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/securityview.go internal/ui/securityview_test.go
git commit -m "feat(ui): adapt security table columns to narrow widths"
```

---

## Phase G — App wiring

### Task G1: Security messages and state fields on `Model`

**Files:**
- Create: `internal/app/messages_security.go`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Add message types**

```go
// internal/app/messages_security.go
package app

import (
	"github.com/janosmiko/lfk/internal/security"
)

// securityFindingsLoadedMsg is sent when Manager.FetchAll completes successfully
// (including partial failures — individual source errors live in result.Errors).
type securityFindingsLoadedMsg struct {
	context   string
	namespace string
	result    security.FetchResult
}

// securityFetchErrorMsg is sent when Manager.FetchAll returns an error
// (should not happen with the current implementation, which never returns
// fatal errors, but defined for future-proofing).
type securityFetchErrorMsg struct {
	context string
	err     error
}

// securityAvailabilityLoadedMsg is sent after Manager.AnyAvailable completes.
// It is used to decide whether the SEC column and per-resource H key are active.
type securityAvailabilityLoadedMsg struct {
	context   string
	available bool
}
```

- [ ] **Step 2: Add fields to the Model**

In `internal/app/app.go`, find the `Model` struct and add:

```go
	// Security dashboard state.
	securityManager   *security.Manager
	securityView      ui.SecurityViewState
	securityAvailable bool // cached result of Manager.AnyAvailable for the current context
```

Add imports for `"github.com/janosmiko/lfk/internal/security"` if not already present.

- [ ] **Step 3: Run `go build`**

Run: `go build ./...`
Expected: Success.

- [ ] **Step 4: Commit**

```bash
git add internal/app/messages_security.go internal/app/app.go
git commit -m "feat(app): add security message types and Model state"
```

---

### Task G2: `loadSecurityDashboard` tea.Cmd

**Files:**
- Create: `internal/app/commands_security.go`
- Create: `internal/app/commands_security_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/app/commands_security_test.go
package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/security"
)

func TestLoadSecurityDashboardDispatchesFetch(t *testing.T) {
	mgr := security.NewManager()
	fake := &security.FakeSource{
		NameStr: "fake", Available: true,
		CategoriesVal: []security.Category{security.CategoryVuln},
		Findings: []security.Finding{{ID: "1", Category: security.CategoryVuln,
			Severity: security.SeverityHigh, Title: "x"}},
	}
	mgr.Register(fake)

	m := Model{
		securityManager: mgr,
	}
	// nav.Context is read by loadSecurityDashboard.
	m.nav.Context = "kctx"

	cmd := m.loadSecurityDashboard()
	require.NotNil(t, cmd)
	msg := cmd()

	loaded, ok := msg.(securityFindingsLoadedMsg)
	require.True(t, ok, "got %T", msg)
	assert.Equal(t, "kctx", loaded.context)
	assert.Len(t, loaded.result.Findings, 1)
	_ = context.Background
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/app/... -run TestLoadSecurityDashboard`
Expected: FAIL — `undefined: loadSecurityDashboard`.

- [ ] **Step 3: Add the command**

```go
// internal/app/commands_security.go
package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logger"
)

// loadSecurityDashboard fetches findings from the security Manager and returns
// a securityFindingsLoadedMsg with the result. Per-source errors are logged
// via internal/logger and included in result.Errors.
func (m Model) loadSecurityDashboard() tea.Cmd {
	if m.securityManager == nil {
		return nil
	}
	mgr := m.securityManager
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := mgr.FetchAll(ctx, kctx, ns)
		if err != nil {
			return securityFetchErrorMsg{context: kctx, err: err}
		}
		for name, e := range res.Errors {
			logger.Logger.Error("security source fetch failed",
				"source", name, "context", kctx, "error", e.Error())
		}
		return securityFindingsLoadedMsg{context: kctx, namespace: ns, result: res}
	}
}

// loadSecurityAvailability probes Manager.AnyAvailable and returns a message so
// the Model can gate UI features (SEC column, H key).
func (m Model) loadSecurityAvailability() tea.Cmd {
	if m.securityManager == nil {
		return nil
	}
	mgr := m.securityManager
	kctx := m.nav.Context
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ok, _ := mgr.AnyAvailable(ctx, kctx)
		return securityAvailabilityLoadedMsg{context: kctx, available: ok}
	}
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/app/... -run TestLoadSecurityDashboard -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/app/commands_security.go internal/app/commands_security_test.go
git commit -m "feat(app): add loadSecurityDashboard tea.Cmd"
```

---

### Task G3: Message handler and key routing for the security preview

**Files:**
- Create: `internal/app/update_security.go`
- Create: `internal/app/update_security_test.go`
- Modify: `internal/app/update.go` (dispatch new messages to handlers)

- [ ] **Step 1: Write failing handler tests**

```go
// internal/app/update_security_test.go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/security"
	"github.com/janosmiko/lfk/internal/ui"
)

func baseSecurityModel() Model {
	return Model{
		securityView: ui.SecurityViewState{
			AvailableCategories: []security.Category{security.CategoryVuln, security.CategoryMisconfig},
			ActiveCategory:      security.CategoryVuln,
			Findings: []security.Finding{
				{ID: "1", Category: security.CategoryVuln, Severity: security.SeverityCritical, Title: "CVE-1"},
				{ID: "2", Category: security.CategoryVuln, Severity: security.SeverityHigh, Title: "CVE-2"},
			},
		},
	}
}

func TestSecurityKeyTabCyclesCategory(t *testing.T) {
	m := baseSecurityModel()
	updated, _ := m.handleSecurityKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, security.CategoryMisconfig, updated.securityView.ActiveCategory)
}

func TestSecurityKeyJMovesCursor(t *testing.T) {
	m := baseSecurityModel()
	updated, _ := m.handleSecurityKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, updated.securityView.Cursor)
}

func TestSecurityKeyEnterTogglesDetail(t *testing.T) {
	m := baseSecurityModel()
	updated, _ := m.handleSecurityKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, updated.securityView.ShowDetail)
	updated2, _ := updated.handleSecurityKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, updated2.securityView.ShowDetail)
}

func TestSecurityKeyRRefreshes(t *testing.T) {
	m := baseSecurityModel()
	m.securityManager = security.NewManager()
	m.securityManager.Register(&security.FakeSource{NameStr: "fake", Available: true})
	_, cmd := m.handleSecurityKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	assert.NotNil(t, cmd, "refresh should dispatch a fetch command")
}

func TestSecurityKeyCClearsResourceFilter(t *testing.T) {
	m := baseSecurityModel()
	ref := security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}
	m.securityView.ResourceFilter = &ref
	updated, _ := m.handleSecurityKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	assert.Nil(t, updated.securityView.ResourceFilter)
}

func TestSecurityFindingsLoadedMsgUpdatesState(t *testing.T) {
	m := Model{}
	msg := securityFindingsLoadedMsg{
		context: "kctx",
		result: security.FetchResult{
			Findings: []security.Finding{{ID: "1", Category: security.CategoryVuln, Severity: security.SeverityLow}},
			Sources: []security.SourceStatus{
				{Name: "fake", Available: true, Count: 1},
			},
		},
	}
	updated, _ := m.handleSecurityFindingsLoaded(msg)
	assert.Len(t, updated.securityView.Findings, 1)
	assert.False(t, updated.securityView.Loading)
	assert.Contains(t, updated.securityView.AvailableCategories, security.CategoryVuln)
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/app/... -run TestSecurity`
Expected: FAIL — handlers undefined.

- [ ] **Step 3: Create the handlers**

```go
// internal/app/update_security.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/security"
)

// handleSecurityKey handles key events when the security preview is focused
// (i.e., selectedMiddleItem().Extra == "__security__"). Returns the updated
// Model and an optional command.
func (m Model) handleSecurityKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		m.securityView = cycleCategory(m.securityView, +1)
		return m, nil
	case tea.KeyShiftTab:
		m.securityView = cycleCategory(m.securityView, -1)
		return m, nil
	case tea.KeyEnter:
		m.securityView.ShowDetail = !m.securityView.ShowDetail
		return m, nil
	}
	if len(msg.Runes) == 0 {
		return m, nil
	}
	switch msg.Runes[0] {
	case 'j':
		m = cursorDown(m)
	case 'k':
		m = cursorUp(m)
	case 'g':
		m.securityView.Cursor = 0
		m.securityView.Scroll = 0
	case 'G':
		visible := m.securityView.VisibleFindings()
		if len(visible) > 0 {
			m.securityView.Cursor = len(visible) - 1
		}
	case 'r':
		m.securityView.Loading = true
		return m, m.loadSecurityDashboard()
	case 'C':
		m.securityView.ResourceFilter = nil
	case '1':
		m = activateCategoryByIndex(m, 0)
	case '2':
		m = activateCategoryByIndex(m, 1)
	case '3':
		m = activateCategoryByIndex(m, 2)
	case '4':
		m = activateCategoryByIndex(m, 3)
	}
	return m, nil
}

// VisibleFindings wraps the unexported helper for use outside the ui package.
// Added as a method on ui.SecurityViewState in Task F1 should be public — if it
// isn't, add an exported wrapper there before running this task.

func cycleCategory(state ui.SecurityViewState, delta int) ui.SecurityViewState {
	if len(state.AvailableCategories) == 0 {
		return state
	}
	idx := 0
	for i, c := range state.AvailableCategories {
		if c == state.ActiveCategory {
			idx = i
			break
		}
	}
	n := len(state.AvailableCategories)
	idx = (idx + delta + n) % n
	state.ActiveCategory = state.AvailableCategories[idx]
	state.Cursor = 0
	state.Scroll = 0
	return state
}

func activateCategoryByIndex(m Model, idx int) Model {
	if idx < 0 || idx >= len(m.securityView.AvailableCategories) {
		return m
	}
	m.securityView.ActiveCategory = m.securityView.AvailableCategories[idx]
	m.securityView.Cursor = 0
	m.securityView.Scroll = 0
	return m
}

func cursorDown(m Model) Model {
	visible := m.securityView.VisibleFindings()
	if m.securityView.Cursor < len(visible)-1 {
		m.securityView.Cursor++
	}
	return m
}

func cursorUp(m Model) Model {
	if m.securityView.Cursor > 0 {
		m.securityView.Cursor--
	}
	return m
}

// handleSecurityFindingsLoaded processes a securityFindingsLoadedMsg.
func (m Model) handleSecurityFindingsLoaded(msg securityFindingsLoadedMsg) (Model, tea.Cmd) {
	// Discard stale results from a previous context.
	if msg.context != m.nav.Context && m.nav.Context != "" {
		return m, nil
	}
	m.securityView.Findings = msg.result.Findings
	m.securityView.Loading = false
	m.securityView.LastError = nil
	m.securityView.LastFetch = nowFunc()

	// Recompute available categories from the source status list.
	catSet := map[security.Category]bool{}
	for _, s := range msg.result.Sources {
		if !s.Available {
			continue
		}
		for _, src := range m.securityManager.Sources() {
			if src.Name() != s.Name {
				continue
			}
			for _, c := range src.Categories() {
				catSet[c] = true
			}
		}
	}
	m.securityView.AvailableCategories = []security.Category{}
	for _, c := range []security.Category{
		security.CategoryVuln, security.CategoryMisconfig,
		security.CategoryPolicy, security.CategoryCompliance,
	} {
		if catSet[c] {
			m.securityView.AvailableCategories = append(m.securityView.AvailableCategories, c)
		}
	}
	if m.securityView.ActiveCategory == "" && len(m.securityView.AvailableCategories) > 0 {
		m.securityView.ActiveCategory = m.securityView.AvailableCategories[0]
	}

	return m, nil
}

// handleSecurityAvailabilityLoaded updates the cached availability flag.
func (m Model) handleSecurityAvailabilityLoaded(msg securityAvailabilityLoadedMsg) (Model, tea.Cmd) {
	if msg.context != m.nav.Context && m.nav.Context != "" {
		return m, nil
	}
	m.securityAvailable = msg.available
	return m, nil
}
```

Add the `ui` import at the top. Add `nowFunc = time.Now` as a package-level variable at the top of the file (allows tests to override). Add `"time"` import.

In the `ui` package (`securityview.go`), add an exported wrapper:

```go
// VisibleFindings returns findings for the active category/filter for callers outside this package.
func (s SecurityViewState) VisibleFindings() []security.Finding {
	return s.visibleFindings()
}
```

In `internal/app/update.go`, route the new messages. Find the main switch over `tea.Msg` (search for `case resourcesLoadedMsg`) and add:

```go
	case securityFindingsLoadedMsg:
		return m.handleSecurityFindingsLoaded(msg)
	case securityFetchErrorMsg:
		m.securityView.Loading = false
		m.securityView.LastError = msg.err
		return m, nil
	case securityAvailabilityLoadedMsg:
		return m.handleSecurityAvailabilityLoaded(msg)
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/... -run TestSecurity -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/app/update_security.go internal/app/update_security_test.go internal/app/update.go internal/ui/securityview.go
git commit -m "feat(app): add security key handler and message dispatch"
```

---

### Task G4: `#` key handler — jump to security item

**Files:**
- Modify: `internal/app/update_keys_actions.go`
- Modify: `internal/app/update_keys_test.go` (or create a new test file)

- [ ] **Step 1: Write failing test**

Append to a test file in `internal/app`:

```go
func TestHandleExplorerActionKeySecurityJumpsToItem(t *testing.T) {
	m := Model{}
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Cluster", Extra: "__overview__"},
		{Name: "Monitoring", Extra: "__monitoring__"},
		{Name: "Security", Extra: "__security__"},
		{Name: "Workloads"},
	}
	updated, _, handled := m.handleExplorerActionKeySecurity()
	assert.True(t, handled)
	assert.Equal(t, 2, updated.cursor)
}

func TestHandleExplorerActionKeySecurityRequiresContext(t *testing.T) {
	m := Model{}
	m.nav.Level = model.LevelClusters
	_, _, handled := m.handleExplorerActionKeySecurity()
	assert.True(t, handled) // consumed key but shows status message
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/app/... -run TestHandleExplorerActionKeySecurity`
Expected: FAIL.

- [ ] **Step 3: Add the handler**

In `internal/app/update_keys_actions.go`, next to `handleExplorerActionKeyMonitoring` (around line 597), add:

```go
func (m Model) handleExplorerActionKeySecurity() (tea.Model, tea.Cmd, bool) {
	if m.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage("Select a cluster first", true)
		return m, scheduleStatusClear(), true
	}
	for i, item := range m.middleItems {
		if item.Extra == "__security__" {
			m.setCursor(i)
			m.clampCursor()
			return m, m.loadPreview(), true
		}
	}
	return m, nil, true
}
```

Then wire it into the switch that dispatches keybindings. Around line 70 where `kb.Monitoring` is handled, add:

```go
	case kb.Security:
		return m.handleExplorerActionKeySecurity()
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/... -run TestHandleExplorerActionKeySecurity -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/app/update_keys_actions.go internal/app/update_keys_test.go
git commit -m "feat(app): add # key handler that jumps to security item"
```

---

### Task G5: Preview dispatch and right-pane rendering

**Files:**
- Modify: `internal/app/commands_load_preview.go`
- Modify: `internal/app/view_right.go`
- Modify: `internal/app/update_navigation.go`
- Modify: `internal/app/main_test.go` (or relevant test)

- [ ] **Step 1: Write failing test**

Add to `internal/app/main_test.go` (or similar):

```go
func TestRenderRightResourceTypesSecurityBranch(t *testing.T) {
	m := Model{
		nav: navState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{
			{Name: "Security", Extra: "__security__"},
		},
		cursor: 0,
	}
	m.securityView.Loading = true
	out := m.renderRightResourceTypes(100, 20)
	assert.Contains(t, out, "Loading security findings")
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/app/... -run TestRenderRightResourceTypesSecurity`
Expected: FAIL.

- [ ] **Step 3: Add the branches**

In `internal/app/commands_load_preview.go` around line 47, add after the `__monitoring__` branch:

```go
	if sel.Extra == "__security__" {
		return m.loadSecurityDashboard()
	}
```

In `internal/app/view_right.go`, extend `renderRightResourceTypes`:

```go
func (m Model) renderRightResourceTypes(width, height int) string {
	sel := m.selectedMiddleItem()
	if sel != nil && sel.Extra == "__overview__" {
		if m.dashboardPreview == "" {
			return ui.DimStyle.Render(m.spinner.View() + " Loading cluster dashboard...")
		}
		return m.dashboardPreview
	}
	if sel != nil && sel.Extra == "__monitoring__" {
		if m.monitoringPreview == "" {
			return ui.DimStyle.Render(m.spinner.View() + " Loading monitoring dashboard...")
		}
		return m.monitoringPreview
	}
	if sel != nil && sel.Extra == "__security__" {
		return ui.RenderSecurityDashboard(m.securityView, width, height)
	}
	return m.renderRightDefault(width, height)
}
```

In `internal/app/update_navigation.go` around line 243 where `m.monitoringPreview = ""`, also reset:

```go
	m.monitoringPreview = ""
	m.securityView = ui.SecurityViewState{}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/... -race`
Expected: `PASS` (excluding Phase H tests that don't exist yet).

- [ ] **Step 5: Commit**

```bash
git add internal/app/commands_load_preview.go internal/app/view_right.go internal/app/update_navigation.go internal/app/main_test.go
git commit -m "feat(app): wire security pseudo-resource to preview pane"
```

---

## Phase H — Per-resource integration

### Task H1: Initialize `securityManager` at startup + reset availability on context change

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/app/update_navigation.go`

- [ ] **Step 1: Find the model constructor**

Run: `grep -n "func NewModel\|func initialModel" internal/app/app.go`

- [ ] **Step 2: Write failing test**

In the appropriate test file, add:

```go
func TestModelHasSecurityManagerAfterInit(t *testing.T) {
	m := initialModelForTest() // use existing helper
	assert.NotNil(t, m.securityManager)
	assert.NotEmpty(t, m.securityManager.Sources(), "heuristic source should be registered by default")
}
```

- [ ] **Step 3: Run — expect fail**

Run: `go test ./internal/app/... -run TestModelHasSecurityManager`
Expected: FAIL.

- [ ] **Step 4: Register sources during model init**

In the `Model` constructor (wherever it lives — likely `NewModel` or `initialModel`), add:

```go
import (
	"github.com/janosmiko/lfk/internal/security"
	"github.com/janosmiko/lfk/internal/security/heuristic"
	"github.com/janosmiko/lfk/internal/security/trivyop"
)

// Build the security manager with the default sources. Sources with unmet
// dependencies (nil client, missing CRDs) will report IsAvailable() == false
// and be skipped at fetch time — no initialization error.
mgr := security.NewManager()
if clientset := m.client.RawClientset(); clientset != nil {
	mgr.Register(heuristic.NewWithClient(clientset))
}
if dyn := m.client.RawDynamic(); dyn != nil {
	mgr.Register(trivyop.NewWithDynamic(dyn))
}
m.securityManager = mgr
```

Note: `RawClientset()` and `RawDynamic()` accessors may not exist on `*k8s.Client`. If they don't, add them as simple getters in `internal/k8s/client.go`. Alternatively, inspect the existing pattern for how other features get access to the raw clientset.

In `update_navigation.go`, when the context changes (around the same place `monitoringPreview = ""` is reset), also clear availability:

```go
m.securityAvailable = false
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/app/... -race`
Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/app/app.go internal/app/update_navigation.go internal/k8s/client.go
git commit -m "feat(app): initialize security manager with heuristic and trivyop sources"
```

---

### Task H2: `SEC` column in the explorer table (auto-gated)

**Files:**
- Modify: `internal/ui/explorer_format.go` (or `explorer_table.go`, depending on where columns are defined)
- Modify: `internal/app/view_right.go` (if it builds column lists)

- [ ] **Step 1: Explore existing column machinery**

Run: `grep -rn "ColumnToggle\|columnDefs\|columnVisible" internal/ui/ | head -20`

Identify where columns are defined and how per-resource columns are added.

- [ ] **Step 2: Write failing test**

In the appropriate test file:

```go
func TestSecColumnHiddenWhenNoSources(t *testing.T) {
	idx := &security.FindingIndex{} // empty
	cols := buildExplorerColumns(idx, false /* securityAvailable */)
	for _, c := range cols {
		assert.NotEqual(t, "SEC", c.Name)
	}
}

func TestSecColumnVisibleWhenSourcesAvailable(t *testing.T) {
	idx := security.BuildFindingIndex([]security.Finding{
		{Severity: security.SeverityCritical,
			Resource: security.ResourceRef{Namespace: "p", Kind: "Deployment", Name: "api"}},
	})
	cols := buildExplorerColumns(idx, true)
	var found bool
	for _, c := range cols {
		if c.Name == "SEC" {
			found = true
		}
	}
	assert.True(t, found)
}
```

- [ ] **Step 3: Run — expect fail**

Run: `go test ./internal/ui/... -run TestSecColumn`
Expected: FAIL — `buildExplorerColumns` may not exist, or may need an extra parameter.

- [ ] **Step 4: Add the column**

Open the explorer column builder. Add a new column with:

```go
// SEC column: severity indicator and count, auto-gated on securityAvailable.
if securityAvailable {
	cols = append(cols, ExplorerColumn{
		Name:  "SEC",
		Width: 6,
		Render: func(item model.Item) string {
			ref := security.ResourceRef{
				Namespace: item.Namespace,
				Kind:      item.Kind,
				Name:      item.Name,
			}
			counts := findingIndex.For(ref)
			if counts.Total() == 0 {
				return ""
			}
			symbol := "○"
			style := StatusRunning
			switch counts.Highest() {
			case security.SeverityCritical:
				symbol, style = "●", StatusFailed
			case security.SeverityHigh:
				symbol, style = "◐", StatusWarning
			case security.SeverityMedium, security.SeverityLow:
				symbol, style = "○", StatusProgressing
			}
			return style.Render(fmt.Sprintf("%s%d", symbol, counts.Total()))
		},
	})
}
```

This is a sketch — the exact struct shape depends on the existing column abstraction. Adapt accordingly.

Thread `findingIndex` and `securityAvailable` through from `Model` to the render call site (`renderRightResources`, `renderRightOwned`, etc.).

- [ ] **Step 5: Run tests**

Run: `go test ./internal/ui/... ./internal/app/... -race`
Expected: `PASS`

- [ ] **Step 6: Commit**

```bash
git add internal/ui/explorer_format.go internal/ui/explorer_table.go internal/app/view_right.go
git commit -m "feat(ui): add auto-gated SEC column to explorer"
```

---

### Task H3: "Security Findings" action menu entry

**Files:**
- Modify: `internal/model/actions.go`
- Modify: `internal/app/update_actions.go`
- Modify: `internal/app/update_actions_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/app/update_actions_test.go`:

```go
func TestSecurityFindingsActionOpensFilteredView(t *testing.T) {
	m := Model{}
	m.securityAvailable = true
	m.nav.ResourceType.Kind = "Deployment"
	m.middleItems = []model.Item{
		{Name: "api", Kind: "Deployment", Namespace: "prod"},
	}
	m.cursor = 0

	updated, _ := m.executeActionSecurityFindings()
	require.NotNil(t, updated.securityView.ResourceFilter)
	assert.Equal(t, "api", updated.securityView.ResourceFilter.Name)
	assert.Equal(t, "Deployment", updated.securityView.ResourceFilter.Kind)
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/app/... -run TestSecurityFindingsAction`
Expected: FAIL.

- [ ] **Step 3: Add action and handler**

In `internal/model/actions.go`, add to the action list:

```go
{Label: "Security Findings", Description: "Show security findings for this resource", Key: "H"},
```

In `internal/app/update_actions.go`, add a handler:

```go
func (m Model) executeActionSecurityFindings() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	ref := security.ResourceRef{
		Namespace: sel.Namespace,
		Kind:      sel.Kind,
		Name:      sel.Name,
	}
	m.securityView.ResourceFilter = &ref
	// Jump to the security item in the middle column if present.
	for i, item := range m.middleItems {
		if item.Extra == "__security__" {
			m.setCursor(i)
			break
		}
	}
	return m, m.loadSecurityDashboard()
}
```

Route the action label in the switch that dispatches action menu items (search for `case "Vuln Scan":` — a similar pattern exists at `update_actions.go:440`):

```go
	case "Security Findings":
		return m.executeActionSecurityFindings()
```

Also gate the action's visibility: find the function that builds the action list for a resource and skip the "Security Findings" entry when `m.securityAvailable == false` or the selected resource is a dashboard pseudo-item.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/... -run TestSecurity -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/model/actions.go internal/app/update_actions.go internal/app/update_actions_test.go
git commit -m "feat(app): add Security Findings action menu entry"
```

---

### Task H4: `H` per-resource hotkey

**Files:**
- Modify: `internal/app/update_keys_actions.go`
- Modify: `internal/app/update_keys_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestSecurityResourceHotkeyOpensFilteredView(t *testing.T) {
	m := Model{}
	m.securityAvailable = true
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{
		{Name: "api", Kind: "Deployment", Namespace: "prod"},
	}
	m.cursor = 0

	updated, _, handled := m.handleExplorerActionKeySecurityResource()
	require.True(t, handled)
	require.NotNil(t, updated.securityView.ResourceFilter)
	assert.Equal(t, "api", updated.securityView.ResourceFilter.Name)
}

func TestSecurityResourceHotkeyNoOpWhenUnavailable(t *testing.T) {
	m := Model{}
	m.securityAvailable = false
	_, _, handled := m.handleExplorerActionKeySecurityResource()
	assert.True(t, handled, "key consumed with status message")
}
```

- [ ] **Step 2: Run — expect fail**

Run: `go test ./internal/app/... -run TestSecurityResourceHotkey`
Expected: FAIL.

- [ ] **Step 3: Add the handler and wire it**

Append to `internal/app/update_keys_actions.go`:

```go
func (m Model) handleExplorerActionKeySecurityResource() (tea.Model, tea.Cmd, bool) {
	if !m.securityAvailable {
		m.setStatusMessage("No security sources available", true)
		return m, scheduleStatusClear(), true
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil, true
	}
	ref := security.ResourceRef{
		Namespace: sel.Namespace,
		Kind:      sel.Kind,
		Name:      sel.Name,
	}
	m.securityView.ResourceFilter = &ref
	for i, item := range m.middleItems {
		if item.Extra == "__security__" {
			m.setCursor(i)
			break
		}
	}
	return m, m.loadSecurityDashboard(), true
}
```

Wire it into the action-key dispatch switch alongside `kb.Security`:

```go
	case kb.SecurityResource:
		return m.handleExplorerActionKeySecurityResource()
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/... -run TestSecurityResourceHotkey -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/app/update_keys_actions.go internal/app/update_keys_test.go
git commit -m "feat(app): add H hotkey for per-resource security view"
```

---

### Task H5: Build `FindingIndex` on Model and refresh after fetch

**Files:**
- Modify: `internal/app/update_security.go`
- Modify: `internal/app/update_security_test.go`

- [ ] **Step 1: Write failing test**

Append to `update_security_test.go`:

```go
func TestFindingIndexRebuiltOnFindingsLoaded(t *testing.T) {
	m := Model{}
	m.securityManager = security.NewManager()
	m.securityManager.Register(&security.FakeSource{
		NameStr: "s", Available: true, CategoriesVal: []security.Category{security.CategoryVuln},
	})
	msg := securityFindingsLoadedMsg{
		context: "kctx",
		result: security.FetchResult{
			Findings: []security.Finding{
				{ID: "1", Severity: security.SeverityCritical,
					Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}},
			},
			Sources: []security.SourceStatus{{Name: "s", Available: true, Count: 1}},
		},
	}
	updated, _ := m.handleSecurityFindingsLoaded(msg)
	idx := updated.securityManager.Index()
	counts := idx.For(security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"})
	assert.Equal(t, 1, counts.Critical)
}
```

- [ ] **Step 2: Run — expect fail or pass**

Run: `go test ./internal/app/... -run TestFindingIndexRebuilt`

If passing: `Manager.FetchAll` already builds the index (from Task A4). Move on.

If failing: `handleSecurityFindingsLoaded` is called with a pre-built `result` that didn't go through the manager's cache logic, so the index is stale. Fix by explicitly rebuilding:

- [ ] **Step 3: Fix (if needed)**

In `handleSecurityFindingsLoaded`, after setting `m.securityView.Findings`:

```go
// Manually rebuild the manager's index from the message's findings, since
// this path is triggered by an async message that bypasses the in-memory cache.
m.securityManager.SetIndex(security.BuildFindingIndex(msg.result.Findings))
```

In `internal/security/manager.go` add:

```go
// SetIndex overrides the cached FindingIndex. Used by callers that produce
// findings outside of FetchAll (e.g., test harnesses and async message paths).
func (m *Manager) SetIndex(idx *FindingIndex) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cachedIndex = idx
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/app/... ./internal/security/... -race`
Expected: `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/app/update_security.go internal/app/update_security_test.go internal/security/manager.go
git commit -m "feat(security): rebuild FindingIndex on async findings-loaded messages"
```

---

## Phase I — Docs & polish

### Task I1: Hint bar and help screen

**Files:**
- Modify: `internal/ui/hintbar.go`
- Modify: `internal/ui/help.go`
- Modify: the corresponding tests

- [ ] **Step 1: Find the hint bar for the explorer**

Run: `grep -n "@" internal/ui/hintbar.go`

Look at how the `@` hint is defined. Add a parallel entry for `#`.

- [ ] **Step 2: Add `#` to the hint bar**

Example (adapt to actual structure):

```go
{Key: "#", Desc: "security"},
```

Right after the monitoring hint entry.

- [ ] **Step 3: Add a Security section to the help screen**

In `internal/ui/help.go`, add a new section to the help content:

```go
// Security dashboard.
help += "\n" + h("Security Dashboard") + "\n"
help += kv("#", "Open security dashboard") + "\n"
help += kv("H", "Security findings for selected resource") + "\n"
help += kv("Tab / Shift+Tab", "Cycle category tabs") + "\n"
help += kv("Enter", "Toggle details pane") + "\n"
help += kv("r", "Refresh security findings") + "\n"
help += kv("C", "Clear resource filter") + "\n"
help += kv("/", "Filter findings") + "\n"
```

Adapt the helper function names (`h`, `kv`) to whatever the existing help code uses.

- [ ] **Step 4: Update any tests that snapshot the hint bar or help output**

Run: `go test ./internal/ui/... -race 2>&1 | grep FAIL`

For each failing test, update the expected snapshot to include the new entries.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/hintbar.go internal/ui/help.go internal/ui/hintbar_test.go internal/ui/help_test.go
git commit -m "docs: add security dashboard entries to hint bar and help screen"
```

---

### Task I2: Startup tips and navigation update

**Files:**
- Modify: `internal/app/commands.go`
- Modify: `internal/app/commands_test.go` (or similar)

- [ ] **Step 1: Find the tips list**

Open `internal/app/commands.go` around line 42 where the monitoring tip lives:

```go
"Press @ to open the monitoring dashboard",
```

- [ ] **Step 2: Add security tips**

Append to the tips slice:

```go
"Press # to open the security dashboard",
"Press H on a resource to see its security findings",
```

- [ ] **Step 3: Run existing test (if any counts tips)**

Run: `go test ./internal/app/... -race 2>&1 | grep FAIL`

Adjust test counts if needed.

- [ ] **Step 4: Commit**

```bash
git add internal/app/commands.go internal/app/commands_test.go
git commit -m "docs(app): add startup tips for security dashboard"
```

---

### Task I3: README, keybindings.md, config reference

**Files:**
- Modify: `README.md`
- Modify: `docs/keybindings.md`
- Modify: `docs/config-reference.md`
- Modify: `docs/config-example.yaml`

- [ ] **Step 1: Update README.md**

In the "Features" section, under "Integrations" or a new "Security" subsection, add:

```markdown
### Security Dashboard

- **Unified security view**: Press `#` to open an aggregate dashboard showing findings from built-in heuristic checks and (when installed) the Trivy Operator (`VulnerabilityReport`, `ConfigAuditReport`).
- **Per-resource indicators**: Optional `SEC` column in the explorer table shows severity-colored badges and counts for each workload (auto-gated on source availability).
- **Per-resource drill-down**: Press `H` on a selected resource to open the dashboard pre-filtered to that resource, or use the "Security Findings" entry in the action menu (`x`).
```

- [ ] **Step 2: Update docs/keybindings.md**

Add to the global keys table:

```markdown
| `#` | Open security dashboard |
| `H` | Security findings for the selected resource |
```

Add a new subsection:

```markdown
### Security Dashboard

| Key | Action |
|---|---|
| `j` / `k` | Move cursor down / up |
| `g` / `G` | Jump to top / bottom of list |
| `Tab` / `Shift+Tab` | Cycle category tabs |
| `1`–`4` | Jump to tab by index |
| `Enter` | Toggle inline details pane |
| `r` | Force refresh |
| `/` | Filter findings (substring) |
| `C` | Clear resource filter (when per-resource view is active) |
| `o` | Jump to the resource referenced by the selected finding |
| `y` | Copy finding ID + primary URL |
```

- [ ] **Step 3: Update docs/config-reference.md**

After the `monitoring` section, add:

```markdown
## Security

Configure the security dashboard (`#` key) and per-resource indicators. All fields are optional with sensible defaults.

```yaml
security:
  default:
    enabled: true
    per_resource_indicators: true
    per_resource_action: true
    refresh_ttl: 30s
    availability_ttl: 60s
    sources:
      heuristic:
        enabled: true
        checks:
          - privileged
          - host_namespaces
          - host_path
          - readonly_root_fs
          - run_as_root
          - allow_priv_esc
          - dangerous_caps
          - missing_resource_limits
          - default_sa
          - latest_tag
      trivy_operator:
        enabled: true
```

### Security Entry Fields

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `true` | Master switch for the dashboard |
| `per_resource_indicators` | bool | `true` | Show the `SEC` column in the explorer when sources are available |
| `per_resource_action` | bool | `true` | Show "Security Findings" in the action menu |
| `refresh_ttl` | duration | `"30s"` | Cache TTL for fetched findings |
| `availability_ttl` | duration | `"60s"` | Cache TTL for source availability checks |
| `sources` | map | see default | Per-source configuration |

### Source Fields

Each source entry accepts:

| Field | Type | Description |
|---|---|---|
| `enabled` | bool | Opt-out toggle (defaults to true for each source) |
| `checks` | []string | (heuristic only) Which checks to run |
```

- [ ] **Step 4: Update docs/config-example.yaml**

Add a commented `security:` block:

```yaml
# Security dashboard configuration. All fields are optional.
# security:
#   default:
#     enabled: true
#     per_resource_indicators: true
#     per_resource_action: true
#     refresh_ttl: 30s
#     sources:
#       heuristic:
#         enabled: true
#       trivy_operator:
#         enabled: true
```

- [ ] **Step 5: Commit**

```bash
git add README.md docs/keybindings.md docs/config-reference.md docs/config-example.yaml
git commit -m "docs: document security dashboard in README and reference guides"
```

---

## Phase J — End-to-end smoke

### Task J1: E2E flow test with fake clients

**Files:**
- Create: `internal/app/security_e2e_test.go`

- [ ] **Step 1: Write the smoke test**

```go
// internal/app/security_e2e_test.go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubefake "k8s.io/client-go/kubernetes/fake"
	dynfake "k8s.io/client-go/dynamic/fake"

	"github.com/janosmiko/lfk/internal/security"
	"github.com/janosmiko/lfk/internal/security/heuristic"
	"github.com/janosmiko/lfk/internal/security/trivyop"
	"github.com/janosmiko/lfk/internal/ui"
)

func boolP(b bool) *bool { return &b }

func TestSecurityFlowEndToEnd(t *testing.T) {
	// Build fake k8s clientset with one privileged pod.
	badPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "bad"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "c", Image: "nginx:latest",
				SecurityContext: &corev1.SecurityContext{Privileged: boolP(true)},
			}},
		},
	}
	kubeClient := kubefake.NewSimpleClientset(badPod)

	// Build fake dynamic client with one VulnerabilityReport.
	vuln := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aquasecurity.github.io/v1alpha1",
			"kind":       "VulnerabilityReport",
			"metadata": map[string]interface{}{
				"namespace": "prod", "name": "vr1",
				"labels": map[string]interface{}{
					"trivy-operator.resource.kind":  "Deployment",
					"trivy-operator.resource.name":  "api",
					"trivy-operator.container.name": "app",
				},
			},
			"report": map[string]interface{}{
				"vulnerabilities": []interface{}{
					map[string]interface{}{
						"vulnerabilityID": "CVE-2024-1234", "severity": "CRITICAL",
						"resource": "openssl",
					},
				},
			},
		},
	}
	scheme := runtime.NewScheme()
	dynClient := dynfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			trivyop.VulnerabilityReportGVR: "VulnerabilityReportList",
			trivyop.ConfigAuditReportGVR:   "ConfigAuditReportList",
		},
		vuln,
	)

	// Build manager wired with real sources against fake clients.
	mgr := security.NewManager()
	mgr.Register(heuristic.NewWithClient(kubeClient))
	mgr.Register(trivyop.NewWithDynamic(dynClient))

	m := Model{securityManager: mgr}
	m.nav.Context = "kctx"

	// Trigger fetch via the same path the app uses.
	cmd := m.loadSecurityDashboard()
	require.NotNil(t, cmd)
	msg := cmd()

	loaded, ok := msg.(securityFindingsLoadedMsg)
	require.True(t, ok)
	updated, _ := m.handleSecurityFindingsLoaded(loaded)

	// Assert both sources contributed.
	var gotHeuristic, gotTrivy bool
	for _, f := range updated.securityView.Findings {
		if f.Source == "heuristic" {
			gotHeuristic = true
		}
		if f.Source == "trivy-operator" {
			gotTrivy = true
		}
	}
	assert.True(t, gotHeuristic, "heuristic findings should be present")
	assert.True(t, gotTrivy, "trivy-operator findings should be present")

	// Render and assert the dashboard contains key elements.
	rendered := ui.RenderSecurityDashboard(updated.securityView, 100, 30)
	assert.Contains(t, rendered, "Security Dashboard")
	assert.Contains(t, rendered, "CVE-2024-1234")
	assert.Contains(t, rendered, "privileged")

	// Press Tab to switch categories.
	tabbed, _ := updated.handleSecurityKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.NotEqual(t, updated.securityView.ActiveCategory, tabbed.securityView.ActiveCategory)
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/app/... -run TestSecurityFlowEndToEnd -race`
Expected: `PASS`

- [ ] **Step 3: Run the full test suite once**

Run: `go test ./... -race -cover`
Expected: All packages pass. Confirm coverage for:
- `internal/security` > 80%
- `internal/security/heuristic` > 90%
- `internal/security/trivyop` > 85%
- `internal/ui` for security code > 85%
- `internal/app` for security code > 80%

- [ ] **Step 4: Run the linter**

Run: `golangci-lint run ./...`
Expected: `0 issues.`

- [ ] **Step 5: Final commit**

```bash
git add internal/app/security_e2e_test.go
git commit -m "test(security): add end-to-end smoke test wiring all sources"
```

---

## Finalization

After Task J1 commits, create the pull request:

- [ ] **Step 1: Run the full test + lint suite once more**

Run: `go test ./... -race -cover && golangci-lint run ./...`

- [ ] **Step 2: Push the branch and open a PR**

```bash
git push -u origin feat/security-dashboard
gh pr create --title "feat: Security metrics dashboard (Phase 1)" --body "$(cat <<'EOF'
## Summary
- Adds a `#` hotkey that opens a Security dashboard as a pseudo-resource in the Dashboards group (mirrors the `@` Monitoring pattern).
- Aggregates findings from two sources: a zero-dependency heuristic source (10 workload hardening checks) and the Trivy Operator CRDs (VulnerabilityReport, ConfigAuditReport).
- Adds a dependency-gated `SEC` column with severity-colored badges, an action-menu "Security Findings" entry, and an `H` per-resource hotkey.
- All UI elements are hidden or disabled when no sources are available.

See `docs/superpowers/specs/2026-04-08-security-metrics-design.md` for the full design.

## Test plan
- [ ] `go test ./... -race -cover` passes
- [ ] `golangci-lint run ./...` is clean
- [ ] Manual smoke test on a real cluster WITH Trivy Operator installed
- [ ] Manual smoke test on a real cluster WITHOUT Trivy Operator (heuristic only)
- [ ] Verify `#` opens the dashboard, tabs cycle, Enter toggles details
- [ ] Verify `H` on a Deployment opens the filtered view
- [ ] Verify the `SEC` column shows badges on pods/deployments with findings
- [ ] Verify the `SEC` column is hidden when all sources are unavailable

EOF
)"
```

---

## Self-review notes

This plan implements the full Phase 1 scope from the design spec. Task-to-spec coverage:

| Spec section | Tasks |
|---|---|
| §1 Architecture overview | A1, A2, A3, A4 |
| §2 SecuritySource interface | A1 |
| §3.1 heuristic source | B1–B8 |
| §3.2 trivyop source | C1–C4 |
| §3.3 policyreport | (Phase 2 — not in this plan) |
| §3.4 kubebench | (Phase 3 — not in this plan) |
| §4 UI (pseudo-resource, render pane) | E1, F1–F7, G5 |
| §5.1 SEC column | H1, H2 |
| §5.2 Per-resource detail (action menu + hotkey) | H3, H4 |
| §5.3 Config | D1, D2 |
| §6 Error handling + integration + refresh | G1–G3, H1, H5 |
| §7 Testing strategy | Tests in every task + J1 |
| §8 Docs | I1, I2, I3 |

**Open risks flagged during the plan:**
- Task H1 assumes `k8s.Client` has accessors for the raw clientset and dynamic client. If they don't exist, add them as simple getters.
- Task D2 assumes a `parseConfigYAML` helper exists in `internal/ui/config.go`. If not, the task needs a small addition to add one.
- Task F2 references `StatusFailed`, `StatusWarning`, etc. — these must match the real style identifiers in `internal/ui/styles.go`.
- Task H2 sketches the `SEC` column based on an assumed column abstraction. The exact integration depends on the existing column machinery.

Each task is committed independently so that failures isolate to the smallest possible scope. All tests run with `-race` and the overall coverage target is 80% for new code.

