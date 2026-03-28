package views

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"twchfetch/internal/irc"
	"twchfetch/internal/tui/styles"
)

// darkenColor returns a lipgloss color that is approximately 35% darker than the hex string c.
// c must be a 7-character hex string in the form "#RRGGBB"; any other format
// is returned unchanged.  Used to tint /me action body text with the sender's
// colour at reduced brightness so it reads as "their colour, but subdued".
func darkenColor(c string) color.Color {
	hex := c
	if len(hex) != 7 || hex[0] != '#' {
		return lipgloss.Color(c)
	}
	r, err1 := strconv.ParseUint(hex[1:3], 16, 8)
	g, err2 := strconv.ParseUint(hex[3:5], 16, 8)
	b, err3 := strconv.ParseUint(hex[5:7], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return lipgloss.Color(c)
	}
	const factor = 0.65
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X",
		uint8(float64(r)*factor),
		uint8(float64(g)*factor),
		uint8(float64(b)*factor),
	))
}

// RenderChat renders the full chat screen around the pre-built viewport.
// logging controls the REC indicator appearance in the header.
// unread is the count of messages received while the user is scrolled up;
// when > 0 a notification is shown to the left of the REC indicator.
// Chat scroll mode — cycled by the "h" key.  These are the authoritative
// definitions; the tui model package references views.ScrollMode* directly.
const (
	ScrollModeLive   = 0 // always jump to the latest message (default)
	ScrollModeFrozen = 1 // viewport holds position; new messages queue below; manual scroll works
	ScrollModeFollow = 2 // viewport advances by the delta of each new message; manual scroll works
)

// Chat send mode — toggled by the "m" key.  Also authoritative; referenced by
// the tui model package as views.ChatMode*.
const (
	ChatModeLurk   = 0 // read-only; no PRIVMSG sent (default)
	ChatModeNormal = 1 // authenticated send enabled
)

func RenderChat(
	channel string,
	roomState irc.RoomState,
	connected, loading bool,
	logging bool,
	recBlink bool, // true = dot ON (red), false = dot OFF (dim); toggled by model tick only
	unread int,
	scrollMode int,
	streamerOnline bool,
	textBadges bool,
	searchActive bool,
	searchBuf string,
	searchCount int,
	chatMode int,
	inputActive bool,
	inputBuf string,
	vp viewport.Model,
	width int,
) string {
	// --- Connection status ---
	var status string
	if loading {
		status = styles.ChatStatusLine.Render("connecting…")
	} else if connected {
		status = styles.ChatConnected.Render("● connected")
	} else {
		status = styles.ChatDisconnected.Render("○ disconnected")
	}

	// --- ROOMSTATE tags ---
	// Each tag is rendered individually so the scroll-mode indicators can carry
	// their own distinct style while ROOMSTATE tags share ChatRoomTag.
	var roomParts []string
	switch scrollMode {
	case ScrollModeFrozen:
		roomParts = append(roomParts, styles.ChatRoomTagPaused.Render("[paused]"))
	case ScrollModeFollow:
		roomParts = append(roomParts, styles.ChatRoomTagPaused.Render("[paused ↓]"))
	}
	if !streamerOnline {
		roomParts = append(roomParts, styles.ChatRoomTag.Render("[offline]"))
	}
	if roomState.EmoteOnly {
		roomParts = append(roomParts, styles.ChatRoomTag.Render("[emote-only]"))
	}
	if roomState.SubsOnly {
		roomParts = append(roomParts, styles.ChatRoomTag.Render("[sub-only]"))
	}
	if roomState.SlowMode > 0 {
		roomParts = append(roomParts, styles.ChatRoomTag.Render(fmt.Sprintf("[slow: %ds]", roomState.SlowMode)))
	}
	if roomState.FollowersOnly >= 0 {
		if roomState.FollowersOnly == 0 {
			roomParts = append(roomParts, styles.ChatRoomTag.Render("[followers-only]"))
		} else {
			roomParts = append(roomParts, styles.ChatRoomTag.Render("[followers: "+formatFollowDuration(roomState.FollowersOnly)+"]"))
		}
	}
	roomStr := ""
	if len(roomParts) > 0 {
		roomStr = "  " + strings.Join(roomParts, "  ")
	}

	// --- Mode badge (lurk / send) ---
	var modeBadge string
	if chatMode == ChatModeNormal {
		modeBadge = "  " + styles.ChatModeBadgeNormal.Render("● send")
	} else {
		modeBadge = "  " + styles.ChatModeBadgeLurk.Render("lurk")
	}

	// --- Left portion of the header line ---
	leftHeader := styles.SubHeaderAccent.Render("▌") + " " +
		styles.SubHeader.Render(channel) +
		"  —  " + styles.ChatStatusLine.Render("chat") +
		"  " + status + modeBadge + roomStr

	// --- Right side: optional unread counter + always-visible REC indicator ---
	//
	// Not recording: dim grey "REC" with no dot.
	// Recording:     red "REC" with a dot that pulses dim↔red every second.
	var recIndicator string
	if logging {
		dotStyle := styles.Dim // dot OFF frame
		if recBlink {
			dotStyle = styles.ChatLogging // dot ON frame (red)
		}
		recIndicator = dotStyle.Render("●") + " " + styles.ChatLogging.Render("REC")
	} else {
		recIndicator = styles.Dim.Render("REC")
	}

	// Unread counter shown when the user is scrolled up and new messages arrived.
	var unreadStr string
	if unread > 0 {
		unreadStr = styles.Dim.Render(fmt.Sprintf("⏬ %d new", unread)) + "  "
	}

	// Right-align: left header | spacer | unread | REC
	leftW := lipgloss.Width(leftHeader)
	rightW := lipgloss.Width(unreadStr) + lipgloss.Width(recIndicator)
	spacerW := width - leftW - rightW
	if spacerW < 1 {
		spacerW = 1
	}
	subHeader := leftHeader + strings.Repeat(" ", spacerW) + unreadStr + recIndicator

	// --- Bottom bar priority: input bar > search > help ---
	var bottomBar string
	switch {
	case inputActive || (chatMode == ChatModeNormal && inputBuf != ""):
		bottomBar = renderChatInputBar(inputBuf, width, inputActive)
	case searchActive || searchBuf != "":
		bottomBar = RenderSearchBar(searchBuf, searchCount, width, searchActive, "enter keep  esc clear")
	default:
		xLabel := "log"
		if logging {
			xLabel = "stop log"
		}
		var hLabel string
		switch scrollMode {
		case ScrollModeFrozen:
			hLabel = "follow new"
		case ScrollModeFollow:
			hLabel = "resume"
		default:
			hLabel = "pause"
		}
		var mLabel string
		if chatMode == ChatModeNormal {
			mLabel = "lurk"
		} else {
			mLabel = "send mode"
		}
		helpParts := []string{
			hintItem("↑↓/jk", "scroll"),
			hintItem("f/b", "page"),
			hintItem("g", "top"),
			hintItem("G", "bottom"),
			hintItem("h", hLabel),
			hintItem("/", "search"),
			hintItem("m", mLabel),
		}
		if chatMode == ChatModeNormal {
			helpParts = append(helpParts, hintItem("shift+enter", "type"))
		}
		helpParts = append(helpParts,
			hintItem("c", "reconnect"),
			hintItem("x", xLabel),
			hintItem("esc", "back"),
		)
		bottomBar = styles.HelpBar.Render(strings.Join(helpParts, "   "))
	}

	// ScreenMinimal: no global app header — sub-header pinned to top,
	// viewport body fills all space between, footer pinned to bottom.
	//   SubHeader = rule + channel info + rule  (SubHeaderLines = 3)
	//   Body      = pre-sized viewport
	//   Footer    = rule + input/help bar       (FooterLines = 2)
	subHeaderBlock := strings.Join([]string{Rule(width), subHeader, Rule(width)}, "\n")
	footerBlock := strings.Join([]string{Rule(width), bottomBar}, "\n")
	return ScreenMinimal.Render("", subHeaderBlock, vp.View(), footerBlock)
}

// renderChatInputBar renders the teal chat compose bar shown in normal mode.
// cursor is a blinking block rendered only when the bar is focused (inputActive).
func renderChatInputBar(buf string, width int, focused bool) string {
	cursor := ""
	if focused {
		cursor = styles.ChatInputIcon.Render("█")
	}
	prompt := styles.ChatInputIcon.Render("> ")
	inner := prompt + buf + cursor
	// Pad inner content to fill the width inside the bar style.
	innerW := lipgloss.Width(inner)
	// Account for the 1-col left padding applied by ChatInputBarStyle.
	target := width - 2
	if target < 1 {
		target = 1
	}
	if innerW < target {
		inner += strings.Repeat(" ", target-innerW)
	}
	return styles.ChatInputBarStyle.Width(width).Render(inner)
}

// hasNonASCII reports whether s contains any byte above 0x7F (i.e. a non-ASCII
// character).  Used to detect localised Twitch display names that include
// characters from non-Latin scripts (CJK, Cyrillic, Arabic, etc.).
func hasNonASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7F {
			return true
		}
	}
	return false
}

// BuildChatContent converts the current message buffer into a single string
// suitable for viewport.Model.SetContent.  Each rendered message is
// word-wrapped to viewportWidth so the viewport never overflows its height.
// hexToANSIBg converts a "#RRGGBB" hex color to an ANSI background escape
// sequence. Returns "" for an empty or malformed input (disables the feature).
func hexToANSIBg(hex string) string {
	hex = strings.TrimSpace(hex)
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return ""
	}
	r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
	g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
	b, err3 := strconv.ParseUint(hex[4:6], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

func BuildChatContent(messages []irc.ChatMessage, emoteColors, showReply, trimReplyMention, localizedNames, textBadges bool, altRowColor string, viewportWidth int, searchQuery Node, thirdPartyEmotes map[string]struct{}, thirdPartyShading bool) string {
	if len(messages) == 0 {
		return styles.ChatStatusLine.Render("  Waiting for messages…")
	}
	altRowBg := hexToANSIBg(altRowColor)
	lines := make([]string, 0, len(messages))
	for _, msg := range messages {
		if !MatchChatMessage(msg, searchQuery) {
			continue
		}
		rendered := renderMessage(msg, emoteColors, showReply, trimReplyMention, localizedNames, textBadges, thirdPartyEmotes, thirdPartyShading)
		if viewportWidth > 4 {
			// Soft-wrap at word boundaries.  We wrap at viewportWidth-2 to
			// leave a 2-column safety buffer for ambiguous-width characters
			// (e.g. box-drawing glyphs, certain symbols) whose terminal
			// rendering width may exceed what ansi.StringWidth measures.
			wrapAt := viewportWidth - 2
			if wrapAt < 4 {
				wrapAt = 4
			}
			rendered = ansi.Wrap(rendered, wrapAt, " ")
			// Per-line hard backstop: clamp any line that still measures over
			// viewportWidth-1 columns.  This catches remaining measurement
			// disagreements (e.g. font-fallback glyphs rendering wider than
			// their Unicode Neutral width) and prevents the terminal from
			// hard-wrapping a line and adding an unexpected visual row.
			limit := viewportWidth - 1
			parts := strings.Split(rendered, "\n")
			for i, part := range parts {
				if ansi.StringWidth(part) > limit {
					parts[i] = ansi.Truncate(part, limit, "")
				}
			}
			rendered = strings.Join(parts, "\n")
		}
		// Alternating row tint: every visible odd-indexed message regardless of
		// type or content. We inject the background ANSI sequence directly and
		// re-inject it after every reset code so it persists through the inner
		// resets that coloured username/badge spans emit.
		if altRowBg != "" && len(lines)%2 == 1 {
			const reset = "\x1b[m"
			msgLines := strings.Split(rendered, "\n")
			for i, l := range msgLines {
				l = strings.ReplaceAll(l, "\x1b[m", "\x1b[m"+altRowBg)
				l = strings.ReplaceAll(l, "\x1b[0m", "\x1b[0m"+altRowBg)
				w := ansi.StringWidth(l)
				if viewportWidth > 0 && w < viewportWidth {
					l += strings.Repeat(" ", viewportWidth-w)
				}
				msgLines[i] = altRowBg + l + reset
			}
			rendered = strings.Join(msgLines, "\n")
		}
		lines = append(lines, rendered)
	}
	return strings.Join(lines, "\n")
}

// newChatTarget builds a SearchTarget for a chat message.
// Badge names and the system-kind label are included in the Badge field so
// searches like "badge:mod" or "badge:raid" work without matching on text.
func newChatTarget(msg irc.ChatMessage) SearchTarget {
	user := strings.ToLower(msg.DisplayName + " " + msg.Username)
	msgText := strings.ToLower(msg.Text)
	var badgeParts []string
	for _, b := range msg.Badges {
		badgeParts = append(badgeParts, string(b))
	}
	if msg.IsSystem {
		if label := systemKindLabel(msg.SystemKind); label != "" {
			badgeParts = append(badgeParts, label)
		}
	}
	badge := strings.Join(badgeParts, " ")
	return SearchTarget{
		User:  user,
		Msg:   msgText,
		Badge: badge,
		All:   user + " " + msgText + " " + badge,
	}
}

// MatchChatMessage reports whether msg satisfies node.
// Always returns true for a nil node (no filter active).
func MatchChatMessage(msg irc.ChatMessage, node Node) bool {
	if node == nil {
		return true
	}
	return MatchNode(node, newChatTarget(msg))
}

// systemKindLabel returns a short searchable keyword for each SystemKind.
func systemKindLabel(kind irc.SystemKind) string {
	switch kind {
	case irc.SystemKindSub:
		return "sub"
	case irc.SystemKindGift:
		return "gift"
	case irc.SystemKindGiftBomb:
		return "gift"
	case irc.SystemKindRaid:
		return "raid"
	case irc.SystemKindRitual:
		return "ritual"
	case irc.SystemKindTimeout:
		return "timeout"
	case irc.SystemKindBan:
		return "ban"
	case irc.SystemKindClear:
		return "clear"
	case irc.SystemKindDelete:
		return "delete"
	default:
		return ""
	}
}

// MessagesLineCount returns the number of viewport lines the given messages
// occupy when rendered and word-wrapped at viewportWidth.  Used by
// rebuildChatViewportWithTrim to measure lines removed from the ring-buffer so
// the offset can be corrected without drifting toward stale content.
func MessagesLineCount(msgs []irc.ChatMessage, emoteColors, showReply, trimReplyMention, localizedNames, textBadges bool, altRowColor string, viewportWidth int, searchQuery Node, thirdPartyEmotes map[string]struct{}, thirdPartyShading bool) int {
	if len(msgs) == 0 {
		return 0
	}
	content := BuildChatContent(msgs, emoteColors, showReply, trimReplyMention, localizedNames, textBadges, altRowColor, viewportWidth, searchQuery, thirdPartyEmotes, thirdPartyShading)
	return strings.Count(content, "\n") + 1
}

// FormatLogLine formats a single ChatMessage as a plaintext log line.
// No ANSI codes — safe to write directly to a text file.
func FormatLogLine(msg irc.ChatMessage) string {
	if msg.IsSystem {
		// Log USERNOTICE-derived events (subs, gifts, raids, rituals) so the
		// record captures notable channel activity.  Mod-action notices
		// (bans, timeouts, clears, deletes) and generic server NOTICEs are
		// excluded — they are display-only and do not belong in the chat log.
		switch msg.SystemKind {
		case irc.SystemKindSub, irc.SystemKindGift, irc.SystemKindGiftBomb,
			irc.SystemKindRaid, irc.SystemKindRitual:
			ts := msg.Timestamp.Format("15:04:05")
			return fmt.Sprintf("[%s] *** %s", ts, msg.Text)
		}
		return ""
	}

	ts := msg.Timestamp.Format("15:04:05")

	// Reply prefix (plaintext)
	replyStr := ""
	if msg.ReplyTo != "" {
		replyStr = "↩ @" + msg.ReplyTo + " · "
	}

	// Action prefix for /me messages
	actionStr := ""
	if msg.IsAction {
		actionStr = "* "
	}

	// Badge annotations
	var badgeParts []string
	for _, b := range msg.Badges {
		badgeParts = append(badgeParts, "["+string(b)+"]")
	}
	badgeStr := ""
	if len(badgeParts) > 0 {
		badgeStr = strings.Join(badgeParts, "") + " "
	}

	if msg.IsAction {
		return fmt.Sprintf("[%s] %s%s%s%s %s", ts, replyStr, actionStr, badgeStr, msg.DisplayName, msg.Text)
	}
	return fmt.Sprintf("[%s] %s%s%s: %s", ts, replyStr, badgeStr, msg.DisplayName, msg.Text)
}

// ---------------------------------------------------------------------------
// Internal rendering helpers
// ---------------------------------------------------------------------------

// buildModPostfix returns the styled post-fix label for a moderated message.
// The label is wrapped in brackets and coloured by action type:
//
//	"banned"       → dark red
//	"timeout …"    → dark amber
//	"deleted"      → mid-dim grey
func buildModPostfix(action string) string {
	if action == "" {
		return ""
	}
	label := "[" + action + "]"
	switch {
	case action == "banned":
		return " " + styles.ChatModBan.Render(label)
	case strings.HasPrefix(action, "timeout"):
		return " " + styles.ChatModTimeout.Render(label)
	case action == "deleted":
		return " " + styles.ChatModDelete.Render(label)
	default:
		return " " + styles.Dim.Render(label)
	}
}

// systemKindStyle returns the lipgloss style used to render a system message
// line based on its SystemKind category.
func systemKindStyle(kind irc.SystemKind) lipgloss.Style {
	switch kind {
	case irc.SystemKindSub:
		return styles.ChatSystemSub
	case irc.SystemKindGift, irc.SystemKindGiftBomb:
		return styles.ChatSystemGift
	case irc.SystemKindRaid:
		return styles.ChatSystemRaid
	case irc.SystemKindRitual:
		return styles.ChatSystemRitual
	case irc.SystemKindTimeout:
		return styles.ChatModTimeout
	case irc.SystemKindBan:
		return styles.ChatModBan
	case irc.SystemKindDelete:
		return styles.ChatModDelete
	default:
		return styles.ChatStatusLine
	}
}

// renderMessage formats one chat line for the TUI viewport:
//
//	[↩ @parent ·] [badges] DisplayName[: | action text] [ [xN] ]
func renderMessage(msg irc.ChatMessage, emoteColors, showReply, trimReplyMention, localizedNames, textBadges bool, thirdPartyEmotes map[string]struct{}, thirdPartyShading bool) string {
	// System messages (server NOTICEs) are rendered as a dim italic line with
	// a bullet indicator — no username, no badges, no colon.
	if msg.IsSystem {
		if msg.Greyed {
			return renderStyledWords("• "+msg.Text, styles.ChatGreyed)
		}
		return renderStyledWords("• "+msg.Text, systemKindStyle(msg.SystemKind))
	}

	// Reply prefix — shown only when showReply is enabled.
	replyPrefix := ""
	if msg.ReplyTo != "" && showReply {
		replyPrefix = styles.ChatReply.Render("↩ @"+msg.ReplyTo+" ·") + " "
	}

	// Badge column — each badge is responsible for its own trailing gap so
	// per-glyph visual width differences are handled individually:
	//
	//   ⚔  (BadgeModerator, U+2694) renders with slightly more ink than other
	//       single-width glyphs; one extra space restores consistent rhythm.
	//   ⚡  (BadgeTurbo, U+26A1) is East-Asian-Width Ambiguous and renders as
	//       double-width in many terminal fonts; skipping its trailing space
	//       compensates for the extra column it already occupies.
	//   All other badges and all text-mode labels use a single space.
	badgePostSep := func(b irc.Badge) string {
		if textBadges {
			return " " // text labels are ASCII-width; uniform gap throughout
		}
		switch b {
		case irc.BadgeModerator:
			return "  " // one extra space for the visually wider ⚔ glyph
		case irc.BadgeTurbo:
			return "" // ⚡ occupies the gap itself; no separator needed
		default:
			return " "
		}
	}
	badgeStr := ""
	if len(msg.Badges) > 0 {
		var sb strings.Builder
		for _, b := range msg.Badges {
			r := renderBadge(b, textBadges)
			if r == "" {
				continue
			}
			sb.WriteString(r)
			sb.WriteString(badgePostSep(b))
		}
		badgeStr = sb.String()
	}

	// Username coloured with their Twitch colour; fall back to muted when unset.
	nameStyle := styles.ChatUsername
	if msg.Color != "" {
		nameStyle = nameStyle.Foreground(lipgloss.Color(msg.Color))
	} else {
		nameStyle = nameStyle.Foreground(styles.ColorTextMuted)
	}

	// Resolved display name.  When the display name contains non-ASCII
	// characters (CJK, Cyrillic, Arabic, …) and localizedNames is on, we
	// append the ASCII login alias in parentheses.  The alias is rendered at
	// reduced brightness so it reads as supplementary info rather than a
	// second name: "名前 (name123)" where "name123" is dimmer.
	// Brackets stay at full nameStyle colour; only the login inside is darkened.
	displayName := msg.DisplayName
	if displayName == "" {
		displayName = msg.Username
	}
	showAlias := localizedNames && msg.Username != "" &&
		msg.DisplayName != "" && hasNonASCII(msg.DisplayName)

	// Mod-action post-fix — present on greyed messages; empty for normal messages.
	modPostfix := buildModPostfix(msg.ModAction)

	// Greyed message: render username/badges normally but replace the message
	// body with dim grey text.  Emote/mention processing is skipped — the
	// content is cosmetically redacted by a moderator action.
	if msg.Greyed {
		var greyName string
		if showAlias {
			var aliasStyle lipgloss.Style
			if msg.Color != "" {
				aliasStyle = lipgloss.NewStyle().Foreground(darkenColor(msg.Color))
			} else {
				aliasStyle = styles.Dim
			}
			greyName = nameStyle.Render(displayName) +
				nameStyle.Render(" (") +
				aliasStyle.Render(msg.Username) +
				nameStyle.Render(")")
		} else {
			greyName = nameStyle.Render(displayName)
		}
		return replyPrefix + badgeStr + greyName + styles.Dim.Render(":") + " " +
			renderStyledWords(msg.Text, styles.ChatGreyed) + modPostfix
	}

	// Trim the leading @username from the message body when the reply prefix is
	// shown and the user has elected to strip the duplicate mention.
	msgText := msg.Text
	msgEmotes := msg.Emotes
	if msg.ReplyTo != "" && showReply && trimReplyMention {
		msgText, msgEmotes = trimLeadingMention(msgText, msgEmotes)
	}

	// Repeat-collapse suffix — dim [xN] appended when N consecutive identical
	// messages from the same user were merged into this one.
	// RepeatCount = number of extra occurrences; total = RepeatCount + 1.
	repeatSuffix := ""
	if msg.RepeatCount > 0 {
		repeatSuffix = styles.Dim.Render(fmt.Sprintf(" [x%d]", msg.RepeatCount+1))
	}

	// /me ACTION: italic name in the user's colour (no colon), followed by the
	// action body rendered segment-by-segment at ~35% reduced brightness.
	// Segment-level rendering is critical: applying an outer lipgloss.Render over
	// pre-ANSI text causes \x1b[0m resets from inner emote/mention styles to kill
	// the outer italic+colour after the first styled span.
	if msg.IsAction {
		var italicName string
		if showAlias {
			// Alias uses the same darkening factor as the /me body so it stays
			// visually consistent; brackets stay at full italic nameStyle.
			var aliasStyle lipgloss.Style
			if msg.Color != "" {
				aliasStyle = lipgloss.NewStyle().Italic(true).Foreground(darkenColor(msg.Color))
			} else {
				aliasStyle = styles.ChatAction
			}
			italicName = nameStyle.Italic(true).Render(displayName) +
				nameStyle.Italic(true).Render(" (") +
				aliasStyle.Render(msg.Username) +
				nameStyle.Italic(true).Render(")")
		} else {
			italicName = nameStyle.Italic(true).Render(displayName)
		}
		var actionBodyStyle lipgloss.Style
		if msg.Color != "" {
			actionBodyStyle = lipgloss.NewStyle().Italic(true).Foreground(darkenColor(msg.Color))
		} else {
			actionBodyStyle = styles.ChatAction // italic muted-grey fallback
		}
		body := renderActionBody(msgText, msgEmotes, emoteColors, actionBodyStyle, thirdPartyEmotes, thirdPartyShading, msg.Bits)
		return replyPrefix + badgeStr + italicName + " " + body + repeatSuffix
	}

	// Normal message — render body with combined emote + mention styling in one pass.
	text := renderTextWithSpans(msgText, msgEmotes, thirdPartyEmotes, emoteColors, thirdPartyShading, msg.Bits)

	var name string
	if showAlias {
		// Brackets at full nameStyle colour; login alias at ~35% reduced brightness.
		var aliasStyle lipgloss.Style
		if msg.Color != "" {
			aliasStyle = lipgloss.NewStyle().Foreground(darkenColor(msg.Color))
		} else {
			aliasStyle = styles.Dim
		}
		name = nameStyle.Render(displayName) +
			nameStyle.Render(" (") +
			aliasStyle.Render(msg.Username) +
			nameStyle.Render(")")
	} else {
		name = nameStyle.Render(displayName)
	}
	return replyPrefix + badgeStr + name + styles.Dim.Render(":") + " " + text + repeatSuffix
}

// renderBadge returns the styled badge representation for the given type.
// When textBadges is true the glyph is replaced with a bracketed label such
// as [Broadcaster] so users whose terminal fonts lack good symbol coverage get
// a readable, consistently-spaced fallback.  Icon glyphs are chosen for
// unambiguous single-column rendering (Na/N East-Asian Width):
//
//	● U+25CF  BLACK CIRCLE              — Na  Broadcaster
//	⚔ U+2694  CROSSED SWORDS            — N   Moderator   (slightly wider; +1 space added in assembly)
//	■ U+25A0  BLACK SQUARE              — Na  Subscriber
//	◆ U+25C6  BLACK DIAMOND             — Na  VIP
//	⚑ U+2691  BLACK FLAG                — N   Staff
//	⚡ U+26A1  HIGH VOLTAGE SIGN         — A   Turbo       (gap handled by skipping trailing space)
func renderBadge(b irc.Badge, textBadges bool) string {
	if textBadges {
		switch b {
		case irc.BadgeBroadcaster:
			return styles.ChatBadgeBroadcaster.Render("[Broadcaster]")
		case irc.BadgeModerator:
			return styles.ChatBadgeMod.Render("[Mod]")
		case irc.BadgeSubscriber:
			return styles.ChatBadgeSub.Render("[Sub]")
		case irc.BadgeVIP:
			return styles.ChatBadgeVIP.Render("[VIP]")
		case irc.BadgeStaff:
			return styles.ChatBadgeStaff.Render("[Staff]")
		case irc.BadgeTurbo:
			return styles.ChatBadgeTurbo.Render("[Turbo]")
		}
		return ""
	}
	switch b {
	case irc.BadgeBroadcaster:
		return styles.ChatBadgeBroadcaster.Render("●") // U+25CF BLACK CIRCLE — Na
	case irc.BadgeModerator:
		return styles.ChatBadgeMod.Render("⚔") // U+2694 CROSSED SWORDS — N
	case irc.BadgeSubscriber:
		return styles.ChatBadgeSub.Render("■") // U+25A0 BLACK SQUARE — Na
	case irc.BadgeVIP:
		return styles.ChatBadgeVIP.Render("◆") // U+25C6 BLACK DIAMOND — Na
	case irc.BadgeStaff:
		return styles.ChatBadgeStaff.Render("⚑") // U+2691 BLACK FLAG — N
	case irc.BadgeTurbo:
		return styles.ChatBadgeTurbo.Render("⚡") // U+26A1 HIGH VOLTAGE SIGN — A (gap via assembly)
	}
	return ""
}

// renderWithEmotes applies the emote style to each emote range in the message
// text. Non-emote segments pass through unchanged. Falls back to plain text
// on any malformed or out-of-range index to prevent panics.
func renderWithEmotes(text string, emotes []irc.EmoteRange) string {
	b := []byte(text)
	var sb strings.Builder
	pos := 0
	for _, e := range emotes {
		if e.Start < 0 || e.End >= len(b) || e.Start > e.End || e.Start < pos {
			return text // malformed — bail safely
		}
		// Defence-in-depth: ensure neither endpoint falls in the middle of a
		// multi-byte UTF-8 character.  A misaligned slice produces invalid UTF-8
		// that corrupts ANSI rendering and bubbletea's cursor tracking.
		if !utf8.RuneStart(b[e.Start]) {
			return text
		}
		if e.End+1 < len(b) && !utf8.RuneStart(b[e.End+1]) {
			return text
		}
		sb.WriteString(string(b[pos:e.Start]))
		sb.WriteString(styles.ChatEmote.Render(string(b[e.Start : e.End+1])))
		pos = e.End + 1
	}
	sb.WriteString(string(b[pos:]))
	return sb.String()
}

// renderActionBody renders the body of a /me action message segment-by-segment
// so that the bodyStyle (italic + darkened colour) survives across multiple
// inner emote and mention spans.
//
// Strategy: collect all emote byte-ranges and @mention byte-ranges from raw,
// sort them, then walk the string emitting:
//   - plain segments  → bodyStyle.Render(segment)
//   - emote segments  → styles.ChatEmote + italic Render(segment)
//   - mention segments → styles.ChatMention + italic Render(segment)
//
// Each segment gets its own Render call — no outer wrapper touches pre-ANSI
// bytes — so \x1b[0m resets from one span cannot bleed into the next.
func renderActionBody(raw string, emotes []irc.EmoteRange, emoteColors bool, bodyStyle lipgloss.Style, thirdPartyEmotes map[string]struct{}, thirdPartyShading bool, bits int) string {
	type spanKind int
	const (
		spanEmote      spanKind = iota
		spanMention    spanKind = iota
		spanThirdParty spanKind = iota
		spanCheer      spanKind = iota
	)
	type span struct {
		start, end int // byte offsets, half-open [start, end)
		kind       spanKind
		amount     int // bits amount for spanCheer; 0 for all other kinds
	}

	var spans []span

	// Collect native emote spans (byte-offsets already corrected by parseChatMessage).
	if emoteColors {
		b := []byte(raw)
		for _, e := range emotes {
			if e.Start < 0 || e.End >= len(b) || e.Start > e.End {
				continue
			}
			// Validate UTF-8 boundaries before trusting the range.
			if !utf8.RuneStart(b[e.Start]) {
				continue
			}
			if e.End+1 < len(b) && !utf8.RuneStart(b[e.End+1]) {
				continue
			}
			spans = append(spans, span{start: e.Start, end: e.End + 1, kind: spanEmote})
		}
	}

	// Collect third-party emote spans (word-boundary matching).
	if len(thirdPartyEmotes) > 0 {
		i := 0
		for i < len(raw) {
			if raw[i] == ' ' || raw[i] == '\t' {
				i++
				continue
			}
			j := i
			for j < len(raw) && raw[j] != ' ' && raw[j] != '\t' {
				j++
			}
			// Strip trailing punctuation before lookup so "Pog!" matches "Pog".
			// Only highlight the stripped portion; any trailing punctuation is
			// left as plain text.
			k := j
			for k > i && isEmoteTerminator(raw[k-1]) {
				k--
			}
			if k > i {
				if _, ok := thirdPartyEmotes[raw[i:k]]; ok {
					spans = append(spans, span{start: i, end: k, kind: spanThirdParty})
				}
			}
			i = j
		}
	}

	// Collect @mention spans.
	for i := 0; i < len(raw); {
		if raw[i] == '@' {
			j := i + 1
			for j < len(raw) && isUsernameChar(raw[j]) {
				j++
			}
			if j > i+1 {
				spans = append(spans, span{start: i, end: j, kind: spanMention})
				i = j
				continue
			}
		}
		i++
	}

	// Collect cheermote spans (only when the message carries bits).
	if bits > 0 {
		i := 0
		for i < len(raw) {
			if raw[i] == ' ' || raw[i] == '\t' {
				i++
				continue
			}
			j := i
			for j < len(raw) && raw[j] != ' ' && raw[j] != '\t' {
				j++
			}
			if amount, ok := parseCheermoteToken(raw[i:j]); ok {
				spans = append(spans, span{start: i, end: j, kind: spanCheer, amount: amount})
			}
			i = j
		}
	}

	if len(spans) == 0 {
		return renderStyledWords(raw, bodyStyle)
	}

	// Sort by start position so we can walk the string once.
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })

	// Remove overlapping spans: keep the first one encountered; skip any span
	// whose start falls inside a previously accepted span's range.
	filtered := spans[:0]
	cursor := 0
	for _, s := range spans {
		if s.start >= cursor {
			filtered = append(filtered, s)
			if s.end > cursor {
				cursor = s.end
			}
		}
	}
	spans = filtered

	// Build the output, rendering each segment with its own style so that no
	// inner \x1b[0m reset can kill adjacent segments' styling.
	var sb strings.Builder
	pos := 0
	for _, s := range spans {
		if s.start > pos {
			sb.WriteString(renderStyledWords(raw[pos:s.start], bodyStyle))
		}
		seg := raw[s.start:s.end]
		switch s.kind {
		case spanEmote:
			// Native emote: muted grey + italic so it stays visually distinct
			// but still reads as part of the italic /me body.
			sb.WriteString(styles.ChatEmote.Italic(true).Render(seg))
		case spanThirdParty:
			// Third-party emote: amber or grey (when shading) + italic.
			if thirdPartyShading {
				sb.WriteString(styles.ChatEmote.Italic(true).Render(seg))
			} else {
				sb.WriteString(styles.ChatEmoteThirdParty.Italic(true).Render(seg))
			}
		case spanMention:
			sb.WriteString(styles.ChatMention.Italic(true).Render(seg))
		case spanCheer:
			sb.WriteString(cheerTierStyle(s.amount).Italic(true).Render(seg))
		}
		pos = s.end
	}
	if pos < len(raw) {
		sb.WriteString(renderStyledWords(raw[pos:], bodyStyle))
	}
	return sb.String()
}

// renderTextWithSpans renders a plain message body applying native emote,
// third-party emote, and @mention styles in a single left-to-right pass.
// This replaces the old sequential renderWithEmotes → highlightMentions chain
// so that all three span types are resolved against the original raw bytes and
// never interact with each other's ANSI escape codes.
func renderTextWithSpans(raw string, nativeEmotes []irc.EmoteRange, thirdPartyEmoteSet map[string]struct{}, emoteColors, thirdPartyShading bool, bits int) string {
	type spanKind int
	const (
		spanNative     spanKind = iota
		spanThirdParty spanKind = iota
		spanMention    spanKind = iota
		spanCheer      spanKind = iota
	)
	type span struct {
		start, end int
		kind       spanKind
		amount     int // bits amount for spanCheer; 0 for all other kinds
	}

	var spans []span

	// 1. Native Twitch emote ranges (server-supplied byte positions).
	if emoteColors {
		b := []byte(raw)
		for _, e := range nativeEmotes {
			if e.Start < 0 || e.End >= len(b) || e.Start > e.End {
				continue
			}
			if !utf8.RuneStart(b[e.Start]) {
				continue
			}
			if e.End+1 < len(b) && !utf8.RuneStart(b[e.End+1]) {
				continue
			}
			spans = append(spans, span{start: e.Start, end: e.End + 1, kind: spanNative})
		}
	}

	// 2. Third-party emote ranges (word-boundary matching, case-sensitive).
	if len(thirdPartyEmoteSet) > 0 {
		i := 0
		for i < len(raw) {
			if raw[i] == ' ' || raw[i] == '\t' {
				i++
				continue
			}
			j := i
			for j < len(raw) && raw[j] != ' ' && raw[j] != '\t' {
				j++
			}
			// Strip trailing punctuation before lookup so "Pog!" matches "Pog".
			// Only the stripped code portion is highlighted; any punctuation tail
			// is left as plain text.
			k := j
			for k > i && isEmoteTerminator(raw[k-1]) {
				k--
			}
			if k > i {
				if _, ok := thirdPartyEmoteSet[raw[i:k]]; ok {
					spans = append(spans, span{start: i, end: k, kind: spanThirdParty})
				}
			}
			i = j
		}
	}

	// 3. @mention ranges (always applied regardless of emote settings).
	if strings.ContainsRune(raw, '@') {
		for i := 0; i < len(raw); {
			if raw[i] == '@' {
				j := i + 1
				for j < len(raw) && isUsernameChar(raw[j]) {
					j++
				}
				if j > i+1 {
					spans = append(spans, span{start: i, end: j, kind: spanMention})
					i = j
					continue
				}
			}
			i++
		}
	}

	// 4. Cheermote ranges — only scanned when the message carries bits.
	// Each space-delimited token is tested with parseCheermoteToken; matching
	// tokens get a spanCheer so they are coloured by their individual amount.
	if bits > 0 {
		i := 0
		for i < len(raw) {
			if raw[i] == ' ' || raw[i] == '\t' {
				i++
				continue
			}
			j := i
			for j < len(raw) && raw[j] != ' ' && raw[j] != '\t' {
				j++
			}
			if amount, ok := parseCheermoteToken(raw[i:j]); ok {
				spans = append(spans, span{start: i, end: j, kind: spanCheer, amount: amount})
			}
			i = j
		}
	}

	if len(spans) == 0 {
		return raw
	}

	// Sort by start; first-wins de-overlap so no two spans share a byte.
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	filtered := spans[:0]
	cursor := 0
	for _, s := range spans {
		if s.start >= cursor {
			filtered = append(filtered, s)
			if s.end > cursor {
				cursor = s.end
			}
		}
	}
	spans = filtered

	var sb strings.Builder
	pos := 0
	for _, s := range spans {
		if s.start > pos {
			sb.WriteString(raw[pos:s.start])
		}
		seg := raw[s.start:s.end]
		switch s.kind {
		case spanNative:
			sb.WriteString(styles.ChatEmote.Render(seg))
		case spanThirdParty:
			if thirdPartyShading {
				sb.WriteString(styles.ChatEmote.Render(seg))
			} else {
				sb.WriteString(styles.ChatEmoteThirdParty.Render(seg))
			}
		case spanMention:
			sb.WriteString(styles.ChatMention.Render(seg))
		case spanCheer:
			sb.WriteString(cheerTierStyle(s.amount).Render(seg))
		}
		pos = s.end
	}
	if pos < len(raw) {
		sb.WriteString(raw[pos:])
	}
	return sb.String()
}

// trimLeadingMention removes a leading "@username" (and one trailing space) from
// text and shifts all emote byte-offsets down by the same number of bytes so
// they remain valid against the trimmed string.
//
// Twitch clients automatically prepend "@ReplyTarget " at the start of a reply
// message body.  When the ↩ reply prefix is already displayed, this creates a
// visually redundant double-mention.  We strip it here so only one reference
// to the parent author is shown.
//
// The function matches any leading "@<username>" where username consists of
// the characters valid in a Twitch login (a-z, A-Z, 0-9, _).  It does NOT
// require the name to match msg.ReplyTo exactly — login names (lowercase) and
// display names (mixed case) should both be handled without a case-sensitive
// compare.
func trimLeadingMention(text string, emotes []irc.EmoteRange) (string, []irc.EmoteRange) {
	if len(text) == 0 || text[0] != '@' {
		return text, emotes
	}
	// Walk past the username characters after '@'.
	j := 1
	for j < len(text) && isUsernameChar(text[j]) {
		j++
	}
	if j == 1 {
		// '@' with no username chars following — leave unchanged.
		return text, emotes
	}
	// Skip one space separator if present.
	if j < len(text) && text[j] == ' ' {
		j++
	}
	// j is the number of bytes to drop from the front.
	trimmed := text[j:]

	// Shift emote byte-offsets down.  Emotes that started inside the trimmed
	// prefix (shouldn't occur — Twitch emotes are not @mentions) are discarded.
	var adjusted []irc.EmoteRange
	for _, e := range emotes {
		if e.Start < j {
			continue // within the trimmed prefix — drop
		}
		adjusted = append(adjusted, irc.EmoteRange{
			ID:    e.ID,
			Start: e.Start - j,
			End:   e.End - j,
		})
	}
	return trimmed, adjusted
}

// highlightMentions scans text for @username tokens and wraps each in
// ChatMention style.  Works on ASCII bytes (Twitch usernames are ASCII).
func highlightMentions(text string) string {
	if !strings.ContainsRune(text, '@') {
		return text // fast path — no @ present
	}
	var sb strings.Builder
	i := 0
	for i < len(text) {
		if text[i] == '@' {
			j := i + 1
			for j < len(text) && isUsernameChar(text[j]) {
				j++
			}
			if j > i+1 {
				// Non-empty token after @
				sb.WriteString(styles.ChatMention.Render(text[i:j]))
				i = j
				continue
			}
		}
		sb.WriteByte(text[i])
		i++
	}
	return sb.String()
}

// isUsernameChar returns true for characters valid in a Twitch username.
func isUsernameChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_'
}

// cheerTierStyle returns the lipgloss style matching the Twitch cheer tier for
// a given bit amount.  Tiers mirror Twitch's official colour palette exactly:
// grey (1), purple (100), teal (1 000), blue (5 000), red (10 000), gold (100 000).
func cheerTierStyle(amount int) lipgloss.Style {
	switch {
	case amount >= 100000:
		return styles.ChatCheer100k
	case amount >= 10000:
		return styles.ChatCheer10k
	case amount >= 5000:
		return styles.ChatCheer5k
	case amount >= 1000:
		return styles.ChatCheer1k
	case amount >= 100:
		return styles.ChatCheer100
	default:
		return styles.ChatCheer1
	}
}

// parseCheermoteToken checks whether word is a cheermote token — an
// alphabetic-only prefix immediately followed by a decimal digit suffix (e.g.
// "Cheer100", "PogChamp50", "uni200").  Returns the bit amount and true on a
// match; 0 and false otherwise.  Only called when the parent message has
// Bits > 0 so the cost of the scan is only paid on genuine cheer messages.
func parseCheermoteToken(word string) (int, bool) {
	if len(word) == 0 {
		return 0, false
	}
	// Find where the alphabetic prefix ends.
	k := 0
	for k < len(word) {
		c := word[k]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			k++
		} else {
			break
		}
	}
	// Must have a non-empty alpha prefix AND a non-empty digit suffix.
	if k == 0 || k == len(word) {
		return 0, false
	}
	// Suffix must be all digits.
	for _, c := range word[k:] {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(word[k:])
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// isEmoteTerminator reports whether b is a punctuation character that commonly
// trails an emote code when the sender types "Pog!" or "monkaS." — characters
// that are NOT part of the emote name and should be stripped before lookup.
func isEmoteTerminator(b byte) bool {
	switch b {
	case '.', ',', '!', '?', ':', ';', ')', '(', ']', '[', '"', '\'':
		return true
	}
	return false
}

// renderStyledWords renders text by applying style to each individual
// space-delimited word rather than wrapping the entire string in one Render
// call.  Spaces and tabs pass through unstyled so ansi.Wrap always breaks at
// a point where the ANSI state is already fully reset.  This prevents
// continuation lines — which the viewport may render as the first visible line
// after a scroll — from losing their styling because the opening escape
// sequence from an earlier line is out of view.
func renderStyledWords(text string, style lipgloss.Style) string {
	if text == "" {
		return ""
	}
	var sb strings.Builder
	i := 0
	for i < len(text) {
		// Spaces and tabs pass through raw — no style needed on separators.
		if text[i] == ' ' || text[i] == '\t' {
			sb.WriteByte(text[i])
			i++
			continue
		}
		// Collect a non-whitespace run (the word, including any ANSI content).
		j := i
		for j < len(text) && text[j] != ' ' && text[j] != '\t' {
			j++
		}
		sb.WriteString(style.Render(text[i:j]))
		i = j
	}
	return sb.String()
}

// formatFollowDuration converts a followers-only requirement in minutes to a
// compact human-readable string: minutes below 60, hours when evenly divisible,
// or "Xh Ym" for mixed values.
func formatFollowDuration(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	h := minutes / 60
	m := minutes % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}
