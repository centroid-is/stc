package ast

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshalSourceFile(t *testing.T) {
	// Build: SourceFile -> ProgramDecl with one VarBlock and one AssignStmt
	sf := &SourceFile{
		NodeBase: NodeBase{NodeKind: KindSourceFile},
		Declarations: []Declaration{
			&ProgramDecl{
				NodeBase: NodeBase{NodeKind: KindProgramDecl},
				Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "Main"},
				VarBlocks: []*VarBlock{
					{
						NodeBase: NodeBase{NodeKind: KindVarBlock},
						Section:  VarLocal,
						Declarations: []*VarDecl{
							{
								NodeBase: NodeBase{NodeKind: KindVarDecl},
								Names:    []*Ident{{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "x"}},
								Type:     &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "INT"}},
							},
						},
					},
				},
				Body: []Statement{
					&AssignStmt{
						NodeBase: NodeBase{NodeKind: KindAssignStmt},
						Target:   &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "x"},
						Value:    &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitInt, Value: "42"},
					},
				},
			},
		},
	}

	data, err := MarshalNode(sf)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	require.Equal(t, "SourceFile", result["kind"])

	decls := result["declarations"].([]interface{})
	require.Len(t, decls, 1)

	prog := decls[0].(map[string]interface{})
	require.Equal(t, "ProgramDecl", prog["kind"])

	varBlocks := prog["var_blocks"].([]interface{})
	require.Len(t, varBlocks, 1)

	vb := varBlocks[0].(map[string]interface{})
	require.Equal(t, "VAR", vb["section"])

	body := prog["body"].([]interface{})
	require.Len(t, body, 1)
	assign := body[0].(map[string]interface{})
	require.Equal(t, "AssignStmt", assign["kind"])
}

func TestMarshalExpr(t *testing.T) {
	// Build: BinaryExpr(Literal(1) + Literal(2))
	expr := &BinaryExpr{
		NodeBase: NodeBase{NodeKind: KindBinaryExpr},
		Left:     &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitInt, Value: "1"},
		Op:       Token{Text: "+"},
		Right:    &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitInt, Value: "2"},
	}

	data, err := MarshalNode(expr)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	require.Equal(t, "BinaryExpr", result["kind"])
	require.Equal(t, "+", result["op"])

	left := result["left"].(map[string]interface{})
	require.Equal(t, "Literal", left["kind"])
	require.Equal(t, "1", left["value"])

	right := result["right"].(map[string]interface{})
	require.Equal(t, "Literal", right["kind"])
	require.Equal(t, "2", right["value"])
}

func TestMarshalOOP(t *testing.T) {
	// Build: FunctionBlockDecl with Extends, Implements, MethodDecl
	fb := &FunctionBlockDecl{
		NodeBase:   NodeBase{NodeKind: KindFunctionBlockDecl},
		Name:       &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "FB_Motor"},
		Extends:    &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "FB_Base"},
		Implements: []*Ident{{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "IMotor"}},
		Methods: []*MethodDecl{
			{
				NodeBase:       NodeBase{NodeKind: KindMethodDecl},
				AccessModifier: AccessPublic,
				Name:           &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "Start"},
				ReturnType:     &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "BOOL"}},
			},
		},
	}

	data, err := MarshalNode(fb)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	require.Equal(t, "FunctionBlockDecl", result["kind"])

	name := result["name"].(map[string]interface{})
	require.Equal(t, "FB_Motor", name["name"])

	extends := result["extends"].(map[string]interface{})
	require.Equal(t, "FB_Base", extends["name"])

	impls := result["implements"].([]interface{})
	require.Len(t, impls, 1)
	impl := impls[0].(map[string]interface{})
	require.Equal(t, "IMotor", impl["name"])

	methods := result["methods"].([]interface{})
	require.Len(t, methods, 1)
	method := methods[0].(map[string]interface{})
	require.Equal(t, "MethodDecl", method["kind"])
	require.Equal(t, "PUBLIC", method["access_modifier"])

	methodName := method["name"].(map[string]interface{})
	require.Equal(t, "Start", methodName["name"])

	retType := method["return_type"].(map[string]interface{})
	require.Equal(t, "NamedType", retType["kind"])
}

func TestMarshalTypes(t *testing.T) {
	t.Run("ArrayType", func(t *testing.T) {
		arr := &ArrayType{
			NodeBase: NodeBase{NodeKind: KindArrayType},
			Ranges: []*SubrangeSpec{
				{
					NodeBase: NodeBase{},
					Low:      &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitInt, Value: "0"},
					High:     &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitInt, Value: "9"},
				},
			},
			ElementType: &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "INT"}},
		}

		data, err := MarshalNode(arr)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		require.Equal(t, "ArrayType", result["kind"])
		require.NotNil(t, result["ranges"])
		require.NotNil(t, result["element_type"])
	})

	t.Run("PointerType", func(t *testing.T) {
		ptr := &PointerType{
			NodeBase: NodeBase{NodeKind: KindPointerType},
			BaseType: &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "INT"}},
		}

		data, err := MarshalNode(ptr)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		require.Equal(t, "PointerType", result["kind"])
		require.NotNil(t, result["base_type"])
	})

	t.Run("StructType", func(t *testing.T) {
		st := &StructType{
			NodeBase: NodeBase{NodeKind: KindStructType},
			Members: []*StructMember{
				{
					NodeBase: NodeBase{},
					Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "x"},
					Type:     &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "REAL"}},
				},
				{
					NodeBase:  NodeBase{},
					Name:      &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "y"},
					Type:      &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "REAL"}},
					InitValue: &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitReal, Value: "0.0"},
				},
			},
		}

		data, err := MarshalNode(st)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		require.Equal(t, "StructType", result["kind"])
		members := result["members"].([]interface{})
		require.Len(t, members, 2)

		m1 := members[0].(map[string]interface{})
		mName := m1["name"].(map[string]interface{})
		require.Equal(t, "x", mName["name"])
	})

	t.Run("EnumType", func(t *testing.T) {
		en := &EnumType{
			NodeBase: NodeBase{NodeKind: KindEnumType},
			Values: []*EnumValue{
				{
					NodeBase: NodeBase{},
					Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "Red"},
				},
				{
					NodeBase: NodeBase{},
					Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "Green"},
					Value:    &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitInt, Value: "1"},
				},
			},
		}

		data, err := MarshalNode(en)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		require.Equal(t, "EnumType", result["kind"])
		vals := result["values"].([]interface{})
		require.Len(t, vals, 2)
	})
}

func TestMarshalTrivia(t *testing.T) {
	// Verify trivia is preserved in JSON output
	ident := &Ident{
		NodeBase: NodeBase{
			NodeKind: KindIdent,
			LeadingTrivia: []Trivia{
				{Kind: TriviaLineComment, Text: "// comment", Span: Span{}},
			},
		},
		Name: "x",
	}

	data, err := MarshalNode(ident)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	require.Equal(t, "Ident", result["kind"])
	trivia := result["leading_trivia"].([]interface{})
	require.Len(t, trivia, 1)

	t0 := trivia[0].(map[string]interface{})
	require.Equal(t, "LineComment", t0["kind"])
	require.Equal(t, "// comment", t0["text"])
}

func TestWalkAndInspect(t *testing.T) {
	// Build a small tree and count nodes via Inspect
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

	var count int
	Inspect(sf, func(n Node) bool {
		count++
		return true
	})

	// SourceFile -> ProgramDecl -> Ident("Main"), AssignStmt -> Ident("x"), Literal(1)
	require.Equal(t, 6, count)
}
