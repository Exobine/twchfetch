package views

import (
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Advanced search engine — shared by the streamer list and the chat view.
//
// Supported operators (in order of precedence, lowest to highest):
//   |            OR  — any clause matches
//   & or space   AND — implicit by whitespace, explicit by &
//   !            NOT — prefix unary negation
//   (...)        grouping — explicit precedence control
//   "..."        phrase — exact substring with spaces preserved
//   ^            prefix anchor — term at start of field
//   $            suffix anchor — term at end of field
//   field:       field targeting — user:, msg:, badge:
//
// Examples:
//   alice bob            contains "alice" AND "bob"
//   alice|bob            contains "alice" OR "bob"
//   !spam                does NOT contain "spam"
//   (sub|gift) !anon     (sub OR gift) AND NOT anon
//   "gifted a sub"       exact phrase including the spaces
//   ^hello               field starts with "hello"
//   sub$                 field ends with "sub"
//   ^sub$                field equals exactly "sub"
//   user:alice           only match username / display-name
//   msg:hello            only match message text
//   badge:mod            only match badge / event-kind labels
//   badge:mod msg:!spam  moderator whose message doesn't contain "spam"
//   user:(alice|bob)     alice OR bob, username field only
// ---------------------------------------------------------------------------

// Node is the interface for all search AST nodes.
type Node interface{ searchNode() }

type andNode struct{ children []Node }
type orNode  struct{ children []Node }
type notNode struct{ child Node }
type termNode struct {
	text   string // lowercased search text
	field  string // "" = all fields; "user", "msg", or "badge"
	prefix bool   // ^ — match at the start of the field value
	suffix bool   // $ — match at the end of the field value
}

func (andNode) searchNode()  {}
func (orNode) searchNode()   {}
func (notNode) searchNode()  {}
func (termNode) searchNode() {}

// SearchTarget holds the pre-separated, lowercased searchable fields of one
// item.  Callers construct it once per item so field-targeted terms can
// match against the correct subset without re-splitting.
type SearchTarget struct {
	User  string // username and/or display name
	Msg   string // message text or item identifier (e.g. list number)
	Badge string // space-separated badge/event-kind labels (chat only)
	All   string // concatenation of all fields for unqualified searches
}

// newListTarget builds a SearchTarget for a streamer-list entry.
func newListTarget(username string, num int) SearchTarget {
	user := strings.ToLower(username)
	numStr := strconv.Itoa(num)
	return SearchTarget{
		User: user,
		Msg:  numStr,
		All:  user + " " + numStr,
	}
}

// ParseSearchQuery parses q into a search AST.
// Returns nil for blank input; a nil Node matches everything.
func ParseSearchQuery(q string) Node {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	p := &parser{toks: lex(q)}
	return p.parseOr()
}

// MatchNode reports whether target satisfies node.
// Always returns true for a nil node (no filter active).
func MatchNode(node Node, target SearchTarget) bool {
	if node == nil {
		return true
	}
	return evalNode(node, target)
}

// ---------------------------------------------------------------------------
// Evaluator
// ---------------------------------------------------------------------------

func evalNode(node Node, target SearchTarget) bool {
	switch n := node.(type) {
	case andNode:
		for _, child := range n.children {
			if !evalNode(child, target) {
				return false
			}
		}
		return true
	case orNode:
		for _, child := range n.children {
			if evalNode(child, target) {
				return true
			}
		}
		return false
	case notNode:
		return !evalNode(n.child, target)
	case termNode:
		return matchTerm(n, target)
	}
	return false
}

func matchTerm(t termNode, target SearchTarget) bool {
	var haystack string
	switch t.field {
	case "user":
		haystack = target.User
	case "msg":
		haystack = target.Msg
	case "badge":
		haystack = target.Badge
	default:
		haystack = target.All
	}
	switch {
	case t.prefix && t.suffix:
		return haystack == t.text
	case t.prefix:
		return strings.HasPrefix(haystack, t.text)
	case t.suffix:
		return strings.HasSuffix(haystack, t.text)
	default:
		return strings.Contains(haystack, t.text)
	}
}

// ---------------------------------------------------------------------------
// Lexer
// ---------------------------------------------------------------------------

type tokKind int

const (
	tokEOF    tokKind = iota
	tokWord           // unquoted token; may carry leading ^ and/or trailing $ anchors
	tokPhrase         // "..." quoted phrase — exact substring, no anchors
	tokOR             // |
	tokAND            // & (explicit)
	tokNOT            // !
	tokLParen         // (
	tokRParen         // )
	tokWS             // whitespace — implicit AND at the parse level
	tokField          // field: prefix, e.g. "user:", "msg:", "badge:"
)

type token struct {
	kind  tokKind
	text  string // tokWord, tokPhrase
	field string // tokField: field name without the colon
}

func lex(input string) []token {
	var toks []token
	i := 0
	for i < len(input) {
		c := input[i]
		switch {
		case c == '|':
			toks = append(toks, token{kind: tokOR})
			i++
		case c == '&':
			toks = append(toks, token{kind: tokAND})
			i++
		case c == '!':
			toks = append(toks, token{kind: tokNOT})
			i++
		case c == '(':
			toks = append(toks, token{kind: tokLParen})
			i++
		case c == ')':
			toks = append(toks, token{kind: tokRParen})
			i++
		case c == '"':
			// Scan to closing quote or EOF.
			j := i + 1
			for j < len(input) && input[j] != '"' {
				j++
			}
			text := strings.ToLower(input[i+1 : j])
			if j < len(input) {
				j++ // consume closing "
			}
			toks = append(toks, token{kind: tokPhrase, text: text})
			i = j
		case c == ' ' || c == '\t':
			j := i
			for j < len(input) && (input[j] == ' ' || input[j] == '\t') {
				j++
			}
			toks = append(toks, token{kind: tokWS})
			i = j
		default:
			// Unquoted word — scan until a delimiter or ':'.
			// We stop AT ':' so that "field:term" is detected as a field prefix below.
			j := i
			for j < len(input) && !isDelim(input[j]) && input[j] != ':' {
				j++
			}
			word := input[i:j]
			i = j
			// A word immediately followed by ':' is a field prefix.
			if i < len(input) && input[i] == ':' {
				i++ // consume ':'
				toks = append(toks, token{kind: tokField, field: strings.ToLower(word)})
			} else {
				toks = append(toks, token{kind: tokWord, text: strings.ToLower(word)})
			}
		}
	}
	toks = append(toks, token{kind: tokEOF})
	return toks
}

// isDelim reports whether c ends an unquoted word.
// ':' is intentionally excluded so field: patterns are detected in the word scanner.
func isDelim(c byte) bool {
	return c == '|' || c == '&' || c == '!' ||
		c == '(' || c == ')' || c == '"' ||
		c == ' ' || c == '\t'
}

// ---------------------------------------------------------------------------
// Parser — recursive descent
//
//  or_expr  = and_expr  ( "|" and_expr )*
//  and_expr = not_expr  ( ("&" | WS) not_expr )*
//  not_expr = "!" not_expr | atom
//  atom     = "(" or_expr ")" | field: not_expr | phrase | word
// ---------------------------------------------------------------------------

type parser struct {
	toks []token
	pos  int
}

func (p *parser) peek() token {
	if p.pos >= len(p.toks) {
		return token{kind: tokEOF}
	}
	return p.toks[p.pos]
}

func (p *parser) consume() token {
	t := p.peek()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return t
}

func (p *parser) skipWS() {
	for p.peek().kind == tokWS {
		p.consume()
	}
}

func (p *parser) parseOr() Node {
	p.skipWS()
	left := p.parseAnd()
	if left == nil {
		return nil
	}
	for {
		p.skipWS()
		if p.peek().kind != tokOR {
			break
		}
		p.consume() // consume '|'
		p.skipWS()
		right := p.parseAnd()
		if right == nil {
			break
		}
		if o, ok := left.(orNode); ok {
			o.children = append(o.children, right)
			left = o
		} else {
			left = orNode{children: []Node{left, right}}
		}
	}
	return left
}

func (p *parser) parseAnd() Node {
	p.skipWS()
	left := p.parseNot()
	if left == nil {
		return nil
	}
	for {
		explicit := p.peek().kind == tokAND
		implicit := p.peek().kind == tokWS
		if !explicit && !implicit {
			break
		}
		// Save position so we can backtrack if no valid right operand follows.
		savedPos := p.pos
		if explicit {
			p.consume() // consume '&'
		}
		p.skipWS()
		// Implicit AND only when the next token is a valid atom start.
		// If it's '|', ')', or EOF the whitespace belongs to the outer parser.
		next := p.peek().kind
		if next == tokOR || next == tokRParen || next == tokEOF || next == tokAND {
			p.pos = savedPos
			break
		}
		right := p.parseNot()
		if right == nil {
			p.pos = savedPos
			break
		}
		if a, ok := left.(andNode); ok {
			a.children = append(a.children, right)
			left = a
		} else {
			left = andNode{children: []Node{left, right}}
		}
	}
	return left
}

func (p *parser) parseNot() Node {
	p.skipWS()
	if p.peek().kind == tokNOT {
		p.consume()
		child := p.parseNot()
		if child == nil {
			return nil
		}
		return notNode{child: child}
	}
	return p.parseAtom()
}

func (p *parser) parseAtom() Node {
	p.skipWS()
	t := p.peek()
	switch t.kind {
	case tokLParen:
		p.consume()
		node := p.parseOr()
		p.skipWS()
		if p.peek().kind == tokRParen {
			p.consume()
		}
		return node

	case tokField:
		p.consume()
		field := t.field
		// Unknown field names fall back to unqualified (all-field) search.
		switch field {
		case "user", "msg", "badge":
		default:
			field = ""
		}
		inner := p.parseNot()
		if inner == nil {
			return nil
		}
		return injectField(inner, field)

	case tokPhrase:
		p.consume()
		return termNode{text: t.text}

	case tokWord:
		p.consume()
		text := t.text
		prefix := strings.HasPrefix(text, "^")
		suffix := strings.HasSuffix(text, "$")
		if prefix {
			text = text[1:]
		}
		if suffix {
			text = text[:len(text)-1]
		}
		if text == "" {
			return nil
		}
		return termNode{text: text, prefix: prefix, suffix: suffix}
	}
	return nil
}

// injectField propagates a field qualifier down through an AST subtree,
// setting it on every termNode that has no field yet.  This allows
// "user:(alice|bob)" to correctly qualify both alice and bob as user-field terms.
func injectField(n Node, field string) Node {
	switch node := n.(type) {
	case termNode:
		if node.field == "" {
			node.field = field
		}
		return node
	case andNode:
		for i, child := range node.children {
			node.children[i] = injectField(child, field)
		}
		return node
	case orNode:
		for i, child := range node.children {
			node.children[i] = injectField(child, field)
		}
		return node
	case notNode:
		node.child = injectField(node.child, field)
		return node
	}
	return n
}
