package interp

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/iomap"
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
	// Network, when set, backs the Tc2_EtherCAT mocks (RuntimeOpts.Network);
	// the services scan is counted once per Tick that runs a task.
	Network *ecat.Network
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
	rt    *Runtime
	tasks []*projectTask // sorted by Priority, then Name
	base  time.Duration
	clock time.Duration
	// netClock is the virtual time of the binder's last network step.
	netClock time.Duration
	binder   *IOBinder
	// io is the process image shared by every engine and the AT slots.
	io    *iomap.IOTable
	files []*ast.SourceFile // library files then project files
	slots []ioSlot
	// persist lists the PERSISTENT/RETAIN paths (see PersistPaths).
	persist []string
}

// LoadProject registers spec.LibraryFiles and then spec.Files on one
// interpreter through NewRuntime and binds every task to the engines of its
// PROGRAMs. With no tasks it creates DefaultTaskName running MAIN every
// DefaultTaskCycle. It rejects non-positive cycles, unknown programs and
// programs bound to more than one task, reporting all problems joined.
func LoadProject(spec ProjectSpec) (*Project, error) {
	rt, err := NewRuntime(spec.Files, RuntimeOpts{LibraryFiles: spec.LibraryFiles, Network: spec.Network})
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

	p := &Project{rt: rt, io: iomap.NewIOTable()}
	p.files = append(append(p.files, spec.LibraryFiles...), spec.Files...)
	for _, pr := range rt.programs {
		pr.engine.ioTable = p.io
	}
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
	p.allocIO()
	p.persist = p.collectPersist()
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
// BaseTick. An attached IOBinder steps its network once before the first due
// task, by the virtual time since its previous step, and copies outputs once
// after the last; the Project's AT slots (see IOSlot)
// are copied in and out at the same points. Engine errors do not stop other tasks;
// they are returned joined with the task and program names. Tick holds the
// Runtime mutex, so it is serialised with Runtime Get and Set.
func (p *Project) Tick() error {
	p.rt.mu.Lock()
	defer p.rt.mu.Unlock()
	p.rt.ticking.Store(true)
	defer p.rt.ticking.Store(false)
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
				// Devices see all virtual time since their last step, also
				// the base ticks on which no task was due.
				p.binder.preScan(now - p.netClock)
				p.netClock = now
			}
			if p.rt.ecat != nil {
				p.rt.ecat.preScan(p.base, p.binder)
			}
			p.syncIOIn()
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
	if ran {
		p.syncIOOut()
		if p.binder != nil {
			p.binder.postScan()
		}
	}
	p.clock = now
	p.rt.interp.clock = now
	return errors.Join(errs...)
}

// Advance runs d / BaseTick Ticks. d must be a non-negative multiple of
// BaseTick. The first Tick error stops the run and is returned.
func (p *Project) Advance(d time.Duration) error {
	if d < 0 || d%p.base != 0 {
		return fmt.Errorf("advance %v is not a multiple of the base tick %v", d, p.base)
	}
	for n := d / p.base; n > 0; n-- {
		if err := p.Tick(); err != nil {
			return err
		}
	}
	return nil
}

// SetIOBinder attaches EtherCAT links to the project. The binder steps the
// network once per Tick that runs a task, not once per task, so it is held
// by the Project rather than by the task engines. Nil detaches it.
func (p *Project) SetIOBinder(b *IOBinder) {
	p.rt.mu.Lock()
	defer p.rt.mu.Unlock()
	p.netClock = p.clock
	if b != nil {
		b.interp = p.rt.interp
		b.progEnv = func(name string) *Env {
			if pr := p.rt.program(name); pr != nil {
				pr.engine.Initialize()
				return pr.engine.env
			}
			return nil
		}
		b.resolved = false
	}
	p.binder = b
	p.allocIO() // wildcards claimed by the binder lose their auto slots
}

// WallClock is the real-time source of Run: Now is monotonic time since an
// arbitrary origin and Sleep waits d or until ctx is done (returning
// ctx.Err()). Tests substitute a fake to run free-running mode
// deterministically.
type WallClock interface {
	Now() time.Duration
	Sleep(ctx context.Context, d time.Duration) error
}

// NewWallClock returns the monotonic WallClock Run uses by default: Now is
// time.Since a start instant (Go's monotonic reading, immune to wall-clock
// steps) and Sleep is timer based.
func NewWallClock() WallClock {
	return monoClock{start: time.Now()}
}

type monoClock struct{ start time.Time }

func (m monoClock) Now() time.Duration { return time.Since(m.start) }

func (monoClock) Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// RunOpts configures Run. Duration 0 runs until ctx is done; a nil Clock
// uses NewWallClock; OnTick, when set, receives the virtual clock after
// every Tick (from Run's goroutine).
type RunOpts struct {
	Duration time.Duration
	Clock    WallClock
	OnTick   func(sim time.Duration)
}

// Run is free-running mode: it calls Tick once per BaseTick of wall time.
// Tick n is due at start + n*BaseTick; Run sleeps until then (never
// busy-waits) and ticks. When a Tick ends after the next due time, every
// task due within the missed base ticks counts one overrun and Run
// realigns to the next due time after now instead of bursting catch-up
// ticks, so the virtual clock falls behind wall time under overload. Run
// returns nil when opts.Duration of wall time is covered, ctx.Err() when
// ctx is done, and the first Tick error.
func (p *Project) Run(ctx context.Context, opts RunOpts) error {
	clk := opts.Clock
	if clk == nil {
		clk = NewWallClock()
	}
	start := clk.Now()
	var n int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		due := time.Duration(n) * p.base
		if opts.Duration > 0 && due >= opts.Duration {
			return nil
		}
		if d := start + due - clk.Now(); d > 0 {
			if err := clk.Sleep(ctx, d); err != nil {
				return err
			}
		}
		if err := p.Tick(); err != nil {
			return err
		}
		if opts.OnTick != nil {
			opts.OnTick(p.Clock())
		}
		n++
		if late := clk.Now() - start; late > time.Duration(n)*p.base {
			next := int64((late + p.base - 1) / p.base)
			p.countOverruns(next - n)
			n = next
		}
	}
}

// countOverruns adds one overrun to every task due within the next missed
// base ticks of the virtual clock.
func (p *Project) countOverruns(missed int64) {
	p.rt.mu.Lock()
	defer p.rt.mu.Unlock()
	window := p.clock + time.Duration(missed)*p.base
	for _, t := range p.tasks {
		if t.nextDue < window {
			t.overruns++
		}
	}
}
