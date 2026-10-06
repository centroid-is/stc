package checker

import (
	"os"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enumTypes declares the enums the TestEnumAttr cases use: E_X and E_Y are
// plain, E_Q is qualified_only, E_S is strict with base UINT, E_T and E_U
// are non-strict with base INT.
const enumTypes = `TYPE E_X : (xa, xb, xc); END_TYPE
TYPE E_Y : (ya, yb); END_TYPE
{attribute 'qualified_only'}
TYPE E_Q : (q1, q2); END_TYPE
{attribute 'strict'}
TYPE E_S : (sa := 0, sb := 1, sc := 2) UINT; END_TYPE
{attribute 'strict'}
TYPE E_S2 : (s2a, s2b); END_TYPE
TYPE E_T : (t0, t1, t2) INT; END_TYPE
TYPE E_U : (u0, u1); END_TYPE
FUNCTION F_Int : INT
VAR_INPUT
	i : INT;
END_VAR
F_Int := i;
END_FUNCTION
`

const enumVars = `VAR
	x : E_X; q : E_Q; e : E_S; e2 : E_S2; f : E_T; g : E_U;
	n : INT; d : DINT; u : UINT; b : BOOL; s : STRING; r : REAL;
END_VAR
`

// enumErrors checks body inside PROGRAM P with enumTypes and enumVars and
// returns the error diagnostics.
func enumErrors(t *testing.T, body string) []diag.Diagnostic {
	t.Helper()
	ds, _ := runGVL(t, []gvlFile{
		{"types.st", enumTypes},
		{"main.st", "PROGRAM P\n" + enumVars + body + "\nEND_PROGRAM\n"},
	})
	return errorsOf(ds)
}

func TestEnumAttr(t *testing.T) {
	t.Run("enum_attr fixture", func(t *testing.T) {
		data, err := os.ReadFile("../../tests/twincat_probes/enum_attr.st")
		require.NoError(t, err)
		src := string(data) + "PROGRAM P\nVAR\n\ts : E_State;\nEND_VAR\ns := E_State.rdy;\nIF s = E_State.nst THEN s := E_State.rdy; END_IF\nEND_PROGRAM\n"
		ds, _ := runGVL(t, []gvlFile{{"enum_attr.st", src}})
		assert.Empty(t, errorsOf(ds))
	})

	t.Run("case labels", func(t *testing.T) {
		assert.Empty(t, enumErrors(t, "CASE x OF E_X.xa: n := 1; E_X.xb, E_X.xc: n := 2; END_CASE"))
		assert.Empty(t, enumErrors(t, "CASE x OF E_X.xa..E_X.xc: n := 1; END_CASE"))
		assert.Empty(t, enumErrors(t, "CASE x OF xa: n := 1; xb, xc: n := 2; END_CASE"))

		errs := enumErrors(t, "CASE x OF E_Y.ya: n := 1; END_CASE")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeTypeMismatch, errs[0].Code)
		assert.Contains(t, errs[0].Message, "E_Y")

		errs = enumErrors(t, "CASE x OF E_X.xa..E_Y.yb: n := 1; END_CASE")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeTypeMismatch, errs[0].Code)

		// A non-strict enum selector takes integer labels (ruling A4); a
		// strict one does not.
		assert.Empty(t, enumErrors(t, "CASE f OF 0: n := 1; 1..2: n := 2; END_CASE"))
		errs = enumErrors(t, "CASE e OF 0: n := 1; END_CASE")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeEnumRule, errs[0].Code)
		errs = enumErrors(t, "CASE e OF E_S.sa..2: n := 1; END_CASE")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeEnumRule, errs[0].Code)
		assert.Empty(t, enumErrors(t, "CASE e OF E_S.sa: n := 1; E_S.sb..E_S.sc: n := 2; END_CASE"))
		// An integer selector takes non-strict enum labels.
		assert.Empty(t, enumErrors(t, "CASE n OF E_T.t1: n := 1; END_CASE"))
	})

	t.Run("enum probe shape", func(t *testing.T) {
		src := `{attribute 'qualified_only'}
{attribute 'strict'}
TYPE E_S : (a := 0, b := 1, c := 2) INT;
END_TYPE
TYPE E_T : (x, y);
END_TYPE
PROGRAM MAIN
VAR e : E_S; f : E_T; END_VAR
e := E_S.b;
f := y;
IF e = E_S.c THEN f := x; END_IF
END_PROGRAM
`
		ds, _ := runGVL(t, []gvlFile{{"enum.st", src}})
		assert.Empty(t, errorsOf(ds))
	})

	t.Run("qualified_only bare value", func(t *testing.T) {
		errs := enumErrors(t, "q := q1;")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeEnumRule, errs[0].Code)
		assert.Equal(t, "enum value 'q1' of qualified_only enum E_Q must be written E_Q.q1", errs[0].Message)
		assert.Empty(t, enumErrors(t, "q := E_Q.q1; IF q = E_Q.q2 THEN q := E_Q.q1; END_IF"))
		// A name no enum declares stays SEMA010.
		errs = enumErrors(t, "n := nothing;")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeUndeclared, errs[0].Code)
	})

	t.Run("strict enum misuse", func(t *testing.T) {
		for _, body := range []string{
			"n := e + 1;",
			"n := 1 + e;",
			"u := e * e;",
			"n := e;",
			"u := e;",
			"e := 1;",
			"e := u;",
			"n := F_Int(e);",
			"n := F_Int(i := e);",
			"IF e = x THEN n := 1; END_IF",
			"IF f = e THEN n := 1; END_IF",
			"IF e = 1 THEN n := 1; END_IF",
			"e := x;",
		} {
			errs := enumErrors(t, body)
			require.Len(t, errs, 1, body)
			assert.Equal(t, CodeEnumRule, errs[0].Code, body)
			assert.Contains(t, errs[0].Message, "strict", body)
		}
		assert.Empty(t, enumErrors(t, "IF e = E_S.sa THEN e := E_S.sb; END_IF"))
		assert.Empty(t, enumErrors(t, "IF e <> E_S.sa THEN e := E_S.sb; END_IF"))
		assert.Empty(t, enumErrors(t, "e := sc;"))
	})

	t.Run("strict_explicit_conversion", func(t *testing.T) {
		assert.Empty(t, enumErrors(t, "n := UINT_TO_INT(e);"))
		assert.Empty(t, enumErrors(t, "n := TO_INT(e);"))
		assert.Empty(t, enumErrors(t, "d := TO_DINT(e2);"))
		assert.Empty(t, enumErrors(t, "s := TO_STRING(e);"))
		assert.Empty(t, enumErrors(t, "u := TO_UINT(E_S.sc);"))
	})

	t.Run("nonstrict_int_ops", func(t *testing.T) {
		assert.Empty(t, enumErrors(t, "n := f;"))
		assert.Empty(t, enumErrors(t, "f := 1;"))
		assert.Empty(t, enumErrors(t, "f := n;"))
		assert.Empty(t, enumErrors(t, "b := f = 1;"))
		assert.Empty(t, enumErrors(t, "b := f > 0;"))
		assert.Empty(t, enumErrors(t, "b := 0 < f;"))
		assert.Empty(t, enumErrors(t, "d := f + 1;"))
		assert.Empty(t, enumErrors(t, "n := F_Int(f);"))
		assert.Empty(t, enumErrors(t, "n := F_Int(i := E_T.t1);"))
		assert.Empty(t, enumErrors(t, "b := x = xa;"))
		// n := f + 1 is DINT -> INT, a literal-typing message (Phase 22),
		// never SEMA036.
		assert.Empty(t, diagsWithCode(enumErrors(t, "n := f + 1;"), CodeEnumRule))
		// Two different non-strict enums compare as today.
		assert.Empty(t, enumErrors(t, "b := f = g;"))
		// A non-strict enum widens like its base type: INT -> DINT.
		assert.Empty(t, enumErrors(t, "d := f;"))
		// INT does not narrow into a UINT-free enum base: DINT -> E_T (INT)
		// is a plain mismatch, not SEMA036.
		errs := enumErrors(t, "f := d;")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeTypeMismatch, errs[0].Code)
		// Arithmetic between a non-strict enum and REAL follows INT.
		assert.Empty(t, enumErrors(t, "r := f * 2.0;"))
	})

	t.Run("TO_STRING", func(t *testing.T) {
		for _, arg := range []string{"e", "f", "n", "u", "b", "r", "s", "E_Q.q1"} {
			assert.Empty(t, enumErrors(t, "s := TO_STRING("+arg+");"), arg)
		}
		errs := enumErrors(t, "n := TO_STRING(n);")
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "STRING")

		errs = enumErrors(t, "s := TO_STRING();")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeWrongArgCount, errs[0].Code)
		assert.Equal(t, types.TypeSTRING, types.BuiltinFunctions["TO_STRING"].ReturnType)
	})

	t.Run("enum initialiser", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{
			{"types.st", enumTypes},
			{"main.st", "PROGRAM P\nVAR\n\te : E_S := E_S.sb;\n\tq : E_Q := E_Q.q2;\nEND_VAR\ne := e; q := q;\nEND_PROGRAM\n"},
		})
		assert.Empty(t, errorsOf(ds))
	})

	t.Run("isConversionBuiltin", func(t *testing.T) {
		for _, name := range []string{"UINT_TO_INT", "to_int", "TO_STRING", "INT_TO_REAL"} {
			assert.True(t, isConversionBuiltin(name), name)
		}
		for _, name := range []string{"ABS", "LIMIT", "TO_NOTHING", "X_TO_Y", "TRUNC", ""} {
			assert.False(t, isConversionBuiltin(name), name)
		}
	})
}
