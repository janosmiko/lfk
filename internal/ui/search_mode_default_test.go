package ui

import "testing"

// setSearchMode sets ConfigDefaultSearchMode for the duration of the test and
// restores the prior value on cleanup.
func setSearchMode(t *testing.T, mode string) {
	t.Helper()
	orig := ConfigDefaultSearchMode
	ConfigDefaultSearchMode = mode
	t.Cleanup(func() { ConfigDefaultSearchMode = orig })
}

func TestDetectSearchMode_FuzzyDefault(t *testing.T) {
	setSearchMode(t, DefaultSearchModeFuzzy)

	tests := []struct {
		name      string
		rawQuery  string
		wantMode  SearchMode
		wantQuery string
	}{
		{"plain becomes fuzzy", "deplymnt", SearchFuzzy, "deplymnt"},
		{"plain with regex metacharacters still fuzzy", "err.r", SearchFuzzy, "err.r"},
		{"fuzzy prefix stays fuzzy", "~deplymnt", SearchFuzzy, "deplymnt"},
		{"literal escape forces substring", `\err.r`, SearchSubstring, "err.r"},
		{"empty", "", SearchSubstring, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, query := DetectSearchMode(tt.rawQuery)
			if mode != tt.wantMode {
				t.Errorf("mode = %d, want %d", mode, tt.wantMode)
			}
			if query != tt.wantQuery {
				t.Errorf("query = %q, want %q", query, tt.wantQuery)
			}
		})
	}
}

func TestDetectSearchMode_RegexDefault(t *testing.T) {
	setSearchMode(t, DefaultSearchModeRegex)

	tests := []struct {
		name      string
		rawQuery  string
		wantMode  SearchMode
		wantQuery string
	}{
		{"plain becomes regex", "deployment", SearchRegex, "deployment"},
		{"plain with metacharacters still regex", "err.r", SearchRegex, "err.r"},
		{"fuzzy prefix still fuzzy", "~deplymnt", SearchFuzzy, "deplymnt"},
		{"literal escape forces substring", `\err.r`, SearchSubstring, "err.r"},
		{"empty", "", SearchSubstring, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, query := DetectSearchMode(tt.rawQuery)
			if mode != tt.wantMode {
				t.Errorf("mode = %d, want %d", mode, tt.wantMode)
			}
			if query != tt.wantQuery {
				t.Errorf("query = %q, want %q", query, tt.wantQuery)
			}
		})
	}
}

func TestDetectSearchMode_LiteralDefault(t *testing.T) {
	setSearchMode(t, DefaultSearchModeLiteral)

	tests := []struct {
		name      string
		rawQuery  string
		wantMode  SearchMode
		wantQuery string
	}{
		{"plain stays substring", "deployment", SearchSubstring, "deployment"},
		{"metacharacters stay literal", "err.r", SearchSubstring, "err.r"},
		{"brackets stay literal", "[0]", SearchSubstring, "[0]"},
		{"fuzzy prefix still fuzzy", "~deplymnt", SearchFuzzy, "deplymnt"},
		{"literal escape strips the prefix", `\err.r`, SearchSubstring, "err.r"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, query := DetectSearchMode(tt.rawQuery)
			if mode != tt.wantMode {
				t.Errorf("mode = %d, want %d", mode, tt.wantMode)
			}
			if query != tt.wantQuery {
				t.Errorf("query = %q, want %q", query, tt.wantQuery)
			}
		})
	}
}

func TestDetectSearchMode_AutoDefault(t *testing.T) {
	setSearchMode(t, DefaultSearchModeAuto)

	mode, query := DetectSearchMode("err.r")
	if mode != SearchRegex || query != "err.r" {
		t.Errorf("got (%d, %q), want (%d, %q)", mode, query, SearchRegex, "err.r")
	}
	mode, query = DetectSearchMode("deployment")
	if mode != SearchSubstring || query != "deployment" {
		t.Errorf("got (%d, %q), want (%d, %q)", mode, query, SearchSubstring, "deployment")
	}
}

func TestSearchModePromptLabel(t *testing.T) {
	setSearchMode(t, DefaultSearchModeAuto)

	if got := SearchModePromptLabel("~asd", "/ "); got != "[fuzzy] " {
		t.Errorf("got %q, want %q", got, "[fuzzy] ")
	}
	if got := SearchModePromptLabel("err.r", "/ "); got != "[regex] " {
		t.Errorf("got %q, want %q", got, "[regex] ")
	}
	if got := SearchModePromptLabel("plain", "/ "); got != "/ " {
		t.Errorf("got %q, want %q", got, "/ ")
	}
	if got := SearchModePromptLabel("", "/ "); got != "/ " {
		t.Errorf("got %q, want %q", got, "/ ")
	}
}

func TestSearchModeHintEntry(t *testing.T) {
	setSearchMode(t, DefaultSearchModeAuto)
	entry := SearchModeHintEntry()
	if entry.Key != "~" || entry.Desc != "fuzzy" {
		t.Errorf("auto default: got %+v, want {~ fuzzy}", entry)
	}

	setSearchMode(t, DefaultSearchModeLiteral)
	entry = SearchModeHintEntry()
	if entry.Key != "~" || entry.Desc != "fuzzy" {
		t.Errorf("literal default: got %+v, want {~ fuzzy}", entry)
	}

	setSearchMode(t, DefaultSearchModeRegex)
	entry = SearchModeHintEntry()
	if entry.Key != "~" || entry.Desc != "fuzzy" {
		t.Errorf("regex default: got %+v, want {~ fuzzy}", entry)
	}

	setSearchMode(t, DefaultSearchModeFuzzy)
	entry = SearchModeHintEntry()
	if entry.Key != `\` || entry.Desc != "literal" {
		t.Errorf("fuzzy default: got %+v, want {\\ literal}", entry)
	}
}
