package components

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/bogusdeck/sqlsaga/internal/game"
)

// StoryPanel renders the narrative + objective + revealed hints.
type StoryPanel struct {
	viewport viewport.Model
	width    int
	height   int
	styles   StoryStyles
	challenge *game.Challenge
	chapter   *game.Chapter
	story       *game.Story
	hints       []string
	animFrame   int
}

// StoryStyles groups the lipgloss styles used by the panel.
type StoryStyles struct {
	PanelTitle lipgloss.Style
	Narrative  lipgloss.Style
	Objective  lipgloss.Style
	Hint       lipgloss.Style
	Muted      lipgloss.Style
}

// NewStoryPanel creates an empty panel; call SetChallenge to populate.
func NewStoryPanel(styles StoryStyles) StoryPanel {
	vp := viewport.New(0, 0)
	return StoryPanel{viewport: vp, styles: styles}
}

// SetSize updates the panel's dimensions.
func (s *StoryPanel) SetSize(w, h int) {
	s.width = w
	s.height = h
	s.viewport.Width = w
	s.viewport.Height = h
	s.refreshContent()
}

// SetChallenge swaps the current challenge (and its chapter for context).
func (s *StoryPanel) SetChallenge(story *game.Story, c *game.Chapter, challenge *game.Challenge, hints []string) {
	s.story = story
	s.chapter = c
	s.challenge = challenge
	s.hints = hints
	s.refreshContent()
}


func (s *StoryPanel) SetFrame(f int) {
	s.animFrame = f
	s.refreshContent()
}

var seniorAsciiFrames = []string{
	"   .-------.\n  /  o   o  \\\n |     >     |\n  \\  -----  /\n   '-------'",
	"   .-------.\n  /  o   o  \\\n |     >     |\n  \\  OOOOO  /\n   '-------'",
}

func (s *StoryPanel) renderSeniorAscii() string {
	frame := seniorAsciiFrames[s.animFrame%len(seniorAsciiFrames)]
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#87ceeb")).Bold(true).Render(frame)
}

// AddHint pushes a hint to the revealed list and re-renders.
func (s *StoryPanel) AddHint(hint string) {
	if hint == "" {
		return
	}
	s.hints = append(s.hints, hint)
	s.refreshContent()
}

// Update forwards Bubble Tea messages to the embedded viewport.
func (s StoryPanel) Update(msg tea.Msg) (StoryPanel, tea.Cmd) {
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// View renders the story panel.
func (s StoryPanel) View() string {
	return s.viewport.View()
}

func (s *StoryPanel) refreshContent() {
	if s.challenge == nil || s.chapter == nil {
		s.viewport.SetContent(s.styles.Muted.Render("No challenge loaded."))
		return
	}
	var b strings.Builder
	
	if s.story != nil {
		b.WriteString(s.styles.PanelTitle.Foreground(lipgloss.Color("#F8BD3F")).Render(strings.ToUpper(s.story.Title)))
		b.WriteString("\n\n")
		b.WriteString(s.styles.Narrative.Width(s.width).Render(s.highlightKeywords(s.story.Description)))
		b.WriteString("\n\n")
		b.WriteString(s.styles.Muted.Render(strings.Repeat("─", 20)))
		b.WriteString("\n\n")
	}

	b.WriteString(s.styles.PanelTitle.Render("Chapter: " + s.chapter.Title))
	b.WriteString("\n\n")
	var narrativeText strings.Builder
	if s.chapter.Narrative != "" {
		narrativeText.WriteString(s.chapter.Narrative)
		narrativeText.WriteString("\n\n")
	}
	if s.challenge.Narrative != "" {
		narrativeText.WriteString(s.challenge.Narrative)
	}
	if narrativeText.Len() == 0 {
		narrativeText.WriteString("Let's get to work.")
	}

	b.WriteString(s.styles.Narrative.Width(s.width).Render(s.highlightKeywords(strings.TrimSpace(narrativeText.String()))))
	b.WriteString("\n\n")
	b.WriteString(s.styles.PanelTitle.Render("Senior's Instructions"))
	b.WriteString("\n")
	
	instructionStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#D7FF00")).
		Foreground(lipgloss.Color("#000000")).
		Bold(true).
		Padding(0, 1).
		Width(s.width - 2)
		
	b.WriteString(instructionStyle.Render(s.challenge.Objective))
	b.WriteString("\n\n")
	
	if len(s.hints) > 0 {
		b.WriteString(s.styles.PanelTitle.Render("Senior's Hints"))
		b.WriteString("\n")
		for i, h := range s.hints {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, s.styles.Hint.Width(s.width-5).Render(h))
		}
	} else {
		b.WriteString(s.styles.Muted.Render("(press Ctrl+H to reveal a hint)"))
	}
	if s.width > 0 {
		b.WriteString("\n" + s.styles.Muted.Render(strings.Repeat("─", s.width-4)))
	}
	s.viewport.SetContent(b.String())
}

var keywordRegex = regexp.MustCompile(`\b(SELECT|FROM|WHERE|INSERT|INTO|VALUES|CREATE|TABLE|PRIMARY KEY|BETWEEN|AND|OR|NOT|NULL|INTEGER|TEXT|JOIN|ON|GROUP BY|ORDER BY|ASC|DESC|LIMIT|OFFSET|UPDATE|SET|DELETE|COUNT|SUM|AVG|MIN|MAX)\b`)

func (s *StoryPanel) highlightKeywords(text string) string {
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5fd7ff")).Bold(true)
	return keywordRegex.ReplaceAllStringFunc(text, func(match string) string {
		return highlightStyle.Render(match)
	})
}
