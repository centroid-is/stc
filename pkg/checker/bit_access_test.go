package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bitTypes = `TYPE ST_Dev : STRUCT
	q_w : WORD;
END_STRUCT
END_TYPE

TYPE ST_S : STRUCT
	w : WORD;
	cBit : BOOL;
END_STRUCT
END_TYPE
`

const bitECT = "VAR_GLOBAL\n\tdev : ST_Dev;\nEND_VAR\n"

const bitGVL = `VAR_GLOBAL CONSTANT
	cG : INT := 4;
	wc : WORD := 16#FF;
END_VAR
VAR_GLOBAL
	gw : WORD;
END_VAR
`

// runBits checks body inside a PROGRAM with one variable of each kind the
// bit access rules care about, plus local and GVL constants.
func runBits(t *testing.T, body string) []diag.Diagnostic {
	t.Helper()
	prog := `PROGRAM P
VAR
	b : BOOL;
	x : BOOL;
	w : WORD;
	d : DWORD;
	l : LWORD;
	i : INT;
	bt : BYTE;
	n : INT;
	r : REAL;
	s : ST_S;
	str : STRING;
	rw : REFERENCE TO WORD;
	nonConst : INT;
END_VAR
VAR CONSTANT
	cBit : INT := 3;
	cBig : INT := 16;
	cHex : UINT := 16#2;
	cNeg : INT := -1;
END_VAR
` + body + "\nEND_PROGRAM\n"
	ds, _ := runGVL(t, []gvlFile{
		{"types.st", bitTypes},
		{"ECT.st", bitECT},
		{"GVL.st", bitGVL},
		{"main.st", prog},
	})
	return errorsOf(ds)
}

func requireCodes(t *testing.T, errs []diag.Diagnostic, code string, n int) []diag.Diagnostic {
	t.Helper()
	got := diagsWithCode(errs, code)
	require.Len(t, got, n, "want %d %s, all errors: %v", n, code, errs)
	require.Len(t, errs, n, "unexpected extra errors: %v", errs)
	return got
}

func TestBitAccess(t *testing.T) {
	t.Run("literal_index_read", func(t *testing.T) {
		errs := runBits(t, `b := w.3;
b := d.31;
b := l.63;
b := i.15;
b := bt.7;
b := bt.0 AND w.15;
b := ECT.dev.q_w.0;
b := s.w.2;`)
		assert.Empty(t, errs)
	})

	t.Run("literal_index_write", func(t *testing.T) {
		errs := runBits(t, `w.3 := TRUE;
s.w.2 := x;
ECT.dev.q_w.1 := b;
gw.0 := TRUE;`)
		assert.Empty(t, errs)
	})

	t.Run("index out of range", func(t *testing.T) {
		errs := runBits(t, "b := w.16;\nb := bt.8;")
		got := requireCodes(t, errs, CodeBitAccess, 2)
		assert.Contains(t, got[0].Message, "bit index 16 out of range for WORD (0..15)")
		assert.Contains(t, got[1].Message, "bit index 8 out of range for BYTE (0..7)")
	})

	t.Run("index overflow is out of range", func(t *testing.T) {
		errs := runBits(t, "b := w.99999999999999999999;")
		got := requireCodes(t, errs, CodeBitAccess, 1)
		assert.Contains(t, got[0].Message, "99999999999999999999")
	})

	t.Run("write out of range", func(t *testing.T) {
		errs := runBits(t, "w.16 := TRUE;")
		requireCodes(t, errs, CodeBitAccess, 1)
	})

	t.Run("non-integer targets", func(t *testing.T) {
		errs := runBits(t, "b := x.0;\nb := r.0;\nb := str.0;\nb := s.0;")
		got := requireCodes(t, errs, CodeBitAccess, 4)
		assert.Contains(t, got[0].Message, "bit access on non-integer type BOOL")
		assert.Contains(t, got[1].Message, "bit access on non-integer type REAL")
		assert.Contains(t, got[2].Message, "bit access on non-integer type STRING")
		assert.Contains(t, got[3].Message, "bit access on non-integer type ST_S")
	})

	t.Run("undeclared target adds no SEMA035", func(t *testing.T) {
		errs := runBits(t, "b := nope.0;")
		requireCodes(t, errs, CodeUndeclared, 1)
	})

	t.Run("expression type is BOOL", func(t *testing.T) {
		errs := runBits(t, "n := w.3 + 1;")
		requireCodes(t, errs, CodeIncompatibleOp, 1)
		errs = runBits(t, "n := w.3;")
		requireCodes(t, errs, CodeTypeMismatch, 1)
	})

	t.Run("reference target is dereferenced", func(t *testing.T) {
		errs := runBits(t, "b := rw.2;\nrw.1 := TRUE;\nb := rw.cBit;")
		assert.Empty(t, errs)
		errs = runBits(t, "b := rw.16;")
		requireCodes(t, errs, CodeBitAccess, 1)
	})

	t.Run("const_index_read", func(t *testing.T) {
		errs := runBits(t, "b := w.cBit;\nb := d.cG;\nb := i.cHex;\nb := s.w.cBit;\nIF w.cBit THEN b := TRUE; END_IF")
		assert.Empty(t, errs)
	})

	t.Run("const_index_read out of range", func(t *testing.T) {
		errs := runBits(t, "b := w.cBig;\nb := bt.cNeg;")
		got := requireCodes(t, errs, CodeBitAccess, 2)
		assert.Contains(t, got[0].Message, "bit index 16 out of range for WORD (0..15)")
		assert.Contains(t, got[1].Message, "bit index -1 out of range for BYTE (0..7)")
	})

	t.Run("const_index_write", func(t *testing.T) {
		errs := runBits(t, "w.cBit := TRUE;\ns.w.cBit := x;\nd.cG := b;")
		assert.Empty(t, errs)
	})

	t.Run("const_index_write out of range", func(t *testing.T) {
		errs := runBits(t, "w.cBig := TRUE;")
		requireCodes(t, errs, CodeBitAccess, 1)
	})

	t.Run("const_index_write type mismatch", func(t *testing.T) {
		errs := runBits(t, "w.cBit := 5;")
		got := requireCodes(t, errs, CodeTypeMismatch, 1)
		assert.Contains(t, got[0].Message, "to BOOL")
	})

	t.Run("non-constant member on an integer keeps the no-member error", func(t *testing.T) {
		errs := runBits(t, "b := w.nonConst;")
		got := requireCodes(t, errs, CodeNoMember, 1)
		assert.Contains(t, got[0].Message, "does not support member access")
	})

	t.Run("struct member with a constant's name keeps its meaning", func(t *testing.T) {
		errs := runBits(t, "b := s.cBit;\ns.cBit := TRUE;")
		assert.Empty(t, errs)
		errs = runBits(t, "n := s.cBit;")
		requireCodes(t, errs, CodeTypeMismatch, 1)
	})

	t.Run("write to a constant through bit access", func(t *testing.T) {
		errs := runBits(t, "wc.3 := TRUE;")
		got := requireCodes(t, errs, CodeAssignToConstant, 1)
		assert.Contains(t, got[0].Message, "'wc'")
		errs = runBits(t, "GVL.wc.3 := TRUE;\nGVL.wc.cBit := TRUE;\nwc.cBit.0 := TRUE;")
		requireCodes(t, errs, CodeAssignToConstant, 2)
	})

	t.Run("write to a GVL constant used as index is not a constant write", func(t *testing.T) {
		errs := runBits(t, "w.cG := TRUE;\nGVL.cG := 1;")
		requireCodes(t, errs, CodeAssignToConstant, 1)
	})

	t.Run("bit access marks the target used", func(t *testing.T) {
		src := "PROGRAM P\nVAR\n\tw : WORD;\n\tb : BOOL;\nEND_VAR\nVAR CONSTANT\n\tc : INT := 1;\nEND_VAR\nb := w.0 OR w.c;\nEND_PROGRAM\n"
		ds, _ := runGVL(t, []gvlFile{{"main.st", src}})
		assert.Empty(t, ds)
	})

	t.Run("bit access on a member callee root marks it used", func(t *testing.T) {
		src := "PROGRAM P\nVAR\n\tarr : ARRAY[0..1] OF WORD;\n\tb : BOOL;\nEND_VAR\nb := arr[0].3;\nEND_PROGRAM\n"
		ds, _ := runGVL(t, []gvlFile{{"main.st", src}})
		assert.Empty(t, ds)
	})
}
