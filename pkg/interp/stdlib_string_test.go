package interp

import (
	"testing"
)

func TestLEN(t *testing.T) {
	fn := StdlibFunctions["LEN"]
	if fn == nil {
		t.Fatal("LEN not registered")
	}
	tests := []struct {
		in   string
		want int64
	}{
		{"hello", 5},
		{"", 0},
		{"a", 1},
	}
	for _, tt := range tests {
		got, err := fn([]Value{StringValue(tt.in)})
		if err != nil {
			t.Fatal(err)
		}
		if got.Kind != ValInt || got.Int != tt.want {
			t.Fatalf("LEN(%q) = %v, want %d", tt.in, got, tt.want)
		}
	}
}

func TestLEFT(t *testing.T) {
	fn := StdlibFunctions["LEFT"]
	if fn == nil {
		t.Fatal("LEFT not registered")
	}
	tests := []struct {
		in   string
		l    int64
		want string
	}{
		{"hello", 3, "hel"},
		{"hi", 5, "hi"},
		{"hello", 0, ""},
	}
	for _, tt := range tests {
		got, err := fn([]Value{StringValue(tt.in), IntValue(tt.l)})
		if err != nil {
			t.Fatal(err)
		}
		if got.Str != tt.want {
			t.Fatalf("LEFT(%q, %d) = %q, want %q", tt.in, tt.l, got.Str, tt.want)
		}
	}
}

func TestRIGHT(t *testing.T) {
	fn := StdlibFunctions["RIGHT"]
	if fn == nil {
		t.Fatal("RIGHT not registered")
	}
	tests := []struct {
		in   string
		l    int64
		want string
	}{
		{"hello", 3, "llo"},
		{"hi", 5, "hi"},
		{"hello", 0, ""},
	}
	for _, tt := range tests {
		got, err := fn([]Value{StringValue(tt.in), IntValue(tt.l)})
		if err != nil {
			t.Fatal(err)
		}
		if got.Str != tt.want {
			t.Fatalf("RIGHT(%q, %d) = %q, want %q", tt.in, tt.l, got.Str, tt.want)
		}
	}
}

func TestMID(t *testing.T) {
	fn := StdlibFunctions["MID"]
	if fn == nil {
		t.Fatal("MID not registered")
	}
	tests := []struct {
		name string
		in   string
		l    int64
		p    int64
		want string
	}{
		{"3 chars at pos 2", "hello", 3, 2, "ell"},
		{"2 chars at pos 1", "hello", 2, 1, "he"},
		{"pos beyond length", "hello", 2, 10, ""},
		{"zero length", "hello", 0, 1, ""},
		{"pos < 1", "hello", 2, 0, ""},
		{"truncate at end", "hello", 10, 3, "llo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fn([]Value{StringValue(tt.in), IntValue(tt.l), IntValue(tt.p)})
			if err != nil {
				t.Fatal(err)
			}
			if got.Str != tt.want {
				t.Fatalf("MID(%q, %d, %d) = %q, want %q", tt.in, tt.l, tt.p, got.Str, tt.want)
			}
		})
	}
}

func TestCONCAT(t *testing.T) {
	fn := StdlibFunctions["CONCAT"]
	if fn == nil {
		t.Fatal("CONCAT not registered")
	}
	got, err := fn([]Value{StringValue("hello"), StringValue(" world")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Str != "hello world" {
		t.Fatalf("CONCAT = %q, want \"hello world\"", got.Str)
	}
}

func TestFIND(t *testing.T) {
	fn := StdlibFunctions["FIND"]
	if fn == nil {
		t.Fatal("FIND not registered")
	}
	// Found case: 1-based position
	got, err := fn([]Value{StringValue("hello world"), StringValue("world")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Int != 7 {
		t.Fatalf("FIND('hello world', 'world') = %d, want 7", got.Int)
	}

	// Not found: returns 0
	got, err = fn([]Value{StringValue("hello"), StringValue("xyz")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Int != 0 {
		t.Fatalf("FIND('hello', 'xyz') = %d, want 0", got.Int)
	}
}

func TestINSERT(t *testing.T) {
	fn := StdlibFunctions["INSERT"]
	if fn == nil {
		t.Fatal("INSERT not registered")
	}
	// INSERT("hello", " world", 6) -> "hello world"
	got, err := fn([]Value{StringValue("hello"), StringValue(" world"), IntValue(6)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Str != "hello world" {
		t.Fatalf("INSERT = %q, want \"hello world\"", got.Str)
	}

	// Edge: pos < 1
	got, err = fn([]Value{StringValue("hello"), StringValue("X"), IntValue(0)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Str != "hello" {
		t.Fatalf("INSERT with pos<1 = %q, want \"hello\"", got.Str)
	}
}

func TestDELETE(t *testing.T) {
	fn := StdlibFunctions["DELETE"]
	if fn == nil {
		t.Fatal("DELETE not registered")
	}
	// DELETE("hello world", 6, 6) -> "hello"
	got, err := fn([]Value{StringValue("hello world"), IntValue(6), IntValue(6)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Str != "hello" {
		t.Fatalf("DELETE = %q, want \"hello\"", got.Str)
	}
}

func TestREPLACE(t *testing.T) {
	fn := StdlibFunctions["REPLACE"]
	if fn == nil {
		t.Fatal("REPLACE not registered")
	}
	// REPLACE("hello world", "earth", 5, 7) -> "hello earth"
	// Replace 5 chars at position 7 with "earth"
	got, err := fn([]Value{StringValue("hello world"), StringValue("earth"), IntValue(5), IntValue(7)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Str != "hello earth" {
		t.Fatalf("REPLACE = %q, want \"hello earth\"", got.Str)
	}
}
