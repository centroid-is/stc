---
phase: 27-plant-scenarios-simulation-cli
plan: 02
subsystem: simulation
tags: [scenario, ethercat, plant, force, ecat-09]

requires:
  - phase: 27-01
    provides: Target/Stepper interfaces, Executor, SCN sentinel errors
  - phase: 22-symbol-tree-value-semantics
    provides: Runtime Get/Set/CheckSet/ToJSON
  - phase: 24/25/26
    provides: Network, IOBinder, device model stimulus APIs, Tc2_EtherCAT mocks
provides:
  - ecat.Network input forces (ForceInput/ReleaseInput/ReleaseAll/Forced), SlaveByName, ApplySlavePreset
  - interp.RuntimeOpts.Network, Runtime.SetIOBinder/IOBinder/Network, IOBinder.InputBinding/OutputBindings
  - scenario.ProjectSources, PlantSpec, BuildPlantSpec, (PlantSpec).New, Plant (implements Target), ParseIECInt
  - tests/ecat_fixtures/scenario/{ECT_Diag.st, MAIN.st, jam.toml}
affects: [27-03 stc sim --scenario and ST built-ins, 27-04 stc test project mode, 29 MCP sim tools]

tech-stack:
  added: []
  patterns:
    - "Scenario inputs are forced at the process image after device models and pseudo-inputs, so the IOBinder copies them into linked variables"
    - "Runtime-level scan hooks: binder preScan and services scan run once per Runtime.Tick regardless of program count"

key-files:
  created:
    - pkg/ecat/force.go
    - pkg/ecat/force_test.go
    - pkg/interp/runtime_ecat.go
    - pkg/interp/iobind_lookup.go
    - pkg/interp/iobind_lookup_test.go
    - pkg/scenario/plantspec.go
    - pkg/scenario/plant.go
    - pkg/scenario/plant_test.go
    - pkg/scenario/plant_target_test.go
    - pkg/scenario/plant_e2e_test.go
    - tests/ecat_fixtures/scenario/ECT_Diag.st
    - tests/ecat_fixtures/scenario/MAIN.st
    - tests/ecat_fixtures/scenario/jam.toml
  modified:
    - pkg/ecat/network.go
    - pkg/interp/runtime.go
    - pkg/interp/iobind.go
    - pkg/interp/ecat_services.go

key-decisions:
  - "Plant adapts the Phase 22 Runtime (Phase 23 Project not merged): Plant owns BaseTick (default 10 ms) and the clock; ProjectSources stands in for interp.ProjectSpec"
  - "RuntimeOpts.Network attaches the Tc2_EtherCAT mocks before RegisterFiles, because the Runtime instantiates FBs eagerly and SetNetwork after that would leave the stub FBs in place"
  - "Runtime.SetIOBinder resolves binding roots across every GVL and PROGRAM and runs the binder plus services scan once per Tick"
  - "Explicit AT %I addresses (not %I*) without TcLinkTo are SCN007, found by walking GVL/PROGRAM declarations"
  - "The fixture derives sNetId from the linked Dev1_AmsNetId via F_CreateAmsNetId because both demo masters share the default AmsNetId and '' is ambiguous with two masters"

requirements-completed: []  # ECAT-09 is shared with 27-03/27-04; marked at the phase gate

duration: 40min
completed: 2026-10-06
---

# Phase 27 Plan 02: Plant Target and Jam Fixture Summary

**Scenario stimuli now reach the real interpreter and EtherCAT device models through a `Plant` Target, and the jam scenario runs end to end with byte-identical reports.**

## Performance

- **Duration:** ~40 min
- **Completed:** 2026-10-06
- **Tasks:** 4/4
- **Files:** 13 created, 4 modified

## Accomplishments

- `Network.ForceInput` pins an input slot after device models and pseudo-inputs on every Step, until released. Values are truncated to BitLen and output slots are rejected (T-27-04). `ClearFaults` also releases forces.
- `SlaveByName` resolves names per D-06, trying exact, case-insensitive and short-prefix matches in that order. Errors list up to 5 candidates or name both masters (T-27-05). `ApplySlavePreset` implements the D-14 presets.
- `RuntimeOpts.Network` and `Runtime.SetIOBinder` give the Phase 22 Runtime one services scan per Tick, even with several PROGRAMs. `IOBinder.InputBinding` and `OutputBindings` give case-insensitive lookup.
- `Plant` implements every ECAT-09 action. set and link force bound input slots, other variables go through `Runtime.Set`, and trip, analog, slave_state, drive_fault and serial_peer drive the Phase 25/26 models. A wrong model returns `ErrUnknownSlave` naming the actual model.
- The fixture project polls FB_EcGetAllSlaveStates into `ECT_Diag.Device_1_Diag` with the FB_EcDeviceDiag rules. It also requests OP for the ATV320 once. All 8 assertions in jam.toml pass: set, ramp, trip, pulled slave, pre-fault and drive_fault.

## Task Commits

1. **Task 1: Network force overrides, slave lookup and state presets**: `15b8075`
2. **Task 2: IOBinder lookup, Runtime network attach and PlantSpec**: `20fc24c`
3. **Task 3: Plant implements scenario.Target**: `4cbedac`
4. **Task 4: Fixture project, jam scenario and end-to-end test**: `bd53e68`

## Interfaces for later plans

```go
type ProjectSources struct { LibraryFiles, Files []*ast.SourceFile; BaseTick time.Duration }
type PlantSpec struct { Sources ProjectSources; Topology *ecat.Topology; Bindings []ecat.Binding; Diagnostics []diag.Diagnostic }
func BuildPlantSpec(src ProjectSources, ioFiles []string) (PlantSpec, error)
func (s PlantSpec) New() (*Plant, error)
// Plant: BaseTick, Clock, Tick, Check, CheckPath, Apply, SetNumber, Read,
//        Runtime() *interp.Runtime, Network() *ecat.Network, IOBinder() *interp.IOBinder
func ParseIECInt(s string) (int64, error)
```

## Deviations from Plan

- **[Rule 3 - Blocking] Phase 23 is not merged.** No `interp.ProjectSpec`, `LoadProject` or `Project` exists on this branch. `PlantSpec` takes `scenario.ProjectSources` and `New` builds an `interp.Runtime`. The Plant keeps its own clock and a 10 ms default base tick. When Phase 23 lands, `New` can swap to `LoadProject` behind the same Plant API.
- **[Rule 3 - Blocking] Network attach before instantiation.** The plan called `SetNetwork` on an engine after loading. The Runtime instantiates FBs eagerly, so that order would leave FB_EcGetAllSlaveStates as the empty stub. `RuntimeOpts.Network` installs the mocks first. `SetNetwork` now shares the `attachNetwork` and `detachNetwork` helpers.
- **[Rule 2 - Correctness] Runtime-level binder.** `Runtime.SetIOBinder` resolves PROGRAM roots across all programs and runs the binder once per Tick, instead of attaching to one engine. It also moves the services scan count to the Runtime, which the plan asked for.
- **Test layout.** pkg/ecat cannot import pkg/ecat/devices, so the force-over-model test uses an in-package device that zeroes its inputs. The real DigitalIO and device paths are covered in pkg/scenario.
- **Fixture NetId.** `sNetId ''` cannot resolve with two masters loaded, so MAIN uses `F_CreateAmsNetId(ECT.Dev1_AmsNetId)`, which resolves to Device 1. The jam scenario gained pre-checks for the healthy slave and the healthy drive.

## Known Stubs

None.

## Verification

- `go test ./pkg/scenario ./pkg/ecat/... ./pkg/interp -count=1 -race -cover`: all pass. Coverage is pkg/scenario 98.3%, pkg/ecat 99.4%, pkg/ecat/devices 100% and pkg/interp 98.9%.
- `go test ./pkg/scenario -run TestPlantE2E -count=2`: passes, and two runs marshal byte-identical reports.
- `go test ./... -count=1`: all packages pass. `go vet` is clean on the touched packages, and `time.Now` does not appear in pkg/scenario.

## Self-Check: PASSED

- All 13 created files exist, and commits 15b8075, 20fc24c, 4cbedac and bd53e68 are on gsd/phase-27-scenarios.
