package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/types"
)

func TestINT_TO_REAL(t *testing.T) {
	fn := StdlibFunctions["INT_TO_REAL"]
	if fn == nil {
		t.Fatal("INT_TO_REAL not registered")
	}
	got, err := fn([]Value{IntValue(42)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValReal || got.Real != 42.0 {
		t.Fatalf("INT_TO_REAL(42) = %v, want 42.0", got)
	}
}

func TestREAL_TO_INT_BankersRounding(t *testing.T) {
	fn := StdlibFunctions["REAL_TO_INT"]
	if fn == nil {
		t.Fatal("REAL_TO_INT not registered")
	}

	tests := []struct {
		name string
		in   float64
		want int64
	}{
		{"2.5 -> 2 (half to even)", 2.5, 2},
		{"3.5 -> 4 (half to even)", 3.5, 4},
		{"4.5 -> 4 (half to even)", 4.5, 4},
		{"5.5 -> 6 (half to even)", 5.5, 6},
		{"2.4 -> 2", 2.4, 2},
		{"2.6 -> 3", 2.6, 3},
		{"-1.5 -> -2 (half to even)", -1.5, -2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fn([]Value{RealValue(tt.in)})
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != ValInt || got.Int != tt.want {
				t.Fatalf("REAL_TO_INT(%g) = %v, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestBOOL_TO_INT(t *testing.T) {
	fn := StdlibFunctions["BOOL_TO_INT"]
	if fn == nil {
		t.Fatal("BOOL_TO_INT not registered")
	}

	got, err := fn([]Value{BoolValue(true)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Int != 1 {
		t.Fatalf("BOOL_TO_INT(TRUE) = %v, want 1", got)
	}

	got, err = fn([]Value{BoolValue(false)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Int != 0 {
		t.Fatalf("BOOL_TO_INT(FALSE) = %v, want 0", got)
	}
}

func TestINT_TO_BOOL(t *testing.T) {
	fn := StdlibFunctions["INT_TO_BOOL"]
	if fn == nil {
		t.Fatal("INT_TO_BOOL not registered")
	}

	got, err := fn([]Value{IntValue(0)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Bool != false {
		t.Fatalf("INT_TO_BOOL(0) = %v, want FALSE", got)
	}

	got, err = fn([]Value{IntValue(5)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Bool != true {
		t.Fatalf("INT_TO_BOOL(5) = %v, want TRUE", got)
	}
}

func TestINT_TO_STRING(t *testing.T) {
	fn := StdlibFunctions["INT_TO_STRING"]
	if fn == nil {
		t.Fatal("INT_TO_STRING not registered")
	}
	got, err := fn([]Value{IntValue(42)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Str != "42" {
		t.Fatalf("INT_TO_STRING(42) = %q, want \"42\"", got.Str)
	}
}

func TestSTRING_TO_INT(t *testing.T) {
	fn := StdlibFunctions["STRING_TO_INT"]
	if fn == nil {
		t.Fatal("STRING_TO_INT not registered")
	}
	got, err := fn([]Value{StringValue("42")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Int != 42 {
		t.Fatalf("STRING_TO_INT(\"42\") = %v, want 42", got)
	}

	// Invalid string should error
	_, err = fn([]Value{StringValue("abc")})
	if err == nil {
		t.Fatal("STRING_TO_INT(\"abc\") should error")
	}
}

func TestREAL_TO_STRING(t *testing.T) {
	fn := StdlibFunctions["REAL_TO_STRING"]
	if fn == nil {
		t.Fatal("REAL_TO_STRING not registered")
	}
	got, err := fn([]Value{RealValue(3.14)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Str != "3.14" {
		t.Fatalf("REAL_TO_STRING(3.14) = %q, want \"3.14\"", got.Str)
	}
}

func TestDINT_TO_LREAL(t *testing.T) {
	fn := StdlibFunctions["DINT_TO_LREAL"]
	if fn == nil {
		t.Fatal("DINT_TO_LREAL not registered")
	}
	got, err := fn([]Value{IntValue(100000)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValReal || got.Real != 100000.0 {
		t.Fatalf("DINT_TO_LREAL(100000) = %v, want 100000.0", got)
	}
}

func TestBYTE_TO_INT(t *testing.T) {
	fn := StdlibFunctions["BYTE_TO_INT"]
	if fn == nil {
		t.Fatal("BYTE_TO_INT not registered")
	}
	got, err := fn([]Value{IntValue(255)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Int != 255 {
		t.Fatalf("BYTE_TO_INT(255) = %v, want 255", got)
	}
}

func TestBOOL_TO_STRING(t *testing.T) {
	fn := StdlibFunctions["BOOL_TO_STRING"]
	if fn == nil {
		t.Fatal("BOOL_TO_STRING not registered")
	}

	got, err := fn([]Value{BoolValue(true)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Str != "TRUE" {
		t.Fatalf("BOOL_TO_STRING(TRUE) = %q, want \"TRUE\"", got.Str)
	}

	got, err = fn([]Value{BoolValue(false)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Str != "FALSE" {
		t.Fatalf("BOOL_TO_STRING(FALSE) = %q, want \"FALSE\"", got.Str)
	}
}

func TestSTRING_TO_REAL(t *testing.T) {
	fn := StdlibFunctions["STRING_TO_REAL"]
	if fn == nil {
		t.Fatal("STRING_TO_REAL not registered")
	}
	got, err := fn([]Value{StringValue("3.14")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValReal || got.Real != 3.14 {
		t.Fatalf("STRING_TO_REAL(\"3.14\") = %v, want 3.14", got)
	}
}

func TestINT_TO_BYTE(t *testing.T) {
	fn := StdlibFunctions["INT_TO_BYTE"]
	if fn == nil {
		t.Fatal("INT_TO_BYTE not registered")
	}
	got, err := fn([]Value{IntValue(256)})
	if err != nil {
		t.Fatal(err)
	}
	// 256 & 0xFF = 0
	if got.Int != 0 {
		t.Fatalf("INT_TO_BYTE(256) = %v, want 0 (masked)", got)
	}
}

func TestTRUNC(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want int64
	}{
		{"2.9 -> 2", 2.9, 2},
		{"2.5 -> 2", 2.5, 2},
		{"-2.5 -> -2 (toward zero)", -2.5, -2},
		{"-2.9 -> -2 (toward zero)", -2.9, -2},
		{"-0.5 -> 0", -0.5, 0},
		{"7.0 -> 7", 7.0, 7},
	}
	for _, name := range []string{"TRUNC", "TRUNC_INT", "TRUNC_DINT"} {
		fn := StdlibFunctions[name]
		if fn == nil {
			t.Fatalf("%s not registered", name)
		}
		for _, tt := range tests {
			t.Run(name+"/"+tt.name, func(t *testing.T) {
				got, err := fn([]Value{RealValue(tt.in)})
				if err != nil {
					t.Fatal(err)
				}
				if got.Kind != ValInt || got.Int != tt.want {
					t.Fatalf("%s(%v) = %v, want %d", name, tt.in, got, tt.want)
				}
			})
		}
	}
}

func TestTRUNC_ReturnKinds(t *testing.T) {
	got, err := StdlibFunctions["TRUNC_INT"]([]Value{RealValue(1.7)})
	if err != nil {
		t.Fatal(err)
	}
	if got.IECType != types.KindINT {
		t.Fatalf("TRUNC_INT IECType = %v, want INT", got.IECType)
	}
	got, err = StdlibFunctions["TRUNC"]([]Value{RealValue(1.7)})
	if err != nil {
		t.Fatal(err)
	}
	if got.IECType != types.KindDINT {
		t.Fatalf("TRUNC IECType = %v, want DINT", got.IECType)
	}
}

func TestTIME_TO_DINT(t *testing.T) {
	fn := StdlibFunctions["TIME_TO_DINT"]
	if fn == nil {
		t.Fatal("TIME_TO_DINT not registered")
	}
	tests := []struct {
		in   time.Duration
		want int64
	}{
		{1500 * time.Millisecond, 1500},
		{2 * time.Second, 2000},
		{0, 0},
		{-250 * time.Millisecond, -250},
		{1500 * time.Microsecond, 1}, // sub-ms truncated
	}
	for _, tt := range tests {
		got, err := fn([]Value{TimeValue(tt.in)})
		if err != nil {
			t.Fatal(err)
		}
		if got.Kind != ValInt || got.Int != tt.want || got.IECType != types.KindDINT {
			t.Fatalf("TIME_TO_DINT(%v) = %v, want %d", tt.in, got, tt.want)
		}
	}
}

func TestTIME_TO_LREAL(t *testing.T) {
	fn := StdlibFunctions["TIME_TO_LREAL"]
	if fn == nil {
		t.Fatal("TIME_TO_LREAL not registered")
	}
	tests := []struct {
		in   time.Duration
		want float64
	}{
		{1500 * time.Millisecond, 1500},
		{2 * time.Second, 2000},
		{0, 0},
		{1500 * time.Microsecond, 1.5}, // sub-ms precision retained
		{-250 * time.Millisecond, -250},
	}
	for _, tt := range tests {
		got, err := fn([]Value{TimeValue(tt.in)})
		if err != nil {
			t.Fatal(err)
		}
		if got.Kind != ValReal || got.Real != tt.want || got.IECType != types.KindLREAL {
			t.Fatalf("TIME_TO_LREAL(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestTO_TypeConversions(t *testing.T) {
	cases := []struct {
		name string
		in   Value
		want Value
	}{
		{"TO_INT", IntValue(7), Value{Kind: ValInt, Int: 7, IECType: types.KindINT}},
		{"TO_UINT", BoolValue(true), Value{Kind: ValInt, Int: 1, IECType: types.KindUINT}},
		{"TO_DINT", BoolValue(false), Value{Kind: ValInt, Int: 0, IECType: types.KindDINT}},
		{"TO_DINT", RealValue(2.5), Value{Kind: ValInt, Int: 2, IECType: types.KindDINT}},
		{"TO_WORD", TimeValue(1500 * time.Millisecond), Value{Kind: ValInt, Int: 1500, IECType: types.KindWORD}},
		{"TO_LINT", StringValue(" 42 "), Value{Kind: ValInt, Int: 42, IECType: types.KindLINT}},
		{"TO_REAL", IntValue(3), Value{Kind: ValReal, Real: 3, IECType: types.KindREAL}},
		{"TO_LREAL", TimeValue(250 * time.Millisecond), Value{Kind: ValReal, Real: 250, IECType: types.KindLREAL}},
		{"TO_LREAL", StringValue("1.5"), Value{Kind: ValReal, Real: 1.5, IECType: types.KindLREAL}},
		{"TO_INT", Value{Kind: ValInt, Int: 2, IECType: types.KindUINT, Enum: "E_S"}, Value{Kind: ValInt, Int: 2, IECType: types.KindINT}},
	}
	for _, c := range cases {
		got, err := StdlibFunctions[c.name]([]Value{c.in})
		if err != nil {
			t.Fatalf("%s(%v): %v", c.name, c.in, err)
		}
		if got.Kind != c.want.Kind || got.Int != c.want.Int || got.Real != c.want.Real || got.IECType != c.want.IECType || got.Enum != "" {
			t.Errorf("%s(%v) = %#v, want %#v", c.name, c.in, got, c.want)
		}
	}

	for _, bad := range []struct {
		name string
		args []Value
	}{
		{"TO_INT", nil},
		{"TO_INT", []Value{StringValue("x")}},
		{"TO_REAL", []Value{StringValue("x")}},
		{"TO_INT", []Value{{Kind: ValArray}}},
		{"TO_REAL", []Value{{Kind: ValArray}}},
	} {
		if _, err := StdlibFunctions[bad.name](bad.args); err == nil {
			t.Errorf("%s(%v): expected an error", bad.name, bad.args)
		}
	}
}
