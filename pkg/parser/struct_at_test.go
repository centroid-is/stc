package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

func TestStructAT(t *testing.T) {
	t.Run("structat fixture keeps wildcard address and attribute", func(t *testing.T) {
		f := parseClean(t, readProbe(t, "structat.st"))
		st := f.Declarations[0].(*ast.TypeDecl).Type.(*ast.StructType)
		require.Len(t, st.Members, 2)
		i1, i2 := st.Members[0], st.Members[1]
		require.Equal(t, "I1", i1.Name.Name)
		require.NotNil(t, i1.AtAddress)
		require.Equal(t, "%I*", i1.AtAddress.Name)
		require.Equal(t, []string{"OPC.UA.DA.Access"}, attrNames(i1.Attributes))
		require.Nil(t, i2.AtAddress)
	})

	t.Run("AT with init value", func(t *testing.T) {
		src := "TYPE S :\nSTRUCT\n\ta AT %Q* : INT := 5;\nEND_STRUCT\nEND_TYPE\n"
		f := parseClean(t, src)
		m := f.Declarations[0].(*ast.TypeDecl).Type.(*ast.StructType).Members[0]
		require.NotNil(t, m.AtAddress)
		require.Equal(t, "%Q*", m.AtAddress.Name)
		require.NotNil(t, m.InitValue)
	})

	t.Run("explicit address on member", func(t *testing.T) {
		src := "TYPE S :\nSTRUCT\n\ta AT %IX0.0 : BOOL;\nEND_STRUCT\nEND_TYPE\n"
		f := parseClean(t, src)
		m := f.Declarations[0].(*ast.TypeDecl).Type.(*ast.StructType).Members[0]
		require.Equal(t, "%IX0.0", m.AtAddress.Name)
	})

	t.Run("AT without address reports error", func(t *testing.T) {
		src := "TYPE S :\nSTRUCT\n\ta AT : BOOL;\nEND_STRUCT\nEND_TYPE\n"
		r := Parse("s.st", src)
		require.NotEmpty(t, r.Diags)
	})

	t.Run("fbat fixture parses clean", func(t *testing.T) {
		parseClean(t, readProbe(t, "fbat.st"))
	})
}
