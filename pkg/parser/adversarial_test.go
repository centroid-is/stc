package parser

import (
	"strings"
	"testing"
)

// TestAdversarial_EmptyInput tests parsing an empty file.
func TestAdversarial_EmptyInput(t *testing.T) {
	result := Parse("empty.st", "")
	if result.File == nil {
		t.Fatal("Parse returned nil File for empty input")
	}
}

// TestAdversarial_SingleCharacters tests parsing every printable ASCII character.
func TestAdversarial_SingleCharacters(t *testing.T) {
	for c := byte(0); c < 128; c++ {
		input := string([]byte{c})
		result := Parse("char.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for byte 0x%02x", c)
		}
	}
}

// TestAdversarial_OnlyComments tests files with only comments.
func TestAdversarial_OnlyComments(t *testing.T) {
	inputs := []string{
		"// just a line comment",
		"// line 1\n// line 2\n// line 3",
		"(* block comment *)",
		"(* multi\nline\nblock\ncomment *)",
		"(* nested (* inner *) outer *)",
		"// mix\n(* block *)\n// more",
	}
	for _, input := range inputs {
		result := Parse("comments.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for comment-only input: %q", input)
		}
	}
}

// TestAdversarial_OnlyPragmas tests files with only pragmas.
func TestAdversarial_OnlyPragmas(t *testing.T) {
	inputs := []string{
		"{pragma}",
		"{attr 'some attribute'}",
		"{warning disable}",
	}
	for _, input := range inputs {
		result := Parse("pragma.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for pragma input: %q", input)
		}
	}
}

// TestAdversarial_DeeplyNestedIf tests deeply nested IF statements.
func TestAdversarial_DeeplyNestedIf(t *testing.T) {
	// Build 500 nested IFs (not 10000 to avoid extreme slowness)
	var b strings.Builder
	b.WriteString("PROGRAM p\nVAR x : INT; END_VAR\n")
	depth := 500
	for i := 0; i < depth; i++ {
		b.WriteString("IF TRUE THEN\n")
	}
	b.WriteString("x := 1;\n")
	for i := 0; i < depth; i++ {
		b.WriteString("END_IF;\n")
	}
	b.WriteString("END_PROGRAM\n")

	result := Parse("deep_if.st", b.String())
	if result.File == nil {
		t.Fatal("Parse returned nil File for deeply nested IF")
	}
}

// TestAdversarial_LongIdentifier tests a very long identifier.
func TestAdversarial_LongIdentifier(t *testing.T) {
	longName := strings.Repeat("x", 10000)
	input := "PROGRAM p\nVAR " + longName + " : INT; END_VAR\nEND_PROGRAM"
	result := Parse("long_ident.st", input)
	if result.File == nil {
		t.Fatal("Parse returned nil File for long identifier")
	}
}

// TestAdversarial_LongString tests a very long string literal.
func TestAdversarial_LongString(t *testing.T) {
	longStr := "'" + strings.Repeat("a", 10000) + "'"
	input := "PROGRAM p\nVAR s : STRING; END_VAR\ns := " + longStr + ";\nEND_PROGRAM"
	result := Parse("long_string.st", input)
	if result.File == nil {
		t.Fatal("Parse returned nil File for long string")
	}
}

// TestAdversarial_BinaryData tests input with null bytes and binary data.
func TestAdversarial_BinaryData(t *testing.T) {
	inputs := []string{
		"\x00",
		"\x00\x00\x00",
		"\xff\xfe\xfd",
		"PROGRAM\x00p END_PROGRAM",
		"PROGRAM p\x00\x01\x02 END_PROGRAM",
		string([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}),
		"\xff\xff\xff\xff\xff",
	}
	for _, input := range inputs {
		result := Parse("binary.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for binary input")
		}
	}
}

// TestAdversarial_Unicode tests various unicode inputs.
func TestAdversarial_Unicode(t *testing.T) {
	inputs := []string{
		// BOM
		"\xef\xbb\xbfPROGRAM p END_PROGRAM",
		// Unicode identifiers
		"\xc3\xa9\xc3\xa0\xc3\xbc",
		// Emoji
		"\xf0\x9f\x98\x80",
		// CJK
		"\xe4\xb8\xad\xe6\x96\x87",
		// Mixed
		"PROGRAM p\nVAR x : INT; END_VAR\n// \xf0\x9f\x98\x80 emoji comment\nEND_PROGRAM",
		// Unicode in strings
		"PROGRAM p\nVAR s : STRING; END_VAR\ns := '\xc3\xa9\xc3\xa0\xc3\xbc';\nEND_PROGRAM",
		// Zero-width space
		"PROGRAM\xe2\x80\x8bp END_PROGRAM",
		// Right-to-left override
		"PROGRAM \xe2\x80\xaep END_PROGRAM",
	}
	for _, input := range inputs {
		result := Parse("unicode.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for unicode input")
		}
	}
}

// TestAdversarial_MismatchedEndKeywords tests mismatched END_ keywords.
func TestAdversarial_MismatchedEndKeywords(t *testing.T) {
	inputs := []string{
		"END_IF",
		"END_PROGRAM",
		"END_FUNCTION_BLOCK",
		"END_FUNCTION",
		"END_VAR",
		"END_FOR",
		"END_WHILE",
		"END_REPEAT",
		"END_CASE",
		"END_TYPE",
		"END_STRUCT",
		"END_INTERFACE",
		"END_METHOD",
		"END_PROPERTY",
		"END_ACTION",
		"END_TEST_CASE",
		// IF without END_IF
		"PROGRAM p\nIF TRUE THEN\nEND_PROGRAM",
		// FOR without END_FOR
		"PROGRAM p\nFOR i := 0 TO 10 DO\nEND_PROGRAM",
		// WHILE without END_WHILE
		"PROGRAM p\nWHILE TRUE DO\nEND_PROGRAM",
		// Wrong END_ keyword
		"PROGRAM p\nIF TRUE THEN END_FOR;\nEND_PROGRAM",
		"PROGRAM p\nFOR i := 0 TO 10 DO END_IF;\nEND_PROGRAM",
		"PROGRAM p\nWHILE TRUE DO END_REPEAT;\nEND_PROGRAM",
		// Double END_
		"PROGRAM p\nIF TRUE THEN END_IF; END_IF;\nEND_PROGRAM",
	}
	for _, input := range inputs {
		result := Parse("mismatch.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for mismatched END: %q", input)
		}
	}
}

// TestAdversarial_KeywordsAsIdentifiers tests using every keyword as an identifier.
func TestAdversarial_KeywordsAsIdentifiers(t *testing.T) {
	keywords := []string{
		"PROGRAM", "END_PROGRAM", "FUNCTION_BLOCK", "END_FUNCTION_BLOCK",
		"FUNCTION", "END_FUNCTION", "VAR", "VAR_INPUT", "VAR_OUTPUT",
		"VAR_IN_OUT", "VAR_TEMP", "VAR_GLOBAL", "END_VAR",
		"IF", "THEN", "ELSIF", "ELSE", "END_IF",
		"CASE", "OF", "END_CASE", "FOR", "TO", "BY", "DO", "END_FOR",
		"WHILE", "END_WHILE", "REPEAT", "UNTIL", "END_REPEAT",
		"EXIT", "CONTINUE", "RETURN",
		"TRUE", "FALSE", "AND", "OR", "XOR", "NOT", "MOD",
		"ARRAY", "STRUCT", "END_STRUCT", "POINTER", "REFERENCE",
		"STRING", "WSTRING",
		"BOOL", "INT", "DINT", "LINT", "SINT", "REAL", "LREAL",
		"BYTE", "WORD", "DWORD", "LWORD",
		"USINT", "UINT", "UDINT", "ULINT",
		"TIME", "DATE", "TIME_OF_DAY", "TOD", "DATE_AND_TIME", "DT",
		"EXTENDS", "IMPLEMENTS", "THIS", "SUPER",
		"ABSTRACT", "FINAL", "OVERRIDE",
		"PUBLIC", "PRIVATE", "PROTECTED", "INTERNAL",
		"CONSTANT", "RETAIN", "PERSISTENT", "AT",
		"TYPE", "END_TYPE", "INTERFACE", "END_INTERFACE",
		"METHOD", "END_METHOD", "PROPERTY", "END_PROPERTY",
		"ACTION", "END_ACTION",
		"TEST_CASE", "END_TEST_CASE",
	}
	for _, kw := range keywords {
		// Try using keyword as variable name
		input := "PROGRAM p\nVAR " + kw + " : INT; END_VAR\nEND_PROGRAM"
		result := Parse("kw_as_ident.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File when using keyword %q as identifier", kw)
		}
	}
}

// TestAdversarial_SemicolonOnlyFile tests a file with only semicolons.
func TestAdversarial_SemicolonOnlyFile(t *testing.T) {
	result := Parse("semicolons.st", ";;;;;;;;;;;;;;;;;;;;")
	if result.File == nil {
		t.Fatal("Parse returned nil File for semicolon-only input")
	}
}

// TestAdversarial_WhitespaceVariations tests various whitespace combinations.
func TestAdversarial_WhitespaceVariations(t *testing.T) {
	inputs := []string{
		"\t\t\t",
		"\r\n\r\n\r\n",
		"\r\r\r",
		"\n\n\n",
		"\r\n",
		" \t \r \n ",
		// CRLF in programs
		"PROGRAM p\r\nVAR x : INT;\r\nEND_VAR\r\nEND_PROGRAM\r\n",
		// CR only
		"PROGRAM p\rVAR x : INT;\rEND_VAR\rEND_PROGRAM\r",
		// Mixed
		"PROGRAM p\nVAR x : INT;\r\nEND_VAR\rEND_PROGRAM\n",
	}
	for _, input := range inputs {
		result := Parse("ws.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for whitespace input")
		}
	}
}

// TestAdversarial_BOM tests byte order mark at start.
func TestAdversarial_BOM(t *testing.T) {
	bom := "\xef\xbb\xbf"
	inputs := []string{
		bom,
		bom + "PROGRAM p END_PROGRAM",
		bom + "\n",
		bom + bom, // double BOM
	}
	for _, input := range inputs {
		result := Parse("bom.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for BOM input")
		}
	}
}

// TestAdversarial_DeeplyNestedExpr tests deeply nested parenthesized expression.
func TestAdversarial_DeeplyNestedExpr(t *testing.T) {
	depth := 500
	open := strings.Repeat("(", depth)
	close := strings.Repeat(")", depth)
	input := "PROGRAM p\nVAR x : INT; END_VAR\nx := " + open + "1" + close + ";\nEND_PROGRAM"
	result := Parse("deep_expr.st", input)
	if result.File == nil {
		t.Fatal("Parse returned nil File for deeply nested expression")
	}
}

// TestAdversarial_IncompleteConstructs tests various incomplete constructs.
func TestAdversarial_IncompleteConstructs(t *testing.T) {
	inputs := []string{
		"PROGRAM",
		"PROGRAM p",
		"PROGRAM p\nVAR",
		"PROGRAM p\nVAR x",
		"PROGRAM p\nVAR x :",
		"PROGRAM p\nVAR x : INT",
		"PROGRAM p\nVAR x : INT;",
		"FUNCTION_BLOCK",
		"FUNCTION",
		"FUNCTION f :",
		"FUNCTION f : INT",
		"TYPE",
		"TYPE t :",
		"IF",
		"IF TRUE",
		"IF TRUE THEN",
		"FOR",
		"FOR i",
		"FOR i :=",
		"FOR i := 0",
		"FOR i := 0 TO",
		"WHILE",
		"WHILE TRUE",
		"WHILE TRUE DO",
		"REPEAT",
		"CASE",
		"CASE x",
		"CASE x OF",
		"x :=",
		"x := ;",
		":= 5",
		// Trailing operators
		"PROGRAM p\nx := 1 +;\nEND_PROGRAM",
		"PROGRAM p\nx := 1 *;\nEND_PROGRAM",
		"PROGRAM p\nx := NOT;\nEND_PROGRAM",
	}
	for _, input := range inputs {
		result := Parse("incomplete.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for incomplete input: %q", input)
		}
	}
}

// TestAdversarial_RepeatedKeywords tests repeated keywords.
func TestAdversarial_RepeatedKeywords(t *testing.T) {
	inputs := []string{
		"PROGRAM PROGRAM PROGRAM",
		"VAR VAR VAR",
		"IF IF IF",
		"END_IF END_IF END_IF",
		"PROGRAM p\nIF IF IF THEN END_IF; END_IF; END_IF;\nEND_PROGRAM",
	}
	for _, input := range inputs {
		result := Parse("repeat_kw.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for repeated keywords: %q", input)
		}
	}
}

// TestAdversarial_VeryLongInput tests parsing a very long but valid program.
func TestAdversarial_VeryLongInput(t *testing.T) {
	var b strings.Builder
	b.WriteString("PROGRAM p\nVAR x : INT; END_VAR\n")
	for i := 0; i < 1000; i++ {
		b.WriteString("x := x + 1;\n")
	}
	b.WriteString("END_PROGRAM\n")
	result := Parse("long.st", b.String())
	if result.File == nil {
		t.Fatal("Parse returned nil File for very long input")
	}
}

// TestAdversarial_UnterminatedStrings tests unterminated string literals.
func TestAdversarial_UnterminatedStrings(t *testing.T) {
	inputs := []string{
		"'unterminated",
		"\"unterminated",
		"'unterminated\n",
		"PROGRAM p\nVAR s : STRING; END_VAR\ns := 'hello;\nEND_PROGRAM",
		"'",
		"\"",
	}
	for _, input := range inputs {
		result := Parse("unterm_str.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for unterminated string: %q", input)
		}
	}
}

// TestAdversarial_UnterminatedBlockComment tests unterminated block comments.
func TestAdversarial_UnterminatedBlockComment(t *testing.T) {
	inputs := []string{
		"(*",
		"(* unterminated",
		"(* nested (* still unterminated *)",
		"PROGRAM p (* oops\nEND_PROGRAM",
	}
	for _, input := range inputs {
		result := Parse("unterm_block.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for unterminated block comment: %q", input)
		}
	}
}

// TestAdversarial_NumberFormats tests edge cases in number literals.
func TestAdversarial_NumberFormats(t *testing.T) {
	inputs := []string{
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 0;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 16#FFFF;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 2#11111111;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 8#777;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 1_000_000;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : REAL; END_VAR\nx := 1.0E+38;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : REAL; END_VAR\nx := 1.0E-38;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : REAL; END_VAR\nx := 0.0;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 16#;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : INT; END_VAR\nx := #FF;\nEND_PROGRAM",
	}
	for _, input := range inputs {
		result := Parse("numbers.st", input)
		if result.File == nil {
			t.Fatalf("Parse returned nil File for number input: %q", input)
		}
	}
}
