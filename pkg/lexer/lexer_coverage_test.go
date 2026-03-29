package lexer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Every token type ---

func TestTokenize_AllPunctuation(t *testing.T) {
	input := "( ) [ ] , ; : . .. ^ # => := + - * / ** = <> < <= > >= &"
	tokens := Tokenize("test.st", input)

	expected := []TokenKind{
		LParen, RParen, LBracket, RBracket, Comma, Semicolon, Colon,
		Dot, DotDot, Caret, Hash, Arrow, Assign, Plus, Minus, Star, Slash,
		Power, Eq, NotEq, Less, LessEq, Greater, GreaterEq, Ampersand,
	}

	sig := nonTrivia(tokens)
	// Last is EOF
	for _, ek := range expected {
		found := false
		for _, tok := range sig {
			if tok.Kind == ek {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing token kind: %s", ek.String())
		}
	}
}

func TestTokenize_AllBoolKeywords(t *testing.T) {
	input := "TRUE FALSE AND OR XOR NOT MOD"
	tokens := Tokenize("test.st", input)

	for _, kw := range []TokenKind{KwTrue, KwFalse, KwAnd, KwOr, KwXor, KwNot, KwMod} {
		assert.True(t, hasTokenKind(tokens, kw), "missing %s", kw.String())
	}
}

func TestTokenize_AllPrimitiveTypeKeywords(t *testing.T) {
	input := "BOOL BYTE WORD DWORD LWORD SINT INT DINT LINT USINT UINT UDINT ULINT REAL LREAL TIME DATE TOD DT"
	tokens := Tokenize("test.st", input)

	for _, kw := range []TokenKind{
		KwBool, KwByte, KwWord, KwDword, KwLword,
		KwSint, KwInt, KwDint, KwLint,
		KwUsint, KwUint, KwUdint, KwUlint,
		KwReal, KwLreal, KwTime, KwDate, KwTod, KwDt,
	} {
		assert.True(t, hasTokenKind(tokens, kw), "missing %s", kw.String())
	}
}

func TestTokenize_OOPKeywords(t *testing.T) {
	input := "EXTENDS IMPLEMENTS THIS SUPER ABSTRACT FINAL OVERRIDE PUBLIC PRIVATE PROTECTED INTERNAL"
	tokens := Tokenize("test.st", input)

	for _, kw := range []TokenKind{
		KwExtends, KwImplements, KwThis, KwSuper,
		KwAbstract, KwFinal, KwOverride,
		KwPublic, KwPrivate, KwProtected, KwInternal,
	} {
		assert.True(t, hasTokenKind(tokens, kw), "missing %s", kw.String())
	}
}

func TestTokenize_ControlFlowKeywords(t *testing.T) {
	input := "IF THEN ELSIF ELSE END_IF CASE OF END_CASE FOR TO BY DO END_FOR WHILE END_WHILE REPEAT UNTIL END_REPEAT EXIT CONTINUE RETURN"
	tokens := Tokenize("test.st", input)

	for _, kw := range []TokenKind{
		KwIf, KwThen, KwElsif, KwElse, KwEndIf,
		KwCase, KwOf, KwEndCase,
		KwFor, KwTo, KwBy, KwDo, KwEndFor,
		KwWhile, KwEndWhile,
		KwRepeat, KwUntil, KwEndRepeat,
		KwExit, KwContinue, KwReturn,
	} {
		assert.True(t, hasTokenKind(tokens, kw), "missing %s", kw.String())
	}
}

func TestTokenize_VarKeywords(t *testing.T) {
	input := "VAR VAR_INPUT VAR_OUTPUT VAR_IN_OUT VAR_TEMP VAR_GLOBAL VAR_ACCESS VAR_EXTERNAL VAR_CONFIG END_VAR CONSTANT RETAIN PERSISTENT AT"
	tokens := Tokenize("test.st", input)

	for _, kw := range []TokenKind{
		KwVar, KwVarInput, KwVarOutput, KwVarInOut,
		KwVarTemp, KwVarGlobal, KwVarAccess, KwVarExternal, KwVarConfig,
		KwEndVar, KwConstant, KwRetain, KwPersistent, KwAt,
	} {
		assert.True(t, hasTokenKind(tokens, kw), "missing %s", kw.String())
	}
}

func TestTokenize_TestCaseKeywords(t *testing.T) {
	input := "TEST_CASE END_TEST_CASE"
	tokens := Tokenize("test.st", input)
	assert.True(t, hasTokenKind(tokens, KwTestCase))
	assert.True(t, hasTokenKind(tokens, KwEndTestCase))
}

func TestTokenize_TypeSystemKeywords(t *testing.T) {
	input := "ARRAY STRUCT END_STRUCT POINTER REFERENCE STRING WSTRING"
	tokens := Tokenize("test.st", input)
	for _, kw := range []TokenKind{KwArray, KwStruct, KwEndStruct, KwPointer, KwReference, KwString, KwWString} {
		assert.True(t, hasTokenKind(tokens, kw), "missing %s", kw.String())
	}
}

func TestTokenize_POUKeywords(t *testing.T) {
	input := "PROGRAM END_PROGRAM FUNCTION_BLOCK END_FUNCTION_BLOCK FUNCTION END_FUNCTION TYPE END_TYPE INTERFACE END_INTERFACE METHOD END_METHOD PROPERTY END_PROPERTY ACTION END_ACTION"
	tokens := Tokenize("test.st", input)
	for _, kw := range []TokenKind{
		KwProgram, KwEndProgram, KwFunctionBlock, KwEndFunctionBlock,
		KwFunction, KwEndFunction, KwType, KwEndType,
		KwInterface, KwEndInterface, KwMethod, KwEndMethod,
		KwProperty, KwEndProperty, KwAction, KwEndAction,
	} {
		assert.True(t, hasTokenKind(tokens, kw), "missing %s", kw.String())
	}
}

// --- Typed literals ---

func TestTokenize_TypedLiterals_All(t *testing.T) {
	tests := []struct {
		input    string
		kind     TokenKind
		wantText string
	}{
		{"INT#42", TypedLiteral, "INT#42"},
		{"REAL#3.14", TypedLiteral, "REAL#3.14"},
		{"DINT#100", TypedLiteral, "DINT#100"},
		{"BOOL#1", TypedLiteral, "BOOL#1"},
		{"BYTE#255", TypedLiteral, "BYTE#255"},
		{"LREAL#1.0E-5", TypedLiteral, "LREAL#1.0E-5"},
		{"UINT#0", TypedLiteral, "UINT#0"},
		{"SINT#127", TypedLiteral, "SINT#127"},
	}
	for _, tt := range tests {
		tokens := Tokenize("test.st", tt.input)
		found := false
		for _, tok := range tokens {
			if tok.Kind == tt.kind && tok.Text == tt.wantText {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %s token for %q", tt.kind.String(), tt.input)
		}
	}
}

// --- Time/Date/DT/TOD literals ---

func TestTokenize_TimeDateLiterals(t *testing.T) {
	tests := []struct {
		input string
		kind  TokenKind
	}{
		{"T#5s", TimeLiteral},
		{"TIME#1h30m", TimeLiteral},
		{"T#100ms", TimeLiteral},
		{"D#2024-01-15", DateLiteral},
		{"DATE#2024-01-15", DateLiteral},
		{"DT#2024-01-15-12:00:00", DateTimeLiteral},
		{"TOD#12:30:00", TodLiteral},
	}
	for _, tt := range tests {
		tokens := Tokenize("test.st", tt.input)
		found := false
		for _, tok := range tokens {
			if tok.Kind == tt.kind {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s for %q", tt.kind.String(), tt.input)
		}
	}
}

// --- Nested block comments ---

func TestTokenize_NestedBlockComment_DeepNesting(t *testing.T) {
	input := "(* level1 (* level2 (* level3 *) back2 *) back1 *)"
	tokens := Tokenize("test.st", input)

	var comments []Token
	for _, tok := range tokens {
		if tok.Kind == BlockComment {
			comments = append(comments, tok)
		}
	}
	require.Len(t, comments, 1)
	assert.Equal(t, input, comments[0].Text)
}

func TestTokenize_UnterminatedBlockComment(t *testing.T) {
	input := "(* unclosed comment"
	tokens := Tokenize("test.st", input)
	// Should produce a BlockComment token (consuming to end)
	found := false
	for _, tok := range tokens {
		if tok.Kind == BlockComment {
			found = true
			break
		}
	}
	assert.True(t, found, "should produce BlockComment for unterminated")
}

// --- String literals with escape sequences ---

func TestTokenize_StringDoubledQuotes(t *testing.T) {
	input := "'it''s a test'"
	tokens := Tokenize("test.st", input)
	var strings []Token
	for _, tok := range tokens {
		if tok.Kind == StringLiteral {
			strings = append(strings, tok)
		}
	}
	require.Len(t, strings, 1)
	assert.Equal(t, "'it''s a test'", strings[0].Text)
}

func TestTokenize_WideStringDoubledQuotes(t *testing.T) {
	input := `"she said ""hello"""`
	tokens := Tokenize("test.st", input)
	var wstrings []Token
	for _, tok := range tokens {
		if tok.Kind == WStringLiteral {
			wstrings = append(wstrings, tok)
		}
	}
	require.Len(t, wstrings, 1)
}

func TestTokenize_EmptyString(t *testing.T) {
	input := "''"
	tokens := Tokenize("test.st", input)
	assert.True(t, hasTokenKind(tokens, StringLiteral))
}

// --- Edge cases ---

func TestTokenize_VeryLongIdentifier(t *testing.T) {
	ident := strings.Repeat("a", 1000)
	tokens := Tokenize("test.st", ident)
	sig := nonTrivia(tokens)
	require.Len(t, sig, 2) // ident + EOF
	assert.Equal(t, Ident, sig[0].Kind)
	assert.Equal(t, 1000, len(sig[0].Text))
}

func TestTokenize_Underscore_InNumber(t *testing.T) {
	input := "1_000_000"
	tokens := Tokenize("test.st", input)
	assert.True(t, hasTokenWithText(tokens, IntLiteral, "1_000_000"))
}

func TestTokenize_RealWithExponent(t *testing.T) {
	tests := []string{"1.0E5", "1.0e-3", "1.0E+2"}
	for _, input := range tests {
		tokens := Tokenize("test.st", input)
		assert.True(t, hasTokenKind(tokens, RealLiteral), "expected RealLiteral for %q", input)
	}
}

func TestTokenize_IntExponentForm(t *testing.T) {
	// 1E5 without decimal point should still be RealLiteral
	tokens := Tokenize("test.st", "1E5")
	assert.True(t, hasTokenKind(tokens, RealLiteral), "1E5 should be RealLiteral")
}

func TestTokenize_MultiBaseIntegers(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"16#FF"},
		{"2#1010"},
		{"8#77"},
		{"16#DEADBEEF"},
	}
	for _, tt := range tests {
		tokens := Tokenize("test.st", tt.input)
		assert.True(t, hasTokenWithText(tokens, IntLiteral, tt.input), "missing IntLiteral %s", tt.input)
	}
}

// --- TokenKind methods ---

func TestTokenKind_String_OutOfRange(t *testing.T) {
	k := TokenKind(9999)
	s := k.String()
	assert.Contains(t, s, "9999")
}

func TestTokenKind_IsKeyword(t *testing.T) {
	assert.True(t, KwProgram.IsKeyword())
	assert.True(t, KwEndTestCase.IsKeyword())
	assert.False(t, Ident.IsKeyword())
	assert.False(t, IntLiteral.IsKeyword())
}

func TestTokenKind_IsOperator(t *testing.T) {
	for _, k := range []TokenKind{Plus, Minus, Star, Slash, Power, Eq, NotEq, Less, LessEq, Greater, GreaterEq, Assign, Ampersand, KwAnd, KwOr, KwXor, KwNot, KwMod} {
		assert.True(t, k.IsOperator(), "%s should be operator", k.String())
	}
	assert.False(t, Ident.IsOperator())
	assert.False(t, LParen.IsOperator())
}

func TestTokenKind_IsTrivia(t *testing.T) {
	assert.True(t, Whitespace.IsTrivia())
	assert.True(t, LineComment.IsTrivia())
	assert.True(t, BlockComment.IsTrivia())
	assert.False(t, Ident.IsTrivia())
	assert.False(t, Pragma.IsTrivia())
}

// --- LookupKeyword ---

func TestLookupKeyword_Found(t *testing.T) {
	k, ok := LookupKeyword("PROGRAM")
	assert.True(t, ok)
	assert.Equal(t, KwProgram, k)
}

func TestLookupKeyword_CaseInsensitive(t *testing.T) {
	k, ok := LookupKeyword("program")
	assert.True(t, ok)
	assert.Equal(t, KwProgram, k)
}

func TestLookupKeyword_NotFound(t *testing.T) {
	_, ok := LookupKeyword("myVariable")
	assert.False(t, ok)
}

// --- Pragma ---

func TestTokenize_PragmaBasic(t *testing.T) {
	input := `{attribute 'qualified_only'}`
	tokens := Tokenize("test.st", input)
	assert.True(t, hasTokenKind(tokens, Pragma))
}

func TestTokenize_UnterminatedPragma(t *testing.T) {
	input := "{unclosed pragma"
	tokens := Tokenize("test.st", input)
	assert.True(t, hasTokenKind(tokens, Pragma))
}

// --- Position tracking ---

func TestTokenize_LineColTracking(t *testing.T) {
	input := "x\ny\nz"
	tokens := Tokenize("test.st", input)

	var xTok, yTok, zTok Token
	for _, tok := range tokens {
		switch {
		case tok.Kind == Ident && tok.Text == "x":
			xTok = tok
		case tok.Kind == Ident && tok.Text == "y":
			yTok = tok
		case tok.Kind == Ident && tok.Text == "z":
			zTok = tok
		}
	}

	assert.Equal(t, 1, xTok.Pos.Line)
	assert.Equal(t, 1, xTok.Pos.Col)
	assert.Equal(t, 2, yTok.Pos.Line)
	assert.Equal(t, 1, yTok.Pos.Col)
	assert.Equal(t, 3, zTok.Pos.Line)
}

// --- Slash (not line comment) ---

func TestTokenize_SlashAlone(t *testing.T) {
	input := "a / b"
	tokens := Tokenize("test.st", input)
	assert.True(t, hasTokenKind(tokens, Slash))
}

// --- Multiple tokens in sequence ---

func TestTokenize_ComplexExpression(t *testing.T) {
	input := "x := (a + b) * c ** d MOD e <> f AND NOT g"
	tokens := Tokenize("test.st", input)
	sig := nonTrivia(tokens)
	// Should have many tokens without any Illegal
	for _, tok := range sig {
		if tok.Kind == EOF {
			break
		}
		assert.NotEqual(t, Illegal, tok.Kind, "unexpected illegal: %q", tok.Text)
	}
}

// --- DATE_AND_TIME and TIME_OF_DAY prefixes ---

func TestTokenize_LongTimePrefixes(t *testing.T) {
	tests := []struct {
		input string
		kind  TokenKind
	}{
		{"DATE_AND_TIME#2024-01-15-12:00:00", DateTimeLiteral},
		{"TIME_OF_DAY#08:30:00", TodLiteral},
	}
	for _, tt := range tests {
		tokens := Tokenize("test.st", tt.input)
		found := false
		for _, tok := range tokens {
			if tok.Kind == tt.kind {
				found = true
				break
			}
		}
		assert.True(t, found, "expected %s for %q", tt.kind.String(), tt.input)
	}
}
