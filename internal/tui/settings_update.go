package tui

import (
	tea "charm.land/bubbletea/v2"

	"twchfetch/internal/config"
	"twchfetch/internal/tui/views"
)

func (m Model) openSettings() (Model, tea.Cmd) {
	m.view = viewSettings
	m.settings = views.NewSettingsModel(
		m.cfg.PlayerType,
		m.cfg.PlayerPath,
		views.PlayerArgsFromSlice(m.cfg.PlayerArgs),
		m.cfg.OAuthToken,
		views.StreamersFromSlice(m.cfg.Streamers.List),
		m.cfg.AutoRefreshMinutes,
		m.cfg.CacheOverrideMinutes,
		m.cfg.Display.DisplayMode,
		m.cfg.Display.CardWidth,
		m.cfg.Display.CardColumns,
		m.cfg.Display.CardPadH,
		m.cfg.Display.CardPadV,
		m.cfg.Display.CardLiveColor,
		m.cfg.Display.CardSelectColor,
		m.cfg.Display.ListTableCount,
		m.cfg.Chat.MaxMessages,
		m.cfg.Chat.EmoteColors,
		m.cfg.Chat.StripDingbats,
		m.cfg.Chat.MaxReconnects,
		m.cfg.Chat.ShowReply,
		m.cfg.Chat.TrimReplyMention,
		m.cfg.Chat.CollapseRepeats,
		m.cfg.Chat.LocalizedNames,
		m.cfg.Chat.TextBadges,
		m.cfg.Chat.AltRowColor,
		m.cfg.Chat.ThirdPartyEmotes,
		m.cfg.Chat.ThirdPartyShading,
		m.tokenSource,
	)
	return m, m.settings.Fields[m.settings.FocusIndex].Focus()
}

// settingsVisibleCount returns how many fields fit in the current terminal
// window for the settings view.  Matches the chrome derived in RenderSettings.
func settingsVisibleCount(m Model) int {
	h := m.height - m.headerHeight() - 1 // available height for the settings body
	// chrome = ScreenSettings.ChromeLines(SettingsSubHeaderLines, SettingsFooterLines)
	chrome := views.ScreenSettings.ChromeLines(views.SettingsSubHeaderLines, views.SettingsFooterLines)
	available := h - chrome
	if available < 3 {
		available = 3
	}
	return available / 3
}

// scrollSettingsToFocus adjusts ScrollOffset so that the currently focused
// field is always within the visible window.  No-op when the header is focused.
func (m Model) scrollSettingsToFocus() Model {
	if m.settings.FocusIndex < 0 {
		return m
	}
	sf := views.ActiveSectionFields(m.settings, m.settings.Section)
	pos := -1
	for i, fi := range sf {
		if fi == m.settings.FocusIndex {
			pos = i
			break
		}
	}
	if pos < 0 {
		return m
	}
	vc := settingsVisibleCount(m)
	if vc < 1 {
		vc = 1
	}
	so := m.settings.ScrollOffset
	if pos < so {
		so = pos
	} else if pos >= so+vc {
		so = pos - vc + 1
	}
	maxOffset := len(sf) - vc
	if maxOffset < 0 {
		maxOffset = 0
	}
	if so > maxOffset {
		so = maxOffset
	}
	if so < 0 {
		so = 0
	}
	m.settings.ScrollOffset = so
	return m
}

func (m Model) updateSettings(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Help tab is read-only — intercept all keys before field logic runs.
	if m.settings.Section == views.SectionHelp {
		availableHeight := m.height - m.headerHeight() - 1
		switch msg.String() {
		case "up", "k":
			m.settings.HelpScrollOffset--
		case "down", "j":
			m.settings.HelpScrollOffset++
		case "pgup":
			m.settings.HelpScrollOffset -= 10
		case "pgdn":
			m.settings.HelpScrollOffset += 10
		case "left":
			m.settings.Section = (m.settings.Section - 1 + len(views.SectionFields)) % len(views.SectionFields)
			m.settings.ScrollOffset = 0
		case "right":
			m.settings.Section = (m.settings.Section + 1) % len(views.SectionFields)
			m.settings.ScrollOffset = 0
			if m.settings.Section == views.SectionHelp {
				m.settings.HelpScrollOffset = 0
			}
		case "esc", "backspace":
			m.view = viewList
			m.cursorPosF = float64(m.listCursor)
			m.cursorVelF = 0
			return m, cardAnimCmd()
		}
		// Always clamp after any offset change so the model is never out of range.
		m.settings.ClampHelpScroll(m.width, availableHeight)
		return m, nil
	}

	fi := m.settings.FocusIndex
	sf := views.ActiveSectionFields(m.settings, m.settings.Section)

	switch msg.String() {
	case "esc":
		m.view = viewList
		// Restart the 60 Hz tick chain and snap the cursor spring — same
		// reasoning as updateDetails: the tick stops on sub-screens.
		m.cursorPosF = float64(m.listCursor)
		m.cursorVelF = 0
		return m, cardAnimCmd()

	case "ctrl+d":
		// Flag intent to clear the token — do NOT touch m.cfg yet.
		// If the user presses esc the flag is discarded with the settings model
		// and the live session token is never touched.  The actual deletion from
		// the OS keyring happens inside saveSettingsCmd when the user saves.
		if fi == views.FieldOAuthToken {
			m.settings.ClearToken = true
			m.settings.TokenSource = config.TokenSourceNone
			m.settings.Fields[views.FieldOAuthToken].Placeholder = views.OAuthPlaceholder(config.TokenSourceNone)
			m, cmd := m.showOKDialog("Token Queued for Removal", "Token will be cleared on save.\nPress esc to cancel without saving.")
			return m, cmd
		}

	case "enter":
		if fi < 0 {
			// Activate first field of the current section.
			if len(sf) > 0 {
				m.settings.FocusIndex = sf[0]
				m = m.scrollSettingsToFocus()
				return m, m.refocusSettings()
			}
			return m, nil
		}
		return m, m.saveSettingsCmd()

	case "tab", "down":
		if fi < 0 {
			if len(sf) > 0 {
				m.settings.FocusIndex = sf[0]
			}
		} else {
			next := -1
			for i, f := range sf {
				if f == fi && i+1 < len(sf) {
					next = sf[i+1]
					break
				}
			}
			m.settings.FocusIndex = next
		}
		m = m.scrollSettingsToFocus()
		return m, m.refocusSettings()

	case "shift+tab", "up":
		if fi < 0 {
			if len(sf) > 0 {
				m.settings.FocusIndex = sf[len(sf)-1]
			}
		} else {
			prev := -1
			for i, f := range sf {
				if f == fi && i > 0 {
					prev = sf[i-1]
					break
				}
			}
			m.settings.FocusIndex = prev
		}
		m = m.scrollSettingsToFocus()
		return m, m.refocusSettings()

	case "pgup":
		vc := settingsVisibleCount(m)
		if fi >= 0 {
			pos := -1
			for i, f := range sf {
				if f == fi {
					pos = i
					break
				}
			}
			if pos >= 0 {
				newPos := pos - vc
				if newPos < 0 {
					newPos = 0
				}
				m.settings.FocusIndex = sf[newPos]
				m = m.scrollSettingsToFocus()
				return m, m.refocusSettings()
			}
		} else {
			m.settings.ScrollOffset -= vc
			if m.settings.ScrollOffset < 0 {
				m.settings.ScrollOffset = 0
			}
		}
		return m, nil

	case "pgdn":
		vc := settingsVisibleCount(m)
		if fi >= 0 {
			pos := -1
			for i, f := range sf {
				if f == fi {
					pos = i
					break
				}
			}
			if pos >= 0 {
				newPos := pos + vc
				if newPos >= len(sf) {
					newPos = len(sf) - 1
				}
				m.settings.FocusIndex = sf[newPos]
				m = m.scrollSettingsToFocus()
				return m, m.refocusSettings()
			}
		} else {
			maxOffset := len(sf) - vc
			if maxOffset < 0 {
				maxOffset = 0
			}
			m.settings.ScrollOffset += vc
			if m.settings.ScrollOffset > maxOffset {
				m.settings.ScrollOffset = maxOffset
			}
		}
		return m, nil

	case "left":
		if fi < 0 {
			m.settings.Section = (m.settings.Section - 1 + len(views.SectionFields)) % len(views.SectionFields)
			m.settings.ScrollOffset = 0
			if m.settings.Section == views.SectionHelp {
				m.settings.HelpScrollOffset = 0
				return m, nil
			}
			newSF := views.ActiveSectionFields(m.settings, m.settings.Section)
			if len(newSF) > 0 {
				m.settings.FocusIndex = newSF[0]
			}
			m = m.scrollSettingsToFocus()
			return m, m.refocusSettings()
		}
		// Forward to text input below.

	case "right":
		if fi < 0 {
			m.settings.Section = (m.settings.Section + 1) % len(views.SectionFields)
			m.settings.ScrollOffset = 0
			if m.settings.Section == views.SectionHelp {
				m.settings.HelpScrollOffset = 0
				return m, nil
			}
			newSF := views.ActiveSectionFields(m.settings, m.settings.Section)
			if len(newSF) > 0 {
				m.settings.FocusIndex = newSF[0]
			}
			m = m.scrollSettingsToFocus()
			return m, m.refocusSettings()
		}
		// Forward to text input below.
	}

	if fi >= 0 {
		c := views.FieldConstraints[fi]
		// Enum fields cycle through their fixed value set with space and arrow
		// keys.  All other input — including backspace and cursor movement — is
		// swallowed so the value stays in a valid state at all times.
		if c.IsEnum() {
			switch msg.String() {
			case " ", "right":
				newVal := c.Cycle(m.settings.Fields[fi].Value(), +1)
				// Gate the two third-party API fields behind a confirmation dialog
				// so users are informed about external data requests before enabling.
				if newVal == "on" && (fi == views.FieldChatThirdPartyEmotes || fi == views.FieldChatThirdPartyShading) {
					capturedFi := fi
					m2, cmd := m.showConfirmDialog(
						"Third-Party API Notice",
						"This feature sends requests to third-party servers:\n\n"+
							"  • BTTV  — api.betterttv.net\n"+
							"  • 7TV   — 7tv.io\n\n"+
							"By enabling this you consent to contacting those\n"+
							"services. Review their terms before proceeding.",
						func(m Model, confirmed bool) Model {
							if confirmed {
								m.settings.Fields[capturedFi].SetValue("on")
							}
							return m
						})
					return m2, cmd
				}
				m.settings.Fields[fi].SetValue(newVal)
		case "left":
			m.settings.Fields[fi].SetValue(c.Cycle(m.settings.Fields[fi].Value(), -1))
		}
		// Sync dependent placeholders whenever player type changes.
		if fi == views.FieldPlayerType {
			m.settings = views.SyncPlayerTypePlaceholders(m.settings)
		}
		return m, nil
		}
		// For free-text and numeric fields, drop printable characters that are
		// outside the accepted set for this field so invalid input never reaches
		// the underlying text-input model.
		if isPrintable(msg.String()) && !c.AllowChar(msg.String()) {
			return m, nil
		}
		var cmd tea.Cmd
		m.settings.Fields[fi], cmd = m.settings.Fields[fi].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) refocusSettings() tea.Cmd {
	cmds := make([]tea.Cmd, views.FieldCount)
	for i := range m.settings.Fields {
		if i == m.settings.FocusIndex {
			cmds[i] = m.settings.Fields[i].Focus()
		} else {
			m.settings.Fields[i].Blur()
		}
	}
	return tea.Batch(cmds...)
}

func (m Model) saveSettingsCmd() tea.Cmd {
	// Validate the token first — before any mutations — so a bad token never
	// leaves m.cfg in a partially-updated state.
	newToken := m.settings.Fields[views.FieldOAuthToken].Value()
	var normalizedToken string
	if !m.settings.ClearToken && newToken != "" {
		var valErr error
		normalizedToken, valErr = config.ValidateOAuthToken(newToken)
		if valErr != nil {
			prevSrc := m.tokenSource
			return func() tea.Msg {
				return SaveConfigDoneMsg{Err: valErr, TokenSource: prevSrc}
			}
		}
	}

	m.cfg.PlayerType = views.PlayerTypeFromString(m.settings.Fields[views.FieldPlayerType].Value())
	m.cfg.PlayerPath = m.settings.Fields[views.FieldPlayerPath].Value()
	m.cfg.PlayerArgs = views.PlayerArgsToSlice(m.settings.Fields[views.FieldPlayerArgs].Value())

	// Resolve the token to save.
	// Priority: explicit ctrl+d clear > typed new value > existing in-memory token.
	switch {
	case m.settings.ClearToken:
		// User explicitly pressed ctrl+d — wipe the token.  config.Save will
		// call keyring.Delete so the OS credential store entry is also removed.
		m.cfg.OAuthToken = ""
	case normalizedToken != "":
		// User typed a valid replacement.
		m.cfg.OAuthToken = normalizedToken
	case m.tokenSource == config.TokenSourceKeyring:
		// Empty field + keyring source and no explicit clear: keep existing token.
	case m.tokenSource == config.TokenSourceEnv:
		// Env var is authoritative — don't clobber it via the settings form.
	default:
		// Empty field + file or none source: treat as intentional clear.
		m.cfg.OAuthToken = ""
	}

	if sl := views.StreamersToSlice(m.settings.Fields[views.FieldStreamers].Value()); len(sl) > 0 {
		m.cfg.Streamers.List = sl
	}
	m.cfg.AutoRefreshMinutes = views.AutoRefreshFromString(m.settings.Fields[views.FieldAutoRefresh].Value())
	m.cfg.CacheOverrideMinutes = views.CacheOverrideFromString(m.settings.Fields[views.FieldCacheOverride].Value())
	m.cfg.Display.DisplayMode = views.DisplayModeFromString(m.settings.Fields[views.FieldDisplayMode].Value())
	m.cfg.Display.CardWidth = views.CardWidthFromString(m.settings.Fields[views.FieldCardWidth].Value())
	m.cfg.Display.CardColumns = views.CardColumnsFromString(m.settings.Fields[views.FieldCardColumns].Value())
	m.cfg.Display.CardPadH = views.CardPadFromString(m.settings.Fields[views.FieldCardPadH].Value())
	m.cfg.Display.CardPadV = views.CardPadFromString(m.settings.Fields[views.FieldCardPadV].Value())
	m.cfg.Display.CardLiveColor = views.ColorFromString(m.settings.Fields[views.FieldCardLiveColor].Value())
	m.cfg.Display.CardSelectColor = views.ColorFromString(m.settings.Fields[views.FieldCardSelectColor].Value())
	m.cfg.Display.ListTableCount = views.ListTableCountFromString(m.settings.Fields[views.FieldListTables].Value())
	m.cfg.Chat.MaxMessages   = views.ChatMaxMessagesFromString(m.settings.Fields[views.FieldChatMaxMessages].Value())
	m.cfg.Chat.EmoteColors   = views.ChatEmoteColorsFromString(m.settings.Fields[views.FieldChatEmoteColors].Value())
	m.cfg.Chat.StripDingbats = views.ChatStripDingbatsFromString(m.settings.Fields[views.FieldChatStripDingbats].Value())
	m.cfg.Chat.MaxReconnects    = views.ChatMaxReconnectsFromString(m.settings.Fields[views.FieldChatMaxReconnects].Value())
	m.cfg.Chat.ShowReply        = views.ChatShowReplyFromString(m.settings.Fields[views.FieldChatShowReply].Value())
	m.cfg.Chat.TrimReplyMention = views.ChatTrimReplyMentionFromString(m.settings.Fields[views.FieldChatTrimReplyMention].Value())
	m.cfg.Chat.CollapseRepeats  = views.ChatCollapseRepeatsFromString(m.settings.Fields[views.FieldChatCollapseRepeats].Value())
	m.cfg.Chat.LocalizedNames   = views.ChatLocalizedNamesFromString(m.settings.Fields[views.FieldChatLocalizedNames].Value())
	m.cfg.Chat.TextBadges       = views.ChatTextBadgesFromString(m.settings.Fields[views.FieldChatTextBadges].Value())
	m.cfg.Chat.AltRowColor        = views.ChatAltRowColorFromString(m.settings.Fields[views.FieldChatAltRows].Value())
	m.cfg.Chat.ThirdPartyEmotes   = views.ChatThirdPartyEmotesFromString(m.settings.Fields[views.FieldChatThirdPartyEmotes].Value())
	m.cfg.Chat.ThirdPartyShading  = views.ChatThirdPartyShadingFromString(m.settings.Fields[views.FieldChatThirdPartyShading].Value())
	cfgPath, cfg, prevSrc := m.cfgPath, m.cfg, m.tokenSource
	return func() tea.Msg {
		src, err := config.Save(cfgPath, cfg, prevSrc)
		return SaveConfigDoneMsg{Err: err, TokenSource: src}
	}
}
