package views

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"twchfetch/internal/api"
	"twchfetch/internal/tui/styles"
	"twchfetch/internal/util"
)

// categoryColorStyles defines the rotating palette for VOD category names.
// Index 0 matches the game colour used on the details page (InfoGame = PurpleLight).
var categoryColorStyles = []lipgloss.Style{
	styles.InfoGame,                                           // PurpleLight — same as details page
	styles.InfoUptime,                                         // Cyan/blue
	lipgloss.NewStyle().Foreground(styles.ColorYellow),        // Gold
	lipgloss.NewStyle().Foreground(styles.ColorLive),          // Green
	lipgloss.NewStyle().Foreground(styles.PurpleLight).Italic(true), // soft purple variant
}

// wrapColoredCategories wraps category names into at most maxLines lines using
// word-level wrapping. Each category name's words share a colour; the " → "
// separators between categories are plain (or bg-only when bg is set). Words
// within a multi-word category name (e.g. "Just Chatting") wrap naturally just
// like regular text. ANSI codes never span a line boundary.
//
// bg, when non-empty, is baked into every span so selection row highlights
// correctly behind coloured text and the arrow separators.
func wrapColoredCategories(names []string, width, maxLines int, bg color.Color) []string {
	const arrow = " → "

	if len(names) == 0 || width <= 0 {
		return []string{"—"}
	}

	bst := func(st lipgloss.Style) lipgloss.Style {
		if bg != nil {
			return st.Copy().Background(bg)
		}
		return st
	}

	type span struct {
		rawText  string
		rendered string
		isArrow  bool
	}

	// Build a flat span list: individual words (with their category colour) +
	// " → " arrow spans between categories.
	var spans []span
	for i, name := range names {
		words := strings.Fields(name)
		if len(words) == 0 {
			continue
		}
		colorSt := bst(categoryColorStyles[i%len(categoryColorStyles)])
		for _, w := range words {
			spans = append(spans, span{rawText: w, rendered: colorSt.Render(w)})
		}
		if i < len(names)-1 {
			arrowSt := bst(lipgloss.NewStyle())
			spans = append(spans, span{
				rawText:  arrow,
				rendered: arrowSt.Render(arrow),
				isArrow:  true,
			})
		}
	}

	if len(spans) == 0 {
		return []string{""}
	}

	spaceSt := bst(lipgloss.NewStyle())

	var lines []string
	curRendered := ""
	curW := 0
	prevWasWord := false // whether the last appended span was a word (not arrow)

	for _, sp := range spans {
		tokW := displayWidth(sp.rawText)

		if curRendered == "" {
			if sp.isArrow {
				// Never start a line with an arrow — skip it.
				continue
			}
			curRendered = sp.rendered
			curW = tokW
			prevWasWord = true
			continue
		}

		// A space is needed before a word span only when the previous span on
		// this line was also a word (arrows include their own surrounding spaces).
		spaceW := 0
		if !sp.isArrow && prevWasWord {
			spaceW = 1
		}

		if curW+spaceW+tokW <= width {
			if spaceW > 0 {
				curRendered += spaceSt.Render(" ")
			}
			curRendered += sp.rendered
			curW += spaceW + tokW
			prevWasWord = !sp.isArrow
		} else {
			// Doesn't fit — must wrap.
			if sp.isArrow {
				// Arrow at a wrap boundary: end the current line (the arrow
				// itself is omitted — the wrap visually implies continuation).
				lines = append(lines, curRendered)
				if len(lines) >= maxLines {
					return lines
				}
				curRendered = ""
				curW = 0
				prevWasWord = false
				continue
			}
			// Word that doesn't fit: flush current line, start a new one.
			lines = append(lines, curRendered)
			if len(lines) >= maxLines {
				return lines
			}
			curRendered = sp.rendered
			curW = tokW
			prevWasWord = true
		}
	}

	if curRendered != "" && len(lines) < maxLines {
		lines = append(lines, curRendered)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// VODColWidths holds the computed column widths for the VOD table.
type VODColWidths struct {
	Num, Date, Dur, Cat, Title int
}

// CalcVODColWidths computes weighted column widths from actual VOD content.
// totalWidth should already account for the 2-char accent prefix.
func CalcVODColWidths(vods []api.VOD, chapters map[string][]api.Chapter, maxCategories, totalWidth int) VODColWidths {
	const numW = 4
	const dateW = 12
	const durW = 8
	const gaps = 4
	const minCat = 16
	const minTitle = 22

	remaining := totalWidth - numW - dateW - durW - gaps
	if remaining < minCat+minTitle {
		return VODColWidths{numW, dateW, durW, minCat, max(minTitle, remaining-minCat)}
	}

	// Sample average content lengths
	sumCat, sumTitle := 0, 0
	for _, v := range vods {
		cat := util.BuildCategoryChain(chapters[v.ID], v.Game, maxCategories, 999)
		sumCat += displayWidth(cat)
		sumTitle += displayWidth(v.Title)
	}
	n := len(vods)
	if n == 0 {
		n = 1
	}
	avgCat := sumCat / n
	avgTitle := sumTitle / n

	if avgCat+avgTitle == 0 {
		catW := remaining * 35 / 100
		return VODColWidths{numW, dateW, durW, catW, remaining - catW}
	}

	total := avgCat + avgTitle
	catW := remaining * avgCat / total
	titleW := remaining - catW

	if catW < minCat {
		catW = minCat
		titleW = remaining - catW
	}
	if titleW < minTitle {
		titleW = minTitle
		catW = remaining - titleW
	}
	return VODColWidths{numW, dateW, durW, catW, titleW}
}

// RenderVODs renders the VOD list for a streamer with scroll support.
//
// vodOffset is the index of the topmost visible row; visibleRows is how many
// rows fit on screen. Only the slice [vodOffset, vodOffset+visibleRows) is
// rendered. A "Load More" entry is appended below the visible rows when
// hasMore or loadingMore is true and the entry falls within the visible window.
func RenderVODs(
	username string,
	vods []api.VOD,
	chapters map[string][]api.Chapter,
	loading bool,
	selectedIdx int,
	vodOffset int,
	visibleRows int,
	hasMore bool,
	loadingMore bool,
	numBuf string,
	maxCategories int,
	width int,
) string {
	// ScreenAdaptive: global app header above; footer floats immediately
	// below the body content rather than pinned to the terminal bottom.
	//   SubHeader = rule + streamer info + rule  (SubHeaderLines = 3)
	//   Body      = content rows (variable height, scroll-limited)
	//   Footer    = rule + hint bar              (FooterLines = 2)
	subHeader := styles.SubHeaderAccent.Render("▌") + " " +
		styles.SubHeader.Render(username) +
		styles.Muted.Render("  —  VODs")
	subHeaderBlock := strings.Join([]string{Rule(width), subHeader, Rule(width)}, "\n")
	escFooter := styles.HelpBar.Render(styles.HelpKey.Render("esc") + " " + styles.Dim.Render("back"))
	footerBlock := strings.Join([]string{Rule(width), escFooter}, "\n")

	if loading {
		body := strings.Join([]string{"", styles.Muted.PaddingLeft(2).Render("Loading VODs…"), ""}, "\n")
		return ScreenAdaptive.Render("", subHeaderBlock, body, footerBlock)
	}

	if len(vods) == 0 {
		body := strings.Join([]string{"", styles.Muted.PaddingLeft(2).Render("No archived VODs found."), ""}, "\n")
		return ScreenAdaptive.Render("", subHeaderBlock, body, footerBlock)
	}

	showLoadMore := hasMore || loadingMore
	totalEntries := len(vods)
	if showLoadMore {
		totalEntries++
	}

	// Clamp visible window to available entries.
	if visibleRows < 1 {
		visibleRows = 1
	}
	if vodOffset < 0 {
		vodOffset = 0
	}
	end := vodOffset + visibleRows
	if end > totalEntries {
		end = totalEntries
	}

	// Account for the 2-char accent prefix when computing column widths.
	// Always compute from the full vod list for consistent column sizing.
	cw := CalcVODColWidths(vods, chapters, maxCategories, width-2)

	var bodyLines []string
	bodyLines = append(bodyLines, renderVODHeader(cw), Rule(width))

	// ↑ scroll indicator
	if vodOffset > 0 {
		bodyLines = append(bodyLines, styles.Muted.PaddingLeft(2).Render(fmt.Sprintf("↑ %d more above", vodOffset)))
	}

	// Visible entries
	for i := vodOffset; i < end; i++ {
		if i < len(vods) {
			v := vods[i]
			selected := i == selectedIdx
			chs := chapters[v.ID]
			names := util.BuildCategoryNames(chs, v.Game, maxCategories)
			bodyLines = append(bodyLines, renderVODRow(i+1, v, names, cw, selected))
		} else {
			// Load More entry (index == len(vods))
			bodyLines = append(bodyLines, renderLoadMoreEntry(selectedIdx == len(vods), loadingMore, width))
		}
	}

	// ↓ scroll indicator
	if end < totalEntries {
		bodyLines = append(bodyLines, styles.Muted.PaddingLeft(2).Render(fmt.Sprintf("↓ %d more below", totalEntries-end)))
	}

	bodyLines = append(bodyLines, "") // trailing blank before footer

	loadMoreSelected := showLoadMore && selectedIdx == len(vods)
	footerBlock = strings.Join([]string{Rule(width), renderVODHelp(numBuf, loadMoreSelected)}, "\n")
	return ScreenAdaptive.Render("", subHeaderBlock, strings.Join(bodyLines, "\n"), footerBlock)
}

// ---------------------------------------------------------------------------
// Internal renderers
// ---------------------------------------------------------------------------

func renderVODHeader(cw VODColWidths) string {
	// 2 spaces for accent prefix alignment
	return "  " + lipgloss.JoinHorizontal(lipgloss.Left,
		styles.ColHeader.Width(cw.Num).Render(" #"),
		styles.ColHeader.Width(cw.Date).Render("Date"),
		styles.ColHeader.Width(cw.Dur).Render("Dur"),
		styles.ColHeader.Width(cw.Cat).Render("Categories"),
		styles.ColHeader.Width(cw.Title).Render("Title"),
	)
}

// renderVODRow renders a single VOD card row with accent glyph.
func renderVODRow(num int, v api.VOD, catNames []string, cw VODColWidths, selected bool) string {
	// Accent glyph
	var accentGlyph string
	if selected {
		accentGlyph = styles.CardAccentSelected.Render("▌")
	} else {
		accentGlyph = styles.CardAccentOffline.Render("╎")
	}

	numStr := fmt.Sprintf("%d", num)
	dateStr := util.FormatDate(v.CreatedAt)
	durStr := util.FormatDuration(v.LengthSeconds, false)
	durColor := uptimeDurationColor(int64(v.LengthSeconds))

	// Wrap categories and title to their column widths (max 2 lines each).
	// Pass the selection background so category colours and arrows are correctly
	// highlighted on the selected row.
	var catBg color.Color
	if selected {
		catBg = styles.PurpleDim
	}
	catLines := wrapColoredCategories(catNames, cw.Cat-1, 2, catBg)
	titleLines := wrapWords(v.Title, cw.Title-1, 2)

	height := max(len(catLines), len(titleLines))
	catLines = padLines(catLines, height)
	titleLines = padLines(titleLines, height)

	// Choose styles
	var rowSt, mutedSt lipgloss.Style
	var numSt lipgloss.Style
	if selected {
		rowSt = styles.CardRowSelected
		mutedSt = styles.CardRowSelected
		numSt = styles.RowNumSelected
	} else {
		rowSt = styles.CardRowOffline
		mutedSt = styles.RowNum
		numSt = styles.RowNum
	}

	numCellFull := numSt.Width(cw.Num).Render(fmt.Sprintf(" %s", numStr))
	numCellBlank := numSt.Width(cw.Num).Render("")

	var rowLines []string
	for i := 0; i < height; i++ {
		var numPart, datePart, durPart string
		if i == 0 {
			numPart = numCellFull
			datePart = mutedSt.Width(cw.Date).Render(dateStr)
			durPart = mutedSt.Width(cw.Dur).Render(lipgloss.NewStyle().Foreground(durColor).Render(durStr))
		} else {
			numPart = numCellBlank
			datePart = mutedSt.Width(cw.Date).Render("")
			durPart = mutedSt.Width(cw.Dur).Render("")
		}

		catPart := rowSt.Width(cw.Cat).Render(catLines[i])
		titlePart := rowSt.Width(cw.Title).Render(titleLines[i])

		inner := lipgloss.JoinHorizontal(lipgloss.Left, numPart, datePart, durPart, catPart, titlePart)

		var prefix string
		if i == 0 {
			prefix = accentGlyph + " "
		} else {
			prefix = "  "
		}
		rowLines = append(rowLines, prefix+inner)
	}
	return strings.Join(rowLines, "\n")
}

// renderLoadMoreEntry renders the "Load More" dummy entry at the bottom of the
// VOD list. It uses the same accent glyph and row style as a regular VOD row
// so it fits visually within the table.
func renderLoadMoreEntry(selected, loading bool, width int) string {
	var accentGlyph string
	if selected {
		accentGlyph = styles.CardAccentSelected.Render("▌")
	} else {
		accentGlyph = styles.CardAccentOffline.Render("╎")
	}

	var label string
	if loading {
		label = "Loading more VODs…"
	} else {
		label = "  Load more VODs"
	}

	var rowSt lipgloss.Style
	if selected {
		rowSt = styles.CardRowSelected
	} else {
		rowSt = styles.CardRowOffline
	}

	// inner width = total width minus the 2-char "▌ " prefix
	inner := rowSt.Width(width - 2).Render(label)
	return accentGlyph + " " + inner
}

// renderVODHelp renders the help bar.
// When loadMoreSelected is true the hint adapts to show the Load More action.
func renderVODHelp(numBuf string, loadMoreSelected bool) string {
	if loadMoreSelected {
		parts := []string{
			hintItem("↑↓", "navigate"),
			hintItem("enter", "load more"),
			hintItem("s", "settings"),
			hintItem("esc", "back"),
		}
		return styles.HelpBar.Render(strings.Join(parts, "   "))
	}
	parts := []string{hintItem("↑↓", "navigate"), hintItem("enter", "play")}
	if numBuf != "" {
		parts = append(parts, styles.Accent.Render("→ #"+numBuf), hintItem("c", "copy #"+numBuf))
	} else {
		parts = append(parts, hintItem("c", "copy url"), hintItem("Nc", "copy #N"))
	}
	parts = append(parts, hintItem("s", "settings"), hintItem("esc", "back"))
	return styles.HelpBar.Render(strings.Join(parts, "   "))
}

// VODUrl returns the Twitch VOD URL.
func VODUrl(vodID string) string {
	return fmt.Sprintf("https://www.twitch.tv/videos/%s", vodID)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// wrapWords word-wraps text to fit within width runes, up to maxLines lines.
func wrapWords(text string, width, maxLines int) []string {
	if width <= 0 || text == "" {
		return []string{""}
	}
	if displayWidth(text) <= width {
		return []string{text}
	}
	words := strings.Fields(text)
	var lines []string
	current := ""

	for _, word := range words {
		wl := displayWidth(word)
		if current == "" {
			if wl >= width {
				lines = append(lines, truncate(word, width))
				if len(lines) >= maxLines {
					return lines
				}
				continue
			}
			current = word
		} else if displayWidth(current)+1+wl <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			if len(lines) >= maxLines {
				return lines
			}
			if wl >= width {
				lines = append(lines, truncate(word, width))
				if len(lines) >= maxLines {
					return lines
				}
				current = ""
			} else {
				current = word
			}
		}
	}
	if current != "" && len(lines) < maxLines {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// padLines extends a slice to height with empty strings.
func padLines(lines []string, height int) []string {
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

