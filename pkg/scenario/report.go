package scenario

import "github.com/centroid-is/stc/pkg/diag"

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
