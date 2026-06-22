package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDarken(t *testing.T) {
	t.Run("amount 0 returns the color unchanged", func(t *testing.T) {
		assert.Equal(t, "#9ece6a", Darken("#9ece6a", 0))
	})

	t.Run("amount 1 returns black", func(t *testing.T) {
		assert.Equal(t, "#000000", Darken("#9ece6a", 1))
	})

	t.Run("darkening lowers luminance but keeps hue", func(t *testing.T) {
		const src = "#9ece6a" // theme green
		out := Darken(src, 0.5)
		assert.NotEqual(t, src, out)

		sr, sg, sb, ok := parseHexColor(src)
		assert.True(t, ok)
		or, og, ob, ok := parseHexColor(out)
		assert.True(t, ok)
		assert.Less(t, relativeLuminance(or, og, ob), relativeLuminance(sr, sg, sb),
			"darkened color must be less luminous")

		sh, _, _ := rgbToHSL(sr, sg, sb)
		oh, _, _ := rgbToHSL(or, og, ob)
		assert.InDelta(t, sh, oh, 0.01, "hue is preserved")
	})

	t.Run("unparseable color is returned unchanged", func(t *testing.T) {
		assert.Equal(t, "rebeccapurple", Darken("rebeccapurple", 0.5))
	})
}
