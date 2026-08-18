package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/bogusdeck/sqlsaga/internal/game"
	"github.com/bogusdeck/sqlsaga/internal/parser"
	"github.com/bogusdeck/sqlsaga/internal/tui/components"
	"github.com/bogusdeck/sqlsaga/internal/utils"
)

// App is the root Bubble Tea model.
type App struct {
	width, height int

	engine      *game.Engine
	cfg         *utils.Config
	styles      Styles
	keys        KeyMap
	hud         components.HUD
	story       components.StoryPanel
	editor      components.Editor
	results     components.ResultsPanel
	completion  components.CompletionPanel
	achievements components.AchievementsPanel
	menu        components.Menu
	dsnInput    textinput.Model

	tickerStop chan struct{}
	ticker     *time.Ticker

	status        string
	statusIsError bool
	showStats     bool
	statsText     string
	showHelp      bool
	showAchievements bool
	showMenu      bool
	helpText      string

	// menuMode: "main" or "story_picker" — controls what the menu shows.
	menuMode     string
	pickerItems  []components.MenuItem

	// Split view state
	splitRatio   float64 // 0.0 to 1.0, position of divider (0.35 = 35% left)
	dragging     bool    // true when user is dragging the divider

	// template populated on first render
	templateVisible bool
	animFrame       int
}

// KeyMap describes every key binding the App reacts to.
type KeyMap struct {
	Submit          key.Binding
	Reset           key.Binding
	Hint            key.Binding
	Next            key.Binding
	Prev            key.Binding
	Plan            key.Binding
	Stats           key.Binding
	Help            key.Binding
	Quit            key.Binding
	Confirm         key.Binding
	Complete        key.Binding
	Achievements    key.Binding
}

// DefaultKeyMap returns the v1 key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Submit:       key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "submit query")),
		Reset:        key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "reset editor")),
		Hint:         key.NewBinding(key.WithKeys("ctrl+h"), key.WithHelp("ctrl+h", "reveal next hint")),
		Next:         key.NewBinding(key.WithKeys("ctrl+n"), key.WithHelp("ctrl+n", "next challenge")),
		Prev:         key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "previous challenge")),
		Plan:         key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "toggle query plan")),
		Stats:        key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", "show stats")),
		Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
		Quit:         key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "quit")),
		Confirm:      key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm")),
		Complete:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "autocomplete")),
		Achievements: key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "show achievements")),
	}
}

// tickMsg is sent every second so the HUD timer can count down.
type tickMsg time.Time

// tickCmd is the command that produces tickMsg.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// NewApp builds the root model.
func NewApp(engine *game.Engine, cfg *utils.Config) *App {
	styles := DefaultStyles()
	keys := DefaultKeyMap()

	editor := components.NewEditor(styles.Editor)
	
	hud := components.NewHUD(components.HUDStyles{
		Bar:   styles.Hud,
		Title: styles.HudTitle,
		Stat:  styles.HudStat,
		Muted: styles.Footer,
	})
	story := components.NewStoryPanel(components.StoryStyles{
		PanelTitle: styles.PanelTitle,
		Narrative:  styles.Narrative,
		Objective:  styles.Objective,
		Hint:       styles.Hint,
		Muted:      styles.Status,
	})
	results := components.NewResultsPanel(components.ResultsStyles{
		Panel: styles.Result,
		Match: styles.ResultMatch,
		Error: styles.ResultError,
		Muted: styles.ResultMuted,
		Head:  styles.PanelTitle,
		Cell:  styles.Narrative,
	})
	completion := components.NewCompletionPanel(components.CompletionStyles{
		Panel: styles.Completion,
	})
	achievements := components.NewAchievementsPanel(components.AchievementsStyles{
		Panel: styles.Completion,
	})
	menu := components.NewMenu(components.MenuStyles{
		Panel:    styles.Completion,
		Title:    styles.PanelTitle,
		Item:     styles.Narrative,
		Selected: styles.ResultMatch,
		Shortcut: styles.Hint,
		Desc:     styles.Status,
		Empty:    styles.Narrative,
	})



	dsnInput := textinput.New()
	dsnInput.Placeholder = "root:@tcp(127.0.0.1:3306)/"
	dsnInput.CharLimit = 256
	dsnInput.Width = 50

	app := &App{
		engine:        engine,
		cfg:           cfg,
		styles:        styles,
		keys:          keys,
		hud:           hud,
		story:         story,
		results:       results,
		completion:    completion,
		achievements:  achievements,
		menu:          menu,
		dsnInput:      dsnInput,
		editor:        editor,
		status:        "Press Ctrl+S to submit, Ctrl+H for a hint, Tab for autocomplete, Ctrl+A for achievements, ? for help.",
		showMenu:      true,
		splitRatio:    0.35,
	}
	
	if cfg.MySQLDSN != "" {
		os.Setenv("MYSQL_DSN", cfg.MySQLDSN)
	}
	
	return app
}

// Init kicks off the timer.
func (a *App) Init() tea.Cmd {
	return tickCmd()
}

// Update handles all messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.layout()
		return a, nil
	case autoAdvanceMsg:
		if a.engine.NextChallenge() {
			a.bindCurrentChallenge()
			a.setStatus("Loaded next challenge.", false)
		} else {
			a.setStatus("You completed all challenges!", false)
		}
		return a, nil
	case tea.MouseMsg:
		if a.showMenu {
			// Calculate if click was inside menu panel
			panel := a.menu.View()
			h := lipgloss.Height(panel)
			w := lipgloss.Width(panel)
			startX := (a.width - w) / 2
			startY := (a.height - 1 - h) / 2
			
			if msg.X >= startX && msg.X < startX+w && msg.Y >= startY && msg.Y < startY+h {
				if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
					idx := a.menu.GetClickedItem(msg.Y - startY)
					if idx != -1 {
						a.menu.Select(idx)
						a.handleMenuAction(a.menu.Items()[idx].Action)
					}
				}
				return a, nil
			}
		}
		return a.handleMouse(msg)
	case tickMsg:
		a.refreshHUD()
		a.animFrame++
		a.story.SetFrame(a.animFrame)
		cmds = append(cmds, tickCmd())
	case components.MenuActionMsg:
		if msg.Action == "quit" {
			return a, tea.Quit
		}
		a.handleMenuAction(msg.Action)
		return a, nil
	case tea.KeyMsg:
		if a.menuMode == "setting_dsn" {
			if key.Matches(msg, a.keys.Quit) {
				return a, tea.Quit
			}
			switch msg.Type {
			case tea.KeyEnter:
				val := a.dsnInput.Value()
				if err := parser.TestConnection(val); err != nil {
					a.setStatus(fmt.Sprintf("Connection failed: %v", err), true)
					return a, nil
				}
				a.cfg.MySQLDSN = val
				utils.Save(a.cfg)
				if a.cfg.MySQLDSN != "" {
					os.Setenv("MYSQL_DSN", a.cfg.MySQLDSN)
				}
				a.openSettingsMenu()
				a.setStatus("MySQL DSN saved & verified.", false)
				return a, nil
			case tea.KeyEsc:
				a.openSettingsMenu()
				return a, nil
			}
			var icmd tea.Cmd
			a.dsnInput, icmd = a.dsnInput.Update(msg)
			return a, icmd
		}

		// Handle menu navigation when menu is visible
		if a.showMenu {
			if key.Matches(msg, a.keys.Quit) {
				return a, tea.Quit
			}
			var mcmd tea.Cmd
			a.menu, mcmd = a.menu.Update(msg)
			return a, mcmd
		}

		if a.showHelp {
			a.showHelp = false
			return a, nil
		}
		if a.showStats {
			a.showStats = false
			return a, nil
		}
		switch {
		case msg.Type == tea.KeyEnter:
			val := strings.TrimSpace(a.editor.Value())
			if strings.HasSuffix(val, ";") {
				a.runQuery()
				return a, nil
			}
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.Submit):
			return a, a.submit()
		case key.Matches(msg, a.keys.Reset):
			a.editor.Reset()
			a.setStatus("Editor reset.", false)
		case key.Matches(msg, a.keys.Hint):
			h := a.engine.RevealNextHint()
			if h == "" {
				a.setStatus("No more hints available.", true)
			} else {
				a.story.AddHint(h)
				a.setStatus("Hint revealed.", false)
			}
		case key.Matches(msg, a.keys.Next):
			if a.engine.NextChallenge() {
				a.bindCurrentChallenge()
				a.setStatus("Loaded next challenge.", false)
			} else {
				a.setStatus("Already at the final challenge.", true)
			}
		case key.Matches(msg, a.keys.Prev):
			if a.engine.PrevChallenge() {
				a.bindCurrentChallenge()
				a.setStatus("Loaded previous challenge.", false)
			} else {
				a.setStatus("Already at the first challenge.", true)
			}

		case key.Matches(msg, a.keys.Plan):
			a.results.TogglePlan()
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+f"))):
			a.editor.FormatSQL()
		case key.Matches(msg, key.NewBinding(key.WithKeys("pgdown", "pgup", "ctrl+u", "ctrl+d"))):
			var cmd1, cmd2 tea.Cmd
			a.story, cmd1 = a.story.Update(msg)
			a.results, cmd2 = a.results.Update(msg)
			return a, tea.Batch(cmd1, cmd2)
		case key.Matches(msg, a.keys.Stats):
			a.showStats = !a.showStats
			if a.showStats {
				a.statsText = a.composeStats()
			}
		case key.Matches(msg, a.keys.Help):
			a.showHelp = !a.showHelp
		case key.Matches(msg, a.keys.Complete):
			a.handleComplete()
		case key.Matches(msg, a.keys.Achievements):
			a.showAchievements = !a.showAchievements
		}
	}
	var cmd tea.Cmd
	a.editor, cmd = a.editor.Update(msg)
	cmds = append(cmds, cmd)
	a.story, cmd = a.story.Update(msg)
	cmds = append(cmds, cmd)
	a.results, cmd = a.results.Update(msg)
	cmds = append(cmds, cmd)
	a.completion, cmd = a.completion.Update(msg)
	cmds = append(cmds, cmd)
	a.achievements, cmd = a.achievements.Update(msg)
	cmds = append(cmds, cmd)

	a.layout()
	return a, tea.Batch(cmds...)
}

// handleMouse processes mouse events for drag-to-resize panels and routes scrolling to hovered panels.
func (a *App) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Calculate panel boundaries
	hudHeight := 1
	footerHeight := 2
	
	lines := strings.Count(a.editor.Value(), "\n") + 1
	if lines < 1 {
		lines = 1
	} else if lines > 15 {
		lines = 15
	}
	editorHeight := lines
	editorFrame := 2
	
	totalFixed := hudHeight + footerHeight + editorHeight + editorFrame
	var middleHeight int
	if totalFixed >= a.height {
		middleHeight = 1
	} else {
		middleHeight = a.height - totalFixed
	}

	middleStartY := hudHeight
	middleEndY := middleStartY + middleHeight
	dividerX := int(float64(a.width) * a.splitRatio)

	// Clear dragging state on release
	if msg.Action == tea.MouseActionRelease {
		a.dragging = false
	}

	// Handle dragging divider
	if a.dragging || (msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress) {
		if msg.Y >= middleStartY && msg.Y < middleEndY {
			const dragThreshold = 3
			if a.dragging || (msg.X >= dividerX-dragThreshold && msg.X <= dividerX+dragThreshold) {
				a.dragging = true
				newRatio := float64(msg.X) / float64(a.width)
				if newRatio < 0.15 {
					newRatio = 0.15
				}
				if newRatio > 0.85 {
					newRatio = 0.85
				}
				a.splitRatio = newRatio
				a.layout()
				return a, nil
			}
		}
	}

	// Route other mouse events to the hovered panel
	var cmd tea.Cmd
	if msg.Y >= middleStartY && msg.Y < middleEndY {
		if msg.X < dividerX {
			a.story, cmd = a.story.Update(msg)
		} else {
			a.results, cmd = a.results.Update(msg)
		}
	} else if msg.Y >= middleEndY && msg.Y < a.height-footerHeight {
		a.editor, cmd = a.editor.Update(msg)
	}

	return a, cmd
}

// View renders the entire TUI.
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "loading…"
	}
	// Minimum terminal size
	if a.width < 80 || a.height < 24 {
		return a.styles.Overlay.Render(fmt.Sprintf("Terminal too small (%dx%d)\nMinimum: 80x24", a.width, a.height))
	}
	if a.menuMode == "setting_dsn" {
		var b strings.Builder
		b.WriteString(a.styles.PanelTitle.Render(" MySQL DSN Configuration "))
		b.WriteString("\n\n")
		b.WriteString("Enter your MySQL connection string. For example:\n")
		b.WriteString("  root:password@tcp(127.0.0.1:3306)/\n\n")
		b.WriteString(a.dsnInput.View())
		b.WriteString("\n\n")
		b.WriteString(a.styles.Status.Render("Press Enter to save, Esc to cancel."))
		
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1, 2).
			Render(b.String())
		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box)
	}
	// Show main menu on startup
	if a.showMenu {
		panel := a.menu.View()
		help := "↑/↓: navigate  •  enter: select"
		if a.menuMode != "main" {
			help += "  •  esc: back"
		} else {
			help += "  •  q: quit"
		}
		
		centered := lipgloss.Place(a.width, a.height-1, lipgloss.Center, lipgloss.Center, panel)
		footer := lipgloss.NewStyle().Width(a.width).Align(lipgloss.Center).Foreground(lipgloss.Color("#7C7C7C")).Render(help)
		
		return lipgloss.JoinVertical(lipgloss.Left, centered, footer)
	}

	// Calculate split sizes. The body is: HUD (1) + MIDDLE + editor + footer.
	// The editor renders into a bordered box whose rendered height is
	// editorHeight+2 (one border line top + bottom). We size everything
	// from a single constant so the final body never exceeds the terminal
	// height, otherwise the top gets clipped.
	hudHeight := 1
	footerHeight := 2
	lines := strings.Count(a.editor.Value(), "\n") + 1
	if lines < 1 {
		lines = 1
	} else if lines > 15 {
		lines = 15
	}
	editorHeight := lines // logical lines of editor content
	editorFrame := 2      // top + bottom border
	totalFixed := hudHeight + footerHeight + editorHeight + editorFrame
	var middleHeight int
	if totalFixed >= a.height {
		middleHeight = 1
	} else {
		middleHeight = a.height - totalFixed
	}

	leftW := int(float64(a.width) * a.splitRatio)
	rightW := a.width - leftW

	// Clamp
	if leftW < 20 {
		leftW = 20
	}
	if rightW < 20 {
		rightW = 20
	}
	if leftW > a.width-25 {
		leftW = a.width - 25
	}
	if rightW > a.width-25 {
		rightW = a.width - 25
	}

	// Build middle section: left panel + divider + right panel.
	// We use MaxHeight (not Height) so the rendered boxes clip to exactly
	// middleHeight lines even when the inner content is taller — Height()
	// only grows the box, it does not clip overflow.
	leftContent := a.styles.LeftPanel.
		Width(leftW - 2). // -2 for borders
		Height(middleHeight - 2).
		Render(a.story.View())

	rightContent := a.styles.RightPanel.
		Width(rightW - 2). // -2 for borders
		Height(middleHeight - 2).
		Render(a.results.View())

	middle := lipgloss.JoinHorizontal(lipgloss.Top, leftContent, rightContent)

	editorView := a.editor.View()

	// Full layout
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		a.hud.View(),
		middle,
		editorView,
		a.footer(),
	)


	if a.showHelp {
		overlay := a.helpOverlay()
		body = overlayOverlay(body, overlay, a.width, a.height)
	}
	if a.showStats {
		overlay := a.statsOverlay()
		body = overlayOverlay(body, overlay, a.width, a.height)
	}
	if a.showAchievements {
		overlay := a.achievementsOverlay()
		body = overlayOverlay(body, overlay, a.width, a.height)
	}
	if a.completion.IsVisible() {
		editorX := 0
		editorY := a.height - 6 // approximate position near editor
		body = a.completion.OverlayView(editorX, editorY, body)
	}
	return body
}

func (a *App) footer() string {
	keys := []string{
		"enter(;): run",
		"ctrl+s: submit",
		"ctrl+f: format",
		"ctrl+n/p: challenge",
		"ctrl+e: plan",
		"ctrl+g: stats",
		"ctrl+a: achievements",
		"tab: autocomplete",
		"?: help",
		"q: quit",
	}
	status := a.statusText()
	
	s := strings.Join(keys, "  ")
	if len(s) > a.width-2 {
		s = s[:a.width-5] + "..."
	}
	if len(status) > a.width-2 {
		status = status[:a.width-5] + "..."
	}
	
	return a.styles.Footer.Render(s + "\n" + status)
}

func (a *App) statusText() string {
	style := a.styles.Status
	if a.statusIsError {
		style = a.styles.ResultError
	}
	return style.Render("» " + a.status)
}

func (a *App) setStatus(s string, isErr bool) {
	a.status = s
	a.statusIsError = isErr
}

func (a *App) refreshHUD() {
	ch := a.engine.CurrentChallenge()
	if ch == nil {
		return
	}
	pos, total := a.engine.ChallengePosition()
	elapsed := int(time.Since(a.engine.StartedAt).Seconds())
	remaining := ch.TimeLimit - elapsed
	if remaining < -1 {
		remaining = -1
	}
	a.hud.SetState(components.HUDState{
		StoryTitle:    a.engine.Story.Title,
		ChapterTitle:  a.engine.CurrentChapter().Title,
		Position:      fmt.Sprintf("%d / %d", pos, total),
		XP:            a.engine.TotalXP,
		Streak:        a.engine.Streak,
		TimerSeconds:  remaining,
		TimerTotal:    ch.TimeLimit,
		StatusMessage: a.status,
	})
}

func (a *App) layout() {
	if a.width < 20 || a.height < 10 {
		return
	}
	hudHeight := 1
	footerHeight := 2
	
	lines := strings.Count(a.editor.Value(), "\n") + 1
	if lines < 1 {
		lines = 1
	} else if lines > 15 {
		lines = 15
	}
	editorHeight := lines
	editorFrame := 2
	totalFixed := hudHeight + footerHeight + editorHeight + editorFrame
	var middleHeight int
	if totalFixed >= a.height {
		middleHeight = 1
	} else {
		middleHeight = a.height - totalFixed
	}

	ratio := a.splitRatio
	if ratio < 0.15 {
		ratio = 0.15
	}
	if ratio > 0.85 {
		ratio = 0.85
	}
	leftW := int(float64(a.width) * ratio)
	rightW := a.width - leftW

	a.hud.SetSize(a.width)
	a.story.SetSize(leftW-4, middleHeight-2)    // -2 for border, -2 for padding
	a.results.SetSize(rightW-4, middleHeight-2) // -2 for border, -2 for padding
	a.editor.SetSize(a.width, editorHeight+editorFrame)
	a.menu.SetSize(a.width, a.height)
}

func (a *App) bindCurrentChallenge() {
	chapter := a.engine.CurrentChapter()
	challenge := a.engine.CurrentChallenge()
	if chapter == nil || challenge == nil {
		return
	}
	a.story.SetChallenge(&a.engine.Story, chapter, challenge, nil)
	a.results.SetSchema(challenge.Schema)
	// Restore hints visible in UI to match already-revealed ones.
	for i := 0; i < a.engine.HintsRevealed(); i++ {
		if i < len(challenge.Hints) {
			a.story.AddHint(challenge.Hints[i])
		}
	}
	a.refreshHUD()
}

func (a *App) submit() tea.Cmd {
	c := a.engine.CurrentChallenge()
	if c == nil {
		return nil
	}
	query := a.editor.Trimmed()
	if query == "" {
		a.setStatus("Write a query first!", true)
		return nil
	}
	a.editor.AddToHistory(query)
	a.editor.Reset()
	sub := a.engine.Submit(context.Background(), query)
	schemaStr, dataStr := a.engine.GetFullStorySchemaAndData()
	plan, _ := parser.Explain("", schemaStr, dataStr, query)
	a.results.ShowResult(query, sub.Result, sub.Diff, plan, true)
	
	var cmd tea.Cmd
	if sub.Matched {
		a.setStatus(fmt.Sprintf("Correct! +%d XP — advancing...", sub.XPEarned), false)
		cmd = autoAdvanceCmd()
	} else if sub.Err != nil {
		a.setStatus("Query errored — see results pane.", true)
	} else {
		a.setStatus("Not quite — see results pane for the diff.", true)
	}
	a.refreshHUD()
	return cmd
}

func (a *App) runQuery() {
	c := a.engine.CurrentChallenge()
	if c == nil {
		return
	}
	query := a.editor.Trimmed()
	if query == "" {
		a.setStatus("Write a query first!", true)
		return
	}
	a.editor.AddToHistory(query)
	a.editor.Reset()
	
	schemaStr, dataStr := a.engine.GetFullStorySchemaAndData()
	rr := parser.Run("", schemaStr, dataStr, query, parser.DefaultQueryTimeout)
	plan, _ := parser.Explain("", schemaStr, dataStr, query)
	
	a.results.ShowResult(query, rr, parser.Diff{}, plan, false)
	if rr.Err != nil {
		a.setStatus("Query errored — see results pane.", true)
	} else {
		a.setStatus("Query executed. (Press Ctrl+S to submit and validate)", false)
	}
	a.refreshHUD()
}

func (a *App) handleComplete() {
	if a.completion.IsVisible() {
		// Insert selected suggestion
		selected := a.completion.GetSelected()
		if selected != "" {
			a.editor.InsertAtCursor(selected + " ")
		}
		a.completion.Hide()
	} else {
		// Show completion panel with filtered suggestions
		word := a.editor.GetCurrentWord()
		a.completion.Filter(word)
		a.completion.Show()
	}
}

func (a *App) composeStats() string {
	pos, total := a.engine.ChallengePosition()
	return fmt.Sprintf(
		"  Story: %s\n  Chapter: %s\n  Challenge: %d / %d\n  Total XP: %d\n  Streak: %d\n  Attempts: %d\n  Failed: %d",
		a.engine.Story.Title,
		a.engine.CurrentChapter().Title,
		pos, total,
		a.engine.TotalXP,
		a.engine.Streak,
		a.engine.Attempts,
		a.engine.FailedAttempts,
	)
}

func (a *App) helpOverlay() string {
	lines := []string{
		"SQL Quest — Keyboard Shortcuts",
		"",
		"  ctrl+s    submit query",
		"  ctrl+r    reset editor",
		"  ctrl+h    reveal next hint",
		"  ctrl+n    next challenge",
		"  ctrl+p    previous challenge",
		"  ctrl+e    toggle EXPLAIN QUERY PLAN",
		"  ctrl+g    toggle stats",
		"  ?         toggle this help",
		"  tab       autocomplete",
		"  q / ctrl+c   quit",
		"",
		"Press any key to dismiss.",
	}
	return a.styles.Overlay.Render(strings.Join(lines, "\n"))
}

func (a *App) statsOverlay() string {
	lines := []string{"SQL Quest — Run Stats", "", a.statsText, "", "Press any key to dismiss."}
	return a.styles.Overlay.Render(strings.Join(lines, "\n"))
}

func (a *App) achievementsOverlay() string {
	achievements := a.engine.GetAchievements(context.Background())
	board, _ := a.engine.GetLeaderboard(context.Background(), 5)

	var lines []string
	lines = append(lines, "SQL MASTERY — Achievements & Leaderboard", "")
	
	lines = append(lines, "  -- Your Achievements --")
	if len(achievements) == 0 {
		lines = append(lines, "  No achievements unlocked yet.", "  Complete challenges to earn achievements!")
	} else {
		for _, a := range achievements {
			lines = append(lines, fmt.Sprintf("  ✓ %s", a))
		}
	}
	lines = append(lines, "")

	lines = append(lines, "  -- Global Leaderboard --")
	if len(board) == 0 {
		lines = append(lines, "  Leaderboard unavailable (offline).")
	} else {
		for i, entry := range board {
			lines = append(lines, fmt.Sprintf("  %d. %s — %d XP", i+1, entry.DisplayName, entry.TotalXP))
		}
	}

	lines = append(lines, "", "Press any key to dismiss.")
	return a.styles.Overlay.Render(strings.Join(lines, "\n"))
}

func (a *App) handleMenuAction(action string) {
	switch action {
	case "new_game":
		a.openStoryPicker()
	case "continue":
		a.openContinueMenu()
	case "select_story":
		a.openStoryPicker()
	case "stats":
		a.openStatsMenu()
	case "achievements":
		a.openAchievementsMenu()
	case "settings":
		a.openSettingsMenu()
	case "setting_toggle_ac":
		a.cfg.Autocomplete = !a.cfg.Autocomplete
		utils.Save(a.cfg)
		a.openSettingsMenu()
	case "setting_toggle_ln":
		a.cfg.Editor.ShowLineNumbers = !a.cfg.Editor.ShowLineNumbers
		a.editor.SetShowLineNumbers(a.cfg.Editor.ShowLineNumbers)
		utils.Save(a.cfg)
		a.openSettingsMenu()
	case "setting_set_mysql":
		a.menuMode = "setting_dsn"
		a.dsnInput.SetValue(a.cfg.MySQLDSN)
		a.dsnInput.Focus()
	case "quit":
		// Handled directly in Update to return tea.Quit
	case "menu_back":
		a.menuMode = "main"
		a.menu.SetTitle(components.AsciiTitle)
		a.populateMainMenu()
	case "play_story":
		a.startPickedStory()
	default:
		if strings.HasPrefix(action, "play_story:") {
			a.startPickedStory()
		} else if strings.HasPrefix(action, "resume_story:") {
			a.resumePickedStory()
		}
	}
}

// overlayOverlay centers the overlay on top of the body.
func overlayOverlay(body, overlay string, w, h int) string {
	bodyLines := strings.Split(body, "\n")
	overlayLines := strings.Split(overlay, "\n")
	startY := (h - len(overlayLines)) / 2
	if startY < 0 {
		startY = 0
	}
	startX := (w - lipgloss.Width(overlay)) / 2
	if startX < 0 {
		startX = 0
	}
	// Pad body with blank lines so the overlay can sit in the middle.
	for len(bodyLines) < h {
		bodyLines = append(bodyLines, "")
	}
	for i, line := range overlayLines {
		target := startY + i
		if target >= len(bodyLines) {
			break
		}
		// Pad line to width, then place overlay.
		bodyLine := padRight(bodyLines[target], w)
		left := bodyLine[:startX]
		right := bodyLine[startX+lipgloss.Width(line):]
		bodyLines[target] = left + line + right
	}
	return strings.Join(bodyLines, "\n")
}

func padRight(s string, w int) string {
	if lipgloss.Width(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

// populateMainMenu resets the menu to the original main-menu items.
func (a *App) populateMainMenu() {
	var items []components.MenuItem
	summaries, err := a.engine.GetPendingGames(context.Background())
	if err == nil && len(summaries) > 0 {
		items = append(items, components.MenuItem{Title: "Continue", Description: "Resume your current story", Action: "continue"})
	}
	items = append(items, 
		components.MenuItem{Title: "New Game", Description: "Pick a story to start", Action: "new_game"},
		components.MenuItem{Title: "Stats", Description: "View your statistics", Action: "stats"},
		components.MenuItem{Title: "Achievements", Description: "View unlocked achievements", Action: "achievements"},
		components.MenuItem{Title: "Settings", Description: "Configure the game", Action: "settings"},
		components.MenuItem{Title: "Quit", Description: "Exit the game", Action: "quit"},
	)
	a.menu.SetItems(items)
}

// openStoryPicker loads all embedded stories and shows them in the menu.
func (a *App) openStoryPicker() {
	stories, err := game.LoadAllStories()
	if err != nil || len(stories) == 0 {
		a.setStatus("No stories available.", true)
		return
	}
	items := make([]components.MenuItem, 0, len(stories))
	for _, s := range stories {
		challengeCount := 0
		for _, ch := range s.Story.Chapters {
			challengeCount += len(ch.Challenges)
		}
		items = append(items, components.MenuItem{
			Title:       s.Story.Title,
			Description: fmt.Sprintf("%s  -  %d chapters, %d challenges", s.Story.Genre, len(s.Story.Chapters), challengeCount),
			Action:      "play_story:" + s.Story.ID,
		})
	}
	a.pickerItems = items
	a.menu.SetTitle("PICK A STORY")
	a.menu.SetEmptyMessage("No stories found in the embedded bundle.")
	a.menu.SetItems(items)
	a.menuMode = "story_picker"
	a.setStatus("Use up/down then Enter to pick a story. Esc goes back.", false)
}

func (a *App) openSettingsMenu() {
	var items []components.MenuItem

	// Autocomplete
	acDesc := "Turn autocomplete on or off (currently: OFF)"
	if a.cfg.Autocomplete {
		acDesc = "Turn autocomplete on or off (currently: ON)"
	}
	items = append(items, components.MenuItem{
		Title:       "Toggle Autocomplete",
		Description: acDesc,
		Action:      "setting_toggle_ac",
	})

	// Line Numbers
	lnDesc := "Show line numbers in editor (currently: OFF)"
	if a.cfg.Editor.ShowLineNumbers {
		lnDesc = "Show line numbers in editor (currently: ON)"
	}
	items = append(items, components.MenuItem{
		Title:       "Toggle Line Numbers",
		Description: lnDesc,
		Action:      "setting_toggle_ln",
	})

	items = append(items, components.MenuItem{
		Title:       "Set MySQL DSN",
		Description: "Configure MySQL connection string",
		Action:      "setting_set_mysql",
	})

	a.pickerItems = items
	a.menu.SetTitle("SETTINGS")
	a.menu.SetItems(items)
	a.menuMode = "settings"
	a.setStatus("Use up/down and Enter to toggle. Esc goes back.", false)
}

func (a *App) openContinueMenu() {
	summaries, err := a.engine.GetPendingGames(context.Background())
	if err != nil || len(summaries) == 0 {
		a.menu.SetTitle("CONTINUE")
		a.menu.SetItems(nil)
		a.menu.SetEmptyMessage("No pending games found.\nStart a new game from 'New Game'!")
		a.menuMode = "continue"
		a.setStatus("Press Esc to go back.", false)
		return
	}

	var items []components.MenuItem
	for _, s := range summaries {
		story, _ := game.LoadStory(s.StoryID)
		title := s.StoryID
		if story != nil {
			title = story.Story.Title
		}
		items = append(items, components.MenuItem{
			Title:       title,
			Description: fmt.Sprintf("%d challenges completed • %d XP • Last played: %s", s.ChallengesDone, s.TotalXP, s.LastPlayed.Format("Jan 02")),
			Action:      "resume_story:" + s.StoryID,
		})
	}
	a.pickerItems = items
	a.menu.SetTitle("CONTINUE")
	a.menu.SetItems(items)
	a.menuMode = "continue"
	a.setStatus("Use up/down then Enter to continue a story. Esc goes back.", false)
}

func (a *App) resumePickedStory() {
	idx := a.menu.Selected()
	if idx < 0 || idx >= len(a.pickerItems) {
		return
	}
	action := a.pickerItems[idx].Action
	const prefix = "resume_story:"
	if !strings.HasPrefix(action, prefix) {
		return
	}
	storyID := action[len(prefix):]
	story, err := game.LoadStory(storyID)
	if err != nil {
		a.setStatus(fmt.Sprintf("Could not load story: %v", err), true)
		return
	}
	a.engine.ResumeStory(context.Background(), story.Story)
	a.editor.Reset()
	a.results.Clear()
	a.showMenu = false
	a.menuMode = "main"
	a.populateMainMenu()
	a.bindCurrentChallenge()
	a.setStatus(fmt.Sprintf("Resumed game: %s", story.Story.Title), false)
}

func (a *App) openStatsMenu() {
	a.menu.SetTitle("STATS")
	a.menu.SetItems(nil)
	a.menu.SetEmptyMessage(a.composeStats())
	a.menuMode = "stats"
	a.setStatus("Press Esc to go back.", false)
}

func (a *App) openAchievementsMenu() {
	a.menu.SetTitle("ACHIEVEMENTS & LEADERBOARD")
	a.menu.SetItems(nil)

	achievements := a.engine.GetAchievements(context.Background())
	board, _ := a.engine.GetLeaderboard(context.Background(), 5)

	var lines []string
	lines = append(lines, "  -- Your Achievements --")
	if len(achievements) == 0 {
		lines = append(lines, "  No achievements unlocked yet.", "  Complete challenges to earn achievements!")
	} else {
		for _, ach := range achievements {
			lines = append(lines, fmt.Sprintf("  ✓ %s", ach))
		}
	}
	lines = append(lines, "")

	lines = append(lines, "  -- Global Leaderboard --")
	if len(board) == 0 {
		lines = append(lines, "  Leaderboard unavailable (offline).")
	} else {
		for i, entry := range board {
			lines = append(lines, fmt.Sprintf("  %d. %s — %d XP", i+1, entry.DisplayName, entry.TotalXP))
		}
	}

	a.menu.SetEmptyMessage(strings.Join(lines, "\n"))
	a.menuMode = "achievements"
	a.setStatus("Press Esc to go back.", false)
}

// startPickedStory swaps the engine to the story under the cursor and
// closes the menu.
func (a *App) startPickedStory() {
	idx := a.menu.Selected()
	if idx < 0 || idx >= len(a.pickerItems) {
		return
	}
	action := a.pickerItems[idx].Action
	const prefix = "play_story:"
	if len(action) <= len(prefix) || action[:len(prefix)] != prefix {
		return
	}
	storyID := action[len(prefix):]
	story, err := game.LoadStory(storyID)
	if err != nil {
		a.setStatus(fmt.Sprintf("Could not load story: %v", err), true)
		return
	}
	a.engine.SetStory(story.Story)
	a.editor.Reset()
	a.results.Clear()
	a.showMenu = false
	a.menuMode = "main"
	a.populateMainMenu()
	a.bindCurrentChallenge()
	a.setStatus(fmt.Sprintf("New game started: %s", story.Story.Title), false)
}

// EnsureCurrentChallenge binds the active challenge into the UI on startup.
func (a *App) EnsureCurrentChallenge() {
	a.bindCurrentChallenge()
}
type autoAdvanceMsg struct{}

func autoAdvanceCmd() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
		return autoAdvanceMsg{}
	})
}
