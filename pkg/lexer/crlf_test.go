package lexer

import "testing"

// TestLineCommentCRLF checks that a line comment in a CRLF source does not
// carry the trailing '\r' into its token text. TwinCAT exports are CRLF and
// fmt/emit print comment trivia verbatim, so a stray CR would corrupt output.
func TestLineCommentCRLF(t *testing.T) {
	src := "// first comment\r\nVAR_GLOBAL\r\n\tx : INT; // trailing\r\nEND_VAR\r\n"
	toks := New("crlf.st", src).Tokenize()
	var comments []string
	for _, tok := range toks {
		if tok.Kind == LineComment {
			comments = append(comments, tok.Text)
		}
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 line comments, got %d: %q", len(comments), comments)
	}
	for _, c := range comments {
		if len(c) > 0 && c[len(c)-1] == '\r' {
			t.Errorf("line comment carries trailing CR: %q", c)
		}
	}
	if comments[0] != "// first comment" || comments[1] != "// trailing" {
		t.Errorf("unexpected comment texts: %q", comments)
	}
}
