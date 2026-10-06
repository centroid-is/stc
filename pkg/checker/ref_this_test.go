package checker

import (
	"fmt"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const refTypes = `TYPE ST_Batch : STRUCT
	x : INT;
	m : INT;
END_STRUCT
END_TYPE
`

// refErrors checks body inside PROGRAM P with the given VAR block and
// returns the error diagnostics.
func refErrors(t *testing.T, vars, body string) []diag.Diagnostic {
	t.Helper()
	ds, _ := runGVL(t, []gvlFile{
		{"types.st", refTypes},
		{"main.st", "PROGRAM P\nVAR\n" + vars + "\nEND_VAR\n" + body + "\nEND_PROGRAM\n"},
	})
	return errorsOf(ds)
}

func TestRefAssign(t *testing.T) {
	const vars = `r : REFERENCE TO INT; x : INT; y : REAL; a : INT; b : INT; i : INT; n : INT;
	arr : ARRAY[0..3] OF INT; s : ST_Batch; p : POINTER TO INT; r2 : REFERENCE TO INT;
	rr : REFERENCE TO REAL; sr : REFERENCE TO ST_Batch;`

	for _, body := range []string{
		"r REF= x;",
		"r REF= arr[i];",
		"r REF= s.m;",
		"r REF= p^;",
		"r REF= r2;",
		"r REF= (x);",
		"sr REF= s; sr.x := 1;",
	} {
		assert.Empty(t, refErrors(t, vars, body), body)
	}

	errs := refErrors(t, vars, "r REF= y;")
	require.Len(t, errs, 1)
	assert.Equal(t, CodeRefThisSuper, errs[0].Code)
	assert.Contains(t, errs[0].Message, "REAL")

	for _, body := range []string{"r REF= 5;", "r REF= a + b;", "r REF= x.3;"} {
		errs := refErrors(t, vars, body)
		require.Len(t, errs, 1, body)
		assert.Equal(t, CodeRefThisSuper, errs[0].Code, body)
		assert.Contains(t, errs[0].Message, "REF= requires a variable", body)
	}

	errs = refErrors(t, vars, "n REF= x;")
	require.Len(t, errs, 1)
	assert.Equal(t, CodeRefThisSuper, errs[0].Code)
	assert.Contains(t, errs[0].Message, "not a REFERENCE TO")

	errs = refErrors(t, vars, "s.m REF= x;")
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Message, "REF= target expression is not a REFERENCE TO")

	// Undeclared names on either side report SEMA010 only.
	assert.Equal(t, []string{CodeUndeclared}, codesOf(refErrors(t, vars, "zz REF= x;")))
	assert.Equal(t, []string{CodeUndeclared}, codesOf(refErrors(t, vars, "r REF= zz;")))
	assert.Equal(t, []string{CodeUndeclared}, codesOf(refErrors(t, vars, "zz REF= 5;")))
}

const thisSuperSrc = `FUNCTION_BLOCK FB_A
VAR_INPUT
	inA : INT;
END_VAR
VAR
	x : INT;
	p : POINTER TO FB_A;
END_VAR
METHOD M : INT
VAR_INPUT
	a : INT;
END_VAR
M := a;
END_METHOD
ACTION Act:
THIS^.x := 2;
END_ACTION
%s
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_B EXTENDS FB_A
VAR
	y : INT;
END_VAR
%s
END_FUNCTION_BLOCK
PROGRAM P
VAR
	n : INT;
END_VAR
%s
END_PROGRAM
FUNCTION F : INT
%s
F := 0;
END_FUNCTION
`

func thisSuperErrors(t *testing.T, fbA, fbB, prog, fn string) []diag.Diagnostic {
	t.Helper()
	src := fmt.Sprintf(thisSuperSrc, fbA, fbB, prog, fn)
	ds, _ := runGVL(t, []gvlFile{{"fb.st", src}})
	return errorsOf(ds)
}

func TestThisSuper(t *testing.T) {
	t.Run("THIS inside an FB", func(t *testing.T) {
		assert.Empty(t, thisSuperErrors(t, "THIS^.x := 1; x := THIS^.inA; p := THIS; THIS^.M(a := 1); x := THIS^.M(a := 2);", "", "", ""))
		errs := thisSuperErrors(t, "THIS^.x := 'x';", "", "", "")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeTypeMismatch, errs[0].Code)
		errs = thisSuperErrors(t, "THIS^.zz := 1;", "", "", "")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeNoMember, errs[0].Code)
	})

	t.Run("THIS in a derived FB sees inherited members", func(t *testing.T) {
		assert.Empty(t, thisSuperErrors(t, "", "THIS^.x := THIS^.y; THIS^.M(a := 1);", "", ""))
	})

	t.Run("THIS outside an FB", func(t *testing.T) {
		errs := thisSuperErrors(t, "", "", "n := THIS^.x;", "")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeRefThisSuper, errs[0].Code)
		assert.Contains(t, errs[0].Message, "THIS")
		errs = thisSuperErrors(t, "", "", "", "THIS^.x := 1;")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeRefThisSuper, errs[0].Code)
		errs = thisSuperErrors(t, "", "", "THIS^.M(a := 1);", "")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeRefThisSuper, errs[0].Code)
	})

	t.Run("SUPER with EXTENDS", func(t *testing.T) {
		assert.Empty(t, thisSuperErrors(t, "", "SUPER^.M(a := 1); SUPER^(); y := SUPER^.M(a := 1); SUPER^.x := 1; SUPER^(inA := 1);", "", ""))
	})

	t.Run("SUPER without EXTENDS", func(t *testing.T) {
		for _, body := range []string{"SUPER^();", "SUPER^.M(a := 1);", "x := SUPER^.x;"} {
			errs := thisSuperErrors(t, body, "", "", "")
			require.Len(t, errs, 1, body)
			assert.Equal(t, CodeRefThisSuper, errs[0].Code, body)
			assert.Contains(t, errs[0].Message, "EXTEND", body)
		}
		errs := thisSuperErrors(t, "", "", "SUPER^();", "")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeRefThisSuper, errs[0].Code)
	})

	t.Run("SUPER with a base that is not an FB", func(t *testing.T) {
		src := refTypes + "FUNCTION_BLOCK FB_C EXTENDS ST_Batch\nSUPER^();\nEND_FUNCTION_BLOCK\n"
		ds, _ := runGVL(t, []gvlFile{{"c.st", src}})
		assert.Equal(t, []string{CodeUndeclaredType}, codesOf(errorsOf(ds)))
	})

	t.Run("THIS member without a type", func(t *testing.T) {
		files := parseGVLFiles(t, []gvlFile{{"a.st", "FUNCTION_BLOCK FB_A\nTHIS^.ghost := 1;\nEND_FUNCTION_BLOCK\n"}})
		table := symbols.NewTable()
		diags := diag.NewCollector()
		NewResolver(table, diags).CollectDeclarations(files)
		require.NoError(t, table.LookupPOU("FB_A").Insert(&symbols.Symbol{Name: "ghost", Kind: symbols.KindProperty}))
		NewChecker(table, diags).CheckBodies(files)
		assert.Empty(t, errorsOf(diags.All()))
	})

	t.Run("SUPER with an undeclared base", func(t *testing.T) {
		src := "FUNCTION_BLOCK FB_C EXTENDS FB_Missing\nSUPER^();\nEND_FUNCTION_BLOCK\n"
		ds, _ := runGVL(t, []gvlFile{{"c.st", src}})
		assert.Equal(t, []string{CodeUndeclaredType}, codesOf(errorsOf(ds)))
	})
}

func TestRefAutoDeref(t *testing.T) {
	const vars = `r : REFERENCE TO ARRAY[0..3] OF ST_Batch; n : INT; d : DINT;
	rs : REFERENCE TO ST_Batch; ri : REFERENCE TO DINT; rr : REFERENCE TO ARRAY[0..3] OF ST_Batch;
	rb : REFERENCE TO BOOL; b : BOOL; ps : POINTER TO ST_Batch;`
	for _, body := range []string{
		"r[1].x := 1;",
		"n := r[0].x;",
		"rs.x := n;",
		"n := rs.m;",
		"d := ri + 1;",
		"d := ri;",
		"ri := d;",
		"ri := 5;",
		"rr := r;",
		"rr REF= r;",
		"b := NOT rb;",
		"d := -ri;",
		"IF ri > 0 THEN d := ri * 2; END_IF",
		"n := ps^.x;",
	} {
		assert.Empty(t, refErrors(t, vars, body), body)
	}
	// d := ri + 1 types as DINT: assigning it to a BOOL is a mismatch
	// naming DINT.
	errs := refErrors(t, vars, "b := ri + 1;")
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Message, "DINT")
	// Assigning a reference to a reference of another type stays an error.
	errs = refErrors(t, vars, "ri := rs;")
	require.Len(t, errs, 1)
	assert.Equal(t, CodeTypeMismatch, errs[0].Code)
}
