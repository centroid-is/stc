package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/scenario"
	"github.com/spf13/cobra"
)

// This file runs `stc sim --scenario`: the scenario drives the project's
// Plant (inputs, faults and expects) and the result is the project status
// plus the scenario report.

// simScenarioResult is the JSON result of a scenario run: every project
// status key, then the scenario report.
type simScenarioResult struct {
	projectStatus
	Scenario *scenario.Report `json:"scenario"`
}

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
	res := simScenarioResult{projectStatus: st, Scenario: rep}
	if err := writeScenarioResult(out, errOut, format, res, gets); err != nil {
		return err
	}
	switch {
	case runErr != nil && !errors.Is(runErr, scenario.ErrPrepareFailed):
		return fmt.Errorf("simulation error: %w", runErr)
	case runErr != nil:
		return fmt.Errorf("%w: %s", errScenarioFailed, runErr)
	case !rep.Passed:
		return fmt.Errorf("%w: %d of %d assertion(s) failed", errScenarioFailed, rep.Failed(), len(rep.Assertions))
	}
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
	if err := writeProjectStatus(out, errOut, format, res.projectStatus, gets); err != nil {
		return err
	}
	fmt.Fprintln(out)
	res.Scenario.Text(out)
	return nil
}

// hasErrorDiag reports whether ds holds an error.
func hasErrorDiag(ds []diag.Diagnostic) bool {
	return countErrors(ds) > 0
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
