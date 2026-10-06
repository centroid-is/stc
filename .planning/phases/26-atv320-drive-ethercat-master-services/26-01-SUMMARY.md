---
phase: 26-atv320-drive-ethercat-master-services
plan: 01
subsystem: ecat
tags: [ethercat, atv320, cia402, device-model, simulation]

requires:
  - phase: 24-ethercat-topology-link-binding
    provides: ecat.Device, Registry/DefaultRegistry, NewNetwork, Slot/ReadBits/WriteBits
provides:
  - ecat.EntrySlot / ecat.LayoutAware hook (SetLayout after Init in NewNetwork)
  - devices.CiA402 pure state machine
  - devices.ATV320 model registered for 0x0800005A/0x389 with stimulus API
  - tests/ecat_fixtures/atv320_device.xml (real ST301 PDO mapping)
affects: [26-02, 26-03, 26-04, 25 merge]

tech-stack:
  added: []
  patterns:
    - "Device models implement ecat.LayoutAware to receive name-addressed entry slots"
    - "Device packages self-register in init() into ecat.DefaultRegistry"
    - "Ramp math in integers with a carried remainder for determinism"

key-files:
  created:
    - pkg/ecat/entryslot.go
    - pkg/ecat/entryslot_test.go
    - pkg/ecat/devices/cia402.go
    - pkg/ecat/devices/cia402_test.go
    - pkg/ecat/devices/atv320.go
    - pkg/ecat/devices/atv320_test.go
    - tests/ecat_fixtures/atv320_device.xml
  modified:
    - pkg/ecat/network.go

key-decisions:
  - "Entry layout hook lives in pkg/ecat/entryslot.go, not layout.go, because Phase 25 creates layout.go"
  - "A device-local packed layout was rejected: Demo Device 2's ProcessImage proves it mis-addresses real exports"
  - "InjectFault shows FRA on the next Step, then Fault; reset needs CMD bit 7 rising edge after ClearFault"
  - "Quick stop stays visible for at least one Step even at standstill before dropping to SOD"

patterns-established:
  - "LayoutAware: SetLayout(in, out []EntrySlot) with view-relative Byte/Bit"

requirements-completed: []  # ECAT-05 partially delivered (process data); completes with 26-02/26-04

duration: 8min
completed: 2026-10-06
---

# Phase 26 Plan 01: ATV320 Drive Model Summary

**ATV320 EtherCAT drive model with a CiA402 state machine, exact integer LFR to RFR ramps, LCR/HMIS/LFT reporting and InjectFault/ClearFault/SetDI/SetSTO, fed by a new LayoutAware entry-slot hook on NewNetwork.**

## Performance

- **Duration:** about 8 min
- **Started:** 2026-10-06T07:17:13Z
- **Completed:** 2026-10-06T07:25:00Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments

- `ecat.EntrySlot`, `ecat.LayoutAware`, `ecat.FindEntry` and `EntrySlot.Get/Set`; NewNetwork calls `SetLayout` once after `Init`. Slots follow the master's real layout, including ProcessImage offsets.
- `devices.CiA402`: `Step(cmd, faultActive)`, `Fault`, `QuickStopDone`, `Disable`, `State`, `Enabled`, `Halt`, `ETA(targetReached)`. ETA words are checked against a Go mirror of FB_ATV320.status_word_to_state.
- `devices.ATV320` (`NewATV320`): tunables HSP/LSP/FRS/NCR/ACC/DEC, getters `RFR/ETA/HMIS/LFT/LCR/OL1R/State`, and constants `HMISRdy/Nst/Run/Acc/Dec/Fst/Fault/Sto`. The ramp reaches RFR 500 after exactly 30 Steps of 100 ms with ACC 30.
- Coverage: pkg/ecat/devices 100%, pkg/ecat 99.4%. Tests pass with `-race -count=3`.

## Task Commits

1. **Task 1: Entry layout hook on Network** - `70577bc` (feat)
2. **Task 2: Pure CiA402 state machine** - `d8c6315` (feat)
3. **Task 3: ATV320 device model** - `219024c` (feat)

## Files Created/Modified

- `pkg/ecat/entryslot.go` - EntrySlot, LayoutAware, FindEntry, slaveEntries, bindLayout
- `pkg/ecat/network.go` - one added line: `bindLayout(m, rt)` after `rt.dev.Init(s)`
- `pkg/ecat/devices/cia402.go` - CiA402 state machine
- `pkg/ecat/devices/atv320.go` - ATV320 model and init() registration
- `tests/ecat_fixtures/atv320_device.xml` - "Device 3 (EtherCAT)" with SIM.FD01, 6 inputs and 5 outputs

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Hook file renamed to entryslot.go**
- **Found during:** Task 1
- **Issue:** Phase 25 (worktree stc-wt-25) creates `pkg/ecat/layout.go` and `layout_test.go` with its own Layout/Field/Binder API. Using the planned filenames would cause add/add merge conflicts.
- **Fix:** The same API as planned now lives in `pkg/ecat/entryslot.go` and `entryslot_test.go`. network.go gets only one inserted line. When Phase 25 merges, expect a one-line conflict next to `rt.dev.Init(s)`; keep both calls.
- **Commit:** 70577bc

**2. [Rule 1 - Bug] Device-local packed layout rejected**
- **Found during:** Task 1
- **Issue:** The team lead suggested a private layout helper inside the ATV320 file. A prototype that packed `Slave.Pdos` failed on Demo Device 2. Its ProcessImage places CMD apart from LFR, so a device cannot derive offsets without the master's slot table.
- **Fix:** Used the plan's NewNetwork hook instead. A test now asserts that the slots follow the ProcessImage layout.
- **Commit:** 70577bc

**3. [Rule 1 - Bug] Ramp remainder reset on ramp-time change**
- **Found during:** Task 3
- **Issue:** The carried remainder was reused after ACC changed, which overshot by 2.
- **Fix:** The remainder now resets when its denominator changes.
- **Commit:** 219024c

## Notes for 26-02 / 26-03

- In this plan PDOs are processed every Step. 26-02 adds EtherCAT state gating and PreOp start.
- A Network does not expose its device instances yet; the tests capture the instance through a fresh Registry factory. Phase 25 adds `Network.DeviceByName`. Until that merges, 26-03 needs its own accessor or the same factory capture.
- Fault reset is edge-triggered. If the master holds CMD bit 7 high through ClearFault, it must drop the bit and raise it again.

## Requirements

ECAT-05 is left Pending in REQUIREMENTS.md. This plan delivers its process-data half. The object dictionary (26-02) and the FB_ATV320 cfgReady gate (26-04) remain.

## Self-Check: PASSED

- Files exist: pkg/ecat/entryslot.go, pkg/ecat/devices/{cia402,atv320}.go, tests/ecat_fixtures/atv320_device.xml
- Commits exist: 70577bc, d8c6315, 219024c
