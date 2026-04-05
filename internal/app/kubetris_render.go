package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/janosmiko/lfk/internal/ui"
)

// kubetrisTickMsg signals a periodic game tick.
type kubetrisTickMsg struct{}

// kubetrisAnimTickMsg signals a fast tick during line clear animation.
type kubetrisAnimTickMsg struct{}

// kubetrisLockTickMsg signals the lock delay has expired.
type kubetrisLockTickMsg struct{}

// kubetrisPieceColor returns the color for a piece index using theme colors.
func kubetrisPieceColor(idx int) string {
	switch idx {
	case 1:
		return ui.ColorPrimary // I - theme primary (blue)
	case 2:
		return ui.ColorWarning // O - theme warning (yellow/orange)
	case 3:
		return ui.ColorPurple // T - theme purple
	case 4:
		return ui.ColorSecondary // S - theme secondary (green)
	case 5:
		return ui.ColorError // Z - theme error (red)
	case 6:
		return ui.ColorSelectedBg // J - theme selected bg (blue variant)
	case 7:
		return ui.ColorDimmed // L - theme dimmed
	default:
		return ui.ColorFile
	}
}

// scheduleKubetrisTick returns a tea.Cmd that fires a kubetrisTickMsg
// after the interval determined by the current level.
// scheduleKubetrisAnimTick returns a fast tick for animation (100ms).
func scheduleKubetrisAnimTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(_ time.Time) tea.Msg {
		return kubetrisAnimTickMsg{}
	})
}

// scheduleKubetrisLockDelay schedules the lock after 500ms.
func scheduleKubetrisLockDelay() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
		return kubetrisLockTickMsg{}
	})
}

func (m Model) scheduleKubetrisTick() tea.Cmd {
	if m.kubetrisGame == nil {
		return nil
	}
	ms := m.kubetrisGame.tickIntervalMs()
	return tea.Tick(time.Duration(ms)*time.Millisecond, func(_ time.Time) tea.Msg {
		return kubetrisTickMsg{}
	})
}

// viewKubetris renders the complete Kubetris game screen, centered in the terminal.
func (m Model) viewKubetris() string {
	g := m.kubetrisGame
	if g == nil {
		return "No game"
	}

	ghostY := g.calculateGhostY()

	// Build the board rows.
	boardLines := make([]string, 0, boardHeight+2)
	boardLines = append(boardLines, renderBoardTopBorder())

	// Build set of animated rows for quick lookup.
	animSet := make(map[int]bool)
	for _, r := range g.animRows {
		animSet[r] = true
	}

	for row := range boardHeight {
		var line strings.Builder
		line.WriteString("\u2502") // left border

		// Animated row: flash effect.
		if g.animating && animSet[row] {
			flashStyle := lipgloss.NewStyle().Bold(true)
			if g.animIsTSpin {
				// T-spin: purple/gold alternating flash.
				if g.animTicks%2 == 0 {
					flashStyle = flashStyle.Foreground(lipgloss.Color(ui.ColorPurple)).Background(lipgloss.Color(ui.ColorWarning))
				} else {
					flashStyle = flashStyle.Foreground(lipgloss.Color(ui.ColorWarning)).Background(lipgloss.Color(ui.ColorPurple))
				}
			} else {
				// Normal clear: white/bright flash.
				if g.animTicks%2 == 0 {
					flashStyle = flashStyle.Foreground(lipgloss.Color(ui.ColorSelectedFg)).Background(lipgloss.Color(ui.ColorSelectedBg))
				} else {
					flashStyle = flashStyle.Foreground(lipgloss.Color(ui.ColorSelectedBg)).Background(lipgloss.Color(ui.ColorSelectedFg))
				}
			}
			line.WriteString(flashStyle.Render(strings.Repeat("\u2588\u2588\u2588", boardWidth)))
			line.WriteString("\u2502")
			boardLines = append(boardLines, line.String())
			continue
		}

		for col := range boardWidth {
			cell := g.board[row][col]
			switch {
			case g.isCurrent(col, row):
				colorIdx := tetrominoes[g.currentPiece].color
				style := lipgloss.NewStyle().Foreground(lipgloss.Color(kubetrisPieceColor(colorIdx)))
				line.WriteString(style.Render("\u2588\u2588\u2588"))
			case g.isGhost(col, row, ghostY) && ghostY != g.currentY:
				colorIdx := tetrominoes[g.currentPiece].color
				style := lipgloss.NewStyle().Foreground(lipgloss.Color(kubetrisPieceColor(colorIdx)))
				line.WriteString(style.Render("\u2591\u2591\u2591"))
			case cell > 0:
				style := lipgloss.NewStyle().Foreground(lipgloss.Color(kubetrisPieceColor(cell)))
				line.WriteString(style.Render("\u2588\u2588\u2588"))
			default:
				line.WriteString("   ")
			}
		}
		line.WriteString("\u2502") // right border
		boardLines = append(boardLines, line.String())
	}

	boardLines = append(boardLines, renderBoardBottomBorder())

	// Build left panel (hold piece).
	leftPanel := m.renderSidePanel("HOLD", g.holdPiece)

	// Build right panel (next piece + stats).
	rightPanel := m.renderRightPanel()

	// Combine panels: left | board | right.
	boardStr := strings.Join(boardLines, "\n")
	leftStr := strings.Join(leftPanel, "\n")
	rightStr := strings.Join(rightPanel, "\n")

	game := lipgloss.JoinHorizontal(lipgloss.Top, leftStr, " ", boardStr, " ", rightStr)

	// Add controls hint below.
	controls := renderControlsHint()
	game = lipgloss.JoinVertical(lipgloss.Center, game, "", controls)

	// Overlay game-over or pause screen.
	if g.gameOver {
		game = overlayGameOver(game, g)
	} else if g.paused {
		return overlayPaused(m.width, m.height)
	}

	// Center in terminal.
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, game)
}

// renderBoardTopBorder returns the top border of the game board.
func renderBoardTopBorder() string {
	return "\u250c" + strings.Repeat("\u2500", boardWidth*3) + "\u2510"
}

// renderBoardBottomBorder returns the bottom border of the game board.
func renderBoardBottomBorder() string {
	return "\u2514" + strings.Repeat("\u2500", boardWidth*3) + "\u2518"
}

// renderSidePanel builds a small side panel showing a label and a mini piece.
// panelWidth is the fixed width of side panels (must fit "T-SPIN DOUBLE!" = 14 chars).
const panelWidth = 15

func (m Model) renderSidePanel(label string, pieceIdx int) []string {
	lines := make([]string, 0, boardHeight+2)
	labelStyle := lipgloss.NewStyle().Bold(true)
	pad := strings.Repeat(" ", panelWidth)

	// Pad to align vertically with the board.
	lines = append(lines, pad)
	lines = append(lines, labelStyle.Render(label)+strings.Repeat(" ", max(0, panelWidth-lipgloss.Width(label))))
	lines = append(lines, pad)

	if pieceIdx >= 0 && pieceIdx < len(tetrominoes) {
		mini := renderMiniPiece(pieceIdx)
		for _, ml := range mini {
			w := lipgloss.Width(ml)
			lines = append(lines, ml+strings.Repeat(" ", max(0, panelWidth-w)))
		}
	} else {
		lines = append(lines, pad)
		lines = append(lines, pad)
	}

	// Pad remaining lines so the panel is tall enough.
	for len(lines) < boardHeight+2 {
		lines = append(lines, pad)
	}
	return lines
}

// renderRightPanel builds the right side panel with next piece and stats.
func (m Model) renderRightPanel() []string {
	g := m.kubetrisGame
	lines := make([]string, 0, boardHeight+2)
	labelStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Faint(true)
	pad := strings.Repeat(" ", panelWidth)

	// Next piece.
	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("NEXT"))
	lines = append(lines, "")
	mini := renderMiniPiece(g.nextPiece)
	lines = append(lines, mini...)

	lines = append(lines, "")

	// Stats.
	lines = append(lines, labelStyle.Render("SCORE"))
	lines = append(lines, fmt.Sprintf(" %d", g.score))
	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("LEVEL"))
	lines = append(lines, fmt.Sprintf(" %d", g.level))
	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("LINES"))
	lines = append(lines, fmt.Sprintf(" %d", g.lines))
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("HIGH"))
	lines = append(lines, fmt.Sprintf(" %d", g.highScore))

	// Show clear label during animation (always reserve the space to prevent shifting).
	lines = append(lines, "")
	if g.lastClearLabel != "" {
		clearStyle := lipgloss.NewStyle().Bold(true).Width(panelWidth).Foreground(lipgloss.Color(ui.ColorWarning))
		if g.animIsTSpin {
			clearStyle = clearStyle.Foreground(lipgloss.Color(ui.ColorPurple))
		}
		lines = append(lines, clearStyle.Render(g.lastClearLabel))
	} else {
		lines = append(lines, pad)
	}

	for len(lines) < boardHeight+2 {
		lines = append(lines, pad)
	}
	return lines
}

// renderMiniPiece renders a small 2-line or 4-line preview of the given piece.
func renderMiniPiece(pieceIdx int) []string {
	if pieceIdx < 0 || pieceIdx >= len(tetrominoes) {
		return []string{"        ", "        "}
	}

	shape := tetrominoes[pieceIdx].rotations[0]
	colorIdx := tetrominoes[pieceIdx].color
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(kubetrisPieceColor(colorIdx)))

	var result []string
	for row := range 4 {
		hasBlock := false
		for col := range 4 {
			if shape[row][col] {
				hasBlock = true
				break
			}
		}
		if !hasBlock {
			continue
		}
		var line strings.Builder
		for col := range 4 {
			if shape[row][col] {
				line.WriteString(style.Render("\u2588\u2588\u2588"))
			} else {
				line.WriteString("   ")
			}
		}
		result = append(result, line.String())
	}
	return result
}

// renderControlsHint shows the available controls at the bottom.
func renderControlsHint() string {
	dimStyle := lipgloss.NewStyle().Faint(true)
	return dimStyle.Render("h/\u2190:left  l/\u2192:right  j/\u2193:down  k/a/\u2191:rotate  i/d:rotateCCW  space:drop  c:hold  p:pause  q/esc:quit")
}

// overlayGameOver renders the game-over message overlaid on the game.
func overlayGameOver(base string, g *kubetrisGame) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorError)).
		Padding(1, 3).
		Bold(true).
		Align(lipgloss.Center)

	content := fmt.Sprintf(
		"GAME OVER\n\nScore: %d\nHigh Score: %d\n\nPress q or Esc to exit",
		g.score, g.highScore,
	)

	overlay := boxStyle.Render(content)
	overlayW := lipgloss.Width(overlay)
	overlayH := lipgloss.Height(overlay)

	baseLines := strings.Split(base, "\n")
	baseW := 0
	for _, l := range baseLines {
		if w := lipgloss.Width(l); w > baseW {
			baseW = w
		}
	}
	baseH := len(baseLines)

	startRow := (baseH - overlayH) / 2
	startCol := (baseW - overlayW) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	overlayLines := strings.Split(overlay, "\n")
	for i, oLine := range overlayLines {
		r := startRow + i
		if r >= len(baseLines) {
			break
		}
		baseLine := baseLines[r]
		baseRunes := []rune(baseLine)
		oRunes := []rune(oLine)

		// Build new line: prefix + overlay + suffix.
		var newLine strings.Builder
		for c := 0; c < startCol && c < len(baseRunes); c++ {
			newLine.WriteRune(baseRunes[c])
		}
		for len([]rune(newLine.String())) < startCol {
			newLine.WriteRune(' ')
		}
		newLine.WriteString(string(oRunes))

		baseLines[r] = newLine.String()
	}

	return strings.Join(baseLines, "\n")
}

// overlayPaused renders a pause message centered on the screen.
func overlayPaused(width, height int) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorWarning)).
		Padding(1, 3).
		Bold(true).
		Align(lipgloss.Center)

	overlay := boxStyle.Render("PAUSED\n\nPress p to resume")
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}

// handleKubetrisKey processes keyboard input during the Kubetris game.
func (m Model) handleKubetrisKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	g := m.kubetrisGame
	if g == nil {
		m.mode = modeExplorer
		return m, nil
	}

	key := msg.String()

	// Always allow quit.
	if key == "q" || key == "esc" || key == "ctrl+c" {
		g.saveHighScore()
		m.kubetrisGame = nil
		m.mode = modeExplorer
		return m, nil
	}

	// Game over: only quit is allowed.
	if g.gameOver {
		return m, nil
	}

	// Toggle pause.
	if key == "p" {
		g.paused = !g.paused
		if g.paused {
			return m, nil
		}
		return m, m.scheduleKubetrisTick()
	}

	// Don't handle game keys while paused.
	if g.paused {
		return m, nil
	}

	var rescheduleLock bool
	switch key {
	case "h", "left":
		rescheduleLock = g.moveLeft()
	case "l", "right":
		rescheduleLock = g.moveRight()
	case "j", "down":
		g.softDrop()
	case "k", "a", "up":
		rescheduleLock = g.rotateCW()
	case "i", "d":
		rescheduleLock = g.rotateCCW()
	case " ":
		g.hardDrop()
		if g.gameOver {
			g.saveHighScore()
			return m, nil
		}
		// Hard drop may trigger animation -- start it immediately.
		if g.animating {
			return m, scheduleKubetrisAnimTick()
		}
		return m, nil
	case "c":
		g.holdCurrentPiece()
	}

	if rescheduleLock {
		return m, scheduleKubetrisLockDelay()
	}

	return m, nil
}
