# twchfetch

A terminal-based Twitch stream and VOD viewer with integrated live chat, written in Go.

Monitor a configurable list of channels, browse VODs, and watch live chat — all without leaving your terminal.

Built on the excellent TUI framework and tooling from [Charmbracelet](https://github.com/charmbracelet) — specifically [Bubbletea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), [Lipgloss](https://github.com/charmbracelet/lipgloss), [Glamour](https://github.com/charmbracelet/glamour), and [Huh](https://github.com/charmbracelet/huh). This project wouldn't exist in its current form without their work.

Also makes use of [BurntSushi/toml](https://github.com/BurntSushi/toml) for config parsing, [zalando/go-keyring](https://github.com/zalando/go-keyring) for secure OS credential storage, [coder/websocket](https://github.com/coder/websocket) for the IRC-over-WebSocket chat connection, and [golang.design/x/clipboard](https://pkg.go.dev/golang.design/x/clipboard) for system clipboard integration.

> **Platform:** currently tested on **Windows only**. Linux and macOS are supported in code but untested — behaviour may vary.
>
> **Stability:** the `main` branch is considered mostly functional. Some features are still in active development and may not behave as fully intended. See the `dev` branch for the latest in-progress changes.

---

## About

This is a project I wanted to make to avoid directly interacting with the ever-growing chaos that is the Twitch website. It started as a basic Python project but has grown substantially — with assistance from AI, Claude specifically.

> **"Why use AI? I hate it!"**
>
> It let me do the thing I wanted to do, and I'm learning from it. Regardless, the planning and iteration needed to get it working properly was still quite time consuming.

> **"Why release this? I still hate AI!"**
>
> Some people would love a simpler, more resource-friendly way of interacting with Twitch. This does most of the basic things you'd expect from navigating Twitch normally, but through a TUI — if you're into that sort of thing.

> **"It's complicated!"**
>
> It's a TUI — it mostly uses keyboard shortcuts to get things done and isn't necessarily fool-proof. A more user-friendly interface may come at some point, but not soon.

> **"What's this name 'twchfetch'?"**
>
> Literally just a random name. It means Twitch Fetch — we're fetching data and info from twitch.tv for use in the TUI app.

---

## Features

- **Stream list** — live/offline status for all tracked channels, auto-refreshing in the background
- **Card and list display modes** — switch between compact table rows or visual channel cards
- **VOD browser** — browse and open VODs per channel, filterable by category
- **Live chat** — full Twitch IRC chat including:
  - Emote colouring and third-party emotes (7TV / BTTV)
  - Coloured badge glyphs (● broadcaster, ⚔ mod, ■ sub, ◆ VIP) — switchable to text labels `[Mod]` / `[Sub]` etc. for terminals that can't render the symbols
  - Reply threading, repeat-message collapsing, localised display names
  - Gift bomb suppression — when a gifter sends multiple subs at once, individual gift pings are collapsed into a single summary line; works in both arrival orders
- **Search** — fuzzy search across tracked channels
- **Settings UI** — adjust display and chat options at runtime without editing the config file
- **Media player integration** — open any stream or VOD directly in mpv or VLC with a single keypress
- **Cross-platform build** — Windows, Linux, and macOS supported in code; currently tested on Windows only

---

## Requirements

| Dependency | Notes |
|---|---|
| [Go](https://go.dev/dl/) 1.26.1+ | Build toolchain |
| [mpv](https://mpv.io/) or [VLC](https://www.videolan.org/) | Stream / VOD playback (at least one required for playback; both optional) |
| Twitch OAuth token | Optional — enables follow-list and authenticated chat |

---

## Building from source

```bash
# 1. Clone the repository
git clone https://github.com/Exobine/twchfetch.git
cd twchfetch

# 2. Build
go build -o twchfetch .        # Linux / macOS
go build -o twchfetch.exe .    # Windows
```

A `config.toml` is created automatically the first time you save a setting in the app. You can also copy `config.example.toml` manually if you prefer to set values like `player_path` before first run.

---

## Configuration

Copy `config.example.toml` to `config.toml` and set at minimum:

| Key | Description |
|---|---|
| `client_id` | Twitch application Client ID |
| `oauth_token` | Your Twitch OAuth token (leave empty for unauthenticated mode) |
| `player_type` | Player type: `"mpv"` (default) or `"vlc"` — determines default install locations and argument style |
| `player_path` | Absolute path to your player binary. Leave empty to search default install locations and `$PATH` for the selected player type. |
| `streamers.list` | Array of Twitch login names to monitor |

### Player setup

twchfetch uses an external media player to open streams and VODs. Set `player_type` to match the player you have installed — `"mpv"` (default) or `"vlc"`.

`player_path` can be left empty. twchfetch will search the following in order:
1. Common default install locations for the selected player type
2. `$PATH` / `%PATH%`

Set `player_path` explicitly only if your player is installed somewhere non-standard:

```toml
# mpv — Windows
player_type = "mpv"
player_path = "C:\\mpv\\mpv.exe"

# VLC — Windows
player_type = "vlc"
player_path = "C:\\Program Files\\VideoLAN\\VLC\\vlc.exe"

# Linux / macOS (usually leave player_path empty — found via $PATH)
player_type = "mpv"
player_path = ""
```

> An error dialog is shown only when the player cannot be found at all (wrong explicit path, not on `$PATH`, not in any standard location). The rest of the app works without a player configured.

---

> `config.toml` is gitignored — **never commit it**.

### OAuth token storage

twchfetch resolves the token using the following priority order:

| Priority | Method | How to set |
|---|---|---|
| 1 | **OS keyring** *(preferred)* | Enter your token in the in-app Settings UI — it is written directly to Windows Credential Manager, macOS Keychain, or the system's secret store on Linux. The `oauth_token` field in `config.toml` stays empty. |
| 2 | **Environment variable** | Set `TWITCH_OAUTH_TOKEN=<token>` in your shell for a per-session override. |
| 3 | **config.toml plaintext** | Set `oauth_token = "..."` directly in the file — last-resort fallback only, used when the keyring is unavailable. The file is written with `0600` permissions (owner read/write only). |

The keyring method is strongly recommended — your token never touches the filesystem in plaintext.

---

## Usage

```bash
# Run with default config.toml in the current directory
./twchfetch

# Specify a different config path
./twchfetch -config /path/to/config.toml
```

### Key bindings (defaults)

#### Channel list
| Key | Action |
|---|---|
| `↑ / ↓` or `j / k` | Navigate |
| `Enter` | Open channel details |
| `/` | Search |
| `a / o / f` | Filter: all / live / offline |
| `r` | Force refresh |
| `s` | Settings |
| `q` | Quit |

#### Channel details
| Key | Action |
|---|---|
| `p` | Play stream in configured player |
| `t` | Open chat |
| `v` | Open VOD browser |
| `c` | Copy channel URL |
| `esc` | Back to list |

#### VODs
| Key | Action |
|---|---|
| `↑ / ↓` or `j / k` | Navigate |
| `Enter` | Play selected VOD |
| `esc` | Back |

---

## Twitch API notice

This application interacts with the [Twitch API](https://dev.twitch.tv/docs/api/) and the Twitch IRC chat servers under Twitch's [Developer Agreement](https://www.twitch.tv/p/en/legal/developer-agreement/).

The API integration in this project — the query design, caching strategy, parallel refresh approach, and chat processing pipeline — is original work and is protected under the project license below. The Twitch API itself and all Twitch trademarks remain the property of Twitch Interactive, Inc.

---

## License

Copyright (C) 2026 Exobine (https://github.com/Exobine)

This program is free software: you can redistribute it and/or modify it under
the terms of the **GNU Affero General Public License** as published by the Free
Software Foundation, either version 3 of the License, or (at your option) any
later version.

This program is distributed in the hope that it will be useful, but **without
any warranty**; without even the implied warranty of merchantability or fitness
for a particular purpose. See the GNU Affero General Public License for more
details.

A copy of the license is included in the [`LICENSE`](LICENSE) file.
Full text: <https://www.gnu.org/licenses/agpl-3.0.html>

---

## Built with assistance from

[Claude](https://claude.ai) (Anthropic) — used as a development assistant for code review, project scaffolding, and documentation.
