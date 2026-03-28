package irc

import "time"

// Badge is a Twitch chat badge identifier.
type Badge string

const (
	BadgeBroadcaster Badge = "broadcaster"
	BadgeModerator   Badge = "moderator"
	BadgeSubscriber  Badge = "subscriber"
	BadgeVIP         Badge = "vip"
	BadgeStaff       Badge = "staff"
	BadgeTurbo       Badge = "turbo"
)

// EmoteRange holds the byte-position span of one emote occurrence in a message.
type EmoteRange struct {
	ID         string
	Start, End int // inclusive byte indices into ChatMessage.Text
}

// SystemKind categorizes system messages for contextual coloring in the TUI.
type SystemKind int

const (
	SystemKindGeneric  SystemKind = iota // server NOTICEs, loading messages, etc.
	SystemKindSub                        // new subscription
	SystemKindGift                       // individual gifted sub (may be part of a bomb)
	SystemKindGiftBomb                   // mystery-gift summary ("X is gifting N subs!")
	SystemKindRaid                       // incoming raid
	SystemKindRitual                     // ritual / new-chatter event
	SystemKindTimeout                    // user timed out by a moderator
	SystemKindBan                        // user permanently banned
	SystemKindClear                      // full chat cleared by a moderator
	SystemKindDelete                     // specific message deleted by a moderator
)

// ChatMessage is a parsed Twitch PRIVMSG or a synthetic system notice.
type ChatMessage struct {
	Username    string
	DisplayName string
	Color       string       // "#RRGGBB" or "" when the user has no colour set
	Badges      []Badge
	ReplyTo     string       // DisplayName of the parent message; "" when not a native reply
	Text        string
	Emotes      []EmoteRange // sorted ascending by Start
	Timestamp   time.Time
	IsAction    bool // true for /me messages (CTCP ACTION)
	IsSystem    bool // true for synthetic server NOTICEs rendered as system lines
	RepeatCount int  // 0 = single occurrence; N = N additional identical messages collapsed into this one
	MsgID      string     // Twitch unique message ID (from the "id" tag on PRIVMSG)
	Greyed     bool       // cosmetically dimmed by a mod action (CLEARCHAT/CLEARMSG)
	ModAction  string     // post-fix label: "banned", "timeout Xm/Xs", or "deleted"
	SystemKind SystemKind // categorizes system messages for contextual coloring
	Bits       int        // total bits cheered in this message (0 = not a cheer)
}

// RoomState captures the current room restrictions set by the broadcaster.
type RoomState struct {
	EmoteOnly     bool
	SlowMode      int // 0 = off; N = minimum seconds between messages
	SubsOnly      bool
	FollowersOnly int // -1 = off; 0 = any follower; N = must have followed N+ minutes
}

// ---------------------------------------------------------------------------
// Event union type — the single channel used by Client.ReadLoop
// ---------------------------------------------------------------------------

// Event is the interface implemented by all values pushed onto the events channel.
// ReadLoop pushes ChatMessageEvent, RoomStateEvent, and NoticeEvent while the
// connection is live.  Connection and disconnection are signalled at the Cmd
// layer instead: Connect() returns ChatConnectedMsg on success; channel closure
// triggers ChatDisconnectedMsg inside waitForIRCEvent.
type Event interface{ ircEvent() }

// ChatMessageEvent wraps a parsed PRIVMSG.
type ChatMessageEvent struct{ Msg ChatMessage }

// RoomStateEvent wraps a parsed ROOMSTATE.
type RoomStateEvent struct{ State RoomState }

// NoticeEvent carries a server NOTICE — e.g. "This room is in followers-only mode."
// MsgID is the Twitch msg-id tag (e.g. "msg_followersonly", "msg_subsonly"); Text
// is the human-readable message body.
type NoticeEvent struct {
	MsgID string
	Text  string
}

func (ChatMessageEvent) ircEvent() {}
func (RoomStateEvent) ircEvent()   {}
func (NoticeEvent) ircEvent()      {}

// UsernoticeEvent carries a parsed USERNOTICE (subscription, raid, ritual…).
type UsernoticeEvent struct{ Msg ChatMessage }

// ClearchatEvent signals CLEARCHAT: a ban, timeout, or full room clear.
// TargetLogin is empty when the entire room was cleared.
// Duration is in seconds; 0 means a permanent ban (TargetLogin is set).
// Notice is the pre-built system message to display in the chat viewport.
type ClearchatEvent struct {
	TargetLogin string
	Duration    int
	Notice      ChatMessage
}

// ClearmsgEvent signals that a specific message was deleted by a moderator.
// TargetMsgID matches ChatMessage.MsgID on the original PRIVMSG.
type ClearmsgEvent struct {
	TargetMsgID string
	Notice      ChatMessage
}

func (UsernoticeEvent) ircEvent() {}
func (ClearchatEvent) ircEvent()  {}
func (ClearmsgEvent) ircEvent()   {}
