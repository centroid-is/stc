package ast

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalNode_Nil(t *testing.T) {
	data, err := MarshalNode(nil)
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))
}

func TestMarshalNode_AllStatementTypes(t *testing.T) {
	tests := []struct {
		name string
		node Node
		kind string
	}{
		{"ReturnStmt", &ReturnStmt{NodeBase: NodeBase{NodeKind: KindReturnStmt}}, "ReturnStmt"},
		{"ExitStmt", &ExitStmt{NodeBase: NodeBase{NodeKind: KindExitStmt}}, "ExitStmt"},
		{"ContinueStmt", &ContinueStmt{NodeBase: NodeBase{NodeKind: KindContinueStmt}}, "ContinueStmt"},
		{"EmptyStmt", &EmptyStmt{NodeBase: NodeBase{NodeKind: KindEmptyStmt}}, "EmptyStmt"},
		{"ErrorNode", &ErrorNode{NodeBase: NodeBase{NodeKind: KindErrorNode}, Message: "test error"}, "ErrorNode"},
		{"PragmaNode", &PragmaNode{NodeBase: NodeBase{}, Text: "pragma text"}, "SourceFile"}, // NodeKind 0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := MarshalNode(tt.node)
			require.NoError(t, err)
			var m map[string]interface{}
			require.NoError(t, json.Unmarshal(data, &m))
			assert.Contains(t, m, "kind")
		})
	}
}

func TestMarshalNode_AssignStmt(t *testing.T) {
	n := &AssignStmt{
		NodeBase: NodeBase{NodeKind: KindAssignStmt},
		Target:   &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "x"},
		Value:    &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitInt, Value: "42"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "AssignStmt", m["kind"])
	assert.NotNil(t, m["target"])
	assert.NotNil(t, m["value"])
}

func TestMarshalNode_CallStmt(t *testing.T) {
	n := &CallStmt{
		NodeBase: NodeBase{NodeKind: KindCallStmt},
		Callee:   &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "fb1"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "CallStmt", m["kind"])
	assert.NotNil(t, m["callee"])
}

func TestMarshalNode_IfStmt(t *testing.T) {
	n := &IfStmt{
		NodeBase:  NodeBase{NodeKind: KindIfStmt},
		Condition: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "flag"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "IfStmt", m["kind"])
	assert.NotNil(t, m["condition"])
}

func TestMarshalNode_CaseStmt(t *testing.T) {
	n := &CaseStmt{
		NodeBase: NodeBase{NodeKind: KindCaseStmt},
		Expr:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "x"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "CaseStmt", m["kind"])
	assert.NotNil(t, m["expr"])
}

func TestMarshalNode_ForStmt(t *testing.T) {
	n := &ForStmt{
		NodeBase: NodeBase{NodeKind: KindForStmt},
		Variable: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "i"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "ForStmt", m["kind"])
	assert.NotNil(t, m["variable"])
}

func TestMarshalNode_WhileStmt(t *testing.T) {
	n := &WhileStmt{
		NodeBase:  NodeBase{NodeKind: KindWhileStmt},
		Condition: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "flag"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "WhileStmt", m["kind"])
}

func TestMarshalNode_RepeatStmt(t *testing.T) {
	n := &RepeatStmt{
		NodeBase:  NodeBase{NodeKind: KindRepeatStmt},
		Condition: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "done"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "RepeatStmt", m["kind"])
}

func TestMarshalNode_UnaryExpr(t *testing.T) {
	n := &UnaryExpr{
		NodeBase: NodeBase{NodeKind: KindUnaryExpr},
		Op:       Token{Text: "NOT"},
		Operand:  &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "flag"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "UnaryExpr", m["kind"])
	assert.Equal(t, "NOT", m["op"])
	assert.NotNil(t, m["operand"])
}

func TestMarshalNode_Literal_TypedPrefix(t *testing.T) {
	n := &Literal{
		NodeBase:   NodeBase{NodeKind: KindLiteral},
		LitKind:    LitTyped,
		Value:      "5",
		TypePrefix: "INT",
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "INT", m["type_prefix"])
}

func TestMarshalNode_CallExpr(t *testing.T) {
	n := &CallExpr{
		NodeBase: NodeBase{NodeKind: KindCallExpr},
		Callee:   &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "ADD"},
		Args: []Expr{
			&Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitInt, Value: "1"},
			&Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitInt, Value: "2"},
		},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "CallExpr", m["kind"])
	assert.Len(t, m["args"], 2)
}

func TestMarshalNode_MemberAccessExpr(t *testing.T) {
	n := &MemberAccessExpr{
		NodeBase: NodeBase{NodeKind: KindMemberAccessExpr},
		Object:   &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "obj"},
		Member:   &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "field"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "MemberAccessExpr", m["kind"])
	assert.NotNil(t, m["object"])
	assert.NotNil(t, m["member"])
}

func TestMarshalNode_IndexExpr(t *testing.T) {
	n := &IndexExpr{
		NodeBase: NodeBase{NodeKind: KindIndexExpr},
		Object:   &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "arr"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "IndexExpr", m["kind"])
}

func TestMarshalNode_DerefExpr(t *testing.T) {
	n := &DerefExpr{
		NodeBase: NodeBase{NodeKind: KindDerefExpr},
		Operand:  &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "ptr"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "DerefExpr", m["kind"])
}

func TestMarshalNode_ParenExpr(t *testing.T) {
	n := &ParenExpr{
		NodeBase: NodeBase{NodeKind: KindParenExpr},
		Inner:    &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "x"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "ParenExpr", m["kind"])
}

func TestMarshalNode_ReferenceType(t *testing.T) {
	n := &ReferenceType{
		NodeBase: NodeBase{NodeKind: KindReferenceType},
		BaseType: &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{Name: "INT"}},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "ReferenceType", m["kind"])
	assert.NotNil(t, m["base_type"])
}

func TestMarshalNode_StringType(t *testing.T) {
	t.Run("wide with length", func(t *testing.T) {
		n := &StringType{
			NodeBase: NodeBase{NodeKind: KindStringType},
			IsWide:   true,
			Length:   &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, Value: "255"},
		}
		data, err := MarshalNode(n)
		require.NoError(t, err)
		var m map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &m))
		assert.Equal(t, true, m["is_wide"])
		assert.NotNil(t, m["length"])
	})

	t.Run("plain string", func(t *testing.T) {
		n := &StringType{
			NodeBase: NodeBase{NodeKind: KindStringType},
		}
		data, err := MarshalNode(n)
		require.NoError(t, err)
		var m map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &m))
		assert.Nil(t, m["is_wide"])
		assert.Nil(t, m["length"])
	})
}

func TestMarshalNode_SubrangeType(t *testing.T) {
	n := &SubrangeType{
		NodeBase: NodeBase{NodeKind: KindSubrangeType},
		BaseType: &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{Name: "INT"}},
		Low:      &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, Value: "0"},
		High:     &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, Value: "100"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "SubrangeType", m["kind"])
}

func TestMarshalNode_CallArg(t *testing.T) {
	n := &CallArg{
		NodeBase: NodeBase{},
		Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "enable"},
		Value:    &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitBool, Value: "TRUE"},
		IsOutput: true,
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, true, m["is_output"])
}

func TestMarshalNode_ElsIf(t *testing.T) {
	n := &ElsIf{
		Condition: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "cond"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["condition"])
}

func TestMarshalNode_CaseBranch(t *testing.T) {
	n := &CaseBranch{}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Contains(t, m, "kind")
}

func TestMarshalNode_CaseLabelValue(t *testing.T) {
	n := &CaseLabelValue{
		Value: &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, Value: "1"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["value"])
}

func TestMarshalNode_CaseLabelRange(t *testing.T) {
	n := &CaseLabelRange{
		Low:  &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, Value: "1"},
		High: &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, Value: "10"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["low"])
	assert.NotNil(t, m["high"])
}

func TestMarshalNode_PragmaNode(t *testing.T) {
	n := &PragmaNode{
		NodeBase: NodeBase{},
		Text:     "{attribute 'qualified_only'}",
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "{attribute 'qualified_only'}", m["text"])
}

func TestMarshalNode_TestCaseDecl(t *testing.T) {
	n := &TestCaseDecl{
		NodeBase: NodeBase{NodeKind: KindTestCaseDecl},
		Name:     "mytest",
		VarBlocks: []*VarBlock{{
			NodeBase: NodeBase{NodeKind: KindVarBlock},
			Section:  VarLocal,
		}},
		Body: []Statement{
			&ReturnStmt{NodeBase: NodeBase{NodeKind: KindReturnStmt}},
		},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "TestCaseDecl", m["kind"])
	assert.Equal(t, "mytest", m["name"])
}

func TestMarshalNode_InterfaceDecl(t *testing.T) {
	n := &InterfaceDecl{
		NodeBase: NodeBase{NodeKind: KindInterfaceDecl},
		Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "IFoo"},
		Extends:  []*Ident{{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "IBar"}},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "InterfaceDecl", m["kind"])
	assert.NotNil(t, m["extends"])
}

func TestMarshalNode_MethodDecl_Flags(t *testing.T) {
	n := &MethodDecl{
		NodeBase:       NodeBase{NodeKind: KindMethodDecl},
		Name:           &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "M"},
		IsAbstract:     true,
		IsFinal:        true,
		IsOverride:     true,
		AccessModifier: AccessProtected,
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, true, m["is_abstract"])
	assert.Equal(t, true, m["is_final"])
	assert.Equal(t, true, m["is_override"])
	assert.Equal(t, "PROTECTED", m["access_modifier"])
}

func TestMarshalNode_PropertyDecl(t *testing.T) {
	n := &PropertyDecl{
		NodeBase:       NodeBase{NodeKind: KindPropertyDecl},
		AccessModifier: AccessPrivate,
		Name:           &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "Prop"},
		Type:           &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{Name: "INT"}},
		Getter:         &MethodDecl{NodeBase: NodeBase{NodeKind: KindMethodDecl}, Name: &Ident{Name: "Get"}},
		Setter:         &MethodDecl{NodeBase: NodeBase{NodeKind: KindMethodDecl}, Name: &Ident{Name: "Set"}},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "PropertyDecl", m["kind"])
	assert.NotNil(t, m["getter"])
	assert.NotNil(t, m["setter"])
	assert.Equal(t, "PRIVATE", m["access_modifier"])
}

func TestMarshalNode_MethodSignature(t *testing.T) {
	n := &MethodSignature{
		NodeBase:   NodeBase{},
		Name:       &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "M"},
		ReturnType: &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{Name: "BOOL"}},
		VarBlocks:  []*VarBlock{{NodeBase: NodeBase{NodeKind: KindVarBlock}, Section: VarInput}},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["return_type"])
	assert.NotNil(t, m["var_blocks"])
}

func TestMarshalNode_PropertySignature(t *testing.T) {
	n := &PropertySignature{
		NodeBase: NodeBase{},
		Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "P"},
		Type:     &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{Name: "INT"}},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["type"])
}

func TestMarshalNode_TypeDecl(t *testing.T) {
	n := &TypeDecl{
		NodeBase: NodeBase{NodeKind: KindTypeDecl},
		Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "MyType"},
		Type:     &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{Name: "INT"}},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "TypeDecl", m["kind"])
}

func TestMarshalNode_ActionDecl(t *testing.T) {
	n := &ActionDecl{
		NodeBase: NodeBase{NodeKind: KindActionDecl},
		Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "Act"},
		Body:     []Statement{&ReturnStmt{NodeBase: NodeBase{NodeKind: KindReturnStmt}}},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "ActionDecl", m["kind"])
	assert.NotNil(t, m["body"])
}

func TestMarshalNode_VarBlock_Flags(t *testing.T) {
	n := &VarBlock{
		NodeBase:     NodeBase{NodeKind: KindVarBlock},
		Section:      VarGlobal,
		IsConstant:   true,
		IsRetain:     true,
		IsPersistent: true,
		Declarations: []*VarDecl{{
			NodeBase: NodeBase{NodeKind: KindVarDecl},
			Names:    []*Ident{{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "x"}},
		}},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "VAR_GLOBAL", m["section"])
	assert.Equal(t, true, m["is_constant"])
	assert.Equal(t, true, m["is_retain"])
	assert.Equal(t, true, m["is_persistent"])
}

func TestMarshalNode_VarDecl_AtAddress(t *testing.T) {
	n := &VarDecl{
		NodeBase:  NodeBase{NodeKind: KindVarDecl},
		Names:     []*Ident{{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "x"}},
		Type:      &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{Name: "BOOL"}},
		InitValue: &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, Value: "TRUE"},
		AtAddress: &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "%IX0.0"},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["at_address"])
	assert.NotNil(t, m["init_value"])
}

func TestMarshalNode_TrailingTrivia(t *testing.T) {
	n := &Ident{
		NodeBase: NodeBase{
			NodeKind: KindIdent,
			TrailingTrivia: []Trivia{
				{Kind: TriviaBlockComment, Text: "(* trailing *)"},
			},
		},
		Name: "x",
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["trailing_trivia"])
}

func TestMarshalNode_FunctionBlockDecl_Properties(t *testing.T) {
	n := &FunctionBlockDecl{
		NodeBase: NodeBase{NodeKind: KindFunctionBlockDecl},
		Name:     &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "FB"},
		Properties: []*PropertyDecl{
			{NodeBase: NodeBase{NodeKind: KindPropertyDecl}, Name: &Ident{Name: "P1"}},
		},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.NotNil(t, m["properties"])
}

func TestMarshalNode_FunctionDecl(t *testing.T) {
	n := &FunctionDecl{
		NodeBase:   NodeBase{NodeKind: KindFunctionDecl},
		Name:       &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: "Func"},
		ReturnType: &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: &Ident{Name: "INT"}},
	}
	data, err := MarshalNode(n)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, "FunctionDecl", m["kind"])
	assert.NotNil(t, m["return_type"])
}
