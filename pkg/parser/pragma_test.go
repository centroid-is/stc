package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/lexer"
	"github.com/stretchr/testify/require"
)

func TestAttributeText(t *testing.T) {
	const link = "{attribute 'TcLinkTo' := '.I1 := TIID^Device 1 (EtherCAT)^ST301.A1.00 (EK1200)^ST301.A1.03 (EL1008)^Channel 1^Input; .I2 := TIID^x'}"
	tests := []struct {
		name     string
		text     string
		wantName string
		wantVal  string
		hasValue bool
		ok       bool
	}{
		{"single quoted no value", "{attribute 'qualified_only'}", "qualified_only", "", false, true},
		{"double quoted no value", `{attribute "qualified_only"}`, "qualified_only", "", false, true},
		{"name and value", "{attribute 'OPC.UA.DA' := '1'}", "OPC.UA.DA", "1", true, true},
		{"doubled quote in value", "{attribute 'OPC.UA.DA.Description' := 'The slave controller''s own lost-link count'}",
			"OPC.UA.DA.Description", "The slave controller's own lost-link count", true, true},
		{"TcLinkTo value", link, "TcLinkTo",
			".I1 := TIID^Device 1 (EtherCAT)^ST301.A1.00 (EK1200)^ST301.A1.03 (EL1008)^Channel 1^Input; .I2 := TIID^x", true, true},
		{"non-ASCII value", "{attribute 'OPC.UA.DA.Description' := 'Ready — waiting'}", "OPC.UA.DA.Description", "Ready — waiting", true, true},
		{"uppercase keyword", "{ATTRIBUTE 'x'}", "x", "", false, true},
		{"extra spaces", "{attribute  'x'  :=  'y' }", "x", "y", true, true},
		{"leading space and tab", "{ \tattribute 'x'}", "x", "", false, true},
		{"empty value", "{attribute 'x' := ''}", "x", "", true, true},
		{"double quoted value with doubling", `{attribute "x" := "a""b"}`, "x", `a"b`, true, true},
		{"warning pragma", "{warning disable C0139}", "", "", false, false},
		{"region pragma", `{region "x"}`, "", "", false, false},
		{"attribute without name", "{attribute}", "", "", false, false},
		{"attribute without space", "{attribute'x'}", "", "", false, false},
		{"missing value", "{attribute 'x' := }", "", "", false, false},
		{"unterminated name", "{attribute 'unterminated}", "", "", false, false},
		{"unterminated value", "{attribute 'x' := 'y}", "", "", false, false},
		{"empty name", "{attribute ''}", "", "", false, false},
		{"unquoted name", "{attribute x}", "", "", false, false},
		{"garbage after name", "{attribute 'x' y}", "", "", false, false},
		{"garbage after value", "{attribute 'x' := 'y' z}", "", "", false, false},
		{"no closing brace", "{attribute 'x'", "", "", false, false},
		{"no opening brace", "attribute 'x'}", "", "", false, false},
		{"short text", "{attr", "", "", false, false},
		{"empty", "", "", "", false, false},
		{"text after closing brace", "{attribute 'x'}}", "", "", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, value, hasValue, ok := parseAttributeText(tc.text)
			require.Equal(t, tc.ok, ok)
			if !tc.ok {
				return
			}
			require.Equal(t, tc.wantName, name)
			require.Equal(t, tc.wantVal, value)
			require.Equal(t, tc.hasValue, hasValue)
		})
	}
}

func TestAttributeTextRoundTrip(t *testing.T) {
	// Re-rendering a parsed attribute with ast.Attribute.String and parsing
	// it again yields the same name and value.
	for _, text := range []string{
		"{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^Term 2 (EL1008)^Channel 1^Input'}",
		"{attribute 'OPC.UA.DA.Description' := 'controller''s — count'}",
		`{attribute "qualified_only"}`,
	} {
		name, value, hasValue, ok := parseAttributeText(text)
		require.True(t, ok, text)
		a := &ast.Attribute{Name: name, Value: value, HasValue: hasValue}
		n2, v2, h2, ok2 := parseAttributeText(a.String())
		require.True(t, ok2, a.String())
		require.Equal(t, name, n2)
		require.Equal(t, value, v2)
		require.Equal(t, hasValue, h2)
	}
}

func TestCollectPragmas(t *testing.T) {
	src := "{attribute 'a'} {warning disable C0001}\n{attribute 'b' := 'c'} x"
	p := newTestParser(src)
	attrs, pragmas := p.collectPragmas()
	require.Len(t, attrs, 2)
	require.Len(t, pragmas, 1)

	require.Equal(t, "a", attrs[0].Name)
	require.False(t, attrs[0].HasValue)
	require.Equal(t, ast.KindAttribute, attrs[0].Kind())
	require.Equal(t, 0, attrs[0].Span().Start.Offset)
	require.Equal(t, len("{attribute 'a'}"), attrs[0].Span().End.Offset)

	require.Equal(t, "b", attrs[1].Name)
	require.Equal(t, "c", attrs[1].Value)
	require.True(t, attrs[1].HasValue)
	require.Equal(t, 2, attrs[1].Span().Start.Line)

	require.Equal(t, "{warning disable C0001}", pragmas[0].Text)
	require.Equal(t, ast.KindPragma, pragmas[0].Kind())
	require.Less(t, attrs[0].Span().Start.Offset, pragmas[0].Span().Start.Offset)
	require.Less(t, pragmas[0].Span().Start.Offset, attrs[1].Span().Start.Offset)

	// The parser stops at the first non-pragma token.
	require.Equal(t, "x", p.peek().Text)

	// No pragmas: nothing collected, nothing consumed.
	a2, p2 := p.collectPragmas()
	require.Nil(t, a2)
	require.Nil(t, p2)
	require.Equal(t, "x", p.peek().Text)
}

// newTestParser builds a Parser over src the same way Parse does, without
// parsing anything.
func newTestParser(src string) *Parser {
	p := &Parser{filename: "t.st", source: src, diags: diag.NewCollector()}
	for _, tok := range lexer.Tokenize("t.st", src) {
		if !tok.Kind.IsTrivia() {
			p.tokens = append(p.tokens, tok)
		}
	}
	return p
}
