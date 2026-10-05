package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

func gvlOf(t *testing.T, f *ast.SourceFile) *ast.GVLDecl {
	t.Helper()
	var found *ast.GVLDecl
	for _, d := range f.Declarations {
		if g, ok := d.(*ast.GVLDecl); ok {
			require.Nil(t, found, "more than one GVLDecl")
			found = g
		}
	}
	require.NotNil(t, found, "no GVLDecl in file")
	return found
}

func TestGVLParse(t *testing.T) {
	t.Run("gvl1 is one GVLDecl named from the file", func(t *testing.T) {
		r := Parse("gvl1.st", readProbe(t, "gvl1.st"))
		require.Empty(t, r.Diags)
		require.Len(t, r.File.Declarations, 1)
		g := r.File.Declarations[0].(*ast.GVLDecl)
		require.Equal(t, ast.KindGVLDecl, g.Kind())
		require.Equal(t, "gvl1", g.Name.Name)
		require.Len(t, g.Blocks, 1)
		require.Equal(t, ast.VarGlobal, g.Blocks[0].Section)
		require.Len(t, g.Blocks[0].Declarations, 1)
		require.Equal(t, "x", g.Blocks[0].Declarations[0].Names[0].Name)
	})

	t.Run("gvl2 keeps qualified_only on the GVL", func(t *testing.T) {
		r := Parse("gvl2.st", readProbe(t, "gvl2.st"))
		require.Empty(t, r.Diags)
		g := gvlOf(t, r.File)
		require.Equal(t, []string{"qualified_only"}, attrNames(g.Attributes))
		require.Empty(t, g.Blocks[0].Attributes)
	})

	t.Run("ECT has a TypeDecl and a GVLDecl with attributes", func(t *testing.T) {
		r := Parse("ECT.st", readProbe(t, "ECT.st"))
		require.Empty(t, r.Diags)
		require.Len(t, r.File.Declarations, 2)
		_, isType := r.File.Declarations[0].(*ast.TypeDecl)
		require.True(t, isType)
		g := r.File.Declarations[1].(*ast.GVLDecl)
		require.Equal(t, "ECT", g.Name.Name)
		require.Equal(t, []string{"qualified_only"}, attrNames(g.Attributes))
		require.Len(t, g.Blocks, 1)
		vb := g.Blocks[0]
		require.True(t, vb.IsPersistent)
		require.True(t, vb.IsRetain)
		require.False(t, vb.IsConstant)
		require.Len(t, vb.Declarations, 2)
		x := vb.Declarations[0]
		require.Equal(t, "X", x.Names[0].Name)
		require.Equal(t, []string{"TcLinkTo", "OPC.UA.DA", "OPC.UA.DA.StructuredType"}, attrNames(x.Attributes))
		lost := vb.Declarations[1]
		require.Equal(t, []string{"OPC.UA.DA.Description"}, attrNames(lost.Attributes))
		require.Equal(t, "The slave controller's own lost-link count", lost.Attributes[0].Value)
	})

	t.Run("name is sanitized from the basename", func(t *testing.T) {
		src := "VAR_GLOBAL\n\tx : BOOL;\nEND_VAR\n"
		require.Equal(t, "My_GVL", gvlOf(t, Parse("dir/My-GVL.st", src).File).Name.Name)
		require.Equal(t, "GVL", gvlOf(t, Parse("", src).File).Name.Name)
		require.Equal(t, "noext", gvlOf(t, Parse("noext", src).File).Name.Name)
	})

	t.Run("blocks aggregate at the first position", func(t *testing.T) {
		src := "VAR_GLOBAL\n\ta : INT;\nEND_VAR\n" +
			"PROGRAM P\nEND_PROGRAM\n" +
			"{attribute 'second'}\nVAR_GLOBAL CONSTANT\n\tc : INT := 5;\nEND_VAR\n"
		f := parseClean(t, src)
		require.Len(t, f.Declarations, 2)
		g := f.Declarations[0].(*ast.GVLDecl)
		_, isProg := f.Declarations[1].(*ast.ProgramDecl)
		require.True(t, isProg)
		require.Len(t, g.Blocks, 2)
		require.Empty(t, g.Attributes)
		require.False(t, g.Blocks[0].IsConstant)
		require.True(t, g.Blocks[1].IsConstant)
		require.Equal(t, []string{"second"}, attrNames(g.Blocks[1].Attributes))
		require.Equal(t, g.Blocks[0].Span().Start, g.Span().Start)
		require.Equal(t, g.Blocks[1].Span().End, g.Span().End)
	})

	t.Run("non-attribute pragma before first block lands on GVL", func(t *testing.T) {
		f := parseClean(t, "{warning disable C0001}\nVAR_GLOBAL\n\ta : INT;\nEND_VAR\n")
		g := gvlOf(t, f)
		require.Len(t, g.Pragmas, 1)
	})

	t.Run("recovery stops at VAR_GLOBAL", func(t *testing.T) {
		r := Parse("r.st", "123 456\nVAR_GLOBAL\n\tx : BOOL;\nEND_VAR\n")
		require.Len(t, r.Diags, 1)
		g := gvlOf(t, r.File)
		require.Len(t, g.Blocks[0].Declarations, 1)
	})
}
