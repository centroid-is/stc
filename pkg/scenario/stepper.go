package scenario

import (
	"fmt"
	"sort"

	"github.com/centroid-is/stc/pkg/diag"
)

// Live is a scenario run whose scan loop belongs to the caller: Run uses it
// for deterministic runs, and `stc serve --scenario` and `stc-mcp
// --scenario` drive it from their free-running or tool-driven loops. Call
// BeforeTick before each Tick and AfterTick after it, then Finish once.
// Live is not safe for concurrent use; call it from the scan goroutine.
type Live struct {
	e       *Executor
	rep     *Report
	results []StepResult
	order   []int
	due     []int
	ramps   rampSet
	waits   []*pending
	next    int
	k       int // completed Ticks
	n       int // run length
	inTick  bool
}

// Start validates the scenario (if Prepare has not run) and returns a run
// of cycles Ticks (cycles <= 0 uses Length). When validation reports errors
// it returns ErrPrepareFailed and a Live whose Finish returns the report
// with the diagnostics.
func (e *Executor) Start(cycles int) (*Live, error) {
	if !e.prepared {
		e.Prepare()
	}
	l := &Live{e: e, rep: &Report{Name: e.s.Name, Diagnostics: append([]diag.Diagnostic(nil), e.diags...)}}
	if hasErrors(e.diags) {
		l.rep.SimTimeNs = int64(e.t.Clock())
		return l, ErrPrepareFailed
	}
	l.n = e.Length(cycles)
	l.rep.Cycles = l.n
	base := e.t.BaseTick()
	l.results = make([]StepResult, len(e.s.Steps))
	l.order = make([]int, len(e.s.Steps))
	l.due = make([]int, len(e.s.Steps))
	for i := range e.s.Steps {
		st := &e.s.Steps[i]
		l.order[i] = i
		l.due[i] = dueTick(st, base)
		l.results[i] = StepResult{Index: st.Index, Line: st.Line, Cycle: l.due[i], Action: Describe(st)}
	}
	sort.SliceStable(l.order, func(a, b int) bool { return l.due[l.order[a]] < l.due[l.order[b]] })
	return l, nil
}

// Done reports whether the run length has been reached; BeforeTick and
// AfterTick do nothing after that.
func (l *Live) Done() bool { return l.results == nil || l.k >= l.n }

// Cycle is the number of Ticks completed since Start.
func (l *Live) Cycle() int { return l.k }

// Assertions returns the expect results recorded so far, in evaluation
// order (Finish sorts them by step).
func (l *Live) Assertions() []AssertionResult { return l.rep.Assertions }

// BeforeTick fires the steps due on the next Tick, in due-Tick then file
// order, and advances the running ramps.
func (l *Live) BeforeTick() {
	if l.Done() || l.inTick {
		return
	}
	l.inTick = true
	e, rep := l.e, l.rep
	for l.next < len(l.order) && l.due[l.order[l.next]] == l.k {
		i := l.order[l.next]
		l.next++
		st := &e.s.Steps[i]
		res := &l.results[i]
		res.Fired = true
		res.AtNs = int64(e.t.Clock())
		if a := st.Action; a != nil {
			switch a.Kind {
			case ActRamp:
				l.ramps.start(st, *a, e.t.Clock())
			default:
				l.ramps.override(a)
				if err := e.t.Apply(*a); err != nil {
					res.Error = err.Error()
					rep.Diagnostics = append(rep.Diagnostics, e.diag(diag.Error, st, "SCN008", "step %d: %s failed: %v", st.Index, a.Kind, err))
				}
			}
		}
		if st.Expect != nil {
			l.waits = append(l.waits, &pending{step: st, deadline: l.k + st.Expect.Within})
		}
	}
	l.ramps.advance(e.t, func(r *ramp, err error) {
		i := r.step.Index - 1
		if l.results[i].Error == "" {
			l.results[i].Error = err.Error()
		}
		rep.Diagnostics = append(rep.Diagnostics, e.diag(diag.Error, r.step, "SCN008", "step %d: ramp failed: %v", r.step.Index, err))
	})
}

// AfterTick evaluates the pending expects against the Tick just run and
// counts it.
func (l *Live) AfterTick() {
	if !l.inTick || l.Done() {
		return
	}
	l.inTick = false
	e, rep, k := l.e, l.rep, l.k
	keep := l.waits[:0]
	for _, w := range l.waits {
		x := w.step.Expect
		act, err := e.t.Read(x.Path)
		w.actual, w.readErr = act, err
		if err == nil && Equal(x.Value, act, x.Tol) {
			rep.Assertions = append(rep.Assertions, AssertionResult{Step: w.step.Index, Path: x.Path, Expected: x.Value, Actual: act, Cycle: k, Pass: true})
			continue
		}
		if k >= w.deadline {
			rep.Assertions = append(rep.Assertions, e.fail(rep, w, k, ""))
			continue
		}
		keep = append(keep, w)
	}
	l.waits = keep
	l.k++
}

// Finish ends the run and returns its report. runErr is the Tick error
// that stopped the loop, if any. A run stopped before its length (a Tick
// error, or a server shut down early) is reported at the Ticks completed;
// without a Tick error its pending expects fail with "run ended at cycle
// N" and its unfired steps are SCN010 warnings.
func (l *Live) Finish(runErr error) *Report {
	rep := l.rep
	if l.results == nil { // validation failed
		return rep
	}
	// A failed Tick skipped AfterTick: drop its half-open state so later
	// hooks (stc-mcp keeps stepping after an error) stay no-ops.
	l.inTick = false
	defer func() { l.waits = nil }()
	if l.k < l.n {
		l.n = l.k
		rep.Cycles = l.k
	}
	if runErr == nil {
		for _, w := range l.waits {
			rep.Assertions = append(rep.Assertions, l.e.fail(rep, w, l.n, fmt.Sprintf("run ended at cycle %d", l.n)))
		}
		l.waits = nil
		for i := range l.results {
			if !l.results[i].Fired {
				st := &l.e.s.Steps[i]
				rep.Diagnostics = append(rep.Diagnostics, l.e.diag(diag.Warning, st, "SCN010", "step %d never fired: due at cycle %d, run ended at cycle %d", st.Index, l.due[i], l.n))
			}
		}
	}
	sort.SliceStable(rep.Assertions, func(a, b int) bool { return rep.Assertions[a].Step < rep.Assertions[b].Step })
	rep.Steps = l.results
	rep.SimTimeNs = int64(l.e.t.Clock())
	rep.Passed = runErr == nil && rep.Failed() == 0 && !hasErrors(rep.Diagnostics)
	return rep
}
