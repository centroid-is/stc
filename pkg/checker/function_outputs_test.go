package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionOutputs(t *testing.T) {
	src := `FUNCTION F : INT
VAR_INPUT
	a : INT;
END_VAR
VAR_OUTPUT
	q : BOOL;
END_VAR
q := a > 0;
F := a;
END_FUNCTION

FUNCTION G : INT
VAR_INPUT
	a : INT;
END_VAR
VAR_IN_OUT
	io : INT;
END_VAR
G := a + io;
END_FUNCTION

FUNCTION_BLOCK FB
METHOD M : INT
VAR_INPUT
	x : INT;
END_VAR
VAR_OUTPUT
	done : BOOL;
	cnt : DINT;
END_VAR
M := x;
END_METHOD
END_FUNCTION_BLOCK
`
	ds, table := runGVL(t, []gvlFile{{"main.st", src}})
	assert.Empty(t, errorsOf(ds))

	f := table.LookupGlobal("F").Type.(*types.FunctionType)
	assert.Equal(t, []string{"a"}, paramNames(f.Params))
	require.Len(t, f.Outputs, 1)
	assert.Equal(t, "q", f.Outputs[0].Name)
	assert.Equal(t, types.DirOutput, f.Outputs[0].Direction)
	assert.Equal(t, types.KindBOOL, f.Outputs[0].Type.Kind())

	g := table.LookupGlobal("G").Type.(*types.FunctionType)
	assert.Equal(t, []string{"a", "io"}, paramNames(g.Params))
	assert.Empty(t, g.Outputs)

	m := table.LookupPOU("FB").LookupLocal("M").Type.(*types.FunctionType)
	assert.Equal(t, []string{"x"}, paramNames(m.Params))
	assert.Equal(t, []string{"done", "cnt"}, paramNames(m.Outputs))
	assert.Equal(t, types.DirOutput, m.Outputs[1].Direction)
}
