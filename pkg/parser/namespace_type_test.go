package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

func requireNamespaced(t *testing.T, ts ast.TypeSpec, ns, name string) *ast.NamedType {
	t.Helper()
	nt, ok := ts.(*ast.NamedType)
	require.True(t, ok, "expected NamedType, got %T", ts)
	require.NotNil(t, nt.Namespace, "namespace must be set")
	require.Equal(t, ns, nt.Namespace.Name)
	require.Equal(t, name, nt.Name.Name)
	require.Equal(t, nt.Namespace.Span().Start, nt.Span().Start)
	require.Equal(t, nt.Name.Span().End, nt.Span().End)
	return nt
}

func TestNamespaceType(t *testing.T) {
	decls := varDeclsOf(t, "ec : Tc2_EtherCAT.ST_EcSlaveState;\n"+
		"p : POINTER TO Lib.T;\n"+
		"a : ARRAY[0..1] OF Lib.T;\n"+
		"r : REFERENCE TO Lib.T;\n"+
		"plain : ST_X;")
	requireNamespaced(t, decls[0].Type, "Tc2_EtherCAT", "ST_EcSlaveState")
	requireNamespaced(t, decls[1].Type.(*ast.PointerType).BaseType, "Lib", "T")
	requireNamespaced(t, decls[2].Type.(*ast.ArrayType).ElementType, "Lib", "T")
	requireNamespaced(t, decls[3].Type.(*ast.ReferenceType).BaseType, "Lib", "T")
	require.Nil(t, decls[4].Type.(*ast.NamedType).Namespace)

	t.Run("missing name after dot", func(t *testing.T) {
		r := Parse("t.st", "PROGRAM P\nVAR\nx : Lib.;\ny : INT;\nEND_VAR\nEND_PROGRAM\n")
		require.NotEmpty(t, r.Diags)
		decls := r.File.Declarations[0].(*ast.ProgramDecl).VarBlocks[0].Declarations
		require.Len(t, decls, 2)
		require.Equal(t, "y", decls[1].Names[0].Name)
	})

	t.Run("struct member", func(t *testing.T) {
		td := typeDeclOf(t, "TYPE S : STRUCT\n ec : Tc2_EtherCAT.ST_EcSlaveState;\nEND_STRUCT\nEND_TYPE")
		requireNamespaced(t, td.Type.(*ast.StructType).Members[0].Type, "Tc2_EtherCAT", "ST_EcSlaveState")
	})
}
