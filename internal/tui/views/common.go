package views

import (
	"strings"

	"charm.land/lipgloss/v2"

	"twchfetch/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// Layout helpers (used by multiple view files)
// ---------------------------------------------------------------------------

// Rule renders a full-width horizontal line.
func Rule(width int) string {
	if width <= 2 {
		width = 60
	}
	return styles.Rule.Render(strings.Repeat("─", width))
}

// ---------------------------------------------------------------------------
// Text helpers
// ---------------------------------------------------------------------------

// displayWidth returns the terminal display width of s, correctly counting
// wide characters (emoji, CJK ideographs, etc.) as 2 columns each.
func displayWidth(s string) int {
	return lipgloss.Width(s)
}

// truncate cuts s to at most maxWidth display columns, appending "…".
// Handles wide characters (emoji, CJK) correctly.
func truncate(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	w := 0
	for i, r := range s {
		cw := lipgloss.Width(string(r))
		if w+cw > maxWidth-1 {
			return s[:i] + "…"
		}
		w += cw
	}
	return s
}

// highlightMatch bolds the first occurrence of query in s.
// Uses byte-range slicing; correct for ASCII usernames.
func highlightMatch(s, query string) string {
	if query == "" {
		return s
	}
	lower := strings.ToLower(s)
	q := strings.ToLower(query)
	idx := strings.Index(lower, q)
	if idx < 0 {
		return s
	}
	end := idx + len(q)
	return s[:idx] + styles.MatchHighlight.Render(s[idx:end]) + s[end:]
}

// ---------------------------------------------------------------------------
// Help-bar helpers
// ---------------------------------------------------------------------------

// hintItem renders a single "key desc" help-bar entry: the key in accent
// style and the description dimmed.
func hintItem(key, desc string) string {
	return styles.HelpKey.Render(key) + " " + styles.Dim.Render(desc)
}
