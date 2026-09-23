package ui

import "testing"

func TestFilterKVKeys_HonorsSearchMode(t *testing.T) {
	keys := []string{"DATABASE_URL", "API_KEY", "LOG_LEVEL"}

	tests := []struct {
		name  string
		mode  string
		query string
		want  []string
	}{
		{"default substring", DefaultSearchModeDefault, "key", []string{"API_KEY"}},
		{"default regex auto-detect", DefaultSearchModeDefault, "^LOG.*", []string{"LOG_LEVEL"}},
		{"fuzzy default typo tolerant", DefaultSearchModeFuzzy, "dtbrl", []string{"DATABASE_URL"}},
		{"regex default anchors", DefaultSearchModeRegex, "^API_KEY$", []string{"API_KEY"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setSearchMode(t, tt.mode)
			got := FilterKVKeys(keys, tt.query)
			if len(got) != len(tt.want) {
				t.Fatalf("FilterKVKeys(%q) = %v, want %v", tt.query, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("FilterKVKeys(%q) = %v, want %v", tt.query, got, tt.want)
				}
			}
		})
	}
}
