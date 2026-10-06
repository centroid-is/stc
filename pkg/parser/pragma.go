package parser

import (
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/lexer"
)

// collectPragmas consumes consecutive Pragma tokens at the current position.
// {attribute ...} pragmas become ast.Attribute nodes; every other pragma
// ({warning ...}, {region}, ...) is kept verbatim as an ast.PragmaNode. Both
// slices are in source order and nil when no pragma is present.
func (p *Parser) collectPragmas() ([]*ast.Attribute, []*ast.PragmaNode) {
	var attrs []*ast.Attribute
	var pragmas []*ast.PragmaNode
	for p.at(lexer.Pragma) {
		tok := p.advance()
		span := ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos))
		if name, value, hasValue, dq, ok := parseAttributeText(tok.Text); ok {
			attrs = append(attrs, &ast.Attribute{
				NodeBase:     ast.NodeBase{NodeKind: ast.KindAttribute, NodeSpan: span},
				Name:         name,
				Value:        value,
				HasValue:     hasValue,
				DoubleQuoted: dq,
			})
			continue
		}
		pragmas = append(pragmas, &ast.PragmaNode{
			NodeBase: ast.NodeBase{NodeKind: ast.KindPragma, NodeSpan: span},
			Text:     tok.Text,
		})
	}
	return attrs, pragmas
}

// parseAttributeText recognises {attribute 'name'} and
// {attribute 'name' := 'value'}. The keyword is case-insensitive, name and
// value may use ' or " quotes, and a doubled quote character inside a quoted
// string stands for one quote. doubleQuoted reports that the name used "
// quotes. Any other shape returns ok=false so the pragma can be kept
// verbatim. The scan is a single forward pass with every index
// bounds-checked, so hostile input cannot panic or loop.
func parseAttributeText(text string) (name, value string, hasValue, doubleQuoted, ok bool) {
	const kw = "attribute"
	if len(text) < 2 || text[0] != '{' || text[len(text)-1] != '}' {
		return "", "", false, false, false
	}
	// Work on the inside of the braces.
	s := text[1 : len(text)-1]
	i := skipPragmaSpace(s, 0)
	if len(s)-i < len(kw) || !strings.EqualFold(s[i:i+len(kw)], kw) {
		return "", "", false, false, false
	}
	i += len(kw)
	j := skipPragmaSpace(s, i)
	if j == i {
		return "", "", false, false, false // keyword must be followed by whitespace
	}
	doubleQuoted = j < len(s) && s[j] == '"'
	name, i, ok = scanPragmaQuoted(s, j)
	if !ok || name == "" {
		return "", "", false, false, false
	}
	i = skipPragmaSpace(s, i)
	if i == len(s) {
		return name, "", false, doubleQuoted, true
	}
	if !strings.HasPrefix(s[i:], ":=") {
		return "", "", false, false, false
	}
	value, i, ok = scanPragmaQuoted(s, skipPragmaSpace(s, i+2))
	if !ok || skipPragmaSpace(s, i) != len(s) {
		return "", "", false, false, false
	}
	return name, value, true, doubleQuoted, true
}

// skipPragmaSpace returns the index of the first non-whitespace byte at or
// after i.
func skipPragmaSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n') {
		i++
	}
	return i
}

// scanPragmaQuoted reads a ' or " quoted string starting at s[i] and returns
// its content (doubled quotes collapsed, $ escapes kept raw) and the index
// just past the closing quote.
func scanPragmaQuoted(s string, i int) (string, int, bool) {
	if i >= len(s) || (s[i] != '\'' && s[i] != '"') {
		return "", i, false
	}
	q := s[i]
	var b strings.Builder
	for j := i + 1; j < len(s); j++ {
		// An IEC escape ($', $$, $N, ...) is kept raw for the consumer
		// (e.g. the OPC UA Description unescaping); its quote does not
		// close the string.
		if s[j] == '$' && j+1 < len(s) {
			b.WriteByte('$')
			b.WriteByte(s[j+1])
			j++
			continue
		}
		if s[j] != q {
			b.WriteByte(s[j])
			continue
		}
		if j+1 < len(s) && s[j+1] == q {
			b.WriteByte(q)
			j++
			continue
		}
		return b.String(), j + 1, true
	}
	return "", i, false
}
