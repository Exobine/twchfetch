package views

import "charm.land/glamour/v2/ansi"

// twitchGlamourStyle returns a glamour StyleConfig tuned to the app's
// Twitch-purple theme. Mirrors the same palette used throughout styles.go.
//
// Body text colour hierarchy (brightest → dimmest):
//
//	#FFFFFF  pure white   — bold / strong (pops against all backgrounds)
//	#EFEFF1  near-white   — Document default / base
//	#D8D6EA  lavender-grey — paragraph body text
//	#ADADB8  muted         — list content, H4, item bullets
//	#6B6B7B  dim           — blockquote, HR, strikethrough, H5/H6
//
// Accent colours:
//
//	#9147FF  Purple       — H1, links
//	#BF94FF  PurpleLight  — H2, emph/italic, enum numbers, inline code
//	#C8B8E8               — H3 soft purple
func twitchGlamourStyle() ansi.StyleConfig {
	b := boolP
	s := strP
	u := uintP

	return ansi.StyleConfig{
		// ── Document ────────────────────────────────────────────────────────
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:       s("#EFEFF1"),
				BlockSuffix: "\n",
			},
			Margin: u(2),
		},

		// ── Headings ────────────────────────────────────────────────────────
		// H1 — full-width banner, uppercase, primary purple
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:        b(true),
				Upper:       b(true),
				Color:       s("#9147FF"),
				BlockSuffix: "\n",
			},
		},
		// H2 — section heading with ▌ accent bar (mirrors selected-row glyph)
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:        b(true),
				Color:       s("#BF94FF"),
				Prefix:      "▌ ",
				BlockSuffix: "\n",
			},
		},
		// H3 — sub-section, softer purple + indent dot
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:   b(true),
				Color:  s("#C8B8E8"),
				Prefix: "  · ",
			},
		},
		// H4 — minor heading, muted
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:   b(true),
				Color:  s("#ADADB8"),
				Prefix: "    ",
			},
		},
		// H5 / H6 — dim, barely differentiated from body
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Faint:  b(true),
				Color:  s("#6B6B7B"),
				Prefix: "    ",
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Faint:  b(true),
				Color:  s("#6B6B7B"),
				Prefix: "      ",
			},
		},

		// ── Inline text ─────────────────────────────────────────────────────
		// Text has no Color — it inherits from whatever block contains it
		// (heading purple, paragraph lavender-grey, list muted, etc.).
		Text: ansi.StylePrimitive{},

		// Strong is pure white: always pops against paragraph (#D8D6EA) and
		// list (#ADADB8) backgrounds.  Inside headings it reads as bright
		// text on the heading colour — acceptable since the help content
		// doesn't use bold inside headings.
		Strong: ansi.StylePrimitive{
			Bold:  b(true),
			Color: s("#FFFFFF"),
		},
		Emph: ansi.StylePrimitive{
			Italic: b(true),
			Color:  s("#BF94FF"),
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: b(true),
			Color:      s("#6B6B7B"),
		},

		// ── Code ────────────────────────────────────────────────────────────
		// Inline code — purple-light text on dark search-bar background
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           s("#BF94FF"),
				BackgroundColor: s("#1F1629"),
			},
		},
		// Fenced code block — dark canvas with chroma syntax colouring
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           s("#ADADB8"),
					BackgroundColor: s("#18181B"),
				},
				Margin: u(2),
			},
			Chroma: &ansi.Chroma{
				Text:                ansi.StylePrimitive{Color: s("#EFEFF1")},
				Error:               ansi.StylePrimitive{Color: s("#E91916")},
				Comment:             ansi.StylePrimitive{Color: s("#6B6B7B"), Italic: b(true)},
				CommentPreproc:      ansi.StylePrimitive{Color: s("#6B6B7B")},
				Keyword:             ansi.StylePrimitive{Color: s("#9147FF"), Bold: b(true)},
				KeywordReserved:     ansi.StylePrimitive{Color: s("#9147FF"), Bold: b(true)},
				KeywordNamespace:    ansi.StylePrimitive{Color: s("#BF94FF"), Bold: b(true)},
				KeywordType:         ansi.StylePrimitive{Color: s("#1FB8F5")},
				Operator:            ansi.StylePrimitive{Color: s("#ADADB8")},
				Punctuation:         ansi.StylePrimitive{Color: s("#6B6B7B")},
				Name:                ansi.StylePrimitive{Color: s("#EFEFF1")},
				NameBuiltin:         ansi.StylePrimitive{Color: s("#1FB8F5")},
				NameFunction:        ansi.StylePrimitive{Color: s("#BF94FF"), Bold: b(true)},
				NameClass:           ansi.StylePrimitive{Color: s("#BF94FF"), Bold: b(true)},
				NameConstant:        ansi.StylePrimitive{Color: s("#FFCD00")},
				NameDecorator:       ansi.StylePrimitive{Color: s("#BF94FF")},
				NameException:       ansi.StylePrimitive{Color: s("#E91916")},
				NameOther:           ansi.StylePrimitive{Color: s("#C8B8E8")},
				NameTag:             ansi.StylePrimitive{Color: s("#9147FF")},
				NameAttribute:       ansi.StylePrimitive{Color: s("#BF94FF")},
				Literal:             ansi.StylePrimitive{Color: s("#00AD03")},
				LiteralNumber:       ansi.StylePrimitive{Color: s("#FFCD00")},
				LiteralString:       ansi.StylePrimitive{Color: s("#00AD03")},
				LiteralStringEscape: ansi.StylePrimitive{Color: s("#BF94FF"), Bold: b(true)},
				GenericDeleted:      ansi.StylePrimitive{Color: s("#E91916")},
				GenericInserted:     ansi.StylePrimitive{Color: s("#00AD03")},
				GenericStrong:       ansi.StylePrimitive{Bold: b(true)},
				GenericEmph:         ansi.StylePrimitive{Italic: b(true)},
				GenericSubheading:   ansi.StylePrimitive{Color: s("#BF94FF")},
				Background:          ansi.StylePrimitive{BackgroundColor: s("#18181B")},
			},
		},

		// ── Block elements ───────────────────────────────────────────────────
		// Paragraph: lavender-grey body text — one step dimmer than Document
		// so bold (#FFFFFF) pops against it.
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: s("#D8D6EA")},
		},

		// BlockQuote: dim + italic with │ left bar.
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  s("#6B6B7B"),
				Italic: b(true),
			},
			Indent:      u(1),
			IndentToken: strP("│ "),
		},

		// ── Lists ────────────────────────────────────────────────────────────
		// List block colour cascades to all child text — muted tier.
		// Bold/emph inside list items still override via Strong/Emph colours.
		List: ansi.StyleList{
			StyleBlock:  ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: s("#ADADB8")}},
			LevelIndent: 2,
		},

		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       s("#ADADB8"),
		},
		// Enumeration: number is rendered then BlockPrefix appended → "1. item"
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       s("#BF94FF"),
		},

		// ── Horizontal rule ──────────────────────────────────────────────────
		HorizontalRule: ansi.StylePrimitive{
			Color:  s("#3A3A3D"),
			Format: "\n─────────────────────────────────\n",
		},

		// ── Links ────────────────────────────────────────────────────────────
		Link: ansi.StylePrimitive{
			Color:     s("#9147FF"),
			Underline: b(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: s("#BF94FF"),
			Bold:  b(true),
		},
		Image: ansi.StylePrimitive{
			Color:     s("#BF94FF"),
			Underline: b(true),
		},
		ImageText: ansi.StylePrimitive{
			Color: s("#ADADB8"),
		},

		// ── Table ────────────────────────────────────────────────────────────
		Table: ansi.StyleTable{
			StyleBlock:      ansi.StyleBlock{},
			CenterSeparator: strP("┼"),
			ColumnSeparator: strP("│"),
			RowSeparator:    strP("─"),
		},

		// ── Definitions ──────────────────────────────────────────────────────
		DefinitionTerm: ansi.StylePrimitive{
			Bold:  b(true),
			Color: s("#BF94FF"),
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "  ",
			Color:       s("#ADADB8"),
		},
	}
}

// boolP / strP / uintP are convenience pointer helpers used exclusively by
// twitchGlamourStyle to avoid cluttering the style literal with &-casts.
func boolP(b bool) *bool   { return &b }
func strP(s string) *string { return &s }
func uintP(u uint) *uint   { return &u }
