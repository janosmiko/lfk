package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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

func (g *kubetrisGame) finishAnimation() {
	g.animating = false
	g.animRows = nil
}

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

	for row := writeRow; row >= 0; row-- {
		g.board[row] = [boardWidth]int{}
	}

	g.lines += cleared
	g.level = g.lines/10 + 1

	return cleared
}

func (g *kubetrisGame) checkTSpin() bool {
	if g.currentPiece != 2 {
		return false
	}
	if !g.lastActionWasRotation {
		return false
	}

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

func (g *kubetrisGame) tickIntervalMs() int {
	table := []int{
		800,
		717,
		633,
		550,
		467,
		383,
		300,
		217,
		133,
		100,
		83,
		83,
		83,
		67,
		67,
		67,
		50,
		50,
		33,
		17,
	}
	idx := max(g.level-1, 0)
	if idx >= len(table) {
		return table[len(table)-1]
	}
	return table[idx]
}

func (g *kubetrisGame) tick() bool {
	if g.gameOver || g.paused {
		return false
	}
	if g.softDrop() {
		if g.lockPending && !g.isOnGround() {
			g.lockPending = false
		}
		return false
	}
	if !g.lockPending {
		g.lockPending = true
		return true
	}
	return false
}

func (g *kubetrisGame) isOnGround() bool {
	return g.collides(g.currentPiece, g.currentX, g.currentY+1, g.currentRot)
}

func (g *kubetrisGame) doLock() {
	if !g.lockPending || g.gameOver {
		return
	}
	if g.isOnGround() {
		g.lockPiece()
	}
	g.lockPending = false
}

func (g *kubetrisGame) resetLockDelay() bool {
	if g.lockPending && g.lockResets < maxLockResets {
		g.lockResets++
		return true
	}
	return false
}

func (g *kubetrisGame) calculateGhostY() int {
	ghostY := g.currentY
	for !g.collides(g.currentPiece, g.currentX, ghostY+1, g.currentRot) {
		ghostY++
	}
	return ghostY
}

func (g *kubetrisGame) isCurrent(x, y int) bool {
	shape := tetrominoes[g.currentPiece].rotations[g.currentRot]
	px := x - g.currentX
	py := y - g.currentY
	if px < 0 || px >= 4 || py < 0 || py >= 4 {
		return false
	}
	return shape[py][px]
}

func (g *kubetrisGame) isGhost(x, y, ghostY int) bool {
	shape := tetrominoes[g.currentPiece].rotations[g.currentRot]
	px := x - g.currentX
	py := y - ghostY
	if px < 0 || px >= 4 || py < 0 || py >= 4 {
		return false
	}
	return shape[py][px]
}

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

func (g *kubetrisGame) saveHighScore() {
	path := highScoreFilePath()
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, fmt.Appendf(nil, "%d\n", g.highScore), 0o644)
}
