package app

import (
	"math/rand"
)

// Board dimensions for the Kubetris game.
const (
	boardWidth  = 10
	boardHeight = 20
)

// tetromino describes a single piece with its four rotation states and color index.
type tetromino struct {
	rotations [4][4][4]bool
	color     int // 1..7 maps to kubetrisPieceColors
}

// tetrominoBaseShapes holds only the rotation-0 grid for I, O, T, S, Z, J, L.
//
//nolint:gochecknoglobals // game constant data
var tetrominoBaseShapes = [7][4][4]bool{
	// I piece
	{
		{false, false, false, false},
		{true, true, true, true},
		{false, false, false, false},
		{false, false, false, false},
	},
	// O piece
	{
		{false, true, true, false},
		{false, true, true, false},
		{false, false, false, false},
		{false, false, false, false},
	},
	// T piece
	{
		{false, true, false, false},
		{true, true, true, false},
		{false, false, false, false},
		{false, false, false, false},
	},
	// S piece
	{
		{false, true, true, false},
		{true, true, false, false},
		{false, false, false, false},
		{false, false, false, false},
	},
	// Z piece
	{
		{true, true, false, false},
		{false, true, true, false},
		{false, false, false, false},
		{false, false, false, false},
	},
	// J piece
	{
		{true, false, false, false},
		{true, true, true, false},
		{false, false, false, false},
		{false, false, false, false},
	},
	// L piece
	{
		{false, false, true, false},
		{true, true, true, false},
		{false, false, false, false},
		{false, false, false, false},
	},
}

// tetrominoColors maps each base shape to its kubetrisPieceColors index (1..7).
//
//nolint:gochecknoglobals // game constant data
var tetrominoColors = [7]int{1, 2, 3, 4, 5, 6, 7}

// tetrominoSpinBoxes: I spins the full 4x4, O a 2x2 shifted one column right, rest a top-left 3x3.
//
//nolint:gochecknoglobals // game constant data
var tetrominoSpinBoxes = [7]struct{ col, size int }{
	{0, 4}, // I
	{1, 2}, // O
	{0, 3}, // T
	{0, 3}, // S
	{0, 3}, // Z
	{0, 3}, // J
	{0, 3}, // L
}

// tetrominoes holds all seven pieces with all four rotation states.
//
//nolint:gochecknoglobals // game constant data
var tetrominoes = buildTetrominoes()

func buildTetrominoes() [7]tetromino {
	var out [7]tetromino
	for i, base := range tetrominoBaseShapes {
		box := tetrominoSpinBoxes[i]
		out[i].color = tetrominoColors[i]
		out[i].rotations[0] = base
		for rot := 1; rot < 4; rot++ {
			out[i].rotations[rot] = spin(out[i].rotations[rot-1], box.col, box.size)
		}
	}
	return out
}

// spin rotates the piece 90 degrees clockwise within its box (rows 0..size,
// columns boxCol..boxCol+size), leaving the rest of the 4x4 grid false.
func spin(grid [4][4]bool, boxCol, size int) [4][4]bool {
	var out [4][4]bool
	for row := range size {
		for col := range size {
			out[row][boxCol+col] = grid[size-1-col][boxCol+row]
		}
	}
	return out
}

// SRS wall kick data for J, L, S, T, Z pieces.
// kicksJLSTZ[fromRot][toRot] = list of (dx, dy) offsets.
// dy is positive downward (increasing row index).
//
//nolint:gochecknoglobals // game constant data
var kicksJLSTZ = map[[2]int][][2]int{
	{0, 1}: {{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}},
	{1, 0}: {{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}},
	{1, 2}: {{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}},
	{2, 1}: {{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}},
	{2, 3}: {{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}},
	{3, 2}: {{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}},
	{3, 0}: {{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}},
	{0, 3}: {{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}},
}

// SRS wall kick data for the I piece.
//
//nolint:gochecknoglobals // game constant data
var kicksI = map[[2]int][][2]int{
	{0, 1}: {{0, 0}, {-2, 0}, {1, 0}, {-2, -1}, {1, 2}},
	{1, 0}: {{0, 0}, {2, 0}, {-1, 0}, {2, 1}, {-1, -2}},
	{1, 2}: {{0, 0}, {-1, 0}, {2, 0}, {-1, 2}, {2, -1}},
	{2, 1}: {{0, 0}, {1, 0}, {-2, 0}, {1, -2}, {-2, 1}},
	{2, 3}: {{0, 0}, {2, 0}, {-1, 0}, {2, 1}, {-1, -2}},
	{3, 2}: {{0, 0}, {-2, 0}, {1, 0}, {-2, -1}, {1, 2}},
	{3, 0}: {{0, 0}, {1, 0}, {-2, 0}, {1, -2}, {-2, 1}},
	{0, 3}: {{0, 0}, {-1, 0}, {2, 0}, {-1, 2}, {2, -1}},
}

// kubetrisGame holds all mutable state for a single Kubetris session.
type kubetrisGame struct {
	board [boardHeight][boardWidth]int // 0 = empty, 1..7 = piece color

	// Current piece state.
	currentPiece int // index into tetrominoes (0..6)
	currentX     int // column of piece origin (top-left of 4x4 grid)
	currentY     int // row of piece origin
	currentRot   int // rotation state (0..3)

	// Next piece and bag system.
	nextPiece int   // index of the next piece
	bag       []int // remaining pieces in current bag

	// Hold piece.
	holdPiece int  // -1 = no piece held, 0..6 = piece index
	holdUsed  bool // true if hold was used this drop

	// Scoring.
	score     int
	highScore int
	level     int
	lines     int

	// Game state flags.
	gameOver bool
	paused   bool

	// T-spin detection: set by rotation methods, checked by lockPiece.
	lastActionWasRotation bool

	// Lock delay: piece sits on ground; a separate 500ms timer locks it.
	// Movement/rotation resets the timer (up to maxLockResets).
	lockPending bool // true when piece is on ground and lock timer is running
	lockResets  int  // number of times lock delay was reset by movement

	// Line clear animation state.
	animating      bool   // true during line clear animation
	animTicks      int    // ticks remaining in animation
	animRows       []int  // row indices being cleared
	animIsTSpin    bool   // true if this was a T-spin clear
	lastClearLabel string // "SINGLE", "DOUBLE", "TRIPLE", "KUBETRIS!", "T-SPIN SINGLE", etc.
}

// newKubetrisGame creates a fresh game with initial state.
func newKubetrisGame() *kubetrisGame {
	g := &kubetrisGame{
		holdPiece: -1,
		level:     1,
	}
	g.bag = g.newBag()
	g.nextPiece = g.drawFromBag()
	g.spawnPiece()
	return g
}

// newBag generates a shuffled bag of all 7 piece indices.
func (g *kubetrisGame) newBag() []int {
	bag := []int{0, 1, 2, 3, 4, 5, 6}
	rand.Shuffle(len(bag), func(i, j int) {
		bag[i], bag[j] = bag[j], bag[i]
	})
	return bag
}

// drawFromBag pulls the next piece index from the bag, refilling when empty.
func (g *kubetrisGame) drawFromBag() int {
	if len(g.bag) == 0 {
		g.bag = g.newBag()
	}
	piece := g.bag[0]
	g.bag = g.bag[1:]
	return piece
}

// spawnPiece places the next piece at the top of the board.
// Returns false if the piece immediately collides (game over).
func (g *kubetrisGame) spawnPiece() bool {
	g.currentPiece = g.nextPiece
	g.nextPiece = g.drawFromBag()
	g.currentX = 3
	g.currentY = 0
	g.currentRot = 0
	g.holdUsed = false
	g.lastActionWasRotation = false
	g.lockPending = false
	g.lockResets = 0

	if g.collides(g.currentPiece, g.currentX, g.currentY, g.currentRot) {
		g.gameOver = true
		return false
	}
	return true
}

// collides checks whether placing the given piece at (x, y) with
// the given rotation would overlap an occupied cell or go out of bounds.
func (g *kubetrisGame) collides(pieceIdx, x, y, rot int) bool {
	shape := tetrominoes[pieceIdx].rotations[rot]
	for row := range 4 {
		for col := range 4 {
			if !shape[row][col] {
				continue
			}
			bx := x + col
			by := y + row
			if bx < 0 || bx >= boardWidth || by < 0 || by >= boardHeight {
				return true
			}
			if g.board[by][bx] != 0 {
				return true
			}
		}
	}
	return false
}

// moveLeft shifts the current piece one cell to the left if possible.
func (g *kubetrisGame) moveLeft() bool {
	if !g.collides(g.currentPiece, g.currentX-1, g.currentY, g.currentRot) {
		g.currentX--
		g.lastActionWasRotation = false
		return g.resetLockDelay()
	}
	return false
}

// moveRight shifts the current piece one cell to the right if possible.
func (g *kubetrisGame) moveRight() bool {
	if !g.collides(g.currentPiece, g.currentX+1, g.currentY, g.currentRot) {
		g.currentX++
		g.lastActionWasRotation = false
		return g.resetLockDelay()
	}
	return false
}

// softDrop moves the current piece one cell downward. Returns true if it moved.
func (g *kubetrisGame) softDrop() bool {
	if !g.collides(g.currentPiece, g.currentX, g.currentY+1, g.currentRot) {
		g.currentY++
		g.lastActionWasRotation = false
		return true
	}
	return false
}

// hardDrop instantly drops the piece to its landing position and locks it.
func (g *kubetrisGame) hardDrop() {
	ghostY := g.calculateGhostY()
	g.currentY = ghostY
	g.lastActionWasRotation = false
	g.lockPiece()
}

// rotateCW rotates the current piece clockwise using SRS wall kicks.
func (g *kubetrisGame) rotateCW() bool {
	newRot := (g.currentRot + 1) % 4
	return g.tryRotation(g.currentRot, newRot)
}

// rotateCCW rotates the current piece counter-clockwise using SRS wall kicks.
func (g *kubetrisGame) rotateCCW() bool {
	newRot := (g.currentRot + 3) % 4
	return g.tryRotation(g.currentRot, newRot)
}

// tryRotation attempts a rotation from oldRot to newRot using wall kick data.
// Returns true if the lock timer should be rescheduled.
func (g *kubetrisGame) tryRotation(oldRot, newRot int) bool {
	kicks := kicksJLSTZ
	if g.currentPiece == 0 { // I piece
		kicks = kicksI
	}

	offsets, ok := kicks[[2]int{oldRot, newRot}]
	if !ok {
		return false
	}

	for _, off := range offsets {
		nx := g.currentX + off[0]
		ny := g.currentY - off[1] // SRS uses y-up; our board is y-down
		if !g.collides(g.currentPiece, nx, ny, newRot) {
			g.currentX = nx
			g.currentY = ny
			g.currentRot = newRot
			g.lastActionWasRotation = true
			return g.resetLockDelay()
		}
	}
	return false
}

// holdCurrentPiece swaps the current piece with the held piece.
// Can only be used once per drop.
func (g *kubetrisGame) holdCurrentPiece() {
	if g.holdUsed {
		return
	}

	if g.holdPiece < 0 {
		g.holdPiece = g.currentPiece
		g.spawnPiece()
		// spawnPiece resets holdUsed; re-set it after spawn.
		g.holdUsed = true
	} else {
		g.holdPiece, g.currentPiece = g.currentPiece, g.holdPiece
		g.currentX = 3
		g.currentY = 0
		g.currentRot = 0
		g.holdUsed = true
		g.lastActionWasRotation = false
		if g.collides(g.currentPiece, g.currentX, g.currentY, g.currentRot) {
			g.gameOver = true
		}
	}
}

// lockPiece writes the current piece to the board, checks for T-spin,
// clears lines, updates the score, and spawns the next piece.
func (g *kubetrisGame) lockPiece() {
	shape := tetrominoes[g.currentPiece].rotations[g.currentRot]
	colorIdx := tetrominoes[g.currentPiece].color

	for row := range 4 {
		for col := range 4 {
			if !shape[row][col] {
				continue
			}
			bx := g.currentX + col
			by := g.currentY + row
			if bx >= 0 && bx < boardWidth && by >= 0 && by < boardHeight {
				g.board[by][bx] = colorIdx
			}
		}
	}

	tSpin := g.checkTSpin()

	// Find full rows for animation.
	var fullRows []int
	for row := range boardHeight {
		full := true
		for col := range boardWidth {
			if g.board[row][col] == 0 {
				full = false
				break
			}
		}
		if full {
			fullRows = append(fullRows, row)
		}
	}

	if len(fullRows) > 0 {
		// Score and clear immediately -- game continues without pause.
		g.addScore(len(fullRows), tSpin)
		g.lastClearLabel = clearLabel(len(fullRows), tSpin)
		// Start visual-only animation (doesn't block gameplay).
		g.animating = true
		g.animTicks = 4
		g.animRows = fullRows
		g.animIsTSpin = tSpin
		g.clearLines()
	} else {
		g.lastClearLabel = ""
	}
	g.spawnPiece()
}

// maxLockResets is the max number of times movement can reset the lock delay.
const maxLockResets = 15
