package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseColorschemesEmbedded(t *testing.T) {
	schemes, light := parseColorschemes()
	assert.Len(t, schemes, 462)
	assert.NotEmpty(t, light)
	for name, l := range light {
		assert.True(t, l, "light map entry %s should be true", name)
	}
}

func TestParseColorschemesTSV(t *testing.T) {
	t.Run("valid row", func(t *testing.T) {
		schemes, light := parseColorschemesTSV("dracula\ttrue\t#1\t#2\t#3\t#4\t#5\t#6\t#7\t#8\t#9\t#10\t#11\t#12\t#13\n")
		assert.Equal(t, Theme{
			Primary: "#1", Secondary: "#2", Text: "#3", SelectedFg: "#4", SelectedBg: "#5",
			Border: "#6", Dimmed: "#7", Error: "#8", Warning: "#9", Purple: "#10",
			Base: "#11", BarBg: "#12", Surface: "#13",
		}, schemes["dracula"])
		assert.True(t, light["dracula"])
	})

	t.Run("blank lines are skipped", func(t *testing.T) {
		schemes, _ := parseColorschemesTSV("\n\n")
		assert.Empty(t, schemes)
	})

	t.Run("wrong column count panics", func(t *testing.T) {
		assert.Panics(t, func() {
			parseColorschemesTSV("dracula\ttrue\t#1\n")
		})
	})

	t.Run("invalid isLight panics", func(t *testing.T) {
		assert.Panics(t, func() {
			parseColorschemesTSV("dracula\tnotabool\t#1\t#2\t#3\t#4\t#5\t#6\t#7\t#8\t#9\t#10\t#11\t#12\t#13\n")
		})
	})
}
