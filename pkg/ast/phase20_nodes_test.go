package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func p20Ident(name string) *Ident {
	return &Ident{NodeBase: NodeBase{NodeKind: KindIdent}, Name: name}
}

func p20Int(v string) *Literal {
	return &Literal{NodeBase: NodeBase{NodeKind: KindLiteral}, LitKind: LitInt, Value: v}
}

func TestPhase20Nodes(t *testing.T) {
	t.Run("BitAccessExpr", func(t *testing.T) {
		target, idx := p20Ident("w"), p20Int("3")
		n := &BitAccessExpr{NodeBase: NodeBase{NodeKind: KindBitAccessExpr}, Target: target, Index: idx}
		m := marshalToMap(t, n)
		assert.Equal(t, "BitAccessExpr", m["kind"])
		assert.Equal(t, "w", m["target"].(map[string]interface{})["name"])
		assert.Equal(t, "3", m["index"].(map[string]interface{})["value"])
		assert.Equal(t, []Node{target, idx}, n.Children())
		assert.Nil(t, (&BitAccessExpr{}).Children())
		var _ Expr = n
	})

	t.Run("BitAccessExpr kind forced when NodeKind unset", func(t *testing.T) {
		m := marshalToMap(t, &BitAccessExpr{Target: p20Ident("w"), Index: p20Int("0")})
		assert.Equal(t, "BitAccessExpr", m["kind"])
	})

	t.Run("CallExpr positional only unchanged", func(t *testing.T) {
		n := &CallExpr{NodeBase: NodeBase{NodeKind: KindCallExpr}, Callee: p20Ident("F"), Args: []Expr{p20Int("1")}}
		m := marshalToMap(t, n)
		assert.Equal(t, "CallExpr", m["kind"])
		assert.Len(t, m["args"], 1)
		_, has := m["named_args"]
		assert.False(t, has)
	})

	t.Run("CallExpr named args in source order", func(t *testing.T) {
		a1 := &CallArg{NodeBase: NodeBase{NodeKind: KindCallArg}, Name: p20Ident("b"), Value: p20Int("2")}
		a2 := &CallArg{NodeBase: NodeBase{NodeKind: KindCallArg}, Name: p20Ident("c"), Value: p20Ident("x"), IsOutput: true}
		a3 := &CallArg{NodeBase: NodeBase{NodeKind: KindCallArg}, Value: p20Int("4")}
		callee, pos := p20Ident("F"), p20Int("1")
		n := &CallExpr{Callee: callee, Args: []Expr{pos}, NamedArgs: []*CallArg{a1, a2, a3}}
		m := marshalToMap(t, n)
		named := m["named_args"].([]interface{})
		require.Len(t, named, 3)
		assert.Equal(t, "b", named[0].(map[string]interface{})["name"].(map[string]interface{})["name"])
		assert.Equal(t, true, named[1].(map[string]interface{})["is_output"])
		_, hasName := named[2].(map[string]interface{})["name"]
		assert.False(t, hasName, "positional after named has no name")
		assert.Equal(t, []Node{callee, pos, a1, a2, a3}, n.Children())
	})

	t.Run("CallArg kind", func(t *testing.T) {
		m := marshalToMap(t, &CallArg{NodeBase: NodeBase{NodeKind: KindCallArg}, Name: p20Ident("a"), Value: p20Int("1")})
		assert.Equal(t, "CallArg", m["kind"])
		// Older parsers left NodeKind unset; JSON still reports CallArg.
		m = marshalToMap(t, &CallArg{Name: p20Ident("a")})
		assert.Equal(t, "CallArg", m["kind"])
	})

	t.Run("RefAssignStmt", func(t *testing.T) {
		target, val := p20Ident("r"), p20Ident("x")
		n := &RefAssignStmt{NodeBase: NodeBase{NodeKind: KindRefAssignStmt}, Target: target, Value: val}
		m := marshalToMap(t, n)
		assert.Equal(t, "RefAssignStmt", m["kind"])
		assert.Equal(t, "r", m["target"].(map[string]interface{})["name"])
		assert.Equal(t, "x", m["value"].(map[string]interface{})["name"])
		assert.Equal(t, []Node{target, val}, n.Children())
		assert.Nil(t, (&RefAssignStmt{}).Children())
		var _ Statement = n
		assert.Equal(t, "RefAssignStmt", marshalToMap(t, &RefAssignStmt{})["kind"])
	})

	t.Run("ThisExpr and SuperExpr", func(t *testing.T) {
		th := &ThisExpr{NodeBase: NodeBase{NodeKind: KindThisExpr}}
		su := &SuperExpr{NodeBase: NodeBase{NodeKind: KindSuperExpr}}
		assert.Equal(t, "ThisExpr", marshalToMap(t, th)["kind"])
		assert.Equal(t, "SuperExpr", marshalToMap(t, su)["kind"])
		assert.Equal(t, "ThisExpr", marshalToMap(t, &ThisExpr{})["kind"])
		assert.Equal(t, "SuperExpr", marshalToMap(t, &SuperExpr{})["kind"])
		assert.Nil(t, th.Children())
		assert.Nil(t, su.Children())
		var _ Expr = th
		var _ Expr = su
		d := marshalToMap(t, &DerefExpr{NodeBase: NodeBase{NodeKind: KindDerefExpr}, Operand: th})
		assert.Equal(t, "ThisExpr", d["operand"].(map[string]interface{})["kind"])
	})

	t.Run("StructInit and FieldInit", func(t *testing.T) {
		fname, fval := p20Ident("a"), p20Int("1")
		fi := &FieldInit{NodeBase: NodeBase{NodeKind: KindFieldInit}, Name: fname, Value: fval}
		si := &StructInit{NodeBase: NodeBase{NodeKind: KindStructInit}, Fields: []*FieldInit{fi}}
		m := marshalToMap(t, si)
		assert.Equal(t, "StructInit", m["kind"])
		fields := m["fields"].([]interface{})
		require.Len(t, fields, 1)
		f0 := fields[0].(map[string]interface{})
		assert.Equal(t, "FieldInit", f0["kind"])
		assert.Equal(t, "a", f0["name"].(map[string]interface{})["name"])
		assert.Equal(t, "1", f0["value"].(map[string]interface{})["value"])
		assert.Equal(t, []Node{fi}, si.Children())
		assert.Equal(t, []Node{fname, fval}, fi.Children())
		assert.Nil(t, (&FieldInit{}).Children())
		assert.Empty(t, (&StructInit{}).Children())
		_, has := marshalToMap(t, &StructInit{})["fields"]
		assert.False(t, has)
		assert.Equal(t, "StructInit", marshalToMap(t, &StructInit{})["kind"])
		assert.Equal(t, "FieldInit", marshalToMap(t, &FieldInit{})["kind"])
		var _ Expr = si
	})

	t.Run("ArrayInit and ArrayInitElem", func(t *testing.T) {
		plain := &ArrayInitElem{NodeBase: NodeBase{NodeKind: KindArrayInitElem}, Value: p20Int("1")}
		count, val := p20Int("3"), p20Int("0")
		rep := &ArrayInitElem{NodeBase: NodeBase{NodeKind: KindArrayInitElem}, Count: count, Value: val}
		ai := &ArrayInit{NodeBase: NodeBase{NodeKind: KindArrayInit}, Elements: []*ArrayInitElem{plain, rep}}
		m := marshalToMap(t, ai)
		assert.Equal(t, "ArrayInit", m["kind"])
		elems := m["elements"].([]interface{})
		require.Len(t, elems, 2)
		e0 := elems[0].(map[string]interface{})
		assert.Equal(t, "ArrayInitElem", e0["kind"])
		_, hasCount := e0["count"]
		assert.False(t, hasCount, "count absent when nil")
		e1 := elems[1].(map[string]interface{})
		assert.Equal(t, "3", e1["count"].(map[string]interface{})["value"])
		assert.Equal(t, "0", e1["value"].(map[string]interface{})["value"])
		assert.Equal(t, []Node{plain, rep}, ai.Children())
		assert.Equal(t, []Node{count, val}, rep.Children())
		assert.Nil(t, (&ArrayInitElem{}).Children())
		assert.Empty(t, (&ArrayInit{}).Children())
		_, has := marshalToMap(t, &ArrayInit{})["elements"]
		assert.False(t, has)
		assert.Equal(t, "ArrayInit", marshalToMap(t, &ArrayInit{})["kind"])
		assert.Equal(t, "ArrayInitElem", marshalToMap(t, &ArrayInitElem{})["kind"])
		var _ Expr = ai
	})

	t.Run("NamedType namespace", func(t *testing.T) {
		ns, name := p20Ident("Tc2_EtherCAT"), p20Ident("ST_EcSlaveState")
		n := &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Namespace: ns, Name: name}
		m := marshalToMap(t, n)
		assert.Equal(t, "Tc2_EtherCAT", m["namespace"].(map[string]interface{})["name"])
		assert.Equal(t, []Node{ns, name}, n.Children())
		plain := marshalToMap(t, &NamedType{NodeBase: NodeBase{NodeKind: KindNamedType}, Name: name})
		_, has := plain["namespace"]
		assert.False(t, has)
	})

	t.Run("TypeDecl init value", func(t *testing.T) {
		name, init := p20Ident("E"), p20Ident("b")
		typ := &EnumType{NodeBase: NodeBase{NodeKind: KindEnumType}}
		n := &TypeDecl{NodeBase: NodeBase{NodeKind: KindTypeDecl}, Name: name, Type: typ, InitValue: init}
		m := marshalToMap(t, n)
		assert.Equal(t, "b", m["init_value"].(map[string]interface{})["name"])
		assert.Equal(t, []Node{name, typ, init}, n.Children())
		_, has := marshalToMap(t, &TypeDecl{NodeBase: NodeBase{NodeKind: KindTypeDecl}, Name: name})["init_value"]
		assert.False(t, has)
	})

	t.Run("NodeKind names and stability", func(t *testing.T) {
		assert.Equal(t, KindVarDecl+3, KindPragma)
		kinds := []struct {
			k    NodeKind
			name string
		}{
			{KindBitAccessExpr, "BitAccessExpr"},
			{KindThisExpr, "ThisExpr"},
			{KindSuperExpr, "SuperExpr"},
			{KindRefAssignStmt, "RefAssignStmt"},
			{KindCallArg, "CallArg"},
			{KindStructInit, "StructInit"},
			{KindFieldInit, "FieldInit"},
			{KindArrayInit, "ArrayInit"},
			{KindArrayInitElem, "ArrayInitElem"},
		}
		for i, kc := range kinds {
			assert.Equal(t, KindPragma+NodeKind(i+1), kc.k, kc.name)
			assert.Equal(t, kc.name, kc.k.String())
		}
	})
}
