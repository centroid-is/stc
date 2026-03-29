package parser

import "testing"

// FuzzParse feeds random inputs to the parser.
// The parser must never panic — only return diagnostics.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"",
		" ",
		"\n",
		";",
		";;;",
		// Valid programs
		"PROGRAM main\nVAR x : INT; END_VAR\nx := 1;\nEND_PROGRAM",
		"FUNCTION_BLOCK MyFB\nVAR_INPUT x : INT; END_VAR\nVAR_OUTPUT y : INT; END_VAR\ny := x * 2;\nEND_FUNCTION_BLOCK",
		"FUNCTION Add : INT\nVAR_INPUT a : INT; b : INT; END_VAR\nAdd := a + b;\nEND_FUNCTION",
		// Control flow
		"PROGRAM p\nIF TRUE THEN x := 1; ELSIF FALSE THEN x := 2; ELSE x := 3; END_IF;\nEND_PROGRAM",
		"PROGRAM p\nFOR i := 0 TO 10 BY 2 DO x := x + 1; END_FOR;\nEND_PROGRAM",
		"PROGRAM p\nWHILE x < 10 DO x := x + 1; END_WHILE;\nEND_PROGRAM",
		"PROGRAM p\nREPEAT x := x + 1; UNTIL x > 10; END_REPEAT;\nEND_PROGRAM",
		"PROGRAM p\nCASE x OF 1: y := 1; 2: y := 2; ELSE y := 0; END_CASE;\nEND_PROGRAM",
		// Expressions
		"PROGRAM p\nx := 1 + 2 * 3 - 4 / 5 MOD 6;\nEND_PROGRAM",
		"PROGRAM p\nx := NOT y AND z OR w XOR v;\nEND_PROGRAM",
		"PROGRAM p\nx := a[0];\nEND_PROGRAM",
		"PROGRAM p\nx := obj.field;\nEND_PROGRAM",
		"PROGRAM p\nx := func(1, 2, 3);\nEND_PROGRAM",
		// Type declarations
		"TYPE MyEnum : (Val1, Val2, Val3); END_TYPE",
		"TYPE MyStruct : STRUCT x : INT; y : REAL; END_STRUCT; END_TYPE",
		"TYPE MyArray : ARRAY[0..9] OF INT; END_TYPE",
		// Comments
		"// just a comment",
		"(* just a block comment *)",
		"(* nested (* comment *) end *)",
		// Edge cases
		"PROGRAM END_PROGRAM",
		"FUNCTION_BLOCK END_FUNCTION_BLOCK",
		"FUNCTION f : INT END_FUNCTION",
		// Mismatched END_ keywords
		"PROGRAM p\nIF TRUE THEN\nEND_PROGRAM",
		"END_IF",
		"END_PROGRAM",
		"END_FUNCTION_BLOCK",
		// Garbage
		"@#$%^&*!~`",
		"123abc",
		"\x00\x00\x00",
		"\xff\xfe\xfd",
		// Deeply nested expression
		"PROGRAM p\nx := ((((((((((1))))))))));\nEND_PROGRAM",
		// Unicode
		"\xef\xbb\xbfPROGRAM p END_PROGRAM", // BOM
		// Tab/CR/LF mixing
		"PROGRAM\tp\r\nVAR\r\tx : INT;\r\nEND_VAR\r\nEND_PROGRAM",
		// Test cases
		"TEST_CASE 'test1'\nASSERT_EQ(1, 1);\nEND_TEST_CASE",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic. Errors in Diags are fine.
		result := Parse("fuzz.st", input)
		if result.File == nil {
			t.Fatal("Parse returned nil File")
		}
	})
}
