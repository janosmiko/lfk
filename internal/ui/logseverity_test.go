package ui

import "testing"

func TestLineSeverity(t *testing.T) {
	tests := []struct {
		name string
		line string
		want int
	}{
		// klog: level field value is "Error" / "Warning" (display names from klogLevelName)
		{"klog error", "E0416 12:00:00.000000       1 main.go:10] boom", SevError},
		{"klog warn", "W0416 12:00:00.000000       1 main.go:10] careful", SevWarn},
		// JSON: level field carries raw token; lvl is an alias recognised by jsonKeyRank
		{"json error", `{"level":"error","msg":"db down"}`, SevError},
		{"json warn lvl", `{"lvl":"warn","msg":"slow"}`, SevWarn},
		// Logfmt: three pairs, level=debug
		{"logfmt", `time=2024 level=debug msg=hi`, SevDebug},
		// Zap (controller-runtime dev/console encoder): LEVEL\tMESSAGE
		{"zap info", "INFO\tstarting manager", SevInfo},
		{"zap error", "ERROR\treconcile failed", SevError},
		// Java Logback: HH:mm:ss.SSS [thread] LEVEL logger - msg
		{"java logback warn", "10:30:00.123 [main] WARN com.example.MyService - slow query", SevWarn},
		// Java Spring Boot: RFC3339 leading timestamp (stripped), then LEVEL PID --- [thread] logger : msg
		{"java spring error", "2024-01-15T10:30:00.123+00:00  ERROR 12345 --- [main] c.e.MyService : DB connection failed", SevError},
		// Postgres: uses "severity" key; LOG maps to SevInfo, DEBUG1 strips digit to "debug"
		{"postgres log", "2024-01-15 10:30:00.123 UTC [1234] LOG:  database system is ready to accept connections", SevInfo},
		{"postgres error", "2024-01-15 10:30:00.123 UTC [1234] ERROR:  relation \"foo\" does not exist", SevError},
		// Plain text has no level field
		{"plain text", "just a plain line with no level", SevUnknown},
		// Java stack trace tail is plain text
		{"stack trace tail", "\tat com.example.Foo.bar(Foo.java:42)", SevUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LineSeverity(tt.line); got != tt.want {
				t.Errorf("LineSeverity(%q) = %d, want %d", tt.line, got, tt.want)
			}
		})
	}
}

func TestSeverityRank(t *testing.T) {
	cases := map[string]int{
		"Error": SevError, "ERROR": SevError, "err": SevError,
		"Warning": SevWarn, "warn": SevWarn,
		"Info": SevInfo, "notice": SevInfo, "LOG": SevInfo,
		"debug": SevDebug, "debug2": SevDebug, "trace": SevTrace,
		"Fatal": SevFatal, "panic": SevFatal, "dpanic": SevFatal, "critical": SevFatal,
		"": SevUnknown, "bogus": SevUnknown,
	}
	for in, want := range cases {
		if got := SeverityRank(in); got != want {
			t.Errorf("SeverityRank(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestSeverityName(t *testing.T) {
	if SeverityName(SevWarn) != "WARN" {
		t.Errorf("SeverityName(SevWarn) = %q, want WARN", SeverityName(SevWarn))
	}
	if SeverityName(SevUnknown) != "" {
		t.Errorf("SeverityName(SevUnknown) should be empty")
	}
}
