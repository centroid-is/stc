package lexer

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lexer scans IEC 61131-3 Structured Text source into a token stream.
// It preserves whitespace and comments as trivia tokens for CST fidelity.
type Lexer struct {
	src    string
	file   string
	pos    int // current byte offset
	line   int // current line (1-based)
	col    int // current column (1-based)
	tokens []Token
}

// New creates a new Lexer for the given source file.
func New(filename, source string) *Lexer {
	return &Lexer{
		src:  source,
		file: filename,
		pos:  0,
		line: 1,
		col:  1,
	}
}

// Tokenize scans the entire source and returns the complete token list.
func (l *Lexer) Tokenize() []Token {
	for {
		tok := l.scan()
		l.tokens = append(l.tokens, tok)
		if tok.Kind == EOF {
			break
		}
	}
	return l.tokens
}

// Tokenize is a convenience function that creates a lexer and returns all tokens.
func Tokenize(filename, source string) []Token {
	return New(filename, source).Tokenize()
}

// currentPos returns the current position as a Pos.
func (l *Lexer) currentPos() Pos {
	return Pos{
		File:   l.file,
		Line:   l.line,
		Col:    l.col,
		Offset: l.pos,
	}
}

// atEnd returns true if the lexer has reached the end of input.
func (l *Lexer) atEnd() bool {
	return l.pos >= len(l.src)
}

// peek returns the current byte without consuming it.
// Returns 0 if at end.
func (l *Lexer) peek() byte {
	if l.atEnd() {
		return 0
	}
	return l.src[l.pos]
}

// peekAt returns the byte at offset ahead from current position.
// Returns 0 if out of bounds.
func (l *Lexer) peekAt(offset int) byte {
	idx := l.pos + offset
	if idx >= len(l.src) {
		return 0
	}
	return l.src[idx]
}

// advance consumes and returns the current byte, updating position tracking.
func (l *Lexer) advance() byte {
	if l.atEnd() {
		return 0
	}
	ch := l.src[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

// makeToken creates a token from start position to current position.
func (l *Lexer) makeToken(kind TokenKind, start Pos) Token {
	return Token{
		Kind:   kind,
		Text:   l.src[start.Offset:l.pos],
		Pos:    start,
		EndPos: l.currentPos(),
	}
}

// scan reads the next token from the source.
func (l *Lexer) scan() Token {
	if l.atEnd() {
		pos := l.currentPos()
		return Token{Kind: EOF, Text: "", Pos: pos, EndPos: pos}
	}

	start := l.currentPos()
	ch := l.peek()

	// Whitespace
	if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
		return l.scanWhitespace(start)
	}

	// Line comment: //
	if ch == '/' && l.peekAt(1) == '/' {
		return l.scanLineComment(start)
	}

	// Block comment: (*
	if ch == '(' && l.peekAt(1) == '*' {
		return l.scanBlockComment(start)
	}

	// Pragma: { ... }
	if ch == '{' {
		return l.scanPragma(start)
	}

	// String literals
	if ch == '\'' {
		return l.scanString(start, '\'', StringLiteral)
	}
	if ch == '"' {
		return l.scanString(start, '"', WStringLiteral)
	}

	// Numbers
	if isDigit(ch) {
		return l.scanNumber(start)
	}

	// Identifiers and keywords (may produce typed/time/date literals)
	if isIdentStart(ch) {
		return l.scanIdentOrKeyword(start)
	}

	// Operators and punctuation
	return l.scanOperator(start)
}

// scanWhitespace consumes a run of whitespace characters.
func (l *Lexer) scanWhitespace(start Pos) Token {
	for !l.atEnd() {
		ch := l.peek()
		if ch != ' ' && ch != '\t' && ch != '\r' && ch != '\n' {
			break
		}
		l.advance()
	}
	return l.makeToken(Whitespace, start)
}

// scanLineComment consumes from // to end of line.
func (l *Lexer) scanLineComment(start Pos) Token {
	// Consume the //
	l.advance()
	l.advance()
	for !l.atEnd() && l.peek() != '\n' {
		l.advance()
	}
	return l.makeToken(LineComment, start)
}

// scanBlockComment consumes (* ... *) with nesting support.
func (l *Lexer) scanBlockComment(start Pos) Token {
	// Consume the opening (*
	l.advance()
	l.advance()
	depth := 1
	for !l.atEnd() && depth > 0 {
		if l.peek() == '(' && l.peekAt(1) == '*' {
			l.advance()
			l.advance()
			depth++
		} else if l.peek() == '*' && l.peekAt(1) == ')' {
			l.advance()
			l.advance()
			depth--
		} else {
			l.advance()
		}
	}
	return l.makeToken(BlockComment, start)
}

// scanPragma consumes { ... } pragma content.
func (l *Lexer) scanPragma(start Pos) Token {
	l.advance() // consume {
	for !l.atEnd() && l.peek() != '}' {
		l.advance()
	}
	if !l.atEnd() {
		l.advance() // consume }
	}
	return l.makeToken(Pragma, start)
}

// scanString consumes a string literal delimited by the given quote character.
// Handles doubled quotes as escape ('' inside single-quoted strings).
func (l *Lexer) scanString(start Pos, quote byte, kind TokenKind) Token {
	l.advance() // consume opening quote
	for !l.atEnd() {
		ch := l.peek()
		if ch == quote {
			l.advance()
			// Check for doubled quote (escape)
			if !l.atEnd() && l.peek() == quote {
				l.advance()
				continue
			}
			break
		}
		l.advance()
	}
	return l.makeToken(kind, start)
}

// scanNumber scans integer, real, or multi-base integer literals.
// Handles forms: 42, 3.14, 1.0E-5, 16#FF, 2#1010.
func (l *Lexer) scanNumber(start Pos) Token {
	// Scan digits
	l.scanDigits()

	// Check for base prefix: digits followed by #
	if !l.atEnd() && l.peek() == '#' {
		// This is a multi-base integer: 16#FF, 2#1010, 8#77
		l.advance() // consume #
		l.scanBaseDigits()
		return l.makeToken(IntLiteral, start)
	}

	// Check for real: digits followed by .
	// But not .. (range operator)
	if !l.atEnd() && l.peek() == '.' && l.peekAt(1) != '.' {
		l.advance() // consume .
		l.scanDigits()
		// Check for exponent
		if !l.atEnd() && (l.peek() == 'E' || l.peek() == 'e') {
			l.advance()
			if !l.atEnd() && (l.peek() == '+' || l.peek() == '-') {
				l.advance()
			}
			l.scanDigits()
		}
		return l.makeToken(RealLiteral, start)
	}

	// Check for exponent without decimal point: 1E5
	if !l.atEnd() && (l.peek() == 'E' || l.peek() == 'e') {
		next := l.peekAt(1)
		if isDigit(next) || next == '+' || next == '-' {
			l.advance()
			if !l.atEnd() && (l.peek() == '+' || l.peek() == '-') {
				l.advance()
			}
			l.scanDigits()
			return l.makeToken(RealLiteral, start)
		}
	}

	return l.makeToken(IntLiteral, start)
}

// scanDigits consumes a run of decimal digits and underscores.
func (l *Lexer) scanDigits() {
	for !l.atEnd() && (isDigit(l.peek()) || l.peek() == '_') {
		l.advance()
	}
}

// scanBaseDigits consumes hex/octal/binary digits and underscores after a base prefix.
func (l *Lexer) scanBaseDigits() {
	for !l.atEnd() && (isHexDigit(l.peek()) || l.peek() == '_') {
		l.advance()
	}
}

// timePrefixes maps uppercased time/date prefix keywords to their literal token kinds.
var timePrefixes = map[string]TokenKind{
	"T":             TimeLiteral,
	"TIME":          TimeLiteral,
	"D":             DateLiteral,
	"DATE":          DateLiteral,
	"DT":            DateTimeLiteral,
	"DATE_AND_TIME": DateTimeLiteral,
	"TOD":           TodLiteral,
	"TIME_OF_DAY":   TodLiteral,
}

// typedLiteralPrefixes contains type names that can appear before # in typed literals.
var typedLiteralPrefixes = map[string]bool{
	"BOOL":  true,
	"BYTE":  true,
	"WORD":  true,
	"DWORD": true,
	"LWORD": true,
	"SINT":  true,
	"INT":   true,
	"DINT":  true,
	"LINT":  true,
	"USINT": true,
	"UINT":  true,
	"UDINT": true,
	"ULINT": true,
	"REAL":  true,
	"LREAL": true,
}

// scanIdentOrKeyword scans an identifier, keyword, or literal with a type/time prefix.
func (l *Lexer) scanIdentOrKeyword(start Pos) Token {
	// Consume identifier characters
	for !l.atEnd() && isIdentPart(l.peek()) {
		l.advance()
	}

	text := l.src[start.Offset:l.pos]
	upper := strings.ToUpper(text)

	// Check if followed by # — could be a time/date literal or typed literal
	if !l.atEnd() && l.peek() == '#' {
		// Time/date literal prefix?
		if kind, ok := timePrefixes[upper]; ok {
			l.advance() // consume #
			l.scanLiteralValue()
			return l.makeToken(kind, start)
		}
		// Typed literal prefix?
		if typedLiteralPrefixes[upper] {
			l.advance() // consume #
			l.scanLiteralValue()
			return l.makeToken(TypedLiteral, start)
		}
	}

	// Regular keyword or identifier
	if kind, ok := LookupKeyword(text); ok {
		return l.makeToken(kind, start)
	}
	return l.makeToken(Ident, start)
}

// scanLiteralValue consumes the value portion after a # in typed/time/date literals.
// Handles: digits, letters, underscores, dots, colons, plus, minus signs.
func (l *Lexer) scanLiteralValue() {
	for !l.atEnd() {
		ch := l.peek()
		if isIdentPart(ch) || isDigit(ch) || ch == '.' || ch == ':' || ch == '-' || ch == '+' {
			l.advance()
		} else {
			break
		}
	}
}

// scanOperator scans single and multi-character operators and punctuation.
func (l *Lexer) scanOperator(start Pos) Token {
	ch := l.advance()

	switch ch {
	case ':':
		if !l.atEnd() && l.peek() == '=' {
			l.advance()
			return l.makeToken(Assign, start)
		}
		return l.makeToken(Colon, start)

	case '*':
		if !l.atEnd() && l.peek() == '*' {
			l.advance()
			return l.makeToken(Power, start)
		}
		return l.makeToken(Star, start)

	case '<':
		if !l.atEnd() && l.peek() == '>' {
			l.advance()
			return l.makeToken(NotEq, start)
		}
		if !l.atEnd() && l.peek() == '=' {
			l.advance()
			return l.makeToken(LessEq, start)
		}
		return l.makeToken(Less, start)

	case '>':
		if !l.atEnd() && l.peek() == '=' {
			l.advance()
			return l.makeToken(GreaterEq, start)
		}
		return l.makeToken(Greater, start)

	case '=':
		if !l.atEnd() && l.peek() == '>' {
			l.advance()
			return l.makeToken(Arrow, start)
		}
		return l.makeToken(Eq, start)

	case '.':
		if !l.atEnd() && l.peek() == '.' {
			l.advance()
			return l.makeToken(DotDot, start)
		}
		return l.makeToken(Dot, start)

	case '+':
		return l.makeToken(Plus, start)
	case '-':
		return l.makeToken(Minus, start)
	case '/':
		return l.makeToken(Slash, start)
	case '(':
		return l.makeToken(LParen, start)
	case ')':
		return l.makeToken(RParen, start)
	case '[':
		return l.makeToken(LBracket, start)
	case ']':
		return l.makeToken(RBracket, start)
	case ',':
		return l.makeToken(Comma, start)
	case ';':
		return l.makeToken(Semicolon, start)
	case '^':
		return l.makeToken(Caret, start)
	case '#':
		return l.makeToken(Hash, start)
	case '&':
		return l.makeToken(Ampersand, start)
	}

	return l.makeToken(Illegal, start)
}

// Helper character classification functions.

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isIdentStart(ch byte) bool {
	if ch < utf8.RuneSelf {
		return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
	}
	r, _ := utf8.DecodeRune([]byte{ch})
	return unicode.IsLetter(r)
}

func isIdentPart(ch byte) bool {
	if ch < utf8.RuneSelf {
		return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
	}
	r, _ := utf8.DecodeRune([]byte{ch})
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
