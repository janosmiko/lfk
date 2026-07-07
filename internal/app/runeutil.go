package app

// trimLastRune drops the final rune (multibyte-safe backspace for
// hand-rolled text inputs).
func trimLastRune(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return string(r[:len(r)-1])
}
