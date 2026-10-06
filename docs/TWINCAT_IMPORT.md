# TwinCAT Project Import

stc reads a Beckhoff TwinCAT 3 project directly. You do not export or convert anything first. `stc check`, `stc sim`, `stc serve` and `stc test --project` all accept a `.tsproj` or `.plcproj`, and `stc vendor import` reports what the importer found.

## Supported inputs

| Input | What stc does with it |
|-------|----------------------|
| `.tsproj` (TwinCAT solution) | Finds the PLC project, its tasks with cycle times and priorities, and the AMS port |
| `.plcproj` (PLC project) | Lists the Compile items in project order and the library references |
| `.TcPOU` | PROGRAM, FUNCTION_BLOCK and FUNCTION with methods, properties, actions and transitions |
| `.TcGVL` | Global variable lists, named after the GVL |
| `.TcDUT` | STRUCT, ENUM, UNION and alias types |
| `.TcIO` | Interfaces with their methods and properties |
| `.TcTTO` | Task objects; the cycle time is cross-checked with the `.tsproj` |
| plain `.st` files and directories | Used as they are; a default task runs `MAIN` every 10 ms |

The importer reads the CDATA of each `<Declaration>` and `<ST>` element and keeps its original file and line. Diagnostics therefore point at the `.TcPOU` line you would open in TwinCAT.

Items the importer cannot use are reported and skipped. Visualizations (`.TcVIS`), TwinSAFE projects and implementations in languages other than ST fall in this group.

## What is imported

- **POUs, GVLs, DUTs and interfaces** from every Compile item of the PLC project.
- **Tasks**: name, cycle time, priority and the PROGRAMs each task calls. `stc sim` and `stc serve` run them on that schedule.
- **Library references**: each placeholder reference is resolved in this order.

| Resolution | Meaning |
|------------|---------|
| `sibling` | A library project next to the solution, imported from source |
| `library_paths` | A `.st` stub directory listed in `stc.toml` |
| `stub` | A Beckhoff stub shipped inside stc (Tc2_EtherCAT, Tc2_System, Tc3_Module, Tc2_SerialCom and others) |
| `builtin` | A library stc implements natively, such as Tc2_Standard |
| `unresolved` | Not found; a VEND020 warning is printed and calls into it are auto-stubbed |

Function blocks that come only from a stub run as zero-output auto-stubs unless stc ships a behavioural mock. The test output marks each one with a `[fidelity] auto-stub` line. See [VENDOR_LIBRARIES.md](VENDOR_LIBRARIES.md) for the stub format.

## TwinCAT declaration syntax

TwinCAT allows declarations that plain IEC 61131-3 does not. stc parses and type-checks them:

- `AT %I*` and `AT %Q*` wildcard addresses, plus explicit `%IX0.0` style addresses.
- `{attribute 'qualified_only'}` GVLs, which must be accessed as `GVL.name`.
- Methods, properties, `THIS^`, `SUPER^`, `EXTENDS`, `IMPLEMENTS`, `POINTER TO` and `REFERENCE TO`.
- Bit access such as `word.3`, named and empty call arguments, and actions.

`tests/twincat_dialect` holds one test file per construct.

## Attributes are preserved

Every `{attribute '...'}` pragma is kept on its declaration. Several of them change what stc does:

| Attribute | Used by |
|-----------|---------|
| `TcLinkTo` | Binds a variable to an EtherCAT slot when `--io` is given. See [ETHERCAT_SIMULATION.md](ETHERCAT_SIMULATION.md) |
| `OPC.UA.DA`, `OPC.UA.DA.Access`, `OPC.UA.DA.Description`, `OPC.UA.DA.StructuredType` | Decide what `stc serve --opcua` publishes. See [OPCUA.md](OPCUA.md) |
| `qualified_only` | Enforced by the checker |

An attribute name in double quotes is ignored by TwinCAT, so stc warns about it.

## Diagnostics

| Code | Severity | Meaning |
|------|----------|---------|
| VEND020 | warning | A library reference resolved nowhere |
| VEND021 | warning | A plcproj Compile item with an unsupported extension was skipped |
| VEND022 | info | A TwinSAFE project, extra PLC project or non-ST implementation was skipped |
| VEND023 | warning | A CDATA segment could not be placed at its XML line |
| VEND024 | warning | No task information; a 10 ms default task is used |
| VEND025 | warning | The `.tsproj` and `.TcTTO` cycle times disagree |
| VEND026 | warning | More than one sibling library candidate; the first sorted path is used |
| VEND027 | error | A project or object file is not valid XML |
| VEND028 | warning | A plcproj item path matches a file on disk only when case is ignored; the on-disk spelling is used |

## Worked example

The repository ships a small solution at `pkg/twincat/testdata/sln`. It has one PLC project, a sibling library, a 1 ms task and a test directory. Paths in the output below are shortened.

Inspect the project:

```bash
stc vendor import "pkg/twincat/testdata/sln/Demo/Demo solution.tsproj"
```

```text
PLC Demo (AMS port 851)
Project: .../sln/Demo/Demo/Demo.plcproj
Task PlcTask: cycle 1ms, priority 20, programs MAIN
Sources: 3 POU(s), 2 GVL(s), 2 DUT(s), 1 interface(s)
Libraries: 7
  DemoLib: sibling (.../sln/DemoLib/DemoLib/DemoLib.plcproj)
  Tc2_EtherCAT: stub
  Tc2_Standard: builtin
  Tc2_System: stub
  Tc3_Module: stub
  Tc2_SerialCom: stub
  Tc2_Missing: unresolved
1 task(s), 8 source(s), 7 librar(ies), 8 library source file(s)
.../sln/Demo/Demo solution.tsproj:0:0: info: TwinSAFE project Safe.xti skipped [VEND022]
.../sln/Demo/Demo/Demo.plcproj:34:5: warning: unsupported plcproj item Visu\Screen.TcVIS skipped [VEND021]
.../sln/Demo/Demo/Demo.plcproj:64:5: warning: unresolved library reference 'Tc2_Missing' [VEND020]
```

Type-check it:

```bash
stc check "pkg/twincat/testdata/sln/Demo/Demo solution.tsproj"
```

```text
.../sln/Demo/Demo/GVLs/GVL_Quoted.TcGVL:4:27: warning: attribute name in double quotes is ignored by TwinCAT; use single quotes
0 error(s), 3 warning(s)
(8 source(s), 8 library source(s) from Demo)
```

Run 100 base ticks of its task schedule and read two variables:

```bash
stc sim "pkg/twincat/testdata/sln/Demo/Demo solution.tsproj" --cycles 100 --set GVL_Main.mode=Run --get GVL_Main.counter --get MAIN.spd
```

```text
Project: 100 cycles, sim time 100ms

TASK     CYCLE  PRIORITY  PROGRAMS  RUNS  OVERRUNS
PlcTask  1ms    20        MAIN      100   0

GVL_Main.counter = 100
MAIN.spd = 0
```

Run the project's ST unit tests against the imported sources:

```bash
stc test pkg/twincat/testdata/sln/tests --project "pkg/twincat/testdata/sln/Demo/Demo solution.tsproj"
```

```text
ok
2 tests, 2 passed, 0 failed
```

## Work on a plain-ST copy

`stc vendor import --out <dir>` writes the project as `.st` files in DUTs, GVLs and POUs folders. It also writes the libraries under `libs/` and an `stc.toml`, so `stc check`, `stc lsp` and `stc test` work on the copy without TwinCAT XML.

```bash
stc vendor import "pkg/twincat/testdata/sln/Demo/Demo solution.tsproj" --out demo_st
```

`stc vendor extract <x.plcproj>` writes declaration-only stubs instead. Use it to turn a library project into stubs for `[build.library_paths]`.

## Proprietary projects

Production projects such as ST301 stay on your machine. The tests that use them read the path from `STC_SILD_DIR` and skip when it is unset. Never commit their sources, their EtherCAT exports or a snapshot of their OPC UA server without deciding to do so.

## Next steps

- Attach the EtherCAT I/O with `--io`: [ETHERCAT_SIMULATION.md](ETHERCAT_SIMULATION.md).
- Serve the running project to an HMI over OPC UA: [OPCUA.md](OPCUA.md).
- Every flag: [CLI_REFERENCE.md](CLI_REFERENCE.md).
