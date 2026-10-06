package lexer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// sig returns the kinds and texts of the non-trivia tokens before EOF.
func sig(src string) ([]TokenKind, []string) {
	var kinds []TokenKind
	var texts []string
	for _, tok := range nonTrivia(Tokenize("t.st", src)) {
		if tok.Kind == EOF {
			break
		}
		kinds = append(kinds, tok.Kind)
		texts = append(texts, tok.Text)
	}
	return kinds, texts
}

func TestTypedBasedLiteral(t *testing.T) {
	single := []string{
		"BYTE#16#10",
		"WORD#2#1010",
		"DWORD#8#17",
		"LWORD#16#FFFF_FFFF",
		"byte#16#ff",
		"UINT#5",
		"INT#-3",
		"REAL#1.5E-3",
	}
	for _, src := range single {
		t.Run(src, func(t *testing.T) {
			kinds, texts := sig(src)
			require.Equal(t, []TokenKind{TypedLiteral}, kinds)
			require.Equal(t, []string{src}, texts)
		})
	}

	t.Run("time literal unchanged", func(t *testing.T) {
		kinds, texts := sig("T#1s")
		require.Equal(t, []TokenKind{TimeLiteral}, kinds)
		require.Equal(t, []string{"T#1s"}, texts)
	})

	t.Run("time of day keeps colons", func(t *testing.T) {
		kinds, texts := sig("TOD#12:30:00")
		require.Equal(t, []TokenKind{TodLiteral}, kinds)
		require.Equal(t, []string{"TOD#12:30:00"}, texts)
	})

	t.Run("in call argument", func(t *testing.T) {
		kinds, texts := sig("SHL(BYTE#16#10, n)")
		require.Equal(t, []TokenKind{Ident, LParen, TypedLiteral, Comma, Ident, RParen}, kinds)
		require.Equal(t, "BYTE#16#10", texts[2])
	})

	// Review LO-04: only bases 2, 8 and 16 are valid, and digits must
	// follow the #. Invalid based literals lex as one Illegal token.
	for _, src := range []string{"BYTE#16#", "DINT#16#", "WORD#3#10", "INT#10#5", "16#", "3#10", "16#__"} {
		t.Run("invalid based literal "+src, func(t *testing.T) {
			kinds, texts := sig(src)
			require.Equal(t, []TokenKind{Illegal}, kinds)
			require.Equal(t, []string{src}, texts)
		})
	}
	t.Run("valid untyped bases", func(t *testing.T) {
		kinds, _ := sig("2#1010 8#17 16#FF")
		require.Equal(t, []TokenKind{IntLiteral, IntLiteral, IntLiteral}, kinds)
	})

	t.Run("non-decimal value before hash is not a base", func(t *testing.T) {
		kinds, _ := sig("BYTE#1A#2")
		require.Equal(t, []TokenKind{TypedLiteral, Hash, IntLiteral}, kinds)
	})

	t.Run("colon after typed literal is a separate token", func(t *testing.T) {
		kinds, texts := sig("INT#5:")
		require.Equal(t, []TokenKind{TypedLiteral, Colon}, kinds)
		require.Equal(t, "INT#5", texts[0])
	})

	t.Run("assign after typed literal is a separate token", func(t *testing.T) {
		kinds, _ := sig("UINT#5:=")
		require.Equal(t, []TokenKind{TypedLiteral, Assign}, kinds)
	})
}
