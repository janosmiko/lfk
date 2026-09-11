package ui

import (
	"os"
	"testing"
)

// A NO_COLOR set in the invoking shell is otherwise inherited by the whole
// binary: LoadConfig (config_apply.go) forces ConfigNoColor on whenever it
// sees the var, and that global then stays on for every later test.
func TestMain(m *testing.M) {
	_ = os.Unsetenv("NO_COLOR")
	os.Exit(m.Run())
}
