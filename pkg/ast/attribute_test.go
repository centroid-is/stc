package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAttribute_String(t *testing.T) {
	tests := []struct {
		name string
		attr Attribute
		want string
	}{
		{"with value", Attribute{Name: "OPC.UA.DA", Value: "1", HasValue: true}, "{attribute 'OPC.UA.DA' := '1'}"},
		{"without value", Attribute{Name: "qualified_only"}, "{attribute 'qualified_only'}"},
		{"quote in value", Attribute{Name: "OPC.UA.DA.Description", Value: "The slave controller's count", HasValue: true},
			"{attribute 'OPC.UA.DA.Description' := 'The slave controller''s count'}"},
		{"quote in name", Attribute{Name: "it's"}, "{attribute 'it''s'}"},
		{"empty value kept", Attribute{Name: "x", Value: "", HasValue: true}, "{attribute 'x' := ''}"},
		{"round trip escaped", Attribute{Name: "x", Value: "it's", HasValue: true}, "{attribute 'x' := 'it''s'}"},
		{"closing brace in value", Attribute{Name: "x", Value: "a}b", HasValue: true}, "{attribute 'x' := 'a)b'}"},
		{"closing brace in name", Attribute{Name: "n}"}, "{attribute 'n)'}"},
		{"double quoted name", Attribute{Name: "qualified_only", DoubleQuoted: true}, `{attribute "qualified_only"}`},
		{"double quoted with value", Attribute{Name: "x", Value: `a"b'c}`, HasValue: true, DoubleQuoted: true}, `{attribute "x" := "a""b'c)"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.attr.String())
		})
	}
}

func TestAttribute_Children(t *testing.T) {
	assert.Nil(t, (&Attribute{Name: "x"}).Children())
}

func TestAttribute_HasAttribute(t *testing.T) {
	attrs := []*Attribute{{Name: "OPC.UA.DA"}, {Name: "qualified_only"}}
	assert.True(t, HasAttribute(attrs, "QUALIFIED_ONLY"))
	assert.True(t, HasAttribute(attrs, "opc.ua.da"))
	assert.False(t, HasAttribute(attrs, "TcLinkTo"))
	assert.False(t, HasAttribute(nil, "qualified_only"))
	assert.False(t, HasAttribute([]*Attribute{nil}, "qualified_only"))
	// TwinCAT ignores attribute names written in double quotes.
	dq := []*Attribute{{Name: "qualified_only", DoubleQuoted: true}}
	assert.False(t, HasAttribute(dq, "qualified_only"))
	assert.True(t, HasAttribute(append(dq, &Attribute{Name: "qualified_only"}), "qualified_only"))
}

func TestSanitizeGVLName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ECT", "ECT"},
		{"gvl-1", "gvl_1"},
		{"1abc", "_1abc"},
		{"", "GVL"},
		{"a b.c", "a_b_c"},
		{"x'; END_VAR", "x___END_VAR"},
		{"Gvl_Ok9", "Gvl_Ok9"},
		{"été", "_t_"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, SanitizeGVLName(tt.in))
		})
	}
}

func TestSetGVLName(t *testing.T) {
	t.Run("renames existing GVL", func(t *testing.T) {
		g1 := &GVLDecl{NodeBase: NodeBase{NodeKind: KindGVLDecl}, Name: &Ident{Name: "old"}, NameDerived: true}
		g2 := &GVLDecl{NodeBase: NodeBase{NodeKind: KindGVLDecl}}
		f := &SourceFile{Declarations: []Declaration{&ProgramDecl{Name: &Ident{Name: "Main"}}, g1, g2}}
		assert.True(t, SetGVLName(f, "ECT"))
		assert.Equal(t, "ECT", g1.Name.Name)
		assert.False(t, g1.NameDerived, "an explicit name is not derived")
		if assert.NotNil(t, g2.Name) {
			assert.Equal(t, "ECT", g2.Name.Name)
			assert.Equal(t, KindIdent, g2.Name.Kind())
		}
	})
	t.Run("no GVL", func(t *testing.T) {
		f := &SourceFile{Declarations: []Declaration{&ProgramDecl{Name: &Ident{Name: "Main"}}}}
		assert.False(t, SetGVLName(f, "ECT"))
	})
	t.Run("nil file", func(t *testing.T) {
		assert.False(t, SetGVLName(nil, "ECT"))
	})
}

func TestGVLDecl_Children(t *testing.T) {
	attr := &Attribute{Name: "qualified_only"}
	pragma := &PragmaNode{Text: "{warning disable C0001}"}
	name := &Ident{Name: "ECT"}
	blk := &VarBlock{Section: VarGlobal}
	g := &GVLDecl{Name: name, Blocks: []*VarBlock{blk}, Attributes: []*Attribute{attr}, Pragmas: []*PragmaNode{pragma}}
	assert.Equal(t, []Node{attr, pragma, name, blk}, g.Children())
	assert.Len(t, (&GVLDecl{}).Children(), 0)
	var _ Declaration = g
}

// TestAttributeFirstChildren checks that every node carrying Attributes and
// Pragmas lists them before its other children, so trivia attachment maps a
// comment above an attribute to the Attribute node.
func TestAttributeFirstChildren(t *testing.T) {
	attr := &Attribute{Name: "a"}
	pragma := &PragmaNode{Text: "{region}"}
	attrs := []*Attribute{attr}
	pragmas := []*PragmaNode{pragma}
	name := &Ident{Name: "n"}

	nodes := map[string]Node{
		"VarBlock":          &VarBlock{Attributes: attrs, Pragmas: pragmas, Declarations: []*VarDecl{{}}},
		"VarDecl":           &VarDecl{Attributes: attrs, Pragmas: pragmas, Names: []*Ident{name}},
		"StructMember":      &StructMember{Attributes: attrs, Pragmas: pragmas, Name: name},
		"EnumValue":         &EnumValue{Attributes: attrs, Pragmas: pragmas, Name: name},
		"TypeDecl":          &TypeDecl{Attributes: attrs, Pragmas: pragmas, Name: name},
		"ProgramDecl":       &ProgramDecl{Attributes: attrs, Pragmas: pragmas, Name: name},
		"FunctionBlockDecl": &FunctionBlockDecl{Attributes: attrs, Pragmas: pragmas, Name: name},
		"FunctionDecl":      &FunctionDecl{Attributes: attrs, Pragmas: pragmas, Name: name},
		"MethodDecl":        &MethodDecl{Attributes: attrs, Pragmas: pragmas, Name: name},
		"PropertyDecl":      &PropertyDecl{Attributes: attrs, Pragmas: pragmas, Name: name},
		"InterfaceDecl":     &InterfaceDecl{Attributes: attrs, Pragmas: pragmas, Name: name},
		"ActionDecl":        &ActionDecl{Attributes: attrs, Pragmas: pragmas, Name: name},
	}
	for kind, n := range nodes {
		t.Run(kind, func(t *testing.T) {
			ch := n.Children()
			if assert.GreaterOrEqual(t, len(ch), 3) {
				assert.Same(t, attr, ch[0])
				assert.Same(t, pragma, ch[1])
			}
		})
	}
}

func TestProgramDecl_ChildrenOrderWithActions(t *testing.T) {
	attr := &Attribute{Name: "a"}
	pragma := &PragmaNode{Text: "{region}"}
	name := &Ident{Name: "Main"}
	vb := &VarBlock{Section: VarLocal}
	stmt := &ReturnStmt{}
	act := &ActionDecl{Name: &Ident{Name: "Reset"}}
	p := &ProgramDecl{
		Attributes: []*Attribute{attr}, Pragmas: []*PragmaNode{pragma},
		Name: name, VarBlocks: []*VarBlock{vb}, Body: []Statement{stmt}, Actions: []*ActionDecl{act},
	}
	assert.Equal(t, []Node{attr, pragma, name, vb, stmt, act}, p.Children())
}

func TestFunctionBlockDecl_ChildrenActionsLast(t *testing.T) {
	act := &ActionDecl{Name: &Ident{Name: "Reset"}}
	m := &MethodDecl{Name: &Ident{Name: "M"}}
	prop := &PropertyDecl{Name: &Ident{Name: "P"}}
	fb := &FunctionBlockDecl{
		Name: &Ident{Name: "FB"}, Methods: []*MethodDecl{m}, Properties: []*PropertyDecl{prop},
		Actions: []*ActionDecl{act},
	}
	ch := fb.Children()
	assert.Same(t, act, ch[len(ch)-1])
	assert.Len(t, ch, 4)
}

func TestStructMember_ChildrenIncludesAtAddress(t *testing.T) {
	at := &Ident{Name: "%I*"}
	sm := &StructMember{Name: &Ident{Name: "I1"}, Type: &NamedType{Name: &Ident{Name: "BOOL"}}, AtAddress: at}
	ch := sm.Children()
	assert.Contains(t, ch, Node(at))
	assert.Len(t, ch, 3)
}
