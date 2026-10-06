package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

func TestPropertyAccessModifier(t *testing.T) {
	for _, mod := range []string{"", "PUBLIC", "PRIVATE", "PROTECTED", "INTERNAL", "ABSTRACT", "FINAL", "PUBLIC FINAL"} {
		t.Run("FB "+mod, func(t *testing.T) {
			src := "FUNCTION_BLOCK FB_A\nVAR x : INT; END_VAR\nPROPERTY " + mod + " P : INT\n" +
				"GET P := x; END_GET\nSET x := P; END_SET\nEND_PROPERTY\nEND_FUNCTION_BLOCK\n"
			r := Parse("t.st", src)
			require.Empty(t, r.Diags)
			fb := r.File.Declarations[0].(*ast.FunctionBlockDecl)
			require.Len(t, fb.Properties, 1)
			require.Equal(t, "P", fb.Properties[0].Name.Name)
			require.NotNil(t, fb.Properties[0].Getter)
			require.NotNil(t, fb.Properties[0].Setter)
		})
		t.Run("INTERFACE "+mod, func(t *testing.T) {
			src := "INTERFACE I_A\nPROPERTY " + mod + " P : INT\nEND_PROPERTY\nEND_INTERFACE\n"
			r := Parse("t.st", src)
			require.Empty(t, r.Diags)
			it := r.File.Declarations[0].(*ast.InterfaceDecl)
			require.Len(t, it.Properties, 1)
			require.Equal(t, "P", it.Properties[0].Name.Name)
		})
	}
}
