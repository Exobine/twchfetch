# twchfetch — Documentation

Full reference for all configuration settings, authentication, chat behaviour,
and operational notes.

---

## Table of contents

1. [Authentication and OAuth token](#1-authentication-and-oauth-token)
2. [Streamer list and followed channels](#2-streamer-list-and-followed-channels)
3. [Token storage methods](#3-token-storage-methods)
4. [Configuration reference](#4-configuration-reference)
5. [Chat system](#5-chat-system)
6. [Stream and VOD playback](#6-stream-and-vod-playback)
7. [Display modes](#7-display-modes)
8. [Refresh and caching](#8-refresh-and-caching)
9. [Network and VPN notes](#9-network-and-vpn-notes)

---

## 1. Authentication and OAuth token

An OAuth token is optional but substantially changes what the application can do.

### Without a token (unauthenticated)

- The `streamers.list` array in `config.toml` is used as the channel list.
- Chat connects anonymously using a randomly generated `justinfanNNNNNN` nick.
- Anonymous connections can read follower-only and sub-only chat rooms normally.
  The restriction only applies to **sending** messages — anonymous users cannot
  chat in restricted rooms, but reading is unaffected. However twitch may disconnect
  you from offline chat rooms every few minutes preventing proper reads of chat.

### With a token (authenticated)

- The application calls the Twitch `/oauth2/validate` endpoint on startup to verify
  the token and retrieve the associated Twitch login name.
- Your followed channel list is fetched automatically via the Twitch Helix API
  (up to 500 channels). When this succeeds, `streamers.list` is **completely
  bypassed** — the followed list is used instead.
- Chat authenticates as your account. This is required if you want to **send**
  messages in follower-only or sub-only channels that your account has access to.
- If token validation fails at startup (network error, expired token), the
  application falls back to anonymous mode silently rather than refusing to start.

### Getting a token

A Twitch OAuth user-access token can be obtained via:

- The [Twitch Token Generator](https://twitchtokengenerator.com/) — third party;
  review its terms before use.
- The [Twitch CLI](https://dev.twitch.tv/docs/cli/) `twitch token` command.
- Any OAuth 2.0 implicit or authorisation-code flow against
  `https://id.twitch.tv/oauth2/authorize`.

Tokens may include an `oauth:` prefix (IRC/Streamlink style). twchfetch strips
this prefix automatically when saving.

### Required OAuth scopes

| Scope | Purpose |
|---|---|
| `user:read:follows` | Fetch followed channel list |
| `chat:edit` | Send messages in chat |

Chat reading is permitted by all user tokens with no additional scope. `chat:edit`
is only required if you intend to send messages; the app functions without it.

---

## 2. Streamer list and followed channels

The channel list the application monitors is determined by one of two sources,
resolved in priority order:

1. **Followed list** — when an OAuth token is set and the Helix followed-channels
   call succeeds. This replaces the manual list entirely for that session.
2. **`streamers.list`** — the array of Twitch login names in `config.toml`. Used
   when no token is set, or when the followed-list fetch fails (e.g. network error,
   insufficient scope).

There is no hybrid mode — it is one source or the other. If you want to monitor
channels you do not follow, use `streamers.list` without a token (or revoke
follow-list access).

> **Note:** the in-app Help tab states "Streamers field is ignored when an OAuth
> token is set." This means the `streamers.list` array in `config.toml` has no
> effect once a valid token is loaded and the follow list is fetched.

---

## 3. Token storage methods

twchfetch resolves the OAuth token from three possible sources, checked in order:

### Priority 1 — OS keyring (recommended)

Enter your token via the in-app **Settings** UI. The token is written directly to:

| Platform | Storage |
|---|---|
| Windows | Windows Credential Manager (`twchfetch/config/go`) |
| macOS | macOS Keychain |
| Linux | Secret Service (e.g. GNOME Keyring, KWallet) |

The `oauth_token` field in `config.toml` remains **empty**. The token never
touches the filesystem in plaintext. This is the preferred method because the OS
credential store is purpose-built for secrets — it is encrypted at rest and access
is tied to your OS user session.

When you clear the token via Settings, the keyring entry is deleted automatically.

### Priority 2 — environment variable

Set `TWITCH_OAUTH_TOKEN` in your shell before launching twchfetch:

```bash
# Linux / macOS (current session only)
export TWITCH_OAUTH_TOKEN=your_token_here
./twchfetch

# Windows PowerShell (current session only)
$env:TWITCH_OAUTH_TOKEN = "your_token_here"
.\twchfetch.exe
```

This overrides both the keyring and `config.toml`. It is useful in CI/CD
environments or when you want a per-session token without touching stored
credentials.

### Priority 3 — config.toml plaintext (last resort)

Set `oauth_token = "your_token_here"` directly in `config.toml`. This is the
fallback for systems where the OS keyring is unavailable (e.g. a headless server
with no secret service daemon running).

`config.toml` is written with `0600` permissions (owner read/write only) to limit
exposure. However, the token is still stored in plaintext on disk. If your system
supports a keyring (most desktop Linux/macOS/Windows installs do), use Priority 1
instead.

`config.toml` is gitignored — **never commit it**.

---

## 4. Configuration reference

All settings live under `config.toml`. Copy `config.example.toml` to
`config.toml` and edit before first run.

### Top-level settings

| Key | Default | Description |
|---|---|---|
| `client_id` | built-in | Twitch application Client ID used for Helix API calls. The default ID ships with the binary; you can substitute your own registered application ID if needed. |
| `oauth_token` | `""` | Plaintext token fallback. Prefer the OS keyring via Settings. |
| `player_path` | `""` | Absolute path to your media player binary. Empty = playback disabled. |
| `player_args` | `[]` | Extra command-line arguments passed to the player after the stream/VOD URL. |
| `refresh_max_workers` | `16` | Maximum parallel goroutines during a stream-status batch refresh. Higher values fetch faster but increase API call rate. |
| `refresh_batch_size` | `5` | Number of channels fetched per API call during batch refresh. The Twitch API accepts up to 100 per request; smaller batches spread load. |
| `request_timeout_sec` | `10` | HTTP request timeout in seconds for all Twitch API calls. |
| `auto_refresh_minutes` | `5` | How often (in minutes) the stream list refreshes automatically in the background. `0` disables auto-refresh entirely. A countdown timer appears in the tab bar when this is active. |
| `cache_override_minutes` | `15` | How long (in minutes) channel detail and VOD data is cached before being re-fetched on next visit. `0` disables the cache — data is fetched on every visit. |
| `debug_logging` | `false` | Write debug-level entries to the log file (`twchfetch.log`). When false, only Info/Warn/Error entries are written. |

### `[streamers]`

| Key | Default | Description |
|---|---|---|
| `list` | `[]` | Array of Twitch login names (lowercase) to monitor. Ignored when an OAuth token is set and the followed list is fetched successfully. |

### `[display]`

| Key | Default | Description |
|---|---|---|
| `display_mode` | `"cards"` | Starting display mode: `"cards"` or `"list"`. Toggle at runtime with Tab. |
| `card_width` | `22` | Width of each channel card in card mode (characters). |
| `card_columns` | `0` | Number of card columns. `0` = auto-fit as many as the terminal width allows. |
| `card_pad_h` | `2` | Horizontal padding (characters) between cards. |
| `card_pad_v` | `0` | Vertical padding (lines) between cards. |
| `card_live_color` | `"#102910"` | Background colour for live channel cards (hex `#RRGGBB`). |
| `card_select_color` | `"#2C2260"` | Background colour for the selected card (hex `#RRGGBB`). |
| `list_split_at` | `20` | Number of rows per column in list mode before adding a new column (see `list_table_count`). |
| `list_table_count` | `1` | Number of side-by-side tables in list mode. Useful on wide terminals with many channels. |
| `vod_max_display` | `10` | Maximum number of VODs shown in the VOD browser per channel. |
| `vod_categories_max_shown` | `5` | Maximum number of distinct categories shown in the VOD category filter. |

### `[chat]`

| Key | Default | Description |
|---|---|---|
| `max_messages` | `300` | Chat buffer size — the maximum number of messages held in memory. Clamped to the range [100, 1000]. Older messages are dropped when the buffer is full. |
| `max_reconnects` | `5` | Maximum automatic reconnect attempts after a chat disconnect before giving up. `0` = unlimited reconnects. After reaching the limit, a prompt is shown to manually retry with `c`. |
| `emote_colors` | `true` | Apply a grey tint to native Twitch emote text so it is visually distinct from regular words. |
| `third_party_emotes` | `false` | Fetch BTTV and 7TV emote lists for the channel and apply styling to matching words in chat. When false, only native Twitch emotes (sent in IRC tags) are styled. |
| `third_party_shading` | `false` | Apply the same grey tint used by `emote_colors` to BTTV/7TV emotes. Only applies when `third_party_emotes` is true. |
| `text_badges` | `false` | Replace icon glyphs (`●`, `⚔`, `■`, `◆`, `⚑`, `⚡`) with bracketed text labels (`[Broadcaster]`, `[Mod]`, `[Sub]`, `[VIP]`, `[Staff]`, `[Turbo]`). Useful for terminals or fonts that cannot render the glyph characters correctly. |
| `strip_dingbats` | `true` | Remove Unicode Dingbats block characters (U+2700–U+27BF) from incoming messages. These glyphs are absent from most programming/terminal fonts; the terminal renders a wide replacement box while width measurement sees a narrow character, causing layout misalignment. Leave enabled unless your font explicitly supports this range. |
| `show_reply` | `true` | Prefix reply messages with `↩ @username ·` to show they are replies to another user. |
| `trim_reply_mention` | `true` | When `show_reply` is on, strip the leading `@username` from the message body to avoid seeing the name twice. Only takes effect when `show_reply` is true. |
| `collapse_repeats` | `"single"` | Consecutive identical message collapsing. `"off"` = no collapsing. `"single"` = collapse only when the same user repeats their own last message. `"all"` = collapse whenever any user posts the same text as the immediately preceding message. Collapsed entries show a `[x3]` counter. |
| `alt_row_color` | `""` | Hex colour (`#RRGGBB`) for alternating row backgrounds in the chat view. Empty string = no alternating colour. |
| `localized_names` | `true` | Append the ASCII login name in parentheses after non-ASCII display names, e.g. `名前 (name123)`. Helps identify users whose display names use characters your font may not render. |

---

## 5. Chat system

### Connection

Chat connects to Twitch's IRC-over-WebSocket endpoint
(`wss://irc-ws.chat.twitch.tv:443`). A WebSocket tunnel is used instead of raw
TCP IRC so it works through environments that block port 6667.

On connection:

1. If a token is configured, twchfetch calls `https://id.twitch.tv/oauth2/validate`
   to get the login name associated with the token, then sends an authenticated
   IRC handshake using `PASS oauth:<token>` and `NICK <login>`.
2. If no token is set — or if validation fails — a random anonymous nick
   (`justinfanNNNNNN`) is used with `PASS blah`. This is an officially supported
   Twitch anonymous IRC method.
3. Twitch Capability tags (`twitch.tv/tags` and `twitch.tv/commands`) are
   requested so messages include metadata (badges, emote positions, colours, etc.).

### Authenticated vs anonymous access

| Feature | Anonymous | Authenticated |
|---|---|---|
| Read normal chat | Yes | Yes |
| Read follower-only chat | Yes | Yes |
| Read sub-only chat | Yes | Yes |
| Send in follower-only chat | No | Yes (if you follow) |
| Send in sub-only chat | No | Yes (if you subscribe) |
| Send messages | No | Yes |

### Auto-reconnect

When the IRC connection drops, twchfetch automatically reconnects after a 3-second
delay. Each attempt is counted and shown in the chat view
(`reconnecting… attempt N/M`).

The counter resets to zero after a connection has been stable for **30 seconds**,
which is long enough to distinguish rapid flapping (several drops in quick
succession) from genuinely separate disconnect events. If `max_reconnects = 0`
(unlimited), the counter still increments but the limit check is never triggered.

When the retry limit is reached, an in-chat system message is shown. Pressing `c`
opens a fresh chat connection with the counter reset to zero.

Twitch occasionally sends a `RECONNECT` command as part of load balancing or
rolling server deploys. twchfetch handles this by closing the connection
gracefully, which triggers the normal auto-reconnect path.

### Gift bomb suppression

When a chatter gifts multiple subscriptions at once ("gift bomb"), Twitch sends:

1. A `submysterygift` event announcing the total count.
2. Individual `subgift` events for each recipient.

Without suppression, a 20-gift bomb floods the chat view with 21 nearly identical
lines. twchfetch collapses this into a single summary line using a 30-second
bidirectional window:

- **Forward suppression** — if the summary arrives first, subsequent individual
  gift events from the same gifter within 30 seconds are silently dropped.
- **Backward suppression** — if individual gift events arrive before the summary
  (the more common Twitch ordering), they are retroactively marked as suppressed
  when the summary arrives. The TUI renders them as greyed/dimmed entries rather
  than removing them from the buffer.

Both anonymous and named gift bombs are handled. The window is keyed by gifter
login name.

### Mod actions

Mod actions do not remove messages from the buffer. They apply cosmetic changes:

- **[banned]** — user permanently banned; their messages are greyed.
- **[timeout Xm]** — user timed out; their messages are greyed.
- **[deleted]** — a single message deleted by a moderator; that entry is greyed.

This is intentional — retaining the message text (dimmed) lets you see what was
said rather than having context disappear silently.

### System events

Subscriptions, gift bombs, raids, and rituals appear as coloured system lines
distinct from regular chat messages.

### Scroll modes

Cycle through scroll modes with `h`:

| Mode | Behaviour |
|---|---|
| **Live** | Viewport always jumps to the latest message. New messages auto-scroll the view. |
| **Frozen** | Viewport holds its current position. New messages queue below silently; an unread count is shown in the header. |
| **Follow** | Viewport advances by the height of each new message, keeping the relative position. |

Scrolling up with `↑`/`j` automatically pauses auto-scrolling. Press `G` to jump
to the bottom and resume.

### Emote styling

Native Twitch emotes are identified by byte-range tags sent in the IRC message.
twchfetch correctly handles multi-byte Unicode characters in message prefixes by
converting Twitch's codepoint-based offsets to byte offsets before rendering.

Third-party emotes (BTTV, 7TV) are fetched per-channel when
`third_party_emotes = true`. They are matched by word against incoming message
text and styled differently from native emotes to distinguish their source.

---

## 6. Stream and VOD playback

twchfetch opens streams and VODs in an external media player. Set `player_path`
to the absolute path of the player binary:

```toml
# Windows
player_path = "C:\\mpv\\mpv.exe"

# Linux / macOS
player_path = "/usr/bin/mpv"
```

[mpv](https://mpv.io/) is recommended — it handles Twitch HLS streams natively
via `yt-dlp` or `streamlink` integration.

If `player_path` is empty the stream/VOD open keybinds (`Enter` for streams,
`Enter` in the VOD browser for VODs) do nothing. The rest of the app functions
without a player configured.

Extra arguments can be passed via `player_args`:

```toml
player_args = ["--volume=70", "--fs"]
```

These are appended after the URL when the player is launched.

---

## 7. Display modes

### Cards mode (default)

Channels are shown as visual cards, one per channel, laid out in columns. Live
channels use a distinct background colour. The selected card uses a separate
highlight colour.

`card_columns = 0` auto-fits as many columns as the terminal width allows given
`card_width`. Set an explicit number to override.

### List mode

Channels are shown as compact table rows. At `list_table_count = 2` (or more),
the list wraps into side-by-side columns after every `list_split_at` rows — useful
on wide terminals with many channels.

Toggle between modes at runtime with `Tab`. The active mode is shown in the tab
bar label ("Cards" / "List").

---

## 8. Refresh and caching

### Auto-refresh

The stream list (live/offline status, viewer counts) refreshes automatically in
the background at the interval set by `auto_refresh_minutes`. A countdown timer
appears in the tab bar. Setting `auto_refresh_minutes = 0` disables this entirely;
press `r` for a manual refresh at any time.

### Batch refresh

During a refresh, channels are fetched in parallel batches using a semaphore of
`refresh_max_workers` concurrent goroutines. Each worker introduces a random 0–150
ms jitter before its first API call to avoid sending all requests simultaneously.
`refresh_batch_size` controls how many channels are combined into a single API
request — the Twitch API supports up to 100 per call, but smaller batches keep
individual calls lighter.

### Cache

Channel details and VOD lists are cached in memory for `cache_override_minutes`
minutes. Opening a channel's details or VOD browser within the cache window serves
the stored result without a network call. Set `cache_override_minutes = 0` to
always fetch fresh data on every visit.

The cache is reset when settings are saved (in case the token or channel list
changed).

---

## 9. Network and VPN notes

### General

twchfetch makes three categories of outbound connections:

| Destination | Purpose |
|---|---|
| `api.twitch.tv` (HTTPS/443) | Helix REST API — stream status, VODs, followed channels |
| `id.twitch.tv` (HTTPS/443) | OAuth token validation |
| `irc-ws.chat.twitch.tv` (WSS/443) | Live chat via IRC-over-WebSocket |

All connections use port 443 (HTTPS/WSS), which is typically allowed through
firewalls and proxies.

### VPN usage

Running twchfetch through a VPN may cause problems with IRC chat:

- **Twitch rate-limits IRC connections by IP.** VPN exit nodes are shared among
  many users. If other VPN users have been connecting to Twitch IRC frequently,
  the shared IP may already be near or at Twitch's connection rate limit. New
  connections from that IP may be silently dropped or delayed.
- **Anonymous IRC** (`justinfanNNNNNN`) is particularly prone to this because
  Twitch applies stricter rate limits to unauthenticated connections. Authenticated
  connections (with a valid OAuth token) are less likely to be affected.
- If chat fails to connect or repeatedly disconnects only when on VPN, try
  switching VPN servers or temporarily disabling the VPN for the chat connection.

The Helix REST API (stream status, VODs) is less sensitive to VPN IPs and will
generally work normally.

### Firewalls

If running on a system with application-layer firewall rules (e.g. Windows
Defender Firewall with outbound rules), ensure the twchfetch binary is allowed
to make outbound connections on port 443.
