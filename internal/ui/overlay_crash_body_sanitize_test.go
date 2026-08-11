package ui

import (
	"strings"
	"testing"
)

// The Logs and Describe tabs render the least constrained content in the app:
// raw container stdout, and the verbatim text of kubectl describe, which prints
// annotation values the API server does not restrict. Both are a second render
// path to bytes the log viewer already guards. Found by the security-reviewer
// pass on TASK-880 after the first round missed them.
func TestCrashLogsAndDescribeTabsDropHostileSequences(t *testing.T) {
	payloads := map[string]struct{ payload, marker string }{
		"OSC-52 clipboard write": {"\x1b]52;c;aGF4\a", "\x1b]"},
		"CSI erase display":      {"\x1b[2J", "\x1b[2J"},
		"bidi override":          {"\u202e", "\u202e"},
		"C1 control":             {"\u009b", "\u009b"},
	}

	for name, tc := range payloads {
		t.Run(name, func(t *testing.T) {
			entry := CrashInvestigatorEntry{
				PodName:         "api-7f9" + tc.payload,
				ActiveContainer: "app" + tc.payload,
				Describe:        "Annotations:  note=" + tc.payload + "\nStatus: Running",
				AppContainers: []CrashContainerEntry{{
					Name:       "app" + tc.payload,
					CurrentLog: "starting up" + tc.payload + "\nready",
				}},
			}
			entry.ActiveContainer = "app" + tc.payload

			logs := renderCrashLogsTab(entry, 0, 100, 30)
			if strings.Contains(logs, tc.marker) {
				t.Errorf("Logs tab: %s survived", name)
			}
			describe := renderCrashDescribeTab(entry, 0, 100, 30)
			if strings.Contains(describe, tc.marker) {
				t.Errorf("Describe tab: %s survived", name)
			}
			if strings.Contains(logs, "\a") || strings.Contains(describe, "\a") {
				t.Errorf("%s: BEL survived", name)
			}
		})
	}
}

// A container that colours its own output must still read correctly, so the
// body path keeps SGR rather than reaching for the blunt sanitizer.
func TestCrashLogsTabKeepsColour(t *testing.T) {
	entry := CrashInvestigatorEntry{
		ActiveContainer: "app",
		AppContainers:   []CrashContainerEntry{{Name: "app", CurrentLog: "\x1b[31mpanic\x1b[0m"}},
	}
	if got := renderCrashLogsTab(entry, 0, 100, 30); !strings.Contains(got, "[31m") {
		t.Error("SGR colour must survive the logs body path")
	}
}

// The tab bodies span many lines, and the body sanitizer treats a newline as a
// control byte. Sanitizing the whole string at once would collapse the body
// onto one row.
func TestCrashBodySanitizerKeepsLineStructure(t *testing.T) {
	got := sanitizeCrashBody("one\ntwo\nthree")
	if want := 3; len(strings.Split(got, "\n")) != want {
		t.Errorf("expected %d lines, got %q", want, got)
	}
}

// The Summary tab's metadata rows are the pod's own identity: Phase, Node, IP,
// QoS, Owner, and the container named in the last-termination heading. They sit
// beside fields the same function already guarded, which is the shape of miss
// this task keeps finding.
func TestCrashSummaryMetadataDropsHostileSequences(t *testing.T) {
	payloads := map[string]struct{ payload, marker string }{
		"OSC-52 clipboard write": {"\x1b]52;c;aGF4\a", "\x1b]"},
		"CSI erase display":      {"\x1b[2J", "\x1b[2J"},
		"bidi override":          {"\u202e", "\u202e"},
		"C1 control":             {"\u009b", "\u009b"},
	}

	for name, tc := range payloads {
		t.Run(name, func(t *testing.T) {
			entry := CrashInvestigatorEntry{
				PodName:         "api" + tc.payload,
				Namespace:       "prod" + tc.payload,
				Phase:           "Running" + tc.payload,
				Node:            "node-1" + tc.payload,
				PodIP:           "10.0.0.1" + tc.payload,
				QoSClass:        "Burstable" + tc.payload,
				OwnerKind:       "ReplicaSet",
				OwnerName:       "api-abc" + tc.payload,
				ActiveContainer: "app" + tc.payload,
				AppContainers: []CrashContainerEntry{{
					Name:        "app" + tc.payload,
					HasLastTerm: true,
					LastReason:  "Error",
				}},
			}

			out := strings.Join(buildCrashSummaryLines(entry), "\n")
			if strings.Contains(out, tc.marker) {
				t.Errorf("Summary tab: %s survived", name)
			}
			if strings.Contains(out, "\a") {
				t.Errorf("%s: BEL survived", name)
			}
		})
	}
}
