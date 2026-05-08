package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

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

// ActiveHiddenBuiltinColumns holds the set of built-in column keys that should
// be suppressed in the current middle-column render. Valid keys: "Context",
// "Namespace", "Ready", "Restarts", "Age", "Status". Set by the app before
// rendering. Nil means no overrides.
var ActiveHiddenBuiltinColumns map[string]bool

// collectExtraColumns discovers which extra columns to show based on item data and config.
// usedWidth is the width already consumed by fixed columns (excluding name).
// kind is the resource Kind (e.g. "Pod") used to resolve per-type column overrides.
// colInfo tracks metadata about a single extra column during collection.
type colInfo struct {
	key      string
	maxValW  int
	count    int
	hasArrow bool // true if any value in this column has a trend arrow
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

func collectExtraColumns(items []model.Item, totalWidth, usedWidth int, kind string) []extraColumn {
	// Collect all available column keys and their max value widths.
	seen := make(map[string]*colInfo)
	var order []string
	for _, item := range items {
		for _, kv := range item.Columns {
			info, ok := seen[kv.Key]
			if !ok {
				info = &colInfo{key: kv.Key}
				seen[kv.Key] = info
				order = append(order, kv.Key)
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

	// Pin the metrics block to a canonical position before candidate
	// selection so display order does not depend on the order the refresh
	// paths happened to write the columns into item.Columns.
	stabilizeColumnOrder(order)

	candidates := selectColumnCandidates(seen, order, kind, items)

	if len(candidates) == 0 {
		return nil
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
	const nameFloor = 20
	maxColWForBudget := 20
	if ActiveFullscreenMode {
		maxColWForBudget = 40
	}
	minExtrasBudget := maxColWForBudget + 1
	longestName := 0
	for _, item := range items {
		if w := lipgloss.Width(item.Name); w > longestName {
			longestName = w
		}
	}
	nameReserve := longestName + 1 // +1 for column spacing
	if nameReserve+usedWidth > totalWidth {
		// Can't fit the full name even after dropping every extra. Cap
		// the reservation so at least one extra gets a fair budget.
		nameReserve = max(totalWidth-usedWidth-minExtrasBudget, nameFloor)
	}
	nameReserve = max(nameReserve, nameFloor)
	available := totalWidth - usedWidth - nameReserve
	if available < 8 {
		return nil
	}

	result := make([]extraColumn, 0, len(candidates))
	naturalW := make([]int, 0, len(candidates)) // pre-cap desired width including spacing
	remainingW := available
	maxColW := 20
	if ActiveFullscreenMode {
		maxColW = 40
	}
	for _, key := range candidates {
		info := seen[key]
		// Column width: max of header length and value length, capped, plus 1 for spacing.
		colW := len(key)
		maxVal := info.maxValW
		// When some values have arrows, non-arrow values need a placeholder space.
		// The arrow values already include the arrow in their visual width,
		// so ensure non-arrow values get +1 to match.
		if info.hasArrow {
			maxVal++ // reserve space for placeholder on non-arrow rows
		}
		if maxVal > colW {
			colW = maxVal
		}
		natural := colW + 1 // pre-cap natural width (with spacing)
		if colW > maxColW {
			colW = maxColW
		}
		colW++ // spacing
		if colW > remainingW {
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
// session overrides, per-kind config, or auto-detection.
//
// ActiveSessionColumns is the authoritative signal when non-nil: an empty
// slice means the user explicitly configured this kind with no extras and
// must not fall through to auto-detect. Only a nil slice means "no session
// override" and lets the config / auto-detect paths run.
func selectColumnCandidates(seen map[string]*colInfo, order []string, kind string, items []model.Item) []string {
	if ActiveSessionColumns != nil {
		candidates := make([]string, 0, len(ActiveSessionColumns))
		for _, key := range ActiveSessionColumns {
			if _, ok := seen[key]; ok {
				candidates = append(candidates, key)
			}
		}
		return candidates
	}

	configCols := ColumnsForKind(kind, ActiveContext)
	if len(configCols) > 0 {
		if len(configCols) == 1 && configCols[0] == "*" {
			return order
		}
		var candidates []string
		for _, cfgKey := range configCols {
			if _, ok := seen[cfgKey]; ok {
				candidates = append(candidates, cfgKey)
			}
		}
		return candidates
	}

	return autoDetectColumns(seen, order, items)
}

// autoDetectColumns selects columns based on heuristic thresholds and blocked lists.
func autoDetectColumns(seen map[string]*colInfo, order []string, items []model.Item) []string {
	blocked := blockedColumnsForMode()
	// Raw metrics columns are always blocked.
	for _, k := range []string{"CPU Req", "CPU Lim", "Mem Req", "Mem Lim", "CPU Alloc", "Mem Alloc"} {
		blocked[k] = true
	}

	threshold := max(len(items)/5, 1)
	alwaysShow := map[string]bool{"Condition": true}
	var candidates []string
	for _, key := range order {
		if blocked[key] || isHiddenColumnPrefix(key) {
			continue
		}
		info := seen[key]
		if info.count >= threshold || alwaysShow[key] {
			candidates = append(candidates, key)
		}
	}
	return candidates
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
