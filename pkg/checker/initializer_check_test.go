package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const initTypes = `TYPE ST_A : STRUCT
	a : INT;
	b : STRING;
END_STRUCT
END_TYPE
{attribute 'strict'}
TYPE E_S : (sa, sb); END_TYPE
TYPE E_T : (ta, tb); END_TYPE
FUNCTION_BLOCK FB_LocalSystemTime
VAR_INPUT
	bEnable : BOOL;
	dwCycle : DWORD;
END_VAR
VAR_OUTPUT
	bValid : BOOL;
END_VAR
END_FUNCTION_BLOCK
VAR_GLOBAL CONSTANT
	C : INT := 3;
END_VAR
`

// initErrors declares vars in PROGRAM P (with initTypes) and returns the
// error diagnostics.
func initErrors(t *testing.T, vars string) []diag.Diagnostic {
	t.Helper()
	ds, _ := runGVL(t, []gvlFile{
		{"types.st", initTypes},
		{"main.st", "PROGRAM P\nVAR\n" + vars + "\nEND_VAR\nEND_PROGRAM\n"},
	})
	return errorsOf(ds)
}

func TestInitializerCheck(t *testing.T) {
	t.Run("struct initialiser", func(t *testing.T) {
		assert.Empty(t, initErrors(t, "s : ST_A := (a := 1, b := 'x');"))
		assert.Empty(t, initErrors(t, "s : ST_A := (b := 'x');"))

		errs := initErrors(t, "s : ST_A := (zz := 1);")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeNoMember, errs[0].Code)
		assert.Contains(t, errs[0].Message, "zz")

		errs = initErrors(t, "s : ST_A := (a := 'x');")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeTypeMismatch, errs[0].Code)

		errs = initErrors(t, "s : ST_A := (a := 1, a := 2);")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeNoMember, errs[0].Code)
		assert.Contains(t, errs[0].Message, "more than once")
	})

	t.Run("FB instance initialiser", func(t *testing.T) {
		assert.Empty(t, initErrors(t, "fbTime : FB_LocalSystemTime := (bEnable := TRUE, dwCycle := 5);"))
		errs := initErrors(t, "fbTime : FB_LocalSystemTime := (bNope := TRUE);")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeNoMember, errs[0].Code)
		assert.Contains(t, errs[0].Message, "bNope")
		errs = initErrors(t, "fbTime : FB_LocalSystemTime := (bEnable := 'x');")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeTypeMismatch, errs[0].Code)
	})

	t.Run("array element counts", func(t *testing.T) {
		assert.Empty(t, initErrors(t, "arr : ARRAY[0..2] OF INT := [1, 2, 3];"))
		assert.Empty(t, initErrors(t, "arr : ARRAY[0..2] OF INT := [3(0)];"))
		assert.Empty(t, initErrors(t, "arr : ARRAY[0..2] OF INT := [1, 2];"))
		assert.Empty(t, initErrors(t, "arr : ARRAY[-1..1] OF INT := [1, 2(5)];"))
		assert.Empty(t, initErrors(t, "arr : ARRAY[0..1, 0..1] OF INT := [1, 2, 3, 4];"))

		errs := initErrors(t, "arr : ARRAY[0..2] OF INT := [1, 2, 3, 4];")
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "too many initialisers (4 > 3)")

		errs = initErrors(t, "arr : ARRAY[0..9] OF INT := [1000000000(0)];")
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "too many initialisers (1000000000 > 10)")

		errs = initErrors(t, "arr : ARRAY[0..9] OF INT := [9223372036854775807(0), 9223372036854775807(0), 4294967295(0)];")
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "too many initialisers")

		errs = initErrors(t, "arr : ARRAY[0..1, 0..1] OF INT := [1, 2, 3, 4, 5];")
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "(5 > 4)")

		errs = initErrors(t, "arr : ARRAY[0..2] OF INT := [1, 'x'];")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeTypeMismatch, errs[0].Code)
	})

	t.Run("array of struct and nested arrays", func(t *testing.T) {
		assert.Empty(t, initErrors(t, "arr : ARRAY[0..1] OF ST_A := [(a := 1), (a := 2, b := 'y')];"))
		errs := initErrors(t, "arr : ARRAY[0..1] OF ST_A := [(a := 1), (zz := 2)];")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeNoMember, errs[0].Code)

		assert.Empty(t, initErrors(t, "m : ARRAY[0..1, 0..1] OF INT := [[1, 2], [3, 4]];"))
		assert.Empty(t, initErrors(t, "m : ARRAY[0..1] OF ARRAY[0..1] OF INT := [[1, 2], [3, 4]];"))
		errs = initErrors(t, "m : ARRAY[0..1, 0..1] OF INT := [[1, 2, 3], [3, 4]];")
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "(3 > 2)")
		errs = initErrors(t, "m : ARRAY[0..1, 0..1] OF INT := [[1, 2], [3, 4], [5, 6]];")
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "(3 > 2)")
		errs = initErrors(t, "m : ARRAY[0..1] OF ARRAY[0..1] OF INT := [[1, 2], ['x', 4]];")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeTypeMismatch, errs[0].Code)
	})

	t.Run("initialiser shape mismatch", func(t *testing.T) {
		errs := initErrors(t, "n : INT := [1, 2];")
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "array initialiser")
		errs = initErrors(t, "n : INT := (a := 1);")
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "structure initialiser")
	})

	t.Run("literal values", func(t *testing.T) {
		for _, vars := range []string{
			"n : INT := 5;",
			"n : INT := -5;",
			"b : BOOL := TRUE;",
			"t : TIME := T#1s;",
			"r : REAL := 1.5;",
			"r : REAL := 0;",
			"lr : LREAL := 2;",
			"w : WORD := 16#FFFF;",
			"by : BYTE := BYTE#16#10;",
			"d : DINT := INT#5;",
			"s : STRING := 'abc';",
			"ws : WSTRING := \"abc\";",
			"c : CHAR := 'a';",
			"e : E_T := 1;",
			"e : E_S := E_S.sb;",
			"p : POINTER TO INT := 0;",
		} {
			assert.Empty(t, initErrors(t, vars), vars)
		}
		for _, vars := range []string{"n : INT := 'x';", "b : BOOL := 'x';", "n : INT := 1.5;", "s : STRING := 5;", "d : DINT := UINT#5;"} {
			errs := initErrors(t, vars)
			require.Len(t, errs, 1, vars)
			assert.Equal(t, CodeTypeMismatch, errs[0].Code, vars)
			assert.Contains(t, errs[0].Message, "cannot initialise", vars)
		}
		errs := initErrors(t, "e : E_S := 1;")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeEnumRule, errs[0].Code)
	})

	t.Run("nonliteral_clean", func(t *testing.T) {
		for _, vars := range []string{
			"A : INT; n : INT := A + 1;",
			"n : INT := C + 1;",
			"x : E_T; e : E_S := x;",
			"A : INT; s : ST_A := (a := A + 1);",
			"A : INT; arr : ARRAY[0..1] OF INT := [A, A + 1];",
			"r : REAL := C;",
		} {
			assert.Empty(t, initErrors(t, vars), vars)
		}
		// Undeclared names in a non-literal value are still reported.
		errs := initErrors(t, "n : INT := zz + 1;")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeUndeclared, errs[0].Code)
	})

	t.Run("unknown targets and bounds", func(t *testing.T) {
		errs := initErrors(t, "x : ST_Missing := (a := 1, zz := 2);")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeUndeclaredType, errs[0].Code)
		errs = initErrors(t, "arr : ARRAY[1..10] OF ST_Missing := [(a := 1), (b := 2)];")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeUndeclaredType, errs[0].Code)
		assert.Empty(t, initErrors(t, "arr : ARRAY[1..GVL.MAX] OF INT := [1, 2, 3, 4];"))
		assert.Empty(t, initErrors(t, "arr : ARRAY[0..1] OF INT := [C(0)];"))
	})

	t.Run("struct members and GVLs", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{
			{"types.st", initTypes},
			{"st.st", "TYPE ST_B : STRUCT\n\ta : ST_A := (zz := 1);\n\tn : INT := 'x';\n\tok : ARRAY[0..1] OF INT := [1, 2];\n\tk : INT := C;\nEND_STRUCT\nEND_TYPE\n"},
			{"G.st", "VAR_GLOBAL\n\tg : ARRAY[0..1] OF INT := [1, 2, 3];\n\tgs : ST_A := (a := 1);\nEND_VAR\n"},
			{"Q.st", "{attribute 'qualified_only'}\nVAR_GLOBAL\n\tq : INT := 'x';\nEND_VAR\n"},
		})
		assert.ElementsMatch(t, []string{CodeNoMember, CodeTypeMismatch, CodeTypeMismatch, CodeTypeMismatch}, codesOf(errorsOf(ds)))
	})

	t.Run("FUNCTION and FB var blocks", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{
			{"types.st", initTypes},
			{"f.st", "FUNCTION F : INT\nVAR_INPUT\n\ti : INT := 'x';\nEND_VAR\nF := i;\nEND_FUNCTION\nFUNCTION_BLOCK FB_X\nVAR\n\tarr : ARRAY[0..0] OF INT := [1, 2];\nEND_VAR\nEND_FUNCTION_BLOCK\n"},
		})
		assert.ElementsMatch(t, []string{CodeTypeMismatch, CodeTypeMismatch}, codesOf(errorsOf(ds)))
	})

	t.Run("array dimension bounds", func(t *testing.T) {
		_, table := runGVL(t, []gvlFile{{"t.st", "TYPE T_A : ARRAY[-2..16#3] OF INT; END_TYPE\nTYPE T_B : ARRAY[0..GVL.MAX] OF INT; END_TYPE\n"}})
		a := table.LookupGlobal("T_A").Type.(*types.ArrayType)
		assert.Equal(t, []types.ArrayDimension{{Low: -2, High: 3, Known: true}}, a.Dimensions)
		b := table.LookupGlobal("T_B").Type.(*types.ArrayType)
		assert.False(t, b.Dimensions[0].Known)
	})
}
