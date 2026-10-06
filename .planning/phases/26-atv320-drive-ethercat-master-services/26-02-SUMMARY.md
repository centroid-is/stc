---
phase: 26-atv320-drive-ethercat-master-services
plan: 02
subsystem: ecat
tags: [ethercat, atv320, coe, sdo, object-dictionary, ethercat-state, simulation]

requires:
  - phase: 26-atv320-drive-ethercat-master-services
    provides: devices.ATV320 model, ecat.EntrySlot/LayoutAware (26-01)
provides:
  - ecat.CoEDevice / ecat.StateDevice interfaces, CoE abort codes, EtherCAT state and link constants
  - Network service API for the Tc2_EtherCAT mocks (slave lookup by address, state requests, link, CRC, master state)
  - ATV320 CoE object dictionary with FB_ATV320 parameter defaults and the 0x2032:01 EEPROM save object
  - ATV320 PreOp boot and EtherCAT OP gating of process data
affects: [26-03, 26-04, 25 merge]

tech-stack:
  added: []
  patterns:
    - "Devices opt into SDO and EtherCAT state by implementing ecat.CoEDevice / ecat.StateDevice"
    - "Table-driven object dictionary keyed by index<<8|subindex with get/set closures"
    - "Network service state lives in a services struct in services.go; network.go only gets small hooks"

key-files:
  created:
    - pkg/ecat/services.go
    - pkg/ecat/services_test.go
    - pkg/ecat/devices/atv320_od.go
    - pkg/ecat/devices/atv320_od_test.go
    - pkg/ecat/devices/atv320_state_test.go
  modified:
    - pkg/ecat/network.go
    - pkg/ecat/devices/atv320.go
    - pkg/ecat/devices/atv320_test.go

key-decisions:
  - "HSP/LSP/FRS/NCR/ACC/DEC stay exported model fields and the OD binds to them directly, so OD and model share one storage"
  - "Below OP the ATV320 zeroes all its input entries, holds CiA402 (STO still disables) and stops the motor"
  - "Plain devices keep a requested state across ClearFaults; ClearFaults resets only overrides (state, WcState, DevState, link, CRC, master state)"
  - "No DeviceByName accessor added; Network.Slave(master, addr) returns the Device, so no name clash with Phase 25"

patterns-established:
  - "Network.Slave(master, addr) -> (index, Device, error) is the entry point for address-based mocks"

requirements-completed: []  # ECAT-05 and ECAT-08 complete with 26-03/26-04

duration: 6min
completed: 2026-10-06
---

# Phase 26 Plan 02: CoE Object Dictionary, PreOp Boot and Network Services Summary

**ATV320 CoE object dictionary with all 15 FB_ATV320 parameters and a 5-Step EEPROM save, PreOp boot with OP-gated process data, and a Network service API (address lookup, state requests, link, CRC, master state) for the Tc2_EtherCAT mocks.**

## Performance

- **Duration:** about 6 min
- **Started:** 2026-10-06T07:25:40Z
- **Completed:** 2026-10-06T07:31:00Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments

- `pkg/ecat/services.go`: `CoEDevice`, `StateDevice`, `Abort*` codes, `StateInit/PreOp/SafeOp`, `LinkNotPresent/LinkWithoutComm`, `DefaultStateDelay`. Network methods: `MasterNames`, `MasterByNetID`, `SlaveCount`, `SlaveAddr`, `Slave`, `SlaveState`, `RequestSlaveState`, `SetLinkState`, `CrcErrors`, `SetCrcErrors`, `ClearCrc`, `MasterState`, `SetMasterState`, field `StateDelay`.
- `Network.Step` publishes the effective state to InfoData.State: SetSlaveState override, else `StateDevice.EcState()`, else the requested state of a plain device, else OP.
- ATV320 object dictionary: CiA402 0x6040/0x6041/0x6060/0x6061, live RFR/LCR/HMIS/LFT/DI, PDO mirrors CMD/LFR/OL1R, ACC/DEC, the 15 FB_ATV320 parameters as UINT, and 0x2032:01 as UDINT. `SDORead`, `SDOWrite`, `SaveCount`.
- ATV320 boots in PreOp and reaches any requested state after `StateDelay` Steps (default 2).
- Coverage: pkg/ecat 99.5%, pkg/ecat/devices 100%. Tests pass with `-race -count=3`. cmd/stc and pkg/interp still pass.

## Task Commits

1. **Task 1: Network service API and device interfaces** - `268dcc8` (feat)
2. **Task 2: ATV320 CoE object dictionary** - `c830f10` (feat)
3. **Task 3: PreOp start and EtherCAT state gating** - `fc12d83` (feat)

## Files Created/Modified

- `pkg/ecat/services.go` - interfaces, constants, services state, Network service methods, `slaveAddr` helper
- `pkg/ecat/network.go` - `svc` and `StateDelay` fields, `stepPending` call, effective state and shared port helper in Step, ClearFaults resets service overrides
- `pkg/ecat/devices/atv320_od.go` - object dictionary table, SDO access, EEPROM countdown
- `pkg/ecat/devices/atv320.go` - OD fields, StateDevice implementation, `status` and `stepNoExchange` split out of Step
- `pkg/ecat/devices/atv320_test.go` - rig now requests OP through `goOP`; `newRigPreOp` for PreOp tests
- `pkg/ecat/devices/atv320_state_test.go` - PreOp boot, transitions, gating, SDO by state, override

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Tunables kept as fields instead of a parameter map**
- **Found during:** Task 2
- **Issue:** The plan asked for HSP/LSP/FRS/NCR/ACC/DEC accessors that read from the parameter map. 26-01 exposes them as exported fields that tests and later plans set directly.
- **Fix:** The OD entries for these six objects read and write the fields. The other parameters live in the map. OD and model still share a single storage.
- **Commit:** c830f10

**2. [Rule 2 - Missing] Writing 0 to 0x2032:01 cancels a pending save**
- **Found during:** Task 2
- **Fix:** A zero write clears the countdown, so no save is counted. Covered by a test.
- **Commit:** c830f10

**3. [Rule 3 - Blocking] State tests in a new file**
- **Found during:** Task 3
- **Issue:** The plan listed atv320_test.go. A separate atv320_state_test.go keeps that file's diff small for the Phase 25 merge.
- **Commit:** fc12d83

## ESI defaults

The ATV320 ESI (Schneider_Electric_ATV320_V116.xml) lists the 0x2042 and 0x2001 objects but has no DefaultData. The plan's fallback defaults are used.

## Notes for 26-03

- Resolve the master with `MasterByNetID`. Callers fall back to the only master when it returns an error.
- Resolve the slave with `Slave(master, nSlaveAddr)`, then type-assert the Device to `ecat.CoEDevice` for SDO. Non-CoE devices should abort with `ecat.AbortNoObject`.
- `RequestSlaveState` then poll `SlaveState` to emulate FB_EcSetSlaveState.
- FB_EcPhysicalWriteCmd to 0x0300 maps to `ClearCrc`.
- For Phase 25 merge: no device accessor was added here. network.go conflicts should be limited to the Network struct fields and the NewNetwork and Step hooks.

## Self-Check: PASSED

- Files exist: pkg/ecat/services.go, pkg/ecat/devices/atv320_od.go, pkg/ecat/devices/atv320_state_test.go
- Commits exist: 268dcc8, c830f10, fc12d83
