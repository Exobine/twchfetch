package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"twchfetch/internal/config"
	"twchfetch/internal/logging"
	"twchfetch/internal/tui"
)

func main() {
	cfgPath := flag.String("config", "config.toml", "path to config.toml")
	flag.Parse()

	// Initialise per-session logger (wipes previous session log on start).
	if err := logging.Init("twchfetch.log"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: logging unavailable: %v\n", err)
	}

	cfg, tokenSrc, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	logging.Info("Config loaded", "path", *cfgPath,
		"streamers", len(cfg.Streamers.List),
		"token_source", tokenSrc)

	model := tui.NewModel(cfg, *cfgPath, tokenSrc)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
