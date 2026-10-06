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

Each slave runs a `Device` chosen from a `Registry` by vendor and product id. Unknown slaves use the `Passthrough` model, which leaves inputs as the test wrote them. Package `pkg/ecat/devices` registers these behavioural models, and `stc sim --io` and `stc serve --io` use them automatically:

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

> **Lands with Phase 27.** The scenario format and the `--scenario` flag below are specified in Phase 27 and are not in this build yet. The flag is reserved in `stc-mcp`, which returns an error when it is used.

A scenario is a TOML file of timed stimuli and expectations. It drives the same Plant the ST built-ins use, so a scenario run is deterministic and two runs produce identical JSON.

```toml
[scenario]
name = "Sensor trips the conveyor"
cycles = 500

[[step]]
at = "100ms"
set = { path = "GVL_IO.xSensor", value = true }
expect = { path = "GVL_Conveyor.Line1.HMI.p_stat_Running", value = false, within = 5 }

[[step]]
cycle = 300
analog = { slave = "ST301.A1.05", channel = 1, ma = 12.0 }
```

Each step has exactly one trigger. `at` is a Go duration on the simulation clock, and `cycle` is a count of completed ticks.

| Action key | Fields | Effect |
|------------|--------|--------|
| `set` | `path`, `value` | Sets a variable through the runtime; a TcLinkTo-bound input or a path with `^` is forced in the process image |
| `link` | `path`, `value` | Forces a `TIID^...` link path; integers only |
| `analog` | `slave`, `channel`, one of `ma`, `volts`, `raw` | Drives an analog input model |
| `trip` | `slave`, `channel` | Trips an EL9222 channel |
| `slave_state` | `slave`, `state` | `init`, `preop`, `safeop`, `op`, `not_present`, `link_error`, `ok` or an integer |
| `drive_fault` | `slave`, `lft` | Injects an ATV320 fault; `lft = 0` clears it |
| `ramp` | `path`, `from`, `to`, `over` (or `slave`, `channel`, `unit`) | Linear ramp over simulation time |
| `serial_peer` | `slave`, `script` | `baader`, `loopback` or `none` |

A step has at most one action and may carry one `expect = {path, value, within, tol}`. The expect passes when the value matches after any tick from the firing tick to `within` ticks later. `tol` defaults to 1e-6 for REAL and LREAL. Integers and BOOL compare exactly, and enums compare by name without regard to case. Expects still pending when the run ends fail.

Slave names match the export's `Name` exactly, then without case, then as the unique prefix before ` (`.

| Code | Meaning |
|------|---------|
| SCN001 | TOML syntax error |
| SCN002 | Unknown key |
| SCN003 | Missing or double trigger |
| SCN004 | Wrong action count or shape |
| SCN005 | Bad duration or out-of-range number |
| SCN006 | Unknown slave, wrong model or no `--io` network |
| SCN007 | Unknown or unsettable path |
| SCN008 | Action failed at run time |
| SCN009 | Expect failed |
| SCN010 | Step never fired (warning) |

Every step is validated before the first tick, so any error stops the run before it starts.

```bash pending-phase-27
stc sim project.tsproj --io "Device*.xml" --scenario trip.toml --cycles 1000 --format json
```

The JSON result adds `scenario` (steps, assertions, passed), `outputs`, `ethercat` (slaves not healthy at the end) and `diagnostics`. The exit code is 1 when an assertion fails.

### Live mode

`stc serve --scenario` will fire the same steps as ticks elapse in free-running mode. OPC UA clients then see the stimuli as data changes. This mode is plan 29-01 and needs Phase 27.

```bash pending-phase-27
stc serve project.tsproj --io "Device*.xml" --scenario trip.toml --opcua :4840
```

## ST built-ins for simulation

> **Lands with Phase 27.**

Tests and programs built with `STC_SIM` can drive the Plant from ST:

| Built-in | Effect |
|----------|--------|
| `SET(path, value)`, `GET(path)` | Set or read any variable by path |
| `RUN_CYCLES(n)` | Run n project ticks |
| `SIM_SET_LINK(link, value)` | Force a link path |
| `SIM_TRIP(slave, ch)` | Trip an EL9222 channel |
| `SIM_SLAVE_STATE(slave, state)` | Set a slave state by name or number |
| `SIM_ANALOG(slave, ch, value, unit)` | Drive an analog input |
| `SIM_DRIVE_FAULT(slave, lft)` | Inject or clear an ATV320 fault |
| `SIM_RAMP(path, from, to, over)` | Start a ramp that advances during `RUN_CYCLES` |
| `SIM_SERIAL_PEER(slave, script)` | Attach a serial peer |

See the project-mode section of [TESTING_GUIDE.md](TESTING_GUIDE.md).
