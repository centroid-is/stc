package interp

import (
	"testing"
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
