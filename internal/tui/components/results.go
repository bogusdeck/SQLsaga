package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
	"github.com/bogusdeck/sqlquest/internal/parser"
)

// ResultsPanel renders the outcome of the most recent query.
type ResultsPanel struct {
	viewport viewport.Model
	width    int
	height   int
	styles   ResultsStyles

	// Latest result state
	matched  bool
	showPlan bool
	query    string
	execMs   float64
	plan     string
	cols     []string
	rows     []map[string]any
	diff     parser.Diff
	err      string
	schema   string
	evaluated bool
}

// ResultsStyles groups the lipgloss styles used by the panel.
type ResultsStyles struct {
	Panel    lipgloss.Style
	Match    lipgloss.Style
	Error    lipgloss.Style
	Muted    lipgloss.Style
	Head     lipgloss.Style
	Cell     lipgloss.Style
}

// NewResultsPanel creates an empty panel.
func NewResultsPanel(styles ResultsStyles) ResultsPanel {
	vp := viewport.New(0, 0)
	return ResultsPanel{viewport: vp, styles: styles}
}

// SetSize updates the panel dimensions.
func (r *ResultsPanel) SetSize(w, h int) {
	r.width = w
	r.height = h
	r.viewport.Width = w
	r.viewport.Height = h
	r.refreshContent()
}

// ShowResult populates the panel with the result of a query.
func (r *ResultsPanel) ShowResult(query string, rr parser.RunResult, diff parser.Diff, plan string, evaluated bool) {
	r.query = query
	r.execMs = rr.ExecMillis
	r.cols = rr.Result.Columns
	r.rows = rr.Result.Rows
	r.diff = diff
	r.matched = diff.Matched
	r.plan = plan
	r.showPlan = false
	r.evaluated = evaluated
	if rr.Err != nil {
		r.err = rr.Err.Error()
	} else {
		r.err = ""
	}
	r.refreshContent()
}

// SetSchema sets the database schema to display.
func (r *ResultsPanel) SetSchema(schema string) {
	r.schema = schema
	r.refreshContent()
}

// Clear wipes the panel.
func (r *ResultsPanel) Clear() {
	r.query = ""
	r.execMs = 0
	r.cols = nil
	r.rows = nil
	r.err = ""
	r.plan = ""
	r.schema = ""
	r.matched = false
	r.evaluated = false
	r.refreshContent()
}

// TogglePlan flips between the result view and the EXPLAIN QUERY PLAN view.
func (r *ResultsPanel) TogglePlan() {
	if r.plan == "" {
		return
	}
	r.showPlan = !r.showPlan
	r.refreshContent()
}

// Update forwards messages.
func (r ResultsPanel) Update(msg tea.Msg) (ResultsPanel, tea.Cmd) {
	var cmd tea.Cmd
	r.viewport, cmd = r.viewport.Update(msg)
	return r, cmd
}

// View renders the panel.
func (r ResultsPanel) View() string { return r.viewport.View() }

func (r *ResultsPanel) refreshContent() {
	var b strings.Builder

	if r.query != "" || r.err != "" {
		b.WriteString(r.styles.Muted.Render(fmt.Sprintf("  exec: %.2f ms   rows: %d   press Ctrl+E to toggle plan", r.execMs, len(r.rows))))
		b.WriteString("\n")
		if r.showPlan && r.plan != "" {
			b.WriteString(r.styles.Muted.Render("EXPLAIN QUERY PLAN"))
			b.WriteString("\n")
			b.WriteString(r.styles.Muted.Render(r.plan))
			b.WriteString("\n")
		} else {
			if r.err != "" {
				b.WriteString(r.styles.Error.Render("Error: " + r.err))
				b.WriteString("\n")
			}
			if r.matched {
				b.WriteString(r.styles.Match.Render("✓ Match! Your query produced the expected output."))
				b.WriteString("\n\n")
			} else if r.err == "" && r.evaluated {
				b.WriteString(r.styles.Error.Render("✗ Mismatch"))
				b.WriteString("\n")
				b.WriteString(r.styles.Muted.Width(r.width).Render(r.diff.String()))
				b.WriteString("\n")
				
				if len(r.diff.MissingRows) > 0 {
					b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87")).Render("Missing Expected Rows:"))
					b.WriteString("\n")
					b.WriteString(r.styles.Muted.Width(r.width).Render(formatRowList(r.diff.ExpectedCols, r.diff.MissingRows)))
					b.WriteString("\n")
				}
				if len(r.diff.ExtraRows) > 0 {
					b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87")).Render("Unexpected Extra Rows:"))
					b.WriteString("\n")
					b.WriteString(r.styles.Muted.Width(r.width).Render(formatRowList(r.diff.ActualCols, r.diff.ExtraRows)))
					b.WriteString("\n")
				}
			}

			if len(r.cols) > 0 {
				b.WriteString(r.renderTable())
			} else if r.err == "" && !r.matched {
				b.WriteString(r.styles.Muted.Render("Your query returned no columns. Re-read the objective."))
				b.WriteString("\n")
			}
		}
	} else if r.schema == "" {
		b.WriteString(r.styles.Muted.Render("Run a query to see results."))
	}
	r.viewport.SetContent(b.String())
}

func (r *ResultsPanel) renderTable() string {
	t := lgtable.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#7C7C7C")))

	// Set headers
	var headers []string
	for _, c := range r.cols {
		headers = append(headers, c)
	}
	t.Headers(headers...)

	// Add rows
	for _, row := range r.rows {
		var cells []string
		for _, c := range r.cols {
			cells = append(cells, fmt.Sprintf("%v", row[c]))
		}
		t.Row(cells...)
	}

	// Dynamic styling
	headerStyle := lipgloss.NewStyle().Bold(true)
	if r.matched {
		headerStyle = headerStyle.Foreground(lipgloss.Color("#04B575"))
	} else if r.err == "" {
		headerStyle = headerStyle.Foreground(lipgloss.Color("#FF5F87"))
	} else {
		headerStyle = headerStyle.Foreground(lipgloss.Color("#7D56F4"))
	}
	t.StyleFunc(func(row, col int) lipgloss.Style {
		if row == 0 {
			return headerStyle
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E1ED")).Padding(0, 1)
	})

	return t.Render()
}

func formatRowList(cols []string, rows []map[string]any) string {
	var b strings.Builder
	for i, r := range rows {
		if i >= 5 {
			b.WriteString(fmt.Sprintf("  ... and %d more\n", len(rows)-5))
			break
		}
		var vals []string
		for _, c := range cols {
			vals = append(vals, fmt.Sprintf("%v: %v", c, r[c]))
		}
		b.WriteString("  - { " + strings.Join(vals, ", ") + " }\n")
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
