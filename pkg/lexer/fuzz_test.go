package lexer

import "testing"

// FuzzTokenize feeds random inputs to the lexer.
// The lexer must never panic — only produce tokens (including Illegal).
func FuzzTokenize(f *testing.F) {
	// Seed with valid ST fragments
	seeds := []string{
		"",
		" ",
		"\t\n\r",
		"PROGRAM main END_PROGRAM",
		"VAR x : INT; END_VAR",
		"IF TRUE THEN x := 1; END_IF;",
		"FOR i := 0 TO 10 BY 2 DO x := x + 1; END_FOR;",
		"WHILE x < 10 DO x := x + 1; END_WHILE;",
		"REPEAT x := x + 1; UNTIL x > 10; END_REPEAT;",
		"CASE x OF 1: y := 1; 2: y := 2; ELSE y := 0; END_CASE;",
		"x := 16#FF;",
		"x := 2#10101010;",
		"x := 8#777;",
		"x := 1_000_000;",
		"x := 3.14;",
		"x := 1.0E10;",
		"'hello world'",
		`"hello world"`,
		"T#1h30m",
		"TIME#500ms",
		"D#2023-01-01",
		"DT#2023-01-01-12:00:00",
		"TOD#12:00:00",
		"// line comment\n",
		"(* block comment *)",
		"(* nested (* comment *) *)",
		"{pragma}",
		"x := y + z * (a - b) / c MOD d;",
		"x := NOT y AND z OR w XOR v;",
		"x.y.z",
		"arr[0]",
		"ptr^",
		"x := y <= z;",
		"x := y >= z;",
		"x := y <> z;",
		"x := y ** z;",
		"func(a := 1, b := 2)",
		":= + - * / ** = <> < <= > >= ( ) [ ] , ; : . .. ^ # => &",
		// Unicode
		"// comment with unicode: \xc3\xa9\xc3\xa0\xc3\xbc",
		"'string with unicode: \xc3\xa9\xc3\xa0\xc3\xbc'",
		// Binary data
		"\x00\x01\x02\xff\xfe\xfd",
		// Very long identifier
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		// All keywords
		"PROGRAM END_PROGRAM FUNCTION_BLOCK END_FUNCTION_BLOCK FUNCTION END_FUNCTION",
		"VAR VAR_INPUT VAR_OUTPUT VAR_IN_OUT VAR_TEMP VAR_GLOBAL END_VAR",
		"IF THEN ELSIF ELSE END_IF CASE OF END_CASE",
		"FOR TO BY DO END_FOR WHILE END_WHILE REPEAT UNTIL END_REPEAT",
		"EXIT CONTINUE RETURN",
		"TRUE FALSE AND OR XOR NOT MOD",
		"ARRAY STRUCT END_STRUCT POINTER REFERENCE STRING WSTRING",
		"BOOL BYTE WORD DWORD LWORD SINT INT DINT LINT USINT UINT UDINT ULINT REAL LREAL",
		"TIME DATE TIME_OF_DAY TOD DATE_AND_TIME DT",
		"EXTENDS IMPLEMENTS THIS SUPER ABSTRACT FINAL OVERRIDE",
		"PUBLIC PRIVATE PROTECTED INTERNAL",
		"CONSTANT RETAIN PERSISTENT AT",
		"TYPE END_TYPE INTERFACE END_INTERFACE METHOD END_METHOD",
		"PROPERTY END_PROPERTY ACTION END_ACTION",
		"TEST_CASE END_TEST_CASE",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic
		tokens := Tokenize("fuzz.st", input)
		// Basic sanity: must end with EOF
		if len(tokens) == 0 {
			t.Fatal("Tokenize returned no tokens")
		}
		if tokens[len(tokens)-1].Kind != EOF {
			t.Fatal("Tokenize did not end with EOF")
		}
	})
}
