package views

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"twchfetch/internal/api"
	"twchfetch/internal/tui/styles"
	"twchfetch/internal/util"
)

// RenderDetails renders the streamer details screen.
func RenderDetails(username string, details *api.StreamDetails, lastSeen *time.Time, loading bool, followers int, width int) string {
	channelURL := fmt.Sprintf("https://www.twitch.tv/%s", username)

	// Sub-header: streamer name below the rule
	subHeader := styles.SubHeaderAccent.Render("▌") + " " + styles.SubHeader.Render(username)

	var infoLines []string

	// Channel URL
	infoLines = append(infoLines, detailRow("Channel", styles.InfoURL.Render(channelURL)))

	// Width available for detail values: subtract label width (12) + padding (2) + left margin (1).
	valueWidth := width - 15
	if valueWidth < 20 {
		valueWidth = 20
	}

	if loading {
		infoLines = append(infoLines, detailRow("Status", styles.Muted.Render("fetching…")))
	} else if details != nil {
		uptimeSec, uptimeStr := util.ParseUptime(details.CreatedAt)
		infoLines = append(infoLines,
			detailRow("Status", styles.LiveBadge.Render("● LIVE")),
		)
		if followers > 0 {
			infoLines = append(infoLines, detailRow("Followers", styles.Muted.Render(formatFollowers(followers))))
		}
		infoLines = append(infoLines,
			detailRow("Title", styles.InfoTitle.Render(truncate(details.Title, valueWidth))),
			detailRow("Game", styles.InfoGame.Render(details.Game)),
			detailRow("Viewers", styles.InfoViewers.Render(formatViewers(details.ViewersCount))),
			detailRow("Uptime", lipgloss.NewStyle().Foreground(uptimeDurationColor(uptimeSec)).Render(uptimeStr)),
		)
	} else {
		infoLines = append(infoLines, detailRow("Status", styles.OfflineBadge.Render("○ offline")))
		if followers > 0 {
			infoLines = append(infoLines, detailRow("Followers", styles.Muted.Render(formatFollowers(followers))))
		}
		if lastSeen != nil {
			rel := util.FormatRelativeTime(*lastSeen)
			utc := lastSeen.UTC().Format("2006-01-02 15:04 UTC")
			infoLines = append(infoLines,
				detailRow("Last seen", styles.Muted.Render(rel+"  ("+utc+")")),
			)
		} else {
			infoLines = append(infoLines,
				detailRow("Last seen", styles.Dim.Render("unknown / no VODs")),
			)
		}
	}

	// Help bar
	var hintParts []string
	if details != nil {
		hintParts = append(hintParts, hintItem("p", "play stream"))
	}
	// Chat is always available — Twitch IRC channels persist whether or not the
	// streamer is live, so t is shown for both online and offline streamers.
	if !loading && username != "" {
		hintParts = append(hintParts, hintItem("t", "chat"))
	}
	hintParts = append(hintParts,
		hintItem("P", "play latest vod"),
		hintItem("v", "vods"),
		hintItem("c", "copy url"),
		hintItem("s", "settings"),
		hintItem("esc", "back"),
	)
	hints := styles.HelpBar.Render(strings.Join(hintParts, "   "))

	// ScreenAdaptive: global app header above; footer floats immediately
	// below the body content rather than pinned to the terminal bottom.
	//   SubHeader = rule + streamer info + rule  (SubHeaderLines = 3)
	//   Body      = blank + info rows + blank    (natural height)
	//   Footer    = rule + hint bar              (FooterLines = 2)
	subHeaderBlock := strings.Join([]string{Rule(width), subHeader, Rule(width)}, "\n")
	bodyBlock := strings.Join(append([]string{""}, append(infoLines, "")...), "\n")
	footerBlock := strings.Join([]string{Rule(width), hints}, "\n")
	return ScreenAdaptive.Render("", subHeaderBlock, bodyBlock, footerBlock)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func detailRow(label, value string) string {
	l := styles.DetailLabel.Render(label + ":")
	v := styles.DetailValue.Render(value)
	return lipgloss.JoinHorizontal(lipgloss.Top, l, v)
}

func formatViewers(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatFollowers(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

