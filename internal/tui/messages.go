package tui

import (
	"os"
	"time"

	"twchfetch/internal/api"
	"twchfetch/internal/config"
	"twchfetch/internal/irc"
)

// RefreshDoneMsg is sent when all batch live-status fetches have completed.
type RefreshDoneMsg struct {
	Results []api.StreamerStatus
}

// BatchProgressMsg reports incremental batch completion during a refresh.
type BatchProgressMsg struct {
	Completed int
	Total     int
}

// DetailsFetchedMsg carries the result of fetching a single streamer's details.
type DetailsFetchedMsg struct {
	Username  string
	Details   *api.StreamDetails // nil if offline
	LastSeen  *time.Time
	Followers int // channel follower count (0 if unavailable)
}

// VODsFetchedMsg carries the initial VOD list and chapters for a streamer.
type VODsFetchedMsg struct {
	Username string
	VODs     []api.VOD
	Chapters map[string][]api.Chapter
	HasMore  bool // whether more VODs may exist (len(VODs) == requested count)
}

// VODsMoreFetchedMsg carries the next batch of VODs for a streamer.
// VODs contains only the newly fetched entries (not duplicates of what the
// model already holds). HasMore indicates whether another Load More is possible.
type VODsMoreFetchedMsg struct {
	Username string
	VODs     []api.VOD
	Chapters map[string][]api.Chapter
	HasMore  bool
}

// MPVLaunchedMsg signals the result of attempting to launch the player.
type MPVLaunchedMsg struct {
	Err error
}

// ClipboardMsg signals the result of a clipboard copy operation.
type ClipboardMsg struct {
	Text string
	Err  error
}

// ErrMsg carries a generic error for display in the status bar.
type ErrMsg struct {
	Err error
}

// SaveConfigDoneMsg signals that config has been saved to disk.
// TokenSource reflects where the OAuth token was actually stored.
type SaveConfigDoneMsg struct {
	Err         error
	TokenSource config.TokenSource
}

// FollowListFetchedMsg is sent when the OAuth follow-list fetch completes.
// Logins contains the lower-cased channel names to monitor.
type FollowListFetchedMsg struct {
	Logins []string
	Err    error
}

// CardAnimTickMsg advances the card selection animation by one frame.
type CardAnimTickMsg struct{}

// ProgressFrameMsg drives the spring-animated progress bar.
type ProgressFrameMsg struct{}

// LatestVODFetchedMsg carries the result of a "play latest VOD" fetch.
type LatestVODFetchedMsg struct {
	Username string
	VODID    string
	Err      error
}

// AutoRefreshTickMsg fires on the auto-refresh interval timer.
// Gen is matched against Model.autoRefreshGen so stale ticks from a superseded
// timer are silently discarded rather than triggering a redundant refresh.
type AutoRefreshTickMsg struct{ Gen int }

// SilentRefreshDoneMsg carries the result of a background auto-refresh.
// Unlike RefreshDoneMsg it does not trigger the loading screen or progress bar.
type SilentRefreshDoneMsg struct {
	Results []api.StreamerStatus
}

// ChatConnectedMsg is sent once the IRC registration handshake completes.
// Client is the live *irc.Client; it is non-nil so the model can store it for
// PRIVMSG sending once the user switches to normal (non-lurk) mode.
type ChatConnectedMsg struct {
	Client *irc.Client
}

// ChatMsg carries one incoming chat message from the IRC ReadLoop.
type ChatMsg struct{ Message irc.ChatMessage }

// ChatRoomStateMsg carries an updated ROOMSTATE for the joined channel.
type ChatRoomStateMsg struct{ State irc.RoomState }

// ChatDisconnectedMsg is sent when the IRC connection closes.
// Generation must match Model.chatGeneration for the message to be processed;
// stale disconnects from cancelled connections are silently discarded.
type ChatDisconnectedMsg struct {
	Err        error
	Generation int
}

// ChatLogStartedMsg is sent when the log file has been opened and the initial
// message buffer has been written.  File is nil and Err is non-nil on failure.
type ChatLogStartedMsg struct {
	Path string
	File *os.File
	Err  error
}

// ChatReconnectMsg fires after the auto-reconnect delay expires, triggering a
// fresh IRC connection attempt for the current channel.
type ChatReconnectMsg struct{}

// ChatReconnectResetMsg fires 2 minutes after a successful connection is
// established.  If the generation still matches the live connection, the
// reconnect counter is reset so stale retries from an earlier outage do not
// eat into the budget of a future one.
type ChatReconnectResetMsg struct{ Generation int }

// ChatRecTickMsg fires every second while chat logging is active to keep the
// pulsing REC indicator re-rendering independently of message arrival rate.
type ChatRecTickMsg struct{}

// ChatUsernoticeMsg carries a USERNOTICE system event (subscription, raid, ritual…).
type ChatUsernoticeMsg struct{ Message irc.ChatMessage }

// ChatClearchatMsg signals a CLEARCHAT event (ban, timeout, or full room clear).
// TargetLogin is empty when the entire room was cleared.
// Duration is the timeout length in seconds; 0 means a permanent ban.
// Notice is the pre-built system message to display in the viewport.
type ChatClearchatMsg struct {
	TargetLogin string
	Duration    int
	Notice      irc.ChatMessage
}

// ChatClearmsgMsg signals that a specific message was deleted by a moderator.
// TargetMsgID matches ChatMessage.MsgID on the original PRIVMSG.
type ChatClearmsgMsg struct {
	TargetMsgID string
	Notice      irc.ChatMessage
}

// ChatSendDoneMsg is returned by sendChatMessageCmd once the PRIVMSG write
// completes.  Err is non-nil if the write failed (e.g. connection closed).
type ChatSendDoneMsg struct {
	Text string
	Err  error
}

// EmotesFetchedMsg carries the third-party (BTTV + 7TV) emote set for a
// channel.  Channel is the lower-cased login name of the streamer; the handler
// discards the message if it no longer matches the active channel.
type EmotesFetchedMsg struct {
	Channel string
	Emotes  map[string]struct{}
}
