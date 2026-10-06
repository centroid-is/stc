package scenario

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
)

// Sentinel errors targets wrap so Prepare can classify failures.
var (
	// ErrUnknownSlave marks an unknown, ambiguous or wrong-model slave (SCN006).
	ErrUnknownSlave = errors.New("unknown slave")
	// ErrUnknownPath marks an unknown or unsettable path (SCN007).
	ErrUnknownPath = errors.New("unknown path")
	// ErrNoNetwork marks a slave action without a loaded network (SCN006).
	ErrNoNetwork = errors.New("no --io network loaded")
	// ErrPrepareFailed is returned by Run when validation reported errors.
	ErrPrepareFailed = errors.New("scenario validation failed")
)

// Stepper is the deterministic scan loop a scenario drives. Phase 22's
// interp.Runtime plus an ecat.Network, or Phase 23's Project, satisfy it
// through a Target adapter.
type Stepper interface {
	// BaseTick is the scan period of one Tick.
	BaseTick() time.Duration
	// Clock is the simulated time before the next Tick.
	Clock() time.Duration
	// Tick runs one scan and advances Clock by BaseTick.
	Tick() error
}

// Target is everything the executor needs from a simulated plant.
// Read returns JSON-style values (bool, int64, float64, string).
type Target interface {
	Stepper
	Check(a Action) error
	CheckPath(path string) error
	Apply(a Action) error
	SetNumber(path string, v float64) error
	Read(path string) (any, error)
}

// Executor runs a Scenario against a Target.
type Executor struct {
	s        *Scenario
	t        Target
	prepared bool
	diags    []diag.Diagnostic
}

// NewExecutor creates an executor for s on t.
func NewExecutor(s *Scenario, t Target) *Executor {
	return &Executor{s: s, t: t}
}

func (e *Executor) diag(sev diag.Severity, st *Step, code, format string, args ...any) diag.Diagnostic {
	line := 0
	if st != nil {
		line = st.Line
	}
	return diag.Diagnostic{
		Severity: sev,
		Pos:      source.Pos{Line: line, Col: 1},
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
	}
}

// Prepare validates every step against the target before the first Tick.
func (e *Executor) Prepare() []diag.Diagnostic {
	e.prepared = true
	e.diags = nil
	if e.t.BaseTick() <= 0 {
		e.diags = append(e.diags, e.diag(diag.Error, nil, "SCN005", "target base tick %v must be positive", e.t.BaseTick()))
	}
	for i := range e.s.Steps {
		st := &e.s.Steps[i]
		if a := st.Action; a != nil {
			if err := e.t.Check(*a); err != nil {
				e.diags = append(e.diags, e.diag(diag.Error, st, checkCode(a, err), "step %d: %s: %v", st.Index, a.Kind, err))
			}
		}
		if x := st.Expect; x != nil {
			if err := e.t.CheckPath(x.Path); err != nil {
				e.diags = append(e.diags, e.diag(diag.Error, st, "SCN007", "step %d: expect %s: %v", st.Index, x.Path, err))
			}
		}
	}
	return e.diags
}

func checkCode(a *Action, err error) string {
	switch {
	case errors.Is(err, ErrUnknownPath):
		return "SCN007"
	case errors.Is(err, ErrUnknownSlave), errors.Is(err, ErrNoNetwork):
		return "SCN006"
	case a.Kind == ActSet || a.Kind == ActLink || (a.Kind == ActRamp && a.Path != ""):
		return "SCN007"
	}
	return "SCN006"
}

func hasErrors(ds []diag.Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == diag.Error {
			return true
		}
	}
	return false
}

// dueTick is the Tick a step fires before (D-07).
func dueTick(st *Step, base time.Duration) int {
	if !st.AtSet {
		return st.Cycle
	}
	if base <= 0 {
		return 0
	}
	k := st.At / base
	if st.At%base != 0 {
		k++
	}
	if k > math.MaxInt32 {
		return math.MaxInt32
	}
	return int(k)
}

// Length is the run length for an explicit cycles value (D-09): cycles > 0
// wins, then Scenario.Cycles, then last due Tick + largest within + 1.
func (e *Executor) Length(cycles int) int {
	if cycles > 0 {
		return cycles
	}
	if e.s.Cycles > 0 {
		return e.s.Cycles
	}
	base := e.t.BaseTick()
	last, within := 0, 0
	for i := range e.s.Steps {
		st := &e.s.Steps[i]
		if d := dueTick(st, base); d > last {
			last = d
		}
		if st.Expect != nil && st.Expect.Within > within {
			within = st.Expect.Within
		}
	}
	return last + within + 1
}

type ramp struct {
	key  string
	step *Step
	t0   time.Duration
	a    Action
}

// value is the ramp's linear interpolation at clock now (From at t0, To
// from t0+Over on) and whether it has reached To.
func (r *ramp) value(now time.Duration) (float64, bool) {
	frac := 1.0
	if r.a.Over > 0 {
		frac = math.Min(1, float64(now-r.t0)/float64(r.a.Over))
	}
	return r.a.From + (r.a.To-r.a.From)*frac, frac >= 1
}

// rampSet holds the active ramps of an Executor or a built-in Session; at
// most one per path or slave channel.
type rampSet struct{ list []*ramp }

// cancel drops the ramp on key.
func (s *rampSet) cancel(key string) {
	out := s.list[:0]
	for _, r := range s.list {
		if r.key != key {
			out = append(out, r)
		}
	}
	s.list = out
}

// start replaces any ramp on the same target with a ramp of a from t0.
func (s *rampSet) start(st *Step, a Action, t0 time.Duration) {
	key := rampKey(&a)
	s.cancel(key)
	s.list = append(s.list, &ramp{key: key, step: st, t0: t0, a: a})
}

// override cancels the ramp an explicit set, link or analog action
// replaces.
func (s *rampSet) override(a *Action) {
	if a.Kind == ActSet || a.Kind == ActLink || a.Kind == ActAnalog {
		s.cancel(rampKey(a))
	}
}

// advance writes every ramp's value at t's clock, keeping the unfinished
// ones; a failing ramp is reported through onErr and dropped.
func (s *rampSet) advance(t Target, onErr func(r *ramp, err error)) {
	live := s.list[:0]
	for _, r := range s.list {
		v, done := r.value(t.Clock())
		var err error
		if r.a.Path != "" {
			err = t.SetNumber(r.a.Path, v)
		} else {
			err = t.Apply(Action{Kind: ActAnalog, Slave: r.a.Slave, Channel: r.a.Channel, Unit: r.a.Unit, Value: v})
		}
		if err != nil {
			onErr(r, err)
			continue
		}
		if !done {
			live = append(live, r)
		}
	}
	s.list = live
}

type pending struct {
	step     *Step
	deadline int
	actual   any
	readErr  error
}

func rampKey(a *Action) string {
	if a.Slave != "" && a.Path == "" {
		return fmt.Sprintf("slave:%s#%d", strings.ToLower(a.Slave), a.Channel)
	}
	return "path:" + strings.ToLower(a.Path)
}

// Run executes the scenario for cycles Ticks (cycles <= 0 uses Length).
// A Tick error stops the run and is returned with the partial report.
func (e *Executor) Run(cycles int) (*Report, error) {
	l, err := e.Start(cycles)
	if err != nil {
		return l.rep, err
	}
	var runErr error
	for !l.Done() {
		l.BeforeTick()
		if err := e.t.Tick(); err != nil {
			runErr = fmt.Errorf("tick %d: %w", l.k, err)
			break
		}
		l.AfterTick()
	}
	return l.Finish(runErr), runErr
}

func (e *Executor) fail(rep *Report, w *pending, k int, msg string) AssertionResult {
	x := w.step.Expect
	if msg == "" {
		if w.readErr != nil {
			msg = fmt.Sprintf("read failed: %v", w.readErr)
		} else {
			msg = fmt.Sprintf("expected %v, got %v", x.Value, w.actual)
		}
	}
	rep.Diagnostics = append(rep.Diagnostics, e.diag(diag.Error, w.step, "SCN009", "step %d: expect %s: %s", w.step.Index, x.Path, msg))
	return AssertionResult{Step: w.step.Index, Path: x.Path, Expected: x.Value, Actual: w.actual, Cycle: k, Pass: false, Message: msg}
}

// Describe renders a step's action (or "expect") as one line of text.
func Describe(st *Step) string {
	a := st.Action
	if a == nil {
		return "expect " + st.Expect.Path
	}
	switch a.Kind {
	case ActSet, ActLink:
		return fmt.Sprintf("%s %s = %v", a.Kind, a.Path, a.Value)
	case ActAnalog:
		return fmt.Sprintf("analog %s ch %d = %v %s", a.Slave, a.Channel, a.Value, a.Unit)
	case ActTrip:
		return fmt.Sprintf("trip %s ch %d", a.Slave, a.Channel)
	case ActSlaveState:
		if a.HasStateCode {
			return fmt.Sprintf("slave_state %s = %d", a.Slave, a.StateCode)
		}
		return fmt.Sprintf("slave_state %s = %s", a.Slave, a.State)
	case ActDriveFault:
		return fmt.Sprintf("drive_fault %s lft %d", a.Slave, a.LFT)
	case ActRamp:
		target := a.Path
		if target == "" {
			target = fmt.Sprintf("%s ch %d %s", a.Slave, a.Channel, a.Unit)
		}
		return fmt.Sprintf("ramp %s %v -> %v over %v", target, a.From, a.To, a.Over)
	case ActSerialPeer:
		return fmt.Sprintf("serial_peer %s = %s", a.Slave, a.Script)
	}
	return string(a.Kind)
}

// Equal compares an expected scenario value with an actual target value
// (D-04): BOOL exact, two integers exact, any float within tol, strings
// case-insensitive with an `Enum#` or `Enum.` prefix stripped.
func Equal(expected, actual any, tol float64) bool {
	switch x := expected.(type) {
	case bool:
		b, ok := actual.(bool)
		return ok && b == x
	case string:
		s, ok := actual.(string)
		return ok && strings.EqualFold(stripEnum(x), stripEnum(s))
	}
	xi, xInt, xok := number(expected)
	ai, aInt, aok := number(actual)
	if !xok || !aok {
		return false
	}
	if xInt && aInt {
		return fmt.Sprint(expected) == fmt.Sprint(actual)
	}
	return math.Abs(xi-ai) <= tol
}

func stripEnum(s string) string {
	if i := strings.LastIndexAny(s, "#."); i >= 0 {
		return s[i+1:]
	}
	return s
}

func number(v any) (float64, bool, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true, true
	case int8:
		return float64(n), true, true
	case int16:
		return float64(n), true, true
	case int32:
		return float64(n), true, true
	case int64:
		return float64(n), true, true
	case uint8:
		return float64(n), true, true
	case uint16:
		return float64(n), true, true
	case uint32:
		return float64(n), true, true
	case uint64:
		return float64(n), true, true
	case float32:
		return float64(n), false, true
	case float64:
		return n, false, true
	}
	return 0, false, false
}
