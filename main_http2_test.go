package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/ui"
)

func TestApplyHTTP2Preference(t *testing.T) {
	tests := []struct {
		name     string
		disable  bool
		env      string
		expected string
	}{
		{name: "off by default", disable: false, env: "", expected: ""},
		{name: "config sets the env var", disable: true, env: "", expected: "1"},
		{name: "an env value already set wins", disable: true, env: "0", expected: "0"},
		{name: "config false leaves the env alone", disable: false, env: "1", expected: "1"},
	}

	orig := ui.ConfigDisableHTTP2
	t.Cleanup(func() { ui.ConfigDisableHTTP2 = orig })

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DISABLE_HTTP2", tc.env)
			if tc.env == "" {
				// t.Setenv cannot unset, and apimachinery keys off emptiness.
				assert.NoError(t, os.Unsetenv("DISABLE_HTTP2"))
			}
			ui.ConfigDisableHTTP2 = tc.disable

			applyHTTP2Preference()

			assert.Equal(t, tc.expected, os.Getenv("DISABLE_HTTP2"))
		})
	}
}
