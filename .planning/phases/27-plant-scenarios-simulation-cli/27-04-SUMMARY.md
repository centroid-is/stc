---
phase: 27-plant-scenarios-simulation-cli
plan: 04
subsystem: testing
tags: [stc-test, scenario, plant-mode, ethercat, devx-01, ecat-09, ecat-10, phase-gate]
requires:
  - phase: 27-03
    provides: scenario.RegisterBuiltins, Plant over interp.Project, stc sim --scenario, expandIOGlobs
  - phase: 23
    provides: loadProjectSpec, Runtime.Interpreter, Interpreter.GlobalParent
provides:
  - "`stc test <dir> --project <.st files|dir|x.tsproj> --io <Device*.xml>` plant mode"
  - stctesting.RunOpts.Plant (fresh scenario.Plant per TEST_CASE)
  - tests/ecat_fixtures/scenario/scenario_test.st (DEVX-01 acceptance)
  - tests/ecat_fixtures/scenario/st301_jam.toml + env-gated TestScenarioST301
  - scenario reference docs (ETHERCAT_SIMULATION, CLI_REFERENCE, TESTING_GUIDE)
  - signed 27-VALIDATION.md
affects: [29 MCP sim tools]
tech-stack:
  added: []
  patterns:
    - "Plant-mode test bodies run via ExecStatements on the plant interpreter, outside Project.Tick"
    - "Plant mode is chosen per run: any --io, or --project values that are not a single .tsproj/.plcproj"
key-files:
  created:
    - pkg/testing/runner_project.go
    - pkg/testing/runner_project_test.go
    - cmd/stc/test_project_test.go
    - tests/ecat_fixtures/scenario/scenario_test.st
    - tests/ecat_fixtures/scenario/st301_jam.toml
    - tests/scenario_st301_test.go
  modified:
    - pkg/testing/runner.go
    - cmd/stc/test_cmd.go
    - cmd/stc/project_mode_test.go
    - pkg/scenario/builtins.go
    - pkg/scenario/builtins_test.go
    - docs/CLI_REFERENCE.md
    - docs/TESTING_GUIDE.md
    - docs/ETHERCAT_SIMULATION.md
    - .planning/phases/27-plant-scenarios-simulation-cli/27-VALIDATION.md
key-decisions:
  - "A single .tsproj/.plcproj without --io keeps the Phase 19 import mode; plant mode needs --io or .st project sources (keeps existing stc test --project users working)"
  - "Plant mode branches per file in RunWithOpts (runProjectFile) instead of inside executeTestCase; runner.go stays a small diff"
  - "Plant.New failures fail each TEST_CASE with 'plant initialisation: ...' instead of aborting the run"
  - "SIM_ANALOG without a unit writes the raw count"
requirements-completed: [DEVX-01, ECAT-09, ECAT-10]
duration: 45min
completed: 2026-10-06
---

# Phase 27 Plan 04: stc test Plant Mode and Phase Gate Summary

**`stc test --project ... --io Device*.xml` runs each TEST_CASE on a fresh simulated plant with SET/GET/SIM_*/RUN_CYCLES. The real ST301 jam scenario passes deterministically, and the Phase 27 gate is signed off.**

## Performance

- **Duration:** ~45 min
- **Completed:** 2026-10-06
- **Tasks:** 4 of 4, plus one bug-fix commit
- **Files:** 6 created, 9 modified

## Accomplishments

- `RunOpts.Plant` switches the runner to plant mode. Every TEST_CASE calls `PlantSpec.New()` and runs on the plant's interpreter in an env under `GlobalParent()`. Assertions, the scenario built-ins and the plant ADVANCE_TIME are registered there. A nil Plant leaves the runner unchanged.
- In plant mode a test file holding a FUNCTION, FUNCTION_BLOCK, TYPE, INTERFACE or GVL fails every case. The error names the declarations and says to declare them in the project (D-18).
- `stc test` gains repeatable `--project` and `--io` flags with glob expansion. The PlantSpec is built once through `loadProjectSpec` and `BuildPlantSpec`. `--io` without `--project` is a usage error. Without `--io`, the SIM_* built-ins fail with "no --io network loaded". The JSON and JUnit shapes are unchanged.
- `scenario_test.st` passes 8 of 8 on the fixture project with Demo Device 1/2. It covers variable and link-path input, the EL9222 trip, a removed slave in ECT_Diag, per-case isolation, a drive fault, SIM_RAMP and ADVANCE_TIME.
- ST301 runs behind STC_SILD_DIR. `st301_jam.toml` sets an EL1008 input by link path, trips `ST301.A1.02 (EL9222-5500)` channel 1 and pulls `ST301.A1.03 (EL1008)`. All 4 assertions pass, the removal is seen at cycle 1504, and two runs print byte-identical JSON in 51.6 s.
- The docs now cover the scenario file reference, `stc sim --scenario` with its JSON keys and exit codes, `stc test --project/--io`, and the built-in table.

## Task Commits

1. **Task 1: runner project mode and stc test --project/--io**: `97b541d`
2. **Task 2: DEVX-01 acceptance tests and env-gated ST301 scenario**: `98f9394`
3. **Fix: SIM_ANALOG default unit**: `a07f452`
4. **Task 3: documentation**: `29f6d54`
5. **Task 4: validation sign-off**: `3f3833e`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] SIM_ANALOG without a unit failed**
- **Found during:** Task 3, while checking the docs against the code
- **Issue:** The signature marks the unit optional, but the plant rejected an empty unit as "unknown analog unit".
- **Fix:** The unit defaults to `raw`. TestBuiltinsAnalogDefaultUnit covers it.
- **Commit:** a07f452

**2. [Plan correction] The ST301 removed-slave index is Device_1_Diag[4], not [3]**
- **Found during:** Task 2
- **Issue:** FB_EcDeviceDiag fills `Device_1_Diag[i]` from FB_EcGetAllSlaveStates entry i - 1. That entry counts the EK1200 coupler `ST301.A1.00` as bus position 0. ST301's `ECT_Diag.Device_1_SlaveInfo` omits the coupler, so the EL1008 sits at SlaveInfo index 3 but Diag index 4. The scenario expects `Device_1_Diag[4]`.
- **Flag for the ST301 owners:** On a real PLC, SlaveInfo[i] and Diag[i] describe different slaves for every Device 1 entry. This is worth checking in the generator that writes ECT_Diag.

**3. [Compatibility] A lone .tsproj without --io keeps the import mode**
- **Issue:** D-18 makes `--project x.tsproj` plant mode even without `--io`. That would break the existing `stc test --project x.tsproj` behaviour and TestTestProjectDemo, where test files declare helpers next to imported project code.
- **Choice:** Plant mode starts on any `--io`, or on `--project` values that are `.st` files, directories or several paths. A `.tsproj` with `--io` runs in plant mode. The behaviour is documented in CLI_REFERENCE.
- **Test change:** TestTestProjectErrors no longer expects a lone `.st` `--project` to be rejected. It now checks that this selects plant mode.

**Other notes**

- SET on a TcLinkTo-bound input forces the slot (D-13), so GET reads it back only after the next scan. The acceptance test reads after RUN_CYCLES and checks an unlinked variable for an immediate read-back.
- TestEmptyFBCall passes on the rebased branch, so the coverage gate ran without a skip.

## Known Stubs

None.

## Verification

- `go vet ./...`: clean.
- `go test ./... -count=1`: all packages pass.
- `bash scripts/coverage-gate.sh`: PASS, 97.08% total.
- pkg/scenario coverage is 98.6%, and runner_project.go is 96.8% to 100% per function.
- `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla go test ./tests -run TestScenarioST301 -count=1 -v`: PASS in 51.6 s. Without the variable the test skips.

## Self-Check: PASSED
