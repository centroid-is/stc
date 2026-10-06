package parser

import (
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

func requireStructInit(t *testing.T, e ast.Expr, fields ...string) *ast.StructInit {
	t.Helper()
	si, ok := e.(*ast.StructInit)
	require.True(t, ok, "expected StructInit, got %T", e)
	require.Equal(t, ast.KindStructInit, si.Kind())
	require.Len(t, si.Fields, len(fields))
	for i, name := range fields {
		require.Equal(t, ast.KindFieldInit, si.Fields[i].Kind())
		require.Equal(t, name, si.Fields[i].Name.Name)
	}
	return si
}

func requireArrayInit(t *testing.T, e ast.Expr, n int) *ast.ArrayInit {
	t.Helper()
	ai, ok := e.(*ast.ArrayInit)
	require.True(t, ok, "expected ArrayInit, got %T", e)
	require.Equal(t, ast.KindArrayInit, ai.Kind())
	require.Len(t, ai.Elements, n)
	for _, el := range ai.Elements {
		require.Equal(t, ast.KindArrayInitElem, el.Kind())
	}
	return ai
}

func diagMessages(r ParseResult) []string {
	var out []string
	for _, d := range r.Diags {
		out = append(out, d.Message)
	}
	return out
}

func TestInitializer(t *testing.T) {
	t.Run("struct initialiser", func(t *testing.T) {
		d := varDeclsOf(t, "fbTime : FB_LocalSystemTime := (bEnable := TRUE, dwCycle := 1);")[0]
		si := requireStructInit(t, d.InitValue, "bEnable", "dwCycle")
		require.Equal(t, "1", si.Fields[1].Value.(*ast.Literal).Value)
		require.Equal(t, d.InitValue.Span().Start.Col, 32)
	})

	t.Run("paren expressions stay ParenExpr", func(t *testing.T) {
		decls := varDeclsOf(t, "x : INT := (1 + 2);\ny : INT := (a);\nz : INT := (a) + 1;")
		_, ok := decls[0].InitValue.(*ast.ParenExpr)
		require.True(t, ok, "got %T", decls[0].InitValue)
		_, ok = decls[1].InitValue.(*ast.ParenExpr)
		require.True(t, ok, "got %T", decls[1].InitValue)
		_, ok = decls[2].InitValue.(*ast.BinaryExpr)
		require.True(t, ok, "got %T", decls[2].InitValue)
	})

	t.Run("array of struct over many lines", func(t *testing.T) {
		src := "Device_1_SlaveInfo : ARRAY[1..EcDiagParam.MAX_EC_SLAVES] OF ST_EcSlaveInfo := [\n" +
			"\t(p_stat_sName := 'A', p_stat_nPhysAddr := 1001),\n" +
			"\t(p_stat_sName := 'B',\n\t p_stat_nPhysAddr := 1002)\n" +
			"];\nnext : INT;"
		decls := varDeclsOf(t, src)
		require.Len(t, decls, 2)
		ai := requireArrayInit(t, decls[0].InitValue, 2)
		for _, el := range ai.Elements {
			require.Nil(t, el.Count)
			requireStructInit(t, el.Value, "p_stat_sName", "p_stat_nPhysAddr")
		}
		at := decls[0].Type.(*ast.ArrayType)
		hi, ok := at.Ranges[0].High.(*ast.MemberAccessExpr)
		require.True(t, ok, "constant-expression bound must parse, got %T", at.Ranges[0].High)
		require.Equal(t, "MAX_EC_SLAVES", hi.Member.Name)
	})

	t.Run("repetition", func(t *testing.T) {
		d := varDeclsOf(t, "a : ARRAY[0..9] OF INT := [3(0), 1, 2(5)];")[0]
		ai := requireArrayInit(t, d.InitValue, 3)
		require.Equal(t, "3", ai.Elements[0].Count.(*ast.Literal).Value)
		require.Equal(t, "0", ai.Elements[0].Value.(*ast.Literal).Value)
		require.Nil(t, ai.Elements[1].Count)
		require.Equal(t, "2", ai.Elements[2].Count.(*ast.Literal).Value)
	})

	t.Run("huge repetition is not expanded", func(t *testing.T) {
		d := varDeclsOf(t, "a : ARRAY[0..9] OF INT := [1000000000(0)];")[0]
		ai := requireArrayInit(t, d.InitValue, 1)
		require.Equal(t, "1000000000", ai.Elements[0].Count.(*ast.Literal).Value)
	})

	t.Run("repetition of struct and empty repetition", func(t *testing.T) {
		d := varDeclsOf(t, "a : ARRAY[0..9] OF ST := [2((x := 1)), 3()];")[0]
		ai := requireArrayInit(t, d.InitValue, 2)
		requireStructInit(t, ai.Elements[0].Value, "x")
		require.Nil(t, ai.Elements[1].Value)
	})

	t.Run("simple, nested and mixed", func(t *testing.T) {
		decls := varDeclsOf(t, "b : ARRAY[1..10] OF BOOL := [TRUE, FALSE, TRUE];\n"+
			"m : ARRAY[0..1, 0..1] OF INT := [[1, 2], [3, 4]];\n"+
			"s : ST := (inner := (a := 1), arr := [1, 2]);\n"+
			"t : ARRAY[0..2] OF INT := [1, 2, 3,];\n"+
			"u : ST := (a := 1, b := -2,);")
		requireArrayInit(t, decls[0].InitValue, 3)
		m := requireArrayInit(t, decls[1].InitValue, 2)
		requireArrayInit(t, m.Elements[0].Value, 2)
		s := requireStructInit(t, decls[2].InitValue, "inner", "arr")
		requireStructInit(t, s.Fields[0].Value, "a")
		requireArrayInit(t, s.Fields[1].Value, 2)
		requireArrayInit(t, decls[3].InitValue, 3)
		requireStructInit(t, decls[4].InitValue, "a", "b")
	})

	t.Run("empty array initialiser", func(t *testing.T) {
		d := varDeclsOf(t, "a : ARRAY[0..1] OF INT := [];")[0]
		requireArrayInit(t, d.InitValue, 0)
	})

	t.Run("struct member and TYPE default", func(t *testing.T) {
		td := typeDeclOf(t, "TYPE S : STRUCT\n m : ST_X := (a := 1);\n n : ARRAY[0..1] OF INT := [2(7)];\nEND_STRUCT\nEND_TYPE")
		st := td.Type.(*ast.StructType)
		requireStructInit(t, st.Members[0].InitValue, "a")
		requireArrayInit(t, st.Members[1].InitValue, 1)

		td = typeDeclOf(t, "TYPE A : ARRAY[0..2] OF INT := [1, 2, 3]; END_TYPE")
		requireArrayInit(t, td.InitValue, 3)
	})

	t.Run("VAR_GLOBAL", func(t *testing.T) {
		f := parseClean(t, "VAR_GLOBAL\n arr : ARRAY[1..2] OF ST := [(a := 1), (a := 2)];\nEND_VAR\n")
		gvl := f.Declarations[0].(*ast.GVLDecl)
		requireArrayInit(t, gvl.Blocks[0].Declarations[0].InitValue, 2)
	})

	t.Run("bit access initialiser", func(t *testing.T) {
		d := varDeclsOf(t, "x : BOOL := i_uStatusWord.0;")[0]
		_, ok := d.InitValue.(*ast.BitAccessExpr)
		require.True(t, ok, "got %T", d.InitValue)
	})

	t.Run("error recovery", func(t *testing.T) {
		cases := []string{
			"a : ST := (a := );",
			"a : ARRAY[0..1] OF INT := [3(];",
			"a : ST := (a := 1 b := 2);",
			"a : ST := (a := 1, 5);",
			"a : ARRAY[0..1] OF INT := [1 2];",
		}
		for _, c := range cases {
			r := Parse("t.st", "PROGRAM P\nVAR\n"+c+"\nnext : INT;\nEND_VAR\nEND_PROGRAM\n")
			require.NotEmpty(t, r.Diags, c)
			decls := r.File.Declarations[0].(*ast.ProgramDecl).VarBlocks[0].Declarations
			require.Equal(t, "next", decls[len(decls)-1].Names[0].Name, "%s: %v", c, diagMessages(r))
		}
	})

	t.Run("unterminated at EOF", func(t *testing.T) {
		r := Parse("t.st", "PROGRAM P\nVAR\na : ARRAY[0..1] OF ST := [(a := 1), ")
		require.NotEmpty(t, r.Diags)
		r = Parse("t.st", "PROGRAM P\nVAR\na : ST := (a := 1, ")
		require.NotEmpty(t, r.Diags)
	})

	t.Run("nesting depth limit", func(t *testing.T) {
		ok64 := strings.Repeat("[", 64) + "1" + strings.Repeat("]", 64)
		varDeclsOf(t, "a : T := "+ok64+";")

		deep := strings.Repeat("[", 65) + "1" + strings.Repeat("]", 65)
		r := Parse("t.st", "PROGRAM P\nVAR\na : T := "+deep+";\nnext : INT;\nEND_VAR\nEND_PROGRAM\n")
		msgs := diagMessages(r)
		require.Equal(t, []string{"initialiser nested too deeply"}, msgs)
		decls := r.File.Declarations[0].(*ast.ProgramDecl).VarBlocks[0].Declarations
		require.Equal(t, "next", decls[1].Names[0].Name)

		structDeep := strings.Repeat("(a := ", 70) + "1" + strings.Repeat(")", 70)
		r = Parse("t.st", "PROGRAM P\nVAR\na : T := "+structDeep+";\nEND_VAR\nEND_PROGRAM\n")
		require.Equal(t, []string{"initialiser nested too deeply"}, diagMessages(r))
	})

	t.Run("adversarial nesting finishes quickly", func(t *testing.T) {
		start := time.Now()
		r := Parse("t.st", "PROGRAM P\nVAR\na : T := "+strings.Repeat("[", 10000)+"\nEND_VAR\nEND_PROGRAM\n")
		require.NotEmpty(t, r.Diags)
		r = Parse("t.st", "PROGRAM P\nVAR\na : T := "+strings.Repeat("(a := ", 10000)+"\nEND_VAR\nEND_PROGRAM\n")
		require.NotEmpty(t, r.Diags)
		require.Less(t, time.Since(start), 5*time.Second)
	})
}
