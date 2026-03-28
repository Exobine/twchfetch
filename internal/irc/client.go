package irc

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coder/websocket"
)

// ClientOptions configures optional behaviour for a Client.
type ClientOptions struct {
	// StripDingbats removes characters in the Unicode Dingbats block
	// (U+2700–U+27BF) from incoming messages.  Dingbats are absent from most
	// terminal/programming fonts; the terminal renders a 2-column replacement
	// box while width libraries measure 1 column, causing layout breakage.
	// Corresponds to the "Strip Dingbats" setting in the Chat settings tab.
	StripDingbats bool

	// OAuthToken is an optional Twitch user-access token (without the "oauth:"
	// prefix).  When provided, Connect authenticates as the token owner instead
	// of using an anonymous justinfan nick.  Authenticated connections can read
	// follower-only and subscribers-only channels that the account has access to.
	// If the token is invalid or the validate call fails, Connect falls back to
	// anonymous mode transparently.
	OAuthToken string
}

// Client is a read-only Twitch IRC-over-WebSocket client.
// Connect using Connect(); then start ReadLoop() in a goroutine.
type Client struct {
	conn          *websocket.Conn
	ctx           context.Context
	out           chan<- Event
	stripDingbats bool
	// roomState accumulates ROOMSTATE updates for this channel.  Twitch sends
	// a full ROOMSTATE on JOIN but only the changed tag on subsequent updates
	// (partial ROOMSTATE).  By merging each update into this running state and
	// pushing the merged result onto out, consumers always receive the complete
	// current room state rather than a partial snapshot.
	roomState RoomState
	// giftBombs tracks active mystery-gift-sub floods so that the subsequent
	// individual subgift/anonsubgift events can be suppressed.  Key = gifter
	// login (or "ananonymousgifter" for anonymous bombs).
	giftBombs map[string]*giftBombState
}

// giftBombState tracks an in-progress mystery gift so that subsequent
// individual subgift events can be suppressed within the expiry window.
type giftBombState struct {
	expected int       // announced gift count from submysterygift (0 = unknown)
	received int       // individual events suppressed so far
	expiry   time.Time // after this instant, individual gifts surface normally
}

// Connect dials the Twitch IRC WebSocket endpoint, sends the IRC handshake,
// and waits for RPL_001 (registration confirmed) before returning.
//
// If opts.OAuthToken is set, Connect calls the Twitch /oauth2/validate endpoint
// to retrieve the login name associated with the token, then authenticates as
// that user.  Authenticated connections can read follower-only and sub-only
// channels that the account has access to.  On any validate failure the
// connection falls back to an anonymous justinfan nick silently.
//
// All subsequent events are pushed onto out by calling ReadLoop() in a goroutine.
func Connect(ctx context.Context, channel string, out chan<- Event, opts ClientOptions) (*Client, error) {
	conn, _, err := websocket.Dial(ctx, "wss://irc-ws.chat.twitch.tv:443", nil)
	if err != nil {
		return nil, fmt.Errorf("irc dial: %w", err)
	}

	c := &Client{
		conn:          conn,
		ctx:           ctx,
		out:           out,
		stripDingbats: opts.StripDingbats,
		roomState:     RoomState{FollowersOnly: -1}, // -1 = followers-only disabled
		giftBombs:     make(map[string]*giftBombState),
	}

	// Determine credentials: authenticated (OAuth) or anonymous (justinfan).
	pass := "blah"
	nick := fmt.Sprintf("justinfan%06d", rand.Intn(900000)+100000)
	if opts.OAuthToken != "" {
		if login, err := validateOAuthLogin(ctx, opts.OAuthToken); err == nil && login != "" {
			pass = "oauth:" + opts.OAuthToken
			nick = login
		}
		// On failure: silently continue with anonymous credentials above.
	}

	handshake := []string{
		"PASS " + pass,
		"NICK " + nick,
		"CAP REQ :twitch.tv/tags twitch.tv/commands",
		"JOIN #" + strings.ToLower(channel),
	}
	for _, line := range handshake {
		if err := c.writeLine(line); err != nil {
			conn.Close(websocket.StatusNormalClosure, "")
			return nil, fmt.Errorf("irc handshake: %w", err)
		}
	}

	// Wait for RPL_001 (001) to confirm registration.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		line, err := c.readLine()
		if err != nil {
			conn.Close(websocket.StatusNormalClosure, "")
			return nil, fmt.Errorf("irc connect: %w", err)
		}
		// Strip tags prefix if present.
		stripped := line
		if strings.HasPrefix(line, "@") {
			if idx := strings.Index(line, " "); idx >= 0 {
				stripped = strings.TrimSpace(line[idx+1:])
			}
		}
		if strings.Contains(stripped, " 001 ") {
			return c, nil
		}
		// Handle PING during handshake.
		if strings.HasPrefix(stripped, "PING") {
			_ = c.writeLine("PONG :tmi.twitch.tv")
		}
	}
	conn.Close(websocket.StatusNormalClosure, "")
	return nil, fmt.Errorf("irc connect: timed out waiting for RPL_001")
}

// ReadLoop reads from the WebSocket until the context is cancelled or the
// connection closes.  It must be called in a goroutine.  When it exits it
// closes the out channel so the caller's waitForIRCEvent cmd unblocks.
func (c *Client) ReadLoop() {
	defer close(c.out)
	for {
		_, b, err := c.conn.Read(c.ctx)
		if err != nil {
			return
		}
		// A single WebSocket frame may contain multiple IRC lines separated by
		// \r\n.  Split and dispatch each one individually so that batched frames
		// don't bleed raw IRC data into the text of the preceding message.
		for _, line := range strings.Split(string(b), "\r\n") {
			line = strings.TrimRight(line, "\r ")
			if line != "" {
				c.handleLine(line)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (c *Client) readLine() (string, error) {
	_, b, err := c.conn.Read(c.ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

func (c *Client) writeLine(line string) error {
	return c.conn.Write(c.ctx, websocket.MessageText, []byte(line+"\r\n"))
}

// Send writes a PRIVMSG to the given channel.  channel must not include the
// leading '#'.  The WebSocket write is concurrency-safe; ReadLoop's concurrent
// reads do not interfere.  Returns an error if the connection is closed.
func (c *Client) Send(channel, text string) error {
	return c.writeLine(fmt.Sprintf("PRIVMSG #%s :%s", channel, text))
}

func (c *Client) handleLine(line string) {
	// Parse optional tags prefix.
	tags := map[string]string{}
	rest := line
	if strings.HasPrefix(line, "@") {
		tags, rest = parseTags(line)
	}

	// Strip the source prefix (:nick!user@host or :tmi.twitch.tv) and capture
	// the nick.  Twitch includes the login in the prefix for all message types
	// but only emits it as a "login" tag on USERNOTICE — for PRIVMSG the prefix
	// is the sole source of the login name.
	if strings.HasPrefix(rest, ":") {
		if idx := strings.Index(rest, " "); idx >= 0 {
			prefix := rest[1:idx] // e.g. "nick!nick@nick.tmi.twitch.tv"
			if bangIdx := strings.Index(prefix, "!"); bangIdx >= 0 {
				nick := prefix[:bangIdx]
				if nick != "" {
					if _, ok := tags["login"]; !ok {
						tags["login"] = nick
					}
				}
			}
			rest = strings.TrimSpace(rest[idx+1:])
		}
	}

	parts := strings.SplitN(rest, " ", 3)
	if len(parts) < 2 {
		return
	}
	command := parts[0]

	switch command {
	case "PING":
		_ = c.writeLine("PONG :tmi.twitch.tv")

	case "PRIVMSG":
		if len(parts) < 3 {
			return
		}
		text := parts[2]
		if strings.HasPrefix(text, ":") {
			text = text[1:]
		}
		msg := parseChatMessage(tags, text, c.stripDingbats)
		select {
		case c.out <- ChatMessageEvent{Msg: msg}:
		case <-c.ctx.Done():
		}

	case "ROOMSTATE":
		// Merge the incoming tags into the running state so that partial
		// ROOMSTATEs (which only contain the changed tag) do not clobber
		// fields that were set by the initial full ROOMSTATE on JOIN.
		c.roomState = mergeRoomState(c.roomState, tags)
		select {
		case c.out <- RoomStateEvent{State: c.roomState}:
		case <-c.ctx.Done():
		}

	case "NOTICE":
		// Twitch sends NOTICEs for certain server-level events.
		// Note: follower-only / sub-only NOTICEs are only delivered when the
		// client attempts to SEND a message — a read-only anonymous client
		// will never receive those.  We forward all NOTICEs so that any
		// authenticated-session notices (e.g. "Login unsuccessful") are visible.
		if len(parts) < 3 {
			return
		}
		text := strings.TrimPrefix(parts[2], ":")
		msgID := tags["msg-id"]
		select {
		case c.out <- NoticeEvent{MsgID: msgID, Text: text}:
		case <-c.ctx.Done():
		}

	case "RECONNECT":
		// Twitch requests that the client reconnect to a new server (e.g. for
		// load balancing or rolling deploys).  Close our end of the connection
		// so ReadLoop exits; the model's disconnect handler will reconnect.
		c.conn.Close(websocket.StatusNormalClosure, "reconnect requested")

	case "USERNOTICE":
		c.handleUsernotice(tags, parts)

	case "CLEARCHAT":
		c.handleClearchat(tags, parts)

	case "CLEARMSG":
		c.handleClearmsg(tags, parts)
	}
}

// validateOAuthLogin calls the Twitch token validation endpoint and returns the
// Twitch login name associated with the token.  Used to build an authenticated
// IRC handshake.  Returns an error if the token is invalid or the request fails.
func validateOAuthLogin(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://id.twitch.tv/oauth2/validate", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "OAuth "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("validate: HTTP %d", resp.StatusCode)
	}

	var body struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.Login, nil
}

// parseTags splits "@k=v;k2=v2 rest" into a tag map and the remainder.
func parseTags(line string) (map[string]string, string) {
	// line starts with '@'
	line = line[1:] // strip '@'
	rest := ""
	if idx := strings.Index(line, " "); idx >= 0 {
		rest = strings.TrimSpace(line[idx+1:])
		line = line[:idx]
	}
	tags := make(map[string]string)
	for _, kv := range strings.Split(line, ";") {
		if idx := strings.Index(kv, "="); idx >= 0 {
			tags[kv[:idx]] = kv[idx+1:]
		}
	}
	return tags, rest
}

// parseChatMessage builds a ChatMessage from IRCv3 tags and the message body.
func parseChatMessage(tags map[string]string, text string, stripDingbats bool) ChatMessage {
	// Detect /me ACTION messages: CTCP format is \x01ACTION <text>\x01.
	// Strip the wrapper before any further processing so that emote byte
	// offsets (which Twitch measures from the stripped text) are correct.
	// We also tolerate the rare malformed variant that omits the trailing \x01.
	isAction := false
	if strings.HasPrefix(text, "\x01ACTION") {
		isAction = true
		// Strip leading \x01ACTION and optional single trailing \x01.
		inner := text[7:] // len("\x01ACTION") == 7
		if strings.HasSuffix(inner, "\x01") {
			inner = inner[:len(inner)-1]
		}
		// The CTCP spec separates the command from its argument with a space.
		text = strings.TrimPrefix(inner, " ")
	}

	// Parse emote ranges and convert to byte offsets BEFORE sanitising.
	//
	// Twitch measures emote positions in Unicode codepoint (character) offsets,
	// NOT UTF-8 byte offsets.  For ASCII-only messages the two are the same,
	// but any multi-byte character in the prefix (e.g. ♡ °˖˚ ⋆ ═) shifts every
	// subsequent byte position further than the codepoint position, so naively
	// treating codepoint N as byte N lands on the wrong text.
	//
	// codepointOffsetsToByteOffsets walks the string once and remaps each
	// Start/End from "codepoint index" to "UTF-8 byte index" so all subsequent
	// operations (sanitisation, rendering) can do plain byte slicing.
	var emotes []EmoteRange
	if e := tags["emotes"]; e != "" {
		emotes = codepointOffsetsToByteOffsets(text, parseEmotes(e))
	}

	// Sanitize and shift emote byte offsets atomically.
	// Stripping multi-byte characters (e.g. Dingbats, Variation Selectors)
	// compresses the string; without adjustment the emote byte ranges would
	// point at wrong positions — or mid-character — in the cleaned text.
	text, emotes = sanitizeWithEmotes(text, emotes, stripDingbats)

	msg := ChatMessage{
		Username:    tags["login"],
		DisplayName: tags["display-name"],
		Color:       tags["color"],
		Text:        text,
		Emotes:      emotes,
		Timestamp:   time.Now(),
		IsAction:    isAction,
		MsgID:       tags["id"],
	}
	if msg.DisplayName == "" {
		msg.DisplayName = msg.Username
	}
	// Bits cheer — the "bits" tag carries the total bits in this message.
	if v := tags["bits"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			msg.Bits = n
		}
	}
	// Native reply: Twitch sends reply-parent-display-name when the user clicks Reply.
	if v := tags["reply-parent-display-name"]; v != "" {
		msg.ReplyTo = v
	}
	// Badges: "broadcaster/1,moderator/1,turbo/1,..."
	if b := tags["badges"]; b != "" {
		for _, part := range strings.Split(b, ",") {
			name := strings.SplitN(part, "/", 2)[0]
			switch Badge(name) {
			case BadgeBroadcaster, BadgeModerator, BadgeSubscriber, BadgeVIP, BadgeStaff, BadgeTurbo:
				msg.Badges = append(msg.Badges, Badge(name))
			}
		}
	}
	return msg
}

// parseEmotes converts a Twitch emote tag into sorted EmoteRange slices.
func parseEmotes(tag string) []EmoteRange {
	var ranges []EmoteRange
	for _, emote := range strings.Split(tag, "/") {
		parts := strings.SplitN(emote, ":", 2)
		if len(parts) != 2 {
			continue
		}
		id := parts[0]
		for _, span := range strings.Split(parts[1], ",") {
			se := strings.SplitN(span, "-", 2)
			if len(se) != 2 {
				continue
			}
			start, err1 := strconv.Atoi(se[0])
			end, err2 := strconv.Atoi(se[1])
			if err1 != nil || err2 != nil {
				continue
			}
			ranges = append(ranges, EmoteRange{ID: id, Start: start, End: end})
		}
	}
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].Start < ranges[j].Start
	})
	return ranges
}

// codepointOffsetsToByteOffsets converts emote ranges whose Start/End values
// are Unicode codepoint (character) indices — as sent by the Twitch IRC API —
// into UTF-8 byte indices suitable for plain byte-slice operations.
//
// Twitch documents this explicitly: emote positions are "measured in
// characters, not bytes."  For pure-ASCII messages the two are identical, but
// any multi-byte character appearing before an emote in the message shifts
// every subsequent byte position past the codepoint position.
//
// The function makes one O(n) pass over the string to build a codepoint→byte
// table, then remaps each range.  Ranges that fall outside the string or
// produce invalid byte spans are silently dropped.
func codepointOffsetsToByteOffsets(text string, emotes []EmoteRange) []EmoteRange {
	if len(emotes) == 0 {
		return emotes
	}

	// Count runes and build codepoint-index → byte-offset table in one pass.
	// cpByte[i] is the byte offset where the i-th codepoint begins.
	// The sentinel cpByte[nCP] = len(text) is the byte offset past the last rune.
	nCP := utf8.RuneCountInString(text)
	cpByte := make([]int, nCP+1)
	cp := 0
	for byteOff := range text { // range over string yields (byteOffset, rune)
		cpByte[cp] = byteOff
		cp++
	}
	cpByte[nCP] = len(text)

	result := make([]EmoteRange, 0, len(emotes))
	for _, e := range emotes {
		if e.Start < 0 || e.End >= nCP || e.Start > e.End {
			continue // malformed codepoint range — drop
		}
		byteStart := cpByte[e.Start]
		// Last byte of codepoint e.End = first byte of codepoint e.End+1, minus 1.
		byteEnd := cpByte[e.End+1] - 1
		if byteStart > byteEnd || byteEnd >= len(text) {
			continue
		}
		result = append(result, EmoteRange{ID: e.ID, Start: byteStart, End: byteEnd})
	}
	return result
}

// isStrippedRune reports whether r must be removed from chat text because it
// would either corrupt terminal cursor tracking or produce a rendered width
// that disagrees with what ansi.StringWidth measures (breaking layout).
//
// Stripped categories:
//   - C0 control codes (U+0000–U+001F): CR, TAB, BELL, backspace, etc.
//   - DEL (U+007F)
//   - C1 control codes (U+0080–U+009F): legacy 8-bit control characters
//   - Unicode Format category (Cf): zero-width spaces, bidirectional marks, etc.
//   - Variation Selectors (U+FE00–U+FE0F, U+E0100–U+E01EF): VS-16 silently
//     widens the preceding glyph to 2 columns while the width library says 1.
//   - Dingbats (U+2700–U+27BF): decorative symbols absent from most
//     programming fonts; terminal renders a 2-column replacement box while
//     every width library says 1 column (Unicode EAW = Neutral).
func isStrippedRune(r rune, stripDingbats bool) bool {
	return r < 0x20 ||
		r == 0x7F ||
		(r >= 0x80 && r <= 0x9F) ||
		unicode.Is(unicode.Cf, r) ||
		(r >= 0xFE00 && r <= 0xFE0F) ||
		(r >= 0xE0100 && r <= 0xE01EF) ||
		(stripDingbats && r >= 0x2700 && r <= 0x27BF)
}

// sanitizeWithEmotes sanitizes s (removing all isStrippedRune characters) and
// simultaneously adjusts the byte Start/End offsets of each EmoteRange so they
// remain valid indices into the returned cleaned string.
//
// This must be called with the ORIGINAL text and emotes parsed from the IRC
// tags, BEFORE any other modification.  Stripping multi-byte characters shifts
// all subsequent byte positions; without adjustment the emote renderer would
// index into the wrong bytes and inject ANSI codes mid-character, producing
// both wrong styling (tint on the wrong word) and terminal corruption.
func sanitizeWithEmotes(s string, emotes []EmoteRange, stripDingbats bool) (string, []EmoteRange) {
	// Fast path: nothing to strip.
	needsClean := false
	for _, r := range s {
		if isStrippedRune(r, stripDingbats) {
			needsClean = true
			break
		}
	}
	if !needsClean {
		return s, emotes
	}

	// Build origToClean: origToClean[i] is the byte offset in the cleaned
	// string that corresponds to byte i in the original string.
	// For stripped runes, origToClean[i] equals the offset of the next kept byte.
	origToClean := make([]int, len(s)+1)
	var b strings.Builder
	b.Grow(len(s))
	cleanPos := 0
	for origPos := 0; origPos < len(s); {
		r, size := utf8.DecodeRuneInString(s[origPos:])
		if isStrippedRune(r, stripDingbats) {
			// Stripped bytes all map to the position of the next kept byte.
			for j := 0; j < size; j++ {
				origToClean[origPos+j] = cleanPos
			}
		} else {
			for j := 0; j < size; j++ {
				origToClean[origPos+j] = cleanPos + j
			}
			b.WriteRune(r)
			cleanPos += size
		}
		origPos += size
	}
	origToClean[len(s)] = cleanPos

	cleaned := b.String()

	// Adjust emote byte ranges.
	adjusted := make([]EmoteRange, 0, len(emotes))
	for _, e := range emotes {
		if e.Start < 0 || e.End >= len(s) || e.Start > e.End {
			continue // malformed: drop
		}
		newStart := origToClean[e.Start]
		newEnd := origToClean[e.End]
		// Validate: range must be non-empty and within the cleaned string,
		// and both endpoints must sit on valid UTF-8 rune boundaries.
		if newStart > newEnd ||
			newEnd >= len(cleaned) ||
			!utf8.RuneStart(cleaned[newStart]) ||
			(newEnd+1 < len(cleaned) && !utf8.RuneStart(cleaned[newEnd+1])) {
			continue
		}
		adjusted = append(adjusted, EmoteRange{ID: e.ID, Start: newStart, End: newEnd})
	}

	return cleaned, adjusted
}

// mergeRoomState applies only the tags present in the ROOMSTATE message to the
// existing state and returns the merged result.
//
// Twitch sends a full ROOMSTATE (all tags) on initial JOIN, but only the
// changed tag on subsequent updates.  Starting from the existing state and
// updating only the tags present ensures that partial updates do not silently
// reset fields whose tags were absent from the message.
func mergeRoomState(existing RoomState, tags map[string]string) RoomState {
	rs := existing
	if v, ok := tags["emote-only"]; ok {
		rs.EmoteOnly = v == "1"
	}
	if v, ok := tags["subscribers-only"]; ok {
		rs.SubsOnly = v == "1"
	}
	if v, ok := tags["slow"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			rs.SlowMode = n
		}
	}
	if v, ok := tags["followers-only"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			rs.FollowersOnly = n
		}
	}
	return rs
}

// unescapeTagValue decodes Twitch IRCv3 tag-value escaping.
// Twitch encodes spaces as \s, backslashes as \\, semicolons as \:, and so on.
func unescapeTagValue(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 's':
				b.WriteByte(' ')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case ':':
				b.WriteByte(';')
			default:
				b.WriteByte(s[i+1])
			}
			i += 2
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// handleUsernotice processes a USERNOTICE line and pushes a UsernoticeEvent.
// Gift bomb suppression works in both directions:
//   - Forward: when submysterygift arrives first, subsequent subgift events
//     within the 30 s window are dropped here (giftBombs map).
//   - Backward: when individual subgift events arrive before the summary
//     (the common Twitch ordering), the TUI retroactively greys them when it
//     receives the SystemKindGiftBomb summary message.
func (c *Client) handleUsernotice(tags map[string]string, parts []string) {
	msgID := tags["msg-id"]

	// Lazy-expire stale gift bomb entries on every USERNOTICE to bound growth.
	now := time.Now()
	for k, v := range c.giftBombs {
		if now.After(v.expiry) {
			delete(c.giftBombs, k)
		}
	}

	// system-msg is the Twitch-generated human-readable line (tag-value-escaped).
	sysMsg := unescapeTagValue(tags["system-msg"])

	var text string
	var kind SystemKind
	var senderLogin string // gifter login for gift events; used for bomb matching

	switch msgID {
	case "sub":
		if sysMsg != "" {
			text = sysMsg
		} else {
			login := tags["display-name"]
			if login == "" {
				login = tags["login"]
			}
			text = login + " just subscribed!"
		}
		kind = SystemKindSub

	case "resub":
		if sysMsg != "" {
			text = sysMsg
		} else {
			login := tags["display-name"]
			if login == "" {
				login = tags["login"]
			}
			text = login + " resubscribed!"
		}
		// Append the user's optional personal resub message when present.
		if len(parts) >= 3 {
			body := strings.TrimPrefix(parts[2], ":")
			if body != "" {
				text += " — " + body
			}
		}
		kind = SystemKindSub

	case "subgift":
		gl := tags["login"]
		if gl == "" {
			gl = "ananonymousgifter"
		}
		// Forward suppression: drop if a bomb window is already registered.
		if state, ok := c.giftBombs[gl]; ok && now.Before(state.expiry) {
			state.received++
			return
		}
		if sysMsg != "" {
			text = sysMsg
		} else {
			gifterDisplay := tags["display-name"]
			if gifterDisplay == "" {
				gifterDisplay = gl
			}
			recipient := tags["msg-param-recipient-display-name"]
			text = gifterDisplay + " gifted a sub to " + recipient + "!"
		}
		senderLogin = gl
		kind = SystemKindGift

	case "anonsubgift":
		// Forward suppression: drop if an anonymous bomb window is active.
		if state, ok := c.giftBombs["ananonymousgifter"]; ok && now.Before(state.expiry) {
			state.received++
			return
		}
		if sysMsg != "" {
			text = sysMsg
		} else {
			recipient := tags["msg-param-recipient-display-name"]
			text = "An anonymous user gifted a sub to " + recipient + "!"
		}
		senderLogin = "ananonymousgifter"
		kind = SystemKindGift

	case "submysterygift", "anonsubmysterygift":
		gl := tags["login"]
		if gl == "" {
			gl = "ananonymousgifter"
		}
		countStr := tags["msg-param-mass-gift-count"]
		count, _ := strconv.Atoi(countStr)
		// Register forward-suppression window for any subgifts that arrive after.
		c.giftBombs[gl] = &giftBombState{
			expected: count,
			received: 0,
			expiry:   now.Add(30 * time.Second),
		}
		if sysMsg != "" {
			text = sysMsg
		} else {
			gifterDisplay := tags["display-name"]
			if gifterDisplay == "" {
				gifterDisplay = gl
			}
			text = fmt.Sprintf("%s is gifting %d subs to the community!", gifterDisplay, count)
		}
		senderLogin = gl
		kind = SystemKindGiftBomb

	case "giftpaidupgrade", "anongiftpaidupgrade":
		if sysMsg != "" {
			text = sysMsg
		} else {
			login := tags["display-name"]
			if login == "" {
				login = tags["login"]
			}
			text = login + " is continuing their gifted subscription!"
		}
		kind = SystemKindGift

	case "raid":
		if sysMsg != "" {
			text = sysMsg
		} else {
			raiderDisplay := tags["msg-param-displayname"]
			if raiderDisplay == "" {
				raiderDisplay = tags["msg-param-login"]
			}
			viewerStr := tags["msg-param-viewerCount"]
			viewers, _ := strconv.Atoi(viewerStr)
			text = fmt.Sprintf("%s is raiding with %d viewers!", raiderDisplay, viewers)
		}
		kind = SystemKindRaid

	case "unraid":
		login := tags["display-name"]
		if login == "" {
			login = tags["login"]
		}
		text = login + " cancelled the raid."
		kind = SystemKindRaid

	case "ritual":
		ritualName := tags["msg-param-ritual-name"]
		login := tags["display-name"]
		if login == "" {
			login = tags["login"]
		}
		if ritualName == "new_chatter" {
			text = login + " is new to the chat — say hello!"
		} else if sysMsg != "" {
			text = sysMsg
		} else {
			text = login + " performed a ritual: " + ritualName
		}
		kind = SystemKindRitual

	case "bitsbadgetier":
		if sysMsg != "" {
			text = sysMsg
		} else {
			login := tags["display-name"]
			if login == "" {
				login = tags["login"]
			}
			threshold := tags["msg-param-threshold"]
			text = login + " just unlocked a new bits badge (" + threshold + " bits)!"
		}
		kind = SystemKindSub

	default:
		// Unknown or unimplemented msg-id — silently ignore.
		return
	}

	if text == "" {
		return
	}

	msg := ChatMessage{
		Text:       text,
		Username:   senderLogin,
		Timestamp:  time.Now(),
		IsSystem:   true,
		SystemKind: kind,
	}
	select {
	case c.out <- UsernoticeEvent{Msg: msg}:
	case <-c.ctx.Done():
	}
}

// handleClearchat processes a CLEARCHAT line (ban, timeout, or full room clear)
// and pushes a ClearchatEvent.  It never wipes the message buffer — the model
// applies cosmetic greying only.
func (c *Client) handleClearchat(tags map[string]string, parts []string) {
	targetLogin := ""
	if len(parts) >= 3 {
		targetLogin = strings.TrimPrefix(parts[2], ":")
	}

	var noticeText string
	var kind SystemKind
	duration := 0

	if targetLogin == "" {
		noticeText = "Chat was cleared by a moderator."
		kind = SystemKindClear
	} else {
		if durStr, ok := tags["ban-duration"]; ok {
			if dur, err := strconv.Atoi(durStr); err == nil && dur > 0 {
				duration = dur
				mins := dur / 60
				secs := dur % 60
				var durText string
				if mins > 0 {
					durText = fmt.Sprintf("%dm", mins)
				} else {
					durText = fmt.Sprintf("%ds", secs)
				}
				noticeText = targetLogin + " was timed out for " + durText + "."
				kind = SystemKindTimeout
			} else {
				noticeText = targetLogin + " was banned."
				kind = SystemKindBan
			}
		} else {
			noticeText = targetLogin + " was banned."
			kind = SystemKindBan
		}
	}

	notice := ChatMessage{
		Text:       noticeText,
		Timestamp:  time.Now(),
		IsSystem:   true,
		SystemKind: kind,
	}
	select {
	case c.out <- ClearchatEvent{TargetLogin: targetLogin, Duration: duration, Notice: notice}:
	case <-c.ctx.Done():
	}
}

// handleClearmsg processes a CLEARMSG line (single message deletion) and
// pushes a ClearmsgEvent.  The model matches TargetMsgID against
// ChatMessage.MsgID and applies cosmetic greying to the matching entry.
func (c *Client) handleClearmsg(tags map[string]string, _ []string) {
	targetMsgID := tags["target-msg-id"]
	if targetMsgID == "" {
		return
	}
	login := tags["login"]
	noticeText := login + "'s message was deleted."
	if login == "" {
		noticeText = "A message was deleted."
	}
	notice := ChatMessage{
		Text:       noticeText,
		Timestamp:  time.Now(),
		IsSystem:   true,
		SystemKind: SystemKindDelete,
	}
	select {
	case c.out <- ClearmsgEvent{TargetMsgID: targetMsgID, Notice: notice}:
	case <-c.ctx.Done():
	}
}
