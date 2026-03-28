package views

import (
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"

	"twchfetch/internal/config"
	"twchfetch/internal/tui/styles"
)

// KeyringTokenSentinel is the value model.go uses to signal "clear the keyring
// token".  It is never placed in the textinput — the field is left empty when
// a keyring token is active so the placeholder message is visible.  Only when
// the user explicitly presses ctrl+d does model.go set the token to this
// value, which config.Save then interprets as a deletion.
//
// The constant is intentionally unexportable-style named but exported so
// model.go can reference it without importing views in a circular manner.
const KeyringTokenSentinel = "\x01clear-token\x01"

// SettingsField IDs.
const (
	// General section
	FieldPlayerType = iota // "mpv" | "vlc"
	FieldPlayerPath
	FieldPlayerArgs
	FieldOAuthToken
	FieldStreamers
	FieldAutoRefresh
	FieldCacheOverride
	FieldDisplayMode // "cards" | "list"
	// Cards section
	FieldCardWidth
	FieldCardColumns
	FieldCardPadH
	FieldCardPadV
	FieldCardLiveColor
	FieldCardSelectColor
	// List section
	FieldListTables // side-by-side table count
	// Chat section
	FieldChatMaxMessages
	FieldChatEmoteColors
	FieldChatStripDingbats
	FieldChatMaxReconnects
	FieldChatShowReply
	FieldChatTrimReplyMention
	FieldChatCollapseRepeats
	FieldChatLocalizedNames
	FieldChatTextBadges
	FieldChatAltRows
	FieldChatThirdPartyEmotes
	FieldChatThirdPartyShading

	FieldCount
)

// SectionHelp is the index of the read-only Help tab.
const SectionHelp = 3

// SectionFields defines the four section slots.
// Section 0 (General) is static.
// Section 1 is mode-dependent — Cards or List, resolved via ActiveSectionFields.
// Section 2 (Chat) is always the same fields.
// Section 3 (Help) has no editable fields.
var SectionFields = [][]int{
	{FieldPlayerType, FieldPlayerPath, FieldPlayerArgs, FieldOAuthToken, FieldStreamers, FieldAutoRefresh, FieldCacheOverride, FieldDisplayMode},
	{}, // placeholder — content resolved dynamically via ActiveSectionFields
	{}, // placeholder — Chat section
	{}, // Help — no editable fields
}

// CardSectionFields and ListSectionFields are the two possible Section 1 layouts.
var CardSectionFields = []int{FieldCardWidth, FieldCardColumns, FieldCardPadH, FieldCardPadV, FieldCardLiveColor, FieldCardSelectColor}
var ListSectionFields = []int{FieldListTables}
var ChatSectionFields = []int{FieldChatMaxMessages, FieldChatEmoteColors, FieldChatStripDingbats, FieldChatMaxReconnects, FieldChatShowReply, FieldChatTrimReplyMention, FieldChatCollapseRepeats, FieldChatLocalizedNames, FieldChatTextBadges, FieldChatAltRows, FieldChatThirdPartyEmotes, FieldChatThirdPartyShading}

// ActiveSectionFields returns the correct field IDs for the given section.
// Section 0 is General (static). Section 1 is Cards or List (mode-dependent).
// Section 2 is Chat (always the same fields). Section 3 is Help (no fields).
func ActiveSectionFields(m SettingsModel, section int) []int {
	switch section {
	case 0:
		return SectionFields[0]
	case 1:
		if strings.ToLower(strings.TrimSpace(m.Fields[FieldDisplayMode].Value())) == "list" {
			return ListSectionFields
		}
		return CardSectionFields
	case 2:
		return ChatSectionFields
	case SectionHelp:
		return nil
	}
	return nil
}

// ActiveSectionName returns the tab label for the given section.
func ActiveSectionName(m SettingsModel, section int) string {
	switch section {
	case 0:
		return "General"
	case 1:
		if strings.ToLower(strings.TrimSpace(m.Fields[FieldDisplayMode].Value())) == "list" {
			return "List"
		}
		return "Cards"
	case 2:
		return "Chat"
	case SectionHelp:
		return "Help"
	}
	return "?"
}

// SettingsModel holds the editable form state.
type SettingsModel struct {
	Fields       []textinput.Model
	FocusIndex   int // -1 = section header focused; 0..FieldCount-1 = field focused
	Section      int
	ScrollOffset int // index of the first visible field within the active section's field list
	TokenSource  config.TokenSource // where the current OAuth token came from
	ClearToken   bool               // true when ctrl+d was pressed; applied on save, not immediately

	// Help tab state
	HelpScrollOffset int      // line offset within the rendered help content
	helpLines        []string // glamour-rendered lines; nil = needs rebuild
	helpLinesWidth   int      // terminal width at which helpLines was rendered
}

var fieldLabels = []string{
	"Player type",
	"Player path",
	"Player args",
	"OAuth token",
	"Streamers",
	"Auto refresh",
	"Cache override",
	"Display mode",   // FieldDisplayMode
	"Card width",
	"Card columns",
	"Card pad H",
	"Card pad V",
	"Live color",
	"Select color",
	"Table count",    // FieldListTables
	"Max messages",    // FieldChatMaxMessages
	"Emote colors",    // FieldChatEmoteColors
	"Strip Dingbats",   // FieldChatStripDingbats
	"Max reconnects",   // FieldChatMaxReconnects
	"Show reply",       // FieldChatShowReply
	"Trim reply @",     // FieldChatTrimReplyMention
	"Collapse repeats", // FieldChatCollapseRepeats
	"Localized names",  // FieldChatLocalizedNames
	"Text badges",      // FieldChatTextBadges
	"Alt row color",    // FieldChatAltRows
	"3rd-party emotes", // FieldChatThirdPartyEmotes
	"3rd-party shading", // FieldChatThirdPartyShading
}

var fieldPlaceholders = []string{
	"mpv",            // FieldPlayerType — overridden dynamically in NewSettingsModel
	"",               // FieldPlayerPath — overridden dynamically in NewSettingsModel
	"",               // FieldPlayerArgs — overridden dynamically in NewSettingsModel
	"", // set dynamically based on TokenSource
	"streamer1, streamer2, ...",
	"5",
	"15",
	"cards",                           // FieldDisplayMode
	"22",
	"0  (0 = auto-fit terminal width)",
	"2",
	"0",
	"#102910",
	"#2C2260",
	"1",                               // FieldListTables
	"300",                             // FieldChatMaxMessages
	"on",                              // FieldChatEmoteColors
	"on",                              // FieldChatStripDingbats
	"5",                               // FieldChatMaxReconnects
	"on",                              // FieldChatShowReply
	"on",                              // FieldChatTrimReplyMention
	"single",                          // FieldChatCollapseRepeats
	"on",                              // FieldChatLocalizedNames
	"off",                             // FieldChatTextBadges
	"",                                // FieldChatAltRows
	"off",                             // FieldChatThirdPartyEmotes
	"off",                             // FieldChatThirdPartyShading
}

var fieldNotes = []string{
	`"mpv" or "vlc" — determines PATH search and default install locations`,
	"path to player executable — leave empty to search $PATH for the selected type",
	"extra flags appended after the URL",
	"", // set dynamically based on TokenSource
	"comma-separated list — ignored when an OAuth token is configured",
	"background refresh interval in minutes (0 = disabled)",
	"minutes before cached details/VODs are re-fetched (0 = no expiry)",
	`"cards" for card grid view, "list" for compact table view`, // FieldDisplayMode
	"inner content width of each streamer card",
	"columns per row; 0 auto-fits based on card width and terminal width",
	"horizontal padding (left & right) inside each card",
	"vertical padding (top & bottom) inside each card",
	"background color for live streamer cards (hex #rrggbb)",
	"background color for the selected card (hex #rrggbb)",
	"number of side-by-side tables in list mode (1–5)",           // FieldListTables
	"messages to keep in the chat buffer (100–1000)",                                       // FieldChatMaxMessages
	`"on" or "off" — grey-tint emote text to distinguish it`,                               // FieldChatEmoteColors
	`"on" or "off" — remove Dingbats (U+2700–U+27BF) that render as wide replacement boxes in most terminal/programming fonts`, // FieldChatStripDingbats
	"auto-reconnect attempts before stopping (0 = unlimited)",                           // FieldChatMaxReconnects
	`"on" or "off" — show ↩ @user · reply-to prefix on reply messages`,                  // FieldChatShowReply
	`"on" or "off" — strip the leading @username from the message body on reply messages`, // FieldChatTrimReplyMention
	`"off", "single" (same user only), or "all" (any user, same text) — merge consecutive identical messages into one line with a [xN] counter`, // FieldChatCollapseRepeats
	`"on" or "off" — append ASCII login alias after non-ASCII display names, e.g. 名前 (name123)`,               // FieldChatLocalizedNames
	`"on" or "off" — replace icon glyphs (●, ⚔, ■…) with [Broadcaster], [Mod], [Sub]… text labels`, // FieldChatTextBadges
	`hex color for alternating row background (#RRGGBB), e.g. "#261A48" — empty = disabled`,          // FieldChatAltRows
	`"on" or "off" — fetch BTTV/7TV emote lists and style recognized codes; only emotes the channel has explicitly enabled are known — globally popular emotes (e.g. KEKW) may not appear if the channel hasn't added them`, // FieldChatThirdPartyEmotes
	`"on" or "off" — apply the same grey tint as native emote_colors to BTTV/7TV emotes (when off, a separate amber color is used to distinguish them)`, // FieldChatThirdPartyShading
}

// OAuthPlaceholder returns the placeholder text for the OAuth field based on
// where the token is currently stored. Exported so model.go can update it
// after a ctrl+d clear without re-opening the settings screen.
func OAuthPlaceholder(src config.TokenSource) string {
	switch src {
	case config.TokenSourceKeyring:
		return "● secured in OS keyring — type to replace, ctrl+d to remove"
	case config.TokenSourceEnv:
		return "managed via TWITCH_OAUTH_TOKEN env var (takes priority)"
	case config.TokenSourceFile:
		return "⚠ token in plaintext — save again to move to OS keyring"
	default:
		return "enter token — will be stored securely in OS keyring"
	}
}

// oauthNote returns the descriptive note shown below the OAuth field.
func oauthNote(src config.TokenSource) string {
	switch src {
	case config.TokenSourceKeyring:
		return "token is secured in the OS credential store and never written to config.toml"
	case config.TokenSourceEnv:
		return "TWITCH_OAUTH_TOKEN env var overrides all other sources; manage it in your shell profile"
	case config.TokenSourceFile:
		return "⚠ OS keyring unavailable — token saved as plaintext in config.toml (consider fixing keyring access)"
	default:
		return "when set, your followed channels are fetched and used instead of the streamer list below"
	}
}

// PlayerTypeFromString normalises a player type string to "mpv" or "vlc".
func PlayerTypeFromString(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "vlc":
		return "vlc"
	default:
		return "mpv"
	}
}

// playerArgsPlaceholder returns an example args string for the given player type.
func playerArgsPlaceholder(playerType string) string {
	switch strings.ToLower(playerType) {
	case "vlc":
		return "--volume=80 --fullscreen"
	default:
		return "--volume=80 --fs"
	}
}

// NewSettingsModel creates a pre-filled SettingsModel.
func NewSettingsModel(
	playerType, playerPath, playerArgs, oauthToken, streamers string,
	autoRefreshMinutes, cacheOverrideMinutes int,
	displayMode string,
	cardWidth, cardColumns, cardPadH, cardPadV int,
	cardLiveColor, cardSelectColor string,
	listTableCount int,
	chatMaxMessages      int,
	chatEmoteColors      bool,
	chatStripDingbats    bool,
	chatMaxReconnects    int,
	chatShowReply        bool,
	chatTrimReplyMention bool,
	chatCollapseRepeats  string,
	chatLocalizedNames   bool,
	chatTextBadges           bool,
	chatAltRowColor          string,
	chatThirdPartyEmotes     bool,
	chatThirdPartyShading    bool,
	tokenSource config.TokenSource,
) SettingsModel {
	// Decide what value to place in the OAuth field:
	//   Keyring → empty (placeholder is visible and explains the state; ctrl+d clears)
	//   Env var → empty (placeholder explains; field is informational only)
	//   File    → actual token masked by EchoPassword (user can edit or clear it)
	//   None    → empty (placeholder prompts for input)
	var oauthFieldValue string
	if tokenSource == config.TokenSourceFile {
		oauthFieldValue = oauthToken
	}

	values := []string{
		PlayerTypeFromString(playerType),
		playerPath,
		playerArgs,
		oauthFieldValue,
		streamers,
		fmt.Sprintf("%d", autoRefreshMinutes),
		fmt.Sprintf("%d", cacheOverrideMinutes),
		displayMode,                              // FieldDisplayMode
		fmt.Sprintf("%d", cardWidth),
		fmt.Sprintf("%d", cardColumns),
		fmt.Sprintf("%d", cardPadH),
		fmt.Sprintf("%d", cardPadV),
		cardLiveColor,
		cardSelectColor,
		fmt.Sprintf("%d", listTableCount),        // FieldListTables
		fmt.Sprintf("%d", chatMaxMessages),              // FieldChatMaxMessages
		ChatEmoteColorsToString(chatEmoteColors),        // FieldChatEmoteColors
		ChatStripDingbatsToString(chatStripDingbats),    // FieldChatStripDingbats
		fmt.Sprintf("%d", chatMaxReconnects),            // FieldChatMaxReconnects
		ChatShowReplyToString(chatShowReply),               // FieldChatShowReply
		ChatTrimReplyMentionToString(chatTrimReplyMention), // FieldChatTrimReplyMention
		chatCollapseRepeats,                                // FieldChatCollapseRepeats
		boolToOnOff(chatLocalizedNames),                    // FieldChatLocalizedNames
		boolToOnOff(chatTextBadges),                        // FieldChatTextBadges
		chatAltRowColor,                                    // FieldChatAltRows
		boolToOnOff(chatThirdPartyEmotes),                  // FieldChatThirdPartyEmotes
		boolToOnOff(chatThirdPartyShading),                 // FieldChatThirdPartyShading
	}

	fields := make([]textinput.Model, FieldCount)
	for i := range fields {
		ti := textinput.New()
		ti.Placeholder = fieldPlaceholders[i]
		ti.SetValue(values[i])
		ti.SetWidth(58)
		s := textinput.DefaultDarkStyles()
		s.Focused.Prompt = styles.Accent
		s.Focused.Text = styles.Text
		s.Blurred.Prompt = styles.Muted
		s.Blurred.Text = styles.Text
		ti.SetStyles(s)
		fields[i] = ti
	}

	// Dynamic placeholders based on the selected player type.
	resolvedType := PlayerTypeFromString(playerType)
	fields[FieldPlayerPath].Placeholder = fmt.Sprintf("leave empty to search $PATH for %s", resolvedType)
	fields[FieldPlayerArgs].Placeholder = playerArgsPlaceholder(resolvedType)

	// OAuth field: always mask the value so the token is never shown in plain text.
	fields[FieldOAuthToken].EchoMode = textinput.EchoPassword
	fields[FieldOAuthToken].Placeholder = OAuthPlaceholder(tokenSource)

	// Apply per-field character limits and enum cycle sets from the single
	// source-of-truth table.  Adding a new field only requires a new entry
	// in FieldConstraints — no other code here changes.
	for i := range fields {
		if lim := FieldConstraints[i].CharLimit; lim > 0 {
			fields[i].CharLimit = lim
		}
	}

	fields[0].Focus()
	return SettingsModel{
		Fields:      fields,
		FocusIndex:  0,
		Section:     0,
		TokenSource: tokenSource,
	}
}

// RenderSettings renders the settings screen.
// RenderSettings renders the settings form clipped to availableHeight so it
// never overflows the terminal.  Each field occupies exactly 3 lines
// (label+input row, note, blank/indicator); the remaining chrome is 7 lines
// (title, tabs, rule, scroll-above, scroll-below, rule, help).  When more
// fields exist than fit, scroll indicators replace the blank lines that would
// otherwise border the visible window, and the caller is responsible for
// keeping ScrollOffset up-to-date via scrollSettingsToFocus.
// RenderSettingsHelp is the entrypoint for the Help tab. It takes a pointer so
// the glamour render cache (helpLines/helpLinesWidth) persists across frames
// when called via the Model pointer path.
func RenderSettingsHelp(m *SettingsModel, width, availableHeight int) string {
	return renderHelpView(m, width, availableHeight)
}

func RenderSettings(m SettingsModel, width, availableHeight int) string {
	if m.Section == SectionHelp {
		return renderHelpView(&m, width, availableHeight)
	}

	// Assemble via ScreenSettings layout:
	//   Title     = "Settings"                        (TitleLines = 1)
	//   SubHeader = tabs + rule + above-indicator     (SettingsSubHeaderLines = 3)
	//   Body      = field blocks, each exactly 3 lines
	//   Footer    = rule + help bar                   (SettingsFooterLines = 2)
	//
	// chrome = ScreenSettings.ChromeLines(SettingsSubHeaderLines, SettingsFooterLines) = 6
	chrome := ScreenSettings.ChromeLines(SettingsSubHeaderLines, SettingsFooterLines)
	title := styles.SectionTitle.PaddingLeft(1).Render("Settings")

	sectionFields := ActiveSectionFields(m, m.Section)

	fieldArea := availableHeight - chrome
	if fieldArea < 3 {
		fieldArea = 3 // always show at least one field
	}
	visibleCount := fieldArea / 3

	// Clamp scroll offset to valid range.
	so := m.ScrollOffset
	maxOffset := len(sectionFields) - visibleCount
	if maxOffset < 0 {
		maxOffset = 0
	}
	if so > maxOffset {
		so = maxOffset
	}
	if so < 0 {
		so = 0
	}

	end := so + visibleCount
	if end > len(sectionFields) {
		end = len(sectionFields)
	}
	visible := sectionFields[so:end]
	aboveCount := so
	belowCount := len(sectionFields) - end

	// Scroll-above indicator always occupies the third line of the sub-header
	// (empty string when nothing is hidden above).
	aboveLine := ""
	if aboveCount > 0 {
		aboveLine = styles.Dim.Render(fmt.Sprintf("  ↑ %d more above", aboveCount))
	}
	subHeaderBlock := strings.Join([]string{renderSettingsTabs(m), Rule(width), aboveLine}, "\n")

	var bodyLines []string
	for i, fi := range visible {
		f := m.Fields[fi]
		var label string
		if fi == m.FocusIndex {
			label = styles.SettingLabelFocused.Render("▶ " + fieldLabels[fi])
		} else {
			label = styles.SettingLabel.Render("  " + fieldLabels[fi])
		}

		// Dynamic note for the OAuth field; static notes for everything else.
		noteText := fieldNotes[fi]
		if fi == FieldOAuthToken {
			noteText = oauthNote(m.TokenSource)
		}

		// For the OAuth field, colour the note based on storage status.
		var note string
		if fi == FieldOAuthToken {
			switch m.TokenSource {
			case config.TokenSourceKeyring:
				note = lipgloss.NewStyle().Foreground(styles.ColorLive).Render("  ● " + noteText)
			case config.TokenSourceFile:
				note = lipgloss.NewStyle().Foreground(styles.ColorYellow).Render("  ⚠ " + noteText)
			case config.TokenSourceEnv:
				note = lipgloss.NewStyle().Foreground(styles.ColorUptime).Render("  ⓘ " + noteText)
			default:
				note = styles.Dim.Render("  " + noteText)
			}
		} else {
			note = styles.Dim.Render("  " + noteText)
		}

		// Scroll-below indicator replaces the trailing blank of the last visible
		// field when more fields are hidden below.
		trailing := ""
		if i == len(visible)-1 && belowCount > 0 {
			trailing = styles.Dim.Render(fmt.Sprintf("  ↓ %d more below", belowCount))
		}

		row := lipgloss.JoinHorizontal(lipgloss.Left, label, "  ", f.View())
		bodyLines = append(bodyLines, row, note, trailing)
	}

	footerBlock := strings.Join([]string{Rule(width), renderSettingsHelp(m.FocusIndex < 0, m.FocusIndex == FieldOAuthToken)}, "\n")
	return ScreenSettings.Render(title, subHeaderBlock, strings.Join(bodyLines, "\n"), footerBlock)
}

func renderSettingsTabs(m SettingsModel) string {
	var tabs []string
	for i := range SectionFields {
		name := ActiveSectionName(m, i)
		var tab string
		if i == m.Section {
			if m.FocusIndex < 0 {
				tab = styles.SettingTabActive.Render(name)
			} else {
				tab = styles.SettingTabActiveBlurred.Render(name)
			}
		} else {
			tab = styles.SettingTab.Render(name)
		}
		tabs = append(tabs, tab)
	}
	return " " + strings.Join(tabs, " ")
}

func renderSettingsHelp(headerFocused bool, oauthFocused bool) string {
	var parts []string
	if headerFocused {
		parts = append(parts, hintItem("←→", "switch section"), hintItem("tab / enter", "edit fields"))
	} else {
		parts = append(parts, hintItem("tab / ↑↓", "navigate"), hintItem("enter", "save"))
		if oauthFocused {
			parts = append(parts, hintItem("ctrl+d", "remove token"))
		}
	}
	parts = append(parts, hintItem("esc", "discard & back"))
	return styles.HelpBar.Render(strings.Join(parts, "   "))
}

func renderHelpBar() string {
	parts := []string{
		hintItem("↑↓", "scroll"),
		hintItem("←→", "switch tab"),
		hintItem("esc", "back"),
	}
	return styles.HelpBar.Render(strings.Join(parts, "   "))
}

// ClampHelpScroll clamps HelpScrollOffset to the valid range for the given
// terminal dimensions. Call this from the Update handler whenever the offset
// changes so the model is always in a valid state before rendering.
func (m *SettingsModel) ClampHelpScroll(width, availableHeight int) {
	// chrome = ScreenSettings.ChromeLines(SettingsSubHeaderLines, SettingsHelpFooterLines) = 7
	chrome := ScreenSettings.ChromeLines(SettingsSubHeaderLines, SettingsHelpFooterLines)
	visibleLines := availableHeight - chrome
	if visibleLines < 1 {
		visibleLines = 1
	}
	lines := m.cachedHelpLines(width)
	maxOffset := len(lines) - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.HelpScrollOffset > maxOffset {
		m.HelpScrollOffset = maxOffset
	}
	if m.HelpScrollOffset < 0 {
		m.HelpScrollOffset = 0
	}
}

// renderHelpView renders the Help tab — a read-only, glamour-rendered markdown
// document. m is a pointer so the render cache (helpLines/helpLinesWidth) can
// be written back without copying the entire SettingsModel on each frame.
func renderHelpView(m *SettingsModel, width, availableHeight int) string {
	// Assemble via ScreenSettings layout:
	//   Title     = "Settings"                               (TitleLines = 1)
	//   SubHeader = tabs + rule + above-indicator            (SettingsSubHeaderLines = 3)
	//   Body      = visible glamour-rendered help content
	//   Footer    = below-indicator + rule + help bar        (SettingsHelpFooterLines = 3)
	//
	// chrome = ScreenSettings.ChromeLines(SettingsSubHeaderLines, SettingsHelpFooterLines) = 7
	chrome := ScreenSettings.ChromeLines(SettingsSubHeaderLines, SettingsHelpFooterLines)
	title := styles.SectionTitle.PaddingLeft(1).Render("Settings")
	tabs := renderSettingsTabs(*m)

	visibleLines := availableHeight - chrome
	if visibleLines < 1 {
		visibleLines = 1
	}

	lines := m.cachedHelpLines(width)

	aboveCount := m.HelpScrollOffset
	end := m.HelpScrollOffset + visibleLines
	if end > len(lines) {
		end = len(lines)
	}
	belowCount := len(lines) - end

	aboveLine := ""
	if aboveCount > 0 {
		aboveLine = styles.Dim.Render(fmt.Sprintf("  ↑ %d more above", aboveCount))
	}
	belowLine := ""
	if belowCount > 0 {
		belowLine = styles.Dim.Render(fmt.Sprintf("  ↓ %d more below", belowCount))
	}

	subHeaderBlock := strings.Join([]string{tabs, Rule(width), aboveLine}, "\n")
	bodyBlock := strings.Join(lines[m.HelpScrollOffset:end], "\n")
	footerBlock := strings.Join([]string{belowLine, Rule(width), renderHelpBar()}, "\n")
	return ScreenSettings.Render(title, subHeaderBlock, bodyBlock, footerBlock)
}

// cachedHelpLines returns glamour-rendered lines for helpMarkdown, rebuilding
// only when the terminal width changes.  The style is defined in help_style.go.
func (m *SettingsModel) cachedHelpLines(width int) []string {
	if m.helpLines != nil && m.helpLinesWidth == width {
		return m.helpLines
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(twitchGlamourStyle()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		m.helpLines = []string{"  (help unavailable: " + err.Error() + ")"}
		m.helpLinesWidth = width
		return m.helpLines
	}

	rendered, err := r.Render(helpMarkdown)
	if err != nil {
		m.helpLines = []string{"  (render error: " + err.Error() + ")"}
		m.helpLinesWidth = width
		return m.helpLines
	}

	m.helpLines = strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	m.helpLinesWidth = width
	return m.helpLines
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func PlayerArgsToSlice(s string) []string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return strings.Fields(s)
}

func PlayerArgsFromSlice(args []string) string {
	return strings.Join(args, " ")
}

func StreamersToSlice(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}

func StreamersFromSlice(list []string) string {
	return strings.Join(list, ", ")
}

func CardWidthFromString(s string) int {
	n := 0
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if n < 10 {
		n = 10
	}
	if n > 60 {
		n = 60
	}
	return n
}

func CardColumnsFromString(s string) int {
	n := 0
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if n < 0 {
		n = 0
	}
	return n
}

func CardPadFromString(s string) int {
	n := 0
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if n < 0 {
		n = 0
	}
	if n > 8 {
		n = 8
	}
	return n
}

func ColorFromString(s string) string {
	return strings.TrimSpace(s)
}

// AutoRefreshFromString parses the auto-refresh interval.
// 0 = disabled; clamped to [0, 120] minutes.
func AutoRefreshFromString(s string) int {
	n := 0
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if n < 0 {
		n = 0
	}
	if n > 120 {
		n = 120
	}
	return n
}

// CacheOverrideFromString parses the cache override period.
// 0 = no TTL expiry; clamped to [0, 1440] minutes (24 h max).
func CacheOverrideFromString(s string) int {
	n := 0
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if n < 0 {
		n = 0
	}
	if n > 1440 {
		n = 1440
	}
	return n
}

// DisplayModeFromString normalises the display mode to "cards" or "list".
func DisplayModeFromString(s string) string {
	if strings.ToLower(strings.TrimSpace(s)) == "list" {
		return "list"
	}
	return "cards"
}

// ListTableCountFromString parses the side-by-side table count.
// Clamped to [1, 5].
func ListTableCountFromString(s string) int {
	n := 1
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if n < 1 {
		n = 1
	}
	if n > 5 {
		n = 5
	}
	return n
}

// ChatMaxMessagesFromString parses the chat buffer size.
// Clamped to [100, 1000].
func ChatMaxMessagesFromString(s string) int {
	n := 300
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if n < 100 {
		n = 100
	}
	if n > 1000 {
		n = 1000
	}
	return n
}

// boolToOnOff converts a bool setting value to its canonical "on"/"off" string.
func boolToOnOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// onOffToBool parses a settings field value as a boolean.
// "on" (case-insensitive, whitespace-trimmed) → true; anything else → false.
func onOffToBool(s string) bool {
	return strings.ToLower(strings.TrimSpace(s)) == "on"
}

// ChatEmoteColorsFromString parses the emote-colors toggle.
func ChatEmoteColorsFromString(s string) bool { return onOffToBool(s) }

// ChatEmoteColorsToString converts the bool setting to its display string.
func ChatEmoteColorsToString(b bool) string { return boolToOnOff(b) }

// ChatStripDingbatsFromString parses the strip-dingbats toggle.
func ChatStripDingbatsFromString(s string) bool { return onOffToBool(s) }

// ChatStripDingbatsToString converts the bool setting to its display string.
func ChatStripDingbatsToString(b bool) string { return boolToOnOff(b) }

// ChatShowReplyFromString parses the show-reply toggle.
func ChatShowReplyFromString(s string) bool { return onOffToBool(s) }

// ChatShowReplyToString converts the bool to its display string.
func ChatShowReplyToString(b bool) string { return boolToOnOff(b) }

// ChatTrimReplyMentionFromString parses the trim-reply-mention toggle.
func ChatTrimReplyMentionFromString(s string) bool { return onOffToBool(s) }

// ChatTrimReplyMentionToString converts the bool to its display string.
func ChatTrimReplyMentionToString(b bool) string { return boolToOnOff(b) }

// ChatAltRowColorFromString normalises the alt-row hex color string.
// Strips whitespace; returns empty string (feature disabled) when blank.
func ChatAltRowColorFromString(s string) string { return strings.TrimSpace(s) }

// ChatCollapseRepeatsFromString parses the collapse-repeats mode.
// Accepts "off", "single", or "all"; anything else normalises to "single".
func ChatCollapseRepeatsFromString(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off":
		return "off"
	case "all":
		return "all"
	default:
		return "single"
	}
}

// ChatLocalizedNamesFromString parses the localized-names toggle.
func ChatLocalizedNamesFromString(s string) bool { return onOffToBool(s) }

// ChatLocalizedNamesToString converts the bool to its display string.
func ChatLocalizedNamesToString(b bool) string { return boolToOnOff(b) }

// ChatTextBadgesFromString parses the text-badges toggle.
func ChatTextBadgesFromString(s string) bool { return onOffToBool(s) }

// ChatTextBadgesToString converts the bool to its display string.
func ChatTextBadgesToString(b bool) string { return boolToOnOff(b) }

// ChatThirdPartyEmotesFromString parses the third-party emotes toggle.
func ChatThirdPartyEmotesFromString(s string) bool { return onOffToBool(s) }

// ChatThirdPartyEmotesToString converts the bool to its display string.
func ChatThirdPartyEmotesToString(b bool) string { return boolToOnOff(b) }

// ChatThirdPartyShadingFromString parses the third-party shading toggle.
func ChatThirdPartyShadingFromString(s string) bool { return onOffToBool(s) }

// ChatThirdPartyShadingToString converts the bool to its display string.
func ChatThirdPartyShadingToString(b bool) string { return boolToOnOff(b) }

// ChatMaxReconnectsFromString parses the max-reconnects value.
// 0 = unlimited; clamped to [0, 99].
func ChatMaxReconnectsFromString(s string) int {
	n := 5
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if n < 0 {
		n = 0
	}
	if n > 99 {
		n = 99
	}
	return n
}

// ---------------------------------------------------------------------------
// FieldConstraint — data-driven input rules (used by settings_update.go)
// ---------------------------------------------------------------------------

// FieldConstraint encodes all input rules for a single settings field.
// Adding a new settings field only requires one entry in FieldConstraints;
// no switch statements or helper functions need to change.
type FieldConstraint struct {
	// CharLimit is passed directly to textinput.Model.CharLimit.
	// 0 means unlimited (the textinput default).
	CharLimit int

	// Allow, when non-nil, is called for each typed character byte; the
	// character is silently dropped when the function returns false.
	// nil means all printable characters are accepted.
	Allow func(ch byte) bool

	// Enum, when non-nil, lists the only valid values for this field.
	// The user cycles through them with space / left / right; all other
	// printable keys are swallowed.
	Enum []string
}

// IsEnum reports whether this field cycles through a fixed value set.
func (c FieldConstraint) IsEnum() bool { return c.Enum != nil }

// AllowChar reports whether the single-character key is acceptable input.
// Multi-rune strings (control sequences) always return true — they are
// forwarded to the textinput for interpretation.
func (c FieldConstraint) AllowChar(key string) bool {
	if len(key) != 1 || c.Allow == nil {
		return true
	}
	return c.Allow(key[0])
}

// Cycle returns the next (dir=+1) or previous (dir=-1) value in the enum.
// If current is not in the set, the first value is returned.
func (c FieldConstraint) Cycle(current string, dir int) string {
	low := strings.ToLower(strings.TrimSpace(current))
	for i, v := range c.Enum {
		if v == low {
			return c.Enum[(i+dir+len(c.Enum))%len(c.Enum)]
		}
	}
	return c.Enum[0] // unknown → reset to first
}

// ---------------------------------------------------------------------------
// Named constraint prototypes — reused across fields that share the same rules.
// ---------------------------------------------------------------------------

var (
	// constraintFree accepts any printable character with no length limit.
	constraintFree = FieldConstraint{}

	// constraintDigit accepts decimal digits only.
	constraintDigit = FieldConstraint{
		Allow: func(ch byte) bool { return ch >= '0' && ch <= '9' },
	}

	// constraintHexColor accepts '#' followed by hex digits; max 7 chars (#RRGGBB).
	constraintHexColor = FieldConstraint{
		CharLimit: 7,
		Allow: func(ch byte) bool {
			return ch == '#' ||
				(ch >= '0' && ch <= '9') ||
				(ch >= 'a' && ch <= 'f') ||
				(ch >= 'A' && ch <= 'F')
		},
	}

	// constraintOnOff cycles "on" ↔ "off".
	constraintOnOff = FieldConstraint{CharLimit: 3, Enum: []string{"on", "off"}}

	// constraintToken accepts alphanumeric + colon ("oauth:" prefix).
	constraintToken = FieldConstraint{
		CharLimit: 206, // 200-char token + 6-char "oauth:" prefix
		Allow: func(ch byte) bool {
			return (ch >= 'a' && ch <= 'z') ||
				(ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') ||
				ch == ':'
		},
	}

	// constraintTwitchList accepts Twitch usernames with comma+space separators.
	constraintTwitchList = FieldConstraint{
		Allow: func(ch byte) bool {
			return (ch >= 'a' && ch <= 'z') ||
				(ch >= 'A' && ch <= 'Z') ||
				(ch >= '0' && ch <= '9') ||
				ch == '_' || ch == ',' || ch == ' '
		},
	}
)

// withLimit returns a copy of c with CharLimit set to n.
func withLimit(c FieldConstraint, n int) FieldConstraint {
	c.CharLimit = n
	return c
}

// withEnum returns a constraint whose only valid values are the given strings.
// CharLimit is set automatically to the length of the longest value.
func withEnum(values ...string) FieldConstraint {
	max := 0
	for _, v := range values {
		if len(v) > max {
			max = len(v)
		}
	}
	return FieldConstraint{CharLimit: max, Enum: values}
}

// FieldConstraints is the single source of truth for per-field input rules.
// Each entry encodes the CharLimit, allowed characters, and (for enum fields)
// the ordered value set.  Adding a new settings field requires only one new
// entry here — no other helper functions need to change.
var FieldConstraints = [FieldCount]FieldConstraint{
	FieldPlayerType:           withEnum("mpv", "vlc"),
	FieldPlayerPath:           constraintFree,
	FieldPlayerArgs:           constraintFree,
	FieldOAuthToken:           constraintToken,
	FieldStreamers:            constraintTwitchList,
	FieldAutoRefresh:          withLimit(constraintDigit, 3),  // 0–120
	FieldCacheOverride:        withLimit(constraintDigit, 4),  // 0–1440
	FieldDisplayMode:          withEnum("cards", "list"),
	FieldCardWidth:            withLimit(constraintDigit, 2),  // 10–60
	FieldCardColumns:          withLimit(constraintDigit, 2),  // 0–99
	FieldCardPadH:             withLimit(constraintDigit, 1),  // 0–8
	FieldCardPadV:             withLimit(constraintDigit, 1),  // 0–8
	FieldCardLiveColor:        constraintHexColor,             // #RRGGBB
	FieldCardSelectColor:      constraintHexColor,
	FieldListTables:           withLimit(constraintDigit, 1),  // 1–5
	FieldChatMaxMessages:      withLimit(constraintDigit, 4),  // 100–1000
	FieldChatEmoteColors:      constraintOnOff,
	FieldChatStripDingbats:    constraintOnOff,
	FieldChatMaxReconnects:    withLimit(constraintDigit, 2),  // 0–99
	FieldChatShowReply:        constraintOnOff,
	FieldChatTrimReplyMention: constraintOnOff,
	FieldChatCollapseRepeats:  withEnum("off", "single", "all"),
	FieldChatLocalizedNames:   constraintOnOff,
	FieldChatTextBadges:       constraintOnOff,
	FieldChatAltRows:              constraintHexColor,  // #RRGGBB or empty
	FieldChatThirdPartyEmotes:     constraintOnOff,
	FieldChatThirdPartyShading:    constraintOnOff,
}
