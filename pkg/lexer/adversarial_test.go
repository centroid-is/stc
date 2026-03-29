package lexer

import (
	"strings"
	"testing"
)

// TestAdversarial_EmptyInput tests tokenizing empty input.
func TestAdversarial_EmptyInput(t *testing.T) {
	tokens := Tokenize("empty.st", "")
	if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
		t.Fatal("empty input should produce at least EOF")
	}
}

// TestAdversarial_NullBytes tests tokenizing null bytes.
func TestAdversarial_NullBytes(t *testing.T) {
	inputs := []string{
		"\x00",
		"\x00\x00\x00",
		"PROGRAM\x00p",
		"\x00PROGRAM p\x00",
	}
	for _, input := range inputs {
		tokens := Tokenize("null.st", input)
		if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
			t.Fatalf("null byte input should end with EOF")
		}
	}
}

// TestAdversarial_AllByteValues tests tokenizing every single byte value.
func TestAdversarial_AllByteValues(t *testing.T) {
	for b := 0; b <= 255; b++ {
		input := string([]byte{byte(b)})
		tokens := Tokenize("byte.st", input)
		if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
			t.Fatalf("byte 0x%02x did not produce EOF-terminated token list", b)
		}
	}
}

// TestAdversarial_BinaryData tests tokenizing binary data.
func TestAdversarial_BinaryData(t *testing.T) {
	// Build input with all byte values
	var data []byte
	for i := 0; i < 256; i++ {
		data = append(data, byte(i))
	}
	tokens := Tokenize("binary.st", string(data))
	if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
		t.Fatal("binary data did not produce EOF-terminated token list")
	}
}

// TestAdversarial_VeryLongIdentifier tests tokenizing a very long identifier.
func TestAdversarial_VeryLongIdentifier(t *testing.T) {
	input := strings.Repeat("x", 100000)
	tokens := Tokenize("long_ident.st", input)
	if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
		t.Fatal("long identifier did not produce EOF")
	}
	// Should be a single Ident token + EOF
	nonTrivia := filterNonTrivia(tokens)
	if len(nonTrivia) != 2 { // Ident + EOF
		t.Fatalf("expected 2 non-trivia tokens (Ident+EOF), got %d", len(nonTrivia))
	}
	if nonTrivia[0].Kind != Ident {
		t.Fatalf("expected Ident, got %s", nonTrivia[0].Kind)
	}
}

// TestAdversarial_VeryLongString tests tokenizing a very long string literal.
func TestAdversarial_VeryLongString(t *testing.T) {
	input := "'" + strings.Repeat("a", 100000) + "'"
	tokens := Tokenize("long_str.st", input)
	if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
		t.Fatal("long string did not produce EOF")
	}
}

// TestAdversarial_UnterminatedString tests unterminated string literals.
func TestAdversarial_UnterminatedString(t *testing.T) {
	inputs := []string{
		"'unterminated",
		"\"unterminated",
		"'",
		"\"",
		"'unterminated\n",
		"'with ''escape but no end",
	}
	for _, input := range inputs {
		tokens := Tokenize("unterm.st", input)
		if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
			t.Fatalf("unterminated string did not produce EOF: %q", input)
		}
	}
}

// TestAdversarial_UnterminatedBlockComment tests unterminated block comments.
func TestAdversarial_UnterminatedBlockComment(t *testing.T) {
	inputs := []string{
		"(*",
		"(* unterminated",
		"(* nested (* but not closed *)",
		"(* \x00 null in comment",
	}
	for _, input := range inputs {
		tokens := Tokenize("unterm_block.st", input)
		if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
			t.Fatalf("unterminated block comment did not produce EOF: %q", input)
		}
	}
}

// TestAdversarial_Unicode tests various unicode inputs.
func TestAdversarial_Unicode(t *testing.T) {
	inputs := []string{
		"\xef\xbb\xbf",           // BOM
		"\xc3\xa9",               // e-acute
		"\xf0\x9f\x98\x80",      // emoji
		"\xe4\xb8\xad\xe6\x96\x87", // Chinese
		"\xe2\x80\x8b",          // zero-width space
		"\xfe\xff",              // UTF-16 BOM (invalid UTF-8)
		"\xc0\x80",              // overlong null (invalid UTF-8)
		"\xed\xa0\x80",          // surrogate half (invalid UTF-8)
	}
	for _, input := range inputs {
		tokens := Tokenize("unicode.st", input)
		if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
			t.Fatalf("unicode input did not produce EOF")
		}
	}
}

// TestAdversarial_AllPunctuation tests every punctuation token.
func TestAdversarial_AllPunctuation(t *testing.T) {
	input := "( ) [ ] , ; : . .. ^ # => := + - * / ** = <> < <= > >= &"
	tokens := Tokenize("punct.st", input)
	if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
		t.Fatal("punctuation did not produce EOF")
	}
}

// TestAdversarial_ManyTokens tests input that produces a very large number of tokens.
func TestAdversarial_ManyTokens(t *testing.T) {
	// 10000 semicolons = 10000 Semicolon tokens + EOF
	input := strings.Repeat("; ", 10000)
	tokens := Tokenize("many_tokens.st", input)
	if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
		t.Fatal("many tokens did not produce EOF")
	}
}

// TestAdversarial_NumberEdgeCases tests edge cases in number lexing.
func TestAdversarial_NumberEdgeCases(t *testing.T) {
	inputs := []string{
		"0",
		"00000",
		"999999999999999999999999999999999",
		"16#FFFFFFFFFFFFFFFF",
		"2#1111111111111111111111111111111111111111111111111111111111111111",
		"8#7777777777777777777777",
		"1_000_000",
		"0.0",
		"1.0E+308",
		"1.0E-308",
		"1.0e999",
		".5",
		"5.",
		"1..10",
		"16#",
		"2#",
		"8#",
		"#FF",
	}
	for _, input := range inputs {
		tokens := Tokenize("numbers.st", input)
		if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
			t.Fatalf("number input did not produce EOF: %q", input)
		}
	}
}

// TestAdversarial_TimeLiterals tests time literal edge cases.
func TestAdversarial_TimeLiterals(t *testing.T) {
	inputs := []string{
		"T#0s",
		"T#1h30m45s500ms",
		"TIME#1d",
		"T#",
		"T#invalid",
		"T#999999999h",
		"T#-1s",
	}
	for _, input := range inputs {
		tokens := Tokenize("time.st", input)
		if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
			t.Fatalf("time literal did not produce EOF: %q", input)
		}
	}
}

// TestAdversarial_MixedNewlines tests all newline conventions.
func TestAdversarial_MixedNewlines(t *testing.T) {
	inputs := []string{
		"\n",
		"\r",
		"\r\n",
		"\n\r",
		"\r\n\r\n",
		"a\nb\rc\r\nd",
		strings.Repeat("\r\n", 10000),
	}
	for _, input := range inputs {
		tokens := Tokenize("newlines.st", input)
		if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
			t.Fatalf("newline input did not produce EOF")
		}
	}
}

func filterNonTrivia(tokens []Token) []Token {
	var result []Token
	for _, t := range tokens {
		if !t.Kind.IsTrivia() {
			result = append(result, t)
		}
	}
	return result
}
