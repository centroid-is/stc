package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/scenario"
)

// serveScenario is a scenario run inside the free-running scan of `stc
// serve`.
type serveScenario struct {
	live  *scenario.Live
	loadD []diag.Diagnostic // load warnings, reported with the run
}

// loadServeScenario loads path and validates it against r's Plant. It runs
// before the OPC UA server starts, so a file that does not validate prints
// its SCN diagnostics and fails serve without serving anything.
func loadServeScenario(r *projectRunner, path string, errOut io.Writer) (*serveScenario, error) {
	sc, lds := scenario.Load(path)
	var ds []diag.Diagnostic
	var live *scenario.Live
	err := scenario.ErrPrepareFailed
	if sc != nil && !hasErrorDiag(lds) {
		ex := scenario.NewExecutor(sc, r.Plant)
		ds = ex.Prepare()
		live, err = ex.Start(0)
	}
	if err != nil {
		all := append(append([]diag.Diagnostic(nil), lds...), ds...)
		sortDiags(all)
		for _, d := range all {
			if d.Severity == diag.Error {
				fmt.Fprintln(errOut, d.String())
			}
		}
		return nil, fmt.Errorf("--scenario %s: %w", path, err)
	}
	return &serveScenario{live: live, loadD: lds}, nil
}

// install hooks the run into r's scan loop, after every other hook was
// added: the first Tick's steps fire now, and between Ticks the expects are
// evaluated first, then pending OPC UA writes are applied, then the next
// Tick's steps fire. Expects therefore see the scan result, and a scenario
// step wins over an OPC UA write to the same variable on the same Tick.
func (s *serveScenario) install(r *projectRunner) {
	s.live.BeforeTick()
	hooks := append([]func(){s.live.AfterTick}, r.BeforeTick...)
	r.BeforeTick = append(hooks, s.live.BeforeTick)
}

// report finishes the run and prints its report to errOut. In serve mode
// failed expects and failed actions are warnings: the server ran, and the
// exit status reflects the server, not the scenario.
func (s *serveScenario) report(errOut io.Writer, format string, tickErr error) *scenario.Report {
	rep := s.live.Finish(tickErr)
	rep.Diagnostics = append(append([]diag.Diagnostic(nil), s.loadD...), rep.Diagnostics...)
	for i := range rep.Diagnostics {
		if rep.Diagnostics[i].Severity == diag.Error {
			rep.Diagnostics[i].Severity = diag.Warning
		}
	}
	sortDiags(rep.Diagnostics)
	if format == "json" {
		b, _ := json.Marshal(map[string]any{"event": "scenario", "scenario": rep})
		fmt.Fprintln(errOut, string(b))
		return rep
	}
	passed := len(rep.Assertions) - rep.Failed()
	fmt.Fprintf(errOut, "scenario %s: %d cycles, %d of %d expect(s) passed\n", rep.Name, rep.Cycles, passed, len(rep.Assertions))
	for _, d := range rep.Diagnostics {
		fmt.Fprintln(errOut, d.String())
	}
	return rep
}
