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

Each slave runs a `Device` chosen from a `Registry` by vendor and product id. The default `Passthrough` model leaves inputs as the test wrote them. Behavioural models for specific terminals and drives are planned for Phases 25 and 26, and scenario scripting on top of the fault API for Phase 27.
