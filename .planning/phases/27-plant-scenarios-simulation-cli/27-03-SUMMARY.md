---
phase: 27-plant-scenarios-simulation-cli
plan: 03
subsystem: simulation
tags: [scenario, stc-sim, ethercat, builtins, ecat-10, devx-01]
requires:
  - phase: 27-02
    provides: PlantSpec, Plant (Target), IOBinder lookup, jam fixture
  - phase: 23
    provides: interp.Project/LoadProject, projectSetup, project-mode stc sim
provides:
  - "`stc sim <project> --io <glob> --scenario x.toml [--cycles N]` with JSON/text report"
  - interp.ProjectSpec.Network (Tc2_EtherCAT mocks on a Project) and one services scan per Project.Tick
  - scenario.Plant over interp.Project (Plant.Project())
  - scenario.RegisterBuiltins / Session / MaxRunCycles
  - Interpreter.SaveCallState, Network.WcBad
  - stc.toml library_paths loaded for .st projects
affects: [27-04 stc test --project --io, 29 MCP sim tools]
tech-stack:
  added: []
  patterns:
    - "Every project-mode run (sim, serve) builds through scenario.BuildPlantSpec + (PlantSpec).New"
    - "ST built-ins take ToJSON-form values and drive the Plant through Apply, like scenario steps"
key-files:
  created:
    - cmd/stc/sim_scenario.go
    - cmd/stc/sim_scenario_test.go
    - pkg/scenario/builtins.go
    - pkg/scenario/builtins_test.go
    - pkg/checker/adr_pointer_test.go
    - tests/ecat_fixtures/scenario/stc.toml
  modified:
    - cmd/stc/sim_cmd.go
    - cmd/stc/project_run.go
    - cmd/stc/project_load.go
    - pkg/interp/project.go
    - pkg/interp/interpreter.go
    - pkg/scenario/plantspec.go
    - pkg/scenario/exec.go
    - pkg/ecat/network.go
    - pkg/checker/check.go
    - pkg/checker/check_calls.go
key-decisions:
  - "Plant wraps the Phase 23 interp.Project (task schedule, AT I/O, persistence) instead of a bare Runtime; ProjectSources removed in favour of interp.ProjectSpec"
  - "projectSetup always builds a Plant, so stc sim --io and stc serve --io now get the Tc2_EtherCAT mocks (attachECat removed)"
  - "Unfired steps stay SCN010 warnings: --cycles shorter than the scenario passes with warnings"
  - "Binder errors are SIM001 warnings (Phase 23 treats them as warnings too)"
  - "ADR() (POINTER TO BYTE) is accepted for any typed POINTER TO target, as TwinCAT does"
requirements-completed: []  # ECAT-09/ECAT-10/DEVX-01 are marked at the phase gate (27-04)
duration: 75min
completed: 2026-10-06
---

# Phase 27 Plan 03: stc sim --scenario and ST Built-ins Summary

**`stc sim` runs TOML scenarios against the whole project and EtherCAT network with a deterministic JSON/text report, and ST code can drive the same Plant through SET/GET/SIM_*/RUN_CYCLES built-ins.**

## Performance

- **Duration:** ~75 min, including the rebase onto Phase 23
- **Completed:** 2026-10-06
- **Tasks:** 3/3, plus one refactor commit and one checker fix
- **Files:** 6 created, 10 modified

## Accomplishments

- The branch is rebased on main with Phase 23. The IOBinder conflict was resolved by keeping main's codec and program-env closure. `Runtime.SetIOBinder` now sets those fields.
- The Plant now wraps `interp.Project`. `ProjectSpec.Network` installs the Tc2_EtherCAT mocks before instantiation, and `Project.Tick` runs one services scan per tick that runs a task.
- In `stc sim` project mode, `--scenario` validates the scenario first, then runs it. Validation errors print SCN diagnostics and exit 1 before any tick. `--cycles` overrides the scenario length only when the flag is given. `--set` applies before the run and `--get` reads after it. `--io` accepts globs, expanded in sorted order.
- The JSON keeps every Phase 23 key and adds four. `scenario` holds the Report. `outputs` holds every TcLinkTo output leaf. `ethercat` lists slaves that are not in OP, or whose link or WcState is bad. `diagnostics` merges ECAT010, SIM001 and SCN entries, sorted. Two identical runs print byte-identical JSON.
- The text output prints the project summary, then the scenario report, then the outputs, ethercat and diagnostics sections.
- `RegisterBuiltins` provides SET, GET, SIM_SET_LINK, SIM_TRIP, SIM_SLAVE_STATE, SIM_ANALOG, SIM_DRIVE_FAULT, SIM_SERIAL_PEER, SIM_RAMP and RUN_CYCLES, and overrides ADVANCE_TIME.
- SIM_RAMP reuses the executor's ramp code through a shared `rampSet`. RUN_CYCLES accepts 0 to 10 000 000 and restores the caller's dt and call depth.

## Task Commits

0. **Rebase adaptation: Plant wraps interp.Project**: `a118369`
1. **Fix: accept ADR() for typed POINTER TO**: `e16c4f1`
2. **Task 1: --scenario in stc sim project mode**: `d67888f`
3. **Task 2: report outputs, ethercat, diagnostics, determinism**: `f0354ed`
4. **Task 3: ST built-in library over a Plant**: `fc05f6d`

## Interfaces for 27-04

```go
func RegisterBuiltins(in *interp.Interpreter, p *Plant) *Session // in = p.Runtime().Interpreter()
func (s *Session) RunCycles(n int64) error
const MaxRunCycles = 10_000_000
func (p *Plant) Project() *interp.Project
func (in *interp.Interpreter) SaveCallState() (restore func())
// cmd/stc: projectSetup(...) -> projectRunner{P, Plant, Binder, ...}; buildPlant(spec, ioFiles); expandIOGlobs
```

- ST bodies that call the built-ins must run outside the Runtime mutex. In practice, call `ExecStatements` directly and not from inside `Project.Tick`. Otherwise Get, Set and Tick deadlock.
- GET returns enums as their STRING name.
- An unknown member under a known root reads as zero, which is the tolerant runtime behaviour. Only an unknown root makes GET or SIM_RAMP fail.
- With a 10 ms tick, `SIM_RAMP(p, 0, 10, T#50ms)` then `RUN_CYCLES(5)` reads 8.0. One more cycle reads 10.0. This matches the executor, which writes the ramp before each tick.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Plant adapted to the Phase 23 Project**
- **Found during:** Rebase
- **Issue:** 27-02 built the Plant on a bare Runtime because Phase 23 was not merged. Main's attachECat created the network after LoadProject, so the Tc2_EtherCAT FB mocks were never installed for `stc sim --io`.
- **Fix:** Added `ProjectSpec.Network` and a services scan in Project.Tick. The Plant now delegates Tick, Clock and BaseTick to the Project. projectSetup always builds a Plant, and attachECat was removed.
- **Behaviour change:** A failing task no longer stops the clock. This follows Phase 23 semantics, and the 27-02 test was updated to match.
- **Commit:** a118369, d67888f

**2. [Rule 3 - Blocking] Checker rejected ADR() into typed pointers**
- **Found during:** Task 1
- **Issue:** Once .st projects loaded the stc.toml stubs, the checker rejected the fixture's `pStateBuf := ADR(aStateBuf)`. TwinCAT accepts this.
- **Fix:** POINTER TO BYTE now converts to any POINTER TO in assignments, input arguments and FB call arguments. Mismatches between typed pointers are still errors.
- **Commit:** e16c4f1

**3. [Rule 2 - Correctness] Network.WcBad getter**
- **Found during:** Task 2
- **Issue:** The `ethercat` report needs each slave's working-counter state, and no getter existed.
- **Fix:** Added `Network.WcBad` with a test.
- **Commit:** d67888f

**Other notes**

- `--cycles 5` on jam.toml exits 0 with eight SCN010 warnings. The 27-01 semantics treat unfired steps as warnings, so the plan's implied failure does not apply.
- The fixture stc.toml uses a relative library path. The CLI tests rewrite it to an absolute path in their temp copy.

## Known Stubs

None.

## Verification

- `go test ./cmd/stc -run 'TestSimScenario' -count=2`: pass. The two runs are compared byte for byte.
- `go test ./pkg/scenario -race -count=1 -cover`: pass at 98.6%.
- `go test ./... -count=1`: all packages pass.
- Coverage is pkg/interp 98.7%, pkg/checker 98.5% and pkg/ecat 99.4%.

## Self-Check: PASSED
