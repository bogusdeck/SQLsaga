// Package tui holds the Bubble Tea model that powers the SQL Quest CLI.
package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary   = lipgloss.Color("#7D56F4")
	colorAccent    = lipgloss.Color("#04B575")
	colorDanger    = lipgloss.Color("#FF5F87")
	colorMuted     = lipgloss.Color("#7C7C7C")
	colorBgPanel   = lipgloss.Color("#1E1E2E")
	colorFgBright  = lipgloss.Color("#FAFAFA")
	colorYellow    = lipgloss.Color("#F8BD3F")
	colorBlue      = lipgloss.Color("#7FB7BE")
)

// Styles bundles every lipgloss style used in the TUI.
type Styles struct {
	App          lipgloss.Style
	Hud          lipgloss.Style
	HudTitle     lipgloss.Style
	HudStat      lipgloss.Style
	Footer       lipgloss.Style
	FooterKey    lipgloss.Style
	Panel        lipgloss.Style
	PanelTitle   lipgloss.Style
	Narrative    lipgloss.Style
	Objective    lipgloss.Style
	Hint         lipgloss.Style
	Editor       lipgloss.Style
	Result       lipgloss.Style
	ResultMatch  lipgloss.Style
	ResultError  lipgloss.Style
	ResultMuted  lipgloss.Style
	Status       lipgloss.Style
	Overlay      lipgloss.Style
	Stats        lipgloss.Style
	Completion   lipgloss.Style
	Divider      lipgloss.Style
	LeftPanel    lipgloss.Style
	RightPanel   lipgloss.Style
}

// DefaultStyles returns the dark theme used by v1.
func DefaultStyles() Styles {
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1)

	leftPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1)

	rightPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBlue).
		Padding(0, 1)

	divider := lipgloss.NewStyle().
		Foreground(colorPrimary).
		SetString("│")

	return Styles{
		App:        lipgloss.NewStyle(),
		Hud:        lipgloss.NewStyle().Foreground(colorFgBright).Background(colorPrimary).Bold(true).Padding(0, 1),
		HudTitle:   lipgloss.NewStyle().Foreground(colorYellow).Bold(true),
		HudStat:    lipgloss.NewStyle().Foreground(colorBgPanel).Background(colorAccent).Bold(true).Padding(0, 1),
		Footer:     lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1),
		FooterKey:  lipgloss.NewStyle().Foreground(colorYellow).Bold(true),
		Panel:      panel,
		LeftPanel:  leftPanel,
		RightPanel: rightPanel,
		PanelTitle: lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Underline(true),
		Narrative:  lipgloss.NewStyle().Foreground(colorFgBright),
		Objective:  lipgloss.NewStyle().Foreground(colorBlue).Bold(true),
		Hint:       lipgloss.NewStyle().Foreground(colorYellow).Italic(true),
		Editor:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorAccent).Padding(0, 1),
		Result:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBlue).Padding(0, 1),
		ResultMatch:  lipgloss.NewStyle().Foreground(colorAccent).Bold(true),
		ResultError:  lipgloss.NewStyle().Foreground(colorDanger).Bold(true),
		ResultMuted:  lipgloss.NewStyle().Foreground(colorMuted),
		Status:       lipgloss.NewStyle().Foreground(colorMuted).Italic(true),
		Overlay:      lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(colorPrimary).Padding(1, 2),
		Stats:        lipgloss.NewStyle().Foreground(colorFgBright),
		Completion:   lipgloss.NewStyle().Foreground(colorFgBright),
		Divider:      divider,
	}
}
