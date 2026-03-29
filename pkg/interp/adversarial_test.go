package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
)

// helper runs a program through parse + interpret via ScanCycleEngine.
// Panics are test failures; errors are expected and acceptable.
func runProgramAdversarial(t *testing.T, src string) error {
	t.Helper()
	result := parser.Parse("adversarial.st", src)
	if result.File == nil {
		t.Fatal("Parse returned nil File")
	}

	for _, decl := range result.File.Declarations {
		prog, ok := decl.(*ast.ProgramDecl)
		if !ok {
			continue
		}

		engine := NewScanCycleEngine(prog)
		engine.interp.MaxLoopIterations = 10000
		return engine.Tick(100 * time.Millisecond)
	}
	return nil
}

// TestAdversarial_DivisionByZeroInt tests integer division by zero.
func TestAdversarial_DivisionByZeroInt(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; END_VAR
x := 10 / 0;
END_PROGRAM
`)
	if err == nil {
		t.Log("Division by zero did not return error (may be acceptable)")
	}
}

// TestAdversarial_DivisionByZeroReal tests real division by zero.
func TestAdversarial_DivisionByZeroReal(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : REAL; END_VAR
x := 10.0 / 0.0;
END_PROGRAM
`)
	if err == nil {
		t.Log("Real division by zero did not return error (may produce Inf)")
	}
}

// TestAdversarial_ModByZero tests modulo by zero.
func TestAdversarial_ModByZero(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; END_VAR
x := 10 MOD 0;
END_PROGRAM
`)
	if err == nil {
		t.Log("Mod by zero did not return error")
	}
}

// TestAdversarial_IntegerOverflow tests integer overflow.
func TestAdversarial_IntegerOverflow(t *testing.T) {
	// INT max is 32767 for 16-bit, but stc uses int64 internally
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; END_VAR
x := 9223372036854775807;
x := x + 1;
END_PROGRAM
`)
	_ = err // overflow behavior: wraps or errors, either is fine, just no panic
}

// TestAdversarial_ForStepZero tests FOR loop with step=0.
func TestAdversarial_ForStepZero(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR i : INT; END_VAR
FOR i := 0 TO 10 BY 0 DO
END_FOR;
END_PROGRAM
`)
	if err == nil {
		t.Log("FOR step=0 did not return error")
	}
}

// TestAdversarial_ForNegativeStep tests FOR loop with negative step.
func TestAdversarial_ForNegativeStep(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR i : INT; END_VAR
FOR i := 10 TO 0 BY -1 DO
END_FOR;
END_PROGRAM
`)
	_ = err
}

// TestAdversarial_WhileTrueWithGuard tests WHILE TRUE (relies on MaxLoopIterations guard).
func TestAdversarial_WhileTrueWithGuard(t *testing.T) {
	result := parser.Parse("adversarial.st", `
PROGRAM p
VAR x : INT; END_VAR
WHILE TRUE DO
  x := x + 1;
END_WHILE;
END_PROGRAM
`)
	if result.File == nil {
		t.Fatal("Parse returned nil File")
	}
	for _, decl := range result.File.Declarations {
		prog, ok := decl.(*ast.ProgramDecl)
		if !ok {
			continue
		}
		engine := NewScanCycleEngine(prog)
		engine.interp.MaxLoopIterations = 100 // small guard
		_ = engine.Tick(10 * time.Millisecond)
	}
}

// TestAdversarial_RepeatForever tests REPEAT with always-false condition.
func TestAdversarial_RepeatForever(t *testing.T) {
	result := parser.Parse("adversarial.st", `
PROGRAM p
VAR x : INT; END_VAR
REPEAT
  x := x + 1;
UNTIL FALSE;
END_REPEAT;
END_PROGRAM
`)
	if result.File == nil {
		t.Fatal("Parse returned nil File")
	}
	for _, decl := range result.File.Declarations {
		prog, ok := decl.(*ast.ProgramDecl)
		if !ok {
			continue
		}
		engine := NewScanCycleEngine(prog)
		engine.interp.MaxLoopIterations = 100
		_ = engine.Tick(10 * time.Millisecond)
	}
}

// TestAdversarial_UndefinedVariable tests accessing undefined variables.
func TestAdversarial_UndefinedVariable(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; END_VAR
x := y;
END_PROGRAM
`)
	if err == nil {
		t.Log("Accessing undefined variable did not return error")
	}
}

// TestAdversarial_EmptyProgram tests a completely empty program body.
func TestAdversarial_EmptyProgram(t *testing.T) {
	err := runProgramAdversarial(t, `PROGRAM p END_PROGRAM`)
	if err != nil {
		t.Fatalf("Empty program returned unexpected error: %v", err)
	}
}

// TestAdversarial_EmptyVarBlock tests a program with empty VAR block.
func TestAdversarial_EmptyVarBlock(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR END_VAR
END_PROGRAM
`)
	if err != nil {
		t.Fatalf("Empty VAR block returned unexpected error: %v", err)
	}
}

// TestAdversarial_NestedForLoops tests deeply nested FOR loops.
func TestAdversarial_NestedForLoops(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR i : INT; j : INT; k : INT; x : INT; END_VAR
FOR i := 0 TO 5 DO
  FOR j := 0 TO 5 DO
    FOR k := 0 TO 5 DO
      x := x + 1;
    END_FOR;
  END_FOR;
END_FOR;
END_PROGRAM
`)
	if err != nil {
		t.Fatalf("Nested FOR loops returned error: %v", err)
	}
}

// TestAdversarial_StringOperations tests string edge cases.
func TestAdversarial_StringOperations(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR s : STRING; END_VAR
s := '';
s := s + '';
s := 'a' + 'b';
END_PROGRAM
`)
	if err != nil {
		t.Fatalf("String operations returned error: %v", err)
	}
}

// TestAdversarial_BooleanExpressions tests complex boolean expressions.
func TestAdversarial_BooleanExpressions(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR b : BOOL; END_VAR
b := TRUE AND FALSE OR TRUE XOR FALSE;
b := NOT NOT NOT TRUE;
b := (TRUE AND (FALSE OR TRUE)) XOR (NOT FALSE);
END_PROGRAM
`)
	if err != nil {
		t.Fatalf("Boolean expressions returned error: %v", err)
	}
}

// TestAdversarial_CaseWithNoMatch tests CASE with no matching branch and no ELSE.
func TestAdversarial_CaseWithNoMatch(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; y : INT; END_VAR
x := 99;
CASE x OF
  1: y := 1;
  2: y := 2;
  3: y := 3;
END_CASE;
END_PROGRAM
`)
	_ = err // no match, no ELSE: should be fine, just skip
}

// TestAdversarial_NestedCaseInIf tests nested CASE inside IF.
func TestAdversarial_NestedCaseInIf(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; y : INT; END_VAR
x := 1;
IF x > 0 THEN
  CASE x OF
    1: y := 10;
  ELSE
    y := 0;
  END_CASE;
ELSE
  y := -1;
END_IF;
END_PROGRAM
`)
	if err != nil {
		t.Fatalf("Nested CASE in IF returned error: %v", err)
	}
}

// TestAdversarial_ReturnInLoop tests RETURN inside a loop.
func TestAdversarial_ReturnInLoop(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR i : INT; END_VAR
FOR i := 0 TO 100 DO
  IF i > 5 THEN
    RETURN;
  END_IF;
END_FOR;
END_PROGRAM
`)
	_ = err // ErrReturn should be swallowed by Tick
}

// TestAdversarial_ExitInNestedLoop tests EXIT in nested loops.
func TestAdversarial_ExitInNestedLoop(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR i : INT; j : INT; END_VAR
FOR i := 0 TO 10 DO
  FOR j := 0 TO 10 DO
    IF j > 2 THEN EXIT; END_IF;
  END_FOR;
END_FOR;
END_PROGRAM
`)
	_ = err
}

// TestAdversarial_ContinueInLoop tests CONTINUE in a loop.
func TestAdversarial_ContinueInLoop(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR i : INT; x : INT; END_VAR
FOR i := 0 TO 10 DO
  IF i MOD 2 = 0 THEN CONTINUE; END_IF;
  x := x + 1;
END_FOR;
END_PROGRAM
`)
	_ = err
}

// TestAdversarial_TimeLiterals tests various time literal formats.
func TestAdversarial_TimeLiterals(t *testing.T) {
	inputs := []string{
		"PROGRAM p\nVAR t1 : TIME; END_VAR\nt1 := T#0s;\nEND_PROGRAM",
		"PROGRAM p\nVAR t1 : TIME; END_VAR\nt1 := T#1h30m;\nEND_PROGRAM",
		"PROGRAM p\nVAR t1 : TIME; END_VAR\nt1 := T#500ms;\nEND_PROGRAM",
		"PROGRAM p\nVAR t1 : TIME; END_VAR\nt1 := TIME#1d2h3m4s5ms;\nEND_PROGRAM",
	}
	for _, input := range inputs {
		result := parser.Parse("time.st", input)
		if result.File == nil {
			t.Fatal("Parse returned nil File")
		}
		for _, decl := range result.File.Declarations {
			prog, ok := decl.(*ast.ProgramDecl)
			if !ok {
				continue
			}
			engine := NewScanCycleEngine(prog)
			err := engine.Tick(100 * time.Millisecond)
			_ = err
		}
	}
}

// TestAdversarial_LargeForRange tests FOR with very large range.
func TestAdversarial_LargeForRange(t *testing.T) {
	result := parser.Parse("adversarial.st", `
PROGRAM p
VAR i : INT; END_VAR
FOR i := 0 TO 999999999 DO
END_FOR;
END_PROGRAM
`)
	if result.File == nil {
		t.Fatal("Parse returned nil File")
	}
	for _, decl := range result.File.Declarations {
		prog, ok := decl.(*ast.ProgramDecl)
		if !ok {
			continue
		}
		engine := NewScanCycleEngine(prog)
		engine.interp.MaxLoopIterations = 100 // guard against actual running
		_ = engine.Tick(10 * time.Millisecond)
	}
}

// TestAdversarial_MultipleProgramDecls tests multiple PROGRAM declarations.
func TestAdversarial_MultipleProgramDecls(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p1
VAR x : INT; END_VAR
x := 1;
END_PROGRAM

PROGRAM p2
VAR y : INT; END_VAR
y := 2;
END_PROGRAM
`)
	_ = err
}

// TestAdversarial_AllOperators tests all arithmetic and comparison operators.
func TestAdversarial_AllOperators(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR a : INT; b : INT; c : BOOL; r : REAL; END_VAR
a := 10;
b := 3;
a := a + b;
a := a - b;
a := a * b;
a := a / b;
a := a MOD b;
c := a = b;
c := a <> b;
c := a < b;
c := a <= b;
c := a > b;
c := a >= b;
r := 1.5;
r := r + 2.5;
r := r - 1.0;
r := r * 2.0;
r := r / 3.0;
c := r = 2.0;
c := r <> 2.0;
c := r < 5.0;
c := r <= 5.0;
c := r > 0.0;
c := r >= 0.0;
END_PROGRAM
`)
	if err != nil {
		t.Fatalf("All operators test returned error: %v", err)
	}
}

// TestAdversarial_MixedTypeArithmetic tests mixed int/real arithmetic.
func TestAdversarial_MixedTypeArithmetic(t *testing.T) {
	// This may or may not be supported. Either way: no panic.
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; r : REAL; END_VAR
x := 10;
r := 3.14;
END_PROGRAM
`)
	_ = err
}

// TestAdversarial_NegativeValues tests negative value operations.
func TestAdversarial_NegativeValues(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; y : INT; END_VAR
x := -42;
y := -x;
x := x * -1;
x := -(-(-x));
END_PROGRAM
`)
	_ = err
}

// TestAdversarial_ArrayIndexOutOfBounds tests array index out of bounds.
func TestAdversarial_ArrayIndexOutOfBounds(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR arr : ARRAY[0..9] OF INT; x : INT; END_VAR
x := arr[100];
END_PROGRAM
`)
	_ = err // should error, not panic
}

// TestAdversarial_ArrayNegativeIndex tests array with negative index.
func TestAdversarial_ArrayNegativeIndex(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR arr : ARRAY[0..9] OF INT; x : INT; END_VAR
x := arr[-1];
END_PROGRAM
`)
	_ = err
}

// TestAdversarial_EmptyStatementsBlock tests empty statement blocks.
func TestAdversarial_EmptyStatementsBlock(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; END_VAR
IF TRUE THEN
END_IF;
FOR x := 0 TO 0 DO
END_FOR;
WHILE FALSE DO
END_WHILE;
END_PROGRAM
`)
	if err != nil {
		t.Fatalf("Empty blocks returned error: %v", err)
	}
}

// TestAdversarial_ScanCycleMultipleTicks tests multiple scan cycle ticks.
func TestAdversarial_ScanCycleMultipleTicks(t *testing.T) {
	result := parser.Parse("adversarial.st", `
PROGRAM p
VAR_INPUT enable : BOOL; END_VAR
VAR_OUTPUT count : INT; END_VAR
VAR x : INT; END_VAR
IF enable THEN
  count := count + 1;
END_IF;
END_PROGRAM
`)
	if result.File == nil {
		t.Fatal("Parse returned nil File")
	}
	for _, decl := range result.File.Declarations {
		prog, ok := decl.(*ast.ProgramDecl)
		if !ok {
			continue
		}
		engine := NewScanCycleEngine(prog)
		engine.SetInput("enable", BoolValue(true))
		for i := 0; i < 100; i++ {
			err := engine.Tick(10 * time.Millisecond)
			if err != nil {
				t.Fatalf("Tick %d returned error: %v", i, err)
			}
		}
		out := engine.GetOutput("count")
		if out.Int != 100 {
			t.Fatalf("Expected count=100, got %d", out.Int)
		}
	}
}

// TestAdversarial_UnaryNot tests NOT on non-boolean values.
func TestAdversarial_UnaryNot(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; b : BOOL; END_VAR
x := 42;
b := NOT x;
END_PROGRAM
`)
	_ = err // might coerce or error
}

// TestAdversarial_ComparisonOnStrings tests comparison on strings.
func TestAdversarial_ComparisonOnStrings(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR s1 : STRING; s2 : STRING; b : BOOL; END_VAR
s1 := 'abc';
s2 := 'def';
b := s1 = s2;
b := s1 <> s2;
b := s1 < s2;
b := s1 > s2;
END_PROGRAM
`)
	_ = err
}

// TestAdversarial_PowerOperator tests the ** power operator.
func TestAdversarial_PowerOperator(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : REAL; END_VAR
x := 2.0 ** 10.0;
END_PROGRAM
`)
	_ = err
}

// TestAdversarial_ElsifChain tests a long ELSIF chain.
func TestAdversarial_ElsifChain(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; y : INT; END_VAR
x := 50;
IF x = 1 THEN y := 1;
ELSIF x = 2 THEN y := 2;
ELSIF x = 3 THEN y := 3;
ELSIF x = 4 THEN y := 4;
ELSIF x = 5 THEN y := 5;
ELSIF x = 10 THEN y := 10;
ELSIF x = 20 THEN y := 20;
ELSIF x = 30 THEN y := 30;
ELSIF x = 40 THEN y := 40;
ELSIF x = 50 THEN y := 50;
ELSE y := 0;
END_IF;
END_PROGRAM
`)
	if err != nil {
		t.Fatalf("ELSIF chain returned error: %v", err)
	}
}

// TestAdversarial_AssignToSelf tests self-referencing assignment.
func TestAdversarial_AssignToSelf(t *testing.T) {
	err := runProgramAdversarial(t, `
PROGRAM p
VAR x : INT; END_VAR
x := x;
x := x + x;
x := x * x;
END_PROGRAM
`)
	if err != nil {
		t.Fatalf("Self-assignment returned error: %v", err)
	}
}
