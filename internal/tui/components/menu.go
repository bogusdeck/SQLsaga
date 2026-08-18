package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const AsciiTitle = `███████╗ ██████╗ ██╗     ███████╗ █████╗  ██████╗  █████╗ 
██╔════╝██╔═══██╗██║     ██╔════╝██╔══██╗██╔════╝ ██╔══██╗
███████╗██║   ██║██║     ███████╗███████║██║  ███╗███████║
╚════██║██║▄▄ ██║██║     ╚════██║██╔══██║██║   ██║██╔══██║
███████║╚██████╔╝███████╗███████║██║  ██║╚██████╔╝██║  ██║
╚══════╝ ╚══▀▀═╝ ╚══════╝╚══════╝╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝`

type MenuItem struct {
	Title       string
	Description string
	Action      string
}

type MenuStyles struct {
	Panel     lipgloss.Style
	Title     lipgloss.Style
	Item      lipgloss.Style
	Selected  lipgloss.Style
	Shortcut  lipgloss.Style
	Desc      lipgloss.Style
	Empty     lipgloss.Style
}

type Menu struct {
	visible  bool
	width    int
	height   int
	items    []MenuItem
	selected int
	title    string
	styles   MenuStyles
	empty    string
}

func NewMenu(styles MenuStyles) Menu {
	return Menu{
		visible: true,
		title:   AsciiTitle,
		styles:  styles,
		items: []MenuItem{
			{Title: "New Game", Description: "Start a new adventure", Action: "new_game"},
			{Title: "Continue", Description: "Resume your progress", Action: "continue"},
			{Title: "Select Story", Description: "Choose a different story", Action: "select_story"},
			{Title: "Stats", Description: "View your statistics", Action: "stats"},
			{Title: "Achievements", Description: "View unlocked achievements", Action: "achievements"},
			{Title: "Settings", Description: "Configure the game", Action: "settings"},
			{Title: "Quit", Description: "Exit the game", Action: "quit"},
		},
	}
}

func (m *Menu) SetSize(w, h int) { m.width = w; m.height = h }
func (m *Menu) SetVisible(v bool) { m.visible = v }
func (m Menu) IsVisible() bool    { return m.visible }
func (m Menu) Selected() int      { return m.selected }
func (m Menu) Items() []MenuItem  { return m.items }
func (m *Menu) Select(idx int) {
	if idx >= 0 && idx < len(m.items) {
		m.selected = idx
	}
}

// SetTitle updates the menu title.
func (m *Menu) SetTitle(t string) { m.title = t }

// SetEmptyMessage sets the message shown when there are no items.
func (m *Menu) SetEmptyMessage(s string) { m.empty = s }

// SetItems replaces the menu items and resets the selection.
func (m *Menu) SetItems(items []MenuItem) {
	m.items = items
	m.selected = 0
}

func (m Menu) Update(msg tea.Msg) (Menu, tea.Cmd) {
	if !m.visible {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Always allow Esc to go back, even if menu is empty
		if key.Matches(msg, key.NewBinding(key.WithKeys("esc"))) {
			return m, func() tea.Msg { return MenuActionMsg{Action: "menu_back"} }
		}

		if len(m.items) == 0 {
			return m, nil
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			m.selected = (m.selected - 1 + len(m.items)) % len(m.items)
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			m.selected = (m.selected + 1) % len(m.items)
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
			return m, func() tea.Msg { return MenuActionMsg{Action: m.items[m.selected].Action} }
		case key.Matches(msg, key.NewBinding(key.WithKeys("q"))):
			if m.items[m.selected].Action == "quit" {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m Menu) GetClickedItem(relY int) int {
	titleRender := m.styles.Title.Align(lipgloss.Center).Render(m.title)
	titleHeight := lipgloss.Height(titleRender)
	
	// 1 line for top border, 1 line for top padding, title height, and 2 lines for \n\n
	currentY := 1 + 1 + titleHeight + 2

	for i, item := range m.items {
		itemHeight := 1
		if item.Description != "" {
			itemHeight++
		}
		
		if relY >= currentY && relY < currentY+itemHeight {
			return i
		}
		
		currentY += itemHeight
		if i < len(m.items)-1 {
			currentY++ // blank line
		}
	}
	return -1
}

type MenuActionMsg struct{ Action string }

func (m Menu) View() string {
	if !m.visible {
		return ""
	}
	var b strings.Builder
	titleRender := m.styles.Title.Align(lipgloss.Center).Render(m.title)
	b.WriteString(titleRender)
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		if m.empty != "" {
			b.WriteString(m.styles.Empty.Render(m.empty))
		}
		return m.styles.Panel.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1, 2).
			Render(b.String())
	}

	for i, item := range m.items {
		prefix := "  "
		style := m.styles.Item
		if i == m.selected {
			prefix = "► "
			style = m.styles.Selected
		}
		line := fmt.Sprintf("%s%-20s", prefix+item.Title, "")
		b.WriteString(style.Render(line))
		if item.Description != "" {
			b.WriteString("\n")
			b.WriteString(m.styles.Desc.Render("    " + item.Description))
		}
		if i < len(m.items)-1 {
			b.WriteString("\n")
		}
	}
	return m.styles.Panel.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Render(b.String())
}
