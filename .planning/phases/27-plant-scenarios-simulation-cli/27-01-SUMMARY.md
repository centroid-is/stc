---
phase: 27-plant-scenarios-simulation-cli
plan: 01
subsystem: simulation
tags: [scenario, toml, executor, determinism, ecat-09]

requires:
  - phase: 22-symbol-tree-value-semantics
    provides: Runtime Tick/Get/Set path grammar that a later Target adapter wraps
provides:
  - pkg/scenario TOML schema, Parse/Load with SCN001-SCN005 diagnostics
  - Stepper and Target interfaces, deterministic Executor (Prepare/Run/Length)
  - Report/StepResult/AssertionResult JSON types with Text output
affects: [27-02 Plant target, 27-03 stc sim --scenario and ST built-ins, 27-04 stc test project mode, 29 MCP sim tools]

tech-stack:
  added: []
  patterns:
    - "Target-independent executor: scenario core never imports interp/ecat; plants adapt via the Target interface"
    - "Typed sentinel errors (ErrUnknownSlave/ErrUnknownPath/ErrNoNetwork) classify target failures into SCN codes"

key-files:
  created:
    - pkg/scenario/scenario.go
    - pkg/scenario/exec.go
    - pkg/scenario/report.go
    - pkg/scenario/scenario_test.go
    - pkg/scenario/exec_test.go
    - pkg/scenario/report_test.go
    - pkg/scenario/testdata/valid.toml
    - pkg/scenario/testdata/bad_trigger.toml
  modified: []

key-decisions:
  - "Target embeds a Stepper {BaseTick, Clock, Tick} so Phase 22 Runtime+ecat.Network now, or Phase 23 Project later, can drive scenarios without executor changes"
  - "Step entries decode as []map[string]any and are validated key by key; MetaData.Undecoded is used only for keys outside [[step]]"
  - "Integer expects compare exactly by decimal rendering; any float operand compares within tol; strings strip an Enum#/Enum. prefix and compare case-insensitively"
  - "Ramp rounding for integer targets is left to Target.SetNumber (D-08 rounding belongs to the plant)"

patterns-established:
  - "Executor order per Tick k: fire due steps (due, file order) -> apply ramps (registration order) -> Tick -> evaluate pending expects"

requirements-completed: []  # ECAT-09 is shared with 27-02..27-04; marked complete at the phase gate

duration: 25min
completed: 2026-10-06
---

# Phase 27 Plan 01: Scenario Model, Parser and Deterministic Executor Summary

**TOML plant scenarios parse into typed steps with SCN001-SCN010 diagnostics and run tick-by-tick against any `Target`, producing byte-identical reports on every run.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-10-06
- **Tasks:** 3/3
- **Files created:** 8

## Accomplishments

- `Parse`/`Load` cover every ECAT-09 action kind (set, link, analog, trip, slave_state, drive_fault, ramp in both forms, serial_peer) plus expect, with SCN001 syntax lines from `toml.ParseError`, SCN002 unknown keys named per step, SCN003 trigger errors, SCN004 shape errors and SCN005 range errors. The D-11 limits are enforced, including the 1 MiB `io.LimitReader` cap and the 10 000-step cap checked before decoding.
- The `Executor` schedules steps by due Tick (`cycle = N`, or `ceil(at / BaseTick)`) and fires them in file order. It interpolates ramps on the sim clock, cancels ramps on a later set/link/analog of the same target and evaluates expects with the `within` tolerance. It applies the D-09 run-length rules, SCN010 for unfired steps and "run ended at cycle N" for pending expects.
- `Prepare` validates every action and expect path before the first Tick (SCN006/SCN007), and `Run` refuses to start on errors with `ErrPrepareFailed`. Apply errors become SCN008 and the run continues. A Tick error stops the run and is returned.
- `Report` has snake_case JSON tags and `diag.Diagnostic` passthrough, plus `Failed()` and `Text()`.

## Task Commits

1. **Task 1: Scenario types, Parse/Load and validation diagnostics**: `68f20f5`
2. **Task 2: Deterministic Executor with ramps and expects**: `b952439`
3. **Task 3: Report JSON shape and coverage**: `86caae8`

## Interfaces for later plans

```go
type Stepper interface { BaseTick() time.Duration; Clock() time.Duration; Tick() error }
type Target interface {
    Stepper
    Check(a Action) error; CheckPath(path string) error
    Apply(a Action) error; SetNumber(path string, v float64) error
    Read(path string) (any, error)
}
```

Also exported: `Equal`, `Describe` and `(*Executor).Length`.

## Deviations from Plan

- **[Rule 3 - Blocking] Phase 23 is not merged.** The plan says to execute after Phases 23 and 26 merge. Per the orchestrator, this plan runs before Phase 23 lands, so no `Project`/`LoadProject` dependency exists. A `Stepper` sub-interface is embedded in `Target` so 27-02 can adapt Phase 22 `interp.Runtime` plus `ecat.Network` now and Phase 23 `Project` later. pkg/scenario imports only `diag` and `source`.
- **Report structs created in Task 2.** The plan created them in exec.go and moved them in Task 3. They went straight into report.go because the executor needs them, and Task 3 added `Text` and the JSON tests.
- **Additions.** `ErrPrepareFailed`, `Describe`, `Equal` and `Length` are extra exports used by the tests and useful to 27-03.

## Known Stubs

None.

## Verification

- `go test ./pkg/scenario -count=1 -race -cover`: ok, coverage 98.4%
- `go vet ./pkg/scenario` is clean, and `gofmt -l pkg/scenario` is empty
- `grep -c time.Now pkg/scenario/*.go` returns 0 for all files
- `go build ./...` succeeds after the rebase on main

## Self-Check: PASSED

- All 8 created files exist, and commits 68f20f5, b952439 and 86caae8 are present on gsd/phase-27-scenarios.
