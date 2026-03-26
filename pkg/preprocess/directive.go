package preprocess

import (
	"strings"
)

// directiveKind identifies the type of preprocessor directive.
type directiveKind int

const (
	dirIF     directiveKind = iota // {IF condition}
	dirELSIF                       // {ELSIF condition}
	dirELSE                        // {ELSE}
	dirENDIF                       // {END_IF}
	dirDEFINE                      // {DEFINE name}
	dirERROR                       // {ERROR "message"}
)

// directive represents a parsed preprocessor directive.
type directive struct {
	kind      directiveKind
	condition string // for IF, ELSIF
	name      string // for DEFINE
	message   string // for ERROR
}

// parseDirective parses the text of a pragma (including braces) and returns
// a directive if it is a preprocessor directive, or nil if it is a regular
// pragma (e.g., {attribute '...'}).
func parseDirective(text string) *directive {
	// Strip outer braces
	text = strings.TrimSpace(text)
	if len(text) < 2 || text[0] != '{' || text[len(text)-1] != '}' {
		return nil
	}
	inner := strings.TrimSpace(text[1 : len(text)-1])
	if inner == "" {
		return nil
	}

	// Extract keyword (first word)
	upperInner := strings.ToUpper(inner)

	// Match keywords in order of specificity
	switch {
	case strings.HasPrefix(upperInner, "END_IF"):
		return &directive{kind: dirENDIF}

	case strings.HasPrefix(upperInner, "ELSIF"):
		cond := strings.TrimSpace(inner[len("ELSIF"):])
		return &directive{kind: dirELSIF, condition: cond}

	case strings.HasPrefix(upperInner, "ELSE"):
		// Make sure it's exactly ELSE and not ELSIF (already handled above)
		rest := inner[len("ELSE"):]
		if rest != "" && !isWhitespaceOrEmpty(rest) {
			return nil
		}
		return &directive{kind: dirELSE}

	case strings.HasPrefix(upperInner, "IF ") || upperInner == "IF":
		cond := strings.TrimSpace(inner[len("IF"):])
		return &directive{kind: dirIF, condition: cond}

	case strings.HasPrefix(upperInner, "DEFINE"):
		name := strings.TrimSpace(inner[len("DEFINE"):])
		return &directive{kind: dirDEFINE, name: name}

	case strings.HasPrefix(upperInner, "ERROR"):
		msg := strings.TrimSpace(inner[len("ERROR"):])
		// Strip surrounding quotes if present
		if len(msg) >= 2 && msg[0] == '"' && msg[len(msg)-1] == '"' {
			msg = msg[1 : len(msg)-1]
		}
		return &directive{kind: dirERROR, message: msg}

	default:
		// Not a preprocessor directive (e.g., {attribute '...'}, {pragma ...})
		return nil
	}
}

func isWhitespaceOrEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// evalCondition evaluates a preprocessor condition expression against
// a set of defined symbols. Supports:
//   - defined(NAME)
//   - NOT defined(NAME)
//   - expr AND expr
//   - expr OR expr
//
// Precedence: NOT > AND > OR
func evalCondition(cond string, defines map[string]bool) bool {
	p := &condParser{input: strings.TrimSpace(cond), pos: 0, defines: defines}
	return p.parseOr()
}

// condParser is a simple recursive descent parser for condition expressions.
type condParser struct {
	input   string
	pos     int
	defines map[string]bool
}

func (p *condParser) skipSpaces() {
	for p.pos < len(p.input) && p.input[p.pos] == ' ' {
		p.pos++
	}
}

func (p *condParser) remaining() string {
	if p.pos >= len(p.input) {
		return ""
	}
	return p.input[p.pos:]
}

// parseOr handles: expr (OR expr)*
func (p *condParser) parseOr() bool {
	left := p.parseAnd()
	for {
		p.skipSpaces()
		rem := strings.ToUpper(p.remaining())
		if strings.HasPrefix(rem, "OR ") || strings.HasPrefix(rem, "OR\t") {
			p.pos += 2
			p.skipSpaces()
			right := p.parseAnd()
			left = left || right
		} else {
			break
		}
	}
	return left
}

// parseAnd handles: expr (AND expr)*
func (p *condParser) parseAnd() bool {
	left := p.parseNot()
	for {
		p.skipSpaces()
		rem := strings.ToUpper(p.remaining())
		if strings.HasPrefix(rem, "AND ") || strings.HasPrefix(rem, "AND\t") {
			p.pos += 3
			p.skipSpaces()
			right := p.parseNot()
			left = left && right
		} else {
			break
		}
	}
	return left
}

// parseNot handles: NOT? primary
func (p *condParser) parseNot() bool {
	p.skipSpaces()
	rem := strings.ToUpper(p.remaining())
	if strings.HasPrefix(rem, "NOT ") || strings.HasPrefix(rem, "NOT\t") {
		p.pos += 3
		p.skipSpaces()
		return !p.parsePrimary()
	}
	return p.parsePrimary()
}

// parsePrimary handles: defined(NAME) | (expr)
func (p *condParser) parsePrimary() bool {
	p.skipSpaces()
	rem := strings.ToUpper(p.remaining())

	if strings.HasPrefix(rem, "DEFINED(") {
		// Advance past "defined("
		p.pos += len("defined(")
		// Read until closing paren
		start := p.pos
		for p.pos < len(p.input) && p.input[p.pos] != ')' {
			p.pos++
		}
		name := strings.TrimSpace(p.input[start:p.pos])
		if p.pos < len(p.input) {
			p.pos++ // skip ')'
		}
		return p.defines[name]
	}

	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		p.pos++ // skip '('
		val := p.parseOr()
		p.skipSpaces()
		if p.pos < len(p.input) && p.input[p.pos] == ')' {
			p.pos++ // skip ')'
		}
		return val
	}

	// Unknown token — treat as false
	return false
}
