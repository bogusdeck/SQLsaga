package components

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Editor wraps a bubbles textarea with a small set of focus-aware helpers.
type Editor struct {
	ta        textarea.Model
	focused   bool
	showLines bool
	style        lipgloss.Style
	prevValue    string
	history      []string
	historyIndex int
}

// NewEditor constructs a default editor styled like a SQL REPL prompt.
func NewEditor(style lipgloss.Style) Editor {
	ta := textarea.New()
	ta.Placeholder = ""
	ta.Prompt = "> "
	ta.CharLimit = 0
	ta.SetWidth(0)
	ta.SetHeight(0)
	ta.Focus()
	ta.ShowLineNumbers = false
	return Editor{ta: ta, focused: true, showLines: false, style: style, history: []string{}, historyIndex: 0}
}

// SetSize updates the editor dimensions.
func (e *Editor) SetSize(w, h int) {
	e.ta.SetWidth(w)
	e.ta.SetHeight(h)
	inner := e.style.GetHorizontalFrameSize()
	e.ta.SetWidth(w - inner)
	e.ta.SetHeight(h - inner)
	if e.focused {
		e.ta.Focus()
	}
}

// Focus / Blur toggle editing.
func (e *Editor) Focus() { e.focused = true; e.ta.Focus() }
func (e *Editor) Blur()  { e.focused = false; e.ta.Blur() }
func (e *Editor) Focused() bool { return e.focused }

// Reset clears the editor and remembers the cleared state.
func (e *Editor) Reset() {
	e.prevValue = e.ta.Value()
	e.ta.Reset()
}

// AddToHistory saves a query to the editor's history stack.
func (e *Editor) AddToHistory(q string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return
	}
	if len(e.history) == 0 || e.history[len(e.history)-1] != q {
		e.history = append(e.history, q)
	}
	e.historyIndex = len(e.history)
	e.prevValue = ""
}

// Restore puts the previously-cleared value back into the editor.
func (e *Editor) Restore() {
	if e.prevValue != "" {
		e.ta.SetValue(e.prevValue)
	}
}

// SetValue overwrites the editor contents.
func (e *Editor) SetValue(v string) { e.ta.SetValue(v) }

// SetShowLineNumbers toggles line numbers on or off.
func (e *Editor) SetShowLineNumbers(show bool) {
	e.showLines = show
	e.ta.ShowLineNumbers = show
}

// Value returns the current editor contents.
func (e *Editor) Value() string { return e.ta.Value() }

// Update forwards messages to the underlying textarea.
func (e Editor) Update(msg tea.Msg) (Editor, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			if e.historyIndex > 0 {
				if e.historyIndex == len(e.history) {
					e.prevValue = e.ta.Value()
				}
				e.historyIndex--
				e.ta.SetValue(e.history[e.historyIndex])
				return e, nil
			}
		case tea.KeyDown:
			if e.historyIndex < len(e.history) {
				e.historyIndex++
				if e.historyIndex == len(e.history) {
					e.ta.SetValue(e.prevValue)
				} else {
					e.ta.SetValue(e.history[e.historyIndex])
				}
				return e, nil
			}
		}
	}
	e.ta, cmd = e.ta.Update(msg)
	return e, cmd
}

// View renders the editor.
func (e Editor) View() string {
	body := e.ta.View()
	body = highlightSQL(body)
	if !e.focused {
		body = lipgloss.NewStyle().Faint(true).Render(body)
	}
	return e.style.Render(body)
}

// Trimmed returns the editor's value, trimmed of surrounding whitespace.
func (e Editor) Trimmed() string { return strings.TrimSpace(e.ta.Value()) }

// GetCurrentWord returns the word at the end of the editor content (for autocomplete).
func (e *Editor) GetCurrentWord() string {
	text := e.ta.Value()
	// Find the last word-like sequence at the end
	for i := len(text) - 1; i >= 0; i-- {
		c := text[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '(' || c == ',' || c == '.' || c == ';' {
			return text[i+1:]
		}
	}
	return text
}

// InsertAtCursor inserts text at the end of the editor content.
func (e *Editor) InsertAtCursor(text string) {
	e.ta.InsertString(text)
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.'
}

// FormatSQL auto-capitalizes common SQL keywords to tidy up the query.
func (e *Editor) FormatSQL() {
	val := e.ta.Value()
	keywords := []string{
		"select", "from", "where", "insert", "into", "values", "update", "set", "delete",
		"create", "table", "drop", "alter", "add", "column", "group", "by", "order", "limit",
		"asc", "desc", "and", "or", "not", "null", "is", "join", "inner", "left", "right",
		"outer", "on", "as", "count", "sum", "avg", "min", "max", "between", "in", "having",
		"with", "union", "intersect", "except", "case", "when", "then", "else", "end",
	}

	for _, kw := range keywords {
		re := regexp.MustCompile(`(?i)\b` + kw + `\b`)
		val = re.ReplaceAllStringFunc(val, strings.ToUpper)
	}
	e.ta.SetValue(val)
}
