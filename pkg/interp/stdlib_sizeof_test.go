package interp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSizeOf(t *testing.T) {
	eng := semRun(t, `
TYPE ST_S : STRUCT a : BYTE; b : UDINT; c : ARRAY[0..1] OF INT; END_STRUCT END_TYPE
TYPE E_X : (A, B) UINT; END_TYPE
{attribute 'qualified_only'}
VAR_GLOBAL CONSTANT
	N : UINT := 4;
END_VAR
PROGRAM Main
VAR
	x : BOOL; b : BYTE; w : WORD; d : DINT; r : REAL; lr : LREAL; li : LINT; tm : TIME;
	s : STRING; e : E_X; st : ST_S;
	arr : ARRAY[0..G.N - 1] OF UDINT;
	arr1 : ARRAY[1..3] OF ST_S;
	sz : ARRAY[0..12] OF UDINT;
END_VAR
sz[0] := SIZEOF(x);
sz[1] := SIZEOF(b);
sz[2] := SIZEOF(w);
sz[3] := SIZEOF(d);
sz[4] := SIZEOF(r);
sz[5] := SIZEOF(lr);
sz[6] := SIZEOF(li);
sz[7] := SIZEOF(tm);
sz[8] := SIZEOF(s);
sz[9] := SIZEOF(e);
sz[10] := SIZEOF(st);
sz[11] := SIZEOF(arr);
sz[12] := SIZEOF(arr1);
END_PROGRAM
`)
	want := []int64{1, 1, 2, 4, 4, 8, 8, 4, 81, 2, 9, 16, 27}
	got := progVar(t, eng, "sz")
	for i, w := range want {
		assert.Equal(t, w, got.Array[i].Int, "sz[%d]", i)
	}
}

func TestSizeOfLongStringAndErrors(t *testing.T) {
	n, err := sizeOfValue(Value{Kind: ValString, Str: string(make([]byte, 100))})
	require.NoError(t, err)
	assert.Equal(t, 101, n)

	_, err = sizeOfValue(Value{Kind: ValFBInstance})
	require.Error(t, err)
	_, err = sizeOfValue(Value{Kind: ValArray, Array: []Value{{Kind: ValFBInstance}}})
	require.Error(t, err)
	_, err = sizeOfValue(Value{Kind: ValStruct, Struct: map[string]Value{"A": {Kind: ValFBInstance}}})
	require.Error(t, err)

	assert.Error(t, semRunErr(t, "PROGRAM Main\nVAR n : UDINT; END_VAR\nn := SIZEOF(1, 2);\nEND_PROGRAM\n"))
	assert.Error(t, semRunErr(t, "PROGRAM Main\nVAR n : UDINT; END_VAR\nn := SIZEOF(nope);\nEND_PROGRAM\n"))
	assert.Error(t, semRunErr(t, "FUNCTION_BLOCK FB\nEND_FUNCTION_BLOCK\nPROGRAM Main\nVAR n : UDINT; f : FB; END_VAR\nn := SIZEOF(f);\nEND_PROGRAM\n"))
}
