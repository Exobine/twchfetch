package player

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DefaultCandidates are the fallback search paths tried when the configured
// player path does not exist.
var DefaultCandidates = []string{
	`C:\mpv\mpv.exe`,
	`C:\Program Files\MPV\mpv.exe`,
	`C:\Program Files (x86)\MPV\mpv.exe`,
	`C:\ProgramData\chocolatey\bin\mpv.exe`,
}

// Launch opens url in the configured player. playerPath is tried first;
// if empty or not found, DefaultCandidates are searched, then PATH.
// extraArgs are appended after the URL (e.g. "--volume=80").
func Launch(url, playerPath string, extraArgs []string) error {
	resolved, err := resolve(playerPath)
	if err != nil {
		return err
	}

	args := append([]string{url}, extraArgs...)
	cmd := exec.Command(resolved, args...)
	setDetached(cmd) // platform-specific: detach from terminal on Windows
	return cmd.Start()
}

// resolve finds the first usable player binary.
// When configured is empty, playback is considered disabled and an error is
// returned immediately — DefaultCandidates and PATH are not searched.
// When configured is set but the exact path is not found, DefaultCandidates
// and PATH are tried as fallbacks.
func resolve(configured string) (string, error) {
	if configured == "" {
		return "", fmt.Errorf("no player configured — set player_path in Settings (s key)")
	}

	// Try the configured path first.
	if _, err := os.Stat(configured); err == nil {
		return configured, nil
	}

	// Configured path not found — try DefaultCandidates as fallback.
	for _, p := range DefaultCandidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// Fall back to PATH lookup using the executable name from configured path.
	name := playerNameFromPath(configured)
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	// Also try bare "mpv" if configured name differs.
	if name != "mpv" {
		if path, err := exec.LookPath("mpv"); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("player not found: %q — check player_path in Settings (s key)", configured)
}

// playerNameFromPath extracts the executable name from a full path.
func playerNameFromPath(p string) string {
	if p == "" {
		return ""
	}
	// Normalize separators
	p = strings.ReplaceAll(p, `\`, "/")
	parts := strings.Split(p, "/")
	name := parts[len(parts)-1]
	// Strip .exe on Windows
	name = strings.TrimSuffix(name, ".exe")
	return name
}

// ValidatePath checks if the given path points to an executable.
// Returns nil if usable, error with a description otherwise.
func ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	info, err := os.Stat(path)
	if err != nil {
		// May still be in PATH
		name := playerNameFromPath(path)
		if _, err2 := exec.LookPath(name); err2 == nil {
			return nil
		}
		return fmt.Errorf("not found: %v", err)
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file")
	}
	return nil
}
