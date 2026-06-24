// Package app — easter_egg_state.go
// Embedded sub-struct of Model holding the hidden easter-egg feature state
// (Konami code, nyan mode, credits, kubetris). Kept in a dedicated file to
// keep app.go under the 800-line cap while preserving direct field access
// (m.nyanMode etc.) via Go's embedded-struct field promotion.
package app

// easterEggState bundles the Model fields driving the easter eggs.
type easterEggState struct {
	konamiProgress int  // current position in the Konami Code sequence
	konamiActive   bool // true when cheat code was just activated (clears after 5s)
	nyanMode       bool // toggleable nyan mode indicator
	nyanTick       int  // animation tick for nyan mode
	creditsScroll  int  // scroll position for credits screen
	creditsStopped bool // true when credits reached center and waiting to close
	kubetrisGame   *kubetrisGame
}
