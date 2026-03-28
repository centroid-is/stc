package parser

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/require"
)

func TestTriviaLeadingLineComment(t *testing.T) {
	src := "PROGRAM Main\nVAR\n    // sensor input\n    x : INT;\nEND_VAR\nEND_PROGRAM\n"
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)

	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	require.Len(t, prog.VarBlocks, 1)
	require.Len(t, prog.VarBlocks[0].Declarations, 1)

	varDecl := prog.VarBlocks[0].Declarations[0]
	require.NotEmpty(t, varDecl.LeadingTrivia, "expected leading trivia on VarDecl")

	found := false
	for _, tr := range varDecl.LeadingTrivia {
		if tr.Kind == ast.TriviaLineComment {
			require.Contains(t, tr.Text, "// sensor input")
			found = true
		}
	}
	require.True(t, found, "expected line comment '// sensor input' in leading trivia")
}

func TestTriviaTrailingBlockComment(t *testing.T) {
	src := "PROGRAM Main\nVAR\n    x : INT; (* units *)\nEND_VAR\nEND_PROGRAM\n"
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)

	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	require.Len(t, prog.VarBlocks, 1)
	require.Len(t, prog.VarBlocks[0].Declarations, 1)

	varDecl := prog.VarBlocks[0].Declarations[0]

	// The block comment is on the same line as x's semicolon, so should be trailing
	found := false
	for _, tr := range varDecl.TrailingTrivia {
		if tr.Kind == ast.TriviaBlockComment {
			require.Contains(t, tr.Text, "(* units *)")
			found = true
		}
	}
	require.True(t, found, "expected block comment '(* units *)' in trailing trivia, got leading=%v trailing=%v",
		varDecl.LeadingTrivia, varDecl.TrailingTrivia)
}

func TestTriviaFileHeaderComment(t *testing.T) {
	src := "// file header\nPROGRAM Main\nEND_PROGRAM\n"
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)

	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	found := false
	for _, tr := range prog.LeadingTrivia {
		if tr.Kind == ast.TriviaLineComment {
			require.Contains(t, tr.Text, "// file header")
			found = true
		}
	}
	require.True(t, found, "expected '// file header' in leading trivia of ProgramDecl, got: %v", prog.LeadingTrivia)
}

func TestTriviaMultipleComments(t *testing.T) {
	src := `// header comment
PROGRAM Main
VAR
    // first var comment
    x : INT;
    // second var comment
    y : REAL;
END_VAR
END_PROGRAM
`
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics, got: %v", result.Diags)

	prog := result.File.Declarations[0].(*ast.ProgramDecl)

	// Header comment should be on ProgramDecl
	headerFound := false
	for _, tr := range prog.LeadingTrivia {
		if tr.Kind == ast.TriviaLineComment {
			headerFound = true
		}
	}
	require.True(t, headerFound, "expected header comment on ProgramDecl")

	require.Len(t, prog.VarBlocks, 1)
	require.Len(t, prog.VarBlocks[0].Declarations, 2)

	// First var comment on x
	xDecl := prog.VarBlocks[0].Declarations[0]
	xHasComment := false
	for _, tr := range xDecl.LeadingTrivia {
		if tr.Kind == ast.TriviaLineComment {
			require.Contains(t, tr.Text, "first var comment")
			xHasComment = true
		}
	}
	require.True(t, xHasComment, "expected comment on x VarDecl")

	// Second var comment on y
	yDecl := prog.VarBlocks[0].Declarations[1]
	yHasComment := false
	for _, tr := range yDecl.LeadingTrivia {
		if tr.Kind == ast.TriviaLineComment {
			require.Contains(t, tr.Text, "second var comment")
			yHasComment = true
		}
	}
	require.True(t, yHasComment, "expected comment on y VarDecl")
}

func TestTriviaExistingParserTestsPass(t *testing.T) {
	// Verify a basic parse still works correctly with trivia attachment
	src := "PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\n    x := 42;\nEND_PROGRAM\n"
	result := Parse("test.st", src)
	require.NotNil(t, result.File)
	require.Empty(t, result.Diags, "expected no diagnostics")

	prog := result.File.Declarations[0].(*ast.ProgramDecl)
	require.Equal(t, "Main", prog.Name.Name)
	require.Len(t, prog.VarBlocks, 1)
	require.Len(t, prog.Body, 1)
}
