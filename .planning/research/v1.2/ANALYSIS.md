# What stc needs to simulate sildarvinnsla's EtherCAT hardware and serve Beckhoff-style OPC UA

Date: 2026-10-05. Analysis of `/Users/jonb/Projects/stc` (HEAD 921dd3c) against `/Users/jonb/Projects/sildarvinnsla` (HEAD 549e3a9).

## 1. The target: what sildarvinnsla actually is

- Three line PLCs (ST101, ST201, ST301; MAIN 1600 to 2300 lines each, 1 ms task), a Baader data-collection PLC (`gagnasofnun`, 20 ms task), and a shared library `SVNCoreComponents` (71 files). ST301 is the biggest and most recently regenerated (I/O GVLs regenerated 2026-09-14).
- Libraries referenced: SVNCoreComponents, Tc2_EtherCAT, Tc2_ModbusSrv, Tc2_Standard, Tc2_System, Tc3_Module (Baader adds Tc3_IPCDiag). No Tc2_MC2, no Tc2_Utilities, no OPC UA library (TF6100 is server-side config only).
- Each line has 4 EtherCAT masters. Hardware per line (ST101 / ST201 / ST301 / Baader):

| Device | ST101 | ST201 | ST301 | Baader |
|---|---|---|---|---|
| ATV320 EtherCAT drive (Schneider, vendor 0x0800005a, product 0x389) | 43 | 32 | 41 | – |
| EL9222-5500 overcurrent protection | 16 | 30 | 36 | 1 |
| EL1008 8ch DI | 9 | 13 | 24 | 10 |
| EL2008 8ch DO | 8 | 11 | 22 | 10 |
| EP2338-0002 / -1002 IP67 8ch DIO | 12 | 18 | 14 | – |
| EK1100 coupler | 6 | 11 | 12 | 10 |
| PS2001-2410 PSU | 5 | 9 | 10 | – |
| EP1918 / EL2912 / EL1904 TwinSAFE | 5 | 8 | 12 | – |
| Festo CTEU-EtherCAT valve terminal | 1 | 1 | 3 | – |
| EL6001 RS-232 | – | – | – | 18 |
| EL3054 4ch AI 4-20 mA | – | – | – | 10 |
| EK1110, EK1200, EL6070, EL9011, CU2508, CX5120 | misc | misc | misc | misc |

### 1.1 How I/O is bound (this is the key design input)

Nothing uses explicit `%IX0.0` addresses. All 488 I/O variables are `AT %I*` / `AT %Q*` (441 inputs, 47 outputs) and are bound by name through `{attribute 'TcLinkTo' := '...'}` pragmas (675 of them). Terminals are modelled as ST structs with `AT %I*` members:

```
TYPE ST_EL1008 : STRUCT
    {attribute 'OPC.UA.DA.Access' := '1'}
    I1 AT %I* : BOOL;  ... I8
END_STRUCT END_TYPE

// generated GVL ECT (ST301/ST301/GVLs/ECT.TcGVL)
{attribute 'OPC.UA.DA.StructuredType' := '1'}
{attribute 'OPC.UA.DA' := '1'}
{attribute 'TcLinkTo' := '.I1 := TIID^Device 1 (EtherCAT)^ST301.A1.00 (EK1200)^ST301.A1.03 (EL1008)^Channel 1^Input; .I2 := ...'}
ST301_A1_03 : ST_EL1008;
```

Drives are a single FB instance per slave with members linked individually (`.q_uCMD := ...^Outputs^CMD; .i_uETA := ...^Inputs^ETA; .amsaddr := ...^InfoData^AdsAddr`). TcLinkTo targets use the TwinCAT I/O tree path: `TIID ^ <master> ^ [<coupler>] ^ <slave> ^ [Module N (...)] ^ <PDO name> ^ <entry path>`; nested entries like `Status__Enabled` become `Status^Enabled`.

The `ECT` and `ECT_Diag` GVLs are **generated** by `IO List from ethercat/generate_gvl.py` (949 lines, test-gated via Makefile) from TwinCAT's "Export Configuration File" output (`EtherCATConfig` XML v1.3, `<Config><Master><Info><Name>` + `<Slave><Info>{Name,VendorId,ProductCode,PhysAddr,Physics}</Info><PreviousPort>..</PreviousPort><ProcessData><TxPdo>/<RxPdo><Index/><Entry><Name/><Index/><BitLen/><DataType/>`). The generator already has the data model stc needs: `Slave{name, model, vendor, product, phys, prev_phys, prev_port, pdos}`, `Pdo{name, direction, entries}`, `Entry{name, index, bitlen, datatype}`, topology via port physics (`YK` opens E-bus, `KK` terminal, `KY` closes, `YY` plain), a `MODULE_MAP` for modular slaves, and per-device handlers (`handler_default`, `handler_atv320`, `struct_selected` for EL1008/EL2008/EL9222/EP2338/PS2001).

The per-box `.xti` files carry the same PDO list plus TwinCAT-generated struct types (`EL1008_I_05C0EB9B` with `BIT` members at bit offsets) and the `WcState` / `InfoData` pseudo-inputs. The ATV320 PDO set: Inputs 0x1A00 = ETA(0x6041 UINT), RFR(INT), LCR, DI, LFT, HMIS (all UINT); Outputs 0x1600 = CMD(0x6040 UINT), LFR(INT), OL1R, ACC, DEC (UINT). EL9222-5500: per channel Status bits (Enabled, Tripped, Hardware Protection, Current Level Warning, Cool Down Lock, Diag, TxPDO State, Input cycle counter BIT2) and Control bits (Reset, Switch).

### 1.2 What the PLC code does beyond plain I/O (things a simulator must answer)

- `FB_ATV320` (1205 lines): CiA402 state machine over CMD/ETA, reads the drive's `InfoData^AdsAddr` to run **CoE SDO reads/writes over ADS** (`FB_EcCoESDoRead/Write` via `FB_Parameter` for ~15 motor parameters), polls slave state with `FB_EcGetSlaveState`, and promotes PreOp to OP with `FB_EcSetSlaveState` (a "configurator" FSM, requires slave Final State = PreOp). Gates "drive ready" on `ec.deviceState = 8`.
- `FB_EcDeviceDiag`: round-robin `FB_EcGetAllSlaveStates`, `FB_EcGetAllSlaveCrcErrors`, `FB_EcGetSlaveCrcErrorEx`, `FB_EcGetMasterState`, `FB_EcPhysicalWriteCmd` (clears ESC CRC registers 0x0300..0B). Fills `ECT_Diag.Device_N_Diag : ARRAY[1..MAX_EC_SLAVES] OF ST_EcSlaveDiag` for the HMI network monitor. Reads master `InfoData^AmsNetId` via `Device_N_AmsNetId AT %I* : T_AmsNetIdArr`.
- Modbus TCP server/client (`Tc2_ModbusSrv`, 192 references), ADSREAD/ADSWRITE, `F_CreateAmsNetId`.
- Baader: `Tc2_SerialCom`-style framing over EL6001 (22-byte mode), `MEMCPY`, pointer arithmetic with indexing, `REFERENCE TO` with `REF=`, `THIS^`.

### 1.3 How OPC UA is used (from 224 tracked files)

| Attribute | Count | Values |
|---|---|---|
| `OPC.UA.DA` | 474 | `'1'` 472, `'0'` 2 |
| `OPC.UA.DA.Access` | 398 | `'1'` 391 (read-only), `'3'` 7 (r/w) |
| `OPC.UA.DA.StructuredType` | 328 | always `'1'` |
| `OPC.UA.DA.Description` | 235 | text with `''` escapes |

Not used anywhere: `TcRpcEnable`, `OPC.UA.DA.Method`, `.Property`, `.Alias`, `.Unit`, AnalogItemType family. Data-only server.

Attachment sites: GVL vars 329 (incl. 17 `VAR_GLOBAL PERSISTENT RETAIN`), STRUCT members 263, FB `VAR` 120 (incl. `VAR PERSISTENT RETAIN`), FB `VAR_IN_OUT` 3, `VAR_OUTPUT` 2, `VAR_INPUT` 1, before `FUNCTION_BLOCK` header 12, before `TYPE` header 4, on enum values 2. **PROGRAM MAIN exposes nothing.** Real browse paths look like `GVL.fbOrArray[i].HMI.p_stat_X`: intermediate FB instances carry only `StructuredType` (or nothing); exposure is decided inside the FB type (`FB_Sensor` has `VAR PERSISTENT RETAIN HMI : ST_Sensor_HMI` marked DA=1, and 40 unmarked instances in `sensors.TcGVL` are all exposed).

Conventions: `p_stat_*` (Access 1), `p_cmd_*` (writable, HMI sets TRUE / FB clears), `p_cfg_*` (writable, often PERSISTENT). Types exposed: BOOL 289, UINT 40, REAL 36, UDINT 18, TIME 15, BYTE, WORD, INT, STRING(n), DT; enums `hmis_e`, `lft_e`, `states`, `ATV320_RunMode`, `E_EcSlaveState`, `ET_WagonStationType`; structs incl. ones with FB instances inside (`ST_StrappingLine_HMI.p_stat_LastSensor : FB_Sensor`); arrays of structs and of FB_ATV320 with struct-array initialisers.

HMI client: Flutter via `open62541_dart` (FFI). NodeIds are `ns=4;s=<GVL>.<path>[i].<member>` with no `PLC1.` prefix, case as declared; also `ns=0;i=2259` (ServerStatus.State). It reads whole StructuredType structs as one ExtensionObject and decodes enum value names (`rdy(2)`), so DataTypeDefinition and EnumStrings must be served. Baader collector and adslog use pyads on AMS port 851 (ADS, not OPC UA).

## 2. What stc has today (HEAD 921dd3c, 20.9k LOC, 26 packages, all tests green)

Relevant pieces and their limits, verified by probes:

- **Parser** drops every `{attribute ...}` pragma (`parser.go:108 skipPragmas`); `ast.PragmaNode` exists but is never constructed. `fmt` and `emit` strip pragmas.
- **No top-level `VAR_GLOBAL`** (a TcGVL body): "unexpected KwVarGlobal in declaration context". No CONFIGURATION/RESOURCE/TASK. Only one PROGRAM is ever executed.
- **iomap**: flat `%I/%Q/%M` byte arrays, `ParseAddress` handles `%IX0.0`, `%IW4`, `%I*`; but wildcards are **skipped** at bind time (`scan.go`, `!addr.IsWildcard`), so `AT %I*` vars are never connected. Typing is by address size not declared type (INT not sign-extended, REAL writes 0). No LWORD.
- **sim**: batch only (`Run()` to completion), waveforms always REAL, cannot run PROGRAMs that use user FBs (`interp.New()` without FBDecls), no plant attach from CLI, no network interface of any kind.
- **interp**: tree-walker, deterministic clock, Value tagged union; no dotted-path Get/Set, no symbol tree, integers don't wrap, array initialisers ignored. METHOD/PROPERTY/INTERFACE/EXTENDS/ADR/`^` work; qualified enums, `REF=`, `THIS^`, `SUPER^` do not.
- **checker**: treats unknown type names as empty FBs, so TON/CTU calls fail `stc check` (B1), `a := a + 1` on INT fails (B2).
- **vendor extract**: reads only `.plcproj` + `.TcPOU` declarations; skips `.TcGVL`, `.TcDUT`, methods, actions; output lacks `END_FUNCTION_BLOCK`.
- **Stubs**: Tc2_MC2, Tc2_System (ADSREAD/ADSWRITE), FB_FormatString, EventLogger. **No Tc2_EtherCAT, Tc2_ModbusSrv, Tc3_Module stubs.**
- Planning docs explicitly put this out of scope (`REQUIREMENTS.md:88-91`, `research/PITFALLS.md` Pitfall 9 "Do NOT attempt to parse or simulate EtherCAT configuration", `FEATURES.md:54` "Real ADS/Modbus communication: stc is a development tool, not a runtime"). This work is a new milestone that reverses those decisions.

### 2.1 Measured parse result on sildarvinnsla (TcPOU/TcGVL/TcDUT flattened to .st)

| Project | Declarations | Diagnostics |
|---|---|---|
| SVNCoreComponents | 100 | 1430 |
| ST101 | 866 | 2007 |
| ST201 | 1049 | 2158 |
| ST301 | 1162 | 2711 |
| Baader | 459 | 697 |

Root causes, each confirmed with a minimal file:

1. Bare `VAR_GLOBAL ... END_VAR` file (every GVL) — not parsed. Cascades into "unexpected Ident" for every following line.
2. `AT %I*` inside STRUCT members (`ST_EL1008` etc.) — rejected.
3. Bit access `word.3` (`ECT.X.q_wDigitalInputs.0`, `Modbus.arr[0].0`) — rejected. Thousands of uses.
4. Empty formal arguments `t(IN := b, PT := , Q => , ET => );` (TwinCAT auto-complete output) — rejected. ~1200 uses.
5. Named-argument **function** calls in expressions `n := F_X(a := 1, b := 2);` — rejected (positional works).
6. Qualified enum CASE labels `lft_e.eef1:` and label lists `E.a, E.b:` — rejected.
7. `ACTION ... END_ACTION` — not supported at all (54 actions; MAIN is a chain of action calls).
8. Enum with base type `(a := 0, b := 1) UINT;` — rejected.
9. `REF=` — rejected. `THIS^` — rejected.
10. `{attribute "qualified_only"}` with double quotes — unverified. Attributes with a blank line before the declaration — unverified.
11. Struct-array initialisers `:= [(a := 1, b := 'x'), ...]` with `ARRAY[1..GVL.CONST]` bounds — parse OK but values are ignored at runtime (B6), and constant-expression bounds are unverified.

## 3. What has to be built

Ordered so each layer is useful on its own.

### Layer 0: dialect parity (blocking everything else)

1. Keep pragmas in the AST: `Attributes []Attribute{Name, Value, Span}` on VarDecl, StructMember, EnumValue, TypeDecl, POU decl. Parser attaches preceding pragmas to the next declaration (skipping blank lines). `fmt`/`emit` round-trip them. Support `'` and `"` quoting and `''` escapes.
2. Top-level `VAR_GLOBAL [PERSISTENT] [RETAIN] [CONSTANT]` → `ast.GVLDecl{Name}` where the name comes from the file (TcGVL `Name=`) or a `--gvl-name`. `qualified_only` enforces `GVL.x` access.
3. `AT %I*` / `%Q*` on STRUCT members and FB VAR/VAR_INPUT/VAR_OUTPUT (downgrade SEMA031 for wildcards: TwinCAT allows them).
4. Bit access `x.N` on BYTE/WORD/DWORD/LWORD (read and write), including after array index and member chains.
5. Empty formal parameters in FB calls (`:= ,` and `=> ,`).
6. Named arguments in FUNCTION calls used as expressions.
7. Qualified enum values everywhere (`E.v` in expressions, CASE labels, label lists, initialisers); enum base types `(...) UINT`; `{attribute 'strict'}`, `'to_string'`.
8. ACTIONs: parse `ACTION name ... END_ACTION` after `END_PROGRAM`/`END_FUNCTION_BLOCK` (CODESYS text form) and from TcPOU `<Action>` XML; call as `name()` inside the POU.
9. `REF=`, `THIS^`, `SUPER^`, `VAR PERSISTENT RETAIN`, `VAR_IN_OUT` with attributes.
10. Runtime fixes the sim will trip on immediately: integer wrap (B5), array/struct initialisers (B6), AT typing by declared type (B9), checker knowing stdlib FBs (B1) and literal typing (B2).
11. `stc vendor import <x.tsproj | x.plcproj>`: read `.plcproj` Compile items for `.TcPOU` (declaration + implementation + methods + actions + properties), `.TcGVL`, `.TcDUT`, library placeholder refs; produce a project model stc can check/run as a whole. Library resolution order: project POUs, then SVNCoreComponents (sibling plcproj), then stubs.
12. Stubs: Tc2_EtherCAT (`FB_EcGetSlaveState`, `FB_EcGetAllSlaveStates`, `FB_EcSetSlaveState`, `FB_EcGetMasterState`, `FB_EcGetAllSlaveCrcErrors`, `FB_EcGetSlaveCrcErrorEx`, `FB_EcCoESDoRead/Write`, `FB_EcPhysicalWriteCmd`, `ST_EcSlaveState`, `E_EcSlaveState`), Tc2_System (`AMSADDR`, `T_AmsNetId`, `T_AmsNetIdArr`, `F_CreateAmsNetId`, `MEMCPY`), Tc2_ModbusSrv, Tc3_Module, Tc2_SerialCom.

Acceptance: `stc check` on ST301 + SVNCoreComponents reports 0 parse errors and only genuine semantic warnings.

### Layer 1: a real runtime model (symbol tree + tasks)

- A **symbol tree** built after checking: `GVL.var`, `MAIN.var`, `MAIN.fb.member`, `arr[i].member`, with type metadata (IEC type, struct layout, enum strings, attributes). Dotted-path `Get/Set(path)` on the live interpreter env. This is what OPC UA, ADS, tests and the CLI all use.
- Multi-POU execution: all GVLs instantiated once; PROGRAM(s) run per task with configured cycle time (from `.tsproj` `CycleTime`, 1 ms for lines). Deterministic clock, `Tick()` step API, plus a free-running mode with wall-clock pacing for serving the HMI.
- `PERSISTENT RETAIN` handling: load/save to a JSON file so `p_cfg_*` survive restarts like on the PLC.

### Layer 2: EtherCAT process-image simulator (`pkg/ecat`)

Simulate at the **process-image level with a PDO/terminal model**, not EtherCAT frames (TE1111 / acontis EC-Simulator territory, no return for ST testing). Concretely:

1. **Topology loader**: parse TwinCAT `EtherCATConfig` exports (the files `generate_gvl.py` already consumes) and, optionally, `.xti`/`.tsproj` for box comments and TwinCAT-only pseudo-inputs. Port the generator's `Slave/Pdo/Entry` model and `link_path()` to Go (`pkg/ecat/topology.go`), including E-bus nesting by port physics and `MODULE_MAP`.
2. **Link resolver**: parse `TcLinkTo` pragma strings (single and `.member := path; ...` multi-form), match them to the topology by path, allocate each PDO entry a (byte, bit) slot in the per-master input/output image, and bind the `AT %I*` variable (or struct member / FB member) to it. Unmatched links are diagnostics (`ECAT001 link target not found`), which doubles as a **static validator for the generated GVLs**, a value on its own.
3. **Device models** (Go, implementing `Slave interface { Init(pdos); Step(dt, outputs []byte) inputs []byte; State() }`), selected by (VendorId, ProductCode):
   - Digital: EL1008/EL1018, EL2008, EP2338 (8 DI + 8 DO, configurable), Festo CTEU outputs. Inputs come from scenario/test injection; outputs are observable.
   - EL3054/EL3064: Status WORD (Underrange bit0, Overrange bit1, Limit1 bits2-3, Limit2 bits4-5, Error bit6, TxPDO State bit14, TxPDO Toggle bit15) + INT value with 4-20 mA / 0-10 V scaling.
   - EL9222-5500: per-channel Enabled/Tripped/Hardware Protection/Current Level Warning/Cool Down Lock/Diag with Reset/Switch controls, trip injection.
   - PS2001-2410: Warning/Error/DC OK/Output voltage/current/Input undervoltage.
   - EL2912 / EP1918 / EL1904: standard field-voltage diag only (safety PDOs owned by TwinSAFE, skipped like the generator does).
   - **ATV320**: CiA402 state machine (Not ready → Switch on disabled → Ready to switch on → Switched on → Operation enabled; Fault; Quick stop) driven by CMD, reporting ETA; LFR → RFR ramp with ACC/DEC; LCR current model; HMIS (`rdy`, `run`, `flt`...) and LFT fault codes; DI/OL1R logic I/O; and a **CoE object dictionary** (0x6040/0x6041, 0x2002:xx, 0x2016, 0x2029, 0x2037, 0x203C, 0x2032:01 EEPROM save and the ~15 motor parameters `FB_Parameter` writes) because `FB_ATV320` will not reach `cfgReady` without SDO round-trips succeeding.
   - EK1100/EK1110/EK1200/EL6070/EL9011: no PDOs; present in slave list with InfoData only.
   - EL6001: 22-byte serial PDO with a pluggable byte stream so the Baader protocol (`md`, `mt1`, `'$R$N+'` terminator) can be emulated against `Baader/protocol.md`.
4. **Per-slave pseudo-inputs**: `WcState` (0 ok), `InfoData.State` (0x8 OP, 0x10 error, 0x100 not present, 0x200 link error), `InfoData.AdsAddr` (master NetId + port 1001+i). **Master inputs**: `DevState`, `SlaveCount`, `Frm0State`, `Frm0WcState`, `InfoData.AmsNetId`. These are already linked in ECT_Diag.
5. **EtherCAT master ADS services**: the Tc2_EtherCAT FBs are ADS calls to the master's AMS port (0xFFFF). Implement them as behavioural mocks backed by the simulator: `FB_EcGetAllSlaveStates` returns each model's state, `FB_EcSetSlaveState` transitions it (with PreOp → OP timing), `FB_EcCoESDoRead/Write` hit the slave's object dictionary, CRC FBs return counters, `FB_EcPhysicalWriteCmd` clears them. This is what makes `FB_ATV320`'s configurator and `FB_EcDeviceDiag` run unmodified.
6. **Fault injection and scenarios**: scripted (TOML/JSON or ST test built-ins): set input bit, set analog value, trip EL9222 channel, pull a slave (`State := 0x100`, `WcState := 1`), drive fault with LFT code, ramp a sensor. Deterministic with the scan clock.
7. CLI: `stc sim --project ST301 --io-config "IO List from ethercat/ST301/Device *.xml" --scenario x.toml --cycles N` and `stc ecat validate` (links vs topology).

### Layer 3: TF6100-compatible OPC UA server (`pkg/opcua`)

Library: **awcullen/opcua** (MIT, pure Go, server with Browse/Read/Write/Subscriptions/Method Call, DataType nodes, NodeSet import). gopcua's server lacks Call and struct DataTypeDefinition; open62541 via cgo breaks the single-binary goal. Verify early that awcullen can expose `DataTypeDefinition` (StructureDefinition) for custom structs, since the HMI decodes ExtensionObjects by field name. Fallback: emit a NodeSet2 XML with the struct definitions and import it.

Address space, matching what the HMI and Beckhoff docs expect:

- Endpoint `opc.tcp://host:4840`, namespace URI `urn:BeckhoffAutomation:Ua:PLC1` at index 4 (register three filler namespaces so the index lands on 4). SecurityPolicy None + Anonymous enabled by default (the real server disables None after TOFU; dev tool can relax this, make it a flag), plus optional Basic256Sha256 with self-signed certs since the compose file mounts `opcua_certs`.
- Hierarchy: `Objects → DeviceSet → PLC1 (DI DeviceType: DeviceManual, DeviceRevision, Model, SerialNumber, DeviceState) → <GVL>/<PROGRAM> object nodes → variables`. String NodeIds `ns=4;s=GVL.path[i].member` (no PLC prefix), case preserved. Serve standard `Server` object incl. `i=2259` ServerStatus.State.
- **Exposure rules** (from TF6100 docs + observed usage): a node is published if it has `OPC.UA.DA='1'` or any ancestor instance or type-level member has it; `'1'` inherits to all children; `'0'` prunes a subtree; `'2'` publishes a struct without member nodes. Type-level attributes (inside FB/STRUCT declarations) apply to every instance. Intermediate FB/array nodes are created when a descendant is exposed. `StructuredType` on the var or on the TYPE/FB header makes the parent readable as an ExtensionObject. `OPC.UA.DA.Access` 1/2/3 → AccessLevel; missing → read/write. `Description` → Description attribute.
- **Type mapping** (PLCopen OPC 30000 table 27): BOOL→Boolean, SINT→SByte, USINT/BYTE→Byte, INT→Int16, UINT/WORD→UInt16, DINT→Int32, UDINT/DWORD→UInt32, LINT→Int64, ULINT/LWORD→UInt64, REAL→Float, LREAL→Double, STRING→String, TIME→Int64 milliseconds (BadOutOfRange above 4294967295 ms), LTIME→Int64 ns, DT/DATE→DateTime, TOD→UInt32. Enums → Int32 with EnumStrings/EnumValues (HMI shows `rdy(2)`). Arrays → single node with ValueRank/ArrayDimensions (no LegacyArrayHandling). Structs → Object node with child Variables plus ExtensionObject value when StructuredType.
- Writes go through the symbol tree with type coercion; `p_cmd_*` handshake pattern (HMI sets TRUE, FB clears) then works naturally as the scan runs.
- Subscriptions/MonitoredItems with sampling tied to the free-running scan; keep deterministic mode for tests by snapshotting per tick.
- CLI: `stc serve --project ST301 --opcua :4840 [--ecat ...] [--persist state.json]` runs scan + simulator + server. Later the same symbol tree can back an **ADS server** (port 851, symbol upload, read/write by name, sum commands) so `ads.py`/pyads and adslog also work; that is a separate milestone.

### Layer 4: test and agent ergonomics

- ST test built-ins over the new model: `SIM_SET_INPUT('TIID^Device 1^...^Channel 1^Input', TRUE)` or by variable path `SET('ECT.ST301_A1_03.I1', TRUE)`, `SIM_TRIP('ST301.A1.02', 1)`, `RUN_CYCLES(n)`, `ASSERT_EQ(GET('CVS03.CN02_MA01_fb.HMI.p_stat_State'), hmis_e.run)`.
- MCP tools: `stc_sim_step`, `stc_sim_read`, `stc_sim_write`, `stc_opcua_browse` so an agent can drive the plant model and observe the HMI-visible tree.
- Goldens: browse the real ST301 TF6100 once (from the plant network) and store the node tree + DataTypeDefinitions as a fixture; diff the emulated tree against it in CI.

## 4. Risks and open questions

1. Scope is large relative to the existing codebase (20.9k LOC). Layer 0 alone touches lexer, parser, AST, checker, interp, fmt, emit. Do it as a GSD milestone with phases matching the layers above, and keep the parity work test-driven against the flattened sildarvinnsla sources (the five `.st` files in the scratchpad show the current failures).
2. ATV320 fidelity: without CoE emulation, `FB_ATV320` never leaves `cfgWriteParams`. Start with a parameter store that accepts any SDO and returns stored values; model the CiA402 words faithfully; leave torque/current physics simple.
3. awcullen DataTypeDefinition support for structs is unverified; test it in the first OPC UA spike before committing to the library.
4. Security: the real server disables SecurityPolicy None; the Flutter HMI may be configured for SignAndEncrypt. Support both.
5. The planning docs (`PITFALLS.md` Pitfall 9) were right that *generic* EtherCAT simulation is a rabbit hole. The mitigation is to reuse the project's own device vocabulary (the ~18 device types above, the generator's data model) and to make the loader data-driven from TwinCAT exports rather than modelling ESI/ESC behaviour.
6. ADS for pyads collectors is out of this scope but the symbol tree should be designed so an ADS server is a thin adapter later.

## 5. Suggested order for tomorrow

1. `/gsd:new-milestone` "v1.2 TwinCAT project import, EtherCAT sim, OPC UA" and paste this document into `.planning/research/`.
2. Phase A (parity): pragmas in AST → GVL → AT in structs → bit access → empty args → named fn args → qualified enums/CASE → ACTION → `vendor import` of tsproj/plcproj → stubs. Gate: ST301 + SVNCoreComponents check clean.
3. Phase B (runtime): symbol tree + dotted paths, multi-POU/task, PERSISTENT, integer wrap, initialisers.
4. Phase C (ecat): topology loader from `EtherCATConfig` XML, TcLinkTo resolver + `stc ecat validate`, digital/analog/EL9222/PSU models, InfoData/WcState, Tc2_EtherCAT mocks, ATV320 with CoE.
5. Phase D (opcua): awcullen spike for struct DataTypeDefinition, address-space builder from attributes, `stc serve`, connect the Flutter HMI to it.

## 6. Local resources

- Beckhoff docs mirror: `/Users/jonb/Projects/beckhoff-docs/` (see its README for contents).
- Flattened sildarvinnsla sources used for parser probing and the minimal failing cases: `/Users/jonb/Projects/beckhoff-docs/stc-probes/` (`st301.st` etc. are whole projects flattened; the short files are one failing construct each).
- Generator to port: `/Users/jonb/Projects/sildarvinnsla/IO List from ethercat/generate_gvl.py`.
- TwinCAT exports: `/Users/jonb/Projects/sildarvinnsla/IO List from ethercat/ST301/Device 1..4.xml`; per-box `.xti` under `ST301/_Config/IO/`.
