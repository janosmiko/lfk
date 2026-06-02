package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderTable renders items in a table format with column headers for resource views.
// headerLabel is used as the first column header; defaults to "NAME" if empty.
func RenderTable(headerLabel string, items []model.Item, cursor int, width, height int, loading bool, spinnerView string, errMsg string, showMarker ...bool) string { //nolint:gocyclo // rendering function with inherent layout complexity
	var b strings.Builder

	if len(items) == 0 {
		switch {
		case loading:
			b.WriteString(DimStyle.Render(spinnerView+" ") + DimStyle.Render("Loading..."))
		case errMsg != "":
			b.WriteString(ErrorStyle.Render(Truncate(errMsg, width)))
		default:
			b.WriteString(DimStyle.Render("No resources found"))
		}
		return b.String()
	}

	var hasContext, hasNs, hasReady, hasRestarts, hasAge, hasStatus bool
	var contextW, nsW, readyW, restartsW, ageW, statusW int
	var anyRecentRestart bool

	// Detect whether any row carries a ClusterName (the union-row signal)
	// so we can reserve a 1-cell leading tile column. The tile is painted
	// only when item.ClusterColor is set, but the cell is reserved on
	// every union-mode row so the column boundaries stay aligned across
	// rows whose source cluster doesn't have a configured color. In
	// non-union sessions hasUnion stays false and the row layout is
	// unchanged.
	hasUnion := false
	for _, item := range items {
		if item.ClusterName != "" {
			hasUnion = true
			break
		}
	}
	tileW := 0
	if hasUnion {
		tileW = 1
	}

	if ActiveTableLayout != nil && ActiveTableLayout.Computed {
		hasContext = ActiveTableLayout.HasContext
		hasNs = ActiveTableLayout.HasNs
		hasReady = ActiveTableLayout.HasReady
		hasRestarts = ActiveTableLayout.HasRestarts
		hasAge = ActiveTableLayout.HasAge
		hasStatus = ActiveTableLayout.HasStatus
		contextW = ActiveTableLayout.ContextW
		nsW = ActiveTableLayout.NsW
		readyW = ActiveTableLayout.ReadyW
		restartsW = ActiveTableLayout.RestartsW
		ageW = ActiveTableLayout.AgeW
		statusW = ActiveTableLayout.StatusW
		anyRecentRestart = ActiveTableLayout.AnyRecentRestart
	} else {
		for _, item := range items {
			if item.ClusterName != "" {
				hasContext = true
			}
			if item.Namespace != "" {
				hasNs = true
			}
			if item.Ready != "" {
				hasReady = true
			}
			if item.Restarts != "" {
				hasRestarts = true
			}
			if item.Age != "" {
				hasAge = true
			}
			if item.Status != "" {
				hasStatus = true
			}
		}

		if ActiveMiddleScroll >= 0 && ActiveHiddenBuiltinColumns != nil {
			if ActiveHiddenBuiltinColumns["Context"] {
				hasContext = false
			}
			if ActiveHiddenBuiltinColumns["Namespace"] {
				hasNs = false
			}
			if ActiveHiddenBuiltinColumns["Ready"] {
				hasReady = false
			}
			if ActiveHiddenBuiltinColumns["Restarts"] {
				hasRestarts = false
			}
			if ActiveHiddenBuiltinColumns["Age"] {
				hasAge = false
			}
			if ActiveHiddenBuiltinColumns["Status"] {
				hasStatus = false
			}
		}

		if hasContext {
			contextW = len("CONTEXT")
			for _, item := range items {
				if w := len(item.ClusterName); w > contextW {
					contextW = w
				}
			}
			contextW++
			if contextW > 30 {
				contextW = 30
			}
		}
		if hasNs {
			nsW = len("NAMESPACE")
			for _, item := range items {
				if w := len(item.Namespace); w > nsW {
					nsW = w
				}
			}
			nsW++
			if nsW > 30 {
				nsW = 30
			}
		}
		if hasReady {
			readyW = len("READY")
			for _, item := range items {
				if w := len(item.Ready); w > readyW {
					readyW = w
				}
			}
			readyW++
		}
		if hasRestarts {
			restartsW = len("RS") + 1
			for _, item := range items {
				if rc, _ := strconv.Atoi(item.Restarts); rc > 0 {
					if !item.LastRestartAt.IsZero() && time.Since(item.LastRestartAt) < time.Hour {
						anyRecentRestart = true
						break
					}
				}
			}
			for _, item := range items {
				w := len(item.Restarts)
				if anyRecentRestart {
					w++
				}
				if w >= restartsW {
					restartsW = w + 1
				}
			}
		}
		if hasAge {
			ageW = len("AGE") + 1
			for _, item := range items {
				if w := len(LiveAge(item)); w >= ageW {
					ageW = w + 1
				}
			}
			if ageW > 10 {
				ageW = 10
			}
		}
		if hasStatus {
			statusW = len("STATUS")
			for _, item := range items {
				if w := len(item.Status); w > statusW {
					statusW = w
				}
			}
			statusW++
			if statusW > 20 {
				statusW = 20
			}
		}
	}

	if hasNs && (ActiveTableLayout == nil || !ActiveTableLayout.Computed) {
		longestName := 0
		for _, item := range items {
			if w := len(item.Name); w > longestName {
				longestName = w
			}
		}
		markerW := 0
		if len(showMarker) == 0 || showMarker[0] {
			markerW = 2
		}
		fixedOther := contextW + readyW + restartsW + ageW + statusW + markerW + tileW
		nsHeaderW := len("NAMESPACE") + 1
		targetNs := max(width-fixedOther-(longestName+1), nsHeaderW)
		if targetNs < nsW {
			nsW = targetNs
		}
	}
	if hasStatus && (ActiveTableLayout == nil || !ActiveTableLayout.Computed) {
		longestName := 0
		for _, item := range items {
			if w := len(item.Name); w > longestName {
				longestName = w
			}
		}
		markerW := 0
		if len(showMarker) == 0 || showMarker[0] {
			markerW = 2
		}
		abbrevMaxW := len("STATUS")
		willShrinkAny := false
		for _, item := range items {
			abbr := AbbreviateStatusForWidth(item.Status, 0)
			if abbr != item.Status {
				willShrinkAny = true
			}
			if w := len(abbr); w > abbrevMaxW {
				abbrevMaxW = w
			}
		}
		abbrevStatusW := abbrevMaxW + 1
		if willShrinkAny && abbrevStatusW < statusW {
			fixedOther := contextW + readyW + restartsW + ageW + markerW + tileW
			minNsW := 0
			if hasNs {
				minNsW = min(len("NAMESPACE")+1, nsW)
			}
			if width-fixedOther-statusW-minNsW-(longestName+1) < 0 {
				statusW = abbrevStatusW
			}
		}
	}
	wantMarker := len(showMarker) == 0 || showMarker[0]
	markerColW := 0
	if wantMarker {
		markerColW = 2
	}

	var extraCols []extraColumn
	if ActiveTableLayout != nil && ActiveTableLayout.Computed {
		extraCols = ActiveTableLayout.ExtraCols
	} else {
		tableKind := ""
		if len(items) > 0 {
			tableKind = items[0].Kind
		}
		extraCols = collectExtraColumns(items, width, contextW+nsW+readyW+restartsW+ageW+statusW+markerColW+tileW, tableKind)

		filtered := extraCols[:0]
		for _, ec := range extraCols {
			if hasContext && ec.key == "Context" {
				continue
			}
			if !isBuiltinColumnKey(ec.key) {
				filtered = append(filtered, ec)
			}
		}
		extraCols = filtered

		if ActiveTableLayout != nil {
			ActiveTableLayout.HasContext = hasContext
			ActiveTableLayout.HasNs = hasNs
			ActiveTableLayout.HasReady = hasReady
			ActiveTableLayout.HasRestarts = hasRestarts
			ActiveTableLayout.HasAge = hasAge
			ActiveTableLayout.HasStatus = hasStatus
			ActiveTableLayout.ContextW = contextW
			ActiveTableLayout.NsW = nsW
			ActiveTableLayout.ReadyW = readyW
			ActiveTableLayout.RestartsW = restartsW
			ActiveTableLayout.AgeW = ageW
			ActiveTableLayout.StatusW = statusW
			ActiveTableLayout.AnyRecentRestart = anyRecentRestart
			ActiveTableLayout.ExtraCols = extraCols
			ActiveTableLayout.Computed = true
		}
	}

	if ActiveMiddleScroll >= 0 {
		ActiveExtraColumnKeys = ActiveExtraColumnKeys[:0]
		for _, ec := range extraCols {
			ActiveExtraColumnKeys = append(ActiveExtraColumnKeys, ec.key)
		}
	}

	order := orderedColumnKeys(hasContext, hasNs, hasReady, hasRestarts, hasStatus, hasAge, extraCols)

	if ActiveMiddleScroll >= 0 {
		ActiveSortableColumns = ActiveSortableColumns[:0]
		ActiveSortableColumns = append(ActiveSortableColumns, "Name")
		ActiveSortableColumns = append(ActiveSortableColumns, order...)
		ActiveSortableColumnCount = len(ActiveSortableColumns)
		ActiveSortColumn = 0
		for i, col := range ActiveSortableColumns {
			if col == ActiveSortColumnName {
				ActiveSortColumn = i
				break
			}
		}
	}

	extraTotalW := 0
	for _, ec := range extraCols {
		extraTotalW += ec.width
	}

	nameW := max(width-contextW-nsW-readyW-restartsW-ageW-statusW-markerColW-extraTotalW-tileW, 10)

	if headerLabel == "" {
		headerLabel = "NAME"
	}
	nameHeader := headerWithIndicator(headerLabel, "Name", nameW)
	colWidths := builtinColWidths{context: contextW, ns: nsW, ready: readyW, restarts: restartsW, status: statusW, age: ageW}
	colHeaders := builtinColHeaders{
		context:  headerWithIndicator("CONTEXT", "Context", contextW),
		ns:       headerWithIndicator("NAMESPACE", "Namespace", nsW),
		ready:    headerWithIndicator("READY", "Ready", readyW),
		restarts: headerWithIndicator("RS", "Restarts", restartsW),
		status:   headerWithIndicator("STATUS", "Status", statusW),
		age:      headerWithIndicator("AGE", "Age", ageW),
	}

	var hdrSegments []headerSegment
	if wantMarker {
		hdrSegments = append(hdrSegments, headerSegment{text: "  "})
	}
	if tileW > 0 {
		// Reserve a blank cell in the header so the leading tile column
		// stays aligned with the data rows below.
		hdrSegments = append(hdrSegments, headerSegment{text: " "})
	}
	hdrSegments = append(hdrSegments, headerSegment{text: nameHeader, colName: "Name"})
	for _, key := range order {
		hdrSegments = append(hdrSegments, headerSegment{
			text:    headerCellForKey(key, colWidths, colHeaders, extraCols),
			colName: key,
		})
	}
	b.WriteString(renderStyledHeader(hdrSegments, width))
	height--

	if ActiveMiddleScroll >= 0 {
		ActiveMiddleColumnLayout = ActiveMiddleColumnLayout[:0]
		x := 0
		if wantMarker {
			x += markerColW
		}
		if tileW > 0 {
			x += tileW
		}
		ActiveMiddleColumnLayout = append(ActiveMiddleColumnLayout, MiddleColumnRegion{Key: "Name", StartX: x, EndX: x + nameW})
		x += nameW
		for _, key := range order {
			w := widthForColumnKey(key, colWidths, extraCols)
			ActiveMiddleColumnLayout = append(ActiveMiddleColumnLayout, MiddleColumnRegion{Key: key, StartX: x, EndX: x + w})
			x += w
		}
	}

	hasCategories := false
	categoryForItem := make([]string, len(items))
	hasSepForItem := make([]bool, len(items))
	{
		lastCat := ""
		for i, item := range items {
			if item.Category != "" && item.Category != lastCat {
				categoryForItem[i] = item.Category
				if lastCat != "" {
					hasCategories = true
					hasSepForItem[i] = true
				}
				lastCat = item.Category
			}
		}
		if !hasCategories {
			for i := range categoryForItem {
				categoryForItem[i] = ""
				hasSepForItem[i] = false
			}
		}
	}

	categoryLines := func(start, end int) int {
		n := 0
		for i := start; i < end && i < len(items); i++ {
			if categoryForItem[i] != "" {
				n++
			}
			if hasSepForItem[i] && i > start {
				n++
			}
		}
		return n
	}

	tableDisplayLines := func(from, to int) int {
		return (to - from) + categoryLines(from, to)
	}

	scrollOff := ConfigScrollOff
	startIdx := 0
	if ActiveMiddleScroll >= 0 {
		startIdx = VimScrollOff(ActiveMiddleScroll, cursor, len(items), height, scrollOff, tableDisplayLines)
		ActiveMiddleScroll = startIdx
	} else {
		totalDisplayLines := tableDisplayLines(0, len(items))
		if totalDisplayLines <= height {
			scrollOff = 0
		} else if maxSO := (height - 1) / 2; scrollOff > maxSO {
			scrollOff = maxSO
		}
		if cursor >= 0 {
			displayLinesUpTo := func(start, idx int) int {
				return tableDisplayLines(start, idx+1)
			}
			for startIdx < len(items) && displayLinesUpTo(startIdx, cursor) > height {
				startIdx++
			}
			if cursor+scrollOff < len(items) {
				for startIdx < len(items) && displayLinesUpTo(startIdx, cursor+scrollOff) > height {
					startIdx++
				}
			}
			if cursor-scrollOff >= 0 && startIdx > cursor-scrollOff {
				startIdx = max(cursor-scrollOff, 0)
			}
			for startIdx > 0 {
				if tableDisplayLines(startIdx-1, len(items)) > height {
					break
				}
				startIdx--
			}
		}
	}

	usedLines := 0
	endIdx := startIdx
	for endIdx < len(items) {
		extraLines := 0
		if categoryForItem[endIdx] != "" {
			extraLines++
		}
		if hasSepForItem[endIdx] && endIdx > startIdx {
			extraLines++
		}
		if usedLines+1+extraLines > height {
			break
		}
		usedLines += 1 + extraLines
		endIdx++
	}

	if ActiveMiddleScroll >= 0 {
		ActiveMiddleLineMap = ActiveMiddleLineMap[:0]
		for i := startIdx; i < endIdx; i++ {
			if hasSepForItem[i] && i > startIdx {
				ActiveMiddleLineMap = append(ActiveMiddleLineMap, -1)
			}
			if hasCategories && categoryForItem[i] != "" {
				ActiveMiddleLineMap = append(ActiveMiddleLineMap, -1)
			}
			ActiveMiddleLineMap = append(ActiveMiddleLineMap, i)
		}
	}

	for i := startIdx; i < endIdx; i++ {
		item := items[i]

		if hasSepForItem[i] && i > startIdx {
			b.WriteString("\n")
		}

		if hasCategories && categoryForItem[i] != "" {
			headerLine := Truncate(categoryForItem[i], width)
			if w := lipgloss.Width(headerLine); w < width {
				headerLine += strings.Repeat(" ", width-w)
			}
			b.WriteString("\n" + CategoryBarStyle.Render(headerLine))
		}

		b.WriteString("\n")

		ns := item.Namespace
		if ns == "" && hasNs {
			ns = "-"
		}

		displayName := item.Name
		if icon := resolveIcon(item.Icon); icon != "" {
			displayName = icon + " " + item.Name
		}

		selected := isItemSelected(item)

		if i == cursor {
			markerPrefix := ""
			if wantMarker {
				markerPrefix = "  "
				if selected {
					markerPrefix = selectionMarker
				}
			}
			tilePrefix := ""
			if tileW > 0 {
				// Cursor row: the tile must re-assert the selection style
				// after its own SGR reset, or the highlight dies for the
				// rest of the row.
				tilePrefix = ClusterColorTileBgOver(item.ClusterColor, ActiveSelectedStyle(i))
			}
			cursorRestarts := item.Restarts
			if hasRestarts {
				restartCount, _ := strconv.Atoi(item.Restarts)
				recentRestart := !item.LastRestartAt.IsZero() && time.Since(item.LastRestartAt) < time.Hour
				if restartCount > 0 && recentRestart {
					cursorRestarts = "↑" + item.Restarts
				} else if anyRecentRestart {
					cursorRestarts = " " + item.Restarts
				}
			}
			row := markerPrefix + tilePrefix + formatTableRowOrdered(displayName, ns, item.Ready, cursorRestarts, item.Status, LiveAge(item),
				nameW, contextW, nsW, readyW, restartsW, statusW, ageW, order, extraCols, &item)
			highlighted := false
			if ActiveHighlightQuery != "" {
				row = highlightNameSelectedOver(row, ActiveHighlightQuery, ActiveSelectedStyle(i))
				highlighted = true
			}
			lineW := lipgloss.Width(row)
			if lineW < width {
				row += strings.Repeat(" ", width-lineW)
			}
			if highlighted {
				b.WriteString(RenderOverPrestyled(row, ActiveSelectedStyle(i)))
			} else {
				b.WriteString(ActiveSelectedStyle(i).MaxWidth(width).Render(row))
			}
		} else {
			var rendered string
			if ActiveRowCache != nil {
				rendered = ActiveRowCache[i]
			}
			if rendered == "" {
				markerPrefix := ""
				if wantMarker {
					markerPrefix = "  "
					if selected {
						markerPrefix = SelectionMarkerStyle.Render(selectionMarker)
					}
				}
				tilePrefix := ""
				if tileW > 0 {
					tilePrefix = ClusterColorTileBg(item.ClusterColor)
				}
				rendered = markerPrefix + tilePrefix + formatTableRowStyledOrdered(item, nameW, contextW, nsW, readyW, restartsW, statusW, ageW,
					order, extraCols, anyRecentRestart)
				if ActiveRowCache != nil {
					ActiveRowCache[i] = rendered
				}
			}
			b.WriteString(rendered)
		}

	}
	return b.String()
}
