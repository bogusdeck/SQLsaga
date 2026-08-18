package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AchievementsStyles groups the lipgloss styles used by the panel.
type AchievementsStyles struct {
	Panel lipgloss.Style
}

// AchievementsPanel shows unlocked achievements.
type AchievementsPanel struct {
	visible  bool
	width    int
	height   int
	style    lipgloss.Style
	items    []AchievementItem
}

// AchievementItem represents a single achievement.
type AchievementItem struct {
	ID          string
	Name        string
	Description string
	Unlocked    bool
}

// NewAchievementsPanel creates a new achievements panel.
func NewAchievementsPanel(styles AchievementsStyles) AchievementsPanel {
	return AchievementsPanel{
		style: styles.Panel,
		items: defaultAchievements(),
	}
}

func defaultAchievements() []AchievementItem {
	return []AchievementItem{
		{ID: "first_steps", Name: "First Steps", Description: "Complete your first challenge", Unlocked: false},
		{ID: "query_master", Name: "Query Master", Description: "Complete 10 challenges in a row without failing", Unlocked: false},
		{ID: "speed_demon", Name: "Speed Demon", Description: "Complete a challenge in under 30 seconds", Unlocked: false},
		{ID: "hint_hoarder", Name: "Hint Hoarder", Description: "Reveal all hints in a single challenge", Unlocked: false},
		{ID: "perfectionist", Name: "Perfectionist", Description: "Complete a challenge on the first try", Unlocked: false},
		{ID: "night_owl", Name: "Night Owl", Description: "Play after midnight", Unlocked: false},
		{ID: "early_bird", Name: "Early Bird", Description: "Play before 6 AM", Unlocked: false},
		{ID: "explorer", Name: "Explorer", Description: "Complete all challenges in a story", Unlocked: false},
	}
}

// SetAchievements updates the achievement list with unlocked status.
func (a *AchievementsPanel) SetAchievements(unlocked []string) {
	unlockedSet := make(map[string]bool)
	for _, id := range unlocked {
		unlockedSet[id] = true
	}
	for i := range a.items {
		a.items[i].Unlocked = unlockedSet[a.items[i].ID]
	}
}

// Show displays the panel.
func (a *AchievementsPanel) Show() { a.visible = true }

// Hide hides the panel.
func (a *AchievementsPanel) Hide() { a.visible = false }

// IsVisible returns whether the panel is visible.
func (a *AchievementsPanel) IsVisible() bool { return a.visible }

// SetSize updates the panel dimensions.
func (a *AchievementsPanel) SetSize(w, h int) {
	a.width = w
	a.height = h
}

// Update handles key messages.
func (a AchievementsPanel) Update(msg tea.Msg) (AchievementsPanel, tea.Cmd) {
	if !a.visible {
		return a, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+a":
			a.Hide()
		}
	}
	return a, nil
}

// View renders the achievements panel.
func (a AchievementsPanel) View() string {
	if !a.visible {
		return ""
	}
	var b strings.Builder
	b.WriteString(a.style.Render("ACHIEVEMENTS"))
	b.WriteString("\n\n")
	
	unlocked := 0
	for _, item := range a.items {
		if item.Unlocked {
			unlocked++
			b.WriteString(fmt.Sprintf("  ✓ %s — %s\n", item.Name, item.Description))
		} else {
			b.WriteString(fmt.Sprintf("  ○ %s — %s\n", item.Name, item.Description))
		}
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Unlocked: %d / %d", unlocked, len(a.items)))
	
	return a.style.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#7D56F4")).Padding(1, 2).Render(b.String())
}

// OverlayView renders the panel as an overlay at position (x, y) on top of base.
func (a AchievementsPanel) OverlayView(x, y int, base string) string {
	if !a.visible {
		return base
	}
	panel := a.View()
	if panel == "" {
		return base
	}
	lines := strings.Split(base, "\n")
	panelLines := strings.Split(panel, "\n")

	for len(lines) <= y+len(panelLines) {
		lines = append(lines, "")
	}

	for i, pline := range panelLines {
		targetY := y + i
		if targetY >= len(lines) {
			break
		}
		line := lines[targetY]
		if len(line) < x {
			line += strings.Repeat(" ", x-len(line))
		}
		lines[targetY] = line[:x] + pline + line[x+len(pline):]
	}

	return strings.Join(lines, "\n")
}