package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestKubetrisPieceCount(t *testing.T) {
	t.Parallel()
	if len(tetrominoes) != 7 {
		t.Errorf("expected 7 tetrominoes, got %d", len(tetrominoes))
	}
}

func TestKubetrisNewGame(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()

	if g.gameOver {
		t.Error("new game should not be game over")
	}
	if g.paused {
		t.Error("new game should not be paused")
	}
	if g.score != 0 {
		t.Errorf("expected initial score 0, got %d", g.score)
	}
	if g.level != 1 {
		t.Errorf("expected initial level 1, got %d", g.level)
	}
	if g.lines != 0 {
		t.Errorf("expected initial lines 0, got %d", g.lines)
	}
	if g.holdPiece != -1 {
		t.Errorf("expected initial hold piece -1, got %d", g.holdPiece)
	}
	if g.holdUsed {
		t.Error("hold should not be used initially")
	}
	if g.currentPiece < 0 || g.currentPiece > 6 {
		t.Errorf("current piece %d out of range", g.currentPiece)
	}
	if g.nextPiece < 0 || g.nextPiece > 6 {
		t.Errorf("next piece %d out of range", g.nextPiece)
	}
}

func TestKubetrisBoardEmpty(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()

	for row := range boardHeight {
		for col := range boardWidth {
			if g.board[row][col] != 0 {
				t.Errorf("board[%d][%d] should be 0, got %d", row, col, g.board[row][col])
			}
		}
	}
}

func TestKubetrisMoveLeft(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	startX := g.currentX
	g.moveLeft()
	if g.currentX != startX-1 {
		t.Errorf("expected X=%d after moveLeft, got %d", startX-1, g.currentX)
	}
}

func TestKubetrisMoveRight(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	startX := g.currentX
	g.moveRight()
	if g.currentX != startX+1 {
		t.Errorf("expected X=%d after moveRight, got %d", startX+1, g.currentX)
	}
}

func TestKubetrisSoftDrop(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	startY := g.currentY
	ok := g.softDrop()
	if !ok {
		t.Error("softDrop should succeed from starting position")
	}
	if g.currentY != startY+1 {
		t.Errorf("expected Y=%d after softDrop, got %d", startY+1, g.currentY)
	}
}

func TestKubetrisHardDrop(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	firstPiece := g.currentPiece
	g.hardDrop()

	// After hard drop, a new piece should have spawned.
	// The first piece should now be on the board.
	foundOnBoard := false
	colorIdx := tetrominoes[firstPiece].color
	for row := range boardHeight {
		for col := range boardWidth {
			if g.board[row][col] == colorIdx {
				foundOnBoard = true
				break
			}
		}
		if foundOnBoard {
			break
		}
	}
	if !foundOnBoard {
		t.Error("after hardDrop, piece should be on the board")
	}
}

func TestKubetrisLineClear(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()

	// Fill the bottom row completely.
	for col := range boardWidth {
		g.board[boardHeight-1][col] = 1
	}

	cleared := g.clearLines()
	if cleared != 1 {
		t.Errorf("expected 1 line cleared, got %d", cleared)
	}

	// Bottom row should now be empty.
	for col := range boardWidth {
		if g.board[boardHeight-1][col] != 0 {
			t.Errorf("board[%d][%d] should be 0 after clear, got %d", boardHeight-1, col, g.board[boardHeight-1][col])
		}
	}
}

func TestKubetrisScoring(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		lines  int
		tSpin  bool
		level  int
		expect int
	}{
		{"single", 1, false, 1, 100},
		{"double", 2, false, 1, 300},
		{"triple", 3, false, 1, 500},
		{"kubetris", 4, false, 1, 800},
		{"single_level2", 1, false, 2, 200},
		{"tspin_single", 1, true, 1, 800},
		{"tspin_double", 2, true, 1, 1200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := &kubetrisGame{level: tt.level}
			g.addScore(tt.lines, tt.tSpin)
			if g.score != tt.expect {
				t.Errorf("expected score %d, got %d", tt.expect, g.score)
			}
		})
	}
}

func TestKubetrisLevelUp(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	if g.level != 1 {
		t.Fatalf("expected initial level 1, got %d", g.level)
	}

	// Simulate 10 lines cleared to trigger level 2.
	g.lines = 9
	// Fill and clear one more row.
	for col := range boardWidth {
		g.board[boardHeight-1][col] = 1
	}
	g.clearLines()

	if g.level != 2 {
		t.Errorf("expected level 2 after 10 lines, got %d", g.level)
	}
}

func TestKubetrisHoldPiece(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	originalPiece := g.currentPiece

	g.holdCurrentPiece()

	if g.holdPiece != originalPiece {
		t.Errorf("expected hold piece %d, got %d", originalPiece, g.holdPiece)
	}
	if g.holdUsed != true {
		t.Error("holdUsed should be true after hold")
	}
}

func TestKubetrisHoldOnlyOncePerDrop(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()

	g.holdCurrentPiece()
	pieceAfterFirstHold := g.currentPiece

	// Second hold should be blocked.
	g.holdCurrentPiece()
	if g.currentPiece != pieceAfterFirstHold {
		t.Error("second hold should be blocked within the same drop")
	}
}

func TestKubetrisGhostY(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	ghostY := g.calculateGhostY()
	if ghostY <= g.currentY {
		t.Errorf("ghost Y (%d) should be below current Y (%d)", ghostY, g.currentY)
	}
}

func TestKubetrisGameOver(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()

	// Fill the board from top, leaving the spawn area blocked.
	for row := range boardHeight {
		for col := range boardWidth {
			g.board[row][col] = 1
		}
	}

	// Spawning should fail because the board is full.
	ok := g.spawnPiece()
	if ok {
		t.Error("spawnPiece should fail when board is full")
	}
	if !g.gameOver {
		t.Error("gameOver should be true when spawn fails")
	}
}

func TestKubetrisTickInterval(t *testing.T) {
	t.Parallel()
	tests := []struct {
		level  int
		expect int
	}{
		{1, 800},
		{2, 717},
		{5, 467},
		{10, 100},
		{15, 67},
		{19, 33},
		{20, 17},
		{100, 17},
	}

	for _, tt := range tests {
		t.Run(
			fmt.Sprintf("level_%d", tt.level),
			func(t *testing.T) {
				t.Parallel()
				g := &kubetrisGame{level: tt.level}
				if got := g.tickIntervalMs(); got != tt.expect {
					t.Errorf("level %d: expected %dms, got %dms", tt.level, tt.expect, got)
				}
			},
		)
	}
}

func TestKubetrisCollides(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()

	// Should not collide at spawn position.
	if g.collides(g.currentPiece, g.currentX, g.currentY, g.currentRot) {
		t.Error("piece should not collide at spawn position")
	}

	// Should collide far to the left.
	if !g.collides(g.currentPiece, -10, g.currentY, g.currentRot) {
		t.Error("piece should collide when far left")
	}

	// Should collide far below.
	if !g.collides(g.currentPiece, g.currentX, boardHeight+5, g.currentRot) {
		t.Error("piece should collide when far below board")
	}
}

func TestKubetrisRotateCW(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	startRot := g.currentRot
	g.rotateCW()

	// Rotation should change (unless wall kicks fail, which shouldn't happen at spawn).
	if g.currentRot == startRot {
		t.Error("rotation should change after rotateCW")
	}
}

func TestKubetrisRotateCCW(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	startRot := g.currentRot
	g.rotateCCW()

	if g.currentRot == startRot {
		t.Error("rotation should change after rotateCCW")
	}
}

func TestKubetrisTick(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	startY := g.currentY
	g.tick()
	if g.currentY != startY+1 {
		t.Errorf("after tick, expected Y=%d, got Y=%d", startY+1, g.currentY)
	}
}

func TestKubetrisIsCurrent(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()

	// At least one cell in the piece area should be current.
	found := false
	for row := g.currentY; row < g.currentY+4; row++ {
		for col := g.currentX; col < g.currentX+4; col++ {
			if g.isCurrent(col, row) {
				found = true
			}
		}
	}
	if !found {
		t.Error("isCurrent should find at least one cell in the current piece area")
	}
}

func TestKubetrisIsGhost(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	ghostY := g.calculateGhostY()

	found := false
	for row := ghostY; row < ghostY+4; row++ {
		for col := g.currentX; col < g.currentX+4; col++ {
			if g.isGhost(col, row, ghostY) {
				found = true
			}
		}
	}
	if !found {
		t.Error("isGhost should find at least one cell in the ghost area")
	}
}

func TestKubetrisBagSystem(t *testing.T) {
	t.Parallel()
	// Use a fresh game just for the bag; draw enough pieces that we
	// know the bag refilled at least once. A fresh bag has 7 pieces.
	g := &kubetrisGame{}
	g.bag = g.newBag()

	// Draw exactly 7 pieces -- should see all 7 unique values.
	seen := make(map[int]bool)
	for range 7 {
		idx := g.drawFromBag()
		if idx < 0 || idx > 6 {
			t.Fatalf("drawn piece %d out of range", idx)
		}
		seen[idx] = true
	}

	if len(seen) != 7 {
		t.Errorf("expected 7 unique pieces in one bag, got %d", len(seen))
	}

	// After drawing 7, the bag should be empty and refill on next draw.
	if len(g.bag) != 0 {
		t.Errorf("bag should be empty after drawing 7, has %d", len(g.bag))
	}
	idx := g.drawFromBag()
	if idx < 0 || idx > 6 {
		t.Fatalf("drawn piece %d out of range after refill", idx)
	}
}

func TestKubetrisHighScoreRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	g := newKubetrisGame()
	g.highScore = 42000
	g.saveHighScore()

	g2 := newKubetrisGame()
	g2.loadHighScore()
	if g2.highScore != 42000 {
		t.Errorf("expected loaded high score 42000, got %d", g2.highScore)
	}
}

func TestKubetrisHighScoreFilePath(t *testing.T) {
	t.Run("uses XDG_STATE_HOME when set", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "/custom/state")
		got := highScoreFilePath()
		expected := filepath.Join("/custom/state", "lfk", "kubetris-highscore")
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("uses default when XDG_STATE_HOME is empty", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		got := highScoreFilePath()
		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, ".local", "state", "lfk", "kubetris-highscore")
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func TestKubetrisCheckTSpin(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()

	// Force T piece (index 2).
	g.currentPiece = 2
	g.currentRot = 0
	g.currentX = 1
	g.currentY = boardHeight - 3

	// Fill corners around the T piece center (center at 1+1=2, boardHeight-3+1=boardHeight-2).
	cx := g.currentX + 1
	cy := g.currentY + 1

	// Fill 3 of 4 corners.
	if cx-1 >= 0 && cy-1 >= 0 {
		g.board[cy-1][cx-1] = 1
	}
	if cx+1 < boardWidth && cy-1 >= 0 {
		g.board[cy-1][cx+1] = 1
	}
	if cx-1 >= 0 && cy+1 < boardHeight {
		g.board[cy+1][cx-1] = 1
	}

	// Without rotation, should not be a T-spin.
	g.lastActionWasRotation = false
	if g.checkTSpin() {
		t.Error("should not detect T-spin without rotation")
	}

	// With rotation flag, should be a T-spin.
	g.lastActionWasRotation = true
	if !g.checkTSpin() {
		t.Error("should detect T-spin with 3 filled corners and rotation")
	}
}

func TestKubetrisMultipleLineClear(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()

	// Fill bottom 4 rows.
	for row := boardHeight - 4; row < boardHeight; row++ {
		for col := range boardWidth {
			g.board[row][col] = 1
		}
	}

	cleared := g.clearLines()
	if cleared != 4 {
		t.Errorf("expected 4 lines cleared (kubetris), got %d", cleared)
	}
}

func TestKubetrisScoreAccumulation(t *testing.T) {
	t.Parallel()
	g := &kubetrisGame{level: 1}

	g.addScore(1, false) // 100
	g.addScore(2, false) // 300
	if g.score != 400 {
		t.Errorf("expected accumulated score 400, got %d", g.score)
	}
}

func TestKubetrisHighScoreTracking(t *testing.T) {
	t.Parallel()
	g := &kubetrisGame{level: 1, highScore: 50}

	g.addScore(1, false) // 100 > 50, should update high score
	if g.highScore != 100 {
		t.Errorf("expected high score 100, got %d", g.highScore)
	}
}

func TestKubetrisMoveLeftWallBlock(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	// Move far left until blocked.
	for range boardWidth {
		g.moveLeft()
	}
	posAfterWall := g.currentX
	g.moveLeft()
	if g.currentX != posAfterWall {
		t.Error("moveLeft should be blocked at the wall")
	}
}

func TestKubetrisMoveRightWallBlock(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()
	for range boardWidth {
		g.moveRight()
	}
	posAfterWall := g.currentX
	g.moveRight()
	if g.currentX != posAfterWall {
		t.Error("moveRight should be blocked at the wall")
	}
}

func TestKubetrisPieceColors(t *testing.T) {
	t.Parallel()
	for i, p := range tetrominoes {
		if p.color < 1 || p.color > 7 {
			t.Errorf("piece %d has color %d, expected 1..7", i, p.color)
		}
		c := kubetrisPieceColor(p.color)
		if c == "" {
			t.Errorf("piece %d has color %d with no color mapping", i, p.color)
		}
	}
}

func TestKubetrisRenderMiniPiece(t *testing.T) {
	t.Parallel()
	for i := range 7 {
		lines := renderMiniPiece(i)
		if len(lines) == 0 {
			t.Errorf("renderMiniPiece(%d) returned no lines", i)
		}
	}

	// Invalid piece index.
	lines := renderMiniPiece(-1)
	if len(lines) != 2 {
		t.Errorf("renderMiniPiece(-1) expected 2 fallback lines, got %d", len(lines))
	}
}

func TestKubetrisHoldSwapBack(t *testing.T) {
	t.Parallel()
	g := newKubetrisGame()

	first := g.currentPiece
	g.holdCurrentPiece() // hold first, spawn new

	second := g.currentPiece
	_ = second

	// On next drop, hold should swap back.
	g.holdUsed = false // simulate new drop
	g.holdCurrentPiece()
	if g.currentPiece != first {
		t.Errorf("expected to get back piece %d from hold, got %d", first, g.currentPiece)
	}
}

func TestKubetrisAddScoreZeroLines(t *testing.T) {
	t.Parallel()
	g := &kubetrisGame{level: 1}
	g.addScore(0, false)
	if g.score != 0 {
		t.Errorf("expected score 0 for 0 lines, got %d", g.score)
	}
}
