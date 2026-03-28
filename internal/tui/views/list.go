package views

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"twchfetch/internal/api"
	"twchfetch/internal/tui/styles"
)

// uptimeDurationColor returns a colour on the blue→yellow→red gradient
// based on live duration in seconds. 1 min = deepest blue, 10 h = deepest red.
func uptimeDurationColor(sec int64) color.Color {
	const minSec int64 = 60    // 1 minute
	const maxSec int64 = 36000 // 10 hours

	t := float64(sec-minSec) / float64(maxSec-minSec)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	// Blue #1E90FF (30,144,255) → Yellow #FFD700 (255,215,0) → Red #FF2222 (255,34,34)
	var r, g, b float64
	if t < 0.5 {
		u := t * 2
		r = 30 + u*225
		g = 144 + u*71
		b = 255 - u*255
	} else {
		u := (t - 0.5) * 2
		r = 255
		g = 215 - u*181
		b = u * 34
	}
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", int(r), int(g), int(b)))
}

// ListItem wraps a streamer with its 1-based display number in the
// pre-search-filtered list.
type ListItem struct {
	Streamer    api.StreamerStatus
	OriginalNum int // 1-based within the current filter
}

// SortStreamers separates live (ascending uptime = newest first) and offline
// (alphabetical).
func SortStreamers(all []api.StreamerStatus) []api.StreamerStatus {
	var live, offline []api.StreamerStatus
	for _, s := range all {
		if s.IsLive {
			live = append(live, s)
		} else {
			offline = append(offline, s)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].UptimeSec < live[j].UptimeSec })
	sort.Slice(offline, func(i, j int) bool {
		return strings.ToLower(offline[i].Username) < strings.ToLower(offline[j].Username)
	})
	return append(live, offline...)
}

// FilterStreamers applies the live/offline tab filter.
func FilterStreamers(all []api.StreamerStatus, filter string) []api.StreamerStatus {
	if filter == "all" || filter == "" {
		return all
	}
	var out []api.StreamerStatus
	for _, s := range all {
		if filter == "live" && s.IsLive {
			out = append(out, s)
		} else if filter == "offline" && !s.IsLive {
			out = append(out, s)
		}
	}
	return out
}

// SearchFilter applies the search query against streamer usernames and display
// numbers.  Supports the full operator set: |, &, !, (), "...", ^, $, field:.
// The recognised fields for list items are "user:" and "msg:" (display number).
func SearchFilter(filtered []api.StreamerStatus, query string) []ListItem {
	n := ParseSearchQuery(query)
	items := make([]ListItem, 0, len(filtered))
	for i, s := range filtered {
		target := newListTarget(s.Username, i+1)
		if MatchNode(n, target) {
			items = append(items, ListItem{Streamer: s, OriginalNum: i + 1})
		}
	}
	return items
}

// ---------------------------------------------------------------------------
// Internal renderers
// ---------------------------------------------------------------------------

func renderFilterTabs(active string, liveN, offlineN, width int, nextRefreshAt time.Time) string {
	type tab struct {
		label, shortcut, val string
	}
	tabs := []tab{
		{"All", "a", "all"},
		{"● Live", "o", "live"},
		{"Offline", "f", "offline"},
	}
	isActive := func(val string) bool {
		return val == active || (active == "" && val == "all")
	}
	var parts []string
	for _, t := range tabs {
		if isActive(t.val) {
			parts = append(parts, styles.FilterTabActive.Render(t.label))
		} else {
			label := t.label + " " + styles.Dim.Render(t.shortcut)
			parts = append(parts, styles.FilterTab.Render(label))
		}
	}
	tabsStr := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	// Right side: live/offline counts + optional auto-refresh countdown.
	stats := "  " + styles.LiveBadge.Render(fmt.Sprintf("● %d", liveN)) +
		"  " + styles.OfflineBadge.Render(fmt.Sprintf("○ %d", offlineN))

	if !nextRefreshAt.IsZero() {
		remaining := time.Until(nextRefreshAt)
		if remaining < 0 {
			remaining = 0
		}
		totalSec := int(remaining.Seconds())
		m := totalSec / 60
		s := totalSec % 60
		// ":" pulses every second: visible on even seconds, dim on odd.
		var colon string
		if time.Now().Second()%2 == 0 {
			colon = styles.Dim.Render(":")
		} else {
			colon = styles.AutoRefreshColon.Render(":")
		}
		countdown := styles.AutoRefreshLabel.Render("Auto Refresh: ") +
			styles.AutoRefreshTimer.Render(fmt.Sprintf("%d", m)) +
			colon +
			styles.AutoRefreshTimer.Render(fmt.Sprintf("%02d", s))
		stats += "   " + countdown
	}

	tabsW := lipgloss.Width(tabsStr)
	statsW := lipgloss.Width(stats)
	gap := width - tabsW - statsW - 1
	if gap < 1 {
		gap = 1
	}
	return tabsStr + strings.Repeat(" ", gap) + stats
}

// RenderSearchBar renders the search input bar used by the list and chat views.
// focused controls whether the blinking cursor is shown (true while the user
// is actively typing into the bar).
// hints is an optional set of "key label" pairs (double-space-separated) that
// are right-aligned inside the bar and shown only when focused and when enough
// space remains after the query and match count.  Pass "" for no hints.
// Example: "enter keep  esc clear"
func RenderSearchBar(query string, resultCount, width int, focused bool, hints string) string {
	// Bake the search background into every element so inner ANSI resets don't
	// expose the terminal background behind coloured text.
	bg := styles.ColorSearch
	bst := func(st lipgloss.Style) lipgloss.Style { return st.Copy().Background(bg) }

	prompt := bst(styles.SearchIcon).Render("/")
	var textPart string
	if query == "" {
		if focused {
			textPart = bst(lipgloss.NewStyle()).Render(" ") + bst(styles.AccentBold).Render("▌")
		} else {
			textPart = bst(lipgloss.NewStyle()).Render(" ")
		}
	} else {
		if focused {
			textPart = bst(lipgloss.NewStyle()).Render(" ") + bst(styles.Text).Render(query) + bst(styles.AccentBold).Render("▌")
		} else {
			textPart = bst(lipgloss.NewStyle()).Render(" ") + bst(styles.Text).Render(query)
		}
	}
	count := ""
	if query != "" {
		count = bst(lipgloss.NewStyle()).Render("  ") + bst(styles.SearchCount).Render(fmt.Sprintf("(%d)", resultCount))
	}
	inner := prompt + textPart + count

	// Right-aligned hints — only when focused and the bar has room.
	// Pairs are separated by two spaces; each pair is "key label".
	if focused && hints != "" {
		var hintParts []string
		for _, pair := range strings.Split(hints, "  ") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			if idx := strings.IndexByte(pair, ' '); idx > 0 {
				hintParts = append(hintParts,
					bst(styles.HelpKey).Render(pair[:idx])+
						bst(styles.Dim).Render(" "+pair[idx+1:]))
			} else {
				hintParts = append(hintParts, bst(styles.HelpKey).Render(pair))
			}
		}
		if len(hintParts) > 0 {
			sep := bst(lipgloss.NewStyle()).Render("   ")
			styledHints := strings.Join(hintParts, sep)
			// SearchBarStyle has PaddingLeft(1)+PaddingRight(1); Width(width)
			// sets total width, so content area = width-2.
			contentW := width - 2
			innerW := lipgloss.Width(inner)
			hintW := lipgloss.Width(styledHints)
			avail := contentW - innerW - hintW
			if avail >= 2 {
				inner = inner +
					bst(lipgloss.NewStyle()).Render(strings.Repeat(" ", avail)) +
					styledHints
			}
		}
	}

	return styles.SearchBarStyle.Width(width).Render(inner)
}


func renderListHelp(inSearch bool) string {
	parts := []string{hintItem("↑↓", "navigate"), hintItem("enter", "select")}
	if inSearch {
		parts = append(parts, hintItem("esc", "cancel search"))
	} else {
		parts = append(parts,
			hintItem("/", "search"),
			hintItem("a", "all"),
			hintItem("o", "live"),
			hintItem("f", "offline"),
			hintItem("r", "refresh"),
			hintItem("s", "settings"),
			hintItem("q", "quit"),
		)
	}
	return styles.HelpBar.Render(strings.Join(parts, "   "))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
