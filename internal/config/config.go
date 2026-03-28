package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	keyring "github.com/zalando/go-keyring"
)

const (
	keyringService = "twchfetch/config/go"
	keyringUser    = "oauth_token"
)

// TokenSource describes where the OAuth token was loaded from.
type TokenSource int

const (
	TokenSourceNone    TokenSource = iota // no token configured
	TokenSourceEnv                        // TWITCH_OAUTH_TOKEN environment variable
	TokenSourceKeyring                    // OS credential store (Windows Credential Manager, macOS Keychain, …)
	TokenSourceFile                       // config.toml plaintext — last-resort fallback only
)

type DisplayConfig struct {
	VodMaxDisplay         int    `toml:"vod_max_display"`
	VodCategoriesMaxShown int    `toml:"vod_categories_max_shown"`
	ListSplitAt           int    `toml:"list_split_at"`
	CardWidth             int    `toml:"card_width"`
	CardColumns           int    `toml:"card_columns"`
	CardPadH              int    `toml:"card_pad_h"`
	CardPadV              int    `toml:"card_pad_v"`
	CardLiveColor         string `toml:"card_live_color"`
	CardSelectColor       string `toml:"card_select_color"`
	DisplayMode           string `toml:"display_mode"`    // "cards" | "list"
	ListTableCount        int    `toml:"list_table_count"` // side-by-side tables in list mode
}

type StreamersConfig struct {
	List []string `toml:"list"`
}

type ChatConfig struct {
	MaxMessages      int  `toml:"max_messages"`       // chat buffer size; default 300; clamped [100, 1000]
	EmoteColors      bool `toml:"emote_colors"`       // grey-tint emote text; default true
	StripDingbats    bool `toml:"strip_dingbats"`     // remove Dingbats (U+2700–U+27BF) that render as wide replacement boxes in most terminal fonts
	MaxReconnects    int  `toml:"max_reconnects"`     // auto-reconnect attempts before stopping (0 = unlimited); default 5
	ShowReply        bool `toml:"show_reply"`         // show ↩ @user · prefix on reply messages; default true
	TrimReplyMention bool `toml:"trim_reply_mention"` // when show_reply is on, strip the leading @username from the message body; default true
	CollapseRepeats  string `toml:"collapse_repeats"` // "off", "single" (same user only), or "all" (any user same text); default "single"
	AltRowColor      string `toml:"alt_row_color"`    // hex color (#RRGGBB) for alternating row background; empty = disabled
	LocalizedNames   bool `toml:"localized_names"`    // append ASCII login alias after non-ASCII display names, e.g. 名前 (name123); default true
	TextBadges          bool `toml:"text_badges"`           // use [Name] text labels instead of icon glyphs for chat badges; default false
	ThirdPartyEmotes    bool `toml:"third_party_emotes"`    // fetch BTTV/7TV emotes and style them distinctively; default false
	ThirdPartyShading   bool `toml:"third_party_shading"`   // apply grey tint to BTTV/7TV emotes (same as native emote_colors); default false
}

type Config struct {
	ClientID           string          `toml:"client_id"`
	OAuthToken         string          `toml:"oauth_token"` // kept for plaintext fallback; normally empty when keyring is used
	PlayerPath         string          `toml:"player_path"`
	PlayerArgs         []string        `toml:"player_args"`
	ChapterHash        string          `toml:"chapter_hash"`
	RefreshMaxWorkers  int             `toml:"refresh_max_workers"`
	RefreshBatchSize   int             `toml:"refresh_batch_size"`
	RequestTimeoutSec  int             `toml:"request_timeout_sec"`
	AutoRefreshMinutes   int             `toml:"auto_refresh_minutes"`   // 0 = disabled
	CacheOverrideMinutes int             `toml:"cache_override_minutes"` // 0 = no TTL expiry
	DebugLogging       bool            `toml:"debug_logging"`
	Display            DisplayConfig   `toml:"display"`
	Streamers          StreamersConfig `toml:"streamers"`
	Chat               ChatConfig      `toml:"chat"`
}

// ValidateOAuthToken normalises and validates a Twitch OAuth token string.
// It accepts:
//   - raw user access tokens:  [a-zA-Z0-9]{20,}
//   - tokens with an "oauth:" prefix (IRC / Streamlink style) which is stripped
//
// Returns the normalised token (prefix stripped, original case preserved) and
// nil on success.  Returns an error describing the problem on failure.
func ValidateOAuthToken(s string) (string, error) {
	s = strings.TrimSpace(s)
	// Strip IRC-style "oauth:" prefix used by Streamlink and some chat clients.
	if strings.HasPrefix(strings.ToLower(s), "oauth:") {
		s = strings.TrimSpace(s[len("oauth:"):])
	}
	if s == "" {
		return "", fmt.Errorf("token is empty")
	}
	for _, ch := range s {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
			return "", fmt.Errorf("token contains invalid character %q (must be alphanumeric)", ch)
		}
	}
	if len(s) < 20 {
		return "", fmt.Errorf("token too short: got %d chars, expected at least 20", len(s))
	}
	if len(s) > 200 {
		return "", fmt.Errorf("token too long: got %d chars", len(s))
	}
	return s, nil
}

func Defaults() *Config {
	return &Config{
		ClientID:          "kimne78kx3ncx6brgo4mv6wki5h1ko",
		PlayerPath:        "",
		PlayerArgs:        []string{},
		ChapterHash:       "71835d5ef425e154bf282453a926d99b328cdc5e32f36d3a209d0f4778b41203",
		RefreshMaxWorkers:  16,
		RefreshBatchSize:   5,
		RequestTimeoutSec:  10,
		AutoRefreshMinutes:  5,
		CacheOverrideMinutes: 15,
		DebugLogging:       false,
		Chat: ChatConfig{
			MaxMessages:      300,
			EmoteColors:      true,
			StripDingbats:    true,
			MaxReconnects:    5,
			ShowReply:        true,
			TrimReplyMention: true,
			CollapseRepeats:  "single",
			LocalizedNames:   true,
		},
		Display: DisplayConfig{
			VodMaxDisplay:         10,
			VodCategoriesMaxShown: 5,
			ListSplitAt:           20,
			CardWidth:             22,
			CardColumns:           0,
			CardPadH:              2,
			CardPadV:              0,
			CardLiveColor:         "#102910",
			CardSelectColor:       "#2C2260",
			DisplayMode:           "cards",
			ListTableCount:        1,
		},
		Streamers: StreamersConfig{
			List: []string{},
		},
	}
}

// Load reads config.toml (if it exists), applies defaults, then resolves the
// OAuth token using the priority chain: env var → OS keyring → TOML plaintext.
// The returned TokenSource tells callers exactly where the token came from.
func Load(path string) (*Config, TokenSource, error) {
	cfg := Defaults()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, TokenSourceNone, err
		}
		// Migrate bool fields from pre-v2 configs to their string equivalents.
		raw = bytes.ReplaceAll(raw, []byte("collapse_repeats = true"), []byte(`collapse_repeats = "single"`))
		raw = bytes.ReplaceAll(raw, []byte("collapse_repeats = false"), []byte(`collapse_repeats = "off"`))
		// alt_rows was a bool before alt_row_color; drop the old key so TOML
		// doesn't error on the type mismatch (the new field defaults to "").
		raw = bytes.ReplaceAll(raw, []byte("alt_rows = true"), []byte(`alt_row_color = "#261A48"`))
		raw = bytes.ReplaceAll(raw, []byte("alt_rows = false"), []byte(`alt_row_color = ""`))
		if _, err := toml.NewDecoder(strings.NewReader(string(raw))).Decode(cfg); err != nil {
			return nil, TokenSourceNone, fmt.Errorf("Error loading config: %w", err)
		}
	}

	// Apply mandatory defaults if TOML left fields empty.
	if cfg.ClientID == "" {
		cfg.ClientID = "kimne78kx3ncx6brgo4mv6wki5h1ko"
	}
	if cfg.ChapterHash == "" {
		cfg.ChapterHash = "71835d5ef425e154bf282453a926d99b328cdc5e32f36d3a209d0f4778b41203"
	}
	if cfg.RefreshMaxWorkers == 0 {
		cfg.RefreshMaxWorkers = 16
	}
	if cfg.RefreshBatchSize == 0 {
		cfg.RefreshBatchSize = 5
	}
	if cfg.RequestTimeoutSec == 0 {
		cfg.RequestTimeoutSec = 10
	}
	// AutoRefreshMinutes: 0 is a valid user-specified value (disabled); no default
	// override here — Defaults() handles first-run users by setting it to 5.
	if cfg.Display.VodMaxDisplay == 0 {
		cfg.Display.VodMaxDisplay = 10
	}
	if cfg.Display.VodCategoriesMaxShown == 0 {
		cfg.Display.VodCategoriesMaxShown = 5
	}
	if cfg.Display.ListSplitAt == 0 {
		cfg.Display.ListSplitAt = 20
	}
	if cfg.Display.CardWidth == 0 {
		cfg.Display.CardWidth = 22
	}
	if cfg.Display.CardLiveColor == "" {
		cfg.Display.CardLiveColor = "#102910"
	}
	if cfg.Display.CardSelectColor == "" {
		cfg.Display.CardSelectColor = "#2C2260"
	}
	if cfg.Display.DisplayMode == "" {
		cfg.Display.DisplayMode = "cards"
	}
	if cfg.Display.ListTableCount == 0 {
		cfg.Display.ListTableCount = 1
	}
	if cfg.Chat.MaxMessages == 0 {
		cfg.Chat.MaxMessages = 300
	}
	// MaxReconnects: 0 is a valid user value (unlimited); only apply default
	// if the field was never set (negative would be invalid — clamp to 0).
	if cfg.Chat.MaxReconnects < 0 {
		cfg.Chat.MaxReconnects = 0
	}

	// Stash any TOML plaintext value, then clear the field — we'll repopulate
	// from the winning source below.
	tomlToken := cfg.OAuthToken
	cfg.OAuthToken = ""

	// Priority 1: environment variable — explicit per-session override.
	if v := os.Getenv("TWITCH_OAUTH_TOKEN"); v != "" {
		cfg.OAuthToken = v
		return cfg, TokenSourceEnv, nil
	}

	// Priority 2: OS keyring — secure persistent storage.
	if v, err := keyring.Get(keyringService, keyringUser); err == nil && v != "" {
		cfg.OAuthToken = v
		return cfg, TokenSourceKeyring, nil
	}

	// Priority 3: TOML plaintext — last-resort fallback.
	if tomlToken != "" {
		cfg.OAuthToken = tomlToken
		return cfg, TokenSourceFile, nil
	}

	return cfg, TokenSourceNone, nil
}

// Save writes the config to path in TOML format.
// The OAuth token is saved to the OS keyring when possible; only on failure does
// it fall back to plaintext in the TOML file.
// prevSrc is the TokenSource from the last Load/Save so Save knows whether a
// keyring entry exists and needs removing when the token is cleared.
// Returns the TokenSource that was actually used so callers can update UI state.
func Save(path string, cfg *Config, prevSrc TokenSource) (TokenSource, error) {
	// Work on a shallow copy so we don't mutate the caller's in-memory config.
	toSave := *cfg
	token := toSave.OAuthToken
	toSave.OAuthToken = "" // default: don't write token to TOML

	tokenSrc := TokenSourceNone
	if token != "" {
		if err := keyring.Set(keyringService, keyringUser, token); err != nil {
			// Keyring unavailable — fall back to plaintext in TOML.
			toSave.OAuthToken = token
			tokenSrc = TokenSourceFile
		} else {
			tokenSrc = TokenSourceKeyring
		}
	} else if prevSrc == TokenSourceKeyring {
		// Token was explicitly cleared and was previously in the OS keyring —
		// remove the entry.  We only attempt this when the token actually lived
		// in the keyring; file-sourced tokens were never written there, so no
		// delete is needed for that case.
		_ = keyring.Delete(keyringService, keyringUser)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(toSave); err != nil {
		return TokenSourceNone, err
	}
	// 0600: only the file owner can read the config (guards the plaintext fallback).
	return tokenSrc, os.WriteFile(path, buf.Bytes(), 0600)
}
