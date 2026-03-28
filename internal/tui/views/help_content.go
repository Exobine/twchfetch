package views

// helpMarkdown is the single source of truth for all Help tab content.
// Rendered by glamour at display time. To update: edit markdown here only.
// No other file needs touching when content changes.
//
// Note: Go raw string literals cannot contain backtick characters, so inline
// code spans use bold instead throughout this document.
var helpMarkdown = `# twchfetch — Help

## Navigation

### List
- **↑↓ / j k** — move cursor
- **enter** — open details
- **a / o / f** — filter: all / live / offline
- **r** — manual refresh
- **/** — search
- **s** — settings  |  **q** — quit

### Details
- **↑↓** — scroll
- **t** — open chat  |  **v** — VODs  |  **p** — play stream
- **esc** — back to list

### VODs
- **↑↓** — scroll  |  **enter** — play VOD  |  **esc** — back

### Chat
- **↑↓** — scroll  |  **g / G** — top / bottom
- **/** — search messages  |  **h** — cycle scroll mode
- **esc** — back

### Settings
- **← →** — switch tab  |  **tab / ↑↓** — navigate fields
- **enter** — save  |  **esc** — discard & back

---

## Search

Applies in the streamer list and the chat view.

### Operators
- **space** or **&** — AND: both terms must match
- **|** — OR: either term matches
- **!** — NOT: exclude matching items
- **( )** — grouping: **(sub | gift) & !mod**

### Qualifiers
- **"..."** — exact phrase: **"good game"**
- **^** — starts-with: **^cohh**
- **$** — ends-with: **gg$**
- **^term$** — exact match

### Field targeting (chat only)
- **user:term** — match display name or login
- **msg:term** — match message text only
- **badge:term** — match badge or event type

Badge values: **mod**  **sub**  **vip**  **broadcaster**

Event values: **raid**  **gift**  **ritual**  **timeout**  **ban**  **delete**

### Examples
- **sub | gift** — subs or gift events
- **!timeout & !ban** — hide mod actions
- **user:^cohh** — names starting with "cohh"
- **badge:raid** — raid announcements only
- **"good game"** — exact phrase anywhere
- **sub | !mod** — avoid: matches nearly everything

---

## Chat

### Badges
| Glyph | Role | Text mode |
|---|---|---|
| **●** | Broadcaster | [Broadcaster] |
| **⚔** | Moderator | [Mod] |
| **■** | Subscriber | [Sub] |
| **◆** | VIP | [VIP] |
| **⚑** | Staff | [Staff] |
| **⚡** | Turbo | [Turbo] |

Text mode replaces glyphs with bracketed labels — useful for terminals that cannot render the symbols. Toggle in Settings under **Text badges**.

### Mod actions
Messages are never removed — only cosmetically dimmed.

- **[banned]** — user permanently banned
- **[timeout Xm]** — timed out; message greyed
- **[deleted]** — single message deleted by a moderator

### System events
Subscriptions, gift bombs, raids, and rituals appear as coloured system lines.

### Gift bomb suppression
When a gifter sends multiple subs at once only the summary line is shown; individual gift pings are suppressed for 30 seconds.

### Repeat collapse
Consecutive identical messages can be merged into one line with a **[x3]** counter.

- **off** — no collapsing
- **single** — same user repeating their own message
- **all** — any user posting the same text as the previous message

### Scroll modes  (cycle with h)
- **Live** — always jumps to the latest message
- **Frozen** — holds position; new messages queue silently below
- **Follow** — advances by the height of each new message

### Auto-scroll & unread
Pauses when you scroll up. Resumes on **G**. Unread count shown in the details header while paused.

---

## Config tips

- **Streamers field** is ignored when an OAuth token is set — your followed channels are fetched automatically instead.
- **Auto refresh** runs silently in the background; a countdown timer appears in the tab bar. Set **0** to disable.
- **Cache override** — minutes before details/VODs are re-fetched. **0** = fetch every visit.
- **Card columns = 0** — auto-fits as many columns as the terminal width allows given the card width.
- **Display mode** — the tab beside this one relabels itself to "Cards" or "List" to match the active mode.
- **Text badges** — replaces icon glyphs (●, ⚔…) with [Broadcaster], [Mod]… for terminals that can't render them.
- **Strip Dingbats** — removes U+2700–U+27BF characters that render as wide replacement boxes in most coding fonts.
- **Max reconnects = 0** — unlimited auto-reconnect attempts on chat disconnect.
`
