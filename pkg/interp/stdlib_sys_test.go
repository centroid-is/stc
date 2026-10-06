package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sysSrc = `
TYPE ST_Regs :
STRUCT
	a : ARRAY[0..3] OF WORD;
	r : REAL;
	l : LREAL;
	b : BOOL;
	s : STRING;
	t : TIME;
	p : POINTER TO INT;
END_STRUCT
END_TYPE
FUNCTION_BLOCK FB_X
VAR_OUTPUT q : DINT; e : LINT; END_VAR
END_FUNCTION_BLOCK
VAR_GLOBAL
	regs : ST_Regs;
	dw : DWORD;
	ws : WSTRING;
	wl : WSTRING(10);
	d : DATE;
END_VAR
PROGRAM MAIN
VAR
	nRegs, nType, nFB, nArr, nDw, nWs, nWl, nD, nInst, nExpr : UDINT;
	inst : FB_X;
	p : POINTER TO WORD;
	pw : POINTER TO DWORD;
	v : WORD;
	shl1, shr1, rol1, ror1, rol0 : WORD;
	si : SINT := -128;
	shs : SINT;
	big : LWORD;
	wrapped : SINT;
	u : UINT;
	b : BOOL;
	bb : BOOL;
	ud : UDINT;
END_VAR
nRegs := SIZEOF(regs);
nType := SIZEOF(ST_Regs);
nFB := SIZEOF(FB_X);
nInst := SIZEOF(inst);
nArr := SIZEOF(regs.a);
nDw := SIZEOF(dw);
nWs := SIZEOF(ws);
nWl := SIZEOF(wl);
nD := SIZEOF(d);
nExpr := SIZEOF(dw + 1);
p := ADR(regs.a[2]);
p^ := 16#BEEF;
v := p^;
pw := ADR(GVL.dw);
pw^ := 16#12345678;
shl1 := SHL(WORD#16#8001, 1);
shr1 := SHR(WORD#16#8001, 1);
rol1 := ROL(WORD#16#8001, 1);
ror1 := ROR(WORD#16#8001, 1);
rol0 := ROL(WORD#16#8001, 16);
shs := SHR(si, 1);
big := SHL(LWORD#1, 70);
wrapped := UINT_TO_SINT(200);
u := BOOL_TO_UINT(TRUE);
b := UINT_TO_BOOL(u);
bb := REAL_TO_BOOL(0.0);
ud := LREAL_TO_UDINT(3.6);
END_PROGRAM
`

func TestSystemFunctions(t *testing.T) {
	e := gvlEngine(t, "GVL.st", sysSrc)
	require.NoError(t, e.Tick(time.Millisecond))
	// WORD x4 = 8, REAL 4, LREAL 8, BOOL 1, STRING 81, TIME 4, POINTER 8
	assert.Equal(t, int64(114), intVar(t, e.env, "nRegs"))
	assert.Equal(t, int64(114), intVar(t, e.env, "nType"))
	assert.Equal(t, int64(12), intVar(t, e.env, "nFB"))
	assert.Equal(t, int64(12), intVar(t, e.env, "nInst"))
	assert.Equal(t, int64(8), intVar(t, e.env, "nArr"))
	assert.Equal(t, int64(4), intVar(t, e.env, "nDw"))
	assert.Equal(t, int64(162), intVar(t, e.env, "nWs"))
	assert.Equal(t, int64(162), intVar(t, e.env, "nWl"), "declared length is not tracked")
	assert.Equal(t, int64(4), intVar(t, e.env, "nD"))
	assert.Equal(t, int64(4), intVar(t, e.env, "nExpr"))

	assert.Equal(t, int64(0xBEEF), intVar(t, e.env, "v"))
	g := e.interp.lookupGVL("GVL")
	regs, _ := g.GetLocal("REGS")
	assert.Equal(t, int64(0xBEEF), regs.Struct["A"].Array[2].Int)
	dw, _ := g.GetLocal("DW")
	assert.Equal(t, int64(0x12345678), dw.Int)

	assert.Equal(t, int64(0x0002), intVar(t, e.env, "shl1"))
	assert.Equal(t, int64(0x4000), intVar(t, e.env, "shr1"))
	assert.Equal(t, int64(0x0003), intVar(t, e.env, "rol1"))
	assert.Equal(t, int64(0xC000), intVar(t, e.env, "ror1"))
	assert.Equal(t, int64(0x8001), intVar(t, e.env, "rol0"))
	assert.Equal(t, int64(0x40), intVar(t, e.env, "shs"), "SINT shifts its 8-bit pattern")
	assert.Equal(t, int64(0), intVar(t, e.env, "big"))
	assert.Equal(t, int64(-56), intVar(t, e.env, "wrapped"))
	assert.Equal(t, int64(1), intVar(t, e.env, "u"))
	assert.True(t, boolVar(t, e.env, "b"))
	assert.False(t, boolVar(t, e.env, "bb"))
	assert.Equal(t, int64(4), intVar(t, e.env, "ud"))
}

func TestSystemFunctionErrors(t *testing.T) {
	cases := map[string]string{
		"SIZEOF arity":   "n := SIZEOF(a, b);",
		"SIZEOF unknown": "n := SIZEOF(nope);",
		"ADR arity":      "n := ADR(a, b);",
		"ADR unknown":    "n := ADR(nope);",
		"ADR bad member": "n := ADR(s.nope);",
		"ADR not lvalue": "n := ADR(a + 1);",
		"SHL types":      "n := SHL(r, 1);",
		"to BOOL string": "b := STRING_TO_BOOL('x');",
		"wrap error":     "n := STRING_TO_SINT('x');",
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			e := gvlEngine(t, "P.st", `
TYPE ST_S : STRUCT x : INT; END_STRUCT END_TYPE
PROGRAM MAIN
VAR n : UDINT; a : INT; b : BOOL; r : REAL; s : ST_S; END_VAR
`+stmt+`
END_PROGRAM
`)
			assert.Error(t, e.Tick(time.Millisecond))
		})
	}
}

func TestSystemFunctionHelpers(t *testing.T) {
	assert.Equal(t, int64(4), sizeOfValue(Value{Kind: ValInt}, 0), "untyped integer")
	assert.Equal(t, int64(0), sizeOfValue(Value{Kind: ValFBInstance}, 0))
	assert.Equal(t, int64(0), sizeOfValue(Value{Kind: ValBool}, maxSizeofDepth+1))
	assert.Equal(t, int64(pointerSize), sizeOfValue(Value{Kind: ValReference}, 0))
	v := shiftOp("ROL", Value{Kind: ValInt, Int: -1 << 63}, 1)
	assert.Equal(t, int64(1), v.Int, "unknown type rotates 64 bits")
	v = shiftOp("SHL", Value{Kind: ValInt, Int: 1, IECType: types.KindINT}, 15)
	assert.Equal(t, int64(-32768), v.Int, "INT sign-extends")
	v = shiftOp("ROR", Value{Kind: ValInt, Int: 1, IECType: types.KindBYTE}, -1)
	assert.Equal(t, int64(2), v.Int, "negative rotate count")
	b, err := convertWrapped("X_TO_BOOL", types.KindBOOL, BoolValue(true))
	require.NoError(t, err)
	assert.True(t, b.Bool)
	r, err := convertWrapped("X_TO_REAL", types.KindREAL, Value{Kind: ValInt, Int: 3})
	require.NoError(t, err)
	assert.Equal(t, 3.0, r.Real)
	_, ok := (&Interpreter{TypeDecls: map[string]ast.TypeSpec{}}).sizeOfTypeName(nil, "nope")
	assert.False(t, ok)
}
