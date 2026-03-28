package player

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// mpvCandidates are common install locations for mpv on Windows.
var mpvCandidates = []string{
	`C:\mpv\mpv.exe`,
	`C:\Program Files\MPV\mpv.exe`,
	`C:\Program Files (x86)\MPV\mpv.exe`,
	`C:\ProgramData\chocolatey\bin\mpv.exe`,
}

// vlcCandidates are common install locations for VLC on Windows.
var vlcCandidates = []string{
	`C:\Program Files\VideoLAN\VLC\vlc.exe`,
	`C:\Program Files (x86)\VideoLAN\VLC\vlc.exe`,
}

// DefaultCandidates is kept for backwards compatibility; points to mpvCandidates.
var DefaultCandidates = mpvCandidates

// candidatesForType returns the default install-location candidates for the
// given player type. Falls back to mpv candidates for unknown types.
func candidatesForType(playerType string) ([]string, string) {
	switch strings.ToLower(playerType) {
	case "vlc":
		return vlcCandidates, "vlc"
	default:
		return mpvCandidates, "mpv"
	}
}

// Launch opens url in the configured player. playerPath is tried first;
// if empty or not found, default install locations for playerType are searched,
// then PATH. extraArgs are appended after the URL.
func Launch(url, playerPath, playerType string, extraArgs []string) error {
	resolved, err := resolve(playerPath, playerType)
	if err != nil {
		return err
	}

	args := append([]string{url}, extraArgs...)
	cmd := exec.Command(resolved, args...)
	setDetached(cmd) // platform-specific: detach from terminal on Windows
	return cmd.Start()
}

// resolve finds the first usable player binary.
// If playerPath is set, it is tried first. When empty or not found, default
// install locations for the player type are searched, then PATH.
// An error is returned only when no usable binary can be found at all.
func resolve(configured, playerType string) (string, error) {
	// Try the explicitly configured path first.
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, nil
		}
	}

	// Try default install-location candidates for the selected player type.
	candidates, typeName := candidatesForType(playerType)
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// Fall back to PATH lookup. If a custom path was configured, try its
	// executable name first, then fall back to the player type name.
	if configured != "" {
		if name := playerNameFromPath(configured); name != "" {
			if path, err := exec.LookPath(name); err == nil {
				return path, nil
			}
		}
	}
	if path, err := exec.LookPath(typeName); err == nil {
		return path, nil
	}

	if configured != "" {
		return "", fmt.Errorf("player not found: %q — check player_path in Settings (s key)", configured)
	}
	return "", fmt.Errorf("%s not found — install it or set player_path in Settings (s key)", typeName)
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
