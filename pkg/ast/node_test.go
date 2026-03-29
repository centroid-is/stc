package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NodeKind ---

func TestNodeKind_String(t *testing.T) {
	tests := []struct {
		kind NodeKind
		want string
	}{
		{KindSourceFile, "SourceFile"},
		{KindProgramDecl, "ProgramDecl"},
		{KindFunctionBlockDecl, "FunctionBlockDecl"},
		{KindFunctionDecl, "FunctionDecl"},
		{KindInterfaceDecl, "InterfaceDecl"},
		{KindMethodDecl, "MethodDecl"},
		{KindPropertyDecl, "PropertyDecl"},
		{KindTypeDecl, "TypeDecl"},
		{KindActionDecl, "ActionDecl"},
		{KindTestCaseDecl, "TestCaseDecl"},
		{KindAssignStmt, "AssignStmt"},
		{KindCallStmt, "CallStmt"},
		{KindIfStmt, "IfStmt"},
		{KindCaseStmt, "CaseStmt"},
		{KindForStmt, "ForStmt"},
		{KindWhileStmt, "WhileStmt"},
		{KindRepeatStmt, "RepeatStmt"},
		{KindReturnStmt, "ReturnStmt"},
		{KindExitStmt, "ExitStmt"},
		{KindContinueStmt, "ContinueStmt"},
		{KindEmptyStmt, "EmptyStmt"},
		{KindErrorNode, "ErrorNode"},
		{KindBinaryExpr, "BinaryExpr"},
		{KindUnaryExpr, "UnaryExpr"},
		{KindLiteral, "Literal"},
		{KindIdent, "Ident"},
		{KindCallExpr, "CallExpr"},
		{KindMemberAccessExpr, "MemberAccessExpr"},
		{KindIndexExpr, "IndexExpr"},
		{KindDerefExpr, "DerefExpr"},
		{KindParenExpr, "ParenExpr"},
		{KindNamedType, "NamedType"},
		{KindArrayType, "ArrayType"},
		{KindPointerType, "PointerType"},
		{KindReferenceType, "ReferenceType"},
		{KindStringType, "StringType"},
		{KindSubrangeType, "SubrangeType"},
		{KindEnumType, "EnumType"},
		{KindStructType, "StructType"},
		{KindVarBlock, "VarBlock"},
		{KindVarDecl, "VarDecl"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.kind.String())
		})
	}
}

func TestNodeKind_String_Unknown(t *testing.T) {
	unknown := NodeKind(9999)
	assert.Equal(t, "Unknown", unknown.String())
}

// --- NodeBase ---

func TestNodeBase_Kind(t *testing.T) {
	nb := &NodeBase{NodeKind: KindIdent}
	assert.Equal(t, KindIdent, nb.Kind())
}

func TestNodeBase_Span(t *testing.T) {
	span := Span{
		Start: Pos{File: "test.st", Line: 1, Col: 1, Offset: 0},
		End:   Pos{File: "test.st", Line: 1, Col: 10, Offset: 9},
	}
	nb := &NodeBase{NodeSpan: span}
	assert.Equal(t, span, nb.Span())
}

// --- SpanFrom ---

func TestSpanFrom(t *testing.T) {
	start := Pos{File: "a.st", Line: 1, Col: 1}
	end := Pos{File: "a.st", Line: 1, Col: 10}
	s := SpanFrom(start, end)
	assert.Equal(t, start, s.Start)
	assert.Equal(t, end, s.End)
}

// --- ErrorNode ---

func TestErrorNode_Children(t *testing.T) {
	en := &ErrorNode{Message: "test error"}
	assert.Nil(t, en.Children())
}

func TestErrorNode_InterfaceSatisfaction(t *testing.T) {
	en := &ErrorNode{
		NodeBase: NodeBase{NodeKind: KindErrorNode},
		Message:  "test",
	}
	assert.Equal(t, KindErrorNode, en.Kind())
	// Verify it satisfies all marker interfaces
	var _ Declaration = en
	var _ Statement = en
	var _ Expr = en
	var _ TypeSpec = en
}

// --- Ident ---

func TestIdent_Children(t *testing.T) {
	id := &Ident{Name: "x"}
	assert.Nil(t, id.Children())
}

// --- AccessModifier ---

func TestAccessModifier_String(t *testing.T) {
	tests := []struct {
		mod  AccessModifier
		want string
	}{
		{AccessNone, ""},
		{AccessPublic, "PUBLIC"},
		{AccessPrivate, "PRIVATE"},
		{AccessProtected, "PROTECTED"},
		{AccessInternal, "INTERNAL"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.mod.String())
	}
}

func TestAccessModifier_String_OutOfRange(t *testing.T) {
	assert.Equal(t, "", AccessModifier(999).String())
}

// --- VarSection ---

func TestVarSection_String(t *testing.T) {
	tests := []struct {
		sec  VarSection
		want string
	}{
		{VarLocal, "VAR"},
		{VarInput, "VAR_INPUT"},
		{VarOutput, "VAR_OUTPUT"},
		{VarInOut, "VAR_IN_OUT"},
		{VarTemp, "VAR_TEMP"},
		{VarGlobal, "VAR_GLOBAL"},
		{VarAccess, "VAR_ACCESS"},
		{VarExternal, "VAR_EXTERNAL"},
		{VarConfig, "VAR_CONFIG"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.sec.String())
	}
}

func TestVarSection_String_OutOfRange(t *testing.T) {
	assert.Equal(t, "VAR", VarSection(999).String())
}

// --- LiteralKind ---

func TestLiteralKind_String(t *testing.T) {
	tests := []struct {
		kind LiteralKind
		want string
	}{
		{LitInt, "Int"},
		{LitReal, "Real"},
		{LitString, "String"},
		{LitWString, "WString"},
		{LitBool, "Bool"},
		{LitTime, "Time"},
		{LitDate, "Date"},
		{LitDateTime, "DateTime"},
		{LitTod, "Tod"},
		{LitTyped, "Typed"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.kind.String())
	}
}

func TestLiteralKind_String_Unknown(t *testing.T) {
	assert.Equal(t, "Unknown", LiteralKind(999).String())
}

// --- TriviaKind ---

func TestTriviaKind_String(t *testing.T) {
	tests := []struct {
		kind TriviaKind
		want string
	}{
		{TriviaWhitespace, "Whitespace"},
		{TriviaLineComment, "LineComment"},
		{TriviaBlockComment, "BlockComment"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.kind.String())
	}
}

func TestTriviaKind_String_Unknown(t *testing.T) {
	assert.Equal(t, "Unknown", TriviaKind(999).String())
}

// --- Children() for all node types ---

func TestSourceFile_Children(t *testing.T) {
	t.Run("with declarations", func(t *testing.T) {
		sf := &SourceFile{
			Declarations: []Declaration{
				&ProgramDecl{Name: &Ident{Name: "P1"}},
				&ProgramDecl{Name: &Ident{Name: "P2"}},
			},
		}
		children := sf.Children()
		assert.Len(t, children, 2)
	})

	t.Run("empty declarations", func(t *testing.T) {
		sf := &SourceFile{}
		children := sf.Children()
		assert.Len(t, children, 0)
	})
}

func TestProgramDecl_Children(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		p := &ProgramDecl{
			Name:      &Ident{Name: "Main"},
			VarBlocks: []*VarBlock{{Section: VarLocal}},
			Body:      []Statement{&ReturnStmt{}},
		}
		children := p.Children()
		assert.Len(t, children, 3) // Name + VarBlock + ReturnStmt
	})

	t.Run("nil name", func(t *testing.T) {
		p := &ProgramDecl{}
		assert.Len(t, p.Children(), 0)
	})
}

func TestFunctionBlockDecl_Children(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		fb := &FunctionBlockDecl{
			Name:       &Ident{Name: "FB"},
			Extends:    &Ident{Name: "Base"},
			Implements: []*Ident{{Name: "I1"}},
			VarBlocks:  []*VarBlock{{Section: VarLocal}},
			Body:       []Statement{&ReturnStmt{}},
			Methods:    []*MethodDecl{{Name: &Ident{Name: "M"}}},
			Properties: []*PropertyDecl{{Name: &Ident{Name: "P"}}},
		}
		children := fb.Children()
		assert.Len(t, children, 7)
	})

	t.Run("minimal", func(t *testing.T) {
		fb := &FunctionBlockDecl{}
		assert.Len(t, fb.Children(), 0)
	})
}

func TestFunctionDecl_Children(t *testing.T) {
	t.Run("with return type", func(t *testing.T) {
		f := &FunctionDecl{
			Name:       &Ident{Name: "Add"},
			ReturnType: &NamedType{Name: &Ident{Name: "INT"}},
			VarBlocks:  []*VarBlock{{Section: VarInput}},
			Body:       []Statement{&ReturnStmt{}},
		}
		children := f.Children()
		assert.Len(t, children, 4)
	})

	t.Run("nil return type", func(t *testing.T) {
		f := &FunctionDecl{Name: &Ident{Name: "F"}}
		assert.Len(t, f.Children(), 1) // just name
	})
}

func TestInterfaceDecl_Children(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		i := &InterfaceDecl{
			Name:       &Ident{Name: "IFoo"},
			Extends:    []*Ident{{Name: "IBar"}},
			Methods:    []*MethodSignature{{Name: &Ident{Name: "M"}}},
			Properties: []*PropertySignature{{Name: &Ident{Name: "P"}}},
		}
		children := i.Children()
		assert.Len(t, children, 4)
	})
}

func TestMethodDecl_Children(t *testing.T) {
	m := &MethodDecl{
		Name:       &Ident{Name: "DoWork"},
		ReturnType: &NamedType{Name: &Ident{Name: "BOOL"}},
		VarBlocks:  []*VarBlock{{Section: VarLocal}},
		Body:       []Statement{&ReturnStmt{}},
	}
	children := m.Children()
	assert.Len(t, children, 4)
}

func TestPropertyDecl_Children(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		p := &PropertyDecl{
			Name:   &Ident{Name: "Prop"},
			Type:   &NamedType{Name: &Ident{Name: "INT"}},
			Getter: &MethodDecl{Name: &Ident{Name: "Get"}},
			Setter: &MethodDecl{Name: &Ident{Name: "Set"}},
		}
		children := p.Children()
		assert.Len(t, children, 4)
	})

	t.Run("no getter/setter", func(t *testing.T) {
		p := &PropertyDecl{Name: &Ident{Name: "P"}}
		assert.Len(t, p.Children(), 1)
	})
}

func TestMethodSignature_Children(t *testing.T) {
	t.Run("with return type", func(t *testing.T) {
		ms := &MethodSignature{
			Name:       &Ident{Name: "M"},
			ReturnType: &NamedType{Name: &Ident{Name: "BOOL"}},
			VarBlocks:  []*VarBlock{{Section: VarInput}},
		}
		assert.Len(t, ms.Children(), 3)
	})

	t.Run("nil return type", func(t *testing.T) {
		ms := &MethodSignature{Name: &Ident{Name: "M"}}
		assert.Len(t, ms.Children(), 1)
	})
}

func TestPropertySignature_Children(t *testing.T) {
	ps := &PropertySignature{
		Name: &Ident{Name: "P"},
		Type: &NamedType{Name: &Ident{Name: "INT"}},
	}
	assert.Len(t, ps.Children(), 2)
}

func TestTypeDecl_Children(t *testing.T) {
	td := &TypeDecl{
		Name: &Ident{Name: "T"},
		Type: &NamedType{Name: &Ident{Name: "INT"}},
	}
	assert.Len(t, td.Children(), 2)
}

func TestActionDecl_Children(t *testing.T) {
	a := &ActionDecl{
		Name: &Ident{Name: "Act"},
		Body: []Statement{&ReturnStmt{}, &ExitStmt{}},
	}
	assert.Len(t, a.Children(), 3) // name + 2 stmts
}

func TestTestCaseDecl_Children(t *testing.T) {
	tc := &TestCaseDecl{
		Name:      "test1",
		VarBlocks: []*VarBlock{{Section: VarLocal}},
		Body:      []Statement{&ReturnStmt{}},
	}
	assert.Len(t, tc.Children(), 2) // VarBlock + ReturnStmt
}

// --- Expression Children ---

func TestBinaryExpr_Children(t *testing.T) {
	t.Run("both operands", func(t *testing.T) {
		be := &BinaryExpr{
			Left:  &Ident{Name: "a"},
			Right: &Ident{Name: "b"},
		}
		assert.Len(t, be.Children(), 2)
	})

	t.Run("nil left", func(t *testing.T) {
		be := &BinaryExpr{Right: &Ident{Name: "b"}}
		assert.Len(t, be.Children(), 1)
	})

	t.Run("nil right", func(t *testing.T) {
		be := &BinaryExpr{Left: &Ident{Name: "a"}}
		assert.Len(t, be.Children(), 1)
	})

	t.Run("both nil", func(t *testing.T) {
		be := &BinaryExpr{}
		assert.Len(t, be.Children(), 0)
	})
}

func TestUnaryExpr_Children(t *testing.T) {
	t.Run("with operand", func(t *testing.T) {
		ue := &UnaryExpr{Operand: &Ident{Name: "x"}}
		assert.Len(t, ue.Children(), 1)
	})

	t.Run("nil operand", func(t *testing.T) {
		ue := &UnaryExpr{}
		assert.Nil(t, ue.Children())
	})
}

func TestLiteral_Children(t *testing.T) {
	lit := &Literal{Value: "42"}
	assert.Nil(t, lit.Children())
}

func TestCallExpr_Children(t *testing.T) {
	t.Run("with callee and args", func(t *testing.T) {
		ce := &CallExpr{
			Callee: &Ident{Name: "func"},
			Args:   []Expr{&Literal{Value: "1"}, &Literal{Value: "2"}},
		}
		assert.Len(t, ce.Children(), 3)
	})

	t.Run("nil callee", func(t *testing.T) {
		ce := &CallExpr{Args: []Expr{&Literal{Value: "1"}}}
		assert.Len(t, ce.Children(), 1)
	})
}

func TestMemberAccessExpr_Children(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		ma := &MemberAccessExpr{
			Object: &Ident{Name: "obj"},
			Member: &Ident{Name: "field"},
		}
		assert.Len(t, ma.Children(), 2)
	})

	t.Run("nil object", func(t *testing.T) {
		ma := &MemberAccessExpr{Member: &Ident{Name: "field"}}
		assert.Len(t, ma.Children(), 1)
	})

	t.Run("nil member", func(t *testing.T) {
		ma := &MemberAccessExpr{Object: &Ident{Name: "obj"}}
		assert.Len(t, ma.Children(), 1)
	})
}

func TestIndexExpr_Children(t *testing.T) {
	ie := &IndexExpr{
		Object:  &Ident{Name: "arr"},
		Indices: []Expr{&Literal{Value: "0"}, &Literal{Value: "1"}},
	}
	assert.Len(t, ie.Children(), 3)
}

func TestDerefExpr_Children(t *testing.T) {
	t.Run("with operand", func(t *testing.T) {
		de := &DerefExpr{Operand: &Ident{Name: "ptr"}}
		assert.Len(t, de.Children(), 1)
	})

	t.Run("nil operand", func(t *testing.T) {
		de := &DerefExpr{}
		assert.Nil(t, de.Children())
	})
}

func TestParenExpr_Children(t *testing.T) {
	t.Run("with inner", func(t *testing.T) {
		pe := &ParenExpr{Inner: &Ident{Name: "x"}}
		assert.Len(t, pe.Children(), 1)
	})

	t.Run("nil inner", func(t *testing.T) {
		pe := &ParenExpr{}
		assert.Nil(t, pe.Children())
	})
}

// --- Statement Children ---

func TestAssignStmt_Children(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		as := &AssignStmt{
			Target: &Ident{Name: "x"},
			Value:  &Literal{Value: "42"},
		}
		assert.Len(t, as.Children(), 2)
	})

	t.Run("nil target and value", func(t *testing.T) {
		as := &AssignStmt{}
		assert.Len(t, as.Children(), 0)
	})
}

func TestCallStmt_Children(t *testing.T) {
	cs := &CallStmt{
		Callee: &Ident{Name: "fb"},
		Args: []*CallArg{
			{Name: &Ident{Name: "x"}, Value: &Literal{Value: "1"}},
		},
	}
	assert.Len(t, cs.Children(), 2)
}

func TestCallArg_Children(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		ca := &CallArg{
			Name:  &Ident{Name: "x"},
			Value: &Literal{Value: "1"},
		}
		assert.Len(t, ca.Children(), 2)
	})

	t.Run("no name", func(t *testing.T) {
		ca := &CallArg{Value: &Literal{Value: "1"}}
		assert.Len(t, ca.Children(), 1)
	})

	t.Run("nil value", func(t *testing.T) {
		ca := &CallArg{Name: &Ident{Name: "x"}}
		assert.Len(t, ca.Children(), 1)
	})
}

func TestIfStmt_Children(t *testing.T) {
	is := &IfStmt{
		Condition: &Ident{Name: "flag"},
		Then:      []Statement{&ReturnStmt{}},
		ElsIfs:    []*ElsIf{{Condition: &Ident{Name: "other"}}},
		Else:      []Statement{&ExitStmt{}},
	}
	assert.Len(t, is.Children(), 4)
}

func TestElsIf_Children(t *testing.T) {
	ei := &ElsIf{
		Condition: &Ident{Name: "cond"},
		Body:      []Statement{&ReturnStmt{}},
	}
	assert.Len(t, ei.Children(), 2)
}

func TestCaseStmt_Children(t *testing.T) {
	cs := &CaseStmt{
		Expr: &Ident{Name: "x"},
		Branches: []*CaseBranch{{
			Labels: []CaseLabel{&CaseLabelValue{Value: &Literal{Value: "1"}}},
			Body:   []Statement{&ReturnStmt{}},
		}},
		ElseBranch: []Statement{&ExitStmt{}},
	}
	children := cs.Children()
	assert.Len(t, children, 3) // expr + branch + else stmt
}

func TestCaseBranch_Children(t *testing.T) {
	cb := &CaseBranch{
		Labels: []CaseLabel{
			&CaseLabelValue{Value: &Literal{Value: "1"}},
			&CaseLabelRange{Low: &Literal{Value: "2"}, High: &Literal{Value: "5"}},
		},
		Body: []Statement{&ReturnStmt{}},
	}
	assert.Len(t, cb.Children(), 3) // 2 labels + 1 stmt
}

func TestCaseLabelValue_Children(t *testing.T) {
	t.Run("with value", func(t *testing.T) {
		clv := &CaseLabelValue{Value: &Literal{Value: "1"}}
		assert.Len(t, clv.Children(), 1)
	})

	t.Run("nil value", func(t *testing.T) {
		clv := &CaseLabelValue{}
		assert.Nil(t, clv.Children())
	})
}

func TestCaseLabelRange_Children(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		clr := &CaseLabelRange{
			Low:  &Literal{Value: "1"},
			High: &Literal{Value: "10"},
		}
		assert.Len(t, clr.Children(), 2)
	})

	t.Run("nil low", func(t *testing.T) {
		clr := &CaseLabelRange{High: &Literal{Value: "10"}}
		assert.Len(t, clr.Children(), 1)
	})
}

func TestForStmt_Children(t *testing.T) {
	t.Run("full with BY", func(t *testing.T) {
		fs := &ForStmt{
			Variable: &Ident{Name: "i"},
			From:     &Literal{Value: "0"},
			To:       &Literal{Value: "10"},
			By:       &Literal{Value: "2"},
			Body:     []Statement{&ReturnStmt{}},
		}
		assert.Len(t, fs.Children(), 5)
	})

	t.Run("without BY", func(t *testing.T) {
		fs := &ForStmt{
			Variable: &Ident{Name: "i"},
			From:     &Literal{Value: "0"},
			To:       &Literal{Value: "10"},
		}
		assert.Len(t, fs.Children(), 3)
	})
}

func TestWhileStmt_Children(t *testing.T) {
	ws := &WhileStmt{
		Condition: &Ident{Name: "flag"},
		Body:      []Statement{&ReturnStmt{}},
	}
	assert.Len(t, ws.Children(), 2)
}

func TestRepeatStmt_Children(t *testing.T) {
	rs := &RepeatStmt{
		Body:      []Statement{&ReturnStmt{}},
		Condition: &Ident{Name: "done"},
	}
	assert.Len(t, rs.Children(), 2)
}

func TestReturnStmt_Children(t *testing.T) {
	assert.Nil(t, (&ReturnStmt{}).Children())
}

func TestExitStmt_Children(t *testing.T) {
	assert.Nil(t, (&ExitStmt{}).Children())
}

func TestContinueStmt_Children(t *testing.T) {
	assert.Nil(t, (&ContinueStmt{}).Children())
}

func TestEmptyStmt_Children(t *testing.T) {
	assert.Nil(t, (&EmptyStmt{}).Children())
}

// --- Type spec Children ---

func TestNamedType_Children(t *testing.T) {
	t.Run("with name", func(t *testing.T) {
		nt := &NamedType{Name: &Ident{Name: "INT"}}
		assert.Len(t, nt.Children(), 1)
	})

	t.Run("nil name", func(t *testing.T) {
		nt := &NamedType{}
		assert.Nil(t, nt.Children())
	})
}

func TestArrayType_Children(t *testing.T) {
	at := &ArrayType{
		Ranges: []*SubrangeSpec{
			{Low: &Literal{Value: "0"}, High: &Literal{Value: "9"}},
		},
		ElementType: &NamedType{Name: &Ident{Name: "INT"}},
	}
	assert.Len(t, at.Children(), 2)
}

func TestSubrangeSpec_Children(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		ss := &SubrangeSpec{
			Low:  &Literal{Value: "0"},
			High: &Literal{Value: "9"},
		}
		assert.Len(t, ss.Children(), 2)
	})

	t.Run("nil low", func(t *testing.T) {
		ss := &SubrangeSpec{High: &Literal{Value: "9"}}
		assert.Len(t, ss.Children(), 1)
	})
}

func TestPointerType_Children(t *testing.T) {
	t.Run("with base", func(t *testing.T) {
		pt := &PointerType{BaseType: &NamedType{Name: &Ident{Name: "INT"}}}
		assert.Len(t, pt.Children(), 1)
	})

	t.Run("nil base", func(t *testing.T) {
		pt := &PointerType{}
		assert.Nil(t, pt.Children())
	})
}

func TestReferenceType_Children(t *testing.T) {
	t.Run("with base", func(t *testing.T) {
		rt := &ReferenceType{BaseType: &NamedType{Name: &Ident{Name: "INT"}}}
		assert.Len(t, rt.Children(), 1)
	})

	t.Run("nil base", func(t *testing.T) {
		rt := &ReferenceType{}
		assert.Nil(t, rt.Children())
	})
}

func TestStringType_Children(t *testing.T) {
	t.Run("with length", func(t *testing.T) {
		st := &StringType{Length: &Literal{Value: "255"}}
		assert.Len(t, st.Children(), 1)
	})

	t.Run("no length", func(t *testing.T) {
		st := &StringType{}
		assert.Nil(t, st.Children())
	})
}

func TestSubrangeType_Children(t *testing.T) {
	srt := &SubrangeType{
		BaseType: &NamedType{Name: &Ident{Name: "INT"}},
		Low:      &Literal{Value: "0"},
		High:     &Literal{Value: "100"},
	}
	assert.Len(t, srt.Children(), 3)
}

func TestEnumType_Children(t *testing.T) {
	t.Run("with base type", func(t *testing.T) {
		et := &EnumType{
			BaseType: &NamedType{Name: &Ident{Name: "INT"}},
			Values:   []*EnumValue{{Name: &Ident{Name: "Red"}}},
		}
		assert.Len(t, et.Children(), 2)
	})

	t.Run("no base type", func(t *testing.T) {
		et := &EnumType{
			Values: []*EnumValue{{Name: &Ident{Name: "Red"}}},
		}
		assert.Len(t, et.Children(), 1)
	})
}

func TestEnumValue_Children(t *testing.T) {
	t.Run("with init value", func(t *testing.T) {
		ev := &EnumValue{
			Name:  &Ident{Name: "Red"},
			Value: &Literal{Value: "0"},
		}
		assert.Len(t, ev.Children(), 2)
	})

	t.Run("no init value", func(t *testing.T) {
		ev := &EnumValue{Name: &Ident{Name: "Red"}}
		assert.Len(t, ev.Children(), 1)
	})
}

func TestStructType_Children(t *testing.T) {
	st := &StructType{
		Members: []*StructMember{
			{Name: &Ident{Name: "x"}, Type: &NamedType{Name: &Ident{Name: "INT"}}},
		},
	}
	assert.Len(t, st.Children(), 1)
}

func TestStructMember_Children(t *testing.T) {
	t.Run("with init value", func(t *testing.T) {
		sm := &StructMember{
			Name:      &Ident{Name: "x"},
			Type:      &NamedType{Name: &Ident{Name: "INT"}},
			InitValue: &Literal{Value: "0"},
		}
		assert.Len(t, sm.Children(), 3)
	})

	t.Run("no init value", func(t *testing.T) {
		sm := &StructMember{
			Name: &Ident{Name: "x"},
			Type: &NamedType{Name: &Ident{Name: "INT"}},
		}
		assert.Len(t, sm.Children(), 2)
	})
}

func TestVarBlock_Children(t *testing.T) {
	vb := &VarBlock{
		Declarations: []*VarDecl{
			{Names: []*Ident{{Name: "x"}}},
			{Names: []*Ident{{Name: "y"}}},
		},
	}
	assert.Len(t, vb.Children(), 2)
}

func TestVarDecl_Children(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		vd := &VarDecl{
			Names:     []*Ident{{Name: "x"}, {Name: "y"}},
			Type:      &NamedType{Name: &Ident{Name: "INT"}},
			InitValue: &Literal{Value: "0"},
			AtAddress: &Ident{Name: "%IX0.0"},
		}
		assert.Len(t, vd.Children(), 5) // 2 names + type + init + at
	})

	t.Run("minimal", func(t *testing.T) {
		vd := &VarDecl{
			Names: []*Ident{{Name: "x"}},
		}
		assert.Len(t, vd.Children(), 1)
	})
}

func TestPragmaNode_Children(t *testing.T) {
	pn := &PragmaNode{Text: "{attribute 'qualified_only'}"}
	assert.Nil(t, pn.Children())
}

// --- Inspect with early termination ---

func TestInspect_EarlyTermination(t *testing.T) {
	sf := &SourceFile{
		NodeBase: NodeBase{NodeKind: KindSourceFile},
		Declarations: []Declaration{
			&ProgramDecl{
				NodeBase: NodeBase{NodeKind: KindProgramDecl},
				Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "Main"},
				Body: []Statement{
					&AssignStmt{
						NodeBase: NodeBase{NodeKind: KindAssignStmt},
						Target:   &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "x"},
						Value:    &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitInt, Value: "1"},
					},
				},
			},
		},
	}

	// Stop descending at ProgramDecl level
	var count int
	Inspect(sf, func(n Node) bool {
		count++
		if n.Kind() == KindProgramDecl {
			return false // don't descend
		}
		return true
	})
	// Only SourceFile + ProgramDecl visited (children of ProgramDecl skipped)
	assert.Equal(t, 2, count)
}

func TestInspect_NilNode(t *testing.T) {
	var count int
	Inspect(nil, func(n Node) bool {
		count++
		return true
	})
	assert.Equal(t, 0, count)
}

// --- Walk with Visitor ---

type countingVisitor struct {
	count int
}

func (v *countingVisitor) Visit(node Node) Visitor {
	v.count++
	return v
}

func TestWalk_CountsNodes(t *testing.T) {
	sf := &SourceFile{
		Declarations: []Declaration{
			&ProgramDecl{
				Name: &Ident{Name: "Main"},
				Body: []Statement{
					&AssignStmt{
						Target: &Ident{Name: "x"},
						Value:  &Literal{Value: "42"},
					},
				},
			},
		},
	}

	v := &countingVisitor{}
	Walk(v, sf)
	// SourceFile -> ProgramDecl -> Ident(Main), AssignStmt -> Ident(x), Literal
	assert.Equal(t, 6, v.count)
}

func TestWalk_NilNode(t *testing.T) {
	v := &countingVisitor{}
	Walk(v, nil)
	assert.Equal(t, 0, v.count)
}

type stoppingVisitor struct {
	count   int
	stopAt  NodeKind
}

func (v *stoppingVisitor) Visit(node Node) Visitor {
	v.count++
	if node.Kind() == v.stopAt {
		return nil // stop
	}
	return v
}

func TestWalk_StopVisiting(t *testing.T) {
	sf := &SourceFile{
		NodeBase: NodeBase{NodeKind: KindSourceFile},
		Declarations: []Declaration{
			&ProgramDecl{
				NodeBase: NodeBase{NodeKind: KindProgramDecl},
				Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "Main"},
			},
		},
	}

	v := &stoppingVisitor{stopAt: KindProgramDecl}
	Walk(v, sf)
	// SourceFile visited (returns v), ProgramDecl visited (returns nil, so children skipped)
	assert.Equal(t, 2, v.count)
}

// --- JSON MarshalJSON for custom types ---

func TestNodeKind_MarshalJSON(t *testing.T) {
	data, err := KindIdent.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `"Ident"`, string(data))
}

func TestVarSection_MarshalJSON(t *testing.T) {
	data, err := VarInput.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `"VAR_INPUT"`, string(data))
}

func TestAccessModifier_MarshalJSON(t *testing.T) {
	data, err := AccessPublic.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `"PUBLIC"`, string(data))
}

func TestLiteralKind_MarshalJSON(t *testing.T) {
	data, err := LitBool.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `"Bool"`, string(data))
}

func TestTriviaKind_MarshalJSON(t *testing.T) {
	data, err := TriviaBlockComment.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `"BlockComment"`, string(data))
}
