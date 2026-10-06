package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/parser"
)

// emptyArgProgram builds a PROGRAM that calls a TON with the given call
// statement and mirrors the timer outputs to VAR_OUTPUTs.
func emptyArgProgram(t *testing.T, call string) *ScanCycleEngine {
	t.Helper()
	src := `PROGRAM P
VAR_INPUT b : BOOL; END_VAR
VAR_OUTPUT q : BOOL; et : TIME; e : TIME; sentinel : BOOL := TRUE; END_VAR
VAR t : TON; END_VAR
` + call + `
q := t.Q;
et := t.ET;
END_PROGRAM
`
	r := parser.Parse("empty_arg.st", src)
	if len(r.Diags) > 0 {
		t.Fatalf("parse diagnostics: %v", r.Diags)
	}
	return NewScanCycleEngine(findProgram(t, src))
}

func TestEmptyArg(t *testing.T) {
	t.Run("empty args run like omitted args", func(t *testing.T) {
		withEmpty := emptyArgProgram(t, "t(IN := b, PT := , Q => , ET => );")
		omitted := emptyArgProgram(t, "t(IN := b);")
		inputs := []bool{false, true, true, false, true, true, true, false}
		for i, b := range inputs {
			for _, e := range []*ScanCycleEngine{withEmpty, omitted} {
				e.SetInput("b", BoolValue(b))
				if err := e.Tick(10 * time.Millisecond); err != nil {
					t.Fatalf("tick %d: %v", i, err)
				}
			}
			for _, name := range []string{"q", "et"} {
				got, want := withEmpty.GetOutput(name), omitted.GetOutput(name)
				if got.Bool != want.Bool || got.Time != want.Time {
					t.Fatalf("tick %d: %s differs: empty=%v omitted=%v", i, name, got, want)
				}
			}
		}
	})

	t.Run("empty output is not written, bound output is", func(t *testing.T) {
		e := emptyArgProgram(t, "t(IN := b, PT := T#100MS, Q => , ET => e);")
		e.SetInput("b", BoolValue(true))
		for i := 0; i < 12; i++ {
			if err := e.Tick(10 * time.Millisecond); err != nil {
				t.Fatalf("tick %d: %v", i, err)
			}
		}
		if got := e.GetOutput("e").Time; got != 100*time.Millisecond {
			t.Fatalf("ET => e should write elapsed time, got %v", got)
		}
		if !e.GetOutput("q").Bool {
			t.Fatal("timer should have fired")
		}
		if !e.GetOutput("sentinel").Bool {
			t.Fatal("empty Q => must not write any variable")
		}
	})

	t.Run("no runtime error for empty input", func(t *testing.T) {
		e := emptyArgProgram(t, "t(PT := , IN := b);")
		e.SetInput("b", BoolValue(true))
		if err := e.Tick(10 * time.Millisecond); err != nil {
			t.Fatalf("unexpected runtime error: %v", err)
		}
	})
}
