package util

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"twchfetch/internal/api"
)

// FormatDuration converts seconds to "2h 34m 12s" (seconds optional).
func FormatDuration(seconds int, includeSeconds bool) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if includeSeconds {
		return fmt.Sprintf("%dh %02dm %02ds", h, m, s)
	}
	return fmt.Sprintf("%dh %02dm", h, m)
}

// FormatDate parses an ISO-8601 string and returns "Jan 02 2006".
func FormatDate(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return t.Format("Jan 02 2006")
}

// FormatRelativeTime returns a human-readable "X ago" string.
func FormatRelativeTime(t time.Time) string {
	dur := time.Since(t)
	switch {
	case dur < time.Minute:
		return "just now"
	case dur < time.Hour:
		m := int(dur.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case dur < 24*time.Hour:
		h := int(dur.Hours())
		return fmt.Sprintf("%dh ago", h)
	case dur < 7*24*time.Hour:
		d := int(dur.Hours() / 24)
		return fmt.Sprintf("%dd ago", d)
	default:
		w := int(dur.Hours() / (24 * 7))
		return fmt.Sprintf("%dw ago", w)
	}
}

// BuildCategoryChain builds a "Game1 → Game2 → Game3" chain from VOD chapters,
// capping at maxShown categories and maxChars total length.
func BuildCategoryChain(chapters []api.Chapter, fallback string, maxShown, maxChars int) string {
	names := BuildCategoryNames(chapters, fallback, maxShown)
	if len(names) == 0 {
		return "—"
	}
	chain := strings.Join(names, " → ")
	return Truncate(chain, maxChars)
}

// BuildCategoryNames returns the ordered, deduplicated list of category names
// from VOD chapters, capped at maxShown. Returns [fallback] when no chapters
// are found, or nil when fallback is also empty.
func BuildCategoryNames(chapters []api.Chapter, fallback string, maxShown int) []string {
	seen := make(map[string]bool)
	var games []string
	for _, ch := range chapters {
		g := ch.Game
		if g == "" {
			g = ch.Description
		}
		if g == "" || seen[g] {
			continue
		}
		seen[g] = true
		games = append(games, g)
		if len(games) >= maxShown {
			break
		}
	}
	if len(games) == 0 {
		if fallback != "" {
			return []string{fallback}
		}
		return nil
	}
	return games
}

// Truncate cuts a string to at most max runes, appending "…" if truncated.
func Truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max-1]) + "…"
}

// FormatUptime formats a live duration in seconds as "Xh YYm ZZs".
// Used when combining the GQL-fetched baseline UptimeSec with elapsed wall-clock time.
func FormatUptime(totalSec int64) string {
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	return fmt.Sprintf("%dh %02dm %02ds", h, m, s)
}

// ParseUptime parses an ISO-8601 stream start time and returns the live
// duration in seconds and a formatted "Xh YYm ZZs" string.
// Returns (0, "—") when the timestamp cannot be parsed.
func ParseUptime(createdAt string) (int64, string) {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return 0, "—"
	}
	dur := time.Since(t)
	sec := int64(dur.Seconds())
	h := int(dur.Hours())
	m := int(dur.Minutes()) % 60
	s := int(dur.Seconds()) % 60
	return sec, fmt.Sprintf("%dh %02dm %02ds", h, m, s)
}

// StreamEndTime computes the approximate end time of a VOD
// (creation time + duration).
func StreamEndTime(createdAt string, lengthSeconds int) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return time.Time{}, false
	}
	return t.Add(time.Duration(lengthSeconds) * time.Second), true
}
