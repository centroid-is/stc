---
phase: 23-project-execution-runtime
plan: 01
subsystem: interp, cmd/stc, checker, types
tags: [runtime, scheduler, tasks, deterministic-clock, builtins, SIZEOF, ADR]
requires:
  - Phase 22-05 Runtime (NewRuntime, RuntimeOpts, Engine, Get/Set, mu)
  - Phase 21 twincat.Import, analyzer.AnalyzeProject, Model.Tasks
  - Phase 24 IOBinder preScan/postScan
provides:
  - interp.ProjectSpec, TaskSpec, TaskStats, Project, LoadProject
  - Project.Tick/Advance/Clock/BaseTick/Tasks/Runtime/SetIOBinder
  - cmd/stc loadProjectSpec(paths, defines) (interp.ProjectSpec, []diag.Diagnostic, error)
  - SIZEOF, ADR (member paths), SHL/SHR/ROL/ROR, all BOOL/int/bit/real X_TO_Y conversions
affects: [23-02 free-running Run, 23-03 sim/serve + persist, 23-04 gate]
tech-stack:
  added: []
  patterns: [GCD base tick scheduler, project-level IOBinder with program lookup func]
key-files:
  created:
    - pkg/interp/project.go
    - pkg/interp/project_test.go
    - pkg/interp/stdlib_sys.go
    - pkg/interp/stdlib_sys_test.go
    - pkg/types/builtin_sys.go
    - pkg/checker/sysfunc_test.go
    - cmd/stc/project_load.go
    - cmd/stc/project_load_test.go
    - cmd/stc/project_load_sild_test.go
  modified:
    - pkg/interp/iobind.go
    - pkg/interp/interpreter.go
    - pkg/interp/ref_path.go
    - pkg/interp/fb_instance.go
    - pkg/types/builtin.go
    - pkg/checker/check.go
decisions:
  - "Project.Tick sets interp.dt to the task cycle and interp.clock to the end-of-tick time, so FB timers in a 10 ms task see 10 ms per run"
  - "IOBinder belongs to the Project (one preScan/postScan per tick that runs a task) and resolves program roots across all project programs"
  - "loadProjectSpec drops program-less tasks (VEND024 default PlcTask) so LoadProject applies its MAIN default"
  - "SIZEOF of STRING/WSTRING assumes length 80 because values do not carry their declared length"
metrics:
  duration: ~45 min
  completed: 2026-10-06
  tasks: 3 (+1 required built-ins deviation)
  files: 15
---

# Phase 23 Plan 01: Project Loading and Per-Task Deterministic Scheduling Summary

LoadProject puts a whole project on one interpreter and Project.Tick runs each task's PROGRAMs at the task's own cycle in priority order on a GCD base-tick clock; the imported ST101 now runs MAIN for 1000 ticks after adding SIZEOF, ADR member paths, shifts and the full X_TO_Y conversion matrix.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | ProjectSpec, LoadProject and per-task engines | 8be5d02 |
| 2 | Deterministic Project.Tick with priority scheduling | 0caaa87 |
| 3 | Shared CLI loader loadProjectSpec | f5ac404 |
| - | Built-ins so imported ST101 runs (Rule 1) | cfc5b38 |

## API for 23-02..23-04

- `interp.LoadProject(interp.ProjectSpec{LibraryFiles, Files, Tasks []TaskSpec{Name, Cycle, Priority, Programs}}) (*Project, error)`
- `(*Project).Tick() error` (holds Runtime.mu), `Advance(d) error`, `Clock()`, `BaseTick()`, `Tasks() []TaskStats{Name, Cycle, Priority, Programs, Runs, Overruns}`, `Runtime() *Runtime`, `SetIOBinder(*IOBinder)`
- `Overruns` exists in TaskStats but is never incremented yet; 23-02's free-running `Run(ctx)` owns it.
- `DefaultTaskName` = "PlcTask", `DefaultTaskCycle` = 10 ms.
- `cmd/stc` `loadProjectSpec(paths []string, defines map[string]bool) (interp.ProjectSpec, []diag.Diagnostic, error)`; `loadSimProject` is untouched for 23-03 to switch.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] IOBinder resolved paths through a single engine**
- **Found during:** Task 2
- **Issue:** `resolveOne` dereferenced `b.engine`, which is nil for a Project-level binder (panic).
- **Fix:** IOBinder now holds `interp` and a `progEnv(name)` lookup; `ScanCycleEngine.SetIOBinder` keeps the old single-program behaviour and `Project.SetIOBinder` resolves any project PROGRAM.
- **Files:** pkg/interp/iobind.go, pkg/interp/project.go
- **Commit:** 0caaa87

**2. [Rule 1 - Bug] Imported ST101 could not be checked or run (orchestrator-requested)**
- **Found during:** post-Task-3 run of `STC_SILD_DIR` ST101/ST301
- **Issue:** `stc check` reported undeclared SIZEOF, ADR, SHL, UINT_TO_WORD; the runtime then stopped on `undefined function: BOOL_TO_UINT`. ADR also only accepted bare identifiers, but the project uses `ADR(Modbus.brettakerfi_Read)`.
- **Fix:** built-ins added to checker and interpreter:
  - `SIZEOF(any)` returns UDINT. It also accepts a TYPE or FUNCTION_BLOCK name. Sizes: BOOL/BYTE/SINT/USINT 1, INT/UINT/WORD 2, DINT/UDINT/DWORD/REAL/TIME/DATE/TOD/DT 4, 64-bit types and pointers 8, STRING 81, WSTRING 162. Arrays, structs and FB instances sum their members.
  - `ADR(any)` returns POINTER TO BYTE. It now accepts GVL members, struct members and array elements, and dereferencing such a pointer reads and writes through the path.
  - `SHL`, `SHR`, `ROL`, `ROR` work within the operand's IEC width and sign-extend signed results.
  - Every missing BOOL, integer, bit-string and real `X_TO_Y` conversion is registered: UINT_TO_WORD, BOOL_TO_UINT, UINT_TO_BOOL, DWORD_TO_REAL and the rest. Integer results wrap to the target width.
  - WSTRING zero values now carry KindWSTRING.
- **Files:** pkg/types/builtin_sys.go, pkg/types/builtin.go, pkg/checker/check.go, pkg/interp/stdlib_sys.go, pkg/interp/interpreter.go, pkg/interp/ref_path.go, pkg/interp/fb_instance.go
- **Commit:** cfc5b38

## Verification

- `go test ./pkg/interp ./cmd/stc -run 'TestLoadProject|TestProjectTick|TestProjectAdvance|TestProjectIOBinder|TestLoadProjectSpec' -race -count=1` passes.
- `go test ./pkg/... ./cmd/... -count=1` passes everywhere.
- pkg/interp coverage is 98.7%, above the 95% gate.
- `grep -c time.Now pkg/interp/project.go` prints 0.
- `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla go test ./cmd/stc -run TestLoadProjectSild` loads ST101 and runs 1000 ticks without runtime errors.

## Known Issues / Deferred

- **ST301 does not load.** Its sources reference `FB_TwoWayConveyor` from SPB03.TcGVL and `ST_LineRecipe.stopDistanceFromEnd` from MAIN, and the project declares neither. `stc check` rejects it the same way. This is a project source problem, not an stc gap, so TestLoadProjectSild covers ST101 only. 23-04's ST301 gate needs a fixed project checkout or the right defines.
- **SIZEOF of strings assumes length 80.** Values do not carry their declared length, so a STRING(n) with n other than 80 reports the wrong size.

## Known Stubs

None.

## TDD Gate Compliance

Each task's tests and implementation went into a single feat commit. There is no separate RED test commit, because Task 1's fixture needed Tick to compile.

## Self-Check: PASSED

- The created files exist: pkg/interp/project.go, cmd/stc/project_load.go, pkg/interp/stdlib_sys.go and pkg/types/builtin_sys.go.
- The commits are in git log: 8be5d02, 0caaa87, f5ac404 and cfc5b38.
