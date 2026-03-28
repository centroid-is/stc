package format

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
)

// formatST is a test helper that parses ST source and formats it.
func formatST(input string) string {
	result := parser.Parse("test.st", input)
	return Format(result.File, DefaultFormatOptions())
}

// formatSTWith parses and formats with custom options.
func formatSTWith(input string, opts FormatOptions) string {
	result := parser.Parse("test.st", input)
	return Format(result.File, opts)
}

func TestFormatProgram(t *testing.T) {
	input := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "PROGRAM Main") {
		t.Errorf("expected PROGRAM Main, got:\n%s", got)
	}
	if !strings.Contains(got, "    x : INT;") {
		t.Errorf("expected 4-space indented var decl, got:\n%s", got)
	}
	if !strings.Contains(got, "    x := 42;") {
		t.Errorf("expected 4-space indented assignment, got:\n%s", got)
	}
	if !strings.Contains(got, "END_PROGRAM") {
		t.Errorf("expected END_PROGRAM, got:\n%s", got)
	}
}

func TestFormatTwoSpaceIndent(t *testing.T) {
	input := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`
	opts := FormatOptions{Indent: "  ", UppercaseKeywords: true}
	got := formatSTWith(input, opts)
	if !strings.Contains(got, "  x : INT;") {
		t.Errorf("expected 2-space indented var decl, got:\n%s", got)
	}
	if !strings.Contains(got, "  x := 42;") {
		t.Errorf("expected 2-space indented assignment, got:\n%s", got)
	}
}

func TestFormatLowercaseKeywords(t *testing.T) {
	input := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`
	opts := FormatOptions{Indent: "    ", UppercaseKeywords: false}
	got := formatSTWith(input, opts)
	if !strings.Contains(got, "program Main") {
		t.Errorf("expected lowercase 'program', got:\n%s", got)
	}
	if !strings.Contains(got, "end_program") {
		t.Errorf("expected lowercase 'end_program', got:\n%s", got)
	}
	if !strings.Contains(got, "var") {
		t.Errorf("expected lowercase 'var', got:\n%s", got)
	}
	if !strings.Contains(got, "end_var") {
		t.Errorf("expected lowercase 'end_var', got:\n%s", got)
	}
}

func TestFormatPreservesLineComments(t *testing.T) {
	// Build an AST with trivia manually attached to verify the formatter
	// emits trivia correctly. The current parser does not attach trivia,
	// but the formatter infrastructure handles it when present.
	result := parser.Parse("test.st", "PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\n    x := 42;\nEND_PROGRAM\n")
	file := result.File

	// Manually attach a line comment as leading trivia to the VarDecl
	file.Declarations[0].(*ast.ProgramDecl).VarBlocks[0].Declarations[0].LeadingTrivia = []ast.Trivia{
		{Kind: ast.TriviaLineComment, Text: "// this is a line comment\n"},
	}

	got := Format(file, DefaultFormatOptions())
	if !strings.Contains(got, "// this is a line comment") {
		t.Errorf("expected line comment preserved, got:\n%s", got)
	}
}

func TestFormatPreservesBlockComments(t *testing.T) {
	// Same approach: manually attach block comment trivia to verify emission.
	result := parser.Parse("test.st", "PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\n    x := 42;\nEND_PROGRAM\n")
	file := result.File

	file.Declarations[0].(*ast.ProgramDecl).VarBlocks[0].Declarations[0].LeadingTrivia = []ast.Trivia{
		{Kind: ast.TriviaBlockComment, Text: "(* block comment *)"},
	}

	got := Format(file, DefaultFormatOptions())
	if !strings.Contains(got, "(* block comment *)") {
		t.Errorf("expected block comment preserved, got:\n%s", got)
	}
}

func TestFormatIdempotent(t *testing.T) {
	inputs := []string{
		`PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`,
		`FUNCTION Add : INT
VAR_INPUT
    a : INT;
    b : INT;
END_VAR
    Add := a + b;
END_FUNCTION
`,
		`PROGRAM Logic
VAR
    flag : BOOL;
    count : INT;
END_VAR
    IF flag THEN
        count := count + 1;
    ELSE
        count := 0;
    END_IF;
END_PROGRAM
`,
	}

	for i, input := range inputs {
		first := formatST(input)
		if first == "" {
			t.Fatalf("input %d: first format returned empty", i)
		}
		second := formatSTWith(first, DefaultFormatOptions())
		if first != second {
			t.Errorf("input %d: not idempotent.\nFirst:\n%s\nSecond:\n%s", i, first, second)
		}
	}
}

func TestFormatFunctionBlock(t *testing.T) {
	input := `FUNCTION_BLOCK MyFB EXTENDS BaseFB IMPLEMENTS IRunnable
VAR
    counter : INT;
END_VAR
    counter := counter + 1;
END_FUNCTION_BLOCK
`
	got := formatST(input)
	if !strings.Contains(got, "FUNCTION_BLOCK MyFB") {
		t.Errorf("expected FUNCTION_BLOCK MyFB, got:\n%s", got)
	}
	if !strings.Contains(got, "EXTENDS BaseFB") {
		t.Errorf("expected EXTENDS BaseFB, got:\n%s", got)
	}
	if !strings.Contains(got, "IMPLEMENTS IRunnable") {
		t.Errorf("expected IMPLEMENTS IRunnable, got:\n%s", got)
	}
	if !strings.Contains(got, "END_FUNCTION_BLOCK") {
		t.Errorf("expected END_FUNCTION_BLOCK, got:\n%s", got)
	}
}

func TestFormatIfElsifElse(t *testing.T) {
	input := `PROGRAM Main
VAR
    x : INT;
END_VAR
    IF x > 10 THEN
        x := 10;
    ELSIF x > 5 THEN
        x := 5;
    ELSE
        x := 0;
    END_IF;
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "IF") {
		t.Errorf("expected IF, got:\n%s", got)
	}
	if !strings.Contains(got, "ELSIF") {
		t.Errorf("expected ELSIF, got:\n%s", got)
	}
	if !strings.Contains(got, "ELSE") {
		t.Errorf("expected ELSE, got:\n%s", got)
	}
	if !strings.Contains(got, "END_IF;") {
		t.Errorf("expected END_IF;, got:\n%s", got)
	}
}

func TestFormatForLoop(t *testing.T) {
	input := `PROGRAM Main
VAR
    i : INT;
    sum : INT;
END_VAR
    FOR i := 1 TO 10 BY 1 DO
        sum := sum + i;
    END_FOR;
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "FOR") {
		t.Errorf("expected FOR, got:\n%s", got)
	}
	if !strings.Contains(got, "TO") {
		t.Errorf("expected TO, got:\n%s", got)
	}
	if !strings.Contains(got, "BY") {
		t.Errorf("expected BY, got:\n%s", got)
	}
	if !strings.Contains(got, "DO") {
		t.Errorf("expected DO, got:\n%s", got)
	}
	if !strings.Contains(got, "END_FOR;") {
		t.Errorf("expected END_FOR;, got:\n%s", got)
	}
}

func TestFormatWhileLoop(t *testing.T) {
	input := `PROGRAM Main
VAR
    x : INT;
END_VAR
    WHILE x < 10 DO
        x := x + 1;
    END_WHILE;
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "WHILE") {
		t.Errorf("expected WHILE, got:\n%s", got)
	}
	if !strings.Contains(got, "END_WHILE;") {
		t.Errorf("expected END_WHILE;, got:\n%s", got)
	}
}

func TestFormatRepeatLoop(t *testing.T) {
	input := `PROGRAM Main
VAR
    x : INT;
END_VAR
    REPEAT
        x := x + 1;
    UNTIL x >= 10
    END_REPEAT;
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "REPEAT") {
		t.Errorf("expected REPEAT, got:\n%s", got)
	}
	if !strings.Contains(got, "UNTIL") {
		t.Errorf("expected UNTIL, got:\n%s", got)
	}
	if !strings.Contains(got, "END_REPEAT;") {
		t.Errorf("expected END_REPEAT;, got:\n%s", got)
	}
}

func TestFormatCaseStatement(t *testing.T) {
	input := `PROGRAM Main
VAR
    x : INT;
    y : INT;
END_VAR
    CASE x OF
        1:
            y := 10;
        2, 3:
            y := 20;
    ELSE
        y := 0;
    END_CASE;
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "CASE") {
		t.Errorf("expected CASE, got:\n%s", got)
	}
	if !strings.Contains(got, "OF") {
		t.Errorf("expected OF, got:\n%s", got)
	}
	if !strings.Contains(got, "END_CASE;") {
		t.Errorf("expected END_CASE;, got:\n%s", got)
	}
}

func TestFormatTypeDecl(t *testing.T) {
	input := `TYPE MyStruct :
STRUCT
    x : INT;
    y : REAL;
END_STRUCT
END_TYPE
`
	got := formatST(input)
	if !strings.Contains(got, "TYPE MyStruct") {
		t.Errorf("expected TYPE MyStruct, got:\n%s", got)
	}
	if !strings.Contains(got, "STRUCT") {
		t.Errorf("expected STRUCT, got:\n%s", got)
	}
	if !strings.Contains(got, "END_STRUCT") {
		t.Errorf("expected END_STRUCT, got:\n%s", got)
	}
	if !strings.Contains(got, "END_TYPE") {
		t.Errorf("expected END_TYPE, got:\n%s", got)
	}
}

func TestFormatNilInput(t *testing.T) {
	got := Format(nil, DefaultFormatOptions())
	if got != "" {
		t.Errorf("expected empty string for nil input, got: %q", got)
	}
}
