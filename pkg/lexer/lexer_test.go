package lexer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testdataDir returns the absolute path to the testdata directory.
func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}

// readTestFile reads a testdata file and returns its content.
func readTestFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(testdataDir(), name)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read testdata file: %s", name)
	return string(data)
}

// nonTrivia filters out whitespace and comment tokens, returning only significant tokens.
func nonTrivia(tokens []Token) []Token {
	var result []Token
	for _, tok := range tokens {
		if !tok.Kind.IsTrivia() {
			result = append(result, tok)
		}
	}
	return result
}

// hasTokenKind checks if any token in the list matches the given kind.
func hasTokenKind(tokens []Token, kind TokenKind) bool {
	for _, tok := range tokens {
		if tok.Kind == kind {
			return true
		}
	}
	return false
}

// hasTokenWithText checks if any token matches both kind and text.
func hasTokenWithText(tokens []Token, kind TokenKind, text string) bool {
	for _, tok := range tokens {
		if tok.Kind == kind && tok.Text == text {
			return true
		}
	}
	return false
}

func TestTokenize_MotorControl(t *testing.T) {
	src := readTestFile(t, "motor_control.st")
	tokens := Tokenize("motor_control.st", src)
	significant := nonTrivia(tokens)

	// First non-trivia token is FUNCTION_BLOCK
	require.NotEmpty(t, significant)
	assert.Equal(t, KwFunctionBlock, significant[0].Kind)

	// Contains expected keywords
	assert.True(t, hasTokenKind(tokens, KwVarInput), "missing KwVarInput")
	assert.True(t, hasTokenKind(tokens, KwVarOutput), "missing KwVarOutput")
	assert.True(t, hasTokenKind(tokens, KwEndVar), "missing KwEndVar")
	assert.True(t, hasTokenKind(tokens, KwIf), "missing KwIf")
	assert.True(t, hasTokenKind(tokens, KwAnd), "missing KwAnd")
	assert.True(t, hasTokenKind(tokens, KwNot), "missing KwNot")
	assert.True(t, hasTokenKind(tokens, KwThen), "missing KwThen")
	assert.True(t, hasTokenKind(tokens, KwEndIf), "missing KwEndIf")
	assert.True(t, hasTokenKind(tokens, KwEndFunctionBlock), "missing KwEndFunctionBlock")

	// No illegal tokens
	for _, tok := range tokens {
		assert.NotEqual(t, Illegal, tok.Kind, "illegal token found: %q at %s", tok.Text, tok.Pos)
	}

	// Ident "FB_Motor" present
	assert.True(t, hasTokenWithText(tokens, Ident, "FB_Motor"), "missing ident FB_Motor")
}

func TestTokenize_TypedLiterals(t *testing.T) {
	src := readTestFile(t, "typed_literals.st")
	tokens := Tokenize("typed_literals.st", src)

	// TypedLiteral tokens
	assert.True(t, hasTokenWithText(tokens, TypedLiteral, "INT#42"), "missing INT#42")
	assert.True(t, hasTokenWithText(tokens, TypedLiteral, "REAL#3.14"), "missing REAL#3.14")

	// Multi-base integers
	assert.True(t, hasTokenWithText(tokens, IntLiteral, "16#FF"), "missing 16#FF")
	assert.True(t, hasTokenWithText(tokens, IntLiteral, "2#1010"), "missing 2#1010")

	// Time literals
	assert.True(t, hasTokenKind(tokens, TimeLiteral), "missing TimeLiteral")
	assert.True(t, hasTokenWithText(tokens, TimeLiteral, "T#5s"), "missing T#5s")
	assert.True(t, hasTokenWithText(tokens, TimeLiteral, "T#1h30m"), "missing T#1h30m")

	// Date literal
	assert.True(t, hasTokenKind(tokens, DateLiteral), "missing DateLiteral")
	assert.True(t, hasTokenWithText(tokens, DateLiteral, "D#2024-01-15"), "missing D#2024-01-15")

	// LREAL typed literal
	assert.True(t, hasTokenWithText(tokens, TypedLiteral, "LREAL#1.0E-5"), "missing LREAL#1.0E-5")
}

func TestTokenize_OOP(t *testing.T) {
	src := readTestFile(t, "oop_extensions.st")
	tokens := Tokenize("oop_extensions.st", src)

	assert.True(t, hasTokenKind(tokens, KwInterface), "missing KwInterface")
	assert.True(t, hasTokenKind(tokens, KwEndInterface), "missing KwEndInterface")
	assert.True(t, hasTokenKind(tokens, KwExtends), "missing KwExtends")
	assert.True(t, hasTokenKind(tokens, KwImplements), "missing KwImplements")
	assert.True(t, hasTokenKind(tokens, KwMethod), "missing KwMethod")
	assert.True(t, hasTokenKind(tokens, KwEndMethod), "missing KwEndMethod")
	assert.True(t, hasTokenKind(tokens, KwPublic), "missing KwPublic")
	assert.True(t, hasTokenKind(tokens, KwProperty), "missing KwProperty")
	assert.True(t, hasTokenKind(tokens, KwEndProperty), "missing KwEndProperty")

	// No illegal tokens
	for _, tok := range tokens {
		assert.NotEqual(t, Illegal, tok.Kind, "illegal token found: %q at %s", tok.Text, tok.Pos)
	}
}

func TestTokenize_Pragmas(t *testing.T) {
	src := readTestFile(t, "pragmas.st")
	tokens := Tokenize("pragmas.st", src)

	// Collect pragma tokens
	var pragmas []Token
	for _, tok := range tokens {
		if tok.Kind == Pragma {
			pragmas = append(pragmas, tok)
		}
	}

	require.Len(t, pragmas, 2, "expected 2 pragma tokens")
	assert.Contains(t, pragmas[0].Text, "qualified_only", "first pragma should contain qualified_only")
	assert.Contains(t, pragmas[1].Text, "symbol", "second pragma should contain symbol")
}

func TestTokenize_NestedComments(t *testing.T) {
	input := "(* outer (* inner *) still outer *)"
	tokens := Tokenize("test.st", input)
	significant := nonTrivia(tokens)

	// Should have just BlockComment and EOF
	require.Len(t, significant, 1, "expected 1 non-trivia token (EOF)")
	assert.Equal(t, EOF, significant[0].Kind)

	// The block comment itself
	var comments []Token
	for _, tok := range tokens {
		if tok.Kind == BlockComment {
			comments = append(comments, tok)
		}
	}
	require.Len(t, comments, 1, "expected single BlockComment token")
	assert.Equal(t, input, comments[0].Text, "BlockComment should contain full nested text")
}

func TestTokenize_CaseInsensitive(t *testing.T) {
	input := "If x then y := true; End_If;"
	tokens := Tokenize("test.st", input)

	assert.True(t, hasTokenKind(tokens, KwIf), "missing KwIf for 'If'")
	assert.True(t, hasTokenKind(tokens, KwThen), "missing KwThen for 'then'")
	assert.True(t, hasTokenKind(tokens, KwTrue), "missing KwTrue for 'true'")
	assert.True(t, hasTokenKind(tokens, KwEndIf), "missing KwEndIf for 'End_If'")

	// Verify original casing preserved
	assert.True(t, hasTokenWithText(tokens, KwIf, "If"), "original casing 'If' not preserved")
	assert.True(t, hasTokenWithText(tokens, KwThen, "then"), "original casing 'then' not preserved")
	assert.True(t, hasTokenWithText(tokens, KwEndIf, "End_If"), "original casing 'End_If' not preserved")
}

func TestTokenize_Positions(t *testing.T) {
	input := "x := 42;\ny := TRUE;"
	tokens := Tokenize("test.st", input)

	// Find specific tokens
	var xTok, numTok, yTok Token
	for _, tok := range tokens {
		switch {
		case tok.Kind == Ident && tok.Text == "x":
			xTok = tok
		case tok.Kind == IntLiteral && tok.Text == "42":
			numTok = tok
		case tok.Kind == Ident && tok.Text == "y":
			yTok = tok
		}
	}

	// x at line 1, col 1
	assert.Equal(t, 1, xTok.Pos.Line, "x line")
	assert.Equal(t, 1, xTok.Pos.Col, "x col")

	// 42 at line 1, col 6
	assert.Equal(t, 1, numTok.Pos.Line, "42 line")
	assert.Equal(t, 6, numTok.Pos.Col, "42 col")

	// y at line 2, col 1
	assert.Equal(t, 2, yTok.Pos.Line, "y line")
	assert.Equal(t, 1, yTok.Pos.Col, "y col")
}

func TestTokenize_Operators(t *testing.T) {
	input := "a + b * c ** d MOD e <> f <= g AND h OR i XOR j"
	tokens := Tokenize("test.st", input)

	assert.True(t, hasTokenKind(tokens, Plus), "missing Plus")
	assert.True(t, hasTokenKind(tokens, Star), "missing Star")
	assert.True(t, hasTokenKind(tokens, Power), "missing Power")
	assert.True(t, hasTokenKind(tokens, KwMod), "missing KwMod")
	assert.True(t, hasTokenKind(tokens, NotEq), "missing NotEq")
	assert.True(t, hasTokenKind(tokens, LessEq), "missing LessEq")
	assert.True(t, hasTokenKind(tokens, KwAnd), "missing KwAnd")
	assert.True(t, hasTokenKind(tokens, KwOr), "missing KwOr")
	assert.True(t, hasTokenKind(tokens, KwXor), "missing KwXor")
}

func TestTokenize_64BitTypes(t *testing.T) {
	input := "VAR x : LINT; y : LREAL; z : LWORD; w : ULINT; END_VAR"
	tokens := Tokenize("test.st", input)

	assert.True(t, hasTokenKind(tokens, KwLint), "missing KwLint")
	assert.True(t, hasTokenKind(tokens, KwLreal), "missing KwLreal")
	assert.True(t, hasTokenKind(tokens, KwLword), "missing KwLword")
	assert.True(t, hasTokenKind(tokens, KwUlint), "missing KwUlint")
}

func TestTokenize_EmptyInput(t *testing.T) {
	tokens := Tokenize("test.st", "")
	require.Len(t, tokens, 1)
	assert.Equal(t, EOF, tokens[0].Kind)
}

func TestTokenize_IllegalChar(t *testing.T) {
	tokens := Tokenize("test.st", "@$")

	var illegal []Token
	for _, tok := range tokens {
		if tok.Kind == Illegal {
			illegal = append(illegal, tok)
		}
	}
	assert.Len(t, illegal, 2, "expected 2 illegal tokens for @ and $")
}

func TestTokenize_StringEscapes(t *testing.T) {
	input := "'hello ''world'''"
	tokens := Tokenize("test.st", input)

	var strings []Token
	for _, tok := range tokens {
		if tok.Kind == StringLiteral {
			strings = append(strings, tok)
		}
	}
	require.Len(t, strings, 1)
	assert.Equal(t, "'hello ''world'''", strings[0].Text)
}

func TestTokenize_WideString(t *testing.T) {
	input := `"wide string"`
	tokens := Tokenize("test.st", input)

	assert.True(t, hasTokenKind(tokens, WStringLiteral), "missing WStringLiteral")
}

func TestTokenize_RangeOperator(t *testing.T) {
	input := "1..10"
	tokens := Tokenize("test.st", input)

	assert.True(t, hasTokenKind(tokens, IntLiteral), "missing IntLiteral")
	assert.True(t, hasTokenKind(tokens, DotDot), "missing DotDot")
}

func TestTokenize_AssignAndArrow(t *testing.T) {
	input := "x := 5; y => z"
	tokens := Tokenize("test.st", input)

	assert.True(t, hasTokenKind(tokens, Assign), "missing Assign")
	assert.True(t, hasTokenKind(tokens, Arrow), "missing Arrow")
}

func TestTokenize_TriviaPreservation(t *testing.T) {
	input := "x := 1; // comment\ny := 2;"
	tokens := Tokenize("test.st", input)

	// Check that trivia tokens are present
	assert.True(t, hasTokenKind(tokens, Whitespace), "missing Whitespace trivia")
	assert.True(t, hasTokenKind(tokens, LineComment), "missing LineComment trivia")

	// Check comment content
	for _, tok := range tokens {
		if tok.Kind == LineComment {
			assert.Contains(t, tok.Text, "comment")
			break
		}
	}
}

func TestDirectAddr_SingleTokens(t *testing.T) {
	tests := []struct {
		input string
		text  string
	}{
		{"%IX0.0", "%IX0.0"},
		{"%QW4", "%QW4"},
		{"%MD12", "%MD12"},
		{"%I*", "%I*"},
		{"%QB0", "%QB0"},
		{"%I0.0", "%I0.0"},
		{"%ix0.0", "%ix0.0"},
		{"%IB10", "%IB10"},
		{"%QD100", "%QD100"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens := Tokenize("test.st", tt.input)
			sig := nonTrivia(tokens)
			// Should have DirectAddr + EOF
			require.True(t, len(sig) >= 1, "expected at least 1 non-trivia token")
			assert.Equal(t, DirectAddr, sig[0].Kind, "expected DirectAddr token")
			assert.Equal(t, tt.text, sig[0].Text, "token text mismatch")
		})
	}
}

func TestDirectAddr_ATContext(t *testing.T) {
	input := "x AT %IX0.0 : BOOL"
	tokens := Tokenize("test.st", input)
	sig := nonTrivia(tokens)

	// Expected: Ident("x"), KwAt, DirectAddr("%IX0.0"), Colon, KwBool, EOF
	require.True(t, len(sig) >= 5, "expected at least 5 non-trivia tokens, got %d", len(sig))
	assert.Equal(t, Ident, sig[0].Kind)
	assert.Equal(t, "x", sig[0].Text)
	assert.Equal(t, KwAt, sig[1].Kind)
	assert.Equal(t, DirectAddr, sig[2].Kind)
	assert.Equal(t, "%IX0.0", sig[2].Text)
	assert.Equal(t, Colon, sig[3].Kind)
	assert.Equal(t, KwBool, sig[4].Kind)
}
