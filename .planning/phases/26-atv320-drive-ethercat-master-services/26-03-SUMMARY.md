---
phase: 26-atv320-drive-ethercat-master-services
plan: 03
subsystem: interp
tags: [ethercat, tc2_ethercat, mocks, coe, sdo, ads, simulation]

requires:
  - phase: 26-atv320-drive-ethercat-master-services
    provides: Network service API, CoEDevice, Abort codes, ATV320 object dictionary (26-02)
provides:
  - ScanCycleEngine.SetNetwork(net) attaching Tc2_EtherCAT mocks per engine
  - Interpreter fbOverrides (engine-local StandardFB factories ahead of FBDecls and StdlibFBFactory)
  - Ten Go StandardFB mocks with 2-scan async latency and ADS/CoE error codes
  - F_CreateAmsNetId as an engine-local function with named-argument support
  - Pointer byte codec (readPtrBytes/writePtrBytes) for ADR() buffers
  - stdlib/vendor/beckhoff/tc2_ethercat.st (verbatim copy from main)
affects: [26-04, 22 merge]

tech-stack:
  added: []
  patterns:
    - "Engine-local FB overrides via Interpreter.fbOverrides, consulted by stdFBFactory"
    - "asyncReq handshake: rising edge starts, completes no earlier than MockLatency scans later"
    - "LocalFunctions may accept named arguments when registered in Interpreter.localParams"

key-files:
  created:
    - pkg/interp/ecat_services.go
    - pkg/interp/ecat_services_test.go
    - pkg/interp/ecat_fbs.go
    - pkg/interp/ecat_fbs_test.go
    - stdlib/vendor/beckhoff/tc2_ethercat.st
  modified:
    - pkg/interp/fb_instance.go
    - pkg/interp/interpreter.go
    - pkg/interp/scan.go

key-decisions:
  - "Mocks step the network themselves unless an IOBinder on the same network already does, so each Tick is one Step either way"
  - "Request errors found on the rising edge (unknown master or slave) are still reported after the 2-scan latency"
  - "Pointer buffers use a packed little-endian layout (no alignment padding); struct member order comes from the matching STRUCT in TypeDecls"
  - "Bad or missing buffer pointers report ADS 0x706 (invalid parameter values)"
  - "FB_EcSetSlaveState completes only when SlaveState equals reqState; tTimeout T#0S means 5 s"

patterns-established:
  - "ecatMockCtors registry keyed by upper-case FB name; SetNetwork binds every entry to the engine's services"

requirements-completed: []  # ECAT-08 mocks delivered; requirement closes with 26-04 end-to-end use

duration: 17min
completed: 2026-10-06
---

# Phase 26 Plan 03: Tc2_EtherCAT Mocks Summary

**Ten Tc2_EtherCAT function blocks mocked as Go StandardFBs on the simulated EtherCAT network, each busy for exactly 2 scans, with ADS and CoE abort codes, ADR() buffer access and F_CreateAmsNetId.**

## Performance

- **Duration:** about 17 min
- **Started:** 2026-10-06T07:25:00Z
- **Completed:** 2026-10-06T07:42:00Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments

- `(*ScanCycleEngine).SetNetwork(net)`: registers the mocks as engine-local FB overrides, registers `F_CREATEAMSNETID`, and counts one scan per Tick. `SetNetwork(nil)` detaches. Must be called before Initialize or the first Tick.
- Mocked FB names: FB_ECGETSLAVESTATE, FB_ECGETALLSLAVESTATES, FB_ECSETSLAVESTATE, FB_ECGETMASTERSTATE, FB_ECGETALLSLAVECRCERRORS, FB_ECGETSLAVECRCERROR, FB_ECGETSLAVECRCERROREX, FB_ECPHYSICALWRITECMD, FB_ECCOESDOREAD, FB_ECCOESDOWRITE.
- Errors: 0x6 unknown slave, 0x7 unknown master (empty or unknown sNetId falls back to the only master), 0x745 timeout, 0x706 bad buffer pointer, CoE abort codes from the device.
- ST access to struct outputs works, for example `gs.state.deviceState` and `gcx.CrcError.portD`.
- Coverage: ecat_services.go 100%, ecat_fbs.go 100%, pkg/interp 98.3%. pkg/interp passes with `-race`.

## Task Commits

1. **Task 1: Network attachment, FB override factory, async core and pointer codec** - `f4af04b` (feat)
2. **Task 2: State, master and CRC mocks** - `f0affbc` (feat)
3. **Task 3: CoE SDO read and write mocks** - `5661431` (feat)

## Files Created/Modified

- `pkg/interp/ecat_services.go` - SetNetwork, preScan, localArgs, F_CreateAmsNetId, resolveMaster/resolveSlave, asyncReq, pointer codec, mock registry
- `pkg/interp/ecat_fbs.go` - the ten StandardFB mocks
- `pkg/interp/fb_instance.go` - `stdFBFactory` helper (fbOverrides, then StdlibFBFactory) used for nested FB members
- `pkg/interp/scan.go` - `ecat` field, nil-guarded preScan hook after the IOBinder one, initVarDecl uses stdFBFactory
- `pkg/interp/interpreter.go` - `fbOverrides` and `localParams` fields; LocalFunctions dispatch calls localArgs
- `stdlib/vendor/beckhoff/tc2_ethercat.st` - identical to main

## Decisions Made

See key-decisions in the frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Named arguments for F_CreateAmsNetId**
- **Found during:** Task 1
- **Issue:** LocalFunctions only accepted positional arguments, but SVNCoreComponents calls `F_CreateAmsNetId(nIds := amsaddr.netId)`.
- **Fix:** Added `Interpreter.localParams` and `localArgs`, used by the LocalFunctions dispatch. Unregistered functions keep the positional-only rule. Duplicate, unknown and missing arguments are runtime errors.
- **Commit:** f4af04b

**2. [Rule 3 - Blocking] Network stepping without an IOBinder**
- **Found during:** Task 1
- **Issue:** Only the IOBinder stepped the network, so FB_EcSetSlaveState could never complete in an engine without TcLinkTo bindings.
- **Fix:** The mock preScan steps the network unless an IOBinder bound to the same network is attached.
- **Commit:** f4af04b

**3. [Rule 2 - Missing] SDO cbRead output and ADS 0x706 for bad buffers**
- **Found during:** Tasks 2 and 3
- **Issue:** The stub declares `cbRead` on FB_EcCoESdoRead. The plan did not name an error for nil or non-pointer buffers.
- **Fix:** cbRead reports the bytes copied. Bad buffers end with bError and nErrId 0x706.
- **Commits:** f0affbc, 5661431

**4. [Rule 1 - Bug] readPtrBytes/writePtrBytes are methods on the services**
- **Issue:** Struct member order needs the interpreter's TypeDecls, so the helpers are `(*ecatServices)` methods instead of free functions.
- **Commit:** f4af04b

## TDD Gate Compliance

Tests and implementation were committed together per task. No separate `test(...)` RED commits exist.

## Notes for 26-04

- Call `eng.SetNetwork(net)` before the first Tick. Pass the same network to `NewIOBinder` so the network is stepped once per scan.
- `SIZEOF` is not implemented in pkg/interp on this branch, and array bounds such as `0..EcDiagParam.MAX_EC_SLAVES - 1` are not evaluated (only literals). FB_EcDeviceDiag and FB_Parameter use both. Phase 22 on main may cover this. Otherwise 26-04 needs them.
- Arrays with a lower bound above 0 carry padding elements at the front in this interpreter. The pointer codec walks the whole slice, so such buffers would be shifted. Both real buffers use 0-based arrays.
- Two masters loaded from the fixtures share the default AmsNetId. Set distinct ids with `SetMasterNetID` when a test needs both.

## Self-Check: PASSED

- Files exist: pkg/interp/ecat_services.go, pkg/interp/ecat_fbs.go, pkg/interp/ecat_services_test.go, pkg/interp/ecat_fbs_test.go, stdlib/vendor/beckhoff/tc2_ethercat.st
- Commits exist: f4af04b, f0affbc, 5661431
