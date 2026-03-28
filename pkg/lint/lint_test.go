package lint

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
)

// lintST parses the given ST code and runs the linter with default options.
func lintST(code string) []diag.Diagnostic {
	res := parser.Parse("test.st", code)
	result := LintFile(res.File, DefaultLintOptions())
	return result.Diags
}

// lintSTWithOpts parses then lints with the given options.
func lintSTWithOpts(code string, opts LintOptions) []diag.Diagnostic {
	res := parser.Parse("test.st", code)
	result := LintFile(res.File, opts)
	return result.Diags
}

func hasDiagCode(diags []diag.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func diagWithCode(diags []diag.Diagnostic, code string) *diag.Diagnostic {
	for _, d := range diags {
		if d.Code == code {
			return &d
		}
	}
	return nil
}

// --- PLCopen rules ---

func TestMagicNumber(t *testing.T) {
	code := `PROGRAM Main
VAR
    x : INT;
    y : INT;
END_VAR
    x := y + 42;
END_PROGRAM
`
	diags := lintST(code)
	if !hasDiagCode(diags, CodeMagicNumber) {
		t.Errorf("expected LINT001 magic number warning, got: %v", diags)
	}
}

func TestMagicNumber_AllowedValues(t *testing.T) {
	code := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 0;
    x := 1;
END_PROGRAM
`
	diags := lintST(code)
	if hasDiagCode(diags, CodeMagicNumber) {
		t.Errorf("0 and 1 should not be flagged as magic numbers, got: %v", diags)
	}
}

func TestMagicNumber_ConstantInit(t *testing.T) {
	// Constants should not flag magic numbers in their init values
	code := `PROGRAM Main
VAR CONSTANT
    MAX_SIZE : INT := 100;
END_VAR
END_PROGRAM
`
	diags := lintST(code)
	if hasDiagCode(diags, CodeMagicNumber) {
		t.Errorf("constant init values should not be flagged as magic numbers, got: %v", diags)
	}
}

func TestDeepNesting(t *testing.T) {
	code := `PROGRAM Main
VAR
    a : BOOL;
    b : BOOL;
    c : BOOL;
    d : BOOL;
    x : INT;
END_VAR
    IF a THEN
        IF b THEN
            IF c THEN
                IF d THEN
                    x := 1;
                END_IF;
            END_IF;
        END_IF;
    END_IF;
END_PROGRAM
`
	diags := lintST(code)
	if !hasDiagCode(diags, CodeDeepNesting) {
		t.Errorf("expected LINT002 deep nesting warning, got: %v", diags)
	}
}

func TestDeepNesting_OK(t *testing.T) {
	code := `PROGRAM Main
VAR
    a : BOOL;
    b : BOOL;
    c : BOOL;
    x : INT;
END_VAR
    IF a THEN
        IF b THEN
            IF c THEN
                x := 1;
            END_IF;
        END_IF;
    END_IF;
END_PROGRAM
`
	diags := lintST(code)
	if hasDiagCode(diags, CodeDeepNesting) {
		t.Errorf("3 levels of nesting should NOT trigger deep nesting warning, got: %v", diags)
	}
}

func TestLongPOU(t *testing.T) {
	// Build a POU with >200 statements
	var b strings.Builder
	b.WriteString("PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\n")
	for i := 0; i < 201; i++ {
		b.WriteString("    x := 1;\n")
	}
	b.WriteString("END_PROGRAM\n")

	diags := lintST(b.String())
	if !hasDiagCode(diags, CodeLongPOU) {
		t.Errorf("expected LINT003 long POU warning for 201 statements")
	}
}

func TestLongPOU_OK(t *testing.T) {
	var b strings.Builder
	b.WriteString("PROGRAM Main\nVAR\n    x : INT;\nEND_VAR\n")
	for i := 0; i < 10; i++ {
		b.WriteString("    x := 1;\n")
	}
	b.WriteString("END_PROGRAM\n")

	diags := lintST(b.String())
	if hasDiagCode(diags, CodeLongPOU) {
		t.Errorf("10 statements should NOT trigger long POU warning")
	}
}

func TestMissingReturnType(t *testing.T) {
	code := `FUNCTION MyFunc
VAR_INPUT
    x : INT;
END_VAR
    MyFunc := x;
END_FUNCTION
`
	diags := lintST(code)
	if !hasDiagCode(diags, CodeMissingReturnType) {
		t.Errorf("expected LINT004 missing return type warning, got: %v", diags)
	}
}

func TestMissingReturnType_OK(t *testing.T) {
	code := `FUNCTION MyFunc : INT
VAR_INPUT
    x : INT;
END_VAR
    MyFunc := x;
END_FUNCTION
`
	diags := lintST(code)
	if hasDiagCode(diags, CodeMissingReturnType) {
		t.Errorf("function with return type should NOT trigger missing return type warning")
	}
}

// --- Naming rules ---

func TestNamingFB_Bad(t *testing.T) {
	code := `FUNCTION_BLOCK my_motor
END_FUNCTION_BLOCK
`
	diags := lintST(code)
	if !hasDiagCode(diags, CodeNamingPOU) {
		t.Errorf("expected LINT005 naming warning for 'my_motor', got: %v", diags)
	}
}

func TestNamingFB_Good(t *testing.T) {
	code := `FUNCTION_BLOCK MyMotor
END_FUNCTION_BLOCK
`
	diags := lintST(code)
	if hasDiagCode(diags, CodeNamingPOU) {
		t.Errorf("'MyMotor' should NOT trigger POU naming warning")
	}
}

func TestNamingFB_WithUnderscore(t *testing.T) {
	code := `FUNCTION_BLOCK FB_Motor
END_FUNCTION_BLOCK
`
	diags := lintST(code)
	if hasDiagCode(diags, CodeNamingPOU) {
		t.Errorf("'FB_Motor' should NOT trigger POU naming warning (PascalCase with underscore segments)")
	}
}

func TestNamingProgram_Bad(t *testing.T) {
	code := `PROGRAM main_program
END_PROGRAM
`
	diags := lintST(code)
	if !hasDiagCode(diags, CodeNamingPOU) {
		t.Errorf("expected LINT005 naming warning for 'main_program', got: %v", diags)
	}
}

func TestNamingVar_Bad(t *testing.T) {
	code := `PROGRAM Main
VAR
    MyVar : INT;
END_VAR
END_PROGRAM
`
	diags := lintST(code)
	if !hasDiagCode(diags, CodeNamingVar) {
		t.Errorf("expected LINT006 naming warning for 'MyVar' (starts uppercase), got: %v", diags)
	}
}

func TestNamingVar_Good(t *testing.T) {
	code := `PROGRAM Main
VAR
    my_var : INT;
END_VAR
END_PROGRAM
`
	diags := lintST(code)
	if hasDiagCode(diags, CodeNamingVar) {
		t.Errorf("'my_var' should NOT trigger variable naming warning")
	}
}

func TestNamingConstant_Bad(t *testing.T) {
	code := `PROGRAM Main
VAR CONSTANT
    myConst : INT := 10;
END_VAR
END_PROGRAM
`
	diags := lintST(code)
	if !hasDiagCode(diags, CodeNamingConstant) {
		t.Errorf("expected LINT007 naming warning for constant 'myConst' (should be UPPER_SNAKE_CASE), got: %v", diags)
	}
}

func TestNamingConstant_Good(t *testing.T) {
	code := `PROGRAM Main
VAR CONSTANT
    MAX_SIZE : INT := 100;
END_VAR
END_PROGRAM
`
	diags := lintST(code)
	if hasDiagCode(diags, CodeNamingConstant) {
		t.Errorf("'MAX_SIZE' should NOT trigger constant naming warning")
	}
}

func TestNamingConventionNone(t *testing.T) {
	code := `FUNCTION_BLOCK my_motor
VAR
    MyVar : INT;
END_VAR
END_FUNCTION_BLOCK
`
	opts := LintOptions{NamingConvention: "none"}
	diags := lintSTWithOpts(code, opts)
	for _, d := range diags {
		if strings.HasPrefix(d.Code, "LINT005") || strings.HasPrefix(d.Code, "LINT006") || strings.HasPrefix(d.Code, "LINT007") {
			t.Errorf("naming convention 'none' should disable naming checks, got: %v", d)
		}
	}
}

// --- General ---

func TestCleanCode(t *testing.T) {
	code := `PROGRAM Main
VAR
    counter : INT;
END_VAR
    counter := counter + 1;
END_PROGRAM
`
	diags := lintST(code)
	if len(diags) != 0 {
		t.Errorf("clean code should produce zero diagnostics, got %d: %v", len(diags), diags)
	}
}

func TestDiagnosticPositions(t *testing.T) {
	code := `PROGRAM Main
VAR
    x : INT;
    y : INT;
END_VAR
    x := y + 42;
END_PROGRAM
`
	diags := lintST(code)
	d := diagWithCode(diags, CodeMagicNumber)
	if d == nil {
		t.Fatal("expected magic number diagnostic")
	}
	if d.Pos.Line == 0 || d.Pos.Col == 0 {
		t.Errorf("diagnostic should have non-zero line:col, got %d:%d", d.Pos.Line, d.Pos.Col)
	}
	if d.Pos.File != "test.st" {
		t.Errorf("diagnostic file should be 'test.st', got %q", d.Pos.File)
	}
}

func TestDiagnosticCodesPrefix(t *testing.T) {
	// All codes should be LINT-prefixed
	codes := []string{
		CodeMagicNumber,
		CodeDeepNesting,
		CodeLongPOU,
		CodeMissingReturnType,
		CodeNamingPOU,
		CodeNamingVar,
		CodeNamingConstant,
	}
	for _, c := range codes {
		if !strings.HasPrefix(c, "LINT") {
			t.Errorf("code %q should have LINT prefix", c)
		}
	}
}
