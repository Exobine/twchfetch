package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"

	"twchfetch/internal/logging"
	"twchfetch/internal/tui/styles"
	"twchfetch/internal/tui/views"
)

func (m Model) renderLoading(w int) string {
	spinLine := "  " + m.spinner.View() + "  " + styles.Bold.Render("Fetching streamer statuses…")
	barLine := ""
	if m.progressTotal > 0 {
		// Use the spring-smoothed position so the bar animates fluidly.
		bar := m.progressBar.ViewAs(m.progressSpringPos)
		barLine = "\n  " + bar + "  " + styles.Muted.Render(
			fmt.Sprintf("%d / %d batches", m.progressDone, m.progressTotal))
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(w),
		views.Rule(w),
		"",
		spinLine+barLine,
	)
}

// headerHeight returns the number of lines renderHeader will produce.
func (m Model) headerHeight() int {
	w := m.width
	if w == 0 {
		w = 80
	}
	if w >= titleArtWidth+4 {
		return 3
	}
	return 1
}

func (m Model) renderHeader(w int) string {
	authTag := ""
	if m.apiClient.HasAuth() {
		authTag = "  " + styles.StatusOK.Render("● authed")
	}

	if w >= titleArtWidth+4 {
		// 3-line unicode-art title, centered, middle row brighter
		l1 := lipgloss.PlaceHorizontal(w, lipgloss.Center, styles.AppTitle.Render(titleArt[0]))
		l2 := lipgloss.PlaceHorizontal(w, lipgloss.Center, styles.AppTitleMid.Render(titleArt[1]))
		l3 := lipgloss.PlaceHorizontal(w, lipgloss.Center, styles.AppTitle.Render(titleArt[2])+authTag)
		return l1 + "\n" + l2 + "\n" + l3
	}

	// Narrow fallback: single styled line
	inner := styles.AppTitle.Render("◈ twchfetch ◈") + authTag
	if w > 0 {
		inner = lipgloss.PlaceHorizontal(w, lipgloss.Center, inner)
	}
	return inner
}


func (m Model) visibleItems() []views.ListItem {
	filtered := views.FilterStreamers(m.streamers, m.listFilter)
	return views.SearchFilter(filtered, m.searchBuf)
}

// isListMode reports whether the list is currently in compact table mode.
func (m Model) isListMode() bool {
	return m.cfg.Display.DisplayMode == "list"
}

// gridNumCols returns the number of navigation columns for the current display mode.
// In list mode this is always 1 (items are navigated one at a time).
// In card mode it is the number of card columns.
func (m Model) gridNumCols() int {
	if m.isListMode() {
		return 1
	}
	if m.cfg.Display.CardColumns > 0 {
		return m.cfg.Display.CardColumns
	}
	if m.width <= 0 {
		return 3
	}
	// Cell width: 1 (accent bar) + padH*2 + textWidth + 1 (right-margin spacer)
	padH := m.cfg.Display.CardPadH
	cellW := 1 + padH*2 + m.cfg.Display.CardWidth + 1
	cols := m.width / cellW
	if cols < 1 {
		cols = 1
	}
	return cols
}

// listViewRowsPerTable returns how many list rows fit vertically per table.
func (m Model) listViewRowsPerTable() int {
	if m.height == 0 {
		return 5
	}
	// Screen zones for the list view (ScreenStandard):
	//   SubHeader: filterTabs(1) + rule(1)       = 2  (+ search bar when visible)
	//   Body:      ↑more(1) + headerRow(1) + rows + ↓more(1)
	//   Footer:    rule(1) + help(1)              = FooterLines (2)
	// The ↑/↓ indicators and search bar are counted as fixed overhead here for
	// worst-case sizing so rows never overflow even when all are shown.
	// overhead = headerHeight + filterTabs(1) + rule(1) + headerRow(1) + ↑more(1) + ↓more(1) + FooterLines(2) = header + 7
	overhead := m.headerHeight() + views.FooterLines + 5 // 5 = tabs+rule+headerRow+↑more+↓more
	if m.searchActive || m.searchBuf != "" {
		overhead++ // search bar line
	}
	avail := m.height - overhead
	if avail < 1 {
		return 1
	}
	return avail
}

// visibleGridRows returns the total number of visible items for scroll calculations.
// In list mode: rowsPerTable × tableCount.  In card mode: card rows that fit.
func (m Model) visibleGridRows() int {
	if m.height == 0 {
		return 3
	}
	if m.isListMode() {
		numTables := m.cfg.Display.ListTableCount
		if numTables < 1 {
			numTables = 1
		}
		return m.listViewRowsPerTable() * numTables
	}
	// Screen zones for the card view (ScreenStandard):
	//   SubHeader: filterTabs(1) + rule(1)       = 2  (+ search bar when visible)
	//   Body:      ↑more(1) + card rows + ↓more(1)
	//   Footer:    rule(1) + help(1)              = FooterLines (2)
	// overhead = headerHeight + filterTabs(1) + rule(1) + ↑more(1) + ↓more(1) + FooterLines(2) = header + 6
	overhead := m.headerHeight() + views.FooterLines + 4 // 4 = tabs+rule+↑more+↓more
	if m.searchActive || m.searchBuf != "" {
		overhead++ // search bar line
	}
	avail := m.height - overhead
	rows := avail / views.CardTerminalHeight(m.cfg.Display.CardPadV)
	if rows < 1 {
		rows = 1
	}
	return rows
}

// vodVisibleRows returns the number of VOD table rows that fit in the current
// terminal window for the VOD view. Used for scroll offset calculations and as
// the visible-slice limit passed to RenderVODs.
//
// Layout overhead (ScreenAdaptive):
//
//	headerHeight + SubHeaderLines(3) + headerRow(1) + rule(1)
//	+ ↑indicator(1) + ↓indicator(1) + trailingBlank(1) + FooterLines(2)
//	= headerHeight + 10
//
// Each VOD row renders as 1–2 terminal lines depending on wrapping; we use 2
// as the conservative worst-case so the body never overflows the terminal.
func (m Model) vodVisibleRows() int {
	if m.height == 0 {
		return 5
	}
	// SubHeaderLines(3) + headerRow(1) + rule(1) + ↑(1) + ↓(1) + trailingBlank(1) + FooterLines(2) = 10
	chrome := m.headerHeight() + 10
	avail := m.height - chrome
	if avail < 2 {
		return 1
	}
	rows := avail / 2
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m Model) selectStreamer(cursorIdx int) (Model, tea.Cmd) {
	items := m.visibleItems()
	if cursorIdx < 0 || cursorIdx >= len(items) {
		return m.showErrorDialog("No streamer selected")
	}
	username := items[cursorIdx].Streamer.Username
	logging.Debug("Streamer selected", "user", username)
	m.selectedUser = username
	m.view = viewDetails
	m.detailData = nil
	m.detailFollowers = 0
	m.lastSeen = nil

	if entry, ok := m.cache.Get(username); ok {
		logging.Debug("Details served from cache", "user", username)
		m.detailLoading = false
		m.detailData = entry.Details
		m.detailFollowers = entry.Followers
		m.lastSeen = entry.LastSeen
		return m, nil
	}
	m.detailLoading = true
	return m, fetchDetailsCmd(m.apiClient, username)
}
