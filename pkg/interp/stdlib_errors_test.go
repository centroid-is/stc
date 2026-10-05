package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/iomap"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stdlibErrCase is one direct call into StdlibFunctions. Either errSub is set
// (the call must fail with that substring) or want is checked field by field.
type stdlibErrCase struct {
	name   string
	fn     string
	args   []Value
	errSub string
	want   Value
}

func runStdlibErrCases(t *testing.T, cases []stdlibErrCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn, ok := StdlibFunctions[tc.fn]
			require.True(t, ok, "%s not registered", tc.fn)
			got, err := fn(tc.args)
			if tc.errSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSub)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want.Kind, got.Kind)
			switch tc.want.Kind {
			case ValInt:
				assert.Equal(t, tc.want.Int, got.Int)
			case ValReal:
				assert.InDelta(t, tc.want.Real, got.Real, 1e-9)
			case ValString:
				assert.Equal(t, tc.want.Str, got.Str)
			}
			if tc.want.IECType != 0 {
				assert.Equal(t, tc.want.IECType, got.IECType)
			}
		})
	}
}

func arrayOf(n int) Value {
	elems := make([]Value, n)
	for i := range elems {
		elems[i] = IntValue(int64(i))
	}
	return Value{Kind: ValArray, Array: elems}
}

func TestStdlibErrUpperBound(t *testing.T) {
	runStdlibErrCases(t, []stdlibErrCase{
		{name: "no args", fn: "UPPER_BOUND", errSub: "requires an array"},
		{name: "non-array", fn: "UPPER_BOUND", args: []Value{IntValue(3)}, errSub: "expects an array, got"},
		{name: "non-int dim", fn: "UPPER_BOUND", args: []Value{arrayOf(4), StringValue("1")}, errSub: "dimension must be an integer"},
		{name: "dim 2 on 1-D", fn: "UPPER_BOUND", args: []Value{arrayOf(4), IntValue(2)}, errSub: "only dimension 1 is supported, got 2"},
		{name: "valid dim 1", fn: "UPPER_BOUND", args: []Value{arrayOf(11), IntValue(1)}, want: Value{Kind: ValInt, Int: 10, IECType: types.KindDINT}},
		{name: "valid no dim", fn: "UPPER_BOUND", args: []Value{arrayOf(6)}, want: Value{Kind: ValInt, Int: 5, IECType: types.KindDINT}},
	})
}

func TestStdlibErrLowerBoundAbsent(t *testing.T) {
	// LOWER_BOUND is deliberately not provided (see stdlib_array.go).
	_, ok := StdlibFunctions["LOWER_BOUND"]
	assert.False(t, ok, "LOWER_BOUND must stay unregistered")
}

func TestStdlibErrConversionArgCount(t *testing.T) {
	cases := []stdlibErrCase{
		{name: "UDINT_TO_REAL", fn: "UDINT_TO_REAL", errSub: "conversion requires 1 argument"},
		{name: "UINT_TO_REAL", fn: "UINT_TO_REAL", errSub: "conversion requires 1 argument"},
		{name: "USINT_TO_REAL", fn: "USINT_TO_REAL", errSub: "conversion requires 1 argument"},
		{name: "REAL_TO_LREAL", fn: "REAL_TO_LREAL", errSub: "REAL_TO_LREAL requires 1 argument"},
		{name: "LREAL_TO_REAL", fn: "LREAL_TO_REAL", errSub: "LREAL_TO_REAL requires 1 argument"},
		{name: "TIME_TO_REAL", fn: "TIME_TO_REAL", errSub: "TIME_TO_REAL requires 1 argument"},
		{name: "TRUNC", fn: "TRUNC", errSub: "TRUNC requires 1 argument"},
		{name: "TRUNC_INT", fn: "TRUNC_INT", errSub: "TRUNC requires 1 argument"},
		{name: "TIME_TO_DINT", fn: "TIME_TO_DINT", errSub: "TIME_TO_DINT requires 1 argument"},
		{name: "TIME_TO_LREAL", fn: "TIME_TO_LREAL", errSub: "TIME_TO_LREAL requires 1 argument"},
		// Happy paths alongside, so the returned IEC type is pinned too.
		{name: "UDINT_TO_REAL ok", fn: "UDINT_TO_REAL", args: []Value{IntValue(7)}, want: Value{Kind: ValReal, Real: 7, IECType: types.KindREAL}},
		{name: "REAL_TO_LREAL ok", fn: "REAL_TO_LREAL", args: []Value{RealValue(1.5)}, want: Value{Kind: ValReal, Real: 1.5, IECType: types.KindLREAL}},
		{name: "LREAL_TO_REAL ok", fn: "LREAL_TO_REAL", args: []Value{RealValue(2.25)}, want: Value{Kind: ValReal, Real: 2.25, IECType: types.KindREAL}},
		{name: "TIME_TO_REAL ok", fn: "TIME_TO_REAL", args: []Value{TimeValue(1500 * time.Millisecond)}, want: Value{Kind: ValReal, Real: 1500, IECType: types.KindREAL}},
		{name: "TIME_TO_DINT ok", fn: "TIME_TO_DINT", args: []Value{TimeValue(2 * time.Second)}, want: Value{Kind: ValInt, Int: 2000, IECType: types.KindDINT}},
		{name: "TIME_TO_LREAL ok", fn: "TIME_TO_LREAL", args: []Value{TimeValue(250 * time.Microsecond)}, want: Value{Kind: ValReal, Real: 0.25, IECType: types.KindLREAL}},
		{name: "TRUNC_DINT ok", fn: "TRUNC_DINT", args: []Value{RealValue(-3.9)}, want: Value{Kind: ValInt, Int: -3, IECType: types.KindDINT}},
	}
	runStdlibErrCases(t, cases)
}

func TestStdlibErrString(t *testing.T) {
	runStdlibErrCases(t, []stdlibErrCase{
		{name: "INSERT few args", fn: "INSERT", args: []Value{StringValue("ab"), StringValue("x")}, errSub: "INSERT requires 3 arguments"},
		{name: "INSERT pos 0", fn: "INSERT", args: []Value{StringValue("abc"), StringValue("X"), IntValue(0)}, want: StringValue("abc")},
		{name: "INSERT past end", fn: "INSERT", args: []Value{StringValue("abc"), StringValue("X"), IntValue(99)}, want: StringValue("abcX")},
		{name: "INSERT middle", fn: "INSERT", args: []Value{StringValue("abc"), StringValue("X"), IntValue(2)}, want: StringValue("aXbc")},

		{name: "DELETE few args", fn: "DELETE", args: []Value{StringValue("abc"), IntValue(1)}, errSub: "DELETE requires 3 arguments"},
		{name: "DELETE pos 0", fn: "DELETE", args: []Value{StringValue("abc"), IntValue(1), IntValue(0)}, want: StringValue("abc")},
		{name: "DELETE len 0", fn: "DELETE", args: []Value{StringValue("abc"), IntValue(0), IntValue(1)}, want: StringValue("abc")},
		{name: "DELETE pos past end", fn: "DELETE", args: []Value{StringValue("abc"), IntValue(1), IntValue(4)}, want: StringValue("abc")},
		{name: "DELETE len overruns", fn: "DELETE", args: []Value{StringValue("abcdef"), IntValue(99), IntValue(3)}, want: StringValue("ab")},

		{name: "REPLACE few args", fn: "REPLACE", args: []Value{StringValue("abc"), StringValue("X"), IntValue(1)}, errSub: "REPLACE requires 4 arguments"},
		{name: "REPLACE pos 0", fn: "REPLACE", args: []Value{StringValue("abc"), StringValue("X"), IntValue(1), IntValue(0)}, want: StringValue("abc")},
		{name: "REPLACE pos past end", fn: "REPLACE", args: []Value{StringValue("abc"), StringValue("X"), IntValue(1), IntValue(4)}, want: StringValue("abc")},
		{name: "REPLACE len overruns", fn: "REPLACE", args: []Value{StringValue("abcdef"), StringValue("XY"), IntValue(99), IntValue(5)}, want: StringValue("abcdXY")},
	})
}

func TestStdlibErrMath(t *testing.T) {
	runStdlibErrCases(t, []stdlibErrCase{
		{name: "MIN int returns b", fn: "MIN", args: []Value{IntValue(9), IntValue(4)}, want: Value{Kind: ValInt, Int: 4}},
		{name: "MAX real returns a", fn: "MAX", args: []Value{RealValue(3.5), IntValue(1)}, want: Value{Kind: ValReal, Real: 3.5, IECType: types.KindLREAL}},
		{name: "MAX int returns a", fn: "MAX", args: []Value{IntValue(9), IntValue(4)}, want: Value{Kind: ValInt, Int: 9}},
		{name: "MAX int tie returns a", fn: "MAX", args: []Value{IntValue(4), IntValue(4)}, want: Value{Kind: ValInt, Int: 4}},
		{name: "LIMIT real clamps high", fn: "LIMIT", args: []Value{RealValue(0), RealValue(12.5), RealValue(10)}, want: Value{Kind: ValReal, Real: 10, IECType: types.KindLREAL}},
		{name: "LIMIT real clamps low", fn: "LIMIT", args: []Value{IntValue(1), RealValue(-2), IntValue(5)}, want: Value{Kind: ValReal, Real: 1, IECType: types.KindLREAL}},
		{name: "LIMIT few args", fn: "LIMIT", args: []Value{IntValue(1), IntValue(2)}, errSub: "LIMIT requires 3 arguments"},
	})
}

func TestStdlibErrCloneNilAggregates(t *testing.T) {
	arr := Value{Kind: ValArray}
	assert.Nil(t, arr.Clone().Array)
	st := Value{Kind: ValStruct}
	assert.Nil(t, st.Clone().Struct)
}

// parseProgram parses src and returns its first PROGRAM declaration.
func parseProgram(t *testing.T, src string) *ast.ProgramDecl {
	t.Helper()
	res := parser.Parse("scan_io_sizes.st", src)
	require.NotNil(t, res.File)
	for _, d := range res.File.Declarations {
		if p, ok := d.(*ast.ProgramDecl); ok {
			return p
		}
	}
	t.Fatal("no PROGRAM in source")
	return nil
}

func TestScanIOSizesByteAndDWord(t *testing.T) {
	prog := parseProgram(t, `
PROGRAM IOSizes
VAR
    inB  AT %IB3 : BYTE;
    inD  AT %ID8 : DINT;
    outB AT %QB1 : BYTE;
    outD AT %QD4 : DINT;
    memD AT %MD12 : DINT;
END_VAR
VAR_OUTPUT
    gotB : BYTE;
    gotD : DINT;
END_VAR
    gotB := inB;
    gotD := inD;
    outB := 16#5A;
    outD := 305419896;
    memD := inD + 1;
END_PROGRAM
`)

	engine := NewScanCycleEngine(prog)
	io := engine.IOTable()
	io.SetByte(iomap.AreaInput, 3, 0xA5)
	io.SetDWord(iomap.AreaInput, 8, 70000)

	require.NoError(t, engine.Tick(10*time.Millisecond))

	gotB := engine.GetOutput("gotB")
	assert.Equal(t, int64(0xA5), gotB.Int, "%IB3 should read the byte image")
	gotD := engine.GetOutput("gotD")
	assert.Equal(t, int64(70000), gotD.Int, "%ID8 should read the dword image")

	assert.Equal(t, byte(0x5A), io.GetByte(iomap.AreaOutput, 1), "%QB1 should be written")
	assert.Equal(t, uint32(305419896), io.GetDWord(iomap.AreaOutput, 4), "%QD4 should be written")
	assert.Equal(t, uint32(70001), io.GetDWord(iomap.AreaMemory, 12), "%MD12 should be written back")

	// Second scan reads memory back in: memD is both read and written.
	require.NoError(t, engine.Tick(10*time.Millisecond))
	assert.Equal(t, uint32(70001), io.GetDWord(iomap.AreaMemory, 12))
}
