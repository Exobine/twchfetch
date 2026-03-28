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

// cardContentLines is the number of visible content lines per card.
// Line 1: status dot + username
// Line 2: current game/category (live only, blank for offline)
// Line 3: live duration (colored) + #N right-aligned
const cardContentLines = 3

// CardTerminalHeight returns the total terminal rows one card row occupies:
// content lines + top/bottom padding + 1 blank spacer row.
func CardTerminalHeight(padV int) int {
	return cardContentLines + padV*2 + 1
}

// RenderCardGrid renders the streamer list as a 2-D grid of borderless cards.
//
//   rowOffset   – index of the first visible ROW (0-based), may be fractional
//                 for spring-animated scroll; rendered as the nearest integer
//   logicalRow  – the actual logical row offset used for ↑/↓ more indicators
//   visibleRows – how many card rows fit in the available height
//   accentPos   – spring position [0…1] for selected-card accent colour pulse
func RenderCardGrid(
	streamers []api.StreamerStatus,
	filter, searchBuf string,
	searchActive bool,
	cursor int,
	cardInnerWidth, numCols int,
	cardPadH, cardPadV int,
	cardLiveColor, cardSelectColor color.Color,
	width int,
	liveCount, totalCount int,
	rowOffset, logicalRow, visibleRows int,
	accentPos float64,
	elapsedSecs int64,
	nextRefreshAt time.Time,
) string {
	filtered := FilterStreamers(streamers, filter)
	items := SearchFilter(filtered, searchBuf)

	// ScreenStandard: global app header above; body sized by the caller via
	// visibleRows so the footer is always pinned to the terminal bottom.
	//   SubHeader = filter tabs + [search bar] + rule
	//   Body      = [↑ more] + card rows + [↓ more]
	//   Footer    = rule + help bar   (FooterLines = 2)
	//
	// The ↑/↓ indicators and search bar are conditional but counted as fixed
	// in the caller's overhead calculation (visibleGridRows) so the row count
	// always fits regardless of whether they actually appear.

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
		startItem := rowOffset * numCols
		endItem := (rowOffset + visibleRows) * numCols
		if endItem > len(items) {
			endItem = len(items)
		}
		totalRows := (len(items) + numCols - 1) / numCols

		if logicalRow > 0 {
			bodyLines = append(bodyLines, styles.Dim.PaddingLeft(2).Render("↑ more"))
		}

		for rowStart := startItem; rowStart < endItem; rowStart += numCols {
			rowEnd := rowStart + numCols
			if rowEnd > len(items) {
				rowEnd = len(items)
			}
			var rowCards []string
			for i := rowStart; i < rowEnd; i++ {
				rowCards = append(rowCards, renderCard(items[i], i == cursor, accentPos, searchBuf, cardInnerWidth, cardPadH, cardPadV, cardLiveColor, cardSelectColor, elapsedSecs))
			}
			for len(rowCards) < numCols {
				rowCards = append(rowCards, renderEmptyCard(cardInnerWidth, cardPadH, cardPadV))
			}
			bodyLines = append(bodyLines, lipgloss.JoinHorizontal(lipgloss.Top, rowCards...))
		}

		if logicalRow+visibleRows < totalRows {
			bodyLines = append(bodyLines, styles.Dim.PaddingLeft(2).Render("↓ more"))
		}
	}

	// --- Footer block ---
	footerBlock := strings.Join([]string{Rule(width), renderListHelp(searchActive || searchBuf != "")}, "\n")

	return ScreenStandard.Render("", subHeaderBlock, strings.Join(bodyLines, "\n"), footerBlock)
}

// ---------------------------------------------------------------------------
// Card rendering
// ---------------------------------------------------------------------------

// renderCard builds a card from two independent lipgloss blocks:
//
//	[accentBar (1 col)] [body (padH + textWidth + padH cols)]  [rightMargin (1 col)]
//
// Every inner text piece carries Background(bg) via bst() so the fill colour
// persists behind text characters, not just in padding whitespace.
func renderCard(item ListItem, selected bool, accentPos float64, searchBuf string, innerWidth, padH, padV int, liveColor, selectColor color.Color, elapsedSecs int64) string {
	s := item.Streamer

	// ── Background based on state ─────────────────────────────────────────────
	var bg color.Color
	if selected {
		bg = selectColor
	} else if s.IsLive {
		bg = liveColor
	}

	bst := func(st lipgloss.Style) lipgloss.Style {
		if bg != nil {
			return st.Copy().Background(bg)
		}
		return st
	}

	// ── Accent bar colour: spring-lerped Purple↔PurpleLight when selected ─────
	var accentColor color.Color
	if selected {
		// Lerp Purple #9147FF (145,71,255) → PurpleLight #BF94FF (191,148,255)
		r := int(145 + accentPos*46)
		g := int(71 + accentPos*77)
		accentColor = lipgloss.Color(fmt.Sprintf("#%02X%02XFF", r, g))
	} else if s.IsLive {
		accentColor = styles.ColorLive
	} else {
		accentColor = styles.ColorTextDim
	}

	var accentChar string
	if selected {
		accentChar = "▌"
	} else if s.IsLive {
		accentChar = "│"
	} else {
		accentChar = "╎"
	}

	// ── Accent bar: Width(1) × cardHeight ─────────────────────────────────────
	cardHeight := cardContentLines + padV*2
	accentSt := lipgloss.NewStyle().
		Width(1).
		Height(cardHeight).
		Foreground(accentColor).
		Bold(selected)
	if bg != nil {
		accentSt = accentSt.Background(bg)
	}
	accentLines := strings.Repeat(accentChar+"\n", cardHeight-1) + accentChar
	accentBar := accentSt.Render(accentLines)

	// ── Card body ─────────────────────────────────────────────────────────────
	textWidth := innerWidth
	bodySt := lipgloss.NewStyle().
		Width(textWidth + padH*2).
		Height(cardHeight).
		PaddingLeft(padH).
		PaddingRight(padH).
		PaddingTop(padV).
		PaddingBottom(padV)
	if bg != nil {
		bodySt = bodySt.Background(bg)
	}

	// ── Line 1: status dot + username ─────────────────────────────────────────
	var dotRendered string
	if s.IsLive {
		dotRendered = bst(styles.LiveBadge).Render("●")
	} else {
		dotRendered = bst(styles.OfflineBadge).Render("○")
	}
	sep1 := bst(lipgloss.NewStyle()).Render(" ")

	nameAvail := textWidth - 3 // dot(1) + space(1) + right-buffer(1)
	if nameAvail < 4 {
		nameAvail = 4
	}
	rawName := truncate(s.Username, nameAvail)
	var nameSt lipgloss.Style
	if selected || s.IsLive {
		nameSt = bst(lipgloss.NewStyle().Bold(true).Foreground(styles.ColorText))
	} else {
		nameSt = bst(lipgloss.NewStyle().Foreground(styles.ColorTextMuted))
	}
	line1 := dotRendered + sep1 + nameSt.Render(highlightMatch(rawName, searchBuf))

	// ── Line 2: current game/category (live only) ─────────────────────────────
	var line2 string
	if s.IsLive && s.Game != "" {
		gameTrunc := truncate(s.Game, textWidth-1)
		line2 = bst(styles.InfoGame).Render(gameTrunc)
	}

	// ── Line 3: duration (left, time-colored) + #N (right) ───────────────────
	numStr := fmt.Sprintf("#%d", item.OriginalNum)
	var numSt lipgloss.Style
	if selected {
		numSt = bst(lipgloss.NewStyle().Foreground(styles.PurpleLight))
	} else {
		numSt = bst(styles.Dim)
	}
	numRendered := numSt.Render(numStr)

	var durationRendered string
	durationW := 0
	if s.IsLive && s.UptimeSec > 0 {
		// Extend GQL-snapshot uptime with local wall-clock elapsed since refresh.
		currentSec := s.UptimeSec + elapsedSecs
		uptimeStr := util.FormatUptime(currentSec)
		durColor := bst(lipgloss.NewStyle().Foreground(uptimeDurationColor(currentSec)))
		durationRendered = durColor.Render(uptimeStr)
		durationW = displayWidth(uptimeStr)
	}

	var wsOpts []lipgloss.WhitespaceOption
	if bg != nil {
		wsOpts = append(wsOpts, lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(bg)))
	}

	// Right-align the number within the remaining space after duration.
	rightAligned := lipgloss.PlaceHorizontal(
		textWidth-durationW, lipgloss.Right,
		numRendered,
		wsOpts...,
	)
	line3 := durationRendered + rightAligned

	body := bodySt.Render(strings.Join([]string{line1, line2, line3}, "\n"))

	// ── Assemble ──────────────────────────────────────────────────────────────
	card := lipgloss.JoinHorizontal(lipgloss.Top, accentBar, body)
	cardW := lipgloss.Width(card)
	spacer := strings.Repeat(" ", cardW+1)
	return card + " \n" + spacer
}

// renderEmptyCard returns a blank placeholder matching the dimensions of a
// rendered card, used to pad incomplete final rows so the grid stays aligned.
func renderEmptyCard(innerWidth, padH, padV int) string {
	totalW := 1 + (innerWidth + padH*2) + 1
	blank := strings.Repeat(" ", totalW)
	totalH := cardContentLines + padV*2 + 1
	rows := make([]string, totalH)
	for i := range rows {
		rows[i] = blank
	}
	return strings.Join(rows, "\n")
}
