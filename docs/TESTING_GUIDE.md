# Testing Guide

How to write and run ST unit tests with the stc toolchain.

## Overview

stc provides a built-in testing framework that lets you write unit tests in Structured Text syntax, run them on your development machine with no PLC hardware, and integrate results into CI pipelines. Tests use deterministic time advancement and I/O mocking to simulate real PLC behavior.

## TEST_CASE Syntax

Tests are written in `.st` files with the `_test.st` suffix. Each test is a `TEST_CASE` block:

```iec
TEST_CASE 'descriptive name of the test'
VAR
    (* Local variables for this test *)
    myFB : MyFunctionBlock;
    result : BOOL;
END_VAR
    (* Test body -- setup, execute, assert *)
    myFB(Enable := TRUE);
    result := myFB.Output;
    ASSERT_TRUE(result, 'Output should be TRUE when enabled');
END_TEST_CASE
```

Key points:
- The test name is a string literal in single quotes
- Each test gets its own variable scope -- no state leaks between tests
- Variables are initialized to zero values (or explicit init values) at the start
- A test file can contain multiple `TEST_CASE` blocks
- Test files are named `*_test.st` (e.g., `motor_test.st`, `conveyor_test.st`)

## Assertion Functions

### ASSERT_TRUE

```iec
ASSERT_TRUE(condition);
ASSERT_TRUE(condition, 'optional message');
```

Passes if `condition` is truthy. Fails with the optional message if provided.

### ASSERT_FALSE

```iec
ASSERT_FALSE(condition);
ASSERT_FALSE(condition, 'optional message');
```

Passes if `condition` is falsy.

### ASSERT_EQ

```iec
ASSERT_EQ(actual, expected);
ASSERT_EQ(actual, expected, 'optional message');
```

Passes if `actual` equals `expected`. Works with all value types (BOOL, INT, REAL, STRING, TIME, etc.). For REAL comparisons where floating-point precision matters, use `ASSERT_NEAR` instead.

### ASSERT_NEAR

```iec
ASSERT_NEAR(actual, expected, epsilon);
ASSERT_NEAR(actual, expected, epsilon, 'optional message');
```

Passes if `ABS(actual - expected) <= epsilon`. Use for floating-point comparisons.

```iec
ASSERT_NEAR(calculatedPressure, 101.325, 0.001, 'Pressure within tolerance');
```

## ADVANCE_TIME

Advances the deterministic simulation clock by a specified duration. All timer-based FBs (TON, TOF, TP) and the scan cycle engine use this clock.

```iec
ADVANCE_TIME(T#100ms);
```

This does not execute any FB bodies -- it only advances the clock. You must call your FBs again after advancing time for them to observe the new time:

```iec
TEST_CASE 'TON fires after preset'
VAR
    t : TON;
END_VAR
    (* Start timer *)
    t(IN := TRUE, PT := T#200ms);
    ASSERT_FALSE(t.Q, 'Should not fire immediately');

    (* Advance past preset *)
    ADVANCE_TIME(T#250ms);
    t(IN := TRUE, PT := T#200ms);
    ASSERT_TRUE(t.Q, 'Should fire after 250ms > 200ms preset');
END_TEST_CASE
```

## I/O Mocking

### SET_IO

Inject a value into the mock I/O table before executing logic that reads from I/O addresses.

```iec
SET_IO(area, byte_offset, bit_offset, value);
```

Parameters:
- `area`: `0` = Input (%I), `1` = Output (%Q), `2` = Memory (%M)
- `byte_offset`: Byte offset in the I/O area
- `bit_offset`: Bit offset within the byte (0-7 for bit access, 0 for word/dword)
- `value`: Value to write

### GET_IO

Read a value from the mock I/O table to verify actuator outputs.

```iec
result := GET_IO(area, byte_offset, bit_offset);
```

### Example

```iec
TEST_CASE 'Input reads from I/O table'
VAR
    sensorActive : BOOL;
END_VAR
    (* Simulate sensor at %IX0.3 being active *)
    SET_IO(0, 0, 3, TRUE);

    (* After scan cycle sync, the AT-addressed variable should reflect the value *)
    (* ... execute program logic ... *)

    (* Verify output at %QX1.0 *)
    ASSERT_TRUE(GET_IO(1, 1, 0), 'Actuator output should be set');
END_TEST_CASE
```

## Project Mode

SET_IO and GET_IO work on explicit addresses. Production TwinCAT code instead declares I/O as `AT %I*` or `AT %Q*` and links it to terminals with `TcLinkTo`. Project mode tests that code against the real project.

### Tests against a TwinCAT project

`stc test --project` imports the project and makes its POUs, GVLs, DUTs and interfaces available to the tests. Sibling library projects are imported from source. Libraries that only have stubs run as zero-output auto-stubs.

```bash
stc test pkg/twincat/testdata/sln/tests --project "pkg/twincat/testdata/sln/Demo/Demo solution.tsproj"
```

```text
ok
2 tests, 2 passed, 0 failed
```

See [TWINCAT_IMPORT.md](TWINCAT_IMPORT.md) for what is imported.

### Wildcard addresses

In project mode (`stc sim` and `stc serve` on a project), every `%I*`, `%Q*` and `%M*` variable gets its own slot sized by its declared type. A wildcard that has a `TcLinkTo` link is bound to its EtherCAT slot instead when `--io` is given. The value is decoded by the declared type: an INT is two's complement, a REAL is IEEE 754 single precision and an enum uses its base type. See [ETHERCAT_SIMULATION.md](ETHERCAT_SIMULATION.md).

### Whole-project tests with EtherCAT and scenarios

`stc test --project ... --io "Device*.xml"` runs each TEST_CASE against a fresh project and EtherCAT network, with the SET, GET, RUN_CYCLES and SIM_* built-ins registered. See [Testing against the simulated plant](#testing-against-the-simulated-plant). Scenario files use the same stimuli; see [ETHERCAT_SIMULATION.md](ETHERCAT_SIMULATION.md#scenarios).

## Vendor FB Mocking

When testing code that uses vendor-specific function blocks (e.g., Beckhoff `MC_MoveAbsolute`), you have three options:

### 1. Auto-generated Zero-Value Stubs (default)

If you configure vendor library stubs in `stc.toml` but provide no mock implementation, stc auto-generates FBs that accept all inputs and return zero-valued outputs. A fidelity warning is printed:

```
Warnings:
  [fidelity] Auto-stubbed FB 'MC_MoveAbsolute' used (zero-value outputs, no behavioral simulation)
```

This is sufficient when you only need the code to compile and run without crashing.

### 2. Custom ST Mock FBs

Write a `FUNCTION_BLOCK` with the same name as the vendor FB, with a full body implementing the behavior you want:

```iec
(* mocks/mc_mock.st *)
FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis        : AXIS_REF;
    Execute     : BOOL;
    Position    : LREAL;
    Velocity    : LREAL;
    Acceleration : LREAL;
    Deceleration : LREAL;
    Jerk        : LREAL;
    Direction   : MC_Direction;
END_VAR
VAR_OUTPUT
    Done        : BOOL;
    Busy        : BOOL;
    Active      : BOOL;
    CommandAborted : BOOL;
    Error       : BOOL;
    ErrorID     : UDINT;
END_VAR
VAR
    prevExecute : BOOL;
    cycleCount  : INT;
END_VAR
    IF Execute AND NOT prevExecute THEN
        Busy := TRUE;
        Done := FALSE;
        cycleCount := 0;
    END_IF;
    IF Busy THEN
        cycleCount := cycleCount + 1;
        IF cycleCount >= 5 THEN
            Done := TRUE;
            Busy := FALSE;
        END_IF;
    END_IF;
    prevExecute := Execute;
END_FUNCTION_BLOCK
```

Mock signatures are validated against stub signatures -- parameter count or type mismatches produce errors before test execution.

### 3. Shipped Behavioral Mocks

stc ships behavioral mocks for common Beckhoff FBs in `stdlib/mocks/beckhoff/`. These simulate multi-cycle behavior:
- MC_MoveAbsolute: Simulates position changes over multiple scan cycles
- MC_Power: Simulates enable/disable with Status output
- MC_Home: Simulates homing sequence
- MC_Stop: Simulates deceleration
- ADSREAD: Returns configurable response data

## Testing against the simulated plant

`stc test` can run tests against the whole project and its EtherCAT network instead of isolated units. Pass the project sources with `--project` and the EtherCAT exports with `--io`:

```bash
stc test tests/ --project demo_types.st --project ECT.st --project ECT_Diag.st --project MAIN.st \
  --io "Demo Device [12].xml"
```

Each TEST_CASE starts from a fresh project and network, so a slave pulled in one case is healthy in the next. The body runs on the project's interpreter: GVL paths such as `ECT.A1_01.I1` read directly, and program variables read through `GET('MAIN.x')`. In this mode a test file may contain only TEST_CASEs. Declare functions, function blocks, types and GVLs in the project.

| Built-in | Arguments | Effect |
|----------|-----------|--------|
| `SET` | `path : STRING, value : ANY` | Writes a variable. A TcLinkTo-bound input is forced on its slot and reads back after the next scan. |
| `GET` | `path : STRING` returns ANY | Reads a variable or a `TIID^...` link slot. Enums return their name as a STRING. |
| `SIM_SET_LINK` | `link : STRING, value : ANY` | Forces an input slot by link path. |
| `SIM_TRIP` | `slave : STRING, channel : INT` | Trips an EL9222 OCP channel. |
| `SIM_SLAVE_STATE` | `slave : STRING, state : STRING or INT` | Applies a preset (`not_present`, `link_error`, `init`, `preop`, `safeop`, `op`, `ok`) or a raw state. |
| `SIM_ANALOG` | `slave : STRING, channel : INT, value : REAL [, unit : STRING]` | Sets an analog input. The unit is `mA`, `V` or `raw`, and defaults to `raw` (the process-data count). |
| `SIM_DRIVE_FAULT` | `slave : STRING, lft : INT` | Injects an ATV320 fault code (0 clears it). |
| `SIM_SERIAL_PEER` | `slave : STRING, script : STRING` | Attaches the `baader` or `loopback` peer to an EL6001, or detaches it with `none`. |
| `SIM_RAMP` | `path : STRING, from : REAL, to : REAL, over : TIME` | Ramps a variable linearly, written before every following scan. |
| `RUN_CYCLES` | `n : DINT` | Runs n scans of the whole project (0 to 10 000 000). |
| `ADVANCE_TIME` | `d : TIME` | Runs d / base tick scans. d must be a multiple of the base tick. |

Slave names follow the scenario rules: exact, then case-insensitive, then the unique prefix before ` (`. The semantics match the scenario actions in [EtherCAT Simulation](ETHERCAT_SIMULATION.md#scenarios). Without `--io`, SET, GET, RUN_CYCLES and ADVANCE_TIME work, and the SIM_* built-ins fail with "no --io network loaded".

Example from `tests/ecat_fixtures/scenario/scenario_test.st`:

```iecst
TEST_CASE 'removed slave shows in ECT_Diag'
RUN_CYCLES(20);
ASSERT_TRUE(ECT_Diag.Device_1_Diag[2].p_stat_bOk, 'EL1008 OK before removal');
SIM_SLAVE_STATE('DEMO.A1.01 (EL1008)', 'not_present');
RUN_CYCLES(20);
ASSERT_FALSE(ECT_Diag.Device_1_Diag[2].p_stat_bOk, 'EL1008 not OK after removal');
ASSERT_TRUE(ECT_Diag.Device_1_Diag[2].p_stat_nLinkState <> 0, 'link state reports the removal');
END_TEST_CASE

TEST_CASE 'drive fault'
RUN_CYCLES(30);
ASSERT_FALSE(GET('MAIN.xDriveFault'), 'no fault before');
SIM_DRIVE_FAULT('DEMO.CN01.FD01 (ATV320 EtherCAT)', 16);
RUN_CYCLES(10);
ASSERT_TRUE(GET('MAIN.xDriveFault'), 'CiA402 fault bit in ETA');
END_TEST_CASE
```

## JUnit XML for CI

Generate JUnit XML output for integration with CI systems (Jenkins, GitHub Actions, GitLab CI):

```bash
stc test tests/ --format junit > test-results.xml
```

Example GitHub Actions step:

```yaml
- name: Run ST tests
  run: stc test tests/ --format junit > test-results.xml

- name: Publish test results
  uses: mikepenz/action-junit-report@v4
  if: always()
  with:
    report_paths: test-results.xml
```

## stc.toml Setup

Configure testing with vendor libraries and mocks:

```toml
[project]
name = "my-machine"
version = "1.0.0"

[build]
source_roots = ["src/"]
vendor_target = "beckhoff"

[build.library_paths]
tc2_mc2 = "vendor/beckhoff/tc2_mc2.st"
tc2_system = "vendor/beckhoff/tc2_system.st"

[test]
mock_paths = ["mocks/"]

[lint]
naming_convention = "PascalCase"
```

The test runner loads libraries and mocks in this priority order (highest to lowest):
1. User-written mock FBs from `mock_paths`
2. Built-in stc standard library FBs (TON, TOF, CTU, etc.)
3. Auto-generated zero-value stubs from vendor library declarations

## Running Tests

```bash
# Run all tests in the tests/ directory
stc test tests/

# Run tests in a specific subdirectory
stc test tests/motor_control

# JSON output
stc test tests/ --format json

# JUnit XML output
stc test tests/ --format junit
```

Test discovery is recursive -- all `*_test.st` files under the given directory are found and executed.

## Preprocessor Integration

The `STC_TEST` preprocessor symbol is automatically defined when running `stc test`. Use it for test-only code paths:

```iec
{IF defined(STC_TEST)}
(* Simplified initialization for testing *)
maxVelocity := 100.0;
{ELSE}
(* Production: read from GVL *)
maxVelocity := GVL.MaxVelocity;
{END_IF}
```

## Real-World Examples

### Testing Motor Control with Interlocks

```iec
TEST_CASE 'Motor does not start without interlocks'
VAR
    StartEdge : R_TRIG;
    AllInterlocksOK : BOOL;
    MotorRun : BOOL;
    StartCommand : BOOL;
    EStopOK : BOOL := FALSE;
    OverloadOK : BOOL := TRUE;
    LubeOK : BOOL := TRUE;
    StopPB : BOOL := TRUE;
    StartPB : BOOL;
END_VAR
    AllInterlocksOK := EStopOK AND OverloadOK AND LubeOK AND StopPB;
    ASSERT_FALSE(AllInterlocksOK);

    StartPB := TRUE;
    StartEdge(CLK := StartPB);

    IF StartEdge.Q AND AllInterlocksOK THEN
        StartCommand := TRUE;
    END_IF;
    ASSERT_FALSE(StartCommand, 'Should not start with bad interlocks');

    MotorRun := StartCommand AND AllInterlocksOK;
    ASSERT_FALSE(MotorRun);
END_TEST_CASE
```

### Testing Timer-Based Logic

```iec
TEST_CASE 'TON: output delayed by preset time'
VAR
    t : TON;
END_VAR
    t(IN := TRUE, PT := T#200ms);
    ASSERT_FALSE(t.Q);

    ADVANCE_TIME(T#100ms);
    t(IN := TRUE, PT := T#200ms);
    ASSERT_FALSE(t.Q, 'Should not fire at 100ms');

    ADVANCE_TIME(T#150ms);
    t(IN := TRUE, PT := T#200ms);
    ASSERT_TRUE(t.Q, 'Should fire at 250ms > 200ms preset');
END_TEST_CASE
```

### Testing with Edge Detectors

```iec
TEST_CASE 'R_TRIG detects rising edge'
VAR
    edge : R_TRIG;
END_VAR
    (* First call with FALSE -- no edge *)
    edge(CLK := FALSE);
    ASSERT_FALSE(edge.Q);

    (* Second call with TRUE -- rising edge detected *)
    edge(CLK := TRUE);
    ASSERT_TRUE(edge.Q);

    (* Third call with TRUE -- no edge (already high) *)
    edge(CLK := TRUE);
    ASSERT_FALSE(edge.Q);
END_TEST_CASE
```

### Testing Counter Logic

```iec
TEST_CASE 'CTU counts up and resets'
VAR
    counter : CTU;
    pulse : BOOL;
END_VAR
    (* Count 3 pulses *)
    counter(CU := TRUE, PV := 5);
    counter(CU := FALSE, PV := 5);
    counter(CU := TRUE, PV := 5);
    counter(CU := FALSE, PV := 5);
    counter(CU := TRUE, PV := 5);

    ASSERT_EQ(counter.CV, 3, 'Should count 3 pulses');
    ASSERT_FALSE(counter.Q, 'Not at preset yet');

    (* Count 2 more to reach preset *)
    counter(CU := FALSE, PV := 5);
    counter(CU := TRUE, PV := 5);
    counter(CU := FALSE, PV := 5);
    counter(CU := TRUE, PV := 5);

    ASSERT_EQ(counter.CV, 5);
    ASSERT_TRUE(counter.Q, 'Should be at preset');

    (* Reset *)
    counter(CU := FALSE, RESET := TRUE, PV := 5);
    ASSERT_EQ(counter.CV, 0);
    ASSERT_FALSE(counter.Q);
END_TEST_CASE
```
