package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/projectload"
	"github.com/centroid-is/stc/pkg/scenario"
	"github.com/spf13/cobra"
)

// This file runs `stc sim --scenario`: the scenario drives the project's
// Plant (inputs, faults and expects) and the result is the project status
// plus the scenario report.

// simScenarioResult is the JSON result of a scenario run: every project
// status key (diagnostics then also hold the network, binder and scenario
// diagnostics, sorted), then the scenario report, the TcLinkTo outputs
// and the unhealthy EtherCAT slaves.
type simScenarioResult struct {
	projectStatus
	Scenario *scenario.Report `json:"scenario"`
	// Outputs maps every TcLinkTo output variable path to its value after
	// the run (encoding/json sorts the keys).
	Outputs  map[string]any  `json:"outputs"`
	EtherCAT []ecatSlaveJSON `json:"ethercat"`
}

// ecatSlaveJSON is one slave that is not in OP, has a bad link or an
// invalid working counter at the end of the run.
type ecatSlaveJSON struct {
	Master string `json:"master"`
	Slave  int    `json:"slave"`
	Name   string `json:"name"`
	State  uint16 `json:"state"`
	Link   uint8  `json:"link"`
	WcBad  bool   `json:"wc_bad"`
}

// ecatStateOP is the EtherCAT OP state code.
const ecatStateOP = 8

// errScenarioFailed makes stc sim exit 1 after the report is printed.
var errScenarioFailed = errors.New("scenario failed")

// runSimScenario loads path, validates it against r's Plant (validation
// errors stop before the first Tick) and runs it for cycles Ticks (0: the
// scenario's own length). It prints the report and returns an error when
// the scenario fails, an assertion fails or a Tick errors.
func runSimScenario(cmd *cobra.Command, r *projectRunner, path string, cycles int, gets []string, format string) error {
	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	sc, lds := scenario.Load(path)
	var rep *scenario.Report
	var runErr error
	if sc == nil || hasErrorDiag(lds) {
		rep = &scenario.Report{Name: path, Diagnostics: lds}
		runErr = scenario.ErrPrepareFailed
	} else {
		ex := scenario.NewExecutor(sc, r.Plant)
		ex.Prepare()
		rep, runErr = ex.Run(cycles)
		rep.Diagnostics = append(append([]diag.Diagnostic(nil), lds...), rep.Diagnostics...)
	}
	sortDiags(rep.Diagnostics)
	if rep.Steps == nil {
		rep.Steps = []scenario.StepResult{}
	}
	if rep.Assertions == nil {
		rep.Assertions = []scenario.AssertionResult{}
	}
	st, err := r.status(gets)
	if err != nil {
		return err
	}
	res := simScenarioResult{projectStatus: st, Scenario: rep, Outputs: map[string]any{}, EtherCAT: []ecatSlaveJSON{}}
	if err := r.scenarioExtras(&res); err != nil {
		return err
	}
	if err := writeScenarioResult(out, errOut, format, res, gets); err != nil {
		return err
	}
	switch {
	case runErr != nil && !errors.Is(runErr, scenario.ErrPrepareFailed):
		return fmt.Errorf("simulation error: %w", runErr)
	case runErr != nil:
		return fmt.Errorf("%w: %s", errScenarioFailed, runErr)
	case !rep.Passed || hasErrorDiag(res.Diagnostics):
		return fmt.Errorf("%w: %d of %d assertion(s) failed, %d error diagnostic(s)", errScenarioFailed,
			rep.Failed(), len(rep.Assertions), projectload.CountErrors(res.Diagnostics))
	}
	return nil
}

// scenarioExtras fills the outputs and ethercat sections and merges the
// network warnings (ECAT010), binder errors (SIM001) and scenario
// diagnostics into res.Diagnostics, sorted.
func (r *projectRunner) scenarioExtras(res *simScenarioResult) error {
	ds := append([]diag.Diagnostic(nil), res.Diagnostics...)
	ds = append(ds, res.Scenario.Diagnostics...)
	if b := r.Plant.IOBinder(); b != nil {
		rt := r.P.Runtime()
		for _, bd := range b.OutputBindings() {
			v, err := rt.Get(bd.Var.Path)
			if err != nil {
				continue // dropped at resolution; reported as SIM001 below
			}
			res.Outputs[bd.Var.Path] = rt.ToJSON(v)
		}
		for _, err := range b.Errors() {
			ds = append(ds, diag.Diagnostic{Severity: diag.Warning, Code: "SIM001", Message: err.Error()})
		}
	}
	if net := r.Plant.Network(); net != nil {
		ds = append(ds, net.Diagnostics()...)
		for _, m := range net.Topo.Masters {
			for i, sl := range m.Slaves {
				state, link := net.SlaveState(m.Name, i)
				wc := net.WcBad(m.Name, i)
				if state == ecatStateOP && link == 0 && !wc {
					continue
				}
				res.EtherCAT = append(res.EtherCAT, ecatSlaveJSON{Master: m.Name, Slave: i, Name: sl.Name, State: state, Link: link, WcBad: wc})
			}
		}
		sort.SliceStable(res.EtherCAT, func(i, j int) bool {
			a, b := res.EtherCAT[i], res.EtherCAT[j]
			if a.Master != b.Master {
				return a.Master < b.Master
			}
			return a.Slave < b.Slave
		})
	}
	sortDiags(ds)
	res.Diagnostics = ds
	return nil
}

// writeScenarioResult prints res as indented JSON, or as the project
// status text followed by the scenario report.
func writeScenarioResult(out, errOut io.Writer, format string, res simScenarioResult, gets []string) error {
	if format == "json" {
		b, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return fmt.Errorf("JSON marshal error: %w", err)
		}
		fmt.Fprintln(out, string(b))
		return nil
	}
	// Diagnostics go to the diagnostics section on out, not to errOut.
	st := res.projectStatus
	st.Diagnostics = nil
	if err := writeProjectStatus(out, errOut, format, st, gets); err != nil {
		return err
	}
	fmt.Fprintln(out)
	res.Scenario.Text(out)
	paths := make([]string, 0, len(res.Outputs))
	for p := range res.Outputs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	fmt.Fprintf(out, "\noutputs (%d):\n", len(paths))
	for _, p := range paths {
		b, err := json.Marshal(res.Outputs[p])
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "  %s = %s\n", p, b)
	}
	fmt.Fprintf(out, "\nethercat (%d not healthy):\n", len(res.EtherCAT))
	for _, s := range res.EtherCAT {
		fmt.Fprintf(out, "  %s #%d %s: state %d link %d wc_bad %t\n", s.Master, s.Slave, s.Name, s.State, s.Link, s.WcBad)
	}
	fmt.Fprintf(out, "\ndiagnostics (%d):\n", len(res.Diagnostics))
	for _, d := range res.Diagnostics {
		fmt.Fprintf(out, "  %s: %s: %s %s\n", d.Pos, d.Severity, d.Code, d.Message)
	}
	return nil
}

// hasErrorDiag reports whether ds holds an error.
func hasErrorDiag(ds []diag.Diagnostic) bool {
	return projectload.CountErrors(ds) > 0
}

// sortDiags orders ds by file, line, column, code and message.
func sortDiags(ds []diag.Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		switch {
		case a.Pos.File != b.Pos.File:
			return a.Pos.File < b.Pos.File
		case a.Pos.Line != b.Pos.Line:
			return a.Pos.Line < b.Pos.Line
		case a.Pos.Col != b.Pos.Col:
			return a.Pos.Col < b.Pos.Col
		case a.Code != b.Code:
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
}
