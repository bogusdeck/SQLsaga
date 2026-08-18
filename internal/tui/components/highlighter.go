package components

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var sqlKeywords = []string{
	"SELECT", "FROM", "WHERE", "INSERT", "INTO", "VALUES", "CREATE", "TABLE",
	"PRIMARY", "KEY", "BETWEEN", "AND", "OR", "NOT", "NULL", "INTEGER", "TEXT",
	"JOIN", "ON", "GROUP BY", "ORDER BY", "ASC", "DESC", "LIMIT", "OFFSET",
	"UPDATE", "SET", "DELETE", "COUNT", "SUM", "AVG", "MIN", "MAX",
}

var keywordMap = make(map[string]bool)

func init() {
	for _, kw := range sqlKeywords {
		keywordMap[strings.ToUpper(kw)] = true
	}
}

// highlightSQL takes a string that may contain ANSI escape codes and applies
// a keyword highlight color to SQL keywords, without messing up the ANSI codes.
func highlightSQL(input string) string {
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	
	// Split the string into ANSI codes and plain text.
	// We can use FindAllStringIndex to find all ANSI codes.
	var result strings.Builder
	lastIdx := 0
	
	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF87D7")).Bold(true)
	
	wordRegex := regexp.MustCompile(`[a-zA-Z_]+`)
	
	matches := ansiRegex.FindAllStringIndex(input, -1)
	for _, match := range matches {
		start, end := match[0], match[1]
		
		// Process plain text before the ANSI code
		plainText := input[lastIdx:start]
		result.WriteString(highlightPlainText(plainText, wordRegex, highlightStyle))
		
		// Append the ANSI code unmodified
		result.WriteString(input[start:end])
		
		lastIdx = end
	}
	
	// Process remaining plain text
	if lastIdx < len(input) {
		plainText := input[lastIdx:]
		result.WriteString(highlightPlainText(plainText, wordRegex, highlightStyle))
	}
	
	return result.String()
}

func highlightPlainText(text string, wordRegex *regexp.Regexp, style lipgloss.Style) string {
	var result strings.Builder
	lastIdx := 0
	
	matches := wordRegex.FindAllStringIndex(text, -1)
	for _, match := range matches {
		start, end := match[0], match[1]
		word := text[start:end]
		
		result.WriteString(text[lastIdx:start])
		
		if keywordMap[strings.ToUpper(word)] {
			result.WriteString(style.Render(word))
		} else {
			result.WriteString(word)
		}
		
		lastIdx = end
	}
	
	if lastIdx < len(text) {
		result.WriteString(text[lastIdx:])
	}
	
	return result.String()
}
