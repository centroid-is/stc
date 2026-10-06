package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

// typeDeclOf parses src cleanly and returns its first declaration as a TypeDecl.
func typeDeclOf(t *testing.T, src string) *ast.TypeDecl {
	t.Helper()
	f := parseClean(t, src)
	require.NotEmpty(t, f.Declarations)
	td, ok := f.Declarations[0].(*ast.TypeDecl)
	require.True(t, ok, "expected TypeDecl, got %T", f.Declarations[0])
	return td
}

// varDeclsOf parses a PROGRAM with the given VAR body cleanly and returns its declarations.
func varDeclsOf(t *testing.T, body string) []*ast.VarDecl {
	t.Helper()
	f := parseClean(t, "PROGRAM P\nVAR\n"+body+"\nEND_VAR\nEND_PROGRAM\n")
	return f.Declarations[0].(*ast.ProgramDecl).VarBlocks[0].Declarations
}

func requireBaseType(t *testing.T, ts ast.TypeSpec, want string) *ast.EnumType {
	t.Helper()
	et, ok := ts.(*ast.EnumType)
	require.True(t, ok, "expected EnumType, got %T", ts)
	if want == "" {
		require.Nil(t, et.BaseType)
		return et
	}
	nt, ok := et.BaseType.(*ast.NamedType)
	require.True(t, ok, "expected NamedType base, got %T", et.BaseType)
	require.Equal(t, want, nt.Name.Name)
	return et
}

func TestEnumBaseType(t *testing.T) {
	for _, base := range []string{"UINT", "USINT", "INT", "DWORD", "SINT", "DINT", "LINT", "UDINT", "ULINT", "BYTE", "WORD", "LWORD"} {
		t.Run(base, func(t *testing.T) {
			td := typeDeclOf(t, "TYPE E : (a := 0, b := 1) "+base+"; END_TYPE")
			et := requireBaseType(t, td.Type, base)
			require.Len(t, et.Values, 2)
			require.Greater(t, et.Span().End.Offset, et.Values[1].Span().End.Offset)
		})
	}

	t.Run("no base type", func(t *testing.T) {
		td := typeDeclOf(t, "TYPE E : (a, b); END_TYPE")
		requireBaseType(t, td.Type, "")
		require.Nil(t, td.InitValue)
	})

	t.Run("type default", func(t *testing.T) {
		td := typeDeclOf(t, "TYPE E : (a, b) := b; END_TYPE")
		requireBaseType(t, td.Type, "")
		id, ok := td.InitValue.(*ast.Ident)
		require.True(t, ok, "expected Ident default, got %T", td.InitValue)
		require.Equal(t, "b", id.Name)
	})

	t.Run("base type and default", func(t *testing.T) {
		td := typeDeclOf(t, "TYPE E : (a, b) UINT := b; END_TYPE")
		requireBaseType(t, td.Type, "UINT")
		require.NotNil(t, td.InitValue)
	})

	t.Run("alias default", func(t *testing.T) {
		td := typeDeclOf(t, "TYPE T_Count : INT := 5; END_TYPE")
		lit, ok := td.InitValue.(*ast.Literal)
		require.True(t, ok)
		require.Equal(t, "5", lit.Value)
	})

	t.Run("inline VAR enums", func(t *testing.T) {
		decls := varDeclsOf(t, "eStep : (E_IDLE, E_RUN) := E_IDLE;\nx : (a, b) INT;")
		requireBaseType(t, decls[0].Type, "")
		require.Equal(t, "E_IDLE", decls[0].InitValue.(*ast.Ident).Name)
		requireBaseType(t, decls[1].Type, "INT")
	})

	t.Run("non-integer base type is a parse error", func(t *testing.T) {
		r := Parse("t.st", "TYPE E : (a, b) REAL; END_TYPE")
		require.NotEmpty(t, r.Diags)
	})

	t.Run("svncore shape", func(t *testing.T) {
		src := "{attribute 'qualified_only'}\n{attribute 'to_string'}\nTYPE hmis_e :\n(\n" +
			"  tun := 0,  ///< tuning\n" +
			"  {attribute 'OPC.UA.DA' := '1'}\n" +
			"  rdy := 2,\n" +
			"  nst\n" +
			") UINT;\nEND_TYPE\n"
		td := typeDeclOf(t, src)
		require.Len(t, td.Attributes, 2)
		et := requireBaseType(t, td.Type, "UINT")
		require.Len(t, et.Values, 3)
		require.Len(t, et.Values[1].Attributes, 1)
	})
}

func TestStraySemicolon(t *testing.T) {
	t.Run("in VAR", func(t *testing.T) {
		decls := varDeclsOf(t, "rDropPoint : REAL;;\nOHG : BOOL;\n;")
		require.Len(t, decls, 2)
		require.Equal(t, "OHG", decls[1].Names[0].Name)
	})

	t.Run("in VAR before pragma", func(t *testing.T) {
		decls := varDeclsOf(t, "a : INT;;\n{attribute 'hide'}\nb : INT;")
		require.Len(t, decls, 2)
		require.Len(t, decls[1].Attributes, 1)
	})

	t.Run("in STRUCT", func(t *testing.T) {
		td := typeDeclOf(t, "TYPE S : STRUCT\n a : REAL;;\n b : INT;\n;\nEND_STRUCT\nEND_TYPE")
		st := td.Type.(*ast.StructType)
		require.Len(t, st.Members, 2)
	})

	t.Run("top level", func(t *testing.T) {
		f := parseClean(t, ";\nTYPE E : (a, b); END_TYPE\n;;\nPROGRAM P\nEND_PROGRAM\n;\n")
		require.Len(t, f.Declarations, 2)
	})
}

func TestTypedBasedLiteralParse(t *testing.T) {
	as := bodyOf(t, "x := SHL(BYTE#16#10, nPort);")[0].(*ast.AssignStmt)
	call := as.Value.(*ast.CallExpr)
	lit, ok := call.Args[0].(*ast.Literal)
	require.True(t, ok, "expected Literal, got %T", call.Args[0])
	require.Equal(t, ast.LitTyped, lit.LitKind)
	require.Equal(t, "BYTE", lit.TypePrefix)
	require.Equal(t, "16#10", lit.Value)

	t.Run("typed literal case label without space", func(t *testing.T) {
		cs := bodyOf(t, "CASE n OF\nINT#5:\n  x := 1;\nINT#6, INT#7:\n  x := 2;\nEND_CASE")[0].(*ast.CaseStmt)
		require.Len(t, cs.Branches, 2)
	})
}
