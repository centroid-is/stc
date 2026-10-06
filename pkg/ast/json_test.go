package ast

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
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

func marshalToMap(t *testing.T, n Node) map[string]interface{} {
	t.Helper()
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	return m
}

func TestMarshalAttributes(t *testing.T) {
	withVal := &Attribute{NodeBase: NodeBase{NodeKind: KindAttribute}, Name: "OPC.UA.DA", Value: "1", HasValue: true}
	noVal := &Attribute{Name: "qualified_only"} // zero NodeKind must still marshal as Attribute
	pragma := &PragmaNode{Text: "{warning disable C0139}"}
	boolT := &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "BOOL"}}

	nodes := map[string]Node{
		"VarDecl":           &VarDecl{NodeBase: NodeBase{NodeKind: KindVarDecl}, Type: boolT},
		"VarBlock":          &VarBlock{NodeBase: NodeBase{NodeKind: KindVarBlock}},
		"StructMember":      &StructMember{NodeBase: NodeBase{NodeKind: 0}},
		"EnumValue":         &EnumValue{},
		"TypeDecl":          &TypeDecl{NodeBase: NodeBase{NodeKind: KindTypeDecl}},
		"ProgramDecl":       &ProgramDecl{NodeBase: NodeBase{NodeKind: KindProgramDecl}},
		"FunctionBlockDecl": &FunctionBlockDecl{NodeBase: NodeBase{NodeKind: KindFunctionBlockDecl}},
		"FunctionDecl":      &FunctionDecl{NodeBase: NodeBase{NodeKind: KindFunctionDecl}},
		"MethodDecl":        &MethodDecl{NodeBase: NodeBase{NodeKind: KindMethodDecl}},
		"PropertyDecl":      &PropertyDecl{NodeBase: NodeBase{NodeKind: KindPropertyDecl}},
		"InterfaceDecl":     &InterfaceDecl{NodeBase: NodeBase{NodeKind: KindInterfaceDecl}},
		"ActionDecl":        &ActionDecl{NodeBase: NodeBase{NodeKind: KindActionDecl}},
		"GVLDecl":           &GVLDecl{NodeBase: NodeBase{NodeKind: KindGVLDecl}},
	}
	set := func(n Node) {
		a := []*Attribute{withVal, noVal}
		p := []*PragmaNode{pragma}
		switch v := n.(type) {
		case *VarDecl:
			v.Attributes, v.Pragmas = a, p
		case *VarBlock:
			v.Attributes, v.Pragmas = a, p
		case *StructMember:
			v.Attributes, v.Pragmas = a, p
		case *EnumValue:
			v.Attributes, v.Pragmas = a, p
		case *TypeDecl:
			v.Attributes, v.Pragmas = a, p
		case *ProgramDecl:
			v.Attributes, v.Pragmas = a, p
		case *FunctionBlockDecl:
			v.Attributes, v.Pragmas = a, p
		case *FunctionDecl:
			v.Attributes, v.Pragmas = a, p
		case *MethodDecl:
			v.Attributes, v.Pragmas = a, p
		case *PropertyDecl:
			v.Attributes, v.Pragmas = a, p
		case *InterfaceDecl:
			v.Attributes, v.Pragmas = a, p
		case *ActionDecl:
			v.Attributes, v.Pragmas = a, p
		case *GVLDecl:
			v.Attributes, v.Pragmas = a, p
		}
	}
	for name, n := range nodes {
		t.Run(name, func(t *testing.T) {
			set(n)
			m := marshalToMap(t, n)
			attrs, ok := m["attributes"].([]interface{})
			require.True(t, ok, "attributes array missing")
			require.Len(t, attrs, 2)
			a0 := attrs[0].(map[string]interface{})
			assert.Equal(t, "Attribute", a0["kind"])
			assert.Equal(t, "OPC.UA.DA", a0["name"])
			assert.Equal(t, "1", a0["value"])
			a1 := attrs[1].(map[string]interface{})
			assert.Equal(t, "Attribute", a1["kind"])
			assert.Equal(t, "qualified_only", a1["name"])
			assert.NotContains(t, a1, "value")
			prs, ok := m["pragmas"].([]interface{})
			require.True(t, ok, "pragmas array missing")
			require.Len(t, prs, 1)
			assert.Equal(t, "{warning disable C0139}", prs[0].(map[string]interface{})["text"])
		})
	}

	t.Run("empty value is kept", func(t *testing.T) {
		m := marshalToMap(t, &Attribute{Name: "x", HasValue: true})
		assert.Contains(t, m, "value")
		assert.Equal(t, "", m["value"])
	})

	t.Run("absent when empty", func(t *testing.T) {
		m := marshalToMap(t, &VarDecl{NodeBase: NodeBase{NodeKind: KindVarDecl}})
		assert.NotContains(t, m, "attributes")
		assert.NotContains(t, m, "pragmas")
	})
}

func TestMarshalGVLDecl(t *testing.T) {
	g := &GVLDecl{
		NodeBase: NodeBase{NodeKind: KindGVLDecl},
		Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "ECT"},
		Blocks:   []*VarBlock{{NodeBase: NodeBase{NodeKind: KindVarBlock}, Section: VarGlobal}},
	}
	m := marshalToMap(t, g)
	assert.Equal(t, "GVLDecl", m["kind"])
	assert.Equal(t, "ECT", m["name"].(map[string]interface{})["name"])
	blocks, ok := m["blocks"].([]interface{})
	require.True(t, ok)
	require.Len(t, blocks, 1)
	assert.Equal(t, "VAR_GLOBAL", blocks[0].(map[string]interface{})["section"])

	empty := marshalToMap(t, &GVLDecl{NodeBase: NodeBase{NodeKind: KindGVLDecl}})
	assert.NotContains(t, empty, "name")
	assert.NotContains(t, empty, "blocks")
}

func TestMarshalActions(t *testing.T) {
	act := &ActionDecl{NodeBase: NodeBase{NodeKind: KindActionDecl}, Name: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "Reset"}}
	for name, n := range map[string]Node{
		"ProgramDecl":       &ProgramDecl{NodeBase: NodeBase{NodeKind: KindProgramDecl}, Actions: []*ActionDecl{act}},
		"FunctionBlockDecl": &FunctionBlockDecl{NodeBase: NodeBase{NodeKind: KindFunctionBlockDecl}, Actions: []*ActionDecl{act}},
	} {
		t.Run(name, func(t *testing.T) {
			m := marshalToMap(t, n)
			acts, ok := m["actions"].([]interface{})
			require.True(t, ok)
			require.Len(t, acts, 1)
			assert.Equal(t, "ActionDecl", acts[0].(map[string]interface{})["kind"])
		})
	}
	m := marshalToMap(t, &ProgramDecl{NodeBase: NodeBase{NodeKind: KindProgramDecl}})
	assert.NotContains(t, m, "actions")
}

func TestMarshalStructMemberAT(t *testing.T) {
	sm := &StructMember{
		Name:      &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "I1"},
		AtAddress: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "%I*"},
	}
	m := marshalToMap(t, sm)
	at, ok := m["at_address"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "%I*", at["name"])

	m = marshalToMap(t, &StructMember{})
	assert.NotContains(t, m, "at_address")
}

func TestMarshalCallStmtArgs(t *testing.T) {
	ident := func(s string) *Ident { return &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: s} }
	cs := &CallStmt{
		NodeBase: NodeBase{NodeKind: KindCallStmt},
		Callee:   ident("t"),
		Args: []*CallArg{
			{Name: ident("PT")},
			{Name: ident("Q"), IsOutput: true},
		},
	}
	m := marshalToMap(t, cs)
	args, ok := m["args"].([]interface{})
	require.True(t, ok)
	require.Len(t, args, 2)
	for _, a := range args {
		assert.NotContains(t, a.(map[string]interface{}), "value")
	}
	assert.Equal(t, true, args[1].(map[string]interface{})["is_output"])

	withVal := &CallStmt{NodeBase: NodeBase{NodeKind: KindCallStmt}, Callee: ident("t"),
		Args: []*CallArg{{Name: ident("IN"), Value: ident("x")}}}
	m = marshalToMap(t, withVal)
	assert.Contains(t, m["args"].([]interface{})[0].(map[string]interface{}), "value")

	m = marshalToMap(t, &CallStmt{NodeBase: NodeBase{NodeKind: KindCallStmt}, Callee: ident("t")})
	assert.NotContains(t, m, "args")
}

func TestMarshalPragma(t *testing.T) {
	m := marshalToMap(t, &PragmaNode{Text: "{warning disable C0139}"})
	assert.Equal(t, "Pragma", m["kind"])
	assert.Equal(t, "{warning disable C0139}", m["text"])
}

func TestMarshalEndAttrs(t *testing.T) {
	attr := &Attribute{NodeBase: NodeBase{NodeKind: KindAttribute}, Name: "tail"}
	prag := &PragmaNode{NodeBase: NodeBase{NodeKind: KindPragma}, Text: "{endregion}"}
	for name, n := range map[string]Node{
		"VarBlock":   &VarBlock{NodeBase: NodeBase{NodeKind: KindVarBlock}, EndAttributes: []*Attribute{attr}, EndPragmas: []*PragmaNode{prag}},
		"StructType": &StructType{NodeBase: NodeBase{NodeKind: KindStructType}, EndAttributes: []*Attribute{attr}, EndPragmas: []*PragmaNode{prag}},
		"EnumType":   &EnumType{NodeBase: NodeBase{NodeKind: KindEnumType}, EndAttributes: []*Attribute{attr}, EndPragmas: []*PragmaNode{prag}},
	} {
		t.Run(name, func(t *testing.T) {
			m := marshalToMap(t, n)
			assert.Len(t, m["end_attributes"], 1)
			assert.Len(t, m["end_pragmas"], 1)
			assert.NotContains(t, m, "attributes")
			assert.Len(t, n.Children(), 2)
		})
	}
	m := marshalToMap(t, &StructType{NodeBase: NodeBase{NodeKind: KindStructType}})
	assert.NotContains(t, m, "end_attributes")
	assert.NotContains(t, m, "end_pragmas")
}
