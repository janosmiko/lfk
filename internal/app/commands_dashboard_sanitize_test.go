package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// hostileDashboardPayloads are the sequences TASK-880 sinks are driven with,
// keyed so a failing assertion names the attack. Written as literal escapes
// (not the invisible runes they represent) so a diff shows what leaked.
var hostileDashboardPayloads = map[string]string{
	"bidi override":         "\u202e",
	"raw csi":               "\x1b[31m",
	"csi screen erase":      "\x1b[2J",
	"osc52 clipboard write": "\x1b]52;c;aGF4\x07",
}

func warningEventItem(payload string) model.Item {
	return model.Item{
		Name:      "pod-a",
		Namespace: "ns" + payload,
		Age:       "5m",
		Columns: []model.KeyValue{
			{Key: "Reason", Value: "CrashLoopBackOff" + payload},
			{Key: "Object", Value: "pod/app" + payload},
			{Key: "Message", Value: "back-off restarting" + payload},
			{Key: "Count", Value: "1"},
		},
	}
}

// TestDashboardInlineEventsSection_SanitizesFields guards the single-column
// RECENT WARNING EVENTS section: reason, object, and message all come from
// a cluster Event resource via extractEventFields.
func TestDashboardInlineEventsSection_SanitizesFields(t *testing.T) {
	for name, payload := range hostileDashboardPayloads {
		t.Run(name, func(t *testing.T) {
			lines := dashboardInlineEventsSection(nil, []model.Item{warningEventItem(payload)}, dashboardWidths{sep: 40})
			for _, line := range lines {
				assert.NotContains(t, line, payload)
			}
		})
	}
}

// TestDashboardEventsColumn_SanitizesFields guards the two-column layout's
// dedicated events column, including the namespace label rendered directly
// from ev.Namespace (not routed through extractEventFields).
func TestDashboardEventsColumn_SanitizesFields(t *testing.T) {
	for name, payload := range hostileDashboardPayloads {
		t.Run(name, func(t *testing.T) {
			lines := dashboardEventsColumn([]model.Item{warningEventItem(payload)})
			for _, line := range lines {
				assert.NotContains(t, line, payload)
			}
		})
	}
}

// TestRenderAlertLabels_SanitizesKeysAndValues guards the Prometheus alert
// label lines: both the label key and value are attacker-controllable
// (a rule author or the metric source sets them).
func TestRenderAlertLabels_SanitizesKeysAndValues(t *testing.T) {
	for name, payload := range hostileDashboardPayloads {
		t.Run(name, func(t *testing.T) {
			labels := map[string]string{
				"pod" + payload: "value" + payload,
			}
			lines := renderAlertLabels(nil, labels, nil)
			for _, line := range lines {
				assert.NotContains(t, line, payload)
			}
		})
	}
}

// TestRenderAlertRow_SanitizesNamespaceLabel guards the alert row's
// namespace column, built directly from a.Labels["namespace"].
func TestRenderAlertRow_SanitizesNamespaceLabel(t *testing.T) {
	for name, payload := range hostileDashboardPayloads {
		t.Run(name, func(t *testing.T) {
			a := k8s.AlertInfo{
				State:    "firing",
				Severity: "critical",
				Labels:   map[string]string{"namespace": "ns" + payload},
			}
			out := renderAlertRow(a)
			assert.NotContains(t, out, payload)
		})
	}
}

// The severity and state columns switch on the value and render it in every
// branch. A value matching none of the known cases lands in the default
// branch, which the earlier test could not reach because it passed "critical".
func TestRenderAlertRow_SanitizesUnknownStateAndSeverity(t *testing.T) {
	payloads := map[string]struct{ payload, marker string }{
		"OSC-52 clipboard write": {"\x1b]52;c;aGF4\a", "\x1b]"},
		"CSI erase display":      {"\x1b[2J", "\x1b[2J"},
		"bidi override":          {"\u202e", "\u202e"},
		"C1 control":             {"\u009b", "\u009b"},
	}

	for name, tc := range payloads {
		t.Run(name, func(t *testing.T) {
			row := renderAlertRow(k8s.AlertInfo{
				Name:     "HighLatency",
				State:    "odd" + tc.payload,     // matches no known case
				Severity: "unusual" + tc.payload, // matches no known case
				Labels:   map[string]string{"namespace": "prod"},
			})
			if strings.Contains(row, tc.marker) {
				t.Errorf("%s survived the alert row", name)
			}
			if strings.Contains(row, "\a") {
				t.Errorf("%s: BEL survived", name)
			}
		})
	}
}
