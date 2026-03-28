package tui

import "charm.land/bubbles/v2/key"

// KeyMap defines all key bindings for the application.
type KeyMap struct {
	Quit        key.Binding
	Refresh     key.Binding
	Back        key.Binding
	Play        key.Binding
	CopyURL     key.Binding
	ShowVODs    key.Binding
	FilterAll   key.Binding
	FilterLive  key.Binding
	FilterOffline key.Binding
	Settings    key.Binding
	Help        key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
		Play: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p / Np", "play"),
		),
		CopyURL: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "copy url"),
		),
		ShowVODs: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "vods"),
		),
		FilterAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "all"),
		),
		FilterLive: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "online"),
		),
		FilterOffline: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "offline"),
		),
		Settings: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "settings"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}
