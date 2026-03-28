package views

import "strings"

// Screen is the global layout descriptor for a view.  Every view in the
// application references one of the three named layout instances defined at
// the bottom of this file.  When adding a new view, pick the layout that
// matches its visual contract — do not invent a custom composition pattern.
//
// # Zone hierarchy (top → bottom)
//
//	[Title]      — one line, optional; only present when ShowTitle=true
//	[SubHeader]  — one or more lines of contextual info (always present)
//	[Body]       — the content area; caller sizes this to the available height
//	[Footer]     — one or more hint lines (always present)
//
// # Layout types
//
//	ScreenMinimal  — no global app header, no view title; sub-header pinned
//	                 to the top, footer pinned to the bottom, body fills
//	                 everything in between.  Used by: Chat.
//
//	ScreenStandard — global app header shown by the outer model; no view
//	                 title; sub-header below the global header; body sized by
//	                 the caller to fill the remaining space; footer pinned to
//	                 the terminal bottom.  Used by: List, Cards.
//
//	ScreenAdaptive — global app header shown by the outer model; no view
//	                 title; sub-header below the global header; body is
//	                 natural content height (does NOT fill the terminal);
//	                 footer floats immediately below the body.
//	                 Used by: Details, VODs.
//
// Settings is a special case that adds its own view-level title line and
// always pins the footer — it uses ScreenSettings.
type Screen struct {
	// ShowTitle controls whether the view renders its own title zone (one
	// line) above the sub-header.  false for Minimal, Standard, Adaptive;
	// true for Settings which renders "Settings" as its own title.
	ShowTitle bool

	// HasGlobalHeader documents whether the outer model renders the global
	// application header (TwitchFetch ASCII art) above this view.  This
	// field does not affect rendering — it is metadata for developers reading
	// the layout declaration.  false = Minimal (chat); true = all others.
	HasGlobalHeader bool

	// PinFooter documents whether the caller is expected to size the body
	// so that the footer lands at the terminal bottom on every frame.
	// true  = Minimal, Standard, Settings (body pre-sized by caller).
	// false = Adaptive (body is natural content height, footer floats).
	// This field does not affect Render() — use RenderFull() when you need
	// the Screen itself to enforce the pin.
	PinFooter bool
}

// TitleLines returns the number of lines consumed by the title zone (0 or 1).
func (s Screen) TitleLines() int {
	if s.ShowTitle {
		return 1
	}
	return 0
}

// ChromeLines returns the total number of fixed-chrome lines:
// title(0 or 1) + subHeaderLines + footerLines.
// Use this to compute how many lines are left for the body viewport.
func (s Screen) ChromeLines(subHeaderLines, footerLines int) int {
	return s.TitleLines() + subHeaderLines + footerLines
}

// BodyHeight returns the number of lines available for the body given the
// total terminal height, the rendered sub-header line count, and the footer
// line count.  The return value is always at least 1.
func (s Screen) BodyHeight(totalH, subHeaderLines, footerLines int) int {
	h := totalH - s.ChromeLines(subHeaderLines, footerLines)
	if h < 1 {
		h = 1
	}
	return h
}

// Render assembles the four zones into a single ready-to-display string.
// The title zone is omitted when ShowTitle is false.  All other zones are
// always joined — empty strings still contribute their newline separator,
// keeping vertical positions stable across frames.
//
// Use this for all current layouts.  The caller is responsible for sizing
// the body string correctly before calling Render.
func (s Screen) Render(title, subHeader, body, footer string) string {
	var zones []string
	if s.ShowTitle && title != "" {
		zones = append(zones, title)
	}
	zones = append(zones, subHeader, body, footer)
	return strings.Join(zones, "\n")
}

// RenderFull is identical to Render but pads the body with blank lines so
// the footer is always exactly at line totalH from the top of the view.
// Use this when PinFooter=true and the body content might be shorter than
// the available height (e.g. an empty-state message in a Standard view).
// When totalH <= 0 or the body already fills the space, the result is
// identical to Render.
func (s Screen) RenderFull(title, subHeader, body, footer string, totalH int) string {
	if totalH > 0 {
		subHeaderLines := strings.Count(subHeader, "\n") + 1
		footerLines := strings.Count(footer, "\n") + 1
		bodyTarget := s.BodyHeight(totalH, subHeaderLines, footerLines)
		bodyActual := strings.Count(body, "\n") + 1
		if bodyActual < bodyTarget {
			body += strings.Repeat("\n", bodyTarget-bodyActual)
		}
	}
	return s.Render(title, subHeader, body, footer)
}

// ---------------------------------------------------------------------------
// Global named layout instances — pick one when building a new view.
// All views in the application use exactly one of these four layouts.
// ---------------------------------------------------------------------------

var (
	// ScreenMinimal is for full-screen views that stand alone without the
	// global application header (currently: Chat only).
	//
	//   HasGlobalHeader = false  — model.go does NOT prepend the app title
	//   ShowTitle       = false  — no view-level title line
	//   PinFooter       = true   — body (viewport) pre-sized to pin footer
	//
	// Zone shape:
	//   [SubHeader]          ← rule + channel info + rule
	//   [Body]               ← pre-sized viewport fills all remaining lines
	//   [Footer]             ← rule + input bar / help bar
	ScreenMinimal = Screen{ShowTitle: false, HasGlobalHeader: false, PinFooter: true}

	// ScreenStandard is for full-screen views rendered below the global
	// application header where the footer is always pinned to the terminal
	// bottom (currently: List, Cards).
	//
	//   HasGlobalHeader = true   — model.go prepends the global app title
	//   ShowTitle       = false  — no view-level title line
	//   PinFooter       = true   — body sized by caller (row count) to pin footer
	//
	// Zone shape:
	//   [SubHeader]          ← filter tabs + [search bar] + rule
	//   [Body]               ← [↑ more] + header row + data rows + [↓ more]
	//   [Footer]             ← rule + help bar
	ScreenStandard = Screen{ShowTitle: false, HasGlobalHeader: true, PinFooter: true}

	// ScreenAdaptive is for views rendered below the global application
	// header where the footer floats immediately below the body content
	// rather than being pinned to the terminal bottom (currently: Details,
	// VODs).  Use when the content height is variable and typically shorter
	// than the full available space.
	//
	//   HasGlobalHeader = true   — model.go prepends the global app title
	//   ShowTitle       = false  — no view-level title line
	//   PinFooter       = false  — footer follows content, not pinned
	//
	// Zone shape:
	//   [SubHeader]          ← rule + streamer info + rule
	//   [Body]               ← info rows / VOD rows (natural height)
	//   [Footer]             ← rule + help bar  (appears below last content line)
	ScreenAdaptive = Screen{ShowTitle: false, HasGlobalHeader: true, PinFooter: false}

	// ScreenSettings is for the Settings view which is unique in rendering
	// its own view-level title line and keeping the footer pinned.
	//
	//   HasGlobalHeader = true   — model.go prepends the global app title
	//   ShowTitle       = true   — "Settings" rendered as the first line
	//   PinFooter       = true   — body (field blocks) sized to pin footer
	//
	// Zone shape:
	//   [Title]              ← "Settings"
	//   [SubHeader]          ← tabs + rule + above-indicator
	//   [Body]               ← field blocks (each exactly 3 lines)
	//   [Footer]             ← rule + help bar
	ScreenSettings = Screen{ShowTitle: true, HasGlobalHeader: true, PinFooter: true}
)

// ---------------------------------------------------------------------------
// Standard chrome line counts for the app's common zone shapes.
// Reference these when computing body height in layout.go or view functions.
// ---------------------------------------------------------------------------

const (
	// SubHeaderLines is the number of lines in the standard sub-header block
	// shared by most views:  rule(1) + info-line(1) + rule(1) = 3.
	SubHeaderLines = 3

	// FooterLines is the number of lines in the standard footer block:
	// rule(1) + help-bar(1) = 2.
	FooterLines = 2

	// SettingsSubHeaderLines is the sub-header line count for the Settings view:
	// tabs(1) + rule(1) + above-indicator(1) = 3.
	SettingsSubHeaderLines = 3

	// SettingsFooterLines is the footer line count for the Settings field view:
	// rule(1) + help-bar(1) = 2.
	SettingsFooterLines = 2

	// SettingsHelpFooterLines is the footer line count for the Settings Help tab:
	// below-indicator(1) + rule(1) + help-bar(1) = 3.
	SettingsHelpFooterLines = 3
)
