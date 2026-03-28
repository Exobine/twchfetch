package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"

	"twchfetch/internal/irc"
	"twchfetch/internal/tui/views"
)

func (m Model) openChat() (Model, tea.Cmd) {
	// Twitch IRC (TMI WebSocket) channels persist whether or not the streamer
	// is live — anonymous justinfan connections work regardless of stream status.
	// Note: some channels may restrict chat to subscribers/followers even when
	// offline; the IRC server will still accept the connection and deliver any
	// ROOMSTATE/USERNOTICE events, but PRIVMSG delivery depends on channel settings.
	// We record whether the streamer was live so the header can show [offline].
	if m.selectedUser == "" {
		m, cmd := m.showErrorDialog("No streamer selected")
		return m, cmd
	}

	// Cancel any existing chat connection before opening a new one.
	if m.chatCancel != nil {
		m.chatCancel()
		m.chatCancel = nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.chatCancel = cancel
	m.chatChannel = strings.ToLower(m.selectedUser)
	m.chatStreamerOnline = m.detailData != nil // nil → streamer is offline
	m.chatMessages = nil
	m.chatConnected = false
	m.chatLoading = true
	m.chatAtBottom = true
	m.chatScrollMode = views.ScrollModeLive
	m.chatGeneration++
	m.chatAutoReconnect = true
	m.chatReconnectCount = 0
	m.chatRoomState = irc.RoomState{FollowersOnly: -1} // -1 = disabled; avoids false [followers-only] before ROOMSTATE arrives
	m.chatUnread = 0
	m.chatSearchActive = false
	m.chatSearchBuf = ""
	m.chatSearchQuery = nil
	m.chatMode = views.ChatModeLurk
	m.chatIRCClient = nil
	m.chatInputActive = false
	m.chatInputBuf = ""
	m.closeChatLog()
	m.thirdPartyEmotes = nil // clear stale emote set from any previous channel
	m.view = viewChat

	// Prepend an offline notice so the user knows why activity may be sparse.
	if !m.chatStreamerOnline {
		m.chatMessages = append(m.chatMessages, irc.ChatMessage{
			Text:      "Stream is offline — chat activity may be limited",
			Timestamp: time.Now(),
			IsSystem:  true,
		})
	}

	// Size the viewport to fill the space between the fixed chrome lines,
	// then immediately rebuild so any pre-loaded system messages are visible
	// on the very first render rather than waiting for the first IRC event.
	vp := viewport.New(viewport.WithWidth(m.width), viewport.WithHeight(chatViewportHeight(m.height, 0)))
	m.chatViewport = vp
	m.rebuildChatViewport()

	ch := make(chan irc.Event, 128)
	m.chatEventCh = ch
	cmds := []tea.Cmd{connectChatCmd(ctx, m.chatChannel, ch, m.cfg.Chat.StripDingbats, m.cfg.OAuthToken, m.chatGeneration)}
	if m.cfg.Chat.ThirdPartyEmotes {
		cmds = append(cmds, fetchEmotesCmd(m.apiClient, m.chatChannel))
	}
	return m, tea.Batch(cmds...)
}

// reconnectChat cancels any live connection and starts a fresh one, preserving
// the existing message history.
//
// manual=true  (user pressed "c"): disables auto-reconnect for this session and
//              resets the retry counter so the user starts fresh.  The old
//              connection's close event will carry a stale generation and be
//              silently ignored, preventing a spurious auto-retry.
//
// manual=false (fired by ChatReconnectMsg timer): auto-reconnect continues;
//              the retry counter is NOT reset so the count increments correctly.
func (m Model) reconnectChat(manual bool) (Model, tea.Cmd) {
	if m.chatCancel != nil {
		m.chatCancel()
		m.chatCancel = nil
	}
	// Increment generation BEFORE starting the new connection so that the
	// close event from the just-cancelled connection carries the old generation
	// and is discarded by the ChatDisconnectedMsg handler.
	m.chatGeneration++

	ctx, cancel := context.WithCancel(context.Background())
	m.chatCancel = cancel
	m.chatConnected = false
	m.chatLoading = true
	// Always re-enable auto-reconnect.  The generation counter incremented
	// above ensures that the stale close-event from the just-cancelled
	// connection carries the old generation and is silently ignored, so there
	// is no cascade regardless of whether this is a manual or automatic call.
	m.chatAutoReconnect = true

	if manual {
		// User pressed "c" — give the new session a fresh retry budget.
		m.chatReconnectCount = 0
	}
	// For auto-reconnect: counter was already incremented by the
	// ChatDisconnectedMsg handler before this function was called.

	m.chatMessages = append(m.chatMessages, irc.ChatMessage{
		Text:      "Connecting…",
		Timestamp: time.Now(),
		IsSystem:  true,
	})
	m.rebuildChatViewport()

	ch := make(chan irc.Event, 128)
	m.chatEventCh = ch
	return m, connectChatCmd(ctx, m.chatChannel, ch, m.cfg.Chat.StripDingbats, m.cfg.OAuthToken, m.chatGeneration)
}

func (m Model) updateChat(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// --- Tier 1: Input bar is open ---
	if m.chatInputActive {
		switch k {
		case "esc":
			m.chatInputActive = false
			m.chatInputBuf = ""
			m.syncChatViewportHeight()
			m.rebuildChatViewport()
			return m, nil
		case "enter":
			text := strings.TrimSpace(m.chatInputBuf)
			m.chatInputActive = false
			m.chatInputBuf = ""
			m.syncChatViewportHeight()
			m.rebuildChatViewport()
			if text != "" && m.chatIRCClient != nil {
				return m, sendChatMessageCmd(m.chatIRCClient, m.chatChannel, text)
			}
			return m, nil
		case "backspace":
			if len(m.chatInputBuf) > 0 {
				runes := []rune(m.chatInputBuf)
				m.chatInputBuf = string(runes[:len(runes)-1])
			}
			return m, nil
		default:
			if isPrintable(k) {
				m.chatInputBuf += k
			}
			return m, nil
		}
	}

	// --- Tier 3: Search bar is open ---
	if m.chatSearchActive {
		switch k {
		case "esc":
			// Close the bar AND clear the filter (same behaviour as list search).
			m.chatSearchActive = false
			m.chatSearchBuf = ""
			m.chatSearchQuery = nil
			m.chatUnread = 0
			m.rebuildChatViewport()
			return m, nil
		case "enter":
			// Close the bar but keep the filter active.
			m.chatSearchActive = false
			return m, nil
		case "backspace":
			if len(m.chatSearchBuf) > 0 {
				runes := []rune(m.chatSearchBuf)
				m.chatSearchBuf = string(runes[:len(runes)-1])
				m.chatSearchQuery = views.ParseSearchQuery(m.chatSearchBuf)
				m.chatUnread = 0
				m.rebuildChatViewport()
			}
			return m, nil
		default:
			if isPrintable(k) {
				m.chatSearchBuf += k
				m.chatSearchQuery = views.ParseSearchQuery(m.chatSearchBuf)
				m.chatUnread = 0
				m.rebuildChatViewport()
			}
			return m, nil
		}
	}

	// --- Tier 4: Normal chat key handling ---
	switch k {
	case "esc", "backspace":
		m.chatAutoReconnect = false // prevent reconnect after intentional exit
		if m.chatCancel != nil {
			m.chatCancel()
			m.chatCancel = nil
		}
		m.closeChatLog()
		m.view = viewDetails
		return m, nil

	case "/":
		m.chatSearchActive = true
		return m, nil

	case "m":
		// Toggle chat mode: lurk ↔ normal.
		if m.chatMode == views.ChatModeNormal {
			// Switch back to lurk immediately — no confirmation needed.
			m.chatMode = views.ChatModeLurk
			m.chatInputActive = false
			m.chatInputBuf = ""
			m.syncChatViewportHeight()
			m.rebuildChatViewport()
		} else {
			// Guard: an OAuth token is required to send PRIVMSGs.
			if m.cfg.OAuthToken == "" {
				m, cmd := m.showErrorDialog("OAuth token required to send messages — set one in Settings")
				return m, cmd
			}
			// Lurk → normal: show huh confirmation dialog.
			m, cmd := m.showConfirmDialog(
				"Enable send mode?",
				"Messages you type will be sent to the channel.\nYou must be authenticated (OAuth token set) to send.",
				func(m Model, confirmed bool) Model {
					if confirmed {
						m.chatMode = views.ChatModeNormal
						m.syncChatViewportHeight()
						m.rebuildChatViewport()
					}
					return m
				},
			)
			return m, cmd
		}

	case "shift+enter":
		// Open the input bar only in normal mode.
		if m.chatMode == views.ChatModeNormal {
			m.chatInputActive = true
			m.chatInputBuf = ""
			m.syncChatViewportHeight()
			m.rebuildChatViewport()
		}

	case "c":
		// Manual reconnect: fresh start, new retry budget.
		return m.reconnectChat(true)

	case "h":
		// Cycle: live (0) → locked (1) → free (2) → live (0).
		m.chatScrollMode = (m.chatScrollMode + 1) % 3
		if m.chatScrollMode == views.ScrollModeLive {
			// Returning to live: snap to bottom and clear unread immediately.
			m.chatViewport.GotoBottom()
			m.chatAtBottom = true
			m.chatUnread = 0
		}

	case "g":
		m.chatViewport.GotoTop()
		m.chatAtBottom = false
		m.clampUnread()

	case "G":
		// Explicit bottom jump always returns to live mode and clears unread.
		m.chatScrollMode = views.ScrollModeLive
		m.chatViewport.GotoBottom()
		m.chatAtBottom = true
		m.chatUnread = 0

	case "x":
		return m.toggleChatLogging()

	default:
		// Scroll keys work in all modes.
		var cmd tea.Cmd
		m.chatViewport, cmd = m.chatViewport.Update(msg)
		m.chatAtBottom = m.chatViewport.AtBottom()
		m.clampUnread()
		return m, cmd
	}
	return m, nil
}

// sendChatMessageCmd sends a PRIVMSG to the channel and returns ChatSendDoneMsg.
func sendChatMessageCmd(client *irc.Client, channel, text string) tea.Cmd {
	return func() tea.Msg {
		err := client.Send(channel, text)
		return ChatSendDoneMsg{Text: text, Err: err}
	}
}

// closeChatLog closes the active log file if one is open.
func (m *Model) closeChatLog() {
	if m.chatLogFile != nil {
		m.chatLogFile.Close()
		m.chatLogFile = nil
	}
	m.chatLogging = false
	m.chatLogPath = ""
}

// toggleChatLogging starts or stops chat logging.
func (m Model) toggleChatLogging() (tea.Model, tea.Cmd) {
	if m.chatLogging {
		// Stop logging.
		path := m.chatLogPath
		m.closeChatLog()
		m, cmd := m.showOKDialog("Log Saved", filepath.Base(path))
		return m, cmd
	}
	// Start logging — open file in background cmd so any I/O error surfaces cleanly.
	dir := filepath.Dir(m.cfgPath)
	return m, startChatLoggingCmd(dir, m.chatChannel, m.chatMessages)
}

// startChatLoggingCmd opens a new log file, writes the current message buffer,
// and returns ChatLogStartedMsg so the model can store the *os.File pointer.
func startChatLoggingCmd(dir, channel string, messages []irc.ChatMessage) tea.Cmd {
	return func() tea.Msg {
		ts := time.Now().Format("2006-01-02_15-04-05")
		name := fmt.Sprintf("chat_%s_%s.txt", channel, ts)
		path := filepath.Join(dir, name)
		f, err := os.Create(path)
		if err != nil {
			return ChatLogStartedMsg{Err: err}
		}
		// Write buffered messages that existed before logging was toggled on.
		for _, msg := range messages {
			if line := views.FormatLogLine(msg); line != "" {
				fmt.Fprintln(f, line)
			}
		}
		return ChatLogStartedMsg{Path: path, File: f}
	}
}

// reconnectMaxLabel formats the max-reconnect limit as "∞" when unlimited (0)
// or the numeric string otherwise.  Used in the inline reconnect status line.
func reconnectMaxLabel(max int) string {
	if max == 0 {
		return "∞"
	}
	return fmt.Sprintf("%d", max)
}

// chatViewportHeight returns the number of lines the chat viewport should
// occupy given the total terminal height.  RenderChat has 5 fixed chrome
// lines (rule + sub-header + rule + rule + bottom-bar), plus any extra lines
// consumed by the input bar or confirmation overlay (passed as extraBottomLines).
func chatViewportHeight(termHeight, extraBottomLines int) int {
	h := termHeight - 5 - extraBottomLines
	if h < 1 {
		h = 1
	}
	return h
}

// chatExtraBottomLines returns how many lines the current chat overlay
// (input bar) consumes below the viewport.
func (m Model) chatExtraBottomLines() int {
	if m.chatInputActive || (m.chatMode == views.ChatModeNormal && m.chatInputBuf != "") {
		return 1 // the input bar occupies exactly 1 extra line (border-bottom)
	}
	return 0
}

// syncChatViewportHeight resizes the viewport whenever the chrome changes
// (input bar opens/closes, confirm form appears/disappears).
func (m *Model) syncChatViewportHeight() {
	m.chatViewport.SetHeight(chatViewportHeight(m.height, m.chatExtraBottomLines()))
}

// rebuildChatViewport regenerates the viewport with no top-trim adjustment.
// Call rebuildChatViewportWithTrim when messages have been removed from the
// front of the buffer in the same update cycle.
func (m *Model) rebuildChatViewport() {
	m.rebuildChatViewportWithTrim(0)
}

// rebuildChatViewportWithTrim regenerates the viewport content from the current
// message buffer, then positions it according to the active scroll mode.
//
// topTrimLines is the number of rendered content lines that were removed from
// the front of the ring-buffer in this update cycle (0 when no trim occurred).
// It is used to compensate the saved offset so the viewport stays anchored to
// the same visual content rather than drifting toward old — now-deleted — lines
// and causing clampUnread to falsely lower the unread counter.
//
//	views.ScrollModeLive   — always jumps to the latest message; unread cleared.
//	views.ScrollModeFrozen — restores the offset minus trimmed lines; new content
//	                   queues below the fold; manual scroll always works.
//	views.ScrollModeFollow — advances by lines added at the bottom only (not net
//	                   delta), so new messages slide into view one-by-one
//	                   without snapping to the bottom or jumping backward when
//	                   the trimmed message was longer than the new one.
func (m *Model) rebuildChatViewportWithTrim(topTrimLines int) {
	content := views.BuildChatContent(m.chatMessages, m.cfg.Chat.EmoteColors, m.cfg.Chat.ShowReply, m.cfg.Chat.TrimReplyMention, m.cfg.Chat.LocalizedNames, m.cfg.Chat.TextBadges, m.cfg.Chat.AltRowColor, m.chatViewport.Width(), m.chatSearchQuery, m.thirdPartyEmotes, m.cfg.Chat.ThirdPartyShading)

	switch m.chatScrollMode {
	case views.ScrollModeLive:
		m.chatViewport.SetContent(content)
		m.chatViewport.GotoBottom()
		m.chatAtBottom = true
		m.chatUnread = 0

	case views.ScrollModeFrozen:
		saved := m.chatViewport.YOffset()
		m.chatViewport.SetContent(content)
		// Subtract lines removed from the top so the viewport stays on the same
		// visual content.  If the trim consumed lines up to or past the saved
		// offset (user was near the top), clamp to 0 — content flows naturally.
		adjusted := saved - topTrimLines
		if adjusted < 0 {
			adjusted = 0
		}
		m.chatViewport.SetYOffset(adjusted)
		m.chatAtBottom = m.chatViewport.AtBottom()
		m.clampUnread()

	case views.ScrollModeFollow:
		oldTotal := m.chatViewport.TotalLineCount()
		saved := m.chatViewport.YOffset()
		m.chatViewport.SetContent(content)
		newTotal := m.chatViewport.TotalLineCount()
		// Decompose the net line change into its two parts:
		//   netDelta = linesAdded - topTrimLines
		//   → linesAdded = netDelta + topTrimLines
		// Using net delta alone would move the viewport backward when a long
		// trimmed message shrinks the total more than the new message grows it.
		linesAdded := (newTotal - oldTotal) + topTrimLines
		if linesAdded < 0 {
			linesAdded = 0
		}
		// Adjust base offset for top trim, then advance by lines added at bottom.
		adjusted := saved - topTrimLines
		if adjusted < 0 {
			adjusted = 0
		}
		m.chatViewport.SetYOffset(adjusted + linesAdded)
		m.chatAtBottom = m.chatViewport.AtBottom()
		m.clampUnread()
	}
}

// clampUnread reduces chatUnread to at most the number of content lines that
// sit below the current visible fold.  Messages that have scrolled into view
// are immediately deducted; safe to call from both key handlers and rebuilds.
func (m *Model) clampUnread() {
	linesBelow := m.chatViewport.TotalLineCount() - m.chatViewport.YOffset() - m.chatViewport.Height()
	if linesBelow < 0 {
		linesBelow = 0
	}
	if m.chatUnread > linesBelow {
		m.chatUnread = linesBelow
	}
}

// connectChatCmd dials the IRC server in a Cmd goroutine and returns
// ChatConnectedMsg on success or ChatDisconnectedMsg on failure.
// gen is stamped on the disconnect message so stale failures from cancelled
// connections are ignored by the model.
func connectChatCmd(ctx context.Context, channel string, ch chan<- irc.Event, stripDingbats bool, oauthToken string, gen int) tea.Cmd {
	return func() tea.Msg {
		opts := irc.ClientOptions{
			StripDingbats: stripDingbats,
			OAuthToken:    oauthToken,
		}
		client, err := irc.Connect(ctx, channel, ch, opts)
		if err != nil {
			return ChatDisconnectedMsg{Err: err, Generation: gen}
		}
		go client.ReadLoop()
		return ChatConnectedMsg{Client: client}
	}
}

// waitForIRCEvent blocks on the events channel and converts each IRC event
// into the appropriate tea.Msg.  Must be re-issued after each message is
// processed so that the next event is consumed.
//
// gen is the chatGeneration at the time this Cmd was issued.  It is stamped
// onto ChatDisconnectedMsg so the model can distinguish a natural disconnect
// from the stale close of an old cancelled connection.
func waitForIRCEvent(ch <-chan irc.Event, gen int) tea.Cmd {
	return func() tea.Msg {
		for {
			e, ok := <-ch
			if !ok {
				return ChatDisconnectedMsg{Generation: gen}
			}
			switch ev := e.(type) {
			case irc.ChatMessageEvent:
				return ChatMsg{Message: ev.Msg}
			case irc.RoomStateEvent:
				return ChatRoomStateMsg{State: ev.State}
			case irc.NoticeEvent:
				// Convert server notices into a system ChatMessage so they
				// appear inline in the chat viewport.
				return ChatMsg{Message: irc.ChatMessage{
					Text:      ev.Text,
					Timestamp: time.Now(),
					IsSystem:  true,
				}}
			case irc.UsernoticeEvent:
				return ChatUsernoticeMsg{Message: ev.Msg}
			case irc.ClearchatEvent:
				return ChatClearchatMsg{
					TargetLogin: ev.TargetLogin,
					Duration:    ev.Duration,
					Notice:      ev.Notice,
				}
			case irc.ClearmsgEvent:
				return ChatClearmsgMsg{
					TargetMsgID: ev.TargetMsgID,
					Notice:      ev.Notice,
				}
			// Unknown event types (e.g. future additions): loop and read the
			// next event rather than returning nil and breaking the chain.
			}
		}
	}
}
