package ui

import "testing"

func TestLineLogLevel_Structured(t *testing.T) {
	tests := []struct {
		name string
		line string
		want int
	}{
		// klog: level "Error"/"Warning" (display names) -> buckets
		{"klog error", "E0416 12:00:00.000000       1 main.go:10] boom", LogError},
		{"klog warn", "W0416 12:00:00.000000       1 main.go:10] careful", LogWarn},
		// JSON: level/lvl tokens
		{"json error", `{"level":"error","msg":"db down"}`, LogError},
		{"json warn lvl", `{"lvl":"warn","msg":"slow"}`, LogWarn},
		// Logfmt: level=debug -> debug bucket
		{"logfmt debug", `time=2024 level=debug msg=hi`, LogDebug},
		// Zap
		{"zap info", "INFO\tstarting manager", LogInfo},
		{"zap error", "ERROR\treconcile failed", LogError},
		// Java Logback / Spring Boot
		{"java logback warn", "10:30:00.123 [main] WARN com.example.MyService - slow query", LogWarn},
		{"java spring error", "2024-01-15T10:30:00.123+00:00  ERROR 12345 --- [main] c.e.MyService : boom", LogError},
		// Postgres: LOG -> info bucket; FATAL -> error bucket
		{"postgres log", "2024-01-15 10:30:00.123 UTC [1234] LOG:  ready", LogInfo},
		{"postgres fatal", "2024-01-15 10:30:00.123 UTC [1234] FATAL:  the database system is shutting down", LogError},
		// log4j2 / OpenSearch
		{"log4j info", "[2026-06-17T01:50:33,660][INFO ][o.o.a.t.Cron] [node-0] hourly cron", LogInfo},
		{"log4j warn", "[2026-06-17T01:50:33,660][WARN ][o.o.j.s.JobSweeper] [node-0] slow sweep", LogWarn},
		// Abseil/glog (Dragonfly): 8-digit YYYYMMDD date. The level letter is
		// authoritative even when the message text contains "error" — these
		// would be mis-bucketed by a plain keyword scan.
		{"glog warn msg has Error", `W20260429 10:55:50.349922    12 common.cc:346] ReportError: Operation canceled`, LogWarn},
		{"glog info msg has error", `I20260429 10:55:50.350005    12 dflycmd.cc:747] Replication error: canceled`, LogInfo},
		{"glog error", `E20260429 10:55:50.350005    12 x.cc:1] boom`, LogError},
		// klog 4-digit MMDD still works
		{"klog 4-digit info", "I0416 12:00:00.000000       1 main.go:10] starting", LogInfo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LineLogLevel(tt.line); got != tt.want {
				t.Errorf("LineLogLevel(%q) = %d, want %d", tt.line, got, tt.want)
			}
		})
	}
}

func TestLineLogLevel_PlainTextKeywords(t *testing.T) {
	tests := []struct {
		name string
		line string
		want int
	}{
		// No keyword -> default INFO (shown at INFO+, hidden at WARN+)
		{"plain default info", "Running full sweep", LogInfo},
		{"plain default info 2", "Start running AD hourly cron.", LogInfo},
		// Error keywords
		{"error word", "connection error: timeout", LogError},
		{"failed word", "request failed after 3 retries", LogError},
		{"exception word", "uncaught Exception in handler", LogError},
		// Warn keywords
		{"warning word", "disk space warning: 90% used", LogWarn},
		// Debug keywords
		{"debug word", "DEBUG entering reconcile loop", LogDebug},
		{"trace word", "trace: span started", LogDebug},
		// Word boundary: "err" must not match inside "preferred"
		{"no false positive", "preferred replica selected", LogInfo},
		// Error wins over warn when both present
		{"error beats warn", "warning: this will error out", LogError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LineLogLevel(tt.line); got != tt.want {
				t.Errorf("LineLogLevel(%q) = %d, want %d", tt.line, got, tt.want)
			}
		})
	}
}

func TestLogLevelName(t *testing.T) {
	cases := map[int]string{
		LogInfo:  "INFO",
		LogWarn:  "WARN",
		LogError: "ERROR",
		LogDebug: "", // debug/off is never a threshold indicator
		99:       "",
	}
	for in, want := range cases {
		if got := LogLevelName(in); got != want {
			t.Errorf("LogLevelName(%d) = %q, want %q", in, got, want)
		}
	}
}
