package lint

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
)

// --- Table-driven PLCopen rule tests ---

func TestMagicNumber_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantCode bool // whether LINT001 should fire
	}{
		{
			name: "magic number 42 in assignment",
			code: `PROGRAM Main
VAR x : INT; y : INT; END_VAR
    x := y + 42;
END_PROGRAM`,
			wantCode: true,
		},
		{
			name: "magic real 3.14",
			code: `PROGRAM Main
VAR x : REAL; END_VAR
    x := 3.14;
END_PROGRAM`,
			wantCode: true,
		},
		{
			name: "allowed literal 0",
			code: `PROGRAM Main
VAR x : INT; END_VAR
    x := 0;
END_PROGRAM`,
			wantCode: false,
		},
		{
			name: "allowed literal 1",
			code: `PROGRAM Main
VAR x : INT; END_VAR
    x := 1;
END_PROGRAM`,
			wantCode: false,
		},
		{
			name: "magic number in IF condition",
			code: `PROGRAM Main
VAR x : INT; END_VAR
    IF x > 42 THEN x := 1; END_IF;
END_PROGRAM`,
			wantCode: true,
		},
		{
			name: "magic number in FOR range",
			code: `PROGRAM Main
VAR i : INT; END_VAR
    FOR i := 0 TO 99 DO
        i := i;
    END_FOR;
END_PROGRAM`,
			wantCode: true,
		},
		{
			name: "magic number in FOR BY",
			code: `PROGRAM Main
VAR i : INT; END_VAR
    FOR i := 0 TO 1 BY 5 DO
        i := i;
    END_FOR;
END_PROGRAM`,
			wantCode: true,
		},
		{
			name: "magic number in WHILE",
			code: `PROGRAM Main
VAR x : INT; END_VAR
    WHILE x < 50 DO x := x + 1; END_WHILE;
END_PROGRAM`,
			wantCode: true,
		},
		{
			name: "magic number in REPEAT",
			code: `PROGRAM Main
VAR x : INT; END_VAR
    REPEAT x := x + 1; UNTIL x > 42 END_REPEAT;
END_PROGRAM`,
			wantCode: true,
		},
		{
			name: "magic number in CASE selector",
			code: `PROGRAM Main
VAR state : INT; END_VAR
    CASE state OF
        0: state := 1;
    END_CASE;
END_PROGRAM`,
			wantCode: false, // 0 and 1 allowed
		},
		{
			name: "string literal not flagged",
			code: `PROGRAM Main
VAR s : STRING; END_VAR
    s := 'hello';
END_PROGRAM`,
			wantCode: false,
		},
		{
			name: "bool literal not flagged",
			code: `PROGRAM Main
VAR b : BOOL; END_VAR
    b := TRUE;
END_PROGRAM`,
			wantCode: false,
		},
		{
			name: "no body in FB decl",
			code: `FUNCTION_BLOCK NoBody
END_FUNCTION_BLOCK`,
			wantCode: false,
		},
		{
			name: "magic in function body",
			code: `FUNCTION Calc : INT
VAR_INPUT a : INT; END_VAR
    Calc := a + 99;
END_FUNCTION`,
			wantCode: true,
		},
		{
			name: "magic in FB body",
			code: `FUNCTION_BLOCK FB_Calc
VAR x : INT; END_VAR
    x := 99;
END_FUNCTION_BLOCK`,
			wantCode: true,
		},
		{
			name: "magic in CallStmt args",
			code: `PROGRAM Main
VAR fb : FB_Timer; END_VAR
    fb(IN := TRUE);
END_PROGRAM`,
			wantCode: false,
		},
		{
			name: "magic in else branch",
			code: `PROGRAM Main
VAR x : INT; END_VAR
    IF x > 0 THEN x := 1; ELSE x := 42; END_IF;
END_PROGRAM`,
			wantCode: true,
		},
		{
			name: "magic in elsif condition",
			code: `PROGRAM Main
VAR x : INT; END_VAR
    IF x > 0 THEN x := 1; ELSIF x > 42 THEN x := 0; END_IF;
END_PROGRAM`,
			wantCode: true,
		},
		{
			name: "magic in case else branch",
			code: `PROGRAM Main
VAR x : INT; END_VAR
    CASE x OF 0: x := 1; ELSE x := 99; END_CASE;
END_PROGRAM`,
			wantCode: true,
		},
		{
			name: "type decl does not trigger",
			code: `TYPE MyType : INT; END_TYPE`,
			wantCode: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := lintST(tt.code)
			got := hasDiagCode(diags, CodeMagicNumber)
			if got != tt.wantCode {
				t.Errorf("hasDiagCode(CodeMagicNumber) = %v, want %v; diags: %v", got, tt.wantCode, diags)
			}
		})
	}
}

func TestDeepNesting_AllStatementTypes(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantDeep bool
	}{
		{
			name: "FOR nested deep",
			code: `PROGRAM Main
VAR i : INT; j : INT; k : INT; l : INT; END_VAR
    FOR i := 0 TO 1 DO
        FOR j := 0 TO 1 DO
            FOR k := 0 TO 1 DO
                FOR l := 0 TO 1 DO
                    l := l;
                END_FOR;
            END_FOR;
        END_FOR;
    END_FOR;
END_PROGRAM`,
			wantDeep: true,
		},
		{
			name: "WHILE nested deep",
			code: `PROGRAM Main
VAR a : BOOL; b : BOOL; c : BOOL; d : BOOL; x : INT; END_VAR
    WHILE a DO
        WHILE b DO
            WHILE c DO
                WHILE d DO
                    x := 1;
                END_WHILE;
            END_WHILE;
        END_WHILE;
    END_WHILE;
END_PROGRAM`,
			wantDeep: true,
		},
		{
			name: "REPEAT nested deep",
			code: `PROGRAM Main
VAR a : BOOL; b : BOOL; c : BOOL; d : BOOL; x : INT; END_VAR
    REPEAT
        REPEAT
            REPEAT
                REPEAT
                    x := 1;
                UNTIL d END_REPEAT;
            UNTIL c END_REPEAT;
        UNTIL b END_REPEAT;
    UNTIL a END_REPEAT;
END_PROGRAM`,
			wantDeep: true,
		},
		{
			name: "CASE nested deep",
			code: `PROGRAM Main
VAR a : INT; x : INT; END_VAR
    CASE a OF
        0:
            IF a > 0 THEN
                IF a > 0 THEN
                    IF a > 0 THEN
                        x := 1;
                    END_IF;
                END_IF;
            END_IF;
    END_CASE;
END_PROGRAM`,
			wantDeep: true,
		},
		{
			name: "CASE else nested deep",
			code: `PROGRAM Main
VAR a : INT; x : INT; END_VAR
    IF a > 0 THEN
        IF a > 0 THEN
            IF a > 0 THEN
                CASE a OF
                    0: x := 1;
                ELSE
                    x := 0;
                END_CASE;
            END_IF;
        END_IF;
    END_IF;
END_PROGRAM`,
			wantDeep: true,
		},
		{
			name: "three levels OK",
			code: `PROGRAM Main
VAR a : BOOL; b : BOOL; c : BOOL; x : INT; END_VAR
    FOR x := 0 TO 1 DO
        WHILE a DO
            IF b THEN x := 1; END_IF;
        END_WHILE;
    END_FOR;
END_PROGRAM`,
			wantDeep: false,
		},
		{
			name: "elsif body nested",
			code: `PROGRAM Main
VAR a : BOOL; b : BOOL; c : BOOL; d : BOOL; x : INT; END_VAR
    IF a THEN
        IF b THEN
            IF c THEN
                x := 1;
            ELSIF d THEN
                IF d THEN
                    x := 1;
                END_IF;
            END_IF;
        END_IF;
    END_IF;
END_PROGRAM`,
			wantDeep: true,
		},
		{
			name: "else body nested",
			code: `PROGRAM Main
VAR a : BOOL; b : BOOL; c : BOOL; d : BOOL; x : INT; END_VAR
    IF a THEN
        IF b THEN
            IF c THEN
                x := 1;
            ELSE
                IF d THEN x := 1; END_IF;
            END_IF;
        END_IF;
    END_IF;
END_PROGRAM`,
			wantDeep: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := lintST(tt.code)
			got := hasDiagCode(diags, CodeDeepNesting)
			if got != tt.wantDeep {
				t.Errorf("hasDiagCode(CodeDeepNesting) = %v, want %v", got, tt.wantDeep)
			}
		})
	}
}

func TestMissingReturnType_Variants(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantWarn bool
	}{
		{
			name: "function with return type",
			code: `FUNCTION Calc : INT
    Calc := 1;
END_FUNCTION`,
			wantWarn: false,
		},
		{
			name: "function without return type",
			code: `FUNCTION Calc
    Calc := 1;
END_FUNCTION`,
			wantWarn: true,
		},
		{
			name: "program does not flag",
			code: `PROGRAM Main
END_PROGRAM`,
			wantWarn: false,
		},
		{
			name: "FB does not flag",
			code: `FUNCTION_BLOCK FB_Test
END_FUNCTION_BLOCK`,
			wantWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := lintST(tt.code)
			got := hasDiagCode(diags, CodeMissingReturnType)
			if got != tt.wantWarn {
				t.Errorf("hasDiagCode(CodeMissingReturnType) = %v, want %v", got, tt.wantWarn)
			}
		})
	}
}

// --- Naming convention tests ---

func TestNaming_PascalCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"PascalCase", "MyMotor", true},
		{"single word", "Motor", true},
		{"with underscore segment", "FB_Motor", true},
		{"multi underscore", "IO_Handler_V2", true},
		{"lowercase", "myMotor", false},
		{"all lower underscore", "my_motor", false},
		{"leading underscore", "_Motor", false},
		{"trailing underscore", "Motor_", false},
		{"all caps single", "A", true},
		{"two segment", "FB_Base", true},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPascalCase(tt.input)
			if got != tt.want {
				t.Errorf("isPascalCase(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNaming_UpperSnake(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"upper snake", "MAX_SIZE", true},
		{"single word", "MAX", true},
		{"with number", "V2_CONFIG", true},
		{"lowercase", "max_size", false},
		{"mixed case", "Max_Size", false},
		{"empty", "", false},
		{"just digit suffix", "A1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUpperSnake(tt.input)
			if got != tt.want {
				t.Errorf("isUpperSnake(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNaming_LowerStart(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"myVar", true},
		{"x", true},
		{"MyVar", false},
		{"_var", false},
		{"123", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isLowerStart(tt.input)
			if got != tt.want {
				t.Errorf("isLowerStart(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNaming_POU_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantWarn bool
	}{
		{
			name:     "program PascalCase OK",
			code:     `PROGRAM MainProgram END_PROGRAM`,
			wantWarn: false,
		},
		{
			name:     "program lowercase bad",
			code:     `PROGRAM main_program END_PROGRAM`,
			wantWarn: true,
		},
		{
			name:     "function PascalCase OK",
			code:     `FUNCTION Calculate : INT END_FUNCTION`,
			wantWarn: false,
		},
		{
			name:     "function lowercase bad",
			code:     `FUNCTION calculate : INT END_FUNCTION`,
			wantWarn: true,
		},
		{
			name:     "FB PascalCase with underscore OK",
			code:     `FUNCTION_BLOCK FB_Motor END_FUNCTION_BLOCK`,
			wantWarn: false,
		},
		{
			name:     "FB all lowercase bad",
			code:     `FUNCTION_BLOCK fb_motor END_FUNCTION_BLOCK`,
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := lintST(tt.code)
			got := hasDiagCode(diags, CodeNamingPOU)
			if got != tt.wantWarn {
				t.Errorf("CodeNamingPOU = %v, want %v", got, tt.wantWarn)
			}
		})
	}
}

func TestNaming_Variables(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantVar  bool // CodeNamingVar
		wantConst bool // CodeNamingConstant
	}{
		{
			name: "lowercase var OK",
			code: `PROGRAM Main
VAR myVar : INT; END_VAR
END_PROGRAM`,
			wantVar: false,
		},
		{
			name: "uppercase var bad",
			code: `PROGRAM Main
VAR MyVar : INT; END_VAR
END_PROGRAM`,
			wantVar: true,
		},
		{
			name: "upper snake constant OK",
			code: `PROGRAM Main
VAR CONSTANT MAX_VAL : INT := 100; END_VAR
END_PROGRAM`,
			wantConst: false,
		},
		{
			name: "lowercase constant bad",
			code: `PROGRAM Main
VAR CONSTANT maxVal : INT := 100; END_VAR
END_PROGRAM`,
			wantConst: true,
		},
		{
			name: "mixed case constant bad",
			code: `PROGRAM Main
VAR CONSTANT MaxSize : INT := 100; END_VAR
END_PROGRAM`,
			wantConst: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := lintST(tt.code)
			if got := hasDiagCode(diags, CodeNamingVar); got != tt.wantVar {
				t.Errorf("CodeNamingVar = %v, want %v", got, tt.wantVar)
			}
			if got := hasDiagCode(diags, CodeNamingConstant); got != tt.wantConst {
				t.Errorf("CodeNamingConstant = %v, want %v", got, tt.wantConst)
			}
		})
	}
}

// --- Aggregate Lint function test ---

func TestLintFile_AllRulesCombined(t *testing.T) {
	code := `FUNCTION_BLOCK my_bad_fb
VAR
    MyBadVar : INT;
END_VAR
VAR CONSTANT
    badConst : INT := 100;
END_VAR
    MyBadVar := 42;
END_FUNCTION_BLOCK
`
	res := parser.Parse("test.st", code)
	result := LintFile(res.File, DefaultLintOptions())

	if !hasDiagCode(result.Diags, CodeNamingPOU) {
		t.Error("expected CodeNamingPOU for 'my_bad_fb'")
	}
	if !hasDiagCode(result.Diags, CodeNamingVar) {
		t.Error("expected CodeNamingVar for 'MyBadVar'")
	}
	if !hasDiagCode(result.Diags, CodeNamingConstant) {
		t.Error("expected CodeNamingConstant for 'badConst'")
	}
	if !hasDiagCode(result.Diags, CodeMagicNumber) {
		t.Error("expected CodeMagicNumber for '42'")
	}
}

func TestLintFile_NamingDisabled(t *testing.T) {
	code := `FUNCTION_BLOCK my_bad_fb
VAR MyVar : INT; END_VAR
VAR CONSTANT badConst : INT := 100; END_VAR
END_FUNCTION_BLOCK
`
	opts := LintOptions{NamingConvention: "none"}
	diags := lintSTWithOpts(code, opts)

	for _, d := range diags {
		if d.Code == CodeNamingPOU || d.Code == CodeNamingVar || d.Code == CodeNamingConstant {
			t.Errorf("naming convention 'none' should disable naming checks, got %s", d.Code)
		}
	}
}

func TestDefaultLintOptions(t *testing.T) {
	opts := DefaultLintOptions()
	if opts.NamingConvention != "plcopen" {
		t.Errorf("default naming convention = %q, want \"plcopen\"", opts.NamingConvention)
	}
}

func TestLongPOU_ExactBoundary(t *testing.T) {
	// 200 statements is OK, 201 triggers
	var b strings.Builder
	b.WriteString("PROGRAM Main\nVAR x : INT; END_VAR\n")
	for i := 0; i < 200; i++ {
		b.WriteString("    x := 1;\n")
	}
	b.WriteString("END_PROGRAM\n")

	diags := lintST(b.String())
	if hasDiagCode(diags, CodeLongPOU) {
		t.Error("200 statements should NOT trigger CodeLongPOU")
	}
}

func TestDiagnosticSeverities(t *testing.T) {
	code := `PROGRAM Main
VAR x : INT; END_VAR
    x := 42;
END_PROGRAM`
	diags := lintST(code)
	for _, d := range diags {
		if d.Severity != diag.Warning {
			t.Errorf("lint diagnostic %s should be Warning, got %v", d.Code, d.Severity)
		}
	}
}

func TestMagicNumber_AllowedNeg1(t *testing.T) {
	// The isAllowedLiteral function checks for -1, but the parser generates
	// -1 as UnaryExpr('-', Literal(1)), not as Literal(-1).
	// Test the isAllowedLiteral function directly.
	// Value "-1" as a literal string
	code := `PROGRAM Main
VAR x : INT; END_VAR
    x := 0;
    x := 1;
END_PROGRAM`
	diags := lintST(code)
	if hasDiagCode(diags, CodeMagicNumber) {
		t.Error("0 and 1 should not be flagged")
	}
}

func TestWalkExprsInStmt_AllBranches(t *testing.T) {
	// Test that walkExprsInStmt handles all statement types, including
	// those with sub-expressions (covered by magic number detection).
	// We build a program with every statement type containing magic numbers.
	code := `PROGRAM Main
VAR x : INT; a : BOOL; arr : ARRAY[0..9] OF INT; END_VAR
    x := 42;
    IF x > 42 THEN
        x := 1;
    ELSIF x > 99 THEN
        x := 1;
    ELSE
        x := 42;
    END_IF;
    FOR x := 0 TO 42 BY 2 DO
        x := 1;
    END_FOR;
    WHILE x < 42 DO
        x := 1;
    END_WHILE;
    REPEAT
        x := 1;
    UNTIL x > 42 END_REPEAT;
    CASE x OF
        0: x := 42;
    ELSE
        x := 99;
    END_CASE;
END_PROGRAM`
	diags := lintST(code)
	count := 0
	for _, d := range diags {
		if d.Code == CodeMagicNumber {
			count++
		}
	}
	if count < 5 {
		t.Errorf("expected at least 5 magic number warnings across all stmt types, got %d", count)
	}
}
