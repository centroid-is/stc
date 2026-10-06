package scenario

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const liveSrc = `
[[step]]
cycle = 0
ramp = {path="p", from=0, to=1, over="30ms"}
expect = {path="p", value=1.0, within=5}
[[step]]
cycle = 2
set = {path="a", value=1}
expect = {path="a", value=1}
[[step]]
cycle = 4
set = {path="b", value=2}
`

// TestLiveMatchesRun drives a Live from a caller-owned loop and checks it
// produces the same report and target log as Run.
func TestLiveMatchesRun(t *testing.T) {
	sc := mustParse(t, liveSrc)
	f1 := newFake()
	want, err := NewExecutor(sc, f1).Run(6)
	if err != nil {
		t.Fatal(err)
	}
	f2 := newFake()
	l, err := NewExecutor(sc, f2).Start(6)
	if err != nil {
		t.Fatal(err)
	}
	for !l.Done() {
		l.BeforeTick()
		l.BeforeTick() // repeated hooks are no-ops
		if err := f2.Tick(); err != nil {
			t.Fatal(err)
		}
		l.AfterTick()
		l.AfterTick()
	}
	l.BeforeTick() // after Done: no-op
	got := l.Finish(nil)
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(f1.log, f2.log) {
		t.Fatalf("live differs from Run:\n%+v\n%+v\n%s\n--\n%s", got, want, strings.Join(f2.log, "\n"), strings.Join(f1.log, "\n"))
	}
	if l.Cycle() != 6 || !got.Passed {
		t.Fatalf("cycle %d report %+v", l.Cycle(), got)
	}
}

// TestLiveFinishEarly stops a run before its length, as a server shut down
// early does: pending expects fail and unfired steps warn.
func TestLiveFinishEarly(t *testing.T) {
	sc := mustParse(t, liveSrc)
	f := newFake()
	l, err := NewExecutor(sc, f).Start(0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		l.BeforeTick()
		_ = f.Tick()
		l.AfterTick()
	}
	rep := l.Finish(nil)
	if rep.Passed || rep.Cycles != 2 || rep.Failed() != 1 {
		t.Fatalf("report: %+v", rep)
	}
	var scn010 int
	for _, d := range rep.Diagnostics {
		if d.Code == "SCN010" {
			scn010++
		}
	}
	if scn010 != 2 || !strings.Contains(rep.Assertions[0].Message, "run ended at cycle 2") {
		t.Fatalf("diagnostics %v assertions %+v", rep.Diagnostics, rep.Assertions)
	}
}

func TestLivePrepareFailed(t *testing.T) {
	sc := mustParse(t, "[[step]]\ncycle = 0\nset = {path=\"nope\", value=1}\n")
	f := newFake()
	f.pathFn = func(p string) error { return ErrUnknownPath }
	f.checkFn = func(a Action) error { return ErrUnknownPath }
	l, err := NewExecutor(sc, f).Start(3)
	if !errors.Is(err, ErrPrepareFailed) || !l.Done() {
		t.Fatalf("err %v done %v", err, l.Done())
	}
	l.BeforeTick()
	l.AfterTick()
	rep := l.Finish(nil)
	if rep.Passed || len(rep.Diagnostics) == 0 || len(f.log) != 0 {
		t.Fatalf("report %+v log %v", rep, f.log)
	}
}
