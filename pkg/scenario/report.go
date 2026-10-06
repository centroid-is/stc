package scenario

import (
	"fmt"
	"io"

	"github.com/centroid-is/stc/pkg/diag"
)

// Report is the outcome of one scenario run.
type Report struct {
	Name        string            `json:"name"`
	Cycles      int               `json:"cycles"`
	SimTimeNs   int64             `json:"sim_time_ns"`
	Steps       []StepResult      `json:"steps"`
	Assertions  []AssertionResult `json:"assertions"`
	Diagnostics []diag.Diagnostic `json:"diagnostics"`
	Passed      bool              `json:"passed"`
}

// StepResult records when and whether a step fired.
type StepResult struct {
	Index  int    `json:"index"`
	Line   int    `json:"line"`
	Cycle  int    `json:"cycle"`
	AtNs   int64  `json:"at_ns"`
	Action string `json:"action"`
	Fired  bool   `json:"fired"`
	Error  string `json:"error,omitempty"`
}

// AssertionResult is one evaluated expect.
type AssertionResult struct {
	Step     int    `json:"step"`
	Path     string `json:"path"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Cycle    int    `json:"cycle"`
	Pass     bool   `json:"pass"`
	Message  string `json:"message,omitempty"`
}

// Failed counts failing assertions.
func (r *Report) Failed() int {
	n := 0
	for _, a := range r.Assertions {
		if !a.Pass {
			n++
		}
	}
	return n
}

// Text writes one line per step and per assertion in index order, then the
// diagnostics and a final PASS/FAIL line.
func (r *Report) Text(w io.Writer) {
	fmt.Fprintf(w, "scenario %s: %d cycles, sim time %dns\n", r.Name, r.Cycles, r.SimTimeNs)
	for _, s := range r.Steps {
		state := "fired"
		if !s.Fired {
			state = "not fired"
		}
		fmt.Fprintf(w, "  step %d (line %d) cycle %d: %s [%s]", s.Index, s.Line, s.Cycle, s.Action, state)
		if s.Error != "" {
			fmt.Fprintf(w, " error: %s", s.Error)
		}
		fmt.Fprintln(w)
	}
	for _, a := range r.Assertions {
		if a.Pass {
			fmt.Fprintf(w, "  PASS step %d: %s = %v at cycle %d\n", a.Step, a.Path, a.Actual, a.Cycle)
		} else {
			fmt.Fprintf(w, "  FAIL step %d: %s at cycle %d: %s\n", a.Step, a.Path, a.Cycle, a.Message)
		}
	}
	for _, d := range r.Diagnostics {
		fmt.Fprintf(w, "  %s %s: %s\n", d.Severity, d.Code, d.Message)
	}
	if r.Passed {
		fmt.Fprintln(w, "PASS")
	} else {
		fmt.Fprintf(w, "FAIL: %d assertion(s) failed\n", r.Failed())
	}
}
