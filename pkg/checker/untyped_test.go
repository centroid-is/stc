package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseExpr parses src as the right-hand side of an assignment inside a
// throwaway program and returns that expression.
func parseExpr(t *testing.T, src string) ast.Expr {
	t.Helper()
	res := parser.Parse("e.st", "PROGRAM P\nx := "+src+";\nEND_PROGRAM\n")
	require.Empty(t, res.Diags, src)
	prog, ok := res.File.Declarations[0].(*ast.ProgramDecl)
	require.True(t, ok)
	require.Len(t, prog.Body, 1)
	return prog.Body[0].(*ast.AssignStmt).Value
}

func TestUntypedConst(t *testing.T) {
	tests := []struct {
		src    string
		kind   untypedKind
		val    int64
		hasVal bool
	}{
		{"1", untypedInt, 1, true},
		{"-5", untypedInt, -5, true},
		{"(3)", untypedInt, 3, true},
		{"2 * 3 + 1", untypedInt, 7, true},
		{"7 / 2", untypedInt, 3, true},
		{"7 MOD 4", untypedInt, 3, true},
		{"10 - 4", untypedInt, 6, true},
		{"16#FF", untypedInt, 255, true},
		{"2#1010", untypedInt, 10, true},
		{"9223372036854775807 + 1", untypedInt, 0, false},
		{"16#FFFF_FFFF_FFFF_FFFF", untypedInt, 0, false},
		{"1 / 0", untypedInt, 0, false},
		{"1 MOD 0", untypedInt, 0, false},
		{"-(-9223372036854775807 - 1)", untypedInt, 0, false},
		{"5.0", untypedReal, 0, false},
		{"1e3", untypedReal, 0, false},
		{"1 + 2.5", untypedReal, 0, false},
		{"-2.5", untypedReal, 0, false},
		{"INT#1", untypedNone, 0, false},
		{"x + 1", untypedNone, 0, false},
		{"1 + x", untypedNone, 0, false},
		{"NOT 1", untypedNone, 0, false},
		{"1 = 1", untypedNone, 0, false},
		{"2 ** 3", untypedNone, 0, false},
		{"TRUE", untypedNone, 0, false},
		{"'s'", untypedNone, 0, false},
		{"x", untypedNone, 0, false},
		{"16#FFFF_FFFF_FFFF_FFFF + 1", untypedInt, 0, false},
		{"-x", untypedNone, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			k, v, has := untypedConst(parseExpr(t, tt.src))
			assert.Equal(t, tt.kind, k)
			assert.Equal(t, tt.hasVal, has)
			if tt.hasVal {
				assert.Equal(t, tt.val, v)
			}
		})
	}
	k, _, _ := untypedConst(nil)
	assert.Equal(t, untypedNone, k)

	// Unary plus is not produced by the parser today; build it directly.
	plus := &ast.UnaryExpr{Op: ast.Token{Text: "+"}, Operand: parseExpr(t, "5")}
	k, v, has := untypedConst(plus)
	assert.Equal(t, untypedInt, k)
	assert.True(t, has)
	assert.Equal(t, int64(5), v)
}

func TestUntypedAssignable(t *testing.T) {
	prim := func(k types.TypeKind) types.Type { return &types.PrimitiveType{Kind_: k} }
	tests := []struct {
		src    string
		target types.TypeKind
		ok     bool
		msg    string
	}{
		{"300", types.KindBYTE, false, "constant 300 out of range for BYTE"},
		{"255", types.KindBYTE, true, ""},
		{"-1", types.KindUINT, false, "constant -1 out of range for UINT"},
		{"-1", types.KindBYTE, false, "constant -1 out of range for BYTE"},
		{"65535", types.KindWORD, true, ""},
		{"16#2001", types.KindWORD, true, ""},
		{"16#FFFFFFFF", types.KindDWORD, true, ""},
		{"16#1_0000_0000", types.KindDWORD, false, "constant 4294967296 out of range for DWORD"},
		{"16#FFFF_FFFF_FFFF_FFFF", types.KindLWORD, true, ""},
		{"16#FFFF_FFFF_FFFF_FFFF", types.KindULINT, true, ""},
		{"-128", types.KindSINT, true, ""},
		{"-129", types.KindSINT, false, "constant -129 out of range for SINT"},
		{"32768", types.KindINT, false, "constant 32768 out of range for INT"},
		{"-2147483648", types.KindDINT, true, ""},
		{"2147483648", types.KindDINT, false, "constant 2147483648 out of range for DINT"},
		{"-9223372036854775807", types.KindLINT, true, ""},
		{"256", types.KindUSINT, false, "constant 256 out of range for USINT"},
		{"4294967295", types.KindUDINT, true, ""},
		{"0", types.KindREAL, true, ""},
		{"0", types.KindLREAL, true, ""},
		{"5.0", types.KindREAL, true, ""},
		{"5.0", types.KindINT, false, ""},
		{"5.0", types.KindBYTE, false, ""},
		{"1", types.KindBOOL, false, ""},
		{"1", types.KindTIME, false, ""},
		{"1", types.KindSTRING, false, ""},
		{"INT#1", types.KindSINT, false, ""},
		{"x", types.KindINT, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.src+"->"+tt.target.String(), func(t *testing.T) {
			ok, msg := untypedAssignable(parseExpr(t, tt.src), prim(tt.target))
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.msg, msg)
		})
	}
	ok, _ := untypedAssignable(parseExpr(t, "1"), nil)
	assert.False(t, ok)
}

func TestIntRange(t *testing.T) {
	lo, hi := intRange(types.KindINT)
	assert.Equal(t, int64(-32768), lo)
	assert.Equal(t, uint64(32767), hi)
	lo, hi = intRange(types.KindLWORD)
	assert.Equal(t, int64(0), lo)
	assert.Equal(t, uint64(1<<64-1), hi)
	lo, hi = intRange(types.KindREAL)
	assert.Equal(t, int64(0), lo)
	assert.Equal(t, uint64(0), hi)
}

const untypedDecls = `FUNCTION_BLOCK FB_CoE
VAR_INPUT
	nIndexOffset : BYTE;
	nIndex : WORD;
END_VAR
METHOD M : BOOL
VAR_INPUT
	w : WORD;
END_VAR
END_METHOD
END_FUNCTION_BLOCK
FUNCTION F : BOOL
VAR_INPUT
	bIn : BYTE;
END_VAR
END_FUNCTION
TYPE E_Mode : (mA := 1, mB := 2) INT; END_TYPE
`

// untypedProg checks body inside PROGRAM P with a fixed set of variables
// and returns the error diagnostics.
func untypedProg(t *testing.T, body string) []diag.Diagnostic {
	t.Helper()
	vars := `a : INT; u : UDINT; index : UINT; n : USINT; ua : ARRAY[0..9] OF USINT;
	x : REAL; r : REAL; c : UDINT; b : BYTE; w : WORD; i : UINT; si : INT; d : DINT;
	arr : ARRAY[0..9] OF INT; buf : ARRAY[0..239] OF INT; s : UINT; p : UINT;
	fb : FB_CoE; ok : BOOL; m : E_Mode; li : LINT;`
	ds, _ := runGVL(t, []gvlFile{
		{"decls.st", untypedDecls},
		{"main.st", "PROGRAM P\nVAR\n" + vars + "\nEND_VAR\n" + body + "\nEND_PROGRAM\n"},
	})
	return errorsOf(ds)
}

func TestUntypedLiteral(t *testing.T) {
	clean := []string{
		"a := a + 1;",
		"a := 1 + a;",
		"u := u + 1;",
		"index := (index + 1) MOD 50;",
		"ua[n] := ua[n] + 1;",
		"x := UDINT_TO_REAL(c) / 5.0;",
		"x := r * 100.0;",
		"x := r + 1;",
		"fb(nIndexOffset := 16#02, nIndex := 16#2001);",
		"ok := F(bIn := 16#02);",
		"ok := F(16#FF);",
		"ok := fb.M(w := 16#2001);",
		"IF b <> 0 THEN a := 1; END_IF",
		"IF 0 = w THEN a := 1; END_IF",
		"CASE b AND 16#0F OF 1: a := 1; 2..3: a := 2; END_CASE",
		"b := b AND 16#0F; w := w OR b; w := w XOR 16#FFFF;",
		"a := arr[i - 1];",
		"a := buf[(240 + s - p) MOD 240];",
		"r := 0;",
		"w := 5;",
		"li := 9223372036854775807;",
		"m := 1;",
		"d := 2 * 3 + 1;",
	}
	for _, body := range clean {
		t.Run(body, func(t *testing.T) {
			assert.Empty(t, untypedProg(t, body))
		})
	}

	t.Run("initialiser and assignment share one rule", func(t *testing.T) {
		assert.Empty(t, initErrors(t, "r : REAL := 0;"))
		assert.Empty(t, initErrors(t, "r : REAL := 1 + 2;"))
		errs := initErrors(t, "b : BYTE := 256;")
		require.Len(t, errs, 1)
		assert.Equal(t, CodeTypeMismatch, errs[0].Code)
		assert.Equal(t, "constant 256 out of range for BYTE", errs[0].Message)
		errs = initErrors(t, "u : UINT := 1 - 2;")
		require.Len(t, errs, 1)
		assert.Equal(t, "constant -1 out of range for UINT", errs[0].Message)
		// Typed literals keep the CODESYS initialiser leniency for real and
		// bit-string targets but no longer narrow between integer kinds.
		assert.Empty(t, initErrors(t, "r : REAL := DINT#5;"))
		assert.Empty(t, initErrors(t, "w : WORD := UINT#5;"))
		errs = initErrors(t, "s : SINT := INT#5;")
		require.Len(t, errs, 1)
		assert.Equal(t, "cannot initialise SINT with INT", errs[0].Message)
		errs = initErrors(t, "i : INT := 5.0;")
		require.Len(t, errs, 1)
		assert.Equal(t, "cannot initialise INT with LREAL", errs[0].Message)
	})

	errCases := []struct {
		body, code, msg string
	}{
		{"b := 300;", CodeTypeMismatch, "constant 300 out of range for BYTE"},
		{"ok := F(bIn := 256);", CodeWrongArgType, "constant 256 out of range for BYTE"},
		{"fb(nIndexOffset := 256);", CodeWrongArgType, "constant 256 out of range for BYTE"},
		{"index := -1;", CodeTypeMismatch, "constant -1 out of range for UINT"},
		{"CASE b OF 256: a := 1; END_CASE", CodeTypeMismatch, "constant 256 out of range for BYTE"},
		{"si := 5.0;", CodeTypeMismatch, "cannot assign LREAL to INT"},
		{"ok := 1;", CodeTypeMismatch, "cannot assign DINT to BOOL"},
		{"a := 2.5 * a;", CodeTypeMismatch, "cannot assign LREAL to INT"},
		{"a := a + d;", CodeTypeMismatch, "cannot assign DINT to INT"},
		{"ok := ok AND 1;", CodeTypeMismatch, "boolean operator AND requires BOOL operands, got BOOL and DINT"},
		{"a := a AND a;", CodeTypeMismatch, "boolean operator AND requires BOOL operands, got INT and INT"},
	}
	for _, tt := range errCases {
		t.Run(tt.body, func(t *testing.T) {
			errs := untypedProg(t, tt.body)
			require.Len(t, errs, 1, "%v", errs)
			assert.Equal(t, tt.code, errs[0].Code)
			assert.Equal(t, tt.msg, errs[0].Message)
		})
	}
}
