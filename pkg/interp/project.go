package interp

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
)

// DefaultTaskName and DefaultTaskCycle describe the task LoadProject creates
// when a ProjectSpec has no tasks (a plcproj without tsproj, or .st files):
// one 10 ms task running MAIN.
const (
	DefaultTaskName  = "PlcTask"
	DefaultTaskCycle = 10 * time.Millisecond
)

// TaskSpec is one PLC task: its PROGRAMs run in list order every Cycle.
// Lower Priority values run first when several tasks are due on one tick.
type TaskSpec struct {
	Name     string
	Cycle    time.Duration
	Priority int
	Programs []string
}

// ProjectSpec is everything LoadProject needs: library files (registered
// first), the project files, and the task configuration.
type ProjectSpec struct {
	LibraryFiles []*ast.SourceFile
	Files        []*ast.SourceFile
	Tasks        []TaskSpec
}

// TaskStats reports one task of a Project.
type TaskStats struct {
	Name     string
	Cycle    time.Duration
	Priority int
	Programs []string
	Runs     uint64
	Overruns uint64
}

// projectTask is one scheduled task of a Project.
type projectTask struct {
	spec     TaskSpec
	engines  []*ScanCycleEngine
	progs    []string // declared program names, parallel to engines
	nextDue  time.Duration
	runs     uint64
	overruns uint64
}

// Project runs a whole PLC project on one interpreter: every GVL is
// instantiated once (by NewRuntime) and every task's PROGRAMs run at the
// task's cycle time in priority order on a deterministic clock.
type Project struct {
	rt     *Runtime
	tasks  []*projectTask // sorted by Priority, then Name
	base   time.Duration
	clock  time.Duration
	binder *IOBinder
}

// LoadProject registers spec.LibraryFiles and then spec.Files on one
// interpreter through NewRuntime and binds every task to the engines of its
// PROGRAMs. With no tasks it creates DefaultTaskName running MAIN every
// DefaultTaskCycle. It rejects non-positive cycles, unknown programs and
// programs bound to more than one task, reporting all problems joined.
func LoadProject(spec ProjectSpec) (*Project, error) {
	rt, err := NewRuntime(spec.Files, RuntimeOpts{LibraryFiles: spec.LibraryFiles})
	if err != nil {
		return nil, err
	}
	specs := spec.Tasks
	if len(specs) == 0 {
		if rt.program("MAIN") == nil {
			return nil, errors.New("project has no tasks and no PROGRAM MAIN")
		}
		specs = []TaskSpec{{Name: DefaultTaskName, Cycle: DefaultTaskCycle, Programs: []string{"MAIN"}}}
	}

	p := &Project{rt: rt}
	var errs []error
	owner := make(map[string]string) // upper program name -> task name
	for _, ts := range specs {
		if ts.Cycle <= 0 {
			errs = append(errs, fmt.Errorf("task %s: cycle time must be positive, got %v", ts.Name, ts.Cycle))
			continue
		}
		t := &projectTask{spec: ts}
		for _, name := range ts.Programs {
			pr := rt.program(name)
			if pr == nil {
				errs = append(errs, fmt.Errorf("task %s: unknown PROGRAM %s", ts.Name, name))
				continue
			}
			key := strings.ToUpper(pr.name)
			if prev, dup := owner[key]; dup {
				errs = append(errs, fmt.Errorf("PROGRAM %s is bound to tasks %s and %s", pr.name, prev, ts.Name))
				continue
			}
			owner[key] = ts.Name
			t.engines = append(t.engines, pr.engine)
			t.progs = append(t.progs, pr.name)
		}
		p.tasks = append(p.tasks, t)
		p.base = gcdDuration(p.base, ts.Cycle)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	sort.SliceStable(p.tasks, func(i, j int) bool {
		a, b := p.tasks[i].spec, p.tasks[j].spec
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return a.Name < b.Name
	})
	return p, nil
}

// gcdDuration returns the greatest common divisor of a and b (a may be 0).
func gcdDuration(a, b time.Duration) time.Duration {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// Runtime returns the project's Runtime for Get, Set, Snapshot and ToJSON.
// Its Tick runs every PROGRAM with one dt; use Project.Tick to schedule by
// task instead.
func (p *Project) Runtime() *Runtime {
	return p.rt
}

// Tasks returns the task statistics sorted by priority, then name.
func (p *Project) Tasks() []TaskStats {
	p.rt.mu.Lock()
	defer p.rt.mu.Unlock()
	out := make([]TaskStats, 0, len(p.tasks))
	for _, t := range p.tasks {
		out = append(out, TaskStats{
			Name:     t.spec.Name,
			Cycle:    t.spec.Cycle,
			Priority: t.spec.Priority,
			Programs: append([]string(nil), t.progs...),
			Runs:     t.runs,
			Overruns: t.overruns,
		})
	}
	return out
}

// BaseTick is the clock step of one Tick: the GCD of all task cycles.
func (p *Project) BaseTick() time.Duration {
	return p.base
}

// Clock returns the project's virtual time: BaseTick times the number of
// Ticks run.
func (p *Project) Clock() time.Duration {
	p.rt.mu.Lock()
	defer p.rt.mu.Unlock()
	return p.clock
}

// Tick runs one base tick. Every task whose next due time is at or before
// the current clock runs, in priority then name order, each of its PROGRAMs
// in list order with dt equal to the task cycle; the clock then advances by
// BaseTick. An attached IOBinder steps once before the first due task and
// copies outputs once after the last. Engine errors do not stop other tasks;
// they are returned joined with the task and program names. Tick holds the
// Runtime mutex, so it is serialised with Runtime Get and Set.
func (p *Project) Tick() error {
	p.rt.mu.Lock()
	defer p.rt.mu.Unlock()
	now := p.clock + p.base
	var errs []error
	ran := false
	for _, t := range p.tasks {
		if t.nextDue > p.clock {
			continue
		}
		if !ran {
			ran = true
			if p.binder != nil {
				p.binder.preScan(p.base)
			}
		}
		// FB timers measure the interpreter clock between their runs, so
		// it is the end of this tick; dt is the task cycle for first runs.
		p.rt.interp.dt = t.spec.Cycle
		p.rt.interp.clock = now
		for i, e := range t.engines {
			if err := e.tick(t.spec.Cycle, false); err != nil {
				errs = append(errs, fmt.Errorf("task %s PROGRAM %s: %w", t.spec.Name, t.progs[i], err))
			}
		}
		t.runs++
		t.nextDue += t.spec.Cycle
	}
	if ran && p.binder != nil {
		p.binder.postScan()
	}
	p.clock = now
	p.rt.interp.clock = now
	return errors.Join(errs...)
}
