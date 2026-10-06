package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/scenario"
	"github.com/spf13/cobra"
)

// This file holds the project runner shared by `stc sim` (project mode) and
// `stc serve`: load the project into an interp.Project, attach the EtherCAT
// network (--io), restore PERSISTENT/RETAIN state (--persist), apply --set,
// run it deterministically or free-running, and report its status.

// defaultPersistInterval is how often free-running mode saves --persist.
const defaultPersistInterval = 10 * time.Second

// addProjectRunFlags registers the flags every project runner shares.
func addProjectRunFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("io", nil, "EtherCATConfig export (Device N.xml) whose network is attached to the project's TcLinkTo links (repeatable)")
	cmd.Flags().String("persist", "", "JSON state file for PERSISTENT/RETAIN variables: loaded at start, saved periodically and on stop")
	cmd.Flags().Duration("persist-interval", defaultPersistInterval, "Wall-time interval between --persist saves in free-running mode")
}

// projectRunner is a loaded project ready to run.
type projectRunner struct {
	P        *interp.Project
	Plant    *scenario.Plant // P with its EtherCAT network (scenarios)
	Spec     interp.ProjectSpec
	Analysis analyzer.AnalysisResult // symbol table for the OPC UA address space
	// BeforeTick hooks run on the scan goroutine between Ticks of runFree
	// (stc serve --opcua applies queued OPC UA writes there).
	BeforeTick   []func()
	Binder       *interp.IOBinder
	Diags        []diag.Diagnostic // load warnings
	Warnings     []string          // state file and I/O warnings
	PersistPath  string
	PersistEvery time.Duration
}

// projectSetupOpts are the per-command inputs of projectSetup.
type projectSetupOpts struct {
	Sets []simSet
	// Cycle, when positive, replaces the cycle of the project's only task
	// (or of the default MAIN task); several tasks keep their own cycles
	// and make it an error.
	Cycle time.Duration
}

// projectSetup loads inputs (one .tsproj/.plcproj or .st files) as a
// project, attaches --io, restores --persist and applies opts.Sets in that
// order, so --set overrides persisted values. Errors are returned; load
// diagnostics that fail the load are printed to errOut first.
func projectSetup(cmd *cobra.Command, inputs []string, defines map[string]bool, opts projectSetupOpts, errOut io.Writer) (*projectRunner, error) {
	spec, res, ds, err := loadProjectAnalysis(inputs, defines)
	if err != nil {
		for _, d := range ds {
			if d.Severity == diag.Error {
				fmt.Fprintln(errOut, d.String())
			}
		}
		return nil, err
	}
	if opts.Cycle > 0 {
		switch len(spec.Tasks) {
		case 0:
			spec.Tasks = []interp.TaskSpec{{Name: interp.DefaultTaskName, Cycle: opts.Cycle, Programs: []string{"MAIN"}}}
		case 1:
			spec.Tasks[0].Cycle = opts.Cycle
		default:
			return nil, fmt.Errorf("--cycle cannot override the %d task cycles of the project", len(spec.Tasks))
		}
	}
	ioFlags, _ := cmd.Flags().GetStringSlice("io")
	ioFiles, err := expandIOGlobs(ioFlags)
	if err != nil {
		return nil, err
	}
	plant, eds, err := buildPlant(spec, ioFiles)
	for _, d := range eds {
		if d.Severity == diag.Error {
			fmt.Fprintln(errOut, d.String())
		}
	}
	if err != nil {
		return nil, err
	}
	p := plant.Project()
	r := &projectRunner{P: p, Plant: plant, Spec: spec, Analysis: res, Binder: plant.IOBinder()}
	for _, d := range ds {
		if d.Severity != diag.Error {
			r.Diags = append(r.Diags, d)
		}
	}
	for _, d := range eds {
		if d.Severity != diag.Error {
			r.Diags = append(r.Diags, d)
		}
	}
	r.PersistPath, _ = cmd.Flags().GetString("persist")
	r.PersistEvery, _ = cmd.Flags().GetDuration("persist-interval")
	if r.PersistPath != "" {
		if r.PersistEvery <= 0 {
			return nil, fmt.Errorf("--persist-interval must be positive, got %s", r.PersistEvery)
		}
		warns, err := p.LoadState(r.PersistPath)
		if err != nil {
			return nil, err
		}
		for _, w := range warns {
			r.Warnings = append(r.Warnings, "persist: "+w)
		}
	}
	for _, s := range opts.Sets {
		if err := p.Runtime().Set(s.path, s.value); err != nil {
			return nil, fmt.Errorf("--set %s: %w", s.path, err)
		}
	}
	return r, nil
}

// buildPlant loads the project as a scenario.Plant: with ioFiles it loads
// the EtherCAT exports, resolves the project's TcLinkTo links against them
// and attaches the network (Tc2_EtherCAT mocks and IOBinder). Resolve
// errors (unresolved links, size or direction mismatches) fail with every
// diagnostic returned; warnings are returned with a nil error.
func buildPlant(spec interp.ProjectSpec, ioFiles []string) (*scenario.Plant, []diag.Diagnostic, error) {
	ps, err := scenario.BuildPlantSpec(spec, ioFiles)
	if err != nil {
		if ps.Topology == nil {
			return nil, nil, fmt.Errorf("--io: %w", err)
		}
		return nil, ps.Diagnostics, fmt.Errorf("EtherCAT links do not resolve: %d error(s)", countErrors(ps.Diagnostics))
	}
	plant, err := ps.New()
	if err != nil {
		return nil, ps.Diagnostics, fmt.Errorf("initialisation error: %w", err)
	}
	return plant, ps.Diagnostics, nil
}

// expandIOGlobs expands --io values containing *, ? or [ with
// filepath.Glob (sorted); a pattern without a match is an error. Other
// values are kept as given.
func expandIOGlobs(values []string) ([]string, error) {
	var out []string
	for _, v := range values {
		if !strings.ContainsAny(v, "*?[") {
			out = append(out, v)
			continue
		}
		m, err := filepath.Glob(v)
		if err != nil {
			return nil, fmt.Errorf("--io %q: %w", v, err)
		}
		if len(m) == 0 {
			return nil, fmt.Errorf("--io %q matches no file", v)
		}
		sort.Strings(m)
		out = append(out, m...)
	}
	return out, nil
}

// countErrors counts the error-severity diagnostics of ds.
func countErrors(ds []diag.Diagnostic) int {
	n := 0
	for _, d := range ds {
		if d.Severity == diag.Error {
			n++
		}
	}
	return n
}

// ticks runs n deterministic Ticks; the first Tick error stops the run.
func (r *projectRunner) ticks(n int) error {
	for i := 0; i < n; i++ {
		if err := r.P.Tick(); err != nil {
			return fmt.Errorf("cycle %d: %w", i+1, err)
		}
	}
	return nil
}

// runFree runs free-running until duration of wall time passes (0: until
// ctx is done), saving --persist every PersistEvery of wall time from the
// Run goroutine and once more at the end, whatever stopped the run. A
// cancelled ctx is a normal stop. Save errors are added to Warnings while
// running and returned at the end.
func (r *projectRunner) runFree(ctx context.Context, duration time.Duration, clock interp.WallClock) error {
	if clock == nil {
		clock = interp.NewWallClock()
	}
	opts := interp.RunOpts{Duration: duration, Clock: clock}
	if r.PersistPath != "" || len(r.BeforeTick) > 0 {
		last := clock.Now()
		opts.OnTick = func(time.Duration) {
			for _, h := range r.BeforeTick {
				h()
			}
			if r.PersistPath == "" {
				return
			}
			if now := clock.Now(); now-last >= r.PersistEvery {
				last = now
				if err := r.P.SaveState(r.PersistPath); err != nil {
					r.Warnings = append(r.Warnings, "persist: "+err.Error())
				}
			}
		}
	}
	err := r.P.Run(ctx, opts)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		err = nil
	}
	return errors.Join(err, r.save())
}

// save writes --persist when it is set.
func (r *projectRunner) save() error {
	if r.PersistPath == "" {
		return nil
	}
	return r.P.SaveState(r.PersistPath)
}

// projectTaskJSON is one task in the project status.
type projectTaskJSON struct {
	Name     string   `json:"name"`
	CycleNS  int64    `json:"cycle_ns"`
	Priority int      `json:"priority"`
	Programs []string `json:"programs"`
	Runs     uint64   `json:"runs"`
	Overruns uint64   `json:"overruns"`
}

// projectStatus is the result of a project run (`stc sim` project mode and
// the final `stc serve` status).
type projectStatus struct {
	Cycles      int               `json:"cycles"`
	SimTimeNS   int64             `json:"sim_time_ns"`
	Tasks       []projectTaskJSON `json:"tasks"`
	Get         map[string]any    `json:"get,omitempty"`
	Diagnostics []diag.Diagnostic `json:"diagnostics"`
	Warnings    []string          `json:"warnings"`
}

// status reports the run: ticks, the virtual clock, task counters, the
// --get values, load and runtime diagnostics (auto-stubs RUNT001/RUNT002)
// and state file and I/O warnings.
func (r *projectRunner) status(gets []string) (projectStatus, error) {
	st := projectStatus{SimTimeNS: int64(r.P.Clock()), Tasks: []projectTaskJSON{},
		Diagnostics: []diag.Diagnostic{}, Warnings: []string{}}
	if base := r.P.BaseTick(); base > 0 {
		st.Cycles = int(r.P.Clock() / base)
	}
	for _, t := range r.P.Tasks() {
		st.Tasks = append(st.Tasks, projectTaskJSON{Name: t.Name, CycleNS: int64(t.Cycle),
			Priority: t.Priority, Programs: t.Programs, Runs: t.Runs, Overruns: t.Overruns})
	}
	if len(gets) > 0 {
		st.Get = make(map[string]any, len(gets))
		rt := r.P.Runtime()
		for _, path := range gets {
			v, err := rt.Get(path)
			if err != nil {
				return st, fmt.Errorf("--get %s: %w", path, err)
			}
			st.Get[path] = rt.ToJSON(v)
		}
	}
	st.Diagnostics = append(st.Diagnostics, r.Diags...)
	st.Diagnostics = append(st.Diagnostics, r.P.Runtime().Interpreter().Warnings()...)
	st.Warnings = append(st.Warnings, r.Warnings...)
	if r.Binder != nil {
		for _, err := range r.Binder.Errors() {
			st.Warnings = append(st.Warnings, "io: "+err.Error())
		}
	}
	return st, nil
}

// writeProjectStatus prints st as indented JSON, or as text: diagnostics
// and warnings on errOut, a task table and "PATH = JSON" lines (in gets
// order) on out.
func writeProjectStatus(out, errOut io.Writer, format string, st projectStatus, gets []string) error {
	if format == "json" {
		b, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return fmt.Errorf("JSON marshal error: %w", err)
		}
		fmt.Fprintln(out, string(b))
		return nil
	}
	for _, d := range st.Diagnostics {
		fmt.Fprintln(errOut, d.String())
	}
	for _, w := range st.Warnings {
		fmt.Fprintln(errOut, "warning: "+w)
	}
	fmt.Fprintf(out, "Project: %d cycles, sim time %s\n\n", st.Cycles, time.Duration(st.SimTimeNS))
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TASK\tCYCLE\tPRIORITY\tPROGRAMS\tRUNS\tOVERRUNS")
	for _, t := range st.Tasks {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%d\t%d\n", t.Name, time.Duration(t.CycleNS), t.Priority,
			strings.Join(t.Programs, ","), t.Runs, t.Overruns)
	}
	_ = tw.Flush()
	if len(gets) > 0 {
		fmt.Fprintln(out)
	}
	for _, path := range gets {
		b, err := json.Marshal(st.Get[path])
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s = %s\n", path, b)
	}
	return nil
}
