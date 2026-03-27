package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
)

// findProgram extracts the first ProgramDecl from parsed source.
func findProgram(t *testing.T, src string) *ast.ProgramDecl {
	t.Helper()
	result := parser.Parse("test.st", src)
	for _, d := range result.Diags {
		if d.Severity == 0 { // error
			t.Logf("parse diagnostic: %s", d.Message)
		}
	}
	for _, decl := range result.File.Declarations {
		if prog, ok := decl.(*ast.ProgramDecl); ok {
			return prog
		}
	}
	t.Fatalf("no PROGRAM declaration found in source")
	return nil
}

func TestIntegration_SimpleArithmetic(t *testing.T) {
	src := `
PROGRAM Main
VAR_INPUT x : INT; END_VAR
VAR_OUTPUT y : INT; END_VAR
VAR z : INT; END_VAR
  z := x * 2;
  y := z + 1;
END_PROGRAM
`
	prog := findProgram(t, src)
	engine := NewScanCycleEngine(prog)

	engine.SetInput("x", IntValue(5))
	err := engine.Tick(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	y := engine.GetOutput("y")
	if y.Int != 11 {
		t.Fatalf("expected y=11, got %d", y.Int)
	}
}

func TestIntegration_TONTimer(t *testing.T) {
	src := `
PROGRAM TimerTest
VAR_INPUT StartBtn : BOOL; END_VAR
VAR_OUTPUT MotorRunning : BOOL; END_VAR
VAR myTimer : TON; END_VAR
  myTimer(IN := StartBtn, PT := T#500ms);
  MotorRunning := myTimer.Q;
END_PROGRAM
`
	prog := findProgram(t, src)
	engine := NewScanCycleEngine(prog)

	engine.SetInput("StartBtn", BoolValue(true))

	// 4 ticks of 100ms: should not fire yet
	for i := 0; i < 4; i++ {
		err := engine.Tick(100 * time.Millisecond)
		if err != nil {
			t.Fatalf("Tick %d failed: %v", i+1, err)
		}
		out := engine.GetOutput("MotorRunning")
		if out.Bool {
			t.Fatalf("MotorRunning should be FALSE at tick %d (400ms < 500ms)", i+1)
		}
	}

	// 5th tick: 500ms total, should fire
	err := engine.Tick(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("Tick 5 failed: %v", err)
	}
	out := engine.GetOutput("MotorRunning")
	if !out.Bool {
		t.Fatal("MotorRunning should be TRUE after 500ms")
	}
}

func TestIntegration_CTUCounter(t *testing.T) {
	src := `
PROGRAM CounterTest
VAR_INPUT Pulse : BOOL; END_VAR
VAR_OUTPUT Done : BOOL; END_VAR
VAR cnt : CTU; END_VAR
  cnt(CU := Pulse, R := FALSE, PV := 3);
  Done := cnt.Q;
END_PROGRAM
`
	prog := findProgram(t, src)
	engine := NewScanCycleEngine(prog)

	// Toggle Pulse TRUE/FALSE 3 times
	for i := 0; i < 3; i++ {
		engine.SetInput("Pulse", BoolValue(true))
		if err := engine.Tick(100 * time.Millisecond); err != nil {
			t.Fatalf("Tick failed: %v", err)
		}
		engine.SetInput("Pulse", BoolValue(false))
		if err := engine.Tick(100 * time.Millisecond); err != nil {
			t.Fatalf("Tick failed: %v", err)
		}
	}

	done := engine.GetOutput("Done")
	if !done.Bool {
		t.Fatal("Done should be TRUE after 3 rising edges with PV=3")
	}
}

func TestIntegration_StringFunction(t *testing.T) {
	src := `
PROGRAM StringTest
VAR_OUTPUT result : INT; END_VAR
  result := LEN('hello');
END_PROGRAM
`
	prog := findProgram(t, src)
	engine := NewScanCycleEngine(prog)

	err := engine.Tick(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	result := engine.GetOutput("result")
	if result.Int != 5 {
		t.Fatalf("expected result=5, got %d", result.Int)
	}
}

func TestIntegration_ForLoop(t *testing.T) {
	src := `
PROGRAM LoopTest
VAR_OUTPUT total : INT; END_VAR
VAR i : INT; END_VAR
  total := 0;
  FOR i := 1 TO 10 BY 1 DO
    total := total + i;
  END_FOR;
END_PROGRAM
`
	prog := findProgram(t, src)
	engine := NewScanCycleEngine(prog)

	err := engine.Tick(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	total := engine.GetOutput("total")
	if total.Int != 55 {
		t.Fatalf("expected total=55, got %d", total.Int)
	}
}
