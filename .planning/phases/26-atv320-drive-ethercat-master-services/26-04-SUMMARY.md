---
phase: 26-atv320-drive-ethercat-master-services
plan: 04
subsystem: interp
tags: [ethercat, atv320, fb_atv320, fb_ecdevicediag, e2e, gate, sizeof, shl, conversions]

requires:
  - phase: 26-atv320-drive-ethercat-master-services
    provides: ATV320 model (26-01), object dictionary and network services (26-02), Tc2_EtherCAT mocks and SetNetwork (26-03)
  - phase: 22-symbol-tree-value-semantics
    provides: RegisterFiles, constant-expression array bounds (merged from main)
  - phase: 21-twincat-project-import-library-stubs
    provides: twincat.Import / ParseModel for the real SVNCoreComponents project
provides:
  - CI end-to-end test of configure, run, fault and diag over IOBinder plus mocks (trimmed configurator)
  - Env-gated gate running the unmodified FB_ATV320 and FB_EcDeviceDiag on the simulator
  - Runtime SIZEOF, SHL/SHR/ROL/ROR and the typed X_TO_Y integer conversion family
  - pkg/ecat/devices package documentation and docs/ETHERCAT_SIMULATION.md sections
affects: [23 merge (SIZEOF), 27 scenario scripting, 29 ST301 sim]

tech-stack:
  added: []
  patterns:
    - "Rig order: NewScanCycleEngine, SetNetwork, RegisterFiles, Resolve links, NewIOBinder on the same net"
    - "Real-project gates import via twincat.Import and skip unless STC_SILD_DIR is set"

key-files:
  created:
    - tests/ecat_fixtures/atv320_mini.st
    - pkg/interp/ecat_e2e_test.go
    - pkg/interp/ecat_sild_test.go
    - pkg/interp/stdlib_sizeof.go
    - pkg/interp/stdlib_sizeof_test.go
    - pkg/interp/stdlib_bitshift.go
    - pkg/interp/stdlib_bitshift_test.go
    - pkg/interp/stdlib_convert_any.go
    - pkg/ecat/devices/doc.go
  modified:
    - pkg/interp/interpreter.go
    - pkg/interp/init_value.go
    - pkg/interp/ecat_services.go
    - pkg/ecat/devices/atv320.go
    - docs/ETHERCAT_SIMULATION.md
    - .planning/phases/26-atv320-drive-ethercat-master-services/26-VALIDATION.md

key-decisions:
  - "The gate imports the real SVNCoreComponents.plcproj with twincat.Import instead of the flattened probe, so GVL names like EcDiagParam resolve"
  - "SIZEOF uses the packed layout of the pointer codec, so SIZEOF(buf) equals the bytes ADR(buf) exposes; STRING without a known length counts 81"
  - "Generated X_TO_Y conversions never replace a dedicated implementation and wrap to the target width themselves"

requirements-completed: [ECAT-05, ECAT-08]

duration: 35min
completed: 2026-10-06
---

# Phase 26 Plan 04: Phase Gate Summary

**The unmodified SVNCoreComponents FB_ATV320 configures the simulated ATV320 from PreOp to cfgReady in 107 scans, runs it to RFR 500 and reports fault 16, while FB_EcDeviceDiag fills its records. A trimmed configurator proves the same flow in CI.**

## Performance

- **Duration:** about 35 min
- **Completed:** 2026-10-06
- **Tasks:** 3, plus the merge of main
- **Files modified:** 15

## Accomplishments

- Merged main (Phases 21, 22, 24, 28-core) into the phase branch. Conflicts in fb_instance.go and scan.go were resolved to main's shared `instantiateVar` path. That path and `typeCtx.fbInstance` in init_value.go now use `stdFBFactory`, so the mocks still win over stub declarations. `sortedKeys` in ecat_services.go was renamed `sortedTypeNames` to avoid a clash with main's coerce.go.
- `TestEcatE2EConfigureRunFault` covers the full flow with FB_MiniATV320. It checks PreOp boot, cfgReady after 16 scans, OP, HSP and ACC written over SDO, one EEPROM save, ETA 16#27, a monotonic RFR ramp to 500, HMIS run, then q_xError with LFT 16 and a reset.
- `TestEcatE2EDiag` checks that the all-slave states, master state and all-slave CRC blocks are busy for exactly 2 scans. It then checks deviceState 8, master state 8 and the injected CRC sum of 7.
- `TestEcatSildATV320Gate` passes with `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla` and skips without it. The real FB_ATV320 passes cfgWriteParams and reaches cfgReady after 107 scans, which is 5.35 s against the 5 s tConfigWindow. It shows q_xReady, HSP 1500 synced from ST_MotorParams, operation enabled, RFR 500 and q_rFreq 50.0. HMI.p_stat_State reads run and HMI.p_stat_LastFault reads 16 after InjectFault. FB_EcDeviceDiag sets aDiag[1] to deviceState 8 with bOk, nMasterDevState 8 and bAllOp, and bBusy was seen.
- Coverage gate passes: pkg/interp 98.99%, total 96.92%. pkg/ecat is at 99.5% and pkg/ecat/devices at 100%. ecat_services.go, ecat_fbs.go and the three new stdlib files are at 100%. `go test ./...` is fully green and go vet is clean.

## Task Commits

0. **Merge main** - `e1ee3ef`
1. **SIZEOF (Rule 3)** - `615d0c0` (feat)
2. **Task 1: CI end-to-end** - `748a749` (test)
3. **Task 2: real FB_ATV320 gate, SHL family and conversions** - `813b6d0` (feat)
4. **Task 3: docs and validation** - `ac8407c` (docs)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Runtime SIZEOF**
- **Found during:** Task 1
- **Issue:** SIZEOF was still undefined at runtime after merging main. FB_Parameter, FB_EcDeviceDiag and the mini fixture all use it.
- **Fix:** Added `pkg/interp/stdlib_sizeof.go` with a one-line dispatch next to ADR in interpreter.go. Phase 23-01 adds SIZEOF too, so the orchestrator must keep one of the two on merge.
- **Commit:** 615d0c0

**2. [Rule 3 - Blocking] UINT_TO_WORD and the typed conversion family**
- **Found during:** Task 2
- **Issue:** FB_ATV320 calls UINT_TO_WORD, which did not exist.
- **Fix:** `stdlib_convert_any.go` registers every missing X_TO_Y among BOOL, the integer types and the bit-string types, plus REAL and LREAL targets. Results wrap to the target width.
- **Commit:** 813b6d0

**3. [Rule 3 - Blocking] SHL, SHR, ROL, ROR**
- **Found during:** Task 2
- **Issue:** FB_EcDeviceDiag calls SHL, and no shift or rotate function existed.
- **Fix:** `stdlib_bitshift.go` adds all four. The width comes from IN's type, with 32 bits when the type is unknown.
- **Commit:** 813b6d0

**4. [Rule 3 - Blocking] Merge integration**
- **Issue:** Main's new `instantiateVar` and `typeCtx.fbInstance` read StdlibFBFactory directly, which would bypass the engine-local mocks. The two branches also both defined `sortedKeys`.
- **Fix:** Both call sites now use `stdFBFactory`, and the ecat helper is renamed `sortedTypeNames`.
- **Commit:** e1ee3ef

**5. [Plan adjustment] Real project instead of the flattened probe**
- **Issue:** In the flattened probe, every GVL takes the file name, so `EcDiagParam.MAX_EC_SLAVES` would not resolve.
- **Fix:** The gate imports `SVNCoreComponents.plcproj` read-only through twincat.Import and ParseModel. The sildarvinnsla tree is unchanged.

**6. [Plan adjustment] Run command path**
- The gate turns on auto with `i_xAuto := TRUE` and sets `HMI.p_cfg_AutoFreq := 50.0` through the instance env. FB_ATV320 itself is unchanged.

## Deferred Issues

- `gofmt -l pkg cmd` lists 27 files that came from main, for example pkg/lexer/lexer.go and cmd/stc/main.go. None was touched by this plan. The plan's check of an empty gofmt list holds for every file this plan changed.

## Notes for the orchestrator

- **Reconcile with 23-01:** Phase 23-01 also adds SIZEOF and ADR. Keep one implementation. The tests in stdlib_sizeof_test.go pin the packed sizes the Tc2 mocks need.
- **Checker coverage:** `stc check` may not yet know SHL/SHR/ROL/ROR or the generated X_TO_Y names. This plan only added the runtime side.

## Self-Check: PASSED

- Files exist: tests/ecat_fixtures/atv320_mini.st, pkg/interp/ecat_e2e_test.go, pkg/interp/ecat_sild_test.go, pkg/interp/stdlib_sizeof.go, pkg/interp/stdlib_bitshift.go, pkg/interp/stdlib_convert_any.go, pkg/ecat/devices/doc.go
- Commits exist: e1ee3ef, 615d0c0, 748a749, 813b6d0, ac8407c
