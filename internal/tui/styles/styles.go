package styles

import "charm.land/lipgloss/v2"

// ---------------------------------------------------------------------------
// Twitch colour palette
// ---------------------------------------------------------------------------
var (
	Purple      = lipgloss.Color("#9147FF")
	PurpleLight = lipgloss.Color("#BF94FF")
	PurpleDim   = lipgloss.Color("#251A4A") // card selected background (subtle)
	PurpleMid   = lipgloss.Color("#5C3DB0")

	CardBgSelected = lipgloss.Color("#2C2260") // selection card fill — visible but calm
	CardBgLive     = lipgloss.Color("#102910") // live card fill — dark green tint

	ColorLive    = lipgloss.Color("#00AD03")
	ColorLiveDim = lipgloss.Color("#003A01") // live card bg tint
	ColorOffline = lipgloss.Color("#6B6B7B")

	ColorText      = lipgloss.Color("#EFEFF1")
	ColorTextMuted = lipgloss.Color("#ADADB8")
	ColorTextDim   = lipgloss.Color("#6B6B7B")

	ColorBorder = lipgloss.Color("#3A3A3D")
	ColorSearch = lipgloss.Color("#1F1629") // search bar bg

	// Information type colours
	ColorGame    = lipgloss.Color("#BF94FF") // purple-light for game/category
	ColorViewers = lipgloss.Color("#FFCD00") // gold for viewer count
	ColorUptime  = lipgloss.Color("#1FB8F5") // twitch blue for uptime
	ColorTitle   = lipgloss.Color("#EFEFF1") // bright white for stream title
	ColorURL     = lipgloss.Color("#9147FF") // purple for URLs

	ColorYellow = lipgloss.Color("#FFCD00")
	ColorRed    = lipgloss.Color("#E91916")
)

// ---------------------------------------------------------------------------
// Base text
// ---------------------------------------------------------------------------
var (
	Text  = lipgloss.NewStyle().Foreground(ColorText)
	Muted = lipgloss.NewStyle().Foreground(ColorTextMuted)
	Dim   = lipgloss.NewStyle().Foreground(ColorTextDim)
	Bold  = lipgloss.NewStyle().Foreground(ColorText).Bold(true)

	Live    = lipgloss.NewStyle().Foreground(ColorLive).Bold(true)
	Offline = lipgloss.NewStyle().Foreground(ColorOffline)

	Accent      = lipgloss.NewStyle().Foreground(Purple)
	AccentBold  = lipgloss.NewStyle().Foreground(Purple).Bold(true)
	AccentLight = lipgloss.NewStyle().Foreground(PurpleLight)

	StatusOK  = lipgloss.NewStyle().Foreground(ColorLive).Bold(true)
	StatusErr = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)

	// Info-type specific styles
	InfoGame    = lipgloss.NewStyle().Foreground(ColorGame)
	InfoViewers = lipgloss.NewStyle().Foreground(ColorViewers).Bold(true)
	InfoUptime  = lipgloss.NewStyle().Foreground(ColorUptime)
	InfoTitle   = lipgloss.NewStyle().Foreground(ColorTitle).Bold(true)
	InfoURL     = lipgloss.NewStyle().Foreground(ColorURL)
)

// ---------------------------------------------------------------------------
// App chrome
// ---------------------------------------------------------------------------
var (
	// AppTitle — outer rows of the 3-line unicode-art title (or fallback)
	AppTitle = lipgloss.NewStyle().Foreground(Purple).Bold(true)

	// AppTitleMid — middle row of the unicode-art title (brighter stems)
	AppTitleMid = lipgloss.NewStyle().Foreground(PurpleLight).Bold(true)

	// AppTitleDeco — decoration around the title
	AppTitleDeco = lipgloss.NewStyle().Foreground(PurpleMid)

	// SubHeader — streamer name shown below the rule on sub-screens
	SubHeader = lipgloss.NewStyle().
			Foreground(PurpleLight).
			Bold(true)

	SubHeaderAccent = lipgloss.NewStyle().Foreground(Purple).Bold(true)

	HeaderStat = lipgloss.NewStyle().Foreground(ColorTextMuted)

	// Rule — horizontal separator
	Rule = lipgloss.NewStyle().Foreground(ColorBorder)

	// StatusBar — bottom status strip
	StatusBar = lipgloss.NewStyle().
			Background(lipgloss.Color("#18181B")).
			Foreground(ColorTextMuted).
			PaddingLeft(1)

	// HelpBar
	HelpBar = lipgloss.NewStyle().Foreground(ColorTextDim).PaddingLeft(1)
	HelpKey = lipgloss.NewStyle().Foreground(PurpleLight).Bold(true)
)

// ---------------------------------------------------------------------------
// Filter tabs
// ---------------------------------------------------------------------------
var (
	Tab = lipgloss.NewStyle().Foreground(ColorTextDim).
		PaddingLeft(1).PaddingRight(1)

	TabActive = lipgloss.NewStyle().Foreground(Purple).Bold(true).
			PaddingLeft(1).PaddingRight(1).Underline(true)

	// FilterTabActive — main list filter active tab with subtle purple tint
	FilterTabActive = lipgloss.NewStyle().Foreground(PurpleLight).Bold(true).
			Background(PurpleDim).PaddingLeft(1).PaddingRight(1)
	// FilterTab — inactive filter tab with dim shortcut key
	FilterTab = lipgloss.NewStyle().Foreground(ColorTextDim).
			PaddingLeft(1).PaddingRight(1)
)

// ---------------------------------------------------------------------------
// Card list rows  (streamer list + VOD list)
//
//  Each row has a 1-char accent column on the far left:
//    ▌  →  selected   (thick purple block, purple-dim bg)
//    │  →  live       (thin green bar, no bg)
//    ╎  →  offline    (dashed dim bar, no bg)
// ---------------------------------------------------------------------------
var (
	// Accent glyphs
	CardAccentSelected = lipgloss.NewStyle().Foreground(Purple).Bold(true)
	CardAccentLive     = lipgloss.NewStyle().Foreground(ColorLive)
	CardAccentOffline  = lipgloss.NewStyle().Foreground(ColorTextDim)

	// Row backgrounds
	CardRowSelected = lipgloss.NewStyle().Background(PurpleDim).Foreground(ColorText)
	CardRowLive     = lipgloss.NewStyle().Foreground(ColorText)
	CardRowOffline  = lipgloss.NewStyle().Foreground(ColorTextMuted)

	// Number column
	RowNum         = lipgloss.NewStyle().Foreground(ColorTextDim)
	RowNumSelected = lipgloss.NewStyle().Background(PurpleDim).Foreground(ColorTextDim)

	// Status badges
	LiveBadge    = lipgloss.NewStyle().Foreground(ColorLive).Bold(true)
	OfflineBadge = lipgloss.NewStyle().Foreground(ColorOffline)

	// Column headers
	ColHeader = lipgloss.NewStyle().Foreground(PurpleLight).Bold(true)

	// Match highlight inside search results
	MatchHighlight = lipgloss.NewStyle().Foreground(ColorYellow).Bold(true)

	// Search bar — dark tinted bg with a bottom border underline in accent purple
	SearchBarStyle = lipgloss.NewStyle().
			Background(ColorSearch).
			Foreground(ColorText).
			PaddingLeft(1).PaddingRight(1).
			Border(lipgloss.Border{Bottom: "─"}, false, false, true, false).
			BorderForeground(Purple)
	SearchIcon  = lipgloss.NewStyle().Foreground(Purple).Bold(true)
	SearchCount = lipgloss.NewStyle().Foreground(ColorTextDim)
)

// ---------------------------------------------------------------------------
// Details view
// ---------------------------------------------------------------------------
var (
	DetailLabel = lipgloss.NewStyle().
			Foreground(PurpleLight).
			Bold(true).
			Width(12).
			Align(lipgloss.Right)

	DetailValue = lipgloss.NewStyle().
			Foreground(ColorText).
			PaddingLeft(2)
)

// Auto-refresh countdown in the filter tab bar
var (
	AutoRefreshLabel = lipgloss.NewStyle().Foreground(ColorTextDim)
	AutoRefreshTimer = lipgloss.NewStyle().Foreground(ColorTextDim)
	AutoRefreshColon = lipgloss.NewStyle().Foreground(ColorBorder) // dimmer — creates the pulse
)

// SectionTitle — used for Settings and other sub-screen titles
var SectionTitle = lipgloss.NewStyle().Foreground(PurpleLight).Bold(true)

// ---------------------------------------------------------------------------
// Chat view
// ---------------------------------------------------------------------------
var (
	ChatBadgeBroadcaster = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	ChatBadgeMod         = lipgloss.NewStyle().Foreground(lipgloss.Color("#00AD03"))
	ChatBadgeSub         = lipgloss.NewStyle().Foreground(lipgloss.Color("#9147FF"))
	ChatBadgeVIP         = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF69B4"))
	ChatBadgeStaff       = lipgloss.NewStyle().Foreground(lipgloss.Color("#C8C8D0")) // bright grey — mirrors the ⚑ black/white icon
	ChatBadgeTurbo       = lipgloss.NewStyle().Foreground(ColorYellow) // ⚡ gold/electric
	ChatUsername         = lipgloss.NewStyle().Bold(true)
	ChatEmote            = lipgloss.NewStyle().Foreground(ColorBorder)                   // muted grey — native Twitch emote shading
	ChatEmoteThirdParty  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B6F47"))       // muted amber — BTTV/7TV emote distinctive style
	ChatReply            = lipgloss.NewStyle().Foreground(ColorTextDim).Italic(true) // ↩ @parent · prefix
	ChatMention          = lipgloss.NewStyle().Foreground(PurpleLight).Bold(true)    // @username in text
	ChatLogging          = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)       // ⏺ REC indicator
	ChatAction           = lipgloss.NewStyle().Foreground(ColorTextMuted).Italic(true) // /me action messages
	ChatStatusLine       = lipgloss.NewStyle().Foreground(ColorTextDim).Italic(true)
	ChatConnected        = lipgloss.NewStyle().Foreground(ColorLive)
	ChatDisconnected     = lipgloss.NewStyle().Foreground(ColorOffline)
	ChatRoomTag          = lipgloss.NewStyle().Foreground(ColorTextDim)
	ChatRoomTagPaused    = lipgloss.NewStyle().Foreground(lipgloss.Color("#E8A838")) // amber — visually distinct scroll-pause indicator

	// System event styles — contextual coloring by event type
	ChatSystemSub    = lipgloss.NewStyle().Foreground(lipgloss.Color("#9147FF"))        // purple — subscription
	ChatSystemGift   = lipgloss.NewStyle().Foreground(lipgloss.Color("#BF94FF"))        // light purple — gifted sub
	ChatSystemRaid   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))        // warm red — incoming raid
	ChatSystemRitual = lipgloss.NewStyle().Foreground(lipgloss.Color("#1FB8F5"))        // twitch blue — new chatter / ritual
	ChatModBan       = lipgloss.NewStyle().Foreground(lipgloss.Color("#CC2222"))        // dark red — permanent ban notice
	ChatModTimeout   = lipgloss.NewStyle().Foreground(lipgloss.Color("#BB8800"))        // dark amber — timeout notice
	ChatModDelete    = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C5C6A"))        // mid-dim grey — single message delete notice
	ChatGreyed       = lipgloss.NewStyle().Foreground(lipgloss.Color("#3A3A3D"))        // very dim — cosmetically greyed message text

	// Cheermote tier styles — each tier matches Twitch's official cheer colour palette.
	// Applied to inline cheermote tokens (e.g. "Cheer100") based on the token's bit amount.
	ChatCheer1    = lipgloss.NewStyle().Foreground(lipgloss.Color("#979797")).Bold(true) // 1–99    grey
	ChatCheer100  = lipgloss.NewStyle().Foreground(lipgloss.Color("#9C3EE8")).Bold(true) // 100–999 purple
	ChatCheer1k   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00C7AC")).Bold(true) // 1k–4.9k teal
	ChatCheer5k   = lipgloss.NewStyle().Foreground(lipgloss.Color("#0099FE")).Bold(true) // 5k–9.9k blue
	ChatCheer10k  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F33022")).Bold(true) // 10k–99k red
	ChatCheer100k = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5821F")).Bold(true) // 100k+   gold
)

// ---------------------------------------------------------------------------
// Chat input bar (normal / send mode)
// ---------------------------------------------------------------------------
var (
	ColorChatInput   = lipgloss.Color("#00B4CC") // teal accent — distinct from purple search
	ColorChatInputBg = lipgloss.Color("#0D2D33") // dark teal background

	// ChatInputBarStyle wraps the composed message with a bottom-border underline.
	ChatInputBarStyle = lipgloss.NewStyle().
				Background(ColorChatInputBg).
				Foreground(ColorText).
				PaddingLeft(1).PaddingRight(1).
				Border(lipgloss.Border{Bottom: "─"}, false, false, true, false).
				BorderForeground(ColorChatInput)

	// ChatInputIcon is the "> " prompt rendered inside the input bar.
	ChatInputIcon = lipgloss.NewStyle().Foreground(ColorChatInput).Bold(true)

	// ChatModeBadgeLurk is the dim "lurk" badge in the chat sub-header.
	ChatModeBadgeLurk = lipgloss.NewStyle().Foreground(ColorTextDim)

	// ChatModeBadgeNormal is the teal "● send" badge shown in normal mode.
	ChatModeBadgeNormal = lipgloss.NewStyle().Foreground(ColorChatInput).Bold(true)
)

// ---------------------------------------------------------------------------
// Settings view
// ---------------------------------------------------------------------------
var (
	SettingLabel        = lipgloss.NewStyle().Foreground(ColorTextMuted).Width(16)
	SettingLabelFocused = lipgloss.NewStyle().Foreground(PurpleLight).Bold(true).Width(16)

	// Section tab bar
	SettingTab              = lipgloss.NewStyle().Foreground(ColorTextDim).PaddingLeft(1).PaddingRight(1)
	SettingTabActive        = lipgloss.NewStyle().Foreground(PurpleLight).Bold(true).Background(PurpleDim).PaddingLeft(1).PaddingRight(1)
	SettingTabActiveBlurred = lipgloss.NewStyle().Foreground(PurpleLight).Bold(true).PaddingLeft(1).PaddingRight(1)
)
