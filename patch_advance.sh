cat << 'INNER_EOF' >> internal/tui/app.go

type autoAdvanceMsg struct{}

func autoAdvanceCmd() tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
		return autoAdvanceMsg{}
	})
}
INNER_EOF
