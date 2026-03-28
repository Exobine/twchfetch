package tui

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/harmonica"
	"charm.land/lipgloss/v2"
	"golang.design/x/clipboard"

	"twchfetch/internal/api"
	"twchfetch/internal/cache"
	"twchfetch/internal/config"
	"twchfetch/internal/irc"
	"twchfetch/internal/logging"
	"twchfetch/internal/tui/styles"
	"twchfetch/internal/tui/views"
)

// titleArt is the 3-row unicode box-drawing banner for "twchfetch".
// Each letter is 3 display columns wide with 1-space separators → 35 cols total.
const titleArtWidth = 35

var titleArt = [3]string{
	"╔╦╗ ╦ ╦ ╔═╗ ╦ ╦ ╔═╗ ╔═╗ ╔╦╗ ╔═╗ ╦ ╦",
	" ║  ║║║ ║   ╠═╣ ╠═  ╠═   ║  ║   ╠═╣",
	" ╩  ╚╩╝ ╚═╝ ╩ ╩ ╚   ╚═╝  ╩  ╚═╝ ╩ ╩",
}

// viewState enumerates the possible TUI screens.
type viewState int

const (
	viewList viewState = iota
	viewDetails
	viewVODs
	viewSettings
	viewChat
)

// Chat scroll/mode constants are defined in views (views.ScrollMode*, views.ChatMode*)
// and referenced here directly to keep a single source of truth.

// Model is the root Bubbletea model.
type Model struct {
	// Infrastructure
	cfg       *config.Config
	cfgPath   string
	apiClient *api.Client
	cache     *cache.Cache
	keys      KeyMap
	clipboard bool

	// Layout
	width  int
	height int
	view   viewState

	// List state
	streamers     []api.StreamerStatus
	listFilter    string // "all" | "live" | "offline"
	listCursor    int    // index into visibleItems()
	listOffset    int    // scroll offset (first visible row)
	searchBuf     string // live search query
	searchActive  bool   // true while / search mode is open
	isRefreshing  bool
	followedList  []string // non-nil when OAuth follow-list has been fetched
	spinner       spinner.Model
	progressBar   progress.Model
	progressDone  int
	progressTotal int
	progressCh    chan int

	// Details state
	selectedUser   string
	detailData     *api.StreamDetails
	detailFollowers int
	lastSeen       *time.Time
	detailLoading  bool

	// VOD state
	vods          []api.VOD
	chapters      map[string][]api.Chapter
	vodsLoading   bool
	vodSelected   int    // selected row index; == len(vods) when Load More is focused
	vodOffset     int    // index of the topmost visible row
	vodNumBuf     string // Np play-specific-vod buffer
	vodHasMore    bool   // whether more VODs may exist (last fetch returned a full page)
	vodLoadingMore bool  // true while a "Load More" fetch is in-flight

	// Auto-refresh state
	autoRefreshing  bool      // true while a silent background refresh is in-flight
	autoRefreshGen  int       // generation counter; stale ticks with old gen are discarded
	autoRefreshDue  time.Time // wall-clock time the next auto-refresh is scheduled for

	// Chat state
	chatChannel        string
	chatConnected      bool
	chatLoading        bool
	chatAtBottom       bool              // false = user scrolled up; auto-scroll paused
	chatMessages       []irc.ChatMessage // ring buffer, max cfg.Chat.MaxMessages
	chatRoomState      irc.RoomState
	chatEventCh        <-chan irc.Event
	chatCancel         context.CancelFunc // cancels the IRC goroutine
	chatViewport       viewport.Model
	chatStreamerOnline  bool // true when streamer was live at the moment chat was opened
	chatAutoReconnect  bool // true while in viewChat; false after esc / manual disconnect
	chatReconnectCount int  // number of auto-reconnect attempts since last openChat/manual reconnect
	chatGeneration     int  // incremented on every new connection; stale ChatDisconnectedMsgs carry the old value and are ignored
	chatScrollMode     int  // 0=live (auto-scroll to latest), 1=frozen (hold position), 2=follow (advance by delta)

	// Third-party emote set for the active chat channel (BTTV + 7TV).
	// Populated by fetchEmotesCmd when ThirdPartyEmotes is enabled.
	// Cleared on openChat so stale sets from a previous channel never bleed.
	thirdPartyEmotes map[string]struct{}

	// Chat logging
	chatLogging  bool
	chatLogFile  *os.File
	chatLogPath  string
	chatRecBlink bool // toggled by chatRecTickCmd; drives the REC dot independently of render rate

	// chatUnread counts messages received while the user is scrolled up.
	// Reset to 0 when the user scrolls back to the bottom.
	chatUnread int

	// Chat search
	chatSearchActive bool       // true while the search bar is open and receiving input
	chatSearchBuf    string     // raw query string
	chatSearchQuery  views.Node // parsed search AST; nil = no filter active

	// Chat send
	chatMode        int         // views.ChatModeLurk (default) or views.ChatModeNormal
	chatIRCClient   *irc.Client // non-nil after connecting; used to send PRIVMSGs
	chatInputActive bool        // true while the input bar is accepting text
	chatInputBuf    string      // text being composed in the input bar

	// Modal dialog (replaces status bar)
	// activeDialog is non-nil while a huh-form modal is displayed over the
	// current view.  It is used for both error/info acknowledgment (single
	// "Okay" button) and yes/no confirmation prompts.
	activeDialog    *huh.Form
	dialogOnComplete func(m Model, confirmed bool) Model // nil for OK-only dialogs

	// Settings state
	settings    views.SettingsModel
	tokenSource config.TokenSource // where the OAuth token was loaded from

	// Harmonica springs
	accentSpring  harmonica.Spring
	progressSpring harmonica.Spring
	cursorSpring  harmonica.Spring

	// Card accent animation — accentPos pulses [0→1] via underdamped spring
	accentPos, accentVel, accentTarget float64

	// Progress bar spring — critically damped, no overshoot
	progressSpringPos, progressSpringVel float64

	// Cursor glide — cursorPosF springs toward float64(listCursor) each frame
	cursorPosF, cursorVelF float64

	// refreshedAt is set when a batch refresh completes; used to extend UptimeSec
	// locally without extra GQL requests.
	refreshedAt time.Time
}

// NewModel initialises the root model.
func NewModel(cfg *config.Config, cfgPath string, tokenSrc config.TokenSource) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot // braille spinner — smoother than Points
	sp.Style = styles.Accent

	pb := progress.New(
		progress.WithColors(lipgloss.Color("#9147FF"), lipgloss.Color("#BF94FF")),
		progress.WithoutPercentage(),
	)

	cbOK := false
	if err := clipboard.Init(); err == nil {
		cbOK = true
	}

	var autoRefreshDue time.Time
	if cfg.AutoRefreshMinutes > 0 {
		autoRefreshDue = time.Now().Add(time.Duration(cfg.AutoRefreshMinutes) * time.Minute)
	}

	return Model{
		cfg:         cfg,
		cfgPath:     cfgPath,
		tokenSource: tokenSrc,
		apiClient:   api.NewClient(cfg.ClientID, cfg.OAuthToken, cfg.RequestTimeoutSec),
		cache:       cache.New(time.Duration(cfg.CacheOverrideMinutes) * time.Minute),
		keys:        DefaultKeyMap(),
		clipboard:   cbOK,
		listFilter:  "all",
		spinner:     sp,
		progressBar: pb,
		progressCh:  make(chan int, 32),

		// Spring A: card accent pulse (underdamped for organic breathe)
		accentSpring: harmonica.NewSpring(harmonica.FPS(60), 5.0, 0.6),
		accentTarget: 1.0,

		// Spring B: progress bar smoothing (critically damped — no overshoot)
		progressSpring: harmonica.NewSpring(harmonica.FPS(60), 6.0, 1.0),

		// Spring C: cursor glide across card grid (fast, critically damped)
		cursorSpring: harmonica.NewSpring(harmonica.FPS(60), 20.0, 1.0),

		autoRefreshDue: autoRefreshDue,
	}
}

// ---------------------------------------------------------------------------
// Bubbletea interface
// ---------------------------------------------------------------------------

func (m Model) Init() tea.Cmd {
	arCmd := autoRefreshCmd(m.cfg.AutoRefreshMinutes, m.autoRefreshGen)
	if m.apiClient.HasAuth() {
		// Fetch the user's followed channels before the first batch refresh.
		return tea.Batch(func() tea.Msg { return m.spinner.Tick() }, fetchFollowListCmd(m.apiClient), cardAnimCmd(), arCmd)
	}
	total := batchCount(len(m.cfg.Streamers.List), m.cfg.RefreshBatchSize)
	m.progressTotal = total
	return tea.Batch(
		func() tea.Msg { return m.spinner.Tick() },
		m.doRefresh(),
		waitForProgress(m.progressCh, 0, total),
		cardAnimCmd(),
		arCmd,
	)
}

// cardAnimCmd schedules the next card accent spring frame at ~60 fps.
func cardAnimCmd() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(_ time.Time) tea.Msg {
		return CardAnimTickMsg{}
	})
}

// progressFrameCmd schedules the next progress spring frame at ~60 fps.
func progressFrameCmd() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(_ time.Time) tea.Msg {
		return ProgressFrameMsg{}
	})
}

// chatRecTickCmd schedules a 1-second tick to keep the REC indicator pulsing
// independently of how often chat messages arrive.
func chatRecTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return ChatRecTickMsg{}
	})
}

// autoRefreshCmd schedules one auto-refresh tick after the configured interval.
// Returns nil (no-op) when minutes ≤ 0 (feature disabled).
// gen is stamped onto the message so the handler can discard stale ticks that
// were superseded by a manual refresh or settings change.
func autoRefreshCmd(minutes, gen int) tea.Cmd {
	if minutes <= 0 {
		return nil
	}
	return tea.Tick(time.Duration(minutes)*time.Minute, func(_ time.Time) tea.Msg {
		return AutoRefreshTickMsg{Gen: gen}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.view == viewChat {
			m.chatViewport.SetWidth(m.width)
			m.chatViewport.SetHeight(chatViewportHeight(m.height, m.chatExtraBottomLines()))
			m.rebuildChatViewport()
		}
		// huh's Form.Init() emits tea.RequestWindowSize so the form can
		// calculate its group viewport height once field widths are known.
		// Our switch would swallow the message before the dialog sees it, so
		// forward it explicitly whenever a dialog is open.
		if m.activeDialog != nil {
			updated, cmd := m.activeDialog.Update(msg)
			if f, ok := updated.(*huh.Form); ok {
				m.activeDialog = f
			}
			return m, cmd
		}
		return m, nil

	case CardAnimTickMsg:
		// Spring A: accent pulse
		m.accentPos, m.accentVel = m.accentSpring.Update(m.accentPos, m.accentVel, m.accentTarget)
		if math.Abs(m.accentPos-m.accentTarget) < 0.02 && math.Abs(m.accentVel) < 0.02 {
			if m.accentTarget > 0.5 {
				m.accentTarget = 0.0
			} else {
				m.accentTarget = 1.0
			}
		}
		// Spring C: cursor glide toward logical cursor position
		cursorTarget := float64(m.listCursor)
		m.cursorPosF, m.cursorVelF = m.cursorSpring.Update(m.cursorPosF, m.cursorVelF, cursorTarget)
		// Only re-schedule when the list view is visible — no need to drive the spring
		// at 60 fps while the user is on details, VODs, settings, or chat screens.
		if m.view == viewList {
			return m, cardAnimCmd()
		}
		return m, nil

	case ProgressFrameMsg:
		target := 0.0
		if m.progressTotal > 0 {
			target = float64(m.progressDone) / float64(m.progressTotal)
		}
		m.progressSpringPos, m.progressSpringVel = m.progressSpring.Update(m.progressSpringPos, m.progressSpringVel, target)
		// Use ViewAs in renderLoading — no SetPercent needed; chain next frame until settled.
		if math.Abs(m.progressSpringPos-target) > 0.001 || math.Abs(m.progressSpringVel) > 0.001 {
			return m, progressFrameCmd()
		}
		return m, nil

	case spinner.TickMsg:
		if m.isRefreshing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progressBar, cmd = m.progressBar.Update(msg)
		return m, cmd

	case BatchProgressMsg:
		m.progressDone = msg.Completed
		m.progressTotal = msg.Total
		// Spring animates toward the new percentage; fire a progress frame and
		// keep waiting for the next batch.
		if msg.Completed < msg.Total {
			return m, tea.Batch(progressFrameCmd(), waitForProgress(m.progressCh, msg.Completed, msg.Total))
		}
		return m, progressFrameCmd()

	case RefreshDoneMsg:
		m.isRefreshing = false
		m.streamers = views.SortStreamers(msg.Results)
		liveN := countLive(m.streamers)
		logging.Info("Refresh complete", "total", len(m.streamers), "live", liveN)
		m.refreshedAt = time.Now()
		m.listCursor = 0
		m.listOffset = 0
		m.cursorPosF = 0
		m.cursorVelF = 0
		// Anchor auto-refresh countdown from when data actually arrived.
		m.autoRefreshGen++
		if m.cfg.AutoRefreshMinutes > 0 {
			m.autoRefreshDue = time.Now().Add(time.Duration(m.cfg.AutoRefreshMinutes) * time.Minute)
		}
		return m, autoRefreshCmd(m.cfg.AutoRefreshMinutes, m.autoRefreshGen)

	case DetailsFetchedMsg:
		if msg.Username == m.selectedUser {
			m.detailLoading = false
			m.detailData = msg.Details
			m.detailFollowers = msg.Followers
			m.lastSeen = msg.LastSeen
			if msg.Details != nil {
				logging.Info("Details fetched", "user", msg.Username, "live", true,
					"viewers", msg.Details.ViewersCount, "game", msg.Details.Game)
				// Backfill game onto the streamer in the main list so cards show it.
				for i, s := range m.streamers {
					if strings.EqualFold(s.Username, msg.Username) {
						m.streamers[i].Game = msg.Details.Game
						break
					}
				}
			} else {
				logging.Info("Details fetched", "user", msg.Username, "live", false)
			}
			if msg.Details != nil || msg.LastSeen != nil || msg.Followers > 0 {
				m.cache.Set(msg.Username, &cache.Entry{
					Details:   msg.Details,
					Followers: msg.Followers,
					LastSeen:  msg.LastSeen,
				})
			}
		}
		return m, nil

	case LatestVODFetchedMsg:
		if msg.Err != nil {
			logging.Warn("Latest VOD fetch failed", "user", msg.Username, "err", msg.Err)
			m, cmd := m.showErrorDialog("No VODs: " + msg.Err.Error())
			return m, cmd
		}
		logging.Info("Playing latest VOD", "user", msg.Username, "vod", msg.VODID)
		return m, launchPlayerCmd(views.VODUrl(msg.VODID), m.cfg)

	case VODsFetchedMsg:
		if msg.Username == m.selectedUser {
			m.vodsLoading = false
			m.vods = msg.VODs
			m.chapters = msg.Chapters
			m.vodSelected = 0
			m.vodOffset = 0
			m.vodHasMore = msg.HasMore
			logging.Info("VODs fetched", "user", msg.Username, "count", len(msg.VODs))
			if entry, ok := m.cache.Get(msg.Username); ok {
				entry.VODs = msg.VODs
				entry.Chapters = msg.Chapters
				entry.VODHasMore = msg.HasMore
				m.cache.Set(msg.Username, entry)
			}
			// If no cache entry exists (invalidated during fetch), don't create a
			// Details-nil orphan. selectStreamer will re-fetch details next visit.
		}
		return m, nil

	case VODsMoreFetchedMsg:
		if msg.Username == m.selectedUser {
			prevLen := len(m.vods)
			m.vodLoadingMore = false
			m.vodHasMore = msg.HasMore
			if len(msg.VODs) > 0 {
				// Deduplicate: only append VODs whose IDs we don't already hold.
				// Protects against a new VOD being published between page fetches.
				known := make(map[string]struct{}, len(m.vods))
				for _, v := range m.vods {
					known[v.ID] = struct{}{}
				}
				for _, v := range msg.VODs {
					if _, dup := known[v.ID]; !dup {
						m.vods = append(m.vods, v)
					}
				}
				if m.chapters == nil {
					m.chapters = make(map[string][]api.Chapter)
				}
				for id, chs := range msg.Chapters {
					m.chapters[id] = chs
				}
				if len(m.vods) > prevLen {
					m.vodSelected = prevLen
					m = m.scrollVODsToSelection()
				}
			}
			logging.Info("More VODs fetched", "user", msg.Username, "new", len(msg.VODs), "total", len(m.vods))
			if entry, ok := m.cache.Get(msg.Username); ok {
				entry.VODs = m.vods
				entry.Chapters = m.chapters
				entry.VODHasMore = msg.HasMore
				m.cache.Set(msg.Username, entry)
			}
		}
		return m, nil

	case EmotesFetchedMsg:
		// Only apply if we're still in the same chat channel.
		if m.view == viewChat && strings.EqualFold(msg.Channel, m.chatChannel) {
			m.thirdPartyEmotes = msg.Emotes
			m.rebuildChatViewport()
		}
		return m, nil

	case AutoRefreshTickMsg:
		// Discard ticks that were superseded by a manual refresh or settings save.
		if msg.Gen != m.autoRefreshGen {
			return m, nil
		}
		// Advance generation and set the next due time before firing refresh.
		m.autoRefreshGen++
		if m.cfg.AutoRefreshMinutes > 0 {
			m.autoRefreshDue = time.Now().Add(time.Duration(m.cfg.AutoRefreshMinutes) * time.Minute)
		}
		nextTick := autoRefreshCmd(m.cfg.AutoRefreshMinutes, m.autoRefreshGen)
		// Only do the silent refresh when the user is on the main list and no
		// refresh (manual or silent) is already in-flight.
		if m.view == viewList && !m.isRefreshing && !m.autoRefreshing {
			m.autoRefreshing = true
			logging.Debug("Auto-refresh triggered")
			return m, tea.Batch(nextTick, m.doSilentRefresh())
		}
		return m, nextTick

	case SilentRefreshDoneMsg:
		m.autoRefreshing = false
		liveN := countLive(msg.Results)
		logging.Debug("Silent refresh complete", "total", len(msg.Results), "live", liveN)

		// Remember which streamer the cursor is on so we can restore it.
		var cursorUsername string
		if items := m.visibleItems(); m.listCursor >= 0 && m.listCursor < len(items) {
			cursorUsername = items[m.listCursor].Streamer.Username
		}

		// Invalidate cache entries whose live status or category changed so the
		// details screen shows fresh data on next selection.
		for _, s := range msg.Results {
			m.cache.InvalidateIfChanged(s.Username, s.IsLive, s.Game)
		}

		m.streamers = views.SortStreamers(msg.Results)
		m.refreshedAt = time.Now()

		// Restore cursor to the same streamer in the updated list; if it moved
		// (e.g. streamer went offline and the filter hides them) fall back to 0.
		m.listCursor = 0
		m.listOffset = 0
		if cursorUsername != "" {
			for i, item := range m.visibleItems() {
				if strings.EqualFold(item.Streamer.Username, cursorUsername) {
					m.listCursor = i
					if m.isListMode() {
						// Align offset to the nearest visible window that contains i.
						numTables := m.cfg.Display.ListTableCount
						if numTables < 1 {
							numTables = 1
						}
						vis := m.listViewRowsPerTable() * numTables
						m.listOffset = (i / vis) * vis
					} else {
						m.listOffset = (i / m.gridNumCols()) * m.gridNumCols()
					}
					break
				}
			}
		}
		// Snap spring to new cursor position — no animation from a stale position.
		m.cursorPosF = float64(m.listCursor)
		m.cursorVelF = 0
		return m, nil

	case ChatConnectedMsg:
		if m.view == viewChat {
			m.chatConnected = true
			m.chatLoading = false
			m.chatIRCClient = msg.Client // store for send operations
			gen := m.chatGeneration
			resetCmd := tea.Tick(30*time.Second, func(_ time.Time) tea.Msg {
				return ChatReconnectResetMsg{Generation: gen}
			})
			return m, tea.Batch(waitForIRCEvent(m.chatEventCh, m.chatGeneration), resetCmd)
		}
		return m, nil

	case ChatReconnectResetMsg:
		// Reset the reconnect counter only if the connection that triggered the
		// timer is still the live one.  A generation mismatch means the connection
		// dropped and was replaced before the 2-minute window elapsed — ignore.
		if m.view == viewChat && msg.Generation == m.chatGeneration {
			m.chatReconnectCount = 0
		}
		return m, nil

	case ChatMsg:
		if m.view == viewChat {
			// Collapse consecutive identical messages from the same user into
			// the existing entry by incrementing its RepeatCount.  No new line
			// is added to the buffer, so the viewport offset and trim logic are
			// skipped — we only rebuild to refresh the [xN] counter in place.
			collapseMode := m.cfg.Chat.CollapseRepeats
			if collapseMode != "off" && len(m.chatMessages) > 0 {
				last := &m.chatMessages[len(m.chatMessages)-1]
				sameText := !last.IsSystem && !msg.Message.IsSystem &&
					last.Text == msg.Message.Text &&
					last.IsAction == msg.Message.IsAction &&
					last.ReplyTo == "" && msg.Message.ReplyTo == ""
				sameUser := last.Username == msg.Message.Username
				if sameText && (collapseMode == "all" || sameUser) {
					last.RepeatCount++
					if m.chatLogging && m.chatLogFile != nil {
						if line := views.FormatLogLine(msg.Message); line != "" {
							fmt.Fprintln(m.chatLogFile, line)
						}
					}
					m.rebuildChatViewport()
					return m, waitForIRCEvent(m.chatEventCh, m.chatGeneration)
				}
			}

			m.chatMessages = append(m.chatMessages, msg.Message)
			// Measure lines for messages about to be trimmed from the front
			// before discarding them.  rebuildChatViewportWithTrim uses this
			// count to compensate the saved offset so the viewport stays
			// anchored to the same visual content and clampUnread does not
			// falsely reduce the unread counter.
			topTrimLines := 0
			if max := m.cfg.Chat.MaxMessages; len(m.chatMessages) > max {
				trimCount := len(m.chatMessages) - max
				topTrimLines = views.MessagesLineCount(
					m.chatMessages[:trimCount],
					m.cfg.Chat.EmoteColors,
					m.cfg.Chat.ShowReply,
					m.cfg.Chat.TrimReplyMention,
					m.cfg.Chat.LocalizedNames,
					m.cfg.Chat.TextBadges,
					m.cfg.Chat.AltRowColor,
					m.chatViewport.Width(),
					m.chatSearchQuery,
					m.thirdPartyEmotes,
					m.cfg.Chat.ThirdPartyShading,
				)
				m.chatMessages = m.chatMessages[trimCount:]
			}
			if m.chatLogging && m.chatLogFile != nil {
				if line := views.FormatLogLine(msg.Message); line != "" {
					fmt.Fprintln(m.chatLogFile, line)
				}
			}
			if m.chatScrollMode != views.ScrollModeLive && views.MatchChatMessage(msg.Message, m.chatSearchQuery) {
				m.chatUnread++
			}
			m.rebuildChatViewportWithTrim(topTrimLines)
			return m, waitForIRCEvent(m.chatEventCh, m.chatGeneration)
		}
		return m, nil

	case ChatRoomStateMsg:
		if m.view == viewChat {
			m.chatRoomState = msg.State
			m.rebuildChatViewport()
			return m, waitForIRCEvent(m.chatEventCh, m.chatGeneration)
		}
		return m, nil

	case ChatUsernoticeMsg:
		if m.view == viewChat {
			// Backward suppression: when the gift-bomb summary arrives, remove any
			// individual subgift messages already in the buffer from the same gifter.
			// This covers the common Twitch ordering where individual events arrive
			// before the submysterygift summary line, making both orderings produce
			// identical output — only the bomb summary is ever visible in chat.
			// Chat logs are unaffected because logging happens at message arrival time.
			if msg.Message.SystemKind == irc.SystemKindGiftBomb {
				gifterKey := msg.Message.Username
				if gifterKey == "" {
					gifterKey = "ananonymousgifter"
				}
				filtered := m.chatMessages[:0]
				for _, cm := range m.chatMessages {
					if cm.SystemKind == irc.SystemKindGift && cm.Username == gifterKey {
						continue // drop the individual gift notification
					}
					filtered = append(filtered, cm)
				}
				m.chatMessages = filtered
			}

			m.chatMessages = append(m.chatMessages, msg.Message)
			topTrimLines := 0
			if max := m.cfg.Chat.MaxMessages; len(m.chatMessages) > max {
				trimCount := len(m.chatMessages) - max
				topTrimLines = views.MessagesLineCount(
					m.chatMessages[:trimCount],
					m.cfg.Chat.EmoteColors,
					m.cfg.Chat.ShowReply,
					m.cfg.Chat.TrimReplyMention,
					m.cfg.Chat.LocalizedNames,
					m.cfg.Chat.TextBadges,
					m.cfg.Chat.AltRowColor,
					m.chatViewport.Width(),
					m.chatSearchQuery,
					m.thirdPartyEmotes,
					m.cfg.Chat.ThirdPartyShading,
				)
				m.chatMessages = m.chatMessages[trimCount:]
			}
			if m.chatLogging && m.chatLogFile != nil {
				if line := views.FormatLogLine(msg.Message); line != "" {
					fmt.Fprintln(m.chatLogFile, line)
				}
			}
			if m.chatScrollMode != views.ScrollModeLive && views.MatchChatMessage(msg.Message, m.chatSearchQuery) {
				m.chatUnread++
			}
			m.rebuildChatViewportWithTrim(topTrimLines)
			return m, waitForIRCEvent(m.chatEventCh, m.chatGeneration)
		}
		return m, nil

	case ChatClearchatMsg:
		if m.view == viewChat {
			if msg.TargetLogin != "" {
				// Determine the post-fix label for affected messages.
				var action string
				if msg.Duration == 0 {
					action = "banned"
				} else {
					mins := msg.Duration / 60
					secs := msg.Duration % 60
					if mins > 0 {
						action = fmt.Sprintf("timeout %dm", mins)
					} else {
						action = fmt.Sprintf("timeout %ds", secs)
					}
				}
				// Grey out every message from the target user still in the buffer.
				for i := range m.chatMessages {
					if !m.chatMessages[i].IsSystem &&
						strings.EqualFold(m.chatMessages[i].Username, msg.TargetLogin) {
						m.chatMessages[i].Greyed = true
						m.chatMessages[i].ModAction = action
					}
				}
			}
			m.chatMessages = append(m.chatMessages, msg.Notice)
			m.rebuildChatViewport()
			return m, waitForIRCEvent(m.chatEventCh, m.chatGeneration)
		}
		return m, nil

	case ChatClearmsgMsg:
		if m.view == viewChat {
			for i := range m.chatMessages {
				if m.chatMessages[i].MsgID == msg.TargetMsgID {
					m.chatMessages[i].Greyed = true
					m.chatMessages[i].ModAction = "deleted"
					break
				}
			}
			m.chatMessages = append(m.chatMessages, msg.Notice)
			m.rebuildChatViewport()
			return m, waitForIRCEvent(m.chatEventCh, m.chatGeneration)
		}
		return m, nil

	case ChatSendDoneMsg:
		if msg.Err != nil {
			m, cmd := m.showErrorDialog("Send failed: " + msg.Err.Error())
			return m, cmd
		}
		// Echo the sent message as a local system entry so it appears inline.
		if m.view == viewChat {
			m.chatMessages = append(m.chatMessages, irc.ChatMessage{
				Text:      "(you) " + msg.Text,
				Timestamp: time.Now(),
				IsSystem:  true,
			})
			m.rebuildChatViewport()
		}
		return m, nil

	case ChatDisconnectedMsg:
		// Ignore stale disconnects from connections that were explicitly cancelled
		// (e.g. the old connection's close event after "c" was pressed).
		if msg.Generation != m.chatGeneration {
			return m, nil
		}
		if m.view == viewChat {
			m.chatConnected = false
			if m.chatAutoReconnect {
				max := m.cfg.Chat.MaxReconnects
				if max > 0 && m.chatReconnectCount >= max {
					// Retry limit reached — stop and tell the user.
					m.chatMessages = append(m.chatMessages, irc.ChatMessage{
						Text:      fmt.Sprintf("Disconnected — max reconnect attempts (%d) reached. Press c to retry.", max),
						Timestamp: time.Now(),
						IsSystem:  true,
					})
					m.rebuildChatViewport()
					return m, nil
				}
				m.chatReconnectCount++
				m.chatMessages = append(m.chatMessages, irc.ChatMessage{
					Text:      fmt.Sprintf("Disconnected — reconnecting in 3s… (attempt %d/%s)", m.chatReconnectCount, reconnectMaxLabel(max)),
					Timestamp: time.Now(),
					IsSystem:  true,
				})
				m.rebuildChatViewport()
				return m, tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
					return ChatReconnectMsg{}
				})
			}
		}
		return m, nil

	case ChatReconnectMsg:
		if m.view == viewChat && m.chatAutoReconnect {
			return m.reconnectChat(false) // auto — keep retrying, count already incremented
		}
		return m, nil

	case ChatLogStartedMsg:
		if msg.Err != nil {
			m, cmd := m.showErrorDialog("Log error: " + msg.Err.Error())
			return m, cmd
		}
		m.chatLogging = true
		m.chatLogFile = msg.File
		m.chatLogPath = msg.Path
		// Arm the blink state to ON and start the dedicated 1-second tick chain.
		// The dot state is now driven solely by chatRecBlink; no other event can
		// affect it.
		m.chatRecBlink = true
		return m, chatRecTickCmd()

	case ChatRecTickMsg:
		// Toggle the blink state and reschedule as long as logging is active.
		// Dropping the tick when logging stops means no lingering goroutines.
		if m.chatLogging {
			m.chatRecBlink = !m.chatRecBlink
			return m, chatRecTickCmd()
		}
		return m, nil

	case MPVLaunchedMsg:
		if msg.Err != nil {
			logging.Warn("Player launch failed", "err", msg.Err)
			m, cmd := m.showErrorDialog("Player error: " + msg.Err.Error())
			return m, cmd
		}
		logging.Info("Player launched")
		return m, nil

	case ClipboardMsg:
		if msg.Err != nil {
			logging.Warn("Clipboard write failed", "err", msg.Err)
			m, cmd := m.showErrorDialog("Clipboard error: " + msg.Err.Error())
			return m, cmd
		}
		logging.Debug("Clipboard write", "url", msg.Text)
		m, cmd := m.showOKDialog("Copied", msg.Text)
		return m, cmd

	case FollowListFetchedMsg:
		if msg.Err != nil {
			logging.Warn("Follow list fetch failed — falling back to config list", "err", msg.Err)
			m, dialogCmd := m.showErrorDialog("Follow list unavailable: " + msg.Err.Error())
			m, refreshCmd := m.launchBatchRefresh()
			return m, tea.Batch(dialogCmd, refreshCmd)
		}
		m.followedList = msg.Logins
		logging.Info("Follow list fetched", "count", len(msg.Logins))
		return m.launchBatchRefresh()

	case SaveConfigDoneMsg:
		if msg.Err != nil {
			logging.Warn("Config save failed", "err", msg.Err)
			m, cmd := m.showErrorDialog("Save failed: " + msg.Err.Error())
			return m, cmd
		}
		logging.Info("Config saved", "token_source", msg.TokenSource)
		// Update token source so the settings field re-renders correctly.
		if m.cfg.OAuthToken != "" {
			m.tokenSource = msg.TokenSource
		} else {
			m.tokenSource = config.TokenSourceNone
		}
		m.apiClient = api.NewClient(m.cfg.ClientID, m.cfg.OAuthToken, m.cfg.RequestTimeoutSec)
		m.cache.SetTTL(time.Duration(m.cfg.CacheOverrideMinutes) * time.Minute)
		// Reset follow list so it is re-fetched on next refresh if token changed.
		m.followedList = nil
		// Restart auto-refresh timer with the (possibly updated) interval.
		// Bump generation so the old pending tick (if any) is discarded.
		m.autoRefreshGen++
		if m.cfg.AutoRefreshMinutes > 0 {
			m.autoRefreshDue = time.Now().Add(time.Duration(m.cfg.AutoRefreshMinutes) * time.Minute)
		} else {
			m.autoRefreshDue = time.Time{}
		}
		refreshCmd := autoRefreshCmd(m.cfg.AutoRefreshMinutes, m.autoRefreshGen)
		var dialogBody string
		switch msg.TokenSource {
		case config.TokenSourceKeyring:
			dialogBody = "Token stored securely in the OS keyring."
		case config.TokenSourceFile:
			dialogBody = "Keyring unavailable — token saved in plaintext."
		default:
			dialogBody = "Settings saved successfully."
		}
		m, dialogCmd := m.showOKDialog("Saved", dialogBody)
		return m, tea.Batch(dialogCmd, refreshCmd)

	case ErrMsg:
		logging.Error("Unhandled error", "err", msg.Err)
		m, cmd := m.showErrorDialog(msg.Err.Error())
		return m, cmd

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	// Forward any unmatched message to an active dialog so that huh's internal
	// state-machine messages (nextFieldMsg, nextGroupMsg etc.) are processed.
	// Key messages reach here too, but the dialog key handling in handleKey
	// returns before reaching this point, so there is no double-dispatch.
	if m.activeDialog != nil {
		updated, cmd := m.activeDialog.Update(msg)
		if f, ok := updated.(*huh.Form); ok {
			m.activeDialog = f
		}
		m = m.applyDialogState()
		return m, cmd
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Modal dialog helpers
// ---------------------------------------------------------------------------

// dialogTheme returns a huh.Theme tailored for the app's dialog overlays.
//
// Changes from the default ThemeCharm:
//   - Focused.Base and Blurred.Base have no border — the overlay box already
//     frames the dialog with a rounded purple border.
//   - FocusedButton: solid purple (#9147FF) background, white text — clearly
//     shows which option the cursor is on.
//   - BlurredButton: dark-but-visible background (#3A3A4A) with muted text so
//     the unselected option is readable without competing with the selection.
//   - Title uses PurpleLight to match the app chrome.
func dialogTheme() huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		t := huh.ThemeBase(isDark)

		// Remove the inner thick left-border — our rounded overlay box is the frame.
		t.Focused.Base = lipgloss.NewStyle()
		t.Blurred.Base = lipgloss.NewStyle()

		// Title and description colors.
		t.Focused.Title = lipgloss.NewStyle().Foreground(styles.PurpleLight).Bold(true)
		t.Blurred.Title = t.Focused.Title
		t.Focused.Description = lipgloss.NewStyle().Foreground(styles.ColorText)
		t.Blurred.Description = t.Focused.Description

		// Focused (selected) button — purple fill, white bold text.
		focused := lipgloss.NewStyle().
			Padding(0, 3).
			MarginRight(1).
			Background(styles.Purple).
			Foreground(styles.ColorText).
			Bold(true)
		// Blurred (unselected) button — dark fill, dim text; clearly readable
		// but visually subordinate to the focused button.
		blurred := lipgloss.NewStyle().
			Padding(0, 3).
			MarginRight(1).
			Background(lipgloss.Color("#3A3A4A")).
			Foreground(styles.ColorTextMuted)

		t.Focused.FocusedButton = focused
		t.Blurred.FocusedButton = focused
		t.Focused.BlurredButton = blurred
		t.Blurred.BlurredButton = blurred

		return t
	})
}

// dialogWidth returns the preferred dialog inner width based on terminal size.
func (m Model) dialogWidth() int {
	w := m.width
	if w <= 0 {
		w = 80
	}
	dw := w - 8
	if dw > 56 {
		dw = 56
	}
	if dw < 24 {
		dw = 24
	}
	return dw
}

// showErrorDialog opens a modal "Error" dialog with a single Okay button.
func (m Model) showErrorDialog(body string) (Model, tea.Cmd) {
	return m.showOKDialog("Error", body)
}

// showOKDialog opens a modal info/acknowledgment dialog with a single Okay
// button.  Any key (enter / y / esc) dismisses it.
func (m Model) showOKDialog(title, body string) (Model, tea.Cmd) {
	var ok bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Key("ok").
				Accessor(huh.NewPointerAccessor(&ok)).
				Title(title).
				Description(body).
				Affirmative("Okay").
				Negative(""),
		),
	).WithWidth(m.dialogWidth()).WithTheme(dialogTheme()).WithShowHelp(false)
	m.activeDialog = form
	m.dialogOnComplete = nil
	return m, form.Init()
}

// showConfirmDialog opens a modal Yes/No dialog.  onComplete is called with
// confirmed=true when the user selects Yes, or false when No / esc is pressed.
func (m Model) showConfirmDialog(title, body string, onComplete func(Model, bool) Model) (Model, tea.Cmd) {
	var confirmed bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Key("confirm").
				Accessor(huh.NewPointerAccessor(&confirmed)).
				Title(title).
				Description(body).
				Affirmative("Yes").
				Negative("No"),
		),
	).WithWidth(m.dialogWidth()).WithTheme(dialogTheme()).WithShowHelp(false)
	m.activeDialog = form
	m.dialogOnComplete = onComplete
	return m, form.Init()
}

// applyDialogState checks whether activeDialog has reached a terminal state
// (completed or aborted) and invokes dialogOnComplete if present.
func (m Model) applyDialogState() Model {
	if m.activeDialog == nil {
		return m
	}
	var terminal bool
	var confirmed bool
	switch m.activeDialog.State {
	case huh.StateCompleted:
		confirmed = m.activeDialog.GetBool("confirm")
		terminal = true
	case huh.StateAborted:
		confirmed = false
		terminal = true
	}
	if terminal {
		cb := m.dialogOnComplete
		m.activeDialog = nil
		m.dialogOnComplete = nil
		if cb != nil {
			m = cb(m, confirmed)
		}
	}
	return m
}

// renderDialogOverlay renders the active dialog centered on a near-black
// background, replacing the normal view content while the dialog is open.
func (m Model) renderDialogOverlay(w, h int) string {
	if m.activeDialog == nil {
		return ""
	}
	formView := m.activeDialog.View()
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Purple).
		Padding(0, 1).
		Render(formView)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(lipgloss.Color("#111111"))))
}

func (m Model) View() tea.View {
	w := m.width
	if w == 0 {
		w = 80
	}
	h := m.height
	if h == 0 {
		h = 24
	}

	// Modal dialog overlays everything — render it full-screen on a dark bg.
	if m.activeDialog != nil {
		v := tea.NewView(m.renderDialogOverlay(w, h))
		v.AltScreen = true
		return v
	}

	if m.isRefreshing {
		v := tea.NewView(m.renderLoading(w))
		v.AltScreen = true
		return v
	}

	header := m.renderHeader(w)
	var body string

	switch m.view {
	case viewList:
		liveN := countLive(m.streamers)
		nCols := m.gridNumCols()
		// elapsedSecs extends the GQL-snapshot UptimeSec with wall-clock time since refresh.
		var elapsedSecs int64
		if !m.refreshedAt.IsZero() {
			elapsedSecs = int64(time.Since(m.refreshedAt).Seconds())
		}
		// cursorPosF springs toward listCursor — pass rounded value as visual selection.
		visualCursor := int(math.Round(m.cursorPosF))
		liveColor := lipgloss.Color(m.cfg.Display.CardLiveColor)
		selectColor := lipgloss.Color(m.cfg.Display.CardSelectColor)

		if m.isListMode() {
			numTables := m.cfg.Display.ListTableCount
			if numTables < 1 {
				numTables = 1
			}
			body = views.RenderListView(
				m.streamers,
				m.listFilter, m.searchBuf,
				m.searchActive,
				visualCursor,
				numTables,
				m.listViewRowsPerTable(),
				m.listOffset,
				liveColor, selectColor,
				w,
				liveN, len(m.streamers),
				elapsedSecs,
				m.autoRefreshDue,
			)
		} else {
			logicalRow := m.listOffset / nCols
			body = views.RenderCardGrid(
				m.streamers,
				m.listFilter, m.searchBuf,
				m.searchActive,
				visualCursor,
				m.cfg.Display.CardWidth, nCols,
				m.cfg.Display.CardPadH, m.cfg.Display.CardPadV,
				liveColor, selectColor,
				w,
				liveN, len(m.streamers),
				m.listOffset/nCols, logicalRow, m.visibleGridRows(),
				m.accentPos,
				elapsedSecs,
				m.autoRefreshDue,
			)
		}
	case viewDetails:
		body = views.RenderDetails(m.selectedUser, m.detailData, m.lastSeen, m.detailLoading, m.detailFollowers, w)
	case viewVODs:
		body = views.RenderVODs(m.selectedUser, m.vods, m.chapters, m.vodsLoading,
			m.vodSelected, m.vodOffset, m.vodVisibleRows(), m.vodHasMore, m.vodLoadingMore,
			m.vodNumBuf, m.cfg.Display.VodCategoriesMaxShown, w)
	case viewSettings:
		if m.settings.Section == views.SectionHelp {
			body = views.RenderSettingsHelp(&m.settings, w, m.height-m.headerHeight()-1)
		} else {
			body = views.RenderSettings(m.settings, w, m.height-m.headerHeight()-1)
		}
	case viewChat:
		v := tea.NewView(views.RenderChat(m.chatChannel, m.chatRoomState, m.chatConnected, m.chatLoading, m.chatLogging, m.chatRecBlink, m.chatUnread, m.chatScrollMode, m.chatStreamerOnline, m.cfg.Chat.TextBadges, m.chatSearchActive, m.chatSearchBuf, m.chatMatchCount(), m.chatMode, m.chatInputActive, m.chatInputBuf, m.chatViewport, w))
		v.AltScreen = true
		return v
	}

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, body))
	v.AltScreen = true
	return v
}

// chatMatchCount returns the number of messages in the buffer that match the
// current chat search query.  Returns total message count when no filter is active.
func (m Model) chatMatchCount() int {
	if m.chatSearchQuery == nil {
		return len(m.chatMessages)
	}
	n := 0
	for _, msg := range m.chatMessages {
		if views.MatchChatMessage(msg, m.chatSearchQuery) {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// Ctrl+C always quits
	if k == "ctrl+c" {
		return m, tea.Quit
	}

	// Modal dialog intercepts ALL keys regardless of view.
	if m.activeDialog != nil {
		updated, cmd := m.activeDialog.Update(msg)
		if f, ok := updated.(*huh.Form); ok {
			m.activeDialog = f
		}
		m = m.applyDialogState()
		return m, cmd
	}

	if m.view == viewSettings {
		return m.updateSettings(msg)
	}

	switch m.view {
	case viewList:
		return m.updateList(msg)
	case viewDetails:
		return m.updateDetails(msg)
	case viewVODs:
		return m.updateVODs(msg)
	case viewChat:
		return m.updateChat(msg)
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	items := m.visibleItems()
	// --- Search mode: / opens it, esc closes it ---
	if m.searchActive {
		switch k {
		case "esc":
			m.searchActive = false
			m.searchBuf = ""
			m.listCursor = 0
			m.listOffset = 0
		case "backspace":
			if len(m.searchBuf) > 0 {
				runes := []rune(m.searchBuf)
				m.searchBuf = string(runes[:len(runes)-1])
				m.listCursor = 0
				m.listOffset = 0
			}
		case "enter":
			return m.selectStreamer(m.listCursor)
		// Arrow keys navigate the filtered results; letter keys (j/k/h/l) type
		// as search text so streamer names containing those letters are reachable.
		case "up":
			sNumCols := m.gridNumCols()
			if m.listCursor >= sNumCols {
				m.listCursor -= sNumCols
				curRow := m.listCursor / sNumCols
				if curRow < m.listOffset/sNumCols {
					m.listOffset -= sNumCols
					if m.listOffset < 0 {
						m.listOffset = 0
					}
				}
			}
		case "down":
			sNumCols := m.gridNumCols()
			if m.listCursor+sNumCols < len(items) {
				m.listCursor += sNumCols
				curRow := m.listCursor / sNumCols
				if curRow >= m.listOffset/sNumCols+m.visibleGridRows() {
					m.listOffset += sNumCols
				}
			}
		case "left":
			if m.listCursor > 0 {
				m.listCursor--
			}
		case "right":
			if m.listCursor < len(items)-1 {
				m.listCursor++
			}
		default:
			if isPrintable(k) {
				m.searchBuf += k
				m.listCursor = 0
				m.listOffset = 0
			}
		}
			return m, nil
	}

	// --- Normal (non-search) mode ---
	numCols := m.gridNumCols()
	visRows := m.visibleGridRows()

	switch k {
	case "up", "k":
		if m.listCursor >= numCols {
			m.listCursor -= numCols
			curRow := m.listCursor / numCols
			if curRow < m.listOffset/numCols {
				m.listOffset -= numCols
				if m.listOffset < 0 {
					m.listOffset = 0
				}
			}
		}

	case "down", "j":
		if m.listCursor+numCols < len(items) {
			m.listCursor += numCols
			curRow := m.listCursor / numCols
			if curRow >= m.listOffset/numCols+visRows {
				m.listOffset += numCols
			}
		}

	case "left", "h":
		if m.isListMode() {
			// Jump to the same row in the previous table (step = rowsPerTable).
			step := m.listViewRowsPerTable()
			if m.listCursor >= step {
				m.listCursor -= step
				if m.listCursor < m.listOffset {
					m.listOffset -= step
					if m.listOffset < 0 {
						m.listOffset = 0
					}
				}
			}
		} else {
			if m.listCursor > 0 {
				m.listCursor--
				curRow := m.listCursor / numCols
				if curRow < m.listOffset/numCols {
					m.listOffset -= numCols
					if m.listOffset < 0 {
						m.listOffset = 0
					}
				}
			}
		}

	case "right", "l":
		if m.isListMode() {
			// Jump to the same row in the next table (step = rowsPerTable).
			step := m.listViewRowsPerTable()
			if m.listCursor+step < len(items) {
				m.listCursor += step
				if m.listCursor >= m.listOffset+visRows {
					m.listOffset += step
				}
			}
		} else {
			if m.listCursor < len(items)-1 {
				m.listCursor++
				curRow := m.listCursor / numCols
				if curRow >= m.listOffset/numCols+visRows {
					m.listOffset += numCols
				}
			}
		}

	case "enter":
		return m.selectStreamer(m.listCursor)

	case "/":
		m.searchActive = true

	case "q":
		return m, tea.Quit

	case "r":
		return m.startRefresh()

	case "s":
		return m.openSettings()

	case "o":
		m.listFilter = "live"
		m.listCursor = 0
		m.listOffset = 0

	case "f":
		m.listFilter = "offline"
		m.listCursor = 0
		m.listOffset = 0

	case "a":
		m.listFilter = "all"
		m.listCursor = 0
		m.listOffset = 0
	}

	return m, nil
}

func (m Model) updateDetails(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.view = viewList
		// Restart the 60 Hz tick chain (it stops while on sub-screens) and snap
		// the cursor spring to its target so the selection highlight is
		// immediately correct rather than frozen at a stale mid-animation position.
		m.cursorPosF = float64(m.listCursor)
		m.cursorVelF = 0
		return m, cardAnimCmd()
	case "v":
		return m.openVODs()
	case "p":
		if m.detailData != nil {
			url := fmt.Sprintf("https://www.twitch.tv/%s", m.selectedUser)
			return m, launchPlayerCmd(url, m.cfg)
		}
		m, cmd := m.showErrorDialog("Streamer is offline")
		return m, cmd
	case "t":
		return m.openChat()
	case "P":
		return m.playLatestVOD()
	case "c":
		url := fmt.Sprintf("https://www.twitch.tv/%s", m.selectedUser)
		return m, copyCmd(url, m.clipboard)
	case "s":
		return m.openSettings()
	}
	return m, nil
}

func (m Model) playLatestVOD() (Model, tea.Cmd) {
	// Use cached VODs if available.
	if entry, ok := m.cache.Get(m.selectedUser); ok && len(entry.VODs) > 0 {
		logging.Info("Playing latest VOD from cache", "user", m.selectedUser)
		return m, launchPlayerCmd(views.VODUrl(entry.VODs[0].ID), m.cfg)
	}
	logging.Info("Fetching latest VOD to play", "user", m.selectedUser)
	return m, fetchLatestVODCmd(m.apiClient, m.selectedUser)
}

func (m Model) openVODs() (Model, tea.Cmd) {
	m.view = viewVODs
	m.vodSelected = 0
	m.vodOffset = 0
	m.vodNumBuf = ""

	if entry, ok := m.cache.Get(m.selectedUser); ok && len(entry.VODs) > 0 {
		m.vodsLoading = false
		m.vods = entry.VODs
		m.chapters = entry.Chapters
		m.vodHasMore = entry.VODHasMore
		return m, nil
	}
	m.vodsLoading = true
	m.vods = nil
	m.chapters = nil
	m.vodHasMore = false
	m.vodLoadingMore = false
	return m, fetchVODsCmd(m.apiClient, m.selectedUser, m.cfg)
}

func (m Model) updateVODs(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// Total navigable entries = vods + Load More entry (when applicable).
	showLoadMore := m.vodHasMore || m.vodLoadingMore
	totalEntries := len(m.vods)
	if showLoadMore {
		totalEntries++
	}

	// Digit accumulation for Np commands (meaningful only for actual VOD rows).
	if isDigit(k) {
		m.vodNumBuf += k
		return m, nil
	}

	switch k {
	case "esc", "backspace":
		m.view = viewDetails
		m.vodNumBuf = ""

	case "up", "k":
		m.vodNumBuf = ""
		if m.vodSelected > 0 {
			m.vodSelected--
			m = m.scrollVODsToSelection()
		}

	case "down", "j":
		m.vodNumBuf = ""
		if m.vodSelected < totalEntries-1 {
			m.vodSelected++
			m = m.scrollVODsToSelection()
		}

	case "enter":
		// If Load More is focused, trigger a page fetch.
		if showLoadMore && m.vodSelected == len(m.vods) {
			if !m.vodLoadingMore && m.vodHasMore {
				m.vodLoadingMore = true
				return m, fetchMoreVODsCmd(m.apiClient, m.selectedUser, len(m.vods), m.cfg)
			}
			return m, nil
		}
		// Normal VOD: play selected (or Np play)
		idx := m.vodSelected
		if nb, ok := parseNumBuf(m.vodNumBuf, len(m.vods)); ok {
			idx = nb
		}
		m.vodNumBuf = ""
		if idx >= 0 && idx < len(m.vods) {
			return m, launchPlayerCmd(views.VODUrl(m.vods[idx].ID), m.cfg)
		}

	case "c":
		// c = copy URL of selected VOD (or Nc = copy #N); no-op on Load More row.
		idx := m.vodSelected
		if nb, ok := parseNumBuf(m.vodNumBuf, len(m.vods)); ok {
			idx = nb
		}
		m.vodNumBuf = ""
		if idx >= 0 && idx < len(m.vods) {
			return m, copyCmd(views.VODUrl(m.vods[idx].ID), m.clipboard)
		}

	case "s":
		m.vodNumBuf = ""
		return m.openSettings()

	default:
		m.vodNumBuf = ""
	}
	return m, nil
}

// scrollVODsToSelection adjusts vodOffset so that the selected row is always
// within the visible window. The Load More entry (at index len(vods)) is
// counted as one row for purposes of this calculation.
func (m Model) scrollVODsToSelection() Model {
	showLoadMore := m.vodHasMore || m.vodLoadingMore
	totalEntries := len(m.vods)
	if showLoadMore {
		totalEntries++
	}
	if totalEntries == 0 {
		return m
	}

	// Clamp selection to valid range.
	if m.vodSelected < 0 {
		m.vodSelected = 0
	}
	if m.vodSelected >= totalEntries {
		m.vodSelected = totalEntries - 1
	}

	vc := m.vodVisibleRows()
	if vc < 1 {
		vc = 1
	}

	if m.vodSelected < m.vodOffset {
		m.vodOffset = m.vodSelected
	} else if m.vodSelected >= m.vodOffset+vc {
		m.vodOffset = m.vodSelected - vc + 1
	}

	maxOffset := totalEntries - vc
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.vodOffset > maxOffset {
		m.vodOffset = maxOffset
	}
	if m.vodOffset < 0 {
		m.vodOffset = 0
	}
	return m
}



