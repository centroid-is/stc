# EtherCAT Simulation

stc can run a TwinCAT program against the process image its EtherCAT masters would present on the target. The I/O tree comes from TwinCAT's own exports, and variables are wired to it through the `{attribute 'TcLinkTo' := '...'}` pragmas already in the project. No hardware and no TwinCAT runtime are needed.

## Inputs

- **EtherCATConfig exports.** In TwinCAT, export each EtherCAT master's configuration (`Device N.xml`). Each file becomes one master.
- **ST sources.** The GVLs and PROGRAMs that carry `TcLinkTo` pragmas, plus the DUTs and function blocks their linked members are declared in. Linked leaves should be declared `AT %I*` or `AT %Q*`.

Link paths follow the convention `generate_gvl.py` writes: `TIID^<master>^<coupler>^<box>^<pdo or module>^<entry>`. Terminals nest under the EK coupler or CX head they hang off.

## Validate the links

```bash
stc ecat validate --io "Device 1.xml" --io "Device 2.xml" DUTs/*.st ECT.st ECT_Diag.st
```

Every link is resolved to a byte and bit in a master's input or output image. Unresolved targets, undeclared members, size and direction mismatches and malformed values are errors. See `stc ecat validate` in [CLI_REFERENCE.md](CLI_REFERENCE.md) for the output formats and the ECAT001 to ECAT007 codes.

## Run against the image

The Go API wires the resolved bindings into the scan cycle:

```go
topo, err := ecat.LoadProject("Device 1.xml", "Device 2.xml")
vars, _ := ecat.CollectLinks(files)          // parsed ST files
bindings, diags := ecat.Resolve(topo, vars)  // stop on error diagnostics
net := ecat.NewNetwork(topo, nil)            // nil uses ecat.DefaultRegistry
engine.SetIOBinder(interp.NewIOBinder(bindings, net))
engine.Tick(10 * time.Millisecond)
```

On every `Tick`, the network steps first and linked inputs are copied into their variables before the program body runs. Linked outputs are copied into the output image after it runs. Tests read and write the images through `net.Images().Get(master)` and `topo.Slot(path)`.

## Run a whole project with `--io`

`stc sim` and `stc serve` attach the network to a project from the command
line. The TcLinkTo links of the project's GVLs and PROGRAMs are resolved
against the exports, with member links resolved through struct types
declared in the project or in its resolved libraries (ST301's ECT terminal
structs live in SVNCoreComponents). Any unresolved link fails the load
with every diagnostic printed.

```bash
stc sim "ST301 solution.tsproj" --io "Device 1.xml" --io "Device 2.xml" \
    --io "Device 3.xml" --io "Device 4.xml" --cycles 1000 --format json
stc serve "ST301 solution.tsproj" --io "Device 1.xml" ... --opcua :4840 --realtime
```

ST301 is a proprietary production project. Its sources and exports stay local, so these two commands only run on a machine that has them. The same commands work on the repository fixture:

```bash
stc sim cmd/stc-mcp/testdata/live --io "tests/ecat_fixtures/Demo Device 1.xml" --cycles 50
```

```text
Project: 50 cycles, sim time 500ms

TASK     CYCLE  PRIORITY  PROGRAMS  RUNS  OVERRUNS
PlcTask  10ms   0         MAIN      50    0
```

Linked and AT-bound variables are decoded by their declared type, not by
the slot width alone: an `AT %I*` INT whose two slot bytes are `FB FF`
reads -5 (0xFFFB), a REAL slot is IEEE 754 single precision, an enum uses
its base type, and AT members of a struct get their own slots. Outputs are
encoded the same way, so -5 written to an INT output becomes `FB FF`.

## Healthy defaults and faults

Without any device model, the network reports a healthy bus. Every slave is in OP with a clear working counter, `SlaveCount` matches the export, and `AmsNetId` and each slave's `AdsAddr` are filled in. `SetMasterNetID` changes a master's AmsNetId.

Faults are injected through the network:

| Call | Effect |
|------|--------|
| `SetSlaveState(master, slave, state)` | Override one slave's `InfoData^State` |
| `SetWcState(master, slave, bad)` | Set or clear one slave's working counter error |
| `SetDevState(master, bits)` | Set the master's `Inputs^DevState` |
| `ClearFaults()` | Return to the healthy defaults |

Overrides show up on the next `Tick`.

## Device models

Each slave runs a `Device` chosen from a `Registry` by vendor and product id. Unknown slaves use the `Passthrough` model, which leaves inputs as the test wrote them. Package `pkg/ecat/devices` registers these behavioural models. `stc sim --io` and `stc serve --io` use them automatically, and scenario files drive them through the fault API (see [Scenarios](#scenarios)):

| Model | Slaves | Behaviour |
|-------|--------|-----------|
| Passive | EK1100, EK1110, EK1200, EL6070, EL9011 | Couplers and passive terminals; healthy state only |
| Digital I/O | EL1008, EL1018, EL2008, EP2338, Festo CTEU | Channels numbered per direction; tests set inputs and read outputs by channel |
| Analog input | EL3054 (4 to 20 mA), EL3064 (0 to 10 V) | `SetCurrent`, `SetVoltage` and `SetRaw` with the terminal's scaling and status bits |
| Overcurrent protection | EL9222-5500 | Two channels with switch, trip and rising-edge reset |
| Power supply | PS2001-2410 | Healthy by default (DC OK); voltage, current and fault states can be set |
| Safety diagnostics | EL2912, EP1918-0002, EL1904 | Non-safe field voltage diagnostics; FSoE frames pass through |
| Serial | EL6001, EL6002 | RS232 handshake and FIFOs with a scripted peer (Baader, loopback or none) |
| Drive | Schneider ATV320 | CiA402 state machine, ramps and SDO object dictionary, described below |

### ATV320 drive

`devices.ATV320` models a Schneider Altivar 320 with the EtherCAT option (VendorId 0x0800005A, ProductCode 0x389). It boots in PreOp and needs a state request to reach OP. CMD drives a CiA402 state machine and ETA reports its status word. RFR ramps toward LFR at the ACC and DEC times, clamped to LSP and HSP. HMIS, LFT, LCR and DI report drive status. An object dictionary answers SDO reads and writes for the parameters FB_ATV320 configures, including the EEPROM save object 0x2032:01. Tests can call `InjectFault`, `ClearFault`, `SetDI` and `SetSTO`. See the package documentation for units and status words.

## Tc2_EtherCAT function blocks

Programs that call the Tc2_EtherCAT library talk to the simulated master through Go mocks of its function blocks. These include slave and master state, set state, CRC counters, physical write and CoE SDO read and write. Attach them before the first scan:

```go
net := ecat.NewNetwork(topo, nil)
eng.SetNetwork(net)                        // before FBs are instantiated
eng.SetIOBinder(interp.NewIOBinder(bindings, net)) // same network
```

Each request stays busy for two scans, then reports done or an ADS or CoE error code. `F_CreateAmsNetId` is available too. With this wiring the unmodified SVNCoreComponents `FB_ATV320` configures the simulated drive from PreOp to `cfgReady` and runs it, and `FB_EcDeviceDiag` fills its diagnostic records. `pkg/interp/ecat_e2e_test.go` shows the full setup.

## Scenarios

A scenario is a TOML file that drives a whole project and its EtherCAT network through a timeline of inputs, faults and expectations. Run it with `stc sim`:

```bash
stc sim MAIN.st ECT.st ECT_Diag.st demo_types.st --io "Demo Device [12].xml" --scenario jam.toml --format json
stc sim "ST301 solution.tsproj" --io "Device *.xml" --scenario st301_jam.toml
```

The same stimuli are available from ST test code in `stc test` project mode (see the Testing Guide, "Testing against the simulated plant").

### File layout

```toml
[scenario]
name = "jam"                 # report name (default: the file path)
description = "..."          # free text
cycles = 3000                # optional run length in Ticks (--cycles overrides)

[[step]]
cycle = 0                    # trigger: exactly one of cycle or at
set = { path = "ECT.A1_01.I1", value = true }   # at most one action
expect = { path = "ECT.A1_02.O1", value = true, within = 2 }  # optional
```

Each `[[step]]` has one trigger, at most one action and at most one `expect`. A step needs an action, an expect or both. Unknown keys are SCN002 errors.

### Triggers

| Key | Value | Fires before |
|-----|-------|--------------|
| `cycle` | integer >= 0 | Tick k where k (completed Ticks) equals the value |
| `at` | duration string (`"150ms"`, `"2s"`) | the first Tick k with k * BaseTick >= the duration |

Steps due on the same Tick fire in file order.

### Actions

| Action | Fields | Effect |
|--------|--------|--------|
| `set` | `path`, `value` | A TcLinkTo-bound input variable is forced on its process-image slot (encoded by the slot type). Any other variable is written directly. A variable with an explicit `AT %I` address and no TcLinkTo is an SCN007 error. |
| `link` | `path` (a `TIID^...` link path), `value` | Forces that input slot of the process image. Output slots are an error. |
| `analog` | `slave`, `channel`, exactly one of `ma`, `volts`, `raw` | Sets an analog input channel through the terminal model (EL3xxx). |
| `trip` | `slave`, `channel` | Trips an EL9222 OCP channel: Enabled goes FALSE, Tripped TRUE. |
| `slave_state` | `slave`, `state` (preset name or integer) | Changes the slave's state, link and working counter (presets below). |
| `drive_fault` | `slave`, `lft` (integer fault code, 0 clears) | Injects a fault in an ATV320 model: the CiA402 fault bit in ETA and the code in LFT. |
| `ramp` | `path`, `from`, `to`, `over`; or `slave`, `channel`, `unit` (`"mA"` or `"V"`), `from`, `to`, `over` | Linear ramp written before every Tick until `over` has elapsed. |
| `serial_peer` | `slave`, `script` (`"baader"`, `"loopback"`, `"none"`) | Attaches a scripted serial peer to an EL6001 (`none` detaches it). |

### Expect

| Field | Meaning |
|-------|---------|
| `path` | Variable path (`GVL.member`, `MAIN.x`, array and struct paths) or a `TIID^...` link path |
| `value` | Expected value. Enums compare by name. |
| `within` | Optional number of Ticks after the firing Tick in which the value must be seen (0 to 1 000 000). Without it the value must hold after the firing Tick. |
| `tol` | Optional absolute tolerance for numeric values (>= 0) |

An expect still pending when the run ends fails with "run ended at cycle N".

### Slave names

`slave` matches the export's slave name exactly first, then case-insensitively, then by the unique prefix before ` (`. For example, `DEMO.A1.03` matches `DEMO.A1.03 (EL9222-5500)`. Unknown and ambiguous names are SCN006 errors that list up to five candidates. A name present on several masters is an error naming the masters. A slave whose model lacks the action's API is SCN006, naming its actual model.

### slave_state presets

| Preset | InfoData.State | Link state | WcState |
|--------|----------------|------------|---------|
| `not_present` | 0x0011 (Init, error bit) | not present | bad |
| `link_error` | 0x0011 | link without communication | bad |
| `init` | state nibble 1 | unchanged | unchanged |
| `preop` | state nibble 2 | unchanged | unchanged |
| `safeop` | state nibble 4 | unchanged | unchanged |
| `op` | state nibble 8 | unchanged | unchanged |
| `ok` | 0x0008 (OP) | 0 | good |
| integer | the value as given | unchanged | unchanged |

These are the values FB_EcDeviceDiag decodes: the state nibble, the 16#10 error bit and `linkState <> 0`.

### Timing rules

- The executor owns the loop. Before each Tick it fires every due step, in due-Tick then file order. After each Tick it evaluates the pending expects. Nothing reads the wall clock, so two runs give byte-identical JSON.
- Ramps write `from + (to - from) * min(1, (clock - t0) / over)` before every Tick from the firing Tick until the fraction reaches 1. Integer targets round half away from zero. A new ramp, set, link or analog action on the same target cancels a running ramp.
- Run length: `--cycles` wins, then `[scenario] cycles`. Without either, the run ends at the last due Tick plus the largest `within` plus 1. Steps due after the end are SCN010 warnings.
- Inputs are forced on the process image and stay forced, because the binder copies input slots into linked variables every scan. A linked input therefore reads back after the next Tick.
- Limits: at most 1 MiB per file and 10 000 steps. `over` must be in (0, 24h], `within` at most 1 000 000, and `channel` in 1..64.

### Diagnostics

| Code | Meaning |
|------|---------|
| SCN001 | TOML syntax error (with line) |
| SCN002 | Unknown key |
| SCN003 | Trigger error (missing, both, or out of range) |
| SCN004 | Action count or shape error |
| SCN005 | Bad duration or out-of-range number |
| SCN006 | Unknown or wrong-model slave, or no `--io` network loaded |
| SCN007 | Unknown or unsettable path |
| SCN008 | Action failed at run time |
| SCN009 | Expect failed |
| SCN010 | Step never fired (warning) |

Every step is validated against the project before the first Tick. Any error stops the run before it starts.

### Example

`tests/ecat_fixtures/scenario/jam.toml` runs against the Demo Device 1/2 fixture project:

```toml
# Jam scenario on the Demo Device 1/2 fixture project (10 ms scans).
[scenario]
name = "jam"
description = "Photo eye on, OCP trip, EL1008 pulled, drive fault; setpoint ramp"

[[step]]
cycle = 0
set = { path = "ECT.A1_01.I1", value = true }
expect = { path = "ECT.A1_02.O1", value = true, within = 2 }

[[step]]
at = "50ms"
ramp = { path = "ECT_Diag.rSetpoint", from = 0.0, to = 50.0, over = "100ms" }

[[step]]
at = "100ms"
trip = { slave = "DEMO.A1.03 (EL9222-5500)", channel = 1 }
expect = { path = "ECT.A1_03.p_stat_Enabled", value = false, within = 2 }

[[step]]
at = "100ms"
expect = { path = "ECT.A1_02.O1", value = false, within = 3 }

[[step]]
at = "160ms"
expect = { path = "ECT_Diag.rSetpoint", value = 50.0, tol = 1e-3 }

[[step]]
at = "190ms"
expect = { path = "ECT_Diag.Device_1_Diag[2].p_stat_bOk", value = true }

[[step]]
at = "200ms"
slave_state = { slave = "DEMO.A1.01 (EL1008)", state = "not_present" }
expect = { path = "ECT_Diag.Device_1_Diag[2].p_stat_bOk", value = false, within = 20 }

[[step]]
at = "290ms"
expect = { path = "MAIN.xDriveFault", value = false }

[[step]]
at = "300ms"
drive_fault = { slave = "DEMO.CN01.FD01 (ATV320 EtherCAT)", lft = 16 }
expect = { path = "MAIN.xDriveFault", value = true, within = 10 }
```

The JSON result adds `scenario` (steps, assertions, passed), `outputs`, `ethercat` (slaves not healthy at the end) and `diagnostics`. The exit code is 1 when an assertion fails.

```bash
stc sim project.tsproj --io "Device*.xml" --scenario trip.toml --cycles 1000 --format json
```

### Live mode

`stc serve --scenario` fires the same steps as Ticks elapse in free-running mode. Steps are applied after pending OPC UA writes and before each Tick, so OPC UA clients see the stimuli as data changes. Expectations are evaluated as in `stc sim`, and the result is reported when the server stops. `stc-mcp --scenario` drives the MCP simulation the same way.

```bash
stc serve project.tsproj --io "Device*.xml" --scenario trip.toml --opcua :4840
```

## ST built-ins for simulation

Tests and programs built with `STC_SIM` can drive the Plant from ST:

| Built-in | Effect |
|----------|--------|
| `SET(path, value)`, `GET(path)` | Set or read any variable by path |
| `RUN_CYCLES(n)` | Run n project ticks |
| `ADVANCE_TIME(d)` | Run d divided by the base tick |
| `SIM_SET_LINK(link, value)` | Force a link path |
| `SIM_TRIP(slave, ch)` | Trip an EL9222 channel |
| `SIM_SLAVE_STATE(slave, state)` | Set a slave state by name or number |
| `SIM_ANALOG(slave, ch, value, unit)` | Drive an analog input |
| `SIM_DRIVE_FAULT(slave, lft)` | Inject or clear an ATV320 fault |
| `SIM_RAMP(path, from, to, over)` | Start a ramp that advances during `RUN_CYCLES` |
| `SIM_SERIAL_PEER(slave, script)` | Attach a serial peer |

See the project-mode section of [TESTING_GUIDE.md](TESTING_GUIDE.md).

