package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
)

// FuzzInterpret parses then interprets random ST code.
// The interpreter must never panic — only return errors.
func FuzzInterpret(f *testing.F) {
	seeds := []string{
		// Basic programs
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 1;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 1 + 2 * 3;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : BOOL; END_VAR\nx := TRUE AND FALSE;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : REAL; END_VAR\nx := 3.14;\nEND_PROGRAM",
		"PROGRAM p\nVAR s : STRING; END_VAR\ns := 'hello';\nEND_PROGRAM",
		// Control flow
		"PROGRAM p\nVAR x : INT; END_VAR\nIF TRUE THEN x := 1; ELSE x := 2; END_IF;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : INT; END_VAR\nFOR x := 0 TO 10 DO END_FOR;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 0; WHILE x < 5 DO x := x + 1; END_WHILE;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 0; REPEAT x := x + 1; UNTIL x > 5; END_REPEAT;\nEND_PROGRAM",
		// Division
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 10 / 2;\nEND_PROGRAM",
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 10 MOD 3;\nEND_PROGRAM",
		// Nested IF
		"PROGRAM p\nVAR x : INT; END_VAR\nIF TRUE THEN IF TRUE THEN x := 1; END_IF; END_IF;\nEND_PROGRAM",
		// Empty
		"PROGRAM p END_PROGRAM",
		// FOR with BY
		"PROGRAM p\nVAR i : INT; END_VAR\nFOR i := 0 TO 100 BY 10 DO END_FOR;\nEND_PROGRAM",
		// CASE
		"PROGRAM p\nVAR x : INT; y : INT; END_VAR\nx := 1;\nCASE x OF 1: y := 10; 2: y := 20; ELSE y := 0; END_CASE;\nEND_PROGRAM",
		// Division by zero
		"PROGRAM p\nVAR x : INT; END_VAR\nx := 1 / 0;\nEND_PROGRAM",
		// Garbage
		"",
		"PROGRAM p\nEND_PROGRAM",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result := parser.Parse("fuzz.st", input)
		if result.File == nil {
			return
		}

		interp := New()
		interp.MaxLoopIterations = 1000 // tight limit for fuzzing

		for _, decl := range result.File.Declarations {
			prog, ok := decl.(*ast.ProgramDecl)
			if !ok {
				continue
			}

			engine := NewScanCycleEngine(prog)
			engine.interp.MaxLoopIterations = 1000
			// Errors are fine, panics are bugs
			_ = engine.Tick(100 * time.Millisecond)
		}
	})
}
