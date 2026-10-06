---
phase: 25-ethercat-terminal-models
plan: 02
subsystem: ecat
tags: [ethercat, device-models, analog, el9222, psu, twinsafe]
requires:
  - "25-01 ecat.Layout, devices.Base, knownIDs/beckhoffFallbacks registration"
provides:
  - "devices.Analog (EL3054 4-20 mA, EL3064 0-10 V): SetCurrent, SetVoltage, SetRaw, Channels"
  - "devices.EL9222 (EL9222-5500): Trip, SetWarning, SetCoolDown, SetHardwareProtection, SetLoadCurrent, Enabled, Tripped, Channels"
  - "devices.PSU (PS2001-2410): PSUState, SetPSU, State"
  - "devices.SafetyDiag (EL2912, EP1918-0002, EL1904): SetFieldVoltage"
  - "tests/ecat_fixtures/Demo Analog.xml"
affects: [25-04, 27]
tech-stack:
  added: []
  patterns: ["models with computed inputs write them in Step, then call stepIO so generic Set overrides win", "models resolve channel entries once in Bind"]
key-files:
  created:
    - pkg/ecat/devices/analog.go
    - pkg/ecat/devices/analog_test.go
    - pkg/ecat/devices/el9222.go
    - pkg/ecat/devices/el9222_test.go
    - pkg/ecat/devices/psu.go
    - pkg/ecat/devices/safety.go
    - pkg/ecat/devices/psu_safety_test.go
    - tests/ecat_fixtures/Demo Analog.xml
  modified:
    - pkg/ecat/devices/ids.go
    - pkg/ecat/devices/digital_test.go
    - pkg/ecat/devices/base_test.go
decisions:
  - "EL9222 channel without a mapped Control__Switch counts as switched on (Demo Device 1 maps only Reset)"
  - "EL9222 Current is written in 0.01 A (EL922x doc, 0x60n0:22 'Current [0,01 A]') and reads 0 while disabled"
  - "PSU voltage/current encode as REAL/LREAL bits when typed so, otherwise as milli-units clamped to the entry width"
  - "SetPSU replaces the whole PSUState; the default is DC OK at 24 V, 0 A"
  - "SafetyDiag writes only Fieldvoltage Underrange/Overrange; no 'Field Voltage Status' entry exists in the exports"
metrics:
  duration: "~25 min"
  completed: 2026-10-06
  tasks: 3
  files: 11
---

# Phase 25 Plan 02: Analog, EL9222-5500, PS2001-2410 and Safety Diagnostics Models Summary

Analog inputs, the EL9222-5500 overcurrent protection terminal, the PS2001-2410 power supply and the TwinSAFE terminals' standard diagnostics now have device models with Go stimulus APIs. On Demo Device 1 the only slave without a model is the ATV320 drive, which Phase 26 covers.

## Tasks

| Task | Name | Commits |
|------|------|---------|
| 1 | Analog input model (EL3054, EL3064) with trimmed fixture | 5ec5e5d (test), b69ff7f (feat) |
| 2 | EL9222-5500 per-channel trip state machine | 4972ac3 (test), ab194d0 (feat) |
| 3 | PS2001-2410 PSU and safety-terminal diagnostics | 66c346e (test), dab124e (feat) |

## Stimulus API for Phase 27

- **Analog.** `SetCurrent(ch, mA)` on 4-20 mA terminals and `SetVoltage(ch, V)` on 0-10 V terminals. `SetRaw(ch, int16)` writes the value with a clean status. The wrong stimulus kind returns an error.
- **EL9222.** `Trip(ch)`, `SetWarning(ch, bool)`, `SetCoolDown(ch, bool)`, `SetHardwareProtection(ch, bool)` and `SetLoadCurrent(ch, amps)`. `Enabled(ch)` and `Tripped(ch)` read the state.
- **PSU.** `SetPSU(PSUState{Warning, Error, DCOK, InputUndervoltage, OutputVoltage, OutputCurrent})` and `State()`.
- **SafetyDiag.** `SetFieldVoltage(under, over bool)`.

All channel numbers are 1-based. Out-of-range channels return an error naming the slave.

## Behaviour

- **Analog.** 4..20 mA and 0..10 V map to 0..32767 with rounding, so 12 mA and 5 V both give 16384. Below range sets Underrange and above range sets Overrange, with clamped values. 0 mA or less, or NaN, is an open wire and sets Error with Underrange. Infinities clamp. TxPDO Toggle flips every Step. Status goes to the Status__* bit entries or to a single 16-bit Status word at bits 0, 1, 6, 14 and 15.
- **EL9222.** Enabled requires Switch on with no trip, cool-down or hardware protection. Only a Control__Reset rising edge clears a trip. Error is set for a trip or hardware protection. Diag is set for any fault or warning. The input cycle counter counts mod 4, and State Reset and State Switch echo the control bits.
- **PSU.** The PLC's Disable output forces DC OK, voltage and current to 0.

## Verification

- `go test ./pkg/ecat/... ./cmd/stc -count=1 -cover` passes. Coverage is 99.6% for `pkg/ecat` and 100% for `pkg/ecat/devices`.
- `go vet ./pkg/ecat/... ./cmd/stc` is clean, and `go build ./...` succeeds.
- The 12 mA case is asserted at the master image slot through `LinkPath` on Demo Analog. EL9222 trip, held Reset, rising-edge recovery and cool-down are asserted at the image slots of Demo Device 1.
- `TestDemoOnlyDriveUnmodelled` asserts exactly one ECAT010 on Demo Device 1, for the ATV320.
- The full `go test ./...` run shows only the known pre-existing `pkg/checker` `TestEmptyFBCall` failure.
- Product codes for EL3054, EL9222-5500, PS2001-2410, EL2912, EP1918-0002 and EL1904 were confirmed in the reference exports. EL3064 uses Beckhoff numbering because it is absent from them.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan's EL3064 decimal product code was EL3054's**
- **Found during:** Task 1
- **Issue:** The plan gave "ProductCode 200159314 (0x0bf83052)". 200159314 is 0x0bee3052, the EL3054 code.
- **Fix:** The EL3064 copy in Demo Analog uses 200814674, which is 0x0bf83052.
- **Commit:** 5ec5e5d

**2. [Rule 1 - Bug] 25-01 passive test assumed no model writes inputs without stimulus**
- **Found during:** Task 2
- **Issue:** `TestPassiveCouplersNoDiagnostic` failed because EL9222 reports Enabled with no stimulus.
- **Fix:** The unchanged-image check now covers only Passive, DigitalIO and Passthrough slaves.
- **Commit:** ab194d0, amended into the feat commit before moving on.

**3. [Rule 1 - Bug] No "Field Voltage Status" entry in the real exports**
- **Found during:** Task 3
- **Issue:** EL2912 exports carry only Fieldvoltage Underrange and Overrange under the PDO "FIELDVOLTAGE Field Voltage Status". The EL2912 manual documents no separate status value to cite.
- **Fix:** SafetyDiag writes only the Underrange and Overrange bits, healthy at 0, and leaves every other entry alone. EP1918-0002 and EL1904 have no field-voltage entries, so they are recognised and nothing is written.
- **Commit:** dab124e

**4. Test cleanup.** `TestDemoDiagnosticsOnlyLaterPlanModels` from 25-01 was replaced by `TestDemoOnlyDriveUnmodelled` and `TestDemoDigitalModels`.

**Note.** The real EL9222-5500 exports do not map Current, and the PSU Outputs PDO is inactive in them. Both models handle the entries when present and skip them otherwise.

ECAT-04 is not marked complete; plan 25-04 does that.

## Known Stubs

None.

## Self-Check: PASSED
