package main

import (
	"fmt"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/scenario"
)

// loadScenario loads path and validates it against the session Plant. Its
// steps then fire as stc_sim_step advances the scan, exactly as in `stc
// sim --scenario`: between Ticks the expects are evaluated, pending OPC UA
// writes applied, then the next Tick's steps fire. A file that does not
// validate fails the session with every error diagnostic.
func (s *simSession) loadScenario(path string) error {
	sc, ds := scenario.Load(path)
	if sc != nil && !hasError(ds) {
		ex := scenario.NewExecutor(sc, s.plant)
		ds = append(ds, ex.Prepare()...)
		live, err := ex.Start(0)
		if err == nil {
			s.live = live
			s.diags = append(s.diags, ds...)
			return nil
		}
	}
	return withErrors(fmt.Errorf("--scenario %s: %w", path, scenario.ErrPrepareFailed), ds)
}

// failures renders failed expects as "step N: path: message".
func failures(as []scenario.AssertionResult) []string {
	var out []string
	for _, a := range as {
		if !a.Pass {
			out = append(out, fmt.Sprintf("step %d: %s: %s", a.Step, a.Path, a.Message))
		}
	}
	return out
}

func hasError(ds []diag.Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == diag.Error {
			return true
		}
	}
	return false
}

// withErrors appends every error diagnostic of ds to err, one per line.
func withErrors(err error, ds []diag.Diagnostic) error {
	for _, d := range ds {
		if d.Severity == diag.Error {
			err = fmt.Errorf("%w\n%s", err, d.String())
		}
	}
	return err
}
