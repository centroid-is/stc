package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bitRun parses src as file filename, runs one scan and returns the engine.
func bitRun(t *testing.T, filename, src string) *ScanCycleEngine {
	t.Helper()
	eng := gvlEngine(t, filename, src)
	require.NoError(t, eng.Tick(10*time.Millisecond))
	return eng
}

// bitRunErr parses src, runs one scan and returns the runtime error.
func bitRunErr(t *testing.T, src string) error {
	t.Helper()
	eng := gvlEngine(t, "P.st", src)
	err := eng.Tick(10 * time.Millisecond)
	require.Error(t, err)
	var rt *RuntimeError
	require.ErrorAs(t, err, &rt)
	return err
}

func progVar(t *testing.T, eng *ScanCycleEngine, name string) Value {
	t.Helper()
	v, ok := eng.env.Get(name)
	require.True(t, ok, "no variable %s", name)
	return v
}

func TestBitAccess(t *testing.T) {
	t.Run("read literal index", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
PROGRAM P
VAR w : WORD := 16#0009; h : WORD := 16#8000; b0, b1, b3, b15 : BOOL; END_VAR
b0 := w.0; b1 := w.1; b3 := w.3; b15 := h.15;
END_PROGRAM
`)
		assert.True(t, progVar(t, eng, "b0").Bool)
		assert.False(t, progVar(t, eng, "b1").Bool)
		assert.True(t, progVar(t, eng, "b3").Bool)
		assert.True(t, progVar(t, eng, "b15").Bool)
	})

	t.Run("read in expressions", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
PROGRAM P
VAR w : WORD := 16#0002; ok : BOOL; END_VAR
IF w.1 AND NOT w.0 THEN ok := TRUE; END_IF
END_PROGRAM
`)
		assert.True(t, progVar(t, eng, "ok").Bool)
	})

	t.Run("write keeps type and other bits", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
PROGRAM P
VAR w : WORD; b : BYTE; d : DWORD; l : LWORD; i : INT; END_VAR
w.0 := TRUE; w.3 := TRUE; w.3 := FALSE;
b.7 := TRUE;
d.31 := TRUE;
l.63 := TRUE;
i.15 := TRUE;
END_PROGRAM
`)
		w := progVar(t, eng, "w")
		assert.Equal(t, int64(1), w.Int)
		assert.Equal(t, types.KindWORD, w.IECType)
		assert.Equal(t, int64(128), progVar(t, eng, "b").Int)
		d := progVar(t, eng, "d")
		assert.Equal(t, int64(0x80000000), d.Int, "DWORD bit 31 stays unsigned")
		assert.Equal(t, types.KindDWORD, d.IECType)
		assert.Equal(t, uint64(1)<<63, uint64(progVar(t, eng, "l").Int))
		assert.Equal(t, int64(-32768), progVar(t, eng, "i").Int, "INT bit 15 is the sign bit")
	})

	t.Run("write integer value uses truthiness", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
PROGRAM P
VAR w : WORD; END_VAR
w.2 := 1;
END_PROGRAM
`)
		assert.Equal(t, int64(4), progVar(t, eng, "w").Int)
	})

	t.Run("struct member and array element targets", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
TYPE ST_S : STRUCT w : WORD; END_STRUCT END_TYPE
PROGRAM P
VAR s : ST_S; arr : ARRAY[0..2] OF BYTE; r1, r2 : BOOL; END_VAR
s.w.2 := TRUE;
arr[1].0 := TRUE;
arr[1].3 := TRUE;
r1 := s.w.2;
r2 := arr[1].3;
END_PROGRAM
`)
		assert.Equal(t, int64(4), progVar(t, eng, "s").Struct["W"].Int)
		arr := progVar(t, eng, "arr")
		assert.Equal(t, int64(9), arr.Array[1].Int)
		assert.Equal(t, int64(0), arr.Array[0].Int)
		assert.True(t, progVar(t, eng, "r1").Bool)
		assert.True(t, progVar(t, eng, "r2").Bool)
	})

	t.Run("nested GVL read", func(t *testing.T) {
		eng := bitRun(t, "ECT.st", `
TYPE ST_Dev : STRUCT q_wDigitalInputs : WORD; END_STRUCT END_TYPE
VAR_GLOBAL dev : ST_Dev; END_VAR
PROGRAM P
VAR x, y : BOOL; END_VAR
ECT.dev.q_wDigitalInputs := 16#0001;
x := ECT.dev.q_wDigitalInputs.0;
y := ECT.dev.q_wDigitalInputs.1;
END_PROGRAM
`)
		assert.True(t, progVar(t, eng, "x").Bool)
		assert.False(t, progVar(t, eng, "y").Bool)
	})

	t.Run("GVL member write", func(t *testing.T) {
		eng := bitRun(t, "G.st", `
VAR_GLOBAL w : WORD; END_VAR
PROGRAM P
G.w.4 := TRUE;
END_PROGRAM
`)
		assert.Equal(t, int64(16), gvlVar(t, eng, "G", "w").Int)
	})

	t.Run("FB output read", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
FUNCTION_BLOCK FB_Out
VAR_OUTPUT q_w : WORD; END_VAR
q_w := 16#0002;
END_FUNCTION_BLOCK
PROGRAM P
VAR fb : FB_Out; b : BOOL; END_VAR
fb();
b := fb.q_w.1;
END_PROGRAM
`)
		assert.True(t, progVar(t, eng, "b").Bool)
	})

	t.Run("const_index_read local and GVL constant", func(t *testing.T) {
		eng := bitRun(t, "G.st", `
VAR_GLOBAL CONSTANT cG : INT := 2; END_VAR
PROGRAM P
VAR CONSTANT cBit : INT := 3; END_VAR
VAR w : WORD := 16#000C; a, b : BOOL; END_VAR
a := w.cBit;
b := w.cG;
END_PROGRAM
`)
		assert.True(t, progVar(t, eng, "a").Bool)
		assert.True(t, progVar(t, eng, "b").Bool)
	})

	t.Run("const_index_write set and clear", func(t *testing.T) {
		eng := gvlEngine(t, "P.st", `
PROGRAM P
VAR CONSTANT cBit : INT := 3; END_VAR
VAR w : WORD; clear : BOOL; END_VAR
IF clear THEN w.cBit := FALSE; ELSE w.cBit := TRUE; END_IF
END_PROGRAM
`)
		require.NoError(t, eng.Tick(10*time.Millisecond))
		w := progVar(t, eng, "w")
		assert.Equal(t, int64(8), w.Int)
		assert.Equal(t, types.KindWORD, w.IECType)
		eng.env.Set("clear", BoolValue(true))
		require.NoError(t, eng.Tick(10*time.Millisecond))
		assert.Equal(t, int64(0), progVar(t, eng, "w").Int)
	})

	t.Run("const_index_write struct and GVL member", func(t *testing.T) {
		eng := bitRun(t, "G.st", `
TYPE ST_S : STRUCT w : WORD; END_STRUCT END_TYPE
VAR_GLOBAL w : WORD; END_VAR
PROGRAM P
VAR CONSTANT cBit : INT := 3; END_VAR
VAR s : ST_S; END_VAR
s.w.cBit := TRUE;
G.w.cBit := TRUE;
END_PROGRAM
`)
		assert.Equal(t, int64(8), progVar(t, eng, "s").Struct["W"].Int)
		assert.Equal(t, int64(8), gvlVar(t, eng, "G", "w").Int)
	})

	t.Run("const_index_write out of range", func(t *testing.T) {
		err := bitRunErr(t, `
PROGRAM P
VAR CONSTANT cBig : INT := 16; END_VAR
VAR w : WORD; END_VAR
w.cBig := TRUE;
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("const_index_read out of range", func(t *testing.T) {
		err := bitRunErr(t, `
PROGRAM P
VAR CONSTANT cBig : INT := 8; END_VAR
VAR b : BYTE; x : BOOL; END_VAR
x := b.cBig;
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("struct member named like a constant stays a member", func(t *testing.T) {
		eng := bitRun(t, "P.st", `
TYPE ST_S : STRUCT cBit : INT; END_STRUCT END_TYPE
PROGRAM P
VAR CONSTANT cBit : INT := 3; END_VAR
VAR s : ST_S; n : INT; END_VAR
s.cBit := 5;
n := s.cBit;
END_PROGRAM
`)
		assert.Equal(t, int64(5), progVar(t, eng, "s").Struct["CBIT"].Int)
		assert.Equal(t, int64(5), progVar(t, eng, "n").Int)
	})

	t.Run("member on integer without constant is an error", func(t *testing.T) {
		err := bitRunErr(t, `
PROGRAM P
VAR w : WORD; x : BOOL; END_VAR
x := w.nothing;
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "cannot access member")
		err = bitRunErr(t, `
PROGRAM P
VAR w : WORD; END_VAR
w.nothing := TRUE;
END_PROGRAM
`)
		assert.Contains(t, err.Error(), "cannot assign member")
	})

	t.Run("errors on non-integer targets", func(t *testing.T) {
		for _, src := range []string{
			"PROGRAM P\nVAR r : REAL; x : BOOL; END_VAR\nx := r.0;\nEND_PROGRAM\n",
			"PROGRAM P\nVAR s : STRING; x : BOOL; END_VAR\nx := s.0;\nEND_PROGRAM\n",
			"PROGRAM P\nVAR x : BOOL; END_VAR\nx.0 := TRUE;\nEND_PROGRAM\n",
			"TYPE ST_S : STRUCT a : INT; END_STRUCT END_TYPE\nPROGRAM P\nVAR s : ST_S; x : BOOL; END_VAR\nx := s.0;\nEND_PROGRAM\n",
		} {
			err := bitRunErr(t, src)
			assert.Contains(t, err.Error(), "bit access on non-integer")
		}
	})

	t.Run("errors on index out of range", func(t *testing.T) {
		for _, src := range []string{
			"PROGRAM P\nVAR w : WORD; x : BOOL; END_VAR\nx := w.16;\nEND_PROGRAM\n",
			"PROGRAM P\nVAR b : BYTE; END_VAR\nb.8 := TRUE;\nEND_PROGRAM\n",
			"PROGRAM P\nVAR l : LWORD; x : BOOL; END_VAR\nx := l.64;\nEND_PROGRAM\n",
		} {
			err := bitRunErr(t, src)
			assert.Contains(t, err.Error(), "out of range")
		}
	})

	t.Run("errors propagate from target and huge index", func(t *testing.T) {
		err := bitRunErr(t, "PROGRAM P\nVAR x : BOOL; END_VAR\nx := nope.0;\nEND_PROGRAM\n")
		assert.Contains(t, err.Error(), "undefined variable")
		err = bitRunErr(t, "PROGRAM P\nVAR w : WORD; END_VAR\nnope.0 := TRUE;\nEND_PROGRAM\n")
		assert.Contains(t, err.Error(), "undefined variable")
		err = bitRunErr(t, "PROGRAM P\nVAR w : WORD; x : BOOL; END_VAR\nx := w.99999999999999999999;\nEND_PROGRAM\n")
		assert.Contains(t, err.Error(), "invalid integer literal")
		err = bitRunErr(t, "PROGRAM P\nVAR w : WORD; END_VAR\nw.99999999999999999999 := TRUE;\nEND_PROGRAM\n")
		assert.Contains(t, err.Error(), "invalid integer literal")
	})

	t.Run("negative and non-integer index", func(t *testing.T) {
		in := New()
		env := NewEnv(nil)
		env.Define("w", Value{Kind: ValInt, Int: 1, IECType: types.KindWORD})
		w := &ast.Ident{Name: "w"}
		neg := &ast.BitAccessExpr{Target: w, Index: &ast.UnaryExpr{
			Op:      ast.Token{Text: "-"},
			Operand: &ast.Literal{LitKind: ast.LitInt, Value: "1"},
		}}
		_, err := in.evalExpr(env, neg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")

		real := &ast.BitAccessExpr{Target: w, Index: &ast.Literal{LitKind: ast.LitReal, Value: "1.5"}}
		_, err = in.evalExpr(env, real)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bit index must be an integer")
		require.Error(t, in.assignToTarget(env, real, BoolValue(true)))

		bad := &ast.BitAccessExpr{Target: w, Index: &ast.Ident{Name: "missing"}}
		_, err = in.evalExpr(env, bad)
		require.Error(t, err)
	})

	t.Run("integer with unknown IEC type uses 64 bits", func(t *testing.T) {
		in := New()
		env := NewEnv(nil)
		env.Define("n", Value{Kind: ValInt, Int: 0})
		e := &ast.BitAccessExpr{Target: &ast.Ident{Name: "n"}, Index: &ast.Literal{LitKind: ast.LitInt, Value: "40"}}
		require.NoError(t, in.assignToTarget(env, e, BoolValue(true)))
		n, _ := env.Get("n")
		assert.Equal(t, int64(1)<<40, n.Int)
		v, err := in.evalExpr(env, e)
		require.NoError(t, err)
		assert.True(t, v.Bool)
	})

	t.Run("isAssignable", func(t *testing.T) {
		assert.True(t, isAssignable(&ast.BitAccessExpr{}))
	})

	t.Run("bitWidth table", func(t *testing.T) {
		for k, w := range map[types.TypeKind]int{
			types.KindBYTE: 8, types.KindSINT: 8, types.KindUSINT: 8,
			types.KindWORD: 16, types.KindINT: 16, types.KindUINT: 16,
			types.KindDWORD: 32, types.KindDINT: 32, types.KindUDINT: 32,
			types.KindLWORD: 64, types.KindLINT: 64, types.KindULINT: 64,
			types.KindREAL: 0, types.KindBOOL: 0,
		} {
			assert.Equal(t, w, bitWidth(k), k.String())
		}
	})
}
