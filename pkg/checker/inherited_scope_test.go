package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fbBase = `FUNCTION_BLOCK FB_Base
VAR_INPUT
	baseInput : INT;
END_VAR
VAR_OUTPUT
	baseOutput : INT;
END_VAR
VAR
	x : INT;
END_VAR
baseOutput := baseInput;

METHOD M : BOOL
M := TRUE;
END_METHOD

ACTION A_Base
x := 0;
END_ACTION
END_FUNCTION_BLOCK
`

func TestInheritedScope(t *testing.T) {
	t.Run("derived body sees base variables, methods and actions", func(t *testing.T) {
		src := `PROGRAM P
VAR
	d : FB_Derived;
	y : INT;
END_VAR
d(baseInput := 1, own := TRUE);
y := d.baseOutput;
END_PROGRAM

FUNCTION_BLOCK FB_Derived EXTENDS FB_Base
VAR_INPUT
	own : BOOL;
END_VAR
x := 1;
M();
A_Base();
baseOutput := x;
END_FUNCTION_BLOCK
` + fbBase
		ds, table := runGVL(t, []gvlFile{{"main.st", src}})
		assert.Empty(t, ds, "no errors and no unused warnings for inherited x")

		fb := table.LookupGlobal("FB_Derived").Type.(*types.FunctionBlockType)
		assert.Equal(t, []string{"baseInput", "own"}, paramNames(fb.Inputs), "base inputs first")
		assert.Equal(t, []string{"baseOutput"}, paramNames(fb.Outputs))
	})

	t.Run("three-level chain", func(t *testing.T) {
		src := `PROGRAM P
VAR
	c : FB_C;
	y : INT;
END_VAR
c(aIn := 1, bIn := 2, cIn := 3);
y := c.aOut;
END_PROGRAM
FUNCTION_BLOCK FB_C EXTENDS FB_B
VAR_INPUT
	cIn : INT;
END_VAR
aOut := aIn + bIn + cIn + aLocal;
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_B EXTENDS FB_A
VAR_INPUT
	bIn : INT;
END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_A
VAR_INPUT
	aIn : INT;
END_VAR
VAR_OUTPUT
	aOut : INT;
END_VAR
VAR
	aLocal : INT;
END_VAR
END_FUNCTION_BLOCK
`
		ds, table := runGVL(t, []gvlFile{{"main.st", src}})
		assert.Empty(t, errorsOf(ds))
		fbC := table.LookupGlobal("FB_C").Type.(*types.FunctionBlockType)
		assert.Equal(t, []string{"aIn", "bIn", "cIn"}, paramNames(fbC.Inputs))
		fbB := table.LookupGlobal("FB_B").Type.(*types.FunctionBlockType)
		assert.Equal(t, []string{"aIn", "bIn"}, paramNames(fbB.Inputs), "each level inherited once")
	})

	t.Run("EXTENDS cycle terminates", func(t *testing.T) {
		src := `FUNCTION_BLOCK FB_X EXTENDS FB_Y
VAR_INPUT
	a : INT;
END_VAR
a := zz;
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_Y EXTENDS FB_X
VAR_INPUT
	b : INT;
END_VAR
b := a;
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_Z EXTENDS FB_Z
END_FUNCTION_BLOCK
`
		ds, _ := runGVL(t, []gvlFile{{"main.st", src}})
		// zz is undeclared; the lookup must not loop through the cycle.
		assert.Contains(t, codesOf(errorsOf(ds)), CodeUndeclared)
	})

	t.Run("derived variable shadowing a base variable", func(t *testing.T) {
		src := `FUNCTION_BLOCK FB_Derived EXTENDS FB_Base
VAR
	x : INT;
END_VAR
x := 2;
END_FUNCTION_BLOCK
` + fbBase
		ds := errorsOf(runAction(t, src))
		require.Len(t, ds, 1)
		assert.Equal(t, CodeRedeclared, ds[0].Code)
		assert.Contains(t, ds[0].Message, `"x"`)
		assert.Equal(t, 3, ds[0].Pos.Line)
	})

	t.Run("derived method overriding a base method is not a redeclaration", func(t *testing.T) {
		src := `FUNCTION_BLOCK FB_Derived EXTENDS FB_Base
M();

METHOD M : BOOL
M := FALSE;
END_METHOD
END_FUNCTION_BLOCK
` + fbBase
		assert.Empty(t, errorsOf(runAction(t, src)))
	})

	t.Run("base from a library file", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{{"main.st", `FUNCTION_BLOCK FB_Derived EXTENDS FB_Base
x := 1;
END_FUNCTION_BLOCK
`}}, ResolveOpts{LibraryFiles: parseGVLFiles(t, []gvlFile{{"lib.st", fbBase}})})
		assert.Empty(t, errorsOf(ds))
	})
}
