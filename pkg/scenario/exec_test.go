package scenario

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fake is a deterministic Target that logs every call.
type fake struct {
	base    time.Duration
	k       int
	log     []string
	vals    map[string]any
	checkFn func(a Action) error
	pathFn  func(p string) error
	applyFn func(a Action) error
	tickErr int // Tick index that fails, -1 for none
	readFn  func(p string, k int) (any, error)
}

func newFake() *fake {
	return &fake{base: 10 * time.Millisecond, vals: map[string]any{}, tickErr: -1}
}

func (f *fake) BaseTick() time.Duration { return f.base }
func (f *fake) Clock() time.Duration    { return time.Duration(f.k) * f.base }
func (f *fake) Tick() error {
	if f.k == f.tickErr {
		return errors.New("boom")
	}
	f.log = append(f.log, fmt.Sprintf("tick %d", f.k))
	f.k++
	return nil
}
func (f *fake) Check(a Action) error {
	if f.checkFn != nil {
		return f.checkFn(a)
	}
	return nil
}
func (f *fake) CheckPath(p string) error {
	if f.pathFn != nil {
		return f.pathFn(p)
	}
	return nil
}
func (f *fake) Apply(a Action) error {
	f.log = append(f.log, fmt.Sprintf("apply %s %s%s=%v @k=%d", a.Kind, a.Path, a.Slave, a.Value, f.k))
	if f.applyFn != nil {
		if err := f.applyFn(a); err != nil {
			return err
		}
	}
	if a.Path != "" {
		f.vals[a.Path] = a.Value
	}
	return nil
}
func (f *fake) SetNumber(p string, v float64) error {
	f.log = append(f.log, fmt.Sprintf("setnum %s=%g @k=%d", p, v, f.k))
	if p == "bad" {
		return errors.New("not numeric")
	}
	f.vals[p] = v
	return nil
}
func (f *fake) Read(p string) (any, error) {
	if f.readFn != nil {
		return f.readFn(p, f.k-1)
	}
	v, ok := f.vals[p]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownPath, p)
	}
	return v, nil
}

func mustParse(t *testing.T, src string) *Scenario {
	t.Helper()
	sc, ds := Parse([]byte(src), "t.toml")
	if len(ds) != 0 {
		t.Fatalf("parse: %v", ds)
	}
	return sc
}

func indexOf(log []string, s string) int {
	for i, l := range log {
		if l == s {
			return i
		}
	}
	return -1
}

func TestExecTriggerOrder(t *testing.T) {
	sc := mustParse(t, `
[[step]]
cycle = 3
set = {path="a", value=1}
[[step]]
at = "25ms"
set = {path="b", value=2}
[[step]]
cycle = 3
set = {path="c", value=3}
`)
	f := newFake()
	rep, err := NewExecutor(sc, f).Run(5)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"tick 0", "tick 1", "tick 2",
		"apply set a=1 @k=3", "apply set b=2 @k=3", "apply set c=3 @k=3", "tick 3", "tick 4"}
	if !reflect.DeepEqual(f.log, want) {
		t.Fatalf("log:\n%s", strings.Join(f.log, "\n"))
	}
	if !rep.Passed || rep.Cycles != 5 || rep.SimTimeNs != int64(50*time.Millisecond) {
		t.Fatalf("report: %+v", rep)
	}
	if s := rep.Steps[1]; !s.Fired || s.Cycle != 3 || s.AtNs != int64(30*time.Millisecond) || s.Action != "set b = 2" {
		t.Fatalf("step 2: %+v", s)
	}
}

func TestExecPrepareErrors(t *testing.T) {
	sc := mustParse(t, `
[[step]]
cycle = 0
trip = {slave="nope", channel=1}
[[step]]
cycle = 0
set = {path="x", value=1}
expect = {path="y", value=1}
[[step]]
cycle = 0
analog = {slave="s", channel=1, ma=4}
[[step]]
cycle = 0
link = {path="a^b", value=1}
[[step]]
cycle = 0
drive_fault = {slave="d", lft=1}
`)
	f := newFake()
	f.checkFn = func(a Action) error {
		switch a.Kind {
		case ActTrip:
			return fmt.Errorf("%w: nope", ErrUnknownSlave)
		case ActSet:
			return fmt.Errorf("%w: x", ErrUnknownPath)
		case ActAnalog:
			return ErrNoNetwork
		case ActLink:
			return errors.New("bad link")
		case ActDriveFault:
			return errors.New("not an ATV320")
		}
		return nil
	}
	f.pathFn = func(p string) error { return fmt.Errorf("%w: %s", ErrUnknownPath, p) }
	ex := NewExecutor(sc, f)
	ds := ex.Prepare()
	if got := codes(ds); got != "SCN006,SCN007,SCN007,SCN006,SCN007,SCN006" {
		t.Fatalf("codes %s: %v", got, ds)
	}
	if ds[0].Pos.Line != 2 || !strings.Contains(ds[0].Message, "step 1") {
		t.Errorf("pos: %+v", ds[0])
	}
	rep, err := ex.Run(0)
	if !errors.Is(err, ErrPrepareFailed) || rep.Passed || len(f.log) != 0 || len(rep.Diagnostics) != 6 {
		t.Fatalf("run should refuse: %v %+v %v", err, rep, f.log)
	}
}

func TestExecBadBaseTick(t *testing.T) {
	f := newFake()
	f.base = 0
	sc := mustParse(t, "[[step]]\nat=\"5ms\"\nset={path=\"a\",value=1}")
	_, err := NewExecutor(sc, f).Run(1)
	if !errors.Is(err, ErrPrepareFailed) {
		t.Fatal(err)
	}
	if dueTick(&sc.Steps[0], 0) != 0 {
		t.Fatal("due with zero base")
	}
	huge := &Step{AtSet: true, At: time.Duration(1 << 62)}
	if dueTick(huge, time.Nanosecond) != 1<<31-1 {
		t.Fatal("clamp")
	}
}

func TestExecRamp(t *testing.T) {
	sc := mustParse(t, `
[[step]]
cycle = 0
ramp = {path="p", from=0, to=100, over="50ms"}
`)
	f := newFake()
	if _, err := NewExecutor(sc, f).Run(8); err != nil {
		t.Fatal(err)
	}
	var sets []string
	for _, l := range f.log {
		if strings.HasPrefix(l, "setnum") {
			sets = append(sets, l)
		}
	}
	want := []string{"setnum p=0 @k=0", "setnum p=20 @k=1", "setnum p=40 @k=2", "setnum p=60 @k=3", "setnum p=80 @k=4", "setnum p=100 @k=5"}
	if !reflect.DeepEqual(sets, want) {
		t.Fatalf("sets: %v", sets)
	}
	if indexOf(f.log, "setnum p=0 @k=0") > indexOf(f.log, "tick 0") {
		t.Fatal("ramp must apply before tick")
	}
}

func TestExecSlaveRampAndCancel(t *testing.T) {
	sc := mustParse(t, `
[[step]]
cycle = 0
ramp = {slave="S", channel=2, unit="mA", from=4, to=20, over="40ms"}
[[step]]
cycle = 0
ramp = {path="p", from=0, to=10, over="100ms"}
[[step]]
cycle = 2
set = {path="P", value=7}
[[step]]
cycle = 3
analog = {slave="s", channel=2, volts=1}
`)
	f := newFake()
	if _, err := NewExecutor(sc, f).Run(6); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(f.log, "\n")
	for _, w := range []string{"apply analog S=4 @k=0", "apply analog S=8 @k=1", "apply analog S=12 @k=2", "setnum p=1 @k=1", "apply set P=7 @k=2"} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in\n%s", w, got)
		}
	}
	for _, nw := range []string{"setnum p=2 @k=2", "apply analog S=16 @k=3", "apply analog S=20"} {
		if strings.Contains(got, nw) {
			t.Errorf("ramp not cancelled: %q in\n%s", nw, got)
		}
	}
}

func TestExecRampReplaceAndError(t *testing.T) {
	sc := mustParse(t, `
[[step]]
cycle = 0
ramp = {path="p", from=0, to=10, over="100ms"}
[[step]]
cycle = 1
ramp = {path="p", from=50, to=60, over="10ms"}
[[step]]
cycle = 0
ramp = {path="bad", from=0, to=1, over="10ms"}
[[step]]
cycle = 0
ramp = {slave="X", channel=1, unit="V", from=0, to=1, over="10ms"}
`)
	f := newFake()
	f.applyFn = func(a Action) error {
		if a.Slave == "X" {
			return errors.New("no such channel")
		}
		return nil
	}
	rep, err := NewExecutor(sc, f).Run(4)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(f.log, "\n")
	if !strings.Contains(got, "setnum p=50 @k=1") || !strings.Contains(got, "setnum p=60 @k=2") || strings.Contains(got, "setnum p=1 @k=1") {
		t.Fatalf("replace:\n%s", got)
	}
	if strings.Count(got, "setnum bad") != 1 || strings.Count(got, "apply analog X") != 1 {
		t.Fatalf("failed ramps must be removed:\n%s", got)
	}
	if rep.Passed || rep.Steps[2].Error == "" || rep.Steps[3].Error == "" || !hasDiag(rep.Diagnostics, "SCN008", "ramp failed") {
		t.Fatalf("report: %+v", rep)
	}
}

func TestExecExpect(t *testing.T) {
	sc := mustParse(t, `
[[step]]
cycle = 1
expect = {path="late", value=true, within=2}
[[step]]
cycle = 1
expect = {path="never", value=5, within=1}
[[step]]
cycle = 0
expect = {path="real", value=1.5, tol=0.01}
[[step]]
cycle = 0
expect = {path="enum", value="E_Mode.Run"}
[[step]]
cycle = 0
expect = {path="int", value=1}
[[step]]
cycle = 0
expect = {path="missing", value=1}
`)
	f := newFake()
	f.readFn = func(p string, k int) (any, error) {
		switch p {
		case "late":
			return k >= 2, nil
		case "never":
			return int64(4), nil
		case "real":
			return 1.505, nil
		case "enum":
			return EnumName("RUN"), nil
		case "int":
			return 1.0, nil
		}
		return nil, errors.New("gone")
	}
	rep, err := NewExecutor(sc, f).Run(5)
	if err != nil {
		t.Fatal(err)
	}
	a := rep.Assertions
	if len(a) != 6 {
		t.Fatalf("assertions: %+v", a)
	}
	if !a[0].Pass || a[0].Cycle != 2 {
		t.Errorf("late: %+v", a[0])
	}
	if a[1].Pass || a[1].Cycle != 2 || a[1].Actual != int64(4) || !strings.Contains(a[1].Message, "expected 5, got 4") {
		t.Errorf("never: %+v", a[1])
	}
	if !a[2].Pass || !a[3].Pass || !a[4].Pass {
		t.Errorf("real/enum/int: %+v", a[2:5])
	}
	if a[5].Pass || !strings.Contains(a[5].Message, "read failed: gone") {
		t.Errorf("missing: %+v", a[5])
	}
	if rep.Passed || rep.Failed() != 2 || !hasDiag(rep.Diagnostics, "SCN009", "step 2") {
		t.Errorf("report: %+v", rep)
	}
}

func TestExecRunLength(t *testing.T) {
	src := `
[[step]]
at = "45ms"
set = {path="a", value=1}
expect = {path="a", value=2, within=3}
[[step]]
cycle = 2
expect = {path="a", value=1, within=1}
`
	f := newFake()
	ex := NewExecutor(mustParse(t, src), f)
	if ex.Length(0) != 5+3+1 || ex.Length(7) != 7 {
		t.Fatalf("length %d", ex.Length(0))
	}
	rep, err := ex.Run(0)
	if err != nil || rep.Cycles != 9 {
		t.Fatal(err, rep.Cycles)
	}
	if rep.Assertions[0].Pass || rep.Assertions[0].Message != "expected 2, got 1" {
		t.Errorf("a0: %+v", rep.Assertions[0])
	}

	sc := mustParse(t, "[scenario]\ncycles=4\n"+src)
	if NewExecutor(sc, newFake()).Length(0) != 4 {
		t.Fatal("header cycles")
	}
	// Ends early: step 1 never fires (SCN010), step 2 still pending.
	f2 := newFake()
	f2.vals["a"] = int64(0)
	rep, err = NewExecutor(mustParse(t, src), f2).Run(3)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Steps[0].Fired || !hasDiag(rep.Diagnostics, "SCN010", "step 1 never fired") {
		t.Errorf("SCN010: %+v", rep)
	}
	if len(rep.Assertions) != 1 || rep.Assertions[0].Message != "run ended at cycle 3" {
		t.Errorf("pending: %+v", rep.Assertions)
	}
	if rep.Passed {
		t.Error("should fail")
	}
}

func TestExecApplyAndTickErrors(t *testing.T) {
	sc := mustParse(t, `
[[step]]
cycle = 0
trip = {slave="S", channel=1}
[[step]]
cycle = 1
set = {path="a", value=1}
`)
	f := newFake()
	f.applyFn = func(a Action) error {
		if a.Kind == ActTrip {
			return errors.New("no channel")
		}
		return nil
	}
	f.tickErr = 2
	rep, err := NewExecutor(sc, f).Run(5)
	if err == nil || !strings.Contains(err.Error(), "tick 2: boom") {
		t.Fatal(err)
	}
	if rep.Cycles != 2 || rep.Steps[0].Error != "no channel" || !rep.Steps[1].Fired || rep.Passed {
		t.Fatalf("report: %+v", rep)
	}
	if !hasDiag(rep.Diagnostics, "SCN008", "trip failed") {
		t.Fatalf("diags: %v", rep.Diagnostics)
	}
}

func TestExecDeterministic(t *testing.T) {
	sc := mustParse(t, `
[[step]]
cycle = 0
ramp = {path="p", from=0, to=1, over="30ms"}
expect = {path="p", value=1.0, within=5}
[[step]]
at = "15ms"
slave_state = {slave="S", state="op"}
`)
	r1, _ := NewExecutor(sc, newFake()).Run(0)
	r2, _ := NewExecutor(sc, newFake()).Run(0)
	if !reflect.DeepEqual(r1, r2) {
		t.Fatalf("not deterministic:\n%+v\n%+v", r1, r2)
	}
	if !r1.Passed || r1.Assertions[0].Cycle != 3 {
		t.Fatalf("report: %+v", r1)
	}
}

func TestExecEqual(t *testing.T) {
	cases := []struct {
		x, a any
		want bool
	}{
		{true, true, true}, {true, int64(1), false},
		{"Enum#Run", EnumName("run"), true}, {"x", 1, false},
		{"E_Mode.Run", EnumName("E_Mode#RUN"), true}, {"Run", EnumName("Stop"), false},
		// Plain STRING values compare exactly (review 2 HI-02).
		{"file.txt", "other.txt", false}, {"10.0.0.1", "192.168.0.1", false},
		{"ERR#5", "OK#5", false}, {"Run", "RUN", false}, {"a.b#c", "a.b#c", true},
		{"Enum#Run", "run", false}, {"x", EnumName("x"), true}, {"x", true, false},
		{int64(3), int32(3), true}, {int64(3), uint64(4), false},
		{int64(1), float32(1), true}, {1.0, int8(1), true},
		{1.0, int16(1), true}, {1.0, int(1), true}, {1.0, uint8(1), true},
		{1.0, uint16(1), true}, {1.0, uint32(1), true}, {1.0, "1", false},
		{[]int{1}, 1, false},
	}
	for _, c := range cases {
		if got := Equal(c.x, c.a, 1e-6); got != c.want {
			t.Errorf("Equal(%#v, %#v) = %v", c.x, c.a, got)
		}
	}
}

func TestExecDescribe(t *testing.T) {
	steps := []Step{
		{Expect: &Expect{Path: "x"}},
		{Action: &Action{Kind: ActAnalog, Slave: "S", Channel: 1, Value: 4.0, Unit: "mA"}},
		{Action: &Action{Kind: ActTrip, Slave: "S", Channel: 2}},
		{Action: &Action{Kind: ActSlaveState, Slave: "S", State: "op"}},
		{Action: &Action{Kind: ActSlaveState, Slave: "S", StateCode: 17, HasStateCode: true}},
		{Action: &Action{Kind: ActDriveFault, Slave: "D", LFT: 23}},
		{Action: &Action{Kind: ActRamp, Path: "p", From: 0, To: 1, Over: time.Second}},
		{Action: &Action{Kind: ActRamp, Slave: "S", Channel: 1, Unit: "V", To: 10, Over: time.Second}},
		{Action: &Action{Kind: ActSerialPeer, Slave: "E", Script: "baader"}},
		{Action: &Action{Kind: "other"}},
	}
	want := []string{"expect x", "analog S ch 1 = 4 mA", "trip S ch 2", "slave_state S = op", "slave_state S = 17",
		"drive_fault D lft 23", "ramp p 0 -> 1 over 1s", "ramp S ch 1 V 0 -> 10 over 1s", "serial_peer E = baader", "other"}
	for i := range steps {
		if got := Describe(&steps[i]); got != want[i] {
			t.Errorf("%d: %q", i, got)
		}
	}
}
