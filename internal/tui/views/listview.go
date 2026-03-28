package views

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"twchfetch/internal/api"
	"twchfetch/internal/tui/styles"
	"twchfetch/internal/util"
)

// RenderListView renders the streamer list as one or more side-by-side scrollable
// tables.  All tables share the same column layout and scroll offset — the data
// is one continuous stream that overflows from table 1 into table 2, etc.
//
//	tableCount   – number of side-by-side tables (1–5)
//	rowsPerTable – how many data rows fit vertically per table
//	offset       – index of the first visible item (shared across all tables)
//	cursor       – absolute item index of the selected row
func RenderListView(
	streamers []api.StreamerStatus,
	filter, searchBuf string,
	searchActive bool,
	cursor int,
	tableCount int,
	rowsPerTable int,
	offset int,
	liveColor, selectColor color.Color,
	width int,
	liveCount, totalCount int,
	elapsedSecs int64,
	nextRefreshAt time.Time,
) string {
	filtered := FilterStreamers(streamers, filter)
	items := SearchFilter(filtered, searchBuf)

	if tableCount < 1 {
		tableCount = 1
	}
	if rowsPerTable < 1 {
		rowsPerTable = 1
	}

	// ScreenStandard: global app header above; body sized by the caller via
	// rowsPerTable so the footer is always pinned to the terminal bottom.
	//   SubHeader = filter tabs + [search bar] + rule
	//   Body      = [↑ more] + header row + data rows + [↓ more]
	//   Footer    = rule + help bar   (FooterLines = 2)
	//
	// The ↑/↓ indicators and search bar are conditional but counted as fixed
	// in the caller's overhead calculation (listViewRowsPerTable) so the row
	// count always fits regardless of whether they actually appear.

	// --- Sub-header block ---
	var subHeaderParts []string
	subHeaderParts = append(subHeaderParts, renderFilterTabs(filter, liveCount, totalCount-liveCount, width, nextRefreshAt))
	if searchActive || searchBuf != "" {
		subHeaderParts = append(subHeaderParts, RenderSearchBar(searchBuf, len(items), width, searchActive, ""))
	}
	subHeaderParts = append(subHeaderParts, Rule(width))
	subHeaderBlock := strings.Join(subHeaderParts, "\n")

	// --- Body block ---
	var bodyLines []string
	if len(items) == 0 {
		msg := "No streamers match."
		if searchBuf != "" {
			msg = fmt.Sprintf("No results for %q.", searchBuf)
		}
		bodyLines = append(bodyLines, styles.Muted.PaddingLeft(2).Render(msg))
	} else {
		totalItems := len(items)

		// Per-table width: divide the full width evenly; tables are separated by " │ " (3 chars).
		separatorW := 3
		tableWidth := (width - (tableCount-1)*separatorW) / tableCount
		// 34 = minimum viable table width: numW(4)+sep(1)+dotW(2)+nameMin(8)+sep(1)+gameMin(6)+sep(1)+uptimeW(11)
		if tableWidth < 34 {
			tableWidth = 34
		}

		// ↑ more indicator — shown when the list is scrolled down.
		if offset > 0 {
			bodyLines = append(bodyLines, styles.Dim.PaddingLeft(2).Render("↑ more"))
		}

		// Column headers — same header block repeated for each table.
		headerCells := make([]string, tableCount)
		for t := 0; t < tableCount; t++ {
			headerCells[t] = renderListHeader(tableWidth)
		}
		bodyLines = append(bodyLines, joinTableCells(headerCells))

		// Data rows — one output line per row, all table cells joined horizontally.
		for row := 0; row < rowsPerTable; row++ {
			rowCells := make([]string, tableCount)
			for t := 0; t < tableCount; t++ {
				itemIdx := offset + t*rowsPerTable + row
				if itemIdx < totalItems {
					rowCells[t] = renderListRow(
						items[itemIdx], itemIdx == cursor,
						tableWidth, liveColor, selectColor,
						searchBuf, elapsedSecs,
					)
				} else {
					// Blank placeholder so the separator column stays aligned.
					rowCells[t] = strings.Repeat(" ", tableWidth)
				}
			}
			bodyLines = append(bodyLines, joinTableCells(rowCells))
		}

		// ↓ more indicator — shown when more items exist beyond the visible window.
		if offset+tableCount*rowsPerTable < totalItems {
			bodyLines = append(bodyLines, styles.Dim.PaddingLeft(2).Render("↓ more"))
		}
	}

	// --- Footer block ---
	footerBlock := strings.Join([]string{Rule(width), renderListHelp(searchActive || searchBuf != "")}, "\n")

	return ScreenStandard.Render("", subHeaderBlock, strings.Join(bodyLines, "\n"), footerBlock)
}

// ---------------------------------------------------------------------------
// Internal renderers
// ---------------------------------------------------------------------------

// joinTableCells concatenates per-table cell strings with a dim " │ " separator.
func joinTableCells(cells []string) string {
	sep := " " + styles.Dim.Render("│") + " "
	return strings.Join(cells, sep)
}

// listColWidths computes column widths that fit inside tableWidth display columns.
//
// Physical layout per row:
//
//	[numW] [1] [dotW=2] [nameW] [1] [gameW] [1] [uptimeW]
//	 num   sep   dot     name   sep   game   sep   uptime
//
// Three 1-char separators: num→dot, name→game, game→uptime.
// nameW and gameW share the remaining flexible space.
func listColWidths(tableWidth int) (numW, nameW, gameW, uptimeW int) {
	const (
		dotW     = 2 // "● " or "○ "
		sepTotal = 3 // three 1-char separators
	)
	numW    = 4  // right-aligned number (up to 3 digits) in 4 display cols
	uptimeW = 11 // "10h 05m 30s" — widest realistic uptime string
	// fixed = numW + 1(sep) + dotW + nameW(flex) + 1(sep) + gameW(flex) + 1(sep) + uptimeW
	//       = 4 + 1 + 2 + 1 + 1 + 11 = 20 (excluding nameW + gameW)
	fixed := numW + dotW + uptimeW + sepTotal // = 20
	remaining := tableWidth - fixed
	if remaining < 14 {
		remaining = 14
	}
	// Name gets ~40 % of the flexible space; game gets the rest.
	nameW = remaining * 40 / 100
	if nameW < 8 {
		nameW = 8
	}
	gameW = remaining - nameW
	if gameW < 6 {
		gameW = 6
	}
	return
}

// renderListHeader returns the column header row for one table.
func renderListHeader(tableWidth int) string {
	numW, nameW, gameW, uptimeW := listColWidths(tableWidth)
	h := styles.Dim
	// Right-align "#" within numW chars to match right-aligned numbers in data rows.
	num    := h.Width(numW).Render(fmt.Sprintf("%*s", numW, "#"))
	dot    := h.Width(2).Render("●") // dim dot header aligns with data row dots
	name   := h.Width(nameW).Render("Name")
	game   := h.Width(gameW).Render("Category")
	uptime := h.Width(uptimeW).Render("Uptime")
	sp     := h.Render(" ")
	return num + sp + dot + name + sp + game + sp + uptime
}

// renderListRow renders a single data row for one table column.
func renderListRow(
	item ListItem,
	selected bool,
	tableWidth int,
	liveColor, selectColor color.Color,
	searchBuf string,
	elapsedSecs int64,
) string {
	s := item.Streamer
	numW, nameW, gameW, uptimeW := listColWidths(tableWidth)

	// Background for live/selected rows — mirrors card view colour scheme.
	var bg color.Color
	if selected {
		bg = selectColor
	} else if s.IsLive {
		bg = liveColor
	}

	// bst bakes the background into any style so ANSI resets don't expose terminal bg.
	bst := func(st lipgloss.Style) lipgloss.Style {
		if bg != nil {
			return st.Copy().Background(bg)
		}
		return st
	}

	// ── Number column (numW chars) ────────────────────────────────────────────
	// Selected row shows a ▶ prefix; all others show a plain leading space.
	var numSt lipgloss.Style
	var numPrefix string
	if selected {
		numSt = bst(lipgloss.NewStyle().Foreground(styles.PurpleLight).Bold(true))
		numPrefix = "▶"
	} else {
		numSt = bst(styles.Dim)
		numPrefix = " "
	}
	numPart := numSt.Width(numW).Render(fmt.Sprintf("%s%3d", numPrefix, item.OriginalNum))

	// ── Status dot column (2 chars: dot + trailing space) ─────────────────────
	// Kept as its own column, separated from the number by an explicit space,
	// so the number and dot are never visually fused.
	var dotPart string
	if s.IsLive {
		dotPart = bst(styles.LiveBadge).Render("●") + bst(lipgloss.NewStyle()).Render(" ")
	} else {
		dotPart = bst(styles.OfflineBadge).Render("○") + bst(lipgloss.NewStyle()).Render(" ")
	}

	// ── Name column (nameW chars) ─────────────────────────────────────────────
	rawName := truncate(s.Username, nameW)
	var nameSt lipgloss.Style
	if selected || s.IsLive {
		nameSt = bst(lipgloss.NewStyle().Bold(true).Foreground(styles.ColorText))
	} else {
		nameSt = bst(lipgloss.NewStyle().Foreground(styles.ColorTextMuted))
	}
	namePart := nameSt.Width(nameW).Render(highlightMatch(rawName, searchBuf))

	// ── Game/Category column (gameW chars) ────────────────────────────────────
	var gamePart string
	if s.IsLive && s.Game != "" {
		gamePart = bst(styles.InfoGame).Width(gameW).Render(truncate(s.Game, gameW))
	} else {
		gamePart = bst(styles.Dim).Width(gameW).Render("—")
	}

	// ── Uptime column (uptimeW chars) ─────────────────────────────────────────
	var uptimePart string
	if s.IsLive && s.UptimeSec > 0 {
		currentSec := s.UptimeSec + elapsedSecs
		uptimeStr := util.FormatUptime(currentSec)
		uptimeSt := bst(lipgloss.NewStyle().Foreground(uptimeDurationColor(currentSec)))
		uptimePart = uptimeSt.Width(uptimeW).Render(uptimeStr)
	} else {
		uptimePart = bst(styles.Dim).Width(uptimeW).Render("—")
	}

	// Assembly: num [sp] dot name [sp] game [sp] uptime
	sp := bst(lipgloss.NewStyle()).Render(" ")
	return numPart + sp + dotPart + namePart + sp + gamePart + sp + uptimePart
}
