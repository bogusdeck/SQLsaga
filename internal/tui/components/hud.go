package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HUD is the top bar with the story title, chapter, position, XP, and timer.
type HUD struct {
	width  int
	styles HUDStyles
	state  HUDState
}

// HUDStyles groups the lipgloss styles used by the HUD.
type HUDStyles struct {
	Bar       lipgloss.Style
	Title     lipgloss.Style
	Stat      lipgloss.Style
	Muted     lipgloss.Style
}

// HUDState is the live data rendered by the HUD.
type HUDState struct {
	StoryTitle    string
	ChapterTitle  string
	Position      string // e.g. "2 / 10"
	XP            int
	Streak        int
	TimerSeconds  int
	TimerTotal    int
	StatusMessage string
}

// NewHUD returns a default HUD.
func NewHUD(styles HUDStyles) HUD { return HUD{styles: styles} }

// SetSize updates the bar width.
func (h *HUD) SetSize(w int) { h.width = w }

// SetState pushes new state into the HUD.
func (h *HUD) SetState(s HUDState) { h.state = s }

// View renders the bar.
func (h *HUD) View() string {
	state := h.state
	title := state.StoryTitle
	if state.ChapterTitle != "" {
		title = fmt.Sprintf("%s — %s", state.StoryTitle, state.ChapterTitle)
	}
	left := h.styles.Title.Render(truncate(title, h.width/2))
	pos := h.styles.Stat.Render(fmt.Sprintf(" %s ", state.Position))
	xp := h.styles.Stat.Render(fmt.Sprintf(" XP %d ", state.XP))
	streak := h.styles.Muted.Render(fmt.Sprintf(" streak %d", state.Streak))
	timer := h.timerBadge(state)
	right := pos + " " + xp + " " + timer + streak
	gap := h.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	return h.styles.Bar.Render(left + strings.Repeat(" ", gap) + right)
}

func (h *HUD) timerBadge(s HUDState) string {
	if s.TimerTotal <= 0 {
		return h.styles.Muted.Render(" --:-- ")
	}
	remaining := s.TimerSeconds
	if remaining < 0 {
		remaining = 0
	}
	mins := remaining / 60
	secs := remaining % 60
	label := fmt.Sprintf(" %02d:%02d ", mins, secs)
	if remaining <= 10 {
		return h.styles.Stat.Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#FF5F87")).Render(label)
	}
	return h.styles.Stat.Render(label)
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
