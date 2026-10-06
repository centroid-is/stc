package parser

import (
	"os"
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

func parseClean(t *testing.T, src string) *ast.SourceFile {
	t.Helper()
	r := Parse("t.st", src)
	require.Empty(t, r.Diags, "unexpected diagnostics: %v", r.Diags)
	return r.File
}

func attrNames(attrs []*ast.Attribute) []string {
	var out []string
	for _, a := range attrs {
		out = append(out, a.Name)
	}
	return out
}

func readProbe(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../tests/twincat_probes/" + name)
	require.NoError(t, err)
	return string(data)
}

func TestPragmaAttach(t *testing.T) {
	t.Run("structpragma fixture attaches to I1 only", func(t *testing.T) {
		f := parseClean(t, readProbe(t, "structpragma.st"))
		st := f.Declarations[0].(*ast.TypeDecl).Type.(*ast.StructType)
		require.Len(t, st.Members, 2)
		require.Len(t, st.Members[0].Attributes, 1)
		a := st.Members[0].Attributes[0]
		require.Equal(t, "OPC.UA.DA.Access", a.Name)
		require.Equal(t, "1", a.Value)
		require.True(t, a.HasValue)
		require.Empty(t, st.Members[1].Attributes)
	})

	t.Run("enum_attr fixture attaches to enum value and type", func(t *testing.T) {
		f := parseClean(t, readProbe(t, "enum_attr.st"))
		td := f.Declarations[0].(*ast.TypeDecl)
		require.Equal(t, []string{"qualified_only", "strict"}, attrNames(td.Attributes))
		et := td.Type.(*ast.EnumType)
		require.Len(t, et.Values, 2)
		require.Equal(t, "rdy", et.Values[0].Name.Name)
		require.Len(t, et.Values[0].Attributes, 1)
		require.Equal(t, "OPC.UA.DA.Description", et.Values[0].Attributes[0].Name)
		require.Equal(t, "Ready", et.Values[0].Attributes[0].Value)
		require.Empty(t, et.Values[1].Attributes)
	})

	t.Run("link fixture attaches TcLinkTo to I1", func(t *testing.T) {
		r := Parse("link.st", readProbe(t, "link.st"))
		for _, d := range r.Diags {
			require.NotContains(t, []int{3, 4}, d.Pos.Line, "diagnostic on attribute or AT line: %v", d)
		}
		prog := r.File.Declarations[0].(*ast.ProgramDecl)
		vd := prog.VarBlocks[0].Declarations[0]
		require.Equal(t, "I1", vd.Names[0].Name)
		require.Equal(t, []string{"TcLinkTo"}, attrNames(vd.Attributes))
		require.Contains(t, vd.Attributes[0].Value, "^Term 2 (EL1008)^")
	})

	t.Run("attribute before POUs and TYPE", func(t *testing.T) {
		f := parseClean(t, `{attribute 'p'}
PROGRAM P
END_PROGRAM
{attribute 'fb'}
FUNCTION_BLOCK FB
END_FUNCTION_BLOCK
{attribute 'f'}
FUNCTION F : INT
F := 1;
END_FUNCTION
{attribute 'i'}
INTERFACE I
END_INTERFACE
{attribute 't'}
TYPE T : INT; END_TYPE
`)
		require.Len(t, f.Declarations, 5)
		require.Equal(t, []string{"p"}, attrNames(f.Declarations[0].(*ast.ProgramDecl).Attributes))
		require.Equal(t, []string{"fb"}, attrNames(f.Declarations[1].(*ast.FunctionBlockDecl).Attributes))
		require.Equal(t, []string{"f"}, attrNames(f.Declarations[2].(*ast.FunctionDecl).Attributes))
		require.Equal(t, []string{"i"}, attrNames(f.Declarations[3].(*ast.InterfaceDecl).Attributes))
		require.Equal(t, []string{"t"}, attrNames(f.Declarations[4].(*ast.TypeDecl).Attributes))
	})

	t.Run("attribute before METHOD and PROPERTY in FB", func(t *testing.T) {
		f := parseClean(t, `FUNCTION_BLOCK FB
VAR
    x : INT;
END_VAR
{attribute 'm1'}
METHOD M1 : BOOL
M1 := TRUE;
END_METHOD
x := 1;
{attribute 'm2'}
{warning disable C0001}
PUBLIC METHOD M2
END_METHOD
{attribute 'prop'}
PROPERTY P : INT
GET
P := x;
END_GET
END_PROPERTY
END_FUNCTION_BLOCK
`)
		fb := f.Declarations[0].(*ast.FunctionBlockDecl)
		require.Len(t, fb.Methods, 2)
		require.Equal(t, []string{"m1"}, attrNames(fb.Methods[0].Attributes))
		require.Equal(t, []string{"m2"}, attrNames(fb.Methods[1].Attributes))
		require.Len(t, fb.Methods[1].Pragmas, 1)
		require.Equal(t, "{warning disable C0001}", fb.Methods[1].Pragmas[0].Text)
		require.Len(t, fb.Properties, 1)
		require.Equal(t, []string{"prop"}, attrNames(fb.Properties[0].Attributes))
		require.Len(t, fb.Body, 1, "only x := 1 belongs to the FB body")
		require.IsType(t, &ast.AssignStmt{}, fb.Body[0])
	})

	t.Run("statement pragma inside FB body is skipped", func(t *testing.T) {
		f := parseClean(t, "FUNCTION_BLOCK FB\nVAR x : INT; END_VAR\n{warning disable C0001}\nx := 1;\n{region}\nx := 2;\nEND_FUNCTION_BLOCK\n")
		fb := f.Declarations[0].(*ast.FunctionBlockDecl)
		require.Len(t, fb.Body, 2)
		require.Empty(t, fb.Methods)
	})

	t.Run("attribute before VAR blocks", func(t *testing.T) {
		f := parseClean(t, `FUNCTION_BLOCK FB
{attribute 'in'}
VAR_INPUT
    a : INT;
END_VAR
{attribute 'io'}
VAR_IN_OUT
    {attribute 'member'}
    b : INT;
END_VAR
{attribute 'pr'}
VAR PERSISTENT RETAIN
    c : INT;
END_VAR
{attribute 'loc'}
VAR
    d : INT;
END_VAR
END_FUNCTION_BLOCK
`)
		fb := f.Declarations[0].(*ast.FunctionBlockDecl)
		require.Len(t, fb.VarBlocks, 4)
		require.Equal(t, []string{"in"}, attrNames(fb.VarBlocks[0].Attributes))
		require.Equal(t, []string{"io"}, attrNames(fb.VarBlocks[1].Attributes))
		require.Equal(t, ast.VarInOut, fb.VarBlocks[1].Section)
		require.Equal(t, []string{"member"}, attrNames(fb.VarBlocks[1].Declarations[0].Attributes))
		require.Equal(t, []string{"pr"}, attrNames(fb.VarBlocks[2].Attributes))
		require.True(t, fb.VarBlocks[2].IsPersistent)
		require.True(t, fb.VarBlocks[2].IsRetain)
		require.Equal(t, []string{"loc"}, attrNames(fb.VarBlocks[3].Attributes))
	})

	t.Run("blank line between attribute and declaration", func(t *testing.T) {
		f := parseClean(t, "{attribute 'a'}\n\n\nPROGRAM P\nVAR\n    {attribute 'b'}\n\n    x : INT;\nEND_VAR\nEND_PROGRAM\n")
		prog := f.Declarations[0].(*ast.ProgramDecl)
		require.Equal(t, []string{"a"}, attrNames(prog.Attributes))
		require.Equal(t, []string{"b"}, attrNames(prog.VarBlocks[0].Declarations[0].Attributes))
	})

	t.Run("non-attribute pragma before VarDecl", func(t *testing.T) {
		f := parseClean(t, "PROGRAM P\nVAR\n    {warning disable C0001}\n    x : INT;\nEND_VAR\nEND_PROGRAM\n")
		vd := f.Declarations[0].(*ast.ProgramDecl).VarBlocks[0].Declarations[0]
		require.Empty(t, vd.Attributes)
		require.Len(t, vd.Pragmas, 1)
		require.Equal(t, "{warning disable C0001}", vd.Pragmas[0].Text)
		require.Equal(t, ast.KindPragma, vd.Pragmas[0].Kind())
	})

	t.Run("pragmas inside an interface attach to signatures", func(t *testing.T) {
		f := parseClean(t, "INTERFACE I\n{attribute 'TcRpcEnable'}\nMETHOD M : BOOL\nEND_METHOD\n{warning disable C0001}\nPROPERTY P : INT\nEND_PROPERTY\n{endregion}\nEND_INTERFACE\n")
		it := f.Declarations[0].(*ast.InterfaceDecl)
		require.Len(t, it.Methods, 1)
		require.Equal(t, []string{"TcRpcEnable"}, attrNames(it.Methods[0].Attributes))
		require.Len(t, it.Properties, 1)
		require.Len(t, it.Properties[0].Pragmas, 1)
		require.Len(t, it.EndPragmas, 1)
		require.Equal(t, "{endregion}", it.EndPragmas[0].Text)
	})

	t.Run("pragma at EOF of an unterminated interface is kept", func(t *testing.T) {
		r := Parse("t.st", "INTERFACE I\n{endregion}\n")
		require.NotEmpty(t, r.Diags)
		it := r.File.Declarations[0].(*ast.InterfaceDecl)
		require.Len(t, it.EndPragmas, 1)
	})

	t.Run("pragma before END_VAR is a block end pragma", func(t *testing.T) {
		f := parseClean(t, "PROGRAM P\nVAR\n    x : INT;\n    {attribute 'tail'}\n    {region}\nEND_VAR\nEND_PROGRAM\n")
		vb := f.Declarations[0].(*ast.ProgramDecl).VarBlocks[0]
		require.Len(t, vb.Declarations, 1)
		require.Empty(t, vb.Attributes)
		require.Empty(t, vb.Pragmas)
		require.Equal(t, []string{"tail"}, attrNames(vb.EndAttributes))
		require.Len(t, vb.EndPragmas, 1)
	})

	t.Run("pragma before END_STRUCT is a struct end pragma", func(t *testing.T) {
		f := parseClean(t, "TYPE S :\nSTRUCT\n    a : INT;\n    {attribute 'tail'}\nEND_STRUCT\nEND_TYPE\n")
		st := f.Declarations[0].(*ast.TypeDecl).Type.(*ast.StructType)
		require.Len(t, st.Members, 1)
		require.Empty(t, st.Members[0].Attributes)
		require.Equal(t, []string{"tail"}, attrNames(st.EndAttributes))
	})

	t.Run("pragma in empty struct is kept as an end pragma", func(t *testing.T) {
		f := parseClean(t, "TYPE S :\nSTRUCT\n    {warning disable C0001}\nEND_STRUCT\nEND_TYPE\n")
		st := f.Declarations[0].(*ast.TypeDecl).Type.(*ast.StructType)
		require.Empty(t, st.Members)
		require.Len(t, st.EndPragmas, 1)
	})

	t.Run("pragma before closing paren of enum is an enum end pragma", func(t *testing.T) {
		f := parseClean(t, "TYPE E : (a, b {attribute 'tail'}); END_TYPE\n")
		et := f.Declarations[0].(*ast.TypeDecl).Type.(*ast.EnumType)
		require.Len(t, et.Values, 2)
		require.Empty(t, et.Values[1].Attributes)
		require.Equal(t, []string{"tail"}, attrNames(et.EndAttributes))
	})

	t.Run("pragma after trailing comma of enum is an enum end pragma", func(t *testing.T) {
		f := parseClean(t, "TYPE E : (a, {endregion}\n); END_TYPE\n")
		et := f.Declarations[0].(*ast.TypeDecl).Type.(*ast.EnumType)
		require.Len(t, et.Values, 1)
		require.Len(t, et.EndPragmas, 1)
	})

	t.Run("pragma in empty enum is kept as an end pragma", func(t *testing.T) {
		r := Parse("t.st", "TYPE E : ({region}); END_TYPE\n")
		require.NotNil(t, r.File)
		et := r.File.Declarations[0].(*ast.TypeDecl).Type.(*ast.EnumType)
		require.Len(t, et.EndPragmas, 1)
	})

	t.Run("comment above attribute is leading trivia of the attribute", func(t *testing.T) {
		f := parseClean(t, "PROGRAM P\nVAR\n    // c1\n    {attribute 'x' := 'y'}\n    // c2\n    v : BOOL;\nEND_VAR\nEND_PROGRAM\n")
		vb := f.Declarations[0].(*ast.ProgramDecl).VarBlocks[0]
		require.Empty(t, vb.LeadingTrivia)
		vd := vb.Declarations[0]
		require.Len(t, vd.Attributes, 1)
		require.Len(t, vd.Attributes[0].LeadingTrivia, 1)
		require.Equal(t, "// c1", vd.Attributes[0].LeadingTrivia[0].Text)
		require.Len(t, vd.LeadingTrivia, 1)
		require.Equal(t, "// c2", vd.LeadingTrivia[0].Text)
	})

	t.Run("comment above top-level pragma is its leading trivia", func(t *testing.T) {
		f := parseClean(t, "// head\n{warning disable C0001}\nPROGRAM P\nEND_PROGRAM\n")
		prog := f.Declarations[0].(*ast.ProgramDecl)
		require.Len(t, prog.Pragmas, 1)
		require.Len(t, prog.Pragmas[0].LeadingTrivia, 1)
		require.Equal(t, "// head", prog.Pragmas[0].LeadingTrivia[0].Text)
	})

	t.Run("recovery stops at pragma so next POU keeps attributes", func(t *testing.T) {
		r := Parse("t.st", "x := 1;\n{attribute 'keep'}\nFUNCTION_BLOCK FB2\nEND_FUNCTION_BLOCK\n")
		require.NotEmpty(t, r.Diags)
		var fb *ast.FunctionBlockDecl
		for _, d := range r.File.Declarations {
			if x, ok := d.(*ast.FunctionBlockDecl); ok {
				fb = x
			}
		}
		require.NotNil(t, fb)
		require.Equal(t, []string{"keep"}, attrNames(fb.Attributes))
	})

	t.Run("pragma at end of file", func(t *testing.T) {
		r := Parse("t.st", "PROGRAM P\nEND_PROGRAM\n{attribute 'eof'}\n")
		require.Empty(t, r.Diags)
		require.Len(t, r.File.Declarations, 1)
	})

	t.Run("pragma before non-declaration reports diagnostic", func(t *testing.T) {
		r := Parse("t.st", "{attribute 'a'}\nEND_VAR\n")
		require.NotEmpty(t, r.Diags)
	})

	t.Run("pragma before program body is skipped", func(t *testing.T) {
		f := parseClean(t, "PROGRAM P\nVAR x : INT; END_VAR\n{warning disable C0001}\nx := 1;\nEND_PROGRAM\n")
		prog := f.Declarations[0].(*ast.ProgramDecl)
		require.Len(t, prog.VarBlocks, 1)
		require.Empty(t, prog.VarBlocks[0].Pragmas)
		require.Len(t, prog.Body, 1)
	})
}
