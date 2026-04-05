package app

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// tetrominoes holds the seven standard pieces (I, O, T, S, Z, J, L) with SRS rotation data.
// Each rotation state is stored in a 4x4 grid (row-major: [row][col]).
//
//nolint:gochecknoglobals // game constant data
var tetrominoes = [7]tetromino{
	// I piece (color 1 = cyan)
	{
		rotations: [4][4][4]bool{
			// Rotation 0
			{
				{false, false, false, false},
				{true, true, true, true},
				{false, false, false, false},
				{false, false, false, false},
			},
			// Rotation 1
			{
				{false, false, true, false},
				{false, false, true, false},
				{false, false, true, false},
				{false, false, true, false},
			},
			// Rotation 2
			{
				{false, false, false, false},
				{false, false, false, false},
				{true, true, true, true},
				{false, false, false, false},
			},
			// Rotation 3
			{
				{false, true, false, false},
				{false, true, false, false},
				{false, true, false, false},
				{false, true, false, false},
			},
		},
		color: 1,
	},
	// O piece (color 2 = yellow)
	{
		rotations: [4][4][4]bool{
			{
				{false, true, true, false},
				{false, true, true, false},
				{false, false, false, false},
				{false, false, false, false},
			},
			{
				{false, true, true, false},
				{false, true, true, false},
				{false, false, false, false},
				{false, false, false, false},
			},
			{
				{false, true, true, false},
				{false, true, true, false},
				{false, false, false, false},
				{false, false, false, false},
			},
			{
				{false, true, true, false},
				{false, true, true, false},
				{false, false, false, false},
				{false, false, false, false},
			},
		},
		color: 2,
	},
	// T piece (color 3 = purple)
	{
		rotations: [4][4][4]bool{
			{
				{false, true, false, false},
				{true, true, true, false},
				{false, false, false, false},
				{false, false, false, false},
			},
			{
				{false, true, false, false},
				{false, true, true, false},
				{false, true, false, false},
				{false, false, false, false},
			},
			{
				{false, false, false, false},
				{true, true, true, false},
				{false, true, false, false},
				{false, false, false, false},
			},
			{
				{false, true, false, false},
				{true, true, false, false},
				{false, true, false, false},
				{false, false, false, false},
			},
		},
		color: 3,
	},
	// S piece (color 4 = green)
	{
		rotations: [4][4][4]bool{
			{
				{false, true, true, false},
				{true, true, false, false},
				{false, false, false, false},
				{false, false, false, false},
			},
			{
				{false, true, false, false},
				{false, true, true, false},
				{false, false, true, false},
				{false, false, false, false},
			},
			{
				{false, false, false, false},
				{false, true, true, false},
				{true, true, false, false},
				{false, false, false, false},
			},
			{
				{true, false, false, false},
				{true, true, false, false},
				{false, true, false, false},
				{false, false, false, false},
			},
		},
		color: 4,
	},
	// Z piece (color 5 = red)
	{
		rotations: [4][4][4]bool{
			{
				{true, true, false, false},
				{false, true, true, false},
				{false, false, false, false},
				{false, false, false, false},
			},
			{
				{false, false, true, false},
				{false, true, true, false},
				{false, true, false, false},
				{false, false, false, false},
			},
			{
				{false, false, false, false},
				{true, true, false, false},
				{false, true, true, false},
				{false, false, false, false},
			},
			{
				{false, true, false, false},
				{true, true, false, false},
				{true, false, false, false},
				{false, false, false, false},
			},
		},
		color: 5,
	},
	// J piece (color 6 = blue)
	{
		rotations: [4][4][4]bool{
			{
				{true, false, false, false},
				{true, true, true, false},
				{false, false, false, false},
				{false, false, false, false},
			},
			{
				{false, true, true, false},
				{false, true, false, false},
				{false, true, false, false},
				{false, false, false, false},
			},
			{
				{false, false, false, false},
				{true, true, true, false},
				{false, false, true, false},
				{false, false, false, false},
			},
			{
				{false, true, false, false},
				{false, true, false, false},
				{true, true, false, false},
				{false, false, false, false},
			},
		},
		color: 6,
	},
	// L piece (color 7 = orange)
	{
		rotations: [4][4][4]bool{
			{
				{false, false, true, false},
				{true, true, true, false},
				{false, false, false, false},
				{false, false, false, false},
			},
			{
				{false, true, false, false},
				{false, true, false, false},
				{false, true, true, false},
				{false, false, false, false},
			},
			{
				{false, false, false, false},
				{true, true, true, false},
				{true, false, false, false},
				{false, false, false, false},
			},
			{
				{true, true, false, false},
				{false, true, false, false},
				{false, true, false, false},
				{false, false, false, false},
			},
		},
		color: 7,
	},
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

// clearLabel returns the display label for a line clear.
func clearLabel(lines int, tSpin bool) string {
	if tSpin {
		switch lines {
		case 1:
			return "T-SPIN SINGLE!"
		case 2:
			return "T-SPIN DOUBLE!"
		case 3:
			return "T-SPIN TRIPLE!"
		default:
			return "T-SPIN!"
		}
	}
	switch lines {
	case 1:
		return "SINGLE"
	case 2:
		return "DOUBLE"
	case 3:
		return "TRIPLE"
	case 4:
		return "KUBETRIS!"
	default:
		return ""
	}
}

// finishAnimation ends the visual animation effect.
func (g *kubetrisGame) finishAnimation() {
	g.animating = false
	g.animRows = nil
}

// clearLines removes all complete rows and returns how many were cleared.
func (g *kubetrisGame) clearLines() int {
	cleared := 0
	writeRow := boardHeight - 1

	for readRow := boardHeight - 1; readRow >= 0; readRow-- {
		full := true
		for col := range boardWidth {
			if g.board[readRow][col] == 0 {
				full = false
				break
			}
		}
		if full {
			cleared++
			continue
		}
		if writeRow != readRow {
			g.board[writeRow] = g.board[readRow]
		}
		writeRow--
	}

	// Fill remaining top rows with empty cells.
	for row := writeRow; row >= 0; row-- {
		g.board[row] = [boardWidth]int{}
	}

	g.lines += cleared
	// Level up every 10 lines.
	g.level = g.lines/10 + 1

	return cleared
}

// checkTSpin detects a T-spin: the last move was a rotation of the T piece,
// and at least 3 of the 4 corners around the T piece center are occupied or out of bounds.
func (g *kubetrisGame) checkTSpin() bool {
	if g.currentPiece != 2 { // T piece is index 2
		return false
	}
	if !g.lastActionWasRotation {
		return false
	}

	// Center of the T piece in the 3x3 area is at offset (1, 1) from origin.
	cx := g.currentX + 1
	cy := g.currentY + 1

	corners := [4][2]int{
		{cx - 1, cy - 1},
		{cx + 1, cy - 1},
		{cx - 1, cy + 1},
		{cx + 1, cy + 1},
	}

	filled := 0
	for _, c := range corners {
		if c[0] < 0 || c[0] >= boardWidth || c[1] < 0 || c[1] >= boardHeight {
			filled++
			continue
		}
		if g.board[c[1]][c[0]] != 0 {
			filled++
		}
	}

	return filled >= 3
}

// addScore awards points based on lines cleared, T-spin status, and current level.
func (g *kubetrisGame) addScore(linesCleared int, tSpin bool) {
	if linesCleared == 0 {
		return
	}

	var points int
	if tSpin {
		switch linesCleared {
		case 1:
			points = 800 * g.level
		case 2:
			points = 1200 * g.level
		default:
			points = 1200 * g.level
		}
	} else {
		switch linesCleared {
		case 1:
			points = 100 * g.level
		case 2:
			points = 300 * g.level
		case 3:
			points = 500 * g.level
		case 4:
			points = 800 * g.level
		default:
			points = 800 * g.level
		}
	}

	g.score += points
	if g.score > g.highScore {
		g.highScore = g.score
	}
}

// tickIntervalMs returns the drop interval in milliseconds for the current level.
// Uses the official NES-style gravity curve for natural difficulty progression.
func (g *kubetrisGame) tickIntervalMs() int {
	// NES-style gravity table (approximate ms at 60fps).
	table := []int{
		800, // level 1
		717, // level 2
		633, // level 3
		550, // level 4
		467, // level 5
		383, // level 6
		300, // level 7
		217, // level 8
		133, // level 9
		100, // level 10
		83,  // level 11
		83,  // level 12
		83,  // level 13
		67,  // level 14
		67,  // level 15
		67,  // level 16
		50,  // level 17
		50,  // level 18
		33,  // level 19
		17,  // level 20+
	}
	idx := g.level - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(table) {
		return table[len(table)-1]
	}
	return table[idx]
}

// maxLockResets is the max number of times movement can reset the lock delay.
const maxLockResets = 15

// tick advances the game by one step: moves the piece down or locks it.
// tick advances the game by one gravity step.
// Returns true if the piece just landed and a lock timer should be started.
func (g *kubetrisGame) tick() bool {
	if g.gameOver || g.paused {
		return false
	}
	if g.softDrop() {
		// Piece moved down -- if it was pending lock, cancel (it's no longer on ground).
		if g.lockPending && !g.isOnGround() {
			g.lockPending = false
		}
		return false
	}
	// Piece can't move down. If lock timer isn't already running, start it.
	if !g.lockPending {
		g.lockPending = true
		return true // signal caller to schedule lock timer
	}
	return false
}

// isOnGround returns true if the piece can't move down.
func (g *kubetrisGame) isOnGround() bool {
	return g.collides(g.currentPiece, g.currentX, g.currentY+1, g.currentRot)
}

// doLock is called when the lock timer expires. Locks the piece if still on ground.
func (g *kubetrisGame) doLock() {
	if !g.lockPending || g.gameOver {
		return
	}
	// Only lock if still on the ground.
	if g.isOnGround() {
		g.lockPiece()
	}
	g.lockPending = false
}

// resetLockDelay resets the lock delay (called on successful move/rotation while on ground).
// Returns true if the lock timer should be rescheduled.
func (g *kubetrisGame) resetLockDelay() bool {
	if g.lockPending && g.lockResets < maxLockResets {
		g.lockResets++
		return true // caller should reschedule the lock timer
	}
	return false
}

// calculateGhostY returns the Y position where the current piece would land.
func (g *kubetrisGame) calculateGhostY() int {
	ghostY := g.currentY
	for !g.collides(g.currentPiece, g.currentX, ghostY+1, g.currentRot) {
		ghostY++
	}
	return ghostY
}

// isCurrent reports whether the board cell at (x, y) is part of the current falling piece.
func (g *kubetrisGame) isCurrent(x, y int) bool {
	shape := tetrominoes[g.currentPiece].rotations[g.currentRot]
	px := x - g.currentX
	py := y - g.currentY
	if px < 0 || px >= 4 || py < 0 || py >= 4 {
		return false
	}
	return shape[py][px]
}

// isGhost reports whether the board cell at (x, y) is part of the ghost piece preview.
func (g *kubetrisGame) isGhost(x, y, ghostY int) bool {
	shape := tetrominoes[g.currentPiece].rotations[g.currentRot]
	px := x - g.currentX
	py := y - ghostY
	if px < 0 || px >= 4 || py < 0 || py >= 4 {
		return false
	}
	return shape[py][px]
}

// highScoreFilePath returns the path for persisting the high score.
// Uses $XDG_STATE_HOME/lfk/kubetris-highscore (defaults to ~/.local/state/lfk/kubetris-highscore).
func highScoreFilePath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "lfk", "kubetris-highscore")
}

// loadHighScore reads the persisted high score from disk.
func (g *kubetrisGame) loadHighScore() {
	path := highScoreFilePath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	score, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return
	}
	g.highScore = score
}

// saveHighScore writes the current high score to disk.
func (g *kubetrisGame) saveHighScore() {
	path := highScoreFilePath()
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d\n", g.highScore)), 0o644)
}
