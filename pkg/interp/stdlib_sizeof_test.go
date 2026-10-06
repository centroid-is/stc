package interp

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestSizeOfErrors(t *testing.T) {
	assert.Error(t, semRunErr(t, "PROGRAM Main\nVAR n : UDINT; END_VAR\nn := SIZEOF(1, 2);\nEND_PROGRAM\n"))
	assert.Error(t, semRunErr(t, "PROGRAM Main\nVAR n : UDINT; END_VAR\nn := SIZEOF(nope);\nEND_PROGRAM\n"))
}

// ULINT and LWORD values at or above 2^63 convert to positive reals
// (review 2 LO-04).
func TestUnsigned64ToReal(t *testing.T) {
	eng := semRun(t, `
PROGRAM Main
VAR
	u : ULINT := 16#FFFFFFFFFFFFFFFF;
	w : LWORD := 16#8000000000000000;
	a : LREAL; b : REAL; c : LREAL; d : LREAL;
END_VAR
a := ULINT_TO_LREAL(u);
b := LWORD_TO_REAL(w);
c := TO_LREAL(u);
d := u;
END_PROGRAM
`)
	assert.Equal(t, 18446744073709551615.0, progVar(t, eng, "a").Real)
	assert.InDelta(t, 9223372036854775808.0, progVar(t, eng, "b").Real, 1e6)
	assert.Equal(t, 18446744073709551615.0, progVar(t, eng, "c").Real)
	assert.Equal(t, 18446744073709551615.0, progVar(t, eng, "d").Real)
}
