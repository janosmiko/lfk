package app

import (
	"errors"
	"strings"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// composeUnionMonitoring renders the merged alerts of all members plus a MEMBERS list.
func composeUnionMonitoring(contexts []string, byCtx map[string]monitoringData) string {
	var alerts []k8s.AlertInfo
	members := make([]unionMember, 0, len(contexts))
	for _, kctx := range contexts {
		d, ok := byCtx[kctx]
		switch {
		case !ok:
			members = append(members, unionMember{ctx: kctx, state: unionMemberLoading})
		case d.errMsg != "":
			members = append(members, unionMember{ctx: kctx, state: unionMemberError, err: errors.New(d.errMsg)})
		default:
			members = append(members, unionMember{ctx: kctx, state: unionMemberOK})
			for _, a := range d.alerts {
				a.Context = kctx
				alerts = append(alerts, a)
			}
		}
	}

	lines := []string{"", ui.DimStyle.Bold(true).Render("  MONITORING OVERVIEW"), ""}
	lines = monitoringAlertSummary(lines, alerts)
	lines = append(lines, "")
	sortAlerts(alerts)
	lines = monitoringAlertTable(lines, alerts)
	lines = append(lines, "", ui.DimStyle.Bold(true).Render("  MEMBERS"), "")
	lines = append(lines, unionMemberLines(members)...)
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}
