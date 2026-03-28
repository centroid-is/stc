package incremental

import (
	"testing"
)

func TestDepGraphAddFileAndDependents(t *testing.T) {
	g := NewDepGraph()

	// motor.st declares Motor, references TON and BOOL
	g.AddFile("motor.st", []string{"Motor"}, []string{"TON", "BOOL"})

	// timer.st declares TON
	g.AddFile("timer.st", []string{"TON"}, nil)

	// main.st declares Main, references Motor
	g.AddFile("main.st", []string{"Main"}, []string{"Motor"})

	// Dependents of motor.st: main.st references Motor (declared in motor.st)
	deps := g.Dependents("motor.st")
	if len(deps) != 1 || deps[0] != "main.st" {
		t.Fatalf("Dependents(motor.st) = %v, want [main.st]", deps)
	}

	// Dependents of timer.st: motor.st references TON (declared in timer.st)
	deps = g.Dependents("timer.st")
	if len(deps) != 1 || deps[0] != "motor.st" {
		t.Fatalf("Dependents(timer.st) = %v, want [motor.st]", deps)
	}

	// Dependents of main.st: nobody references Main
	deps = g.Dependents("main.st")
	if len(deps) != 0 {
		t.Fatalf("Dependents(main.st) = %v, want []", deps)
	}
}

func TestDepGraphDependentsCaseInsensitive(t *testing.T) {
	g := NewDepGraph()
	g.AddFile("a.st", []string{"MyFB"}, nil)
	g.AddFile("b.st", nil, []string{"myfb"}) // lowercase ref

	deps := g.Dependents("a.st")
	if len(deps) != 1 || deps[0] != "b.st" {
		t.Fatalf("Dependents(a.st) = %v, want [b.st]", deps)
	}
}

func TestDepGraphAllDirtyTransitiveClosure(t *testing.T) {
	g := NewDepGraph()
	g.AddFile("base.st", []string{"Base"}, nil)
	g.AddFile("mid.st", []string{"Mid"}, []string{"Base"})
	g.AddFile("top.st", []string{"Top"}, []string{"Mid"})

	dirty := g.AllDirty([]string{"base.st"})

	// Should include base.st, mid.st (refs Base), top.st (refs Mid)
	expected := map[string]bool{"base.st": true, "mid.st": true, "top.st": true}
	if len(dirty) != len(expected) {
		t.Fatalf("AllDirty = %v, want %v", dirty, expected)
	}
	for _, f := range dirty {
		if !expected[f] {
			t.Fatalf("unexpected file in AllDirty: %s", f)
		}
	}
}

func TestDepGraphAddFileReplace(t *testing.T) {
	g := NewDepGraph()
	g.AddFile("a.st", []string{"A"}, nil)
	g.AddFile("b.st", nil, []string{"A"})

	// Replace a.st: no longer declares A
	g.AddFile("a.st", []string{"B"}, nil)

	// b.st still references A, but A is no longer declared anywhere
	deps := g.Dependents("a.st")
	if len(deps) != 0 {
		t.Fatalf("After replace, Dependents(a.st) = %v, want []", deps)
	}
}

func TestDepGraphRemoveFile(t *testing.T) {
	g := NewDepGraph()
	g.AddFile("a.st", []string{"A"}, nil)
	g.AddFile("b.st", nil, []string{"A"})

	g.RemoveFile("a.st")

	deps := g.Dependents("a.st")
	if len(deps) != 0 {
		t.Fatalf("After remove, Dependents(a.st) = %v, want []", deps)
	}
}

func TestDepGraphAllDirtySorted(t *testing.T) {
	g := NewDepGraph()
	g.AddFile("c.st", []string{"C"}, nil)
	g.AddFile("b.st", []string{"B"}, []string{"C"})
	g.AddFile("a.st", []string{"A"}, []string{"C"})

	dirty := g.AllDirty([]string{"c.st"})

	// Result should be sorted
	for i := 1; i < len(dirty); i++ {
		if dirty[i] < dirty[i-1] {
			t.Fatalf("AllDirty result not sorted: %v", dirty)
		}
	}
}
