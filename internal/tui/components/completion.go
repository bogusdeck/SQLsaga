package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CompletionStyles groups the lipgloss styles used by the completion panel.
type CompletionStyles struct {
	Panel lipgloss.Style
}

// CompletionPanel shows SQL autocomplete suggestions.
type CompletionPanel struct {
	visible     bool
	width       int
	height      int
	style       lipgloss.Style
	suggestions []string
	selected    int
}

// NewCompletionPanel creates a new completion panel.
func NewCompletionPanel(styles CompletionStyles) CompletionPanel {
	return CompletionPanel{
		style:       styles.Panel,
		suggestions: defaultSQLKeywords(),
	}
}

func defaultSQLKeywords() []string {
	return []string{
		"SELECT", "FROM", "WHERE", "ORDER BY", "GROUP BY", "LIMIT", "OFFSET",
		"INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER",
		"JOIN", "LEFT JOIN", "RIGHT JOIN", "INNER JOIN", "OUTER JOIN",
		"ON", "USING", "AS", "DISTINCT", "ALL",
		"AND", "OR", "NOT", "IN", "LIKE", "BETWEEN", "IS NULL", "IS NOT NULL",
		"COUNT", "SUM", "AVG", "MIN", "MAX",
		"CASE", "WHEN", "THEN", "ELSE", "END",
		"EXPLAIN", "QUERY PLAN",
	}
}

// SetSize updates the panel dimensions.
func (c *CompletionPanel) SetSize(w, h int) {
	c.width = w
	c.height = h
}

// Show displays the panel.
func (c *CompletionPanel) Show() {
	c.visible = true
	c.selected = 0
}

// Hide hides the panel.
func (c *CompletionPanel) Hide() {
	c.visible = false
}

// IsVisible returns whether the panel is visible.
func (c *CompletionPanel) IsVisible() bool {
	return c.visible
}

// Update handles key messages for navigation.
func (c CompletionPanel) Update(msg tea.Msg) (CompletionPanel, tea.Cmd) {
	if !c.visible {
		return c, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "prev"))):
			c.selected = (c.selected - 1 + len(c.suggestions)) % len(c.suggestions)
		case key.Matches(msg, key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next"))):
			c.selected = (c.selected + 1) % len(c.suggestions)
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", "tab"), key.WithHelp("enter/tab", "select"))):
			return c, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))):
			c.Hide()
		}
	}
	return c, nil
}

// SetSuggestions updates the suggestion list.
func (c *CompletionPanel) SetSuggestions(s []string) {
	c.suggestions = s
	c.selected = 0
}

// GetSelected returns the currently selected suggestion.
func (c CompletionPanel) GetSelected() string {
	if c.selected >= 0 && c.selected < len(c.suggestions) {
		return c.suggestions[c.selected]
	}
	return ""
}

// View renders the completion panel.
func (c CompletionPanel) View() string {
	if !c.visible || len(c.suggestions) == 0 {
		return ""
	}
	var b strings.Builder
	maxVisible := c.height
	if maxVisible <= 0 {
		maxVisible = 8
	}
	start := 0
	if c.selected >= maxVisible {
		start = c.selected - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(c.suggestions) {
		end = len(c.suggestions)
	}
	for i := start; i < end; i++ {
		s := c.suggestions[i]
		if i == c.selected {
			b.WriteString(c.style.Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#7D56F4")).Render("► " + s))
		} else {
			b.WriteString(c.style.Render("  " + s))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return c.style.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#7D56F4")).Padding(0, 1).Render(b.String())
}

// Filter filters suggestions by prefix.
func (c *CompletionPanel) Filter(prefix string) {
	if prefix == "" {
		c.suggestions = defaultSQLKeywords()
	} else {
		var filtered []string
		lower := strings.ToLower(prefix)
		for _, s := range defaultSQLKeywords() {
			if strings.HasPrefix(strings.ToLower(s), lower) {
				filtered = append(filtered, s)
			}
		}
		c.suggestions = filtered
	}
	c.selected = 0
}

// OverlayView renders the panel as an overlay at position (x, y) on top of base.
func (c CompletionPanel) OverlayView(x, y int, base string) string {
	if !c.visible {
		return base
	}
	panel := c.View()
	if panel == "" {
		return base
	}
	// Split base into lines
	lines := strings.Split(base, "\n")
	panelLines := strings.Split(panel, "\n")

	// Ensure we have enough lines
	for len(lines) <= y+len(panelLines) {
		lines = append(lines, "")
	}

	// Overlay panel lines
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