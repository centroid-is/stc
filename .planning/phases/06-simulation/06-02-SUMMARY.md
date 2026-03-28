---
phase: 06-simulation
plan: 02
subsystem: simulation
tags: [simulation-engine, closed-loop, waveform-injection, plant-model, deterministic, cli]

requires:
  - phase: 06-simulation
    provides: "WaveformGenerator, PlantModel interface, MotorModel/ValveModel/CylinderModel"
  - phase: 04-interpreter
    provides: "ScanCycleEngine with Tick/SetInput/GetOutput, interp.Value"
provides:
  - "SimulationEngine orchestrating closed-loop scan cycles with waveforms and plant models"
  - "SimConfig, WaveformBinding, PlantBinding configuration types"
  - "SimResult/CycleRecord for simulation output recording"
  - "stc sim CLI command with text and JSON output"
  - "Value.MarshalJSON for clean JSON serialization"
affects: [simulation-config, test-runner, lsp]

tech-stack:
  added: []
  patterns: ["Closed-loop simulation: waveform inject -> scan -> output -> plant -> feedback next cycle", "RecordInterval for memory-efficient large simulation runs", "CLI waveform binding via colon-separated flag strings"]

key-files:
  created:
    - pkg/sim/engine.go
    - pkg/sim/result.go
    - pkg/sim/engine_test.go
    - cmd/stc/sim_cmd.go
    - cmd/stc/sim_cmd_test.go
    - cmd/stc/testdata/sim_test.st
  modified:
    - pkg/interp/scan.go
    - pkg/interp/value.go
    - cmd/stc/main.go

key-decisions:
  - "Plant outputs feed back as inputs on NEXT cycle (not same cycle) matching real PLC behavior"
  - "Added OutputNames/InputNames/Initialize accessors to ScanCycleEngine for simulation engine introspection"
  - "Value.MarshalJSON produces typed JSON (bool->bool, int->number, real->number, string->string)"
  - "CLI waveform binding format: INPUT_NAME:KIND:AMPLITUDE:FREQUENCY via colon-separated strings"

patterns-established:
  - "SimConfig holds all simulation parameters; engine is constructed once and Run() returns immutable result"
  - "Plant feedback stored in separate map, injected at cycle start before waveforms"
  - "RecordInterval defaults to 1 when 0 or negative"

requirements-completed: [SIM-01, SIM-02, SIM-03]

duration: 318s
completed: 2026-03-28
---

# Phase 06 Plan 02: Simulation Engine and CLI Summary

**Closed-loop SimulationEngine wiring waveforms and plant models into scan cycle loop, with stc sim CLI outputting text tables or JSON**

## Performance

- **Duration:** 318s (~5 min)
- **Started:** 2026-03-28T19:23:30Z
- **Completed:** 2026-03-28T19:28:48Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- SimulationEngine running closed-loop simulation with waveform injection, scan cycle execution, and plant model feedback
- Determinism verified: identical configs produce bit-identical float64 results across runs
- stc sim CLI command parsing ST files, running simulations with waveform flags, outputting text tables or JSON
- RecordInterval support for memory-efficient large simulations

## Task Commits

Each task was committed atomically:

1. **Task 1: SimulationEngine with closed-loop waveform + plant model integration** (TDD)
   - `cbb06d9` (test: add failing tests for SimulationEngine)
   - `4e64c22` (feat: implement SimulationEngine with closed-loop waveform + plant integration)
2. **Task 2: CLI stc sim command with text and JSON output**
   - `7ec7903` (feat: add stc sim CLI command with text and JSON output)

_TDD workflow: RED (failing tests) -> GREEN (implementation) for Task 1_

## Files Created/Modified
- `pkg/sim/engine.go` - SimulationEngine with WaveformBinding, PlantBinding, SimConfig, and Run() loop
- `pkg/sim/result.go` - CycleRecord and SimResult types for simulation output
- `pkg/sim/engine_test.go` - 4 tests: closed-loop motor, determinism, waveform-only, record interval
- `cmd/stc/sim_cmd.go` - stc sim CLI command with --cycles, --dt, --wave flags, text/JSON output
- `cmd/stc/sim_cmd_test.go` - 5 CLI integration tests: JSON, text, help, no-args, multiple waveforms
- `cmd/stc/testdata/sim_test.st` - Test fixture: SENSOR * 2.0 program
- `pkg/interp/scan.go` - Added OutputNames(), InputNames(), Initialize() accessors
- `pkg/interp/value.go` - Added MarshalJSON for clean typed JSON serialization
- `cmd/stc/main.go` - Registered newSimCmd() in root command

## Decisions Made
- Plant outputs feed back as inputs on the NEXT cycle (not same cycle), matching real PLC behavior where inputs are read at scan start
- Added OutputNames/InputNames/Initialize methods to ScanCycleEngine to allow simulation engine to introspect available I/O without running a Tick
- Value.MarshalJSON produces native JSON types (bool/number/string) instead of dumping all union fields
- CLI waveform binding uses colon-separated format (INPUT_NAME:KIND:AMPLITUDE:FREQUENCY) for simple flag parsing

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added Value.MarshalJSON for JSON output**
- **Found during:** Task 2 (CLI JSON output)
- **Issue:** interp.Value had no custom JSON marshaling; default would dump all union fields as zero values
- **Fix:** Added MarshalJSON method producing clean typed JSON values
- **Files modified:** pkg/interp/value.go
- **Verification:** JSON output tests pass with valid typed values
- **Committed in:** 7ec7903 (Task 2 commit)

**2. [Rule 3 - Blocking] Added OutputNames/InputNames/Initialize to ScanCycleEngine**
- **Found during:** Task 1 (SimulationEngine needs to iterate outputs)
- **Issue:** ScanCycleEngine had no way to enumerate output variable names; fields were unexported
- **Fix:** Added three exported accessor methods
- **Files modified:** pkg/interp/scan.go
- **Verification:** All interp and sim tests pass
- **Committed in:** 4e64c22 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 missing critical, 1 blocking)
**Impact on plan:** Both auto-fixes necessary for correct operation. No scope creep.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None - all functionality is fully wired with real data.

## Next Phase Readiness
- SimulationEngine and stc sim CLI ready for use
- Plant models not yet configurable via CLI flags (requires structured config file format, not in scope for this plan)
- Simulation results can be consumed by future test assertions or visualization tools
- All 06-simulation plans complete

## Self-Check: PASSED

All files verified present. All commit hashes verified in git log.

---
*Phase: 06-simulation*
*Completed: 2026-03-28*
