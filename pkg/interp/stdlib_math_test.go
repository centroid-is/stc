package interp

import (
	"math"
	"testing"
)

func TestABS(t *testing.T) {
	fn := StdlibFunctions["ABS"]
	if fn == nil {
		t.Fatal("ABS not registered")
	}

	tests := []struct {
		name string
		arg  Value
		want Value
	}{
		{"negative int", IntValue(-5), IntValue(5)},
		{"positive int", IntValue(5), IntValue(5)},
		{"zero int", IntValue(0), IntValue(0)},
		{"negative real", RealValue(-3.14), RealValue(3.14)},
		{"positive real", RealValue(3.14), RealValue(3.14)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fn([]Value{tt.arg})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertValueEqual(t, tt.want, got)
		})
	}
}

func TestSQRT(t *testing.T) {
	fn := StdlibFunctions["SQRT"]
	if fn == nil {
		t.Fatal("SQRT not registered")
	}
	got, err := fn([]Value{RealValue(4.0)})
	if err != nil {
		t.Fatal(err)
	}
	assertRealClose(t, 2.0, got.Real)

	got, err = fn([]Value{RealValue(0.0)})
	if err != nil {
		t.Fatal(err)
	}
	assertRealClose(t, 0.0, got.Real)
}

func TestTrigFunctions(t *testing.T) {
	tests := []struct {
		name string
		fn   string
		arg  float64
		want float64
	}{
		{"SIN(0)", "SIN", 0.0, 0.0},
		{"COS(0)", "COS", 0.0, 1.0},
		{"TAN(0)", "TAN", 0.0, 0.0},
		{"ASIN(1)", "ASIN", 1.0, math.Pi / 2},
		{"ACOS(1)", "ACOS", 1.0, 0.0},
		{"ATAN(1)", "ATAN", 1.0, math.Pi / 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := StdlibFunctions[tt.fn]
			if fn == nil {
				t.Fatalf("%s not registered", tt.fn)
			}
			got, err := fn([]Value{RealValue(tt.arg)})
			if err != nil {
				t.Fatal(err)
			}
			assertRealClose(t, tt.want, got.Real)
		})
	}
}

func TestLnLogExp(t *testing.T) {
	tests := []struct {
		name string
		fn   string
		arg  float64
		want float64
	}{
		{"LN(e)", "LN", math.E, 1.0},
		{"LOG(100)", "LOG", 100.0, 2.0},
		{"EXP(1)", "EXP", 1.0, math.E},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := StdlibFunctions[tt.fn]
			if fn == nil {
				t.Fatalf("%s not registered", tt.fn)
			}
			got, err := fn([]Value{RealValue(tt.arg)})
			if err != nil {
				t.Fatal(err)
			}
			assertRealClose(t, tt.want, got.Real)
		})
	}
}

func TestEXPT(t *testing.T) {
	fn := StdlibFunctions["EXPT"]
	if fn == nil {
		t.Fatal("EXPT not registered")
	}
	got, err := fn([]Value{RealValue(2.0), RealValue(3.0)})
	if err != nil {
		t.Fatal(err)
	}
	assertRealClose(t, 8.0, got.Real)
}

func TestMIN(t *testing.T) {
	fn := StdlibFunctions["MIN"]
	if fn == nil {
		t.Fatal("MIN not registered")
	}

	// Integer
	got, err := fn([]Value{IntValue(3), IntValue(7)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValInt || got.Int != 3 {
		t.Fatalf("MIN(3,7) = %v, want 3", got)
	}

	// Real
	got, err = fn([]Value{RealValue(3.0), RealValue(7.0)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValReal {
		t.Fatalf("MIN(3.0,7.0) kind = %v, want Real", got.Kind)
	}
	assertRealClose(t, 3.0, got.Real)
}

func TestMAX(t *testing.T) {
	fn := StdlibFunctions["MAX"]
	if fn == nil {
		t.Fatal("MAX not registered")
	}
	got, err := fn([]Value{IntValue(3), IntValue(7)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValInt || got.Int != 7 {
		t.Fatalf("MAX(3,7) = %v, want 7", got)
	}
}

func TestLIMIT(t *testing.T) {
	fn := StdlibFunctions["LIMIT"]
	if fn == nil {
		t.Fatal("LIMIT not registered")
	}

	tests := []struct {
		name string
		mn   Value
		in   Value
		mx   Value
		want int64
	}{
		{"in range", IntValue(0), IntValue(5), IntValue(10), 5},
		{"below min", IntValue(0), IntValue(-1), IntValue(10), 0},
		{"above max", IntValue(0), IntValue(15), IntValue(10), 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fn([]Value{tt.mn, tt.in, tt.mx})
			if err != nil {
				t.Fatal(err)
			}
			if got.Int != tt.want {
				t.Fatalf("LIMIT(%d,%d,%d) = %d, want %d", tt.mn.Int, tt.in.Int, tt.mx.Int, got.Int, tt.want)
			}
		})
	}
}

func TestSEL(t *testing.T) {
	fn := StdlibFunctions["SEL"]
	if fn == nil {
		t.Fatal("SEL not registered")
	}

	got, err := fn([]Value{BoolValue(false), IntValue(10), IntValue(20)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Int != 10 {
		t.Fatalf("SEL(FALSE,10,20) = %v, want 10", got)
	}

	got, err = fn([]Value{BoolValue(true), IntValue(10), IntValue(20)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Int != 20 {
		t.Fatalf("SEL(TRUE,10,20) = %v, want 20", got)
	}
}

func TestMUX(t *testing.T) {
	fn := StdlibFunctions["MUX"]
	if fn == nil {
		t.Fatal("MUX not registered")
	}

	got, err := fn([]Value{IntValue(0), IntValue(10), IntValue(20), IntValue(30)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Int != 10 {
		t.Fatalf("MUX(0,10,20,30) = %v, want 10", got)
	}

	got, err = fn([]Value{IntValue(2), IntValue(10), IntValue(20), IntValue(30)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Int != 30 {
		t.Fatalf("MUX(2,10,20,30) = %v, want 30", got)
	}
}

func TestMOVE(t *testing.T) {
	fn := StdlibFunctions["MOVE"]
	if fn == nil {
		t.Fatal("MOVE not registered")
	}
	got, err := fn([]Value{IntValue(42)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Int != 42 {
		t.Fatalf("MOVE(42) = %v, want 42", got)
	}
}

// --- Helpers ---

func assertValueEqual(t *testing.T, want, got Value) {
	t.Helper()
	if want.Kind != got.Kind {
		t.Fatalf("kind mismatch: want %v, got %v", want.Kind, got.Kind)
	}
	switch want.Kind {
	case ValInt:
		if want.Int != got.Int {
			t.Fatalf("int mismatch: want %d, got %d", want.Int, got.Int)
		}
	case ValReal:
		assertRealClose(t, want.Real, got.Real)
	case ValBool:
		if want.Bool != got.Bool {
			t.Fatalf("bool mismatch: want %v, got %v", want.Bool, got.Bool)
		}
	case ValString:
		if want.Str != got.Str {
			t.Fatalf("string mismatch: want %q, got %q", want.Str, got.Str)
		}
	}
}

func assertRealClose(t *testing.T, want, got float64) {
	t.Helper()
	if math.Abs(want-got) > 1e-9 {
		t.Fatalf("real mismatch: want %g, got %g", want, got)
	}
}
