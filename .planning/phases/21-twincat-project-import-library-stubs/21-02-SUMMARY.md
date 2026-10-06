---
phase: 21-twincat-project-import-library-stubs
plan: 02
subsystem: stdlib, vendor stubs
tags: [twincat, beckhoff, stubs, embed, ethercat, modbus, serialcom]

requires:
  - phase: 21-01
    provides: parser/checker fixes (PROPERTY modifiers, fb() calls, SEMA039)
provides:
  - Beckhoff stubs for Tc2_EtherCAT, Tc2_ModbusSrv, Tc2_SerialCom, Tc3_Module, Tc3_IPCDiag
  - Tc2_System and Tc2_Utilities additions (AMS types, PVOID, MEMCMP, F_CreateAmsNetId, RTC, FB_LocalSystemTime, SYSTEMTIME_TO_DT)
  - stdlib.VendorFS (embedded stubs) and package stdlib/beckhoff with FS, Closure, IsBuiltin, Libraries
affects: [21-03, 21-04, 21-05, 21-06, 22]

tech-stack:
  added: []
  patterns:
    - "Embed lives in an ancestor package (stdlib) because Go treats stdlib/vendor/ as a vendor tree"
    - "Stub closures analysed as user code so stub typos surface as errors"

key-files:
  created:
    - stdlib/embed.go
    - stdlib/beckhoff/beckhoff.go
    - stdlib/beckhoff/beckhoff_test.go
    - stdlib/beckhoff/testdata/usage.st
    - stdlib/vendor/beckhoff/tc2_ethercat.st
    - stdlib/vendor/beckhoff/tc2_modbussrv.st
    - stdlib/vendor/beckhoff/tc2_serialcom.st
    - stdlib/vendor/beckhoff/tc3_module.st
    - stdlib/vendor/beckhoff/tc3_ipcdiag.st
  modified:
    - stdlib/vendor/beckhoff/common_types.st
    - stdlib/vendor/beckhoff/tc2_system.st
    - stdlib/vendor/beckhoff/tc2_utilities.st
    - stdlib/mocks/beckhoff/ads_mock.st
    - .github/workflows/ci.yml
    - Makefile

key-decisions:
  - "RTC is declared in tc2_system.st (Beckhoff declares it in Tc2_Utilities 3.79), so a project referencing only Tc2_Standard and Tc2_System resolves it; every closure includes tc2_system"
  - "Closure API lives in package github.com/centroid-is/stc/stdlib/beckhoff, not stdlib/vendor/beckhoff, because packages under a vendor directory cannot be imported by full path"
  - "beckhoff.FS is an fs.FS rooted at the stub directory; read with fs.ReadFile(beckhoff.FS, name)"
  - "CI and make test run ./stdlib/vendor/... explicitly because ./... skips vendor directories"

patterns-established:
  - "New stub file: add .st to stdlib/vendor/beckhoff and an entry in the stubs map; TestStubFilesAllReachable fails otherwise"

requirements-completed: []
requirements-contributed: [IMPT-04]

duration: 7min
completed: 2026-10-06
---

# Phase 21 Plan 02: Beckhoff library stubs and dependency closure Summary

**Beckhoff stubs for Tc2_EtherCAT, Tc2_ModbusSrv, Tc2_SerialCom and Tc3 libraries are embedded, resolvable by library name with a deps-first closure, and every closure plus a full usage fixture checks with zero errors.**

## Performance

- **Duration:** about 7 min
- **Started:** 2026-10-06T06:38:00Z
- **Completed:** 2026-10-06T06:45:00Z
- **Tasks:** 3
- **Files modified:** 15

## Accomplishments

- Tc2_System gains AMSNETID, T_AmsNetIdArr (BYTE arrays), AMSADDR with WORD port, ST_AmsAddr, DEFAULT_ADS_TIMEOUT, F_CreateAmsNetId, MEMCMP and RTC. MEMCPY/MEMSET/MEMMOVE and ADSREAD/ADSWRITE take PVOID.
- Tc2_Utilities gains TIMESTRUCT, E_TimeZoneID, FB_LocalSystemTime and SYSTEMTIME_TO_DT.
- 11 Tc2_EtherCAT FBs, 9 Modbus TCP client FBs and 6 serial FBs follow the Beckhoff manuals. E_EcSlaveState, EcDiagParam and FB_EcDeviceDiag are not declared, and a test asserts that.
- `beckhoff.Closure("Tc2_EtherCAT")` returns common_types.st, tc2_system.st, tc2_utilities.st, tc2_ethercat.st. Tc2_Standard reports builtin.
- Mutation checks confirmed the tests catch a wrong parameter name, a type mismatch and a typo in a stub initialiser.

## Task Commits

1. **Task 1: Tc2_System/Tc2_Utilities extensions and ADS mock** - `fc082f8` (feat)
2. **Task 2: new stub files** - `feea971` (feat)
3. **Task 3: embed and closure** - `bd56e5e` (test), `1b19e30` (test, relocation), `3d1ee05` (feat)

## API for 21-03..21-06

- Import `github.com/centroid-is/stc/stdlib/beckhoff`.
- `beckhoff.FS fs.FS` holds `common_types.st`, `tc2_system.st`, ... at its root. Use `fs.ReadFile(beckhoff.FS, name)`; it is not an `embed.FS`.
- `beckhoff.Closure(lib string) ([]string, bool)`: case-insensitive, deps first, de-duplicated. False for unknown and builtin names.
- `beckhoff.IsBuiltin(lib string) bool`: true only for Tc2_Standard.
- `beckhoff.Libraries() []string`: sorted lower-case names of the 9 stub libraries.
- Stub display path "stdlib/vendor/beckhoff/<file>" (21-04) is still accurate for where the files live on disk.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Closure package moved out of the vendor directory**
- **Found during:** Task 3
- **Issue:** Go treats `stdlib/vendor/` as a vendor tree. The test failed with "must be imported as beckhoff", so 21-04/21-05 could not import a package there.
- **Fix:** `stdlib/embed.go` (package stdlib) embeds `vendor/beckhoff/*.st`. Package `stdlib/beckhoff` exposes FS (an fs.Sub), Closure, IsBuiltin, Libraries. Closure tests and testdata/usage.st live there.
- **Files modified:** stdlib/embed.go, stdlib/beckhoff/*, stdlib/vendor/beckhoff/stubs_test.go
- **Commits:** 1b19e30, 3d1ee05

**2. [Rule 2 - Missing critical] CI never ran stub tests**
- **Found during:** Task 1
- **Issue:** `go test ./...` skips directories named vendor, so stdlib/vendor/* tests never ran in CI.
- **Fix:** Added a "Test vendor stubs" CI step and a Makefile line running `go test ./stdlib/vendor/...`. All three vendor packages pass.
- **Files modified:** .github/workflows/ci.yml, Makefile
- **Commit:** 3d1ee05

**3. [Orchestrator note] RTC placement**
- RTC is declared in tc2_system.st with a header note, not tc2_utilities.st, so Tc2_Standard plus Tc2_System projects resolve it.

**4. [Rule 2] Extra test TestNoSVNCoreDeclarationsInStubs**
- Asserts E_EcSlaveState, EcDiagParam and FB_EcDeviceDiag are absent from all stubs (threat T-21-04).

## Known Stubs

- tc3_module.st and tc3_ipcdiag.st declare only marker types by design. No sildarvinnsla code uses either library.

## Deferred Issues

See deferred-items.md: PVOID typing for Phase 22, and pre-existing gofmt drift in stdlib/vendor/allen_bradley/stubs_test.go.

## Self-Check: PASSED

- All created files exist; commits fc082f8, feea971, bd56e5e, 1b19e30, 3d1ee05 present on main.
- `go test ./stdlib/... ./stdlib/vendor/... ./pkg/vendor/... -count=1` green; stdlib/beckhoff coverage 96.6%.
