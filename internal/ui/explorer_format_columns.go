package ui

import (
	"slices"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/model"
)

// extraColumn represents an additional column discovered from item data.
type extraColumn struct {
	key      string // column key (e.g., "IP", "Node")
	width    int    // display width for this column
	hasArrow bool   // true if any value in this column has a trend arrow
}

// ExtraColumnInfo is an exported representation of an extra column for use by
// the app layer (e.g., header click handling).
type ExtraColumnInfo struct {
	Key   string
	Width int
}

// CollectExtraColumns is an exported wrapper around collectExtraColumns.
// It returns the extra columns as ExtraColumnInfo for use outside the ui package.
func CollectExtraColumns(items []model.Item, totalWidth, usedWidth int, kind string) []ExtraColumnInfo {
	cols := collectExtraColumns(items, totalWidth, usedWidth, kind)
	result := make([]ExtraColumnInfo, len(cols))
	for i, c := range cols {
		result[i] = ExtraColumnInfo{Key: c.key, Width: c.width}
	}
	return result
}

// ActiveSessionColumns holds the session-only column override for the current
// resource type. Set by the app before rendering. Nil means no override.
var ActiveSessionColumns []string

// ActivePrinterColumns maps CRD additionalPrinterColumns names to their
// declared priority for the resource type currently rendered in the middle
// column. Set by the app before rendering; nil for non-CRD resources or CRDs
// without printer columns. Priority 0 columns are treated as mandatory (always
// shown, never dropped by the width budget); priority > 0 columns are hidden by
// default, matching kubectl's standard vs. `-o wide` behaviour.
var ActivePrinterColumns map[string]int

// isMandatoryColumn reports whether key is a CRD printer column that must
// always be shown (priority 0).
func isMandatoryColumn(key string) bool {
	if ActivePrinterColumns == nil {
		return false
	}
	prio, ok := ActivePrinterColumns[key]
	return ok && prio == 0
}

// ActiveHiddenBuiltinColumns holds the set of built-in column keys that should
// be suppressed in the current table render (middle column or right-pane
// preview). Valid keys: "Name", "Context", "Namespace", "Ready", "Restarts",
// "Age", "Status". Set by the app before rendering. Nil means no overrides.
var ActiveHiddenBuiltinColumns map[string]bool

// ActiveNameHidden reports whether the NAME column is hidden for the current
// middle-column render. When true, collectExtraColumns skips the name width
// reservation so extra columns reclaim the space the name would have used.
// Set by RenderTable before rendering.
var ActiveNameHidden bool

// collectExtraColumns discovers which extra columns to show based on item data and config.
// usedWidth is the width already consumed by fixed columns (excluding name).
// kind is the resource Kind (e.g. "Pod") used to resolve per-type column overrides.
// colInfo tracks metadata about a single extra column during collection.
type colInfo struct {
	key      string
	maxValW  int
	count    int
	hasArrow bool // true if any value in this column has a trend arrow
	// minIdx is the smallest column index at which this key appears in any
	// item. Taking the minimum across all rows makes the discovery order a
	// function of the data alone, not of the current row ordering — see
	// sortDiscoveryOrder.
	minIdx int
}

// canonicalColumnPriority pins the display position of the volatile metrics
// columns. Their values are rewritten on every ~2s refresh by several
// independent paths (placeholder / metrics-enriched / carry-over), and those
// paths historically disagreed on whether to prepend or append the block —
// flipping the column order between two layouts each tick (a visible ~1Hz
// blink). Deriving display order purely from item.Columns insertion order
// made every such path load-bearing. Sorting the detected order against this
// list instead makes display order independent of insertion order, so no
// mutation path can reintroduce the flicker.
var canonicalColumnPriority = map[string]int{
	"CPU": 0, "CPU%": 1, "CPU/R": 2, "CPU/L": 3,
	"MEM": 4, "MEM%": 5, "MEM/R": 6, "MEM/L": 7,
	"Uptime": 8,
}

// sortDiscoveryOrder reorders detected column keys in place into a canonical,
// row-order-independent sequence: ascending by each key's minimum position
// within an item (minIdx), ties broken by key name. Because minIdx is a
// minimum over all rows, the result is identical no matter how the rows are
// sorted, which is what keeps the visible columns stable while the user cycles
// the sort key. Runs before stabilizeColumnOrder, which then pins the metrics
// block ahead of everything else.
func sortDiscoveryOrder(order []string, seen map[string]*colInfo) {
	sort.SliceStable(order, func(i, j int) bool {
		ii, jj := seen[order[i]].minIdx, seen[order[j]].minIdx
		if ii != jj {
			return ii < jj
		}
		return order[i] < order[j]
	})
}

// stabilizeColumnOrder reorders detected column keys in place so the canonical
// metrics block always occupies a fixed leading position, while every other
// column keeps its relative discovery order (stable sort).
func stabilizeColumnOrder(order []string) {
	sort.SliceStable(order, func(i, j int) bool {
		ri, iPinned := canonicalColumnPriority[order[i]]
		rj, jPinned := canonicalColumnPriority[order[j]]
		if iPinned && jPinned {
			return ri < rj
		}
		// A pinned key sorts before any unpinned one; two unpinned keys
		// compare equal so the stable sort preserves their discovery order.
		return iPinned && !jPinned
	})
}

// extraColWidth computes the capped display width (including 1 spacing column)
// and the pre-cap natural width for an extra column, given its collected info.
func extraColWidth(info *colInfo, key string, maxColW int) (colW, natural int) {
	colW = len(key)
	maxVal := info.maxValW
	// When some values have arrows, non-arrow values need a placeholder space.
	// The arrow values already include the arrow in their visual width, so
	// ensure non-arrow values get +1 to match.
	if info.hasArrow {
		maxVal++
	}
	if maxVal > colW {
		colW = maxVal
	}
	natural = colW + 1 // pre-cap natural width (with spacing)
	if colW > maxColW {
		colW = maxColW
	}
	colW++ // spacing
	return colW, natural
}

// prioritizeMandatoryColumns returns candidates with mandatory CRD printer
// columns (priority 0) moved to the front, preserving relative order within
// each group. Returns the input unchanged when there are no mandatory columns.
// fitExtraColumns relies on the resulting contiguous mandatory prefix.
func prioritizeMandatoryColumns(candidates []string) []string {
	if !slices.ContainsFunc(candidates, isMandatoryColumn) {
		return candidates
	}
	mandatory := make([]string, 0, len(candidates))
	rest := make([]string, 0, len(candidates))
	for _, k := range candidates {
		if isMandatoryColumn(k) {
			mandatory = append(mandatory, k)
		} else {
			rest = append(rest, k)
		}
	}
	return append(mandatory, rest...)
}

func collectExtraColumns(items []model.Item, totalWidth, usedWidth int, kind string) []extraColumn {
	// Collect all available column keys and their max value widths.
	seen := make(map[string]*colInfo)
	var order []string
	for _, item := range items {
		for idx, kv := range item.Columns {
			info, ok := seen[kv.Key]
			if !ok {
				info = &colInfo{key: kv.Key, minIdx: idx}
				seen[kv.Key] = info
				order = append(order, kv.Key)
			} else if idx < info.minIdx {
				info.minIdx = idx
			}
			info.count++
			if strings.HasPrefix(kv.Value, "↑ ") || strings.HasPrefix(kv.Value, "↓ ") {
				info.hasArrow = true
			}
			valW := lipgloss.Width(kv.Value)
			if valW > info.maxValW {
				info.maxValW = valW
			}
		}
	}

	if len(order) == 0 {
		return nil
	}

	// Reorder by canonical data position so the column set and order depend
	// only on the data, never on how rows happen to be sorted. Without this,
	// cycling the sort key reshuffled which heterogeneous columns (e.g. NODE
	// vs PRIORITY CLASS, when an unscheduled Pod lacks NODE) were discovered
	// first, flickering columns in and out at the width-budget boundary.
	sortDiscoveryOrder(order, seen)

	// Pin the metrics block to a canonical position before candidate
	// selection so display order does not depend on the order the refresh
	// paths happened to write the columns into item.Columns.
	stabilizeColumnOrder(order)

	candidates, fromAutoDetect := selectColumnCandidates(seen, order, kind, items)

	if len(candidates) == 0 {
		return nil
	}

	// Mandatory CRD printer-column protection applies only on the default
	// auto-detect path; an explicit session or configured (views.<kind>.columns)
	// order is authoritative and left untouched. When active, move mandatory
	// columns to the front so the width budget serves them before optional extras.
	mandatoryActive := fromAutoDetect && ActivePrinterColumns != nil
	if mandatoryActive {
		candidates = prioritizeMandatoryColumns(candidates)
	}

	// Reserve budget for the Name column based on the longest item name
	// so resource names with long identifiers (Ingress hostnames, Node
	// FQDNs, helm releases, generated suffixes) don't get squeezed to a
	// 20-char floor while extras (HOSTS, ADDRESS, ROLE, …) eat the rest.
	// See issue #53 and the follow-up node truncation report.
	//
	// Budgeting rule:
	//   1. Default: longestName + 1 spacing column.
	//   2. If that fits in (totalWidth - usedWidth) — i.e. name + builtins
	//      already fit even without any extras — keep the full reservation.
	//      Whatever room is left flows to extras; if it's not enough for a
	//      column, the loop below drops them and Name gets the slack via
	//      the caller's nameW computation. This is the case the user hit:
	//      a 52-char Node FQDN on a 97-char middle column was getting
	//      truncated to 50 chars + "~" because the previous totalWidth/2
	//      cap (48) kicked in even though the full name fits comfortably.
	//   3. Otherwise (name is too long to fit alongside builtins): cap at
	//      totalWidth - usedWidth - minExtrasBudget so a pathologically
	//      long name (e.g. 200 chars on a 120-char column) still surfaces
	//      at least one extra column.
	//   4. Floor at 20 to preserve prior behaviour when names are short.
	//
	// minExtrasBudget = capped column (maxColW + spacing). Tracks the
	// same maxColW used below so the budget scales with fullscreen mode.
	// compressedNameFloor is the smaller floor used when the user has
	// explicitly configured the columns: there NAME yields so the chosen
	// columns survive (issue #354).
	const nameFloor = 20
	const compressedNameFloor = len("NAME") + 1
	maxColW := 20
	if ActiveFullscreenMode {
		maxColW = 40
	}
	minExtrasBudget := maxColW + 1
	longestName := 0
	for _, item := range items {
		if w := lipgloss.Width(item.Name); w > longestName {
			longestName = w
		}
	}

	// Sum the width mandatory CRD printer columns need so the name reservation
	// below can be capped to leave room for them. Without this, long generated
	// resource names (issue #305) consume the budget and trailing user-declared
	// columns get silently dropped.
	mandatoryBudget := 0
	if mandatoryActive {
		for _, key := range candidates {
			if isMandatoryColumn(key) {
				w, _ := extraColWidth(seen[key], key, maxColW)
				mandatoryBudget += w
			}
		}
	}

	nameReserve := longestName + 1 // +1 for column spacing
	switch {
	case ActiveNameHidden:
		// NAME is hidden: reserve nothing so extras get the full row budget.
		nameReserve = 0
	case !fromAutoDetect:
		// Explicitly configured columns (column-toggle overlay or
		// views.<kind>.columns) are authoritative. The default longestName/20
		// reservation starves them in the narrow three-pane list, so the same
		// config that renders fully in the wide full screen list silently drops
		// trailing columns (issue #354). Reserve only what the configured
		// columns leave after their capped widths and let NAME compress to a
		// small floor; NAME still reclaims any leftover via the caller's nameW,
		// so wide panes are unchanged.
		configuredBudget := 0
		for _, key := range candidates {
			w, _ := extraColWidth(seen[key], key, maxColW)
			configuredBudget += w
		}
		nameReserve = max(totalWidth-usedWidth-configuredBudget, compressedNameFloor)
		if nameReserve > longestName+1 {
			nameReserve = max(longestName+1, compressedNameFloor)
		}
	default:
		if nameReserve+usedWidth > totalWidth {
			// Can't fit the full name even after dropping every extra. Cap
			// the reservation so at least one extra gets a fair budget.
			nameReserve = max(totalWidth-usedWidth-minExtrasBudget, nameFloor)
		}
		if mandatoryBudget > 0 {
			// Never let the name reservation crowd out mandatory columns; NAME
			// shrinks toward its floor and overflow is clipped by the caller.
			nameReserve = min(nameReserve, max(totalWidth-usedWidth-mandatoryBudget, nameFloor))
		}
		nameReserve = max(nameReserve, nameFloor)
	}
	// available may be negative when mandatory columns alone exceed the row;
	// fitExtraColumns still emits them and the caller clips the overflow.
	available := totalWidth - usedWidth - nameReserve
	// The "too tight to bother" bail-out is an auto-detect heuristic only.
	// Explicitly configured columns are authoritative, so let fitExtraColumns
	// surface whatever physically fits instead of dropping the whole set when
	// the pane is very narrow (issue #354).
	if available < 8 && mandatoryBudget == 0 && fromAutoDetect {
		return nil
	}

	return fitExtraColumns(candidates, seen, available, maxColW, mandatoryActive)
}

// fitExtraColumns selects columns from candidates that fit within available
// width and computes their display widths. Columns are added in order until the
// budget is exhausted (optional columns then stop); mandatory CRD printer
// columns are always emitted even past the budget, leaving the row to be
// clipped. Leftover budget is redistributed round-robin to capped columns.
//
// When mandatoryActive, callers must pass candidates with the mandatory
// (priority-0) columns as a contiguous prefix (see prioritizeMandatoryColumns):
// the budget break below ends the optional tail, so a mandatory column after an
// over-budget optional one would otherwise be lost.
func fitExtraColumns(candidates []string, seen map[string]*colInfo, available, maxColW int, mandatoryActive bool) []extraColumn {
	result := make([]extraColumn, 0, len(candidates))
	naturalW := make([]int, 0, len(candidates)) // pre-cap desired width including spacing
	remainingW := available
	for _, key := range candidates {
		info := seen[key]
		colW, natural := extraColWidth(info, key, maxColW)
		// Mandatory printer columns are always emitted; remainingW may go
		// negative, which is fine — NAME floors and the row is clipped.
		if colW > remainingW && (!mandatoryActive || !isMandatoryColumn(key)) {
			break
		}
		result = append(result, extraColumn{key: key, width: colW, hasArrow: info.hasArrow})
		naturalW = append(naturalW, natural)
		remainingW -= colW
	}

	// Redistribute remaining budget round-robin to columns that were capped
	// below their natural width. This avoids the failure mode where NAME gets
	// a large empty pad while Ports/Cluster IP/etc. are still truncated.
	// Growth stops at each column's natural width, so columns that already fit
	// don't get inflated — leftover beyond that flows back to NAME via the
	// caller's width calculation, preserving readable resource names.
	for remainingW > 0 {
		grew := false
		for i := range result {
			if result[i].width >= naturalW[i] {
				continue
			}
			result[i].width++
			remainingW--
			grew = true
			if remainingW == 0 {
				break
			}
		}
		if !grew {
			break
		}
	}

	return result
}

// selectColumnCandidates determines which extra columns to display based on
// session overrides, per-kind config, or auto-detection. The second return
// value reports whether the auto-detect path produced the result; only then
// may mandatory CRD printer-column protection reorder/force columns (an
// explicit session or config order is authoritative and left untouched).
//
// ActiveSessionColumns is the authoritative signal when non-nil: an empty
// slice means the user explicitly configured this kind with no extras and
// must not fall through to auto-detect. Only a nil slice means "no session
// override" and lets the config / auto-detect paths run.
func selectColumnCandidates(seen map[string]*colInfo, order []string, kind string, items []model.Item) (candidates []string, fromAutoDetect bool) {
	if ActiveSessionColumns != nil {
		out := make([]string, 0, len(ActiveSessionColumns))
		for _, key := range ActiveSessionColumns {
			if _, ok := seen[key]; ok {
				out = append(out, key)
			}
		}
		return out, false
	}

	// Build a ResourceRef so GVR-keyed view configs resolve. When the
	// rendered kind matches ActiveResourceRef (set by the app for the
	// middle column) carry through the full GVR; otherwise fall back to
	// Kind-only — at LevelOwned/LevelContainers the rendered kind diverges
	// from nav.ResourceType so GVR can't be trusted and Kind lookup applies.
	rt := ResourceRef{Kind: kind}
	if kind != "" && strings.EqualFold(kind, ActiveResourceRef.Kind) {
		rt = ActiveResourceRef
	}
	configCols := ColumnsForKind(rt, ActiveContext)
	if len(configCols) > 0 {
		if len(configCols) == 1 && configCols[0] == "*" {
			return order, false
		}
		var out []string
		for _, cfgKey := range configCols {
			if _, ok := seen[cfgKey]; ok {
				out = append(out, cfgKey)
			}
		}
		return out, false
	}

	return autoDetectColumns(seen, order, items), true
}

// overflowHiddenColumns are blocked columns that stay hidden even when the
// layout has spare width. Their values are long, multi-valued, URL/path-like,
// or can contain newlines, so a fixed-width overflow cell only truncates them
// to noise (and newlines break the single-line row layout). They remain
// reachable via the column-toggle overlay. Compact blocked columns (IPs, Node,
// QoS, Service Account, Priority Class, ...) are deliberately absent so they
// can fill spare width as overflow — see autoDetectColumns.
var overflowHiddenColumns = map[string]bool{
	"Images": true, "Image": true, "Labels": true, "Annotations": true,
	"Finalizers": true, "Selector": true, "Description": true, "References": true,
	"Keys": true, "Health Message": true, "Sync Message": true, "Sync Errors": true,
	"Repo": true, "Path": true, "Source": true, "Dest Server": true, "Used By": true,
	"Deletion": true,
}

// autoDetectColumns selects columns based on heuristic thresholds and blocked
// lists. It returns two groups concatenated: the primary columns (not blocked
// in the current mode) followed by overflow columns — compact columns that the
// mode blocks for space but which may fill spare width when the layout has room
// after the primary columns and the name's content. fitExtraColumns adds them
// in order and stops at the width budget, so overflow never pushes out a
// primary column or shrinks NAME below its content width.
//
// Overflow is fullscreen-only: the split-view block list is intentionally
// aggressive because the middle column shares width with the side panes, so
// revealing extras there is left untouched. Fullscreen is the wide single-pane
// layout where the spare width actually exists.
func autoDetectColumns(seen map[string]*colInfo, order []string, items []model.Item) []string {
	blocked := blockedColumnsForMode()
	// Raw metrics columns are always blocked.
	for _, k := range []string{"CPU Req", "CPU Lim", "Mem Req", "Mem Lim", "CPU Alloc", "Mem Alloc"} {
		blocked[k] = true
	}

	threshold := max(len(items)/5, 1)
	alwaysShow := map[string]bool{"Condition": true}
	var primary, overflow []string
	for _, key := range order {
		// CRD additionalPrinterColumns get kubectl semantics: priority 0 is
		// always shown (bypassing the threshold and blocked list), priority > 0
		// is hidden by default. A user can still reveal priority > 0 columns via
		// the column-toggle overlay (which routes through ActiveSessionColumns).
		if prio, ok := ActivePrinterColumns[key]; ok {
			if prio == 0 {
				primary = append(primary, key)
			}
			continue
		}
		if isHiddenColumnPrefix(key) {
			continue
		}
		info := seen[key]
		if info.count < threshold && !alwaysShow[key] {
			continue
		}
		switch {
		case !blocked[key]:
			primary = append(primary, key)
		case ActiveFullscreenMode && !overflowHiddenColumns[key]:
			overflow = append(overflow, key)
		}
	}
	return append(primary, overflow...)
}

// isHiddenColumnPrefix returns true if the column key uses a prefix reserved for internal data.
func isHiddenColumnPrefix(key string) bool {
	return strings.HasPrefix(key, "__") ||
		strings.HasPrefix(key, "secret:") ||
		strings.HasPrefix(key, "owner:") ||
		strings.HasPrefix(key, "data:") ||
		strings.HasPrefix(key, "condition:") ||
		strings.HasPrefix(key, "step:")
}

// blockedColumnsForMode returns the set of columns blocked in the current display mode.
// Description and References are always blocked because security findings put
// multi-line content into them (Falco's Details has \n-separated key/value
// pairs, References is a \n-joined URL list); they belong in the right-pane
// detail renderer, not in the explorer table where newlines break the layout.
func blockedColumnsForMode() map[string]bool {
	if ActiveFullscreenMode {
		return map[string]bool{
			"Health Message": true, "Keys": true,
			"Service Account": true, "Images": true, "Image": true,
			"Health": true, "Sync": true, "Path": true,
			"Description": true, "References": true,
			"Labels": true, "Finalizers": true, "Annotations": true,
			"Used By": true, "Deletion": true, "Selector": true,
		}
	}
	return map[string]bool{
		"IP": true, "Images": true, "Image": true,
		"Host IP": true, "Pod IP": true, "Cluster IP": true,
		"Repo": true, "Path": true, "Dest Server": true,
		"Health Message": true, "Keys": true,
		"Service Account": true, "Node": true,
		"QoS": true, "Priority Class": true,
		"Health": true, "Sync": true, "Dest NS": true,
		"Sync Message": true, "Sync Errors": true,
		"OS": true, "Runtime": true,
		"Hostname": true, "InternalIP": true, "ExternalIP": true,
		"Source":      true,
		"Description": true, "References": true,
		"Labels": true, "Finalizers": true, "Annotations": true,
		"Used By": true, "Deletion": true, "Selector": true,
	}
}

// GetExtraColumnValue retrieves the value for a given column key from an item.
func GetExtraColumnValue(item *model.Item, key string) string {
	if item == nil {
		return ""
	}
	for _, kv := range item.Columns {
		if kv.Key == key {
			return kv.Value
		}
	}
	return ""
}

// columnHeaderAliases shortens column header labels without renaming the
// underlying Column key. Renaming the key would silently break user session
// state, persisted column configs, and the column-visibility overlay; the
// header alias is purely cosmetic. Names listed here either:
//   - duplicate the resource type's name (Ingress -> "Ingress Class" =>
//     "Class"; the user already knows it's an Ingress).
//   - are unnecessarily verbose given typical column-width budgets.
var columnHeaderAliases = map[string]string{
	"Ingress Class":       "Class",
	"Storage Class":       "Class",
	"Disruptions Allowed": "Allowed",
	"Reclaim Policy":      "Reclaim",
	"Session Affinity":    "Affinity",
	"Image Pull Secrets":  "Pull Secrets",
	"Default Backend":     "Backend",
	"Last Transition":     "Transition",
	"Service Account":     "SA",
	// Security finding columns: the underlying camelCase keys are kept as-is
	// for ColumnValue() lookups in the details renderer; the alias is the
	// human-readable form rendered in the table header.
	"ResourceKind": "Resource Kind",
	"FindingCount": "Findings",
}

// ColumnHeaderLabel returns the uppercase display label for a column key,
// applying any alias from columnHeaderAliases. Used by plainExtraCell so
// internal Column keys can stay descriptive while the rendered table header
// stays compact.
func ColumnHeaderLabel(key string) string {
	if alias, ok := columnHeaderAliases[key]; ok {
		return strings.ToUpper(alias)
	}
	return strings.ToUpper(key)
}
