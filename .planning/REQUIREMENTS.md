# Requirements: STC v1.2 -- TwinCAT Import, EtherCAT Simulation & OPC UA

**Defined:** 2026-10-05
**Core Value:** Write ST once, validate it instantly on your machine, and deploy to any supported PLC vendor -- no hardware required for development and testing.

Reference project for every acceptance test: `/Users/jonb/Projects/sildarvinnsla` (ST301 + SVNCoreComponents, plus Baader for serial). Research: `.planning/research/v1.2/`.

## v1.2 Requirements

### TwinCAT dialect parity (DIAL)

- [x] **DIAL-01**: `{attribute 'name' := 'value'}` pragmas (single- or double-quoted, `''` escapes, blank lines before the declaration) are retained in the AST on VarDecl, StructMember, EnumValue, TypeDecl and POU nodes, appear in `stc parse --format json`, and round-trip through `stc fmt` and `stc emit`
- [x] **DIAL-02**: A file whose top level is `VAR_GLOBAL [PERSISTENT] [RETAIN] [CONSTANT] ... END_VAR` parses as a GVL declaration named from the file (or `--gvl-name`); `qualified_only` enforces `GVL.x` access in the checker
- [x] **DIAL-03**: `AT %I*` / `AT %Q*` is accepted on STRUCT members and on FB `VAR`/`VAR_INPUT`/`VAR_OUTPUT` without warnings (explicit `%IX..` addresses in FBs keep SEMA031)
- [x] **DIAL-04**: Bit access `x.N` on BYTE/WORD/DWORD/LWORD variables, struct members and array elements works for read and write with bounds checked by the checker
- [x] **DIAL-05**: Empty formal arguments in FB calls (`PT := ,` and `Q => ,`) parse and are ignored at runtime
- [x] **DIAL-06**: Named arguments in FUNCTION calls used as expressions (`n := F(a := 1, b := 2)`) parse, type-check and execute
- [x] **DIAL-07**: Qualified enum values (`E.v`) work in expressions, CASE labels, CASE label lists, initialisers and comparisons; enum declarations with a base type (`(a := 0, b := 1) UINT`) and the `strict`/`to_string` attributes are accepted
- [x] **DIAL-08**: `ACTION name ... END_ACTION` blocks (CODESYS text form after the POU and TcPOU `<Action>` XML) parse and are callable as `name()` inside their POU
- [x] **DIAL-09**: `REF=`, `THIS^` and `SUPER^` parse, type-check and execute
- [x] **DIAL-10**: `stc check` on the flattened ST301 + SVNCoreComponents sources (`.planning/research/v1.2` probes) reports zero parse errors

### TwinCAT project import (IMPT)

- [x] **IMPT-01**: `stc vendor import <x.tsproj|x.plcproj>` reads TcPOU (declaration, implementation, methods, actions, properties), TcGVL and TcDUT files listed in the plcproj and builds one project model stc can check and run
- [x] **IMPT-02**: Library placeholder references in the plcproj resolve in order: project POUs, sibling library plcproj (SVNCoreComponents), shipped stubs; unresolved references are reported as diagnostics
- [x] **IMPT-03**: Task cycle time and PLC project name are read from the `.tsproj` and used by the runtime and the OPC UA namespace
- [x] **IMPT-04**: Shipped stubs cover Tc2_EtherCAT (`FB_EcGetSlaveState`, `FB_EcGetAllSlaveStates`, `FB_EcSetSlaveState`, `FB_EcGetMasterState`, `FB_EcGetAllSlaveCrcErrors`, `FB_EcGetSlaveCrcErrorEx`, `FB_EcCoESDoRead`, `FB_EcCoESDoWrite`, `FB_EcPhysicalWriteCmd`, `ST_EcSlaveState`; `E_EcSlaveState` is declared by SVNCoreComponents, not Beckhoff), Tc2_System additions (`AMSADDR`, `T_AmsNetIdArr`, `F_CreateAmsNetId`, `MEMCPY`), Tc2_ModbusSrv, Tc3_Module and Tc2_SerialCom so the sildarvinnsla projects type-check
- [x] **IMPT-05**: `stc vendor extract` emits stubs that parse (closing keywords, methods included) and no longer silently skips TcGVL/TcDUT entries

### Runtime model (RUNT)

- [x] **RUNT-01**: After analysis a symbol tree exists for every GVL, PROGRAM, FB instance, struct member and array element with IEC type, layout, enum strings and attributes, addressable by dotted path (`GVL.fb[2].HMI.p_stat_State`)
- [x] **RUNT-02**: The live interpreter supports `Get(path)` / `Set(path, value)` by dotted path with type coercion, usable from Go, tests, MCP and servers
- [x] **RUNT-03**: All GVLs are instantiated once and PROGRAMs run per task with the configured cycle time; `Tick()` stepping stays deterministic and a free-running mode paces the scan against wall-clock for interactive use
- [x] **RUNT-04**: `PERSISTENT`/`RETAIN` variables load from and save to a state file so `p_cfg_*` values survive restarts
- [x] **RUNT-05**: Integer arithmetic wraps per declared type (INT 32767+1 = -32768, UINT 0-1 = 65535) and untyped literals adopt the context type so `a := a + 1` checks for INT
- [x] **RUNT-06**: Array, struct and struct-array initialisers (`:= [(a := 1, s := 'x'), ...]`) with constant-expression bounds are applied at instantiation
- [x] **RUNT-07**: AT-bound variables read and write by declared type (sign-extended INT, REAL, enums, structs with `AT %I*` members) rather than by address width
- [x] **RUNT-08**: `stc check` knows the standard FBs (TON, TOF, TP, CTU, CTD, CTUD, R_TRIG, F_TRIG, SR, RS) and rejects unknown type names instead of treating them as empty FBs
- [x] **RUNT-09**: `stc sim` and `stc serve` can run PROGRAMs that use user-defined FBs, functions, methods and actions from the imported project

### EtherCAT process-image simulation (ECAT)

- [x] **ECAT-01**: `pkg/ecat` loads TwinCAT `EtherCATConfig` exports (the `Device N.xml` files) into masters, slaves (name, model, vendor, product, phys addr, port physics, parent coupler) and their active TxPdo/RxPdo entries, reproducing the E-bus nesting and `Module N` segments used by `generate_gvl.py`
- [x] **ECAT-02**: `TcLinkTo` pragma strings (single target and multi-member `.m := path; ...` form) are parsed and resolved against the loaded topology to a (master, byte, bit) slot; `stc ecat validate` reports unresolved links and type-size mismatches with file positions
- [x] **ECAT-03**: Each master has an input and output process image; `AT %I*`/`%Q*` variables, struct members and FB members bound by `TcLinkTo` are copied from/to their slots at scan boundaries
- [x] **ECAT-04**: Device models selected by (VendorId, ProductCode) implement a common `Slave` interface and ship for EL1008/EL1018, EL2008, EP2338-0002/-1002, Festo CTEU outputs, EL3054/EL3064 (status word + scaled INT), EL9222-5500 (per-channel status/control with trip injection), PS2001-2410, EL2912/EP1918/EL1904 standard diagnostics, and couplers/passive terminals with no PDOs
- [x] **ECAT-05**: An ATV320 model implements the CiA402 state machine on CMD/ETA, frequency reference LFR to RFR with ACC/DEC ramps, LCR current, HMIS and LFT codes, DI/OL1R logic I/O, and a CoE object dictionary (0x6040/0x6041, 0x2002, 0x2016, 0x2029, 0x2032:01, 0x2037, 0x203C and the parameters written by `FB_Parameter`) so `FB_ATV320` reaches `cfgReady` and runs a motor unmodified
- [x] **ECAT-06**: An EL6001 model exposes the 22-byte serial PDO with a pluggable byte-stream peer so the Baader `md`/`mt1` protocol can be scripted against `FB_BaaderSerial`
- [x] **ECAT-07**: Every slave publishes `WcState`, `InfoData.State` and `InfoData.AdsAddr`; every master publishes `DevState`, `SlaveCount`, `Frm0State`, `Frm0WcState` and `InfoData.AmsNetId` with Beckhoff bit semantics, linkable through `TcLinkTo`
- [x] **ECAT-08**: Tc2_EtherCAT FBs (`FB_EcGetSlaveState`, `FB_EcGetAllSlaveStates`, `FB_EcSetSlaveState`, `FB_EcGetMasterState`, `FB_EcGetAllSlaveCrcErrors`, `FB_EcGetSlaveCrcErrorEx`, `FB_EcCoESDoRead/Write`, `FB_EcPhysicalWriteCmd`) have behavioural mocks backed by the simulator with asynchronous busy/done timing, so `FB_EcDeviceDiag` and the ATV320 configurator run unmodified
- [x] **ECAT-09**: A scenario file (TOML) and ST test built-ins can set inputs by variable path or link path, set analog values, trip an EL9222 channel, remove a slave (not present / link error), raise a drive fault with an LFT code and ramp a value over time, all deterministic against the scan clock
- [x] **ECAT-10**: `stc sim --project <plcproj|tsproj> --io <Device*.xml> --scenario <toml> --cycles N` runs the imported project against the simulator and reports outputs and diagnostics in text and JSON

### OPC UA server (OPCUA)

- [x] **OPCUA-01**: `stc serve --project ... --opcua :4840` starts an OPC UA server (awcullen/opcua) with SecurityPolicy None + Anonymous by default and optional Basic256Sha256 with self-signed certificates
- [x] **OPCUA-02**: The PLC namespace `urn:BeckhoffAutomation:Ua:PLC1` is registered at index 4; nodes are addressable as `ns=4;s=<GVL|PROGRAM>.<path>[i].<member>` with declared case; the standard Server object including `i=2259` ServerStatus.State is served
- [x] **OPCUA-03**: Exposure follows TF6100 rules: a symbol is published if it or any ancestor instance or type-level member has `OPC.UA.DA := '1'`; `'1'` inherits to children; `'0'` prunes a subtree; `'2'` publishes a struct without member nodes; type-level attributes inside FB/STRUCT declarations apply to every instance; intermediate FB/array object nodes are created when a descendant is exposed
- [x] **OPCUA-04**: `OPC.UA.DA.Access` 1/2/3 maps to AccessLevel (missing = read/write) and `OPC.UA.DA.Description` to the Description attribute
- [x] **OPCUA-05**: Structs with `OPC.UA.DA.StructuredType` (on the variable or on the TYPE/FB header) are readable as ExtensionObjects with a served DataTypeDefinition, while members remain individually addressable; enums are Int32 with EnumStrings/EnumValues; arrays are single nodes with ValueRank/ArrayDimensions
- [x] **OPCUA-06**: Data types map per PLCopen OPC 30000 (BOOL Boolean, INT Int16, UINT/WORD UInt16, DINT Int32, UDINT/DWORD UInt32, REAL Float, LREAL Double, STRING String, TIME Int64 ms, DT DateTime, TOD UInt32, BYTE Byte)
- [x] **OPCUA-07**: Writes go through the symbol tree with coercion so the `p_cmd_*` set-TRUE / FB-clears handshake works while the scan runs
- [x] **OPCUA-08**: Subscriptions and monitored items deliver data changes sampled from the running scan
- [x] **OPCUA-09**: The emulated address space for ST301 is diffed in CI against a stored browse fixture of the real TF6100 server (node ids, data types, access levels, struct definitions)
- [x] **OPCUA-10**: The sildarvinnsla Flutter HMI (tfc-hmi / open62541_dart) connects to `stc serve` running ST301 and shows live sensor, conveyor and drive HMI structs

### Test and agent ergonomics (DEVX)

- [x] **DEVX-01**: ST test built-ins `SET(path, value)`, `GET(path)`, `SIM_SET_LINK(linkpath, value)`, `SIM_TRIP(slave, channel)`, `SIM_SLAVE_STATE(slave, state)` and `RUN_CYCLES(n)` are available in `*_test.st` when a project and I/O config are loaded
- [x] **DEVX-02**: MCP tools `stc_sim_step`, `stc_sim_read`, `stc_sim_write` and `stc_opcua_browse` expose the running simulation to agents
- [x] **DEVX-03**: `docs/` gains a TwinCAT import, EtherCAT simulation and OPC UA guide, and the stale claims in `TESTING_GUIDE.md`, `ST_LANGUAGE_SUPPORT.md` and `stdlib/vendor/beckhoff/ethercat_io.md` are corrected

## v2 Requirements

Deferred. Tracked but not in the current roadmap.

### ADS server (ADS)

- **ADS-01**: ADS server on AMS port 851 with symbol upload, read/write by name and sum commands so pyads collectors (`Baader/collect/ads.py`, adslog) work against `stc serve`
- **ADS-02**: ADSREAD/ADSWRITE from ST routed to the simulated master and slaves

### Extended simulation (ECATX)

- **ECATX-01**: EL40xx analog output, EL70xx stepper and EL72xx servo models
- **ECATX-02**: Modbus TCP server/client emulation for Tc2_ModbusSrv
- **ECATX-03**: TwinSAFE logic (EL6900 / FSoE) simulation beyond standard diagnostics

### Extended OPC UA (OPCUAX)

- **OPCUAX-01**: `TcRpcEnable` method calls, `OPC.UA.DA.Property`, `AnalogItemType`, `Alias`, `Status`
- **OPCUAX-02**: `LegacyArrayHandling` per-element nodes and `ImportBigEnumsNumeric`
- **OPCUAX-03**: Username/password and certificate trust-list handling mirroring TF6100 TOFU behaviour

## Out of Scope

| Feature | Reason |
|---------|--------|
| Frame-level EtherCAT emulation (ESC registers, mailbox protocol, DC clocks) | Process-image/PDO level gives everything ST code can observe; frame level is TE1111 / acontis territory with no return for host testing |
| Deterministic real-time guarantees in free-running mode | stc is a development tool; free-running mode exists for HMI development only |
| Editing or generating `TcUaDaConfig.xml` / `TcUaServerConfig.xml` | stc emulates the server's behaviour, it does not configure the real one |
| Changes to the Flutter HMI or tfc-hmi | The HMI is the acceptance oracle and must work unmodified |
| Parsing `.library` / `.compiled-library` binaries | Still proprietary; stubs and plcproj import cover the need |
| Generic ESI-driven device modelling for arbitrary slaves | Unbounded; models are hand-written per (VendorId, ProductCode) with a generic byte-passthrough fallback |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| DIAL-01 | Phase 19 | Complete |
| DIAL-02 | Phase 19 | Complete |
| DIAL-03 | Phase 19 | Complete |
| DIAL-04 | Phase 20 | Complete |
| DIAL-05 | Phase 19 | Complete |
| DIAL-06 | Phase 20 | Complete |
| DIAL-07 | Phase 20 | Complete |
| DIAL-08 | Phase 19 | Complete |
| DIAL-09 | Phase 20 | Complete |
| DIAL-10 | Phase 20 | Complete |
| IMPT-01 | Phase 21 | Complete |
| IMPT-02 | Phase 21 | Complete |
| IMPT-03 | Phase 21 | Complete |
| IMPT-04 | Phase 21 | Complete |
| IMPT-05 | Phase 21 | Complete |
| RUNT-01 | Phase 22 | Complete |
| RUNT-02 | Phase 22 | Complete |
| RUNT-03 | Phase 23 | Complete |
| RUNT-04 | Phase 23 | Complete |
| RUNT-05 | Phase 22 | Complete |
| RUNT-06 | Phase 22 | Complete |
| RUNT-07 | Phase 23 | Complete |
| RUNT-08 | Phase 20 | Complete |
| RUNT-09 | Phase 23 | Complete |
| ECAT-01 | Phase 24 | Complete |
| ECAT-02 | Phase 24 | Complete |
| ECAT-03 | Phase 24 | Complete |
| ECAT-04 | Phase 25 | Complete |
| ECAT-05 | Phase 26 | Complete |
| ECAT-06 | Phase 25 | Complete |
| ECAT-07 | Phase 24 | Complete |
| ECAT-08 | Phase 26 | Complete |
| ECAT-09 | Phase 27 | Complete |
| ECAT-10 | Phase 27 | Complete |
| OPCUA-01 | Phase 28 | Complete |
| OPCUA-02 | Phase 28 | Complete |
| OPCUA-03 | Phase 28 | Complete |
| OPCUA-04 | Phase 28 | Complete |
| OPCUA-05 | Phase 28 | Complete |
| OPCUA-06 | Phase 28 | Complete |
| OPCUA-07 | Phase 29 | Complete |
| OPCUA-08 | Phase 29 | Complete |
| OPCUA-09 | Phase 29 | Complete |
| OPCUA-10 | Phase 29 | Complete |
| DEVX-01 | Phase 27 | Complete |
| DEVX-02 | Phase 29 | Complete |
| DEVX-03 | Phase 29 | Complete |

**Coverage:**
- v1.2 requirements: 47 total
- Mapped to phases: 47
- Unmapped: 0

---
*Requirements defined: 2026-10-05*
*Last updated: 2026-10-05 after v1.2 roadmap creation*
