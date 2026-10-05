# Reference: TF6100 OPC UA semantics, Go OPC UA servers, EtherCAT simulation, terminal process data

Web research compiled 2026-10-05 for milestone v1.2. Confidence marks: HIGH / MEDIUM / LOW. An offline mirror of the Beckhoff InfoSys books and PDFs cited here lives at `/Users/jonb/Projects/beckhoff-docs/` (grep `pdf-text/` first).

## A. Beckhoff TwinCAT 3 OPC UA Server (TF6100)

### A1. `OPC.UA.DA` symbol enabling

| Value | Meaning | Conf |
|---|---|---|
| `'1'` | Enables the symbol for OPC UA | HIGH |
| `'0'` | Explicitly blocks the symbol; stops inheritance below that point | HIGH |
| `'2'` | Enables the symbol, but a STRUCT's members are NOT loaded as separate nodes (struct readable only as a whole) | HIGH |

Inheritance (HIGH, InfoSys "Enabling symbols", tf6100_tc3_opcua_server/15620470667):
- "The pragma for enabling a symbol is automatically inherited to all child symbols." Marking a struct instance exposes every member; marking an FB instance exposes all symbols it contains. Block a sub-tree with `'0'` on the member.
- Pragma on an instance exposes only that instance and children. Pragma on a variable inside the FB type definition exposes that variable in every instance.
- Works in PROGRAM VAR blocks and GVLs alike. Node path is the full symbol path including the program or GVL name.
- Filter mode: Data Access device type "TwinCAT 3 PLC (TMC) – Filtered" shows only marked symbols; TMC symbol export must be enabled in the PLC project. An unfiltered device type exists (MEDIUM).

### A2. Other `OPC.UA.DA.*` attributes

| Attribute | Values | Effect | Conf |
|---|---|---|---|
| `OPC.UA.DA.Access` | 1 read-only, 2 write-only, 3 read/write (default) | AccessLevel / UserAccessLevel | HIGH |
| `OPC.UA.DA.StructuredType` | 0/1 | StructuredDataType (ExtensionObject with type description) for a STRUCT | HIGH |
| `OPC.UA.DA.Alias` | string | Different browse/display name (no separate DisplayName pragma) | HIGH |
| `OPC.UA.DA.Description` | string | UA Description attribute; needs `OPC.UA.DA := '1'` too | HIGH |
| `OPC.UA.DA.Status` | quality | Force the StatusCode of the symbol | HIGH |
| `OPC.UA.DA.AnalogItemType := '1'` + `.EngineeringUnits` (UNECE id) + `.EURange 'min:max'` + `.InstrumentRange` + `.WriteBehavior` 0/1/2 | AnalogItemType with EURange/EngineeringUnits properties | HIGH |
| `OPC.UA.DA.Property := '1'` + `{attribute 'monitoring' := 'call'}` on a PLC PROPERTY | UA Property (TC3.1 4024+; `ImportPlcProperties` in TcUaDaConfig.xml) | HIGH |
| `OPC.UA.DA.Deactivate`, `.Unit`, `.DisplayName`, `.Method` | Not in Beckhoff docs; treat as non-existent | MEDIUM |

Methods (HIGH, tf6100_tc3_opcua_server/15617760267): `{attribute 'TcRpcEnable' := '1'}` on the METHOD plus `OPC.UA.DA := '1'` on the FB instance. Method node under the FB object node; inputs/outputs → InputArguments/OutputArguments; return value included; pointer VAR_IN_OUT rejected; runs in PLC task real-time context. **Not used by sildarvinnsla.**

Arrays (HIGH, 15548029195): one node by default (ArrayDimensions/ValueRank). `LegacyArrayHandling` publishes every element as its own node. Element NodeId form `MAIN.arr[0]` is ADS syntax (MEDIUM that the server uses it verbatim).

Enums (HIGH, 15563504395): UA enumerations are Int32 with EnumStrings. Base types beyond Int32 return BadOutOfRange unless `ImportBigEnumsNumeric`.

Structs (HIGH, 15563720971 and 2115999988321170610699): each STRUCT is an Object node with one child Variable per member; with StructuredType the parent is also readable as an ExtensionObject. Custom types registered under "BeckhoffCtrlTypes". `OPC.UA.AdditionalStructuredType.NamespaceName`/`.Id` + `pack_mode` map to companion-spec types. FB instances are Object nodes with inputs/outputs/locals as children.

### A3. Address space and NodeIds

- Namespace URI of runtime 1: `urn:BeckhoffAutomation:Ua:PLC1` (HIGH; some installs `urn://<HOST>/BeckhoffAutomation/Ua/PLC1`, MEDIUM). Namespace index typically 4, not guaranteed. The sildarvinnsla HMI hard-codes 4.
- String NodeIds: `ns=4;s=MAIN.nCounter`, `ns=4;s=GVL_Name.var`, `ns=4;s=MAIN.stData.member`, `ns=4;s=MAIN.fbMotor.bEnable` (HIGH for program/GVL, MEDIUM for nested).
- Hierarchy: `Objects → DeviceSet → PLC1` (OPC UA DI DeviceType: DeviceManual, DeviceRevision, Model, SerialNumber, DeviceState; program and GVL object nodes beneath); separate configuration namespace exposes server config (MEDIUM for exact DI names).

### A4. Data type mapping (PLCopen OPC 30000 §9.2 table 27; Beckhoff follows it)

BOOL→Boolean, SINT→SByte, USINT/BYTE/CHAR→Byte, INT→Int16, UINT/WORD/WCHAR→UInt16, DINT→Int32, UDINT/DWORD→UInt32, LINT→Int64, ULINT/LWORD→UInt64, REAL→Float, LREAL→Double, STRING/WSTRING→String, TIME→Int64 ms (0..4294967295 else BadOutOfRange), LTIME→Int64 ns, DATE/DT→DateTime, TOD→UInt32 ms of day, LDATE/LDT/LTOD→Int64 ns. Enums→Int32 enumeration. STRUCT→ExtensionObject (StructuredType) or Object node. FB→Object node. (HIGH; TIME/LTIME from tf6100_tc3_opcua_server/19793371147.)

### A5. Endpoint, security, config files

- `opc.tcp://<host>:4840`. Default policies Basic256Sha256, Aes256_Sha256_RsaPss, Aes128_Sha256_RsaOaep (Sign, SignAndEncrypt). `None` disabled since setup 4.3.28; `<AllowDeprecatedSecurityPolicies>` re-enables old ones (HIGH, 15563159819).
- Anonymous works only until the one-time TOFU initialisation; then Username/Password or certificate (HIGH, 15563121419). The sildarvinnsla compose file mounts client certs, so the HMI likely uses SignAndEncrypt. A dev emulator should default to None + Anonymous and offer Basic256Sha256 with self-signed certs.
- Config files: `TcUaServerConfig.xml` (endpoints, policies), `TcUaDaConfig.xml` (DA devices: AmsNetId + port 851/852, device type, symbol file, `LegacyArrayHandling`, `ImportPlcProperties`, `ImportBigEnumsNumeric`), `TcUaEventLogConfig.xml`. Exact schema not retrieved online (MEDIUM); the mirror's TF6100 PDF has the configurator chapters.

## B. Go OPC UA server libraries (Oct 2026)

| Library | License | Browse | Read/Write | Subscriptions | Call | Custom structs | None + anonymous |
|---|---|---|---|---|---|---|---|
| gopcua/opcua `server/` (v0.9.x) | MIT | yes | yes | yes | NO server-side Call | no DataTypeDefinition documented | yes; encryption "not fully functional" |
| awcullen/opcua `server/` (v1.4.0, Dec 2024; commits to 2026) | MIT | yes | yes | DataChange + Event + triggered | yes (`MethodNode.SetCallMethodHandler`) | `DataTypeNode`, NodeSet2 import (`LoadNodeSetFromBuffer`), ExtensionObject encoding of registered Go structs; DataTypeDefinition exposure MEDIUM, verify | `WithSecurityPolicyNone`, `WithAnonymousIdentity`, username func |
| open62541 via cgo | MPL-2.0 (C) | full | full | full | full | full | full; wrappers experimental, cgo + CMake, hurts single-binary cross-compile |

Recommendation: **awcullen/opcua**. Only pure-Go server with Call, DataType nodes and NodeSet import. Risks: single maintainer, small community. Mitigation: keep the address-space builder behind an interface. First spike must confirm StructureDefinition exposure for custom structs; fallback is generating a NodeSet2 XML with the type definitions and importing it.

Dart/Flutter: `open62541` pub package (centroid.is, MIT, 1.5.7+3, FFI via native build hooks; client connect/reconnect, browse, read/write, subscriptions, SignAndEncrypt, custom types via DynamicValue) is what tfc-hmi uses. `open62541_libs` bundles prebuilt binaries.

## C. EtherCAT simulation without hardware

ESI (ETG.2000 EtherCATInfo.xsd 1.20): `<EtherCATInfo><Vendor><Id>` / `<Descriptions><Devices><Device><Type ProductCode RevisionNo>` / `<Sm>` / `<RxPdo Sm="2"><Index><Entry><Index><SubIndex><BitLen><DataType>` / `<TxPdo Sm="3">` / `<Mailbox><CoE/>` / `<Dc>`.

TwinCAT `.xti` per-box export: `<TcSmItem ClassName="CFlbTermDef">` → `<DataTypes>` (generated BIT-level structs with `BitOffs`) → `<Box><EtherCAT VendorId ProductCode RevisionNo><SyncMan><Fmmu><Pdo Index Flags SyncMan><Entry Name Index Sub><Type>`. TwinCAT "Export Configuration File" produces `EtherCATConfig` XML v1.3 with `<Config><Master><Info><Name>` and `<Slave><Info>{Name,VendorId,ProductCode,PhysAddr,Physics}</Info><PreviousPort>{PhysAddr,Port}</PreviousPort><ProcessData><Sm><Pdo>..</Pdo></Sm><TxPdo>/<RxPdo><Index><Name><Entry>...`. This export is what `generate_gvl.py` consumes and what stc should load.

Process image: TwinCAT concatenates each slave's assigned PDO entries in bus order into the device's input/output image; each entry is a linkable variable; `AT %I*`/`%Q*` are bound by link, fixed `%IX` index the PLC image directly.

Simulators: Beckhoff TE1111 (second Windows PC emulating the slave list at frame level, commercial); acontis EC-Simulator (SiL from ENI, commercial); icECAT (commercial); IgH `libfakeethercat` (open source, process-data level via shared memory, no state-machine faults); SOES needs an ESC; no mature open-source frame-level slave emulator. TwinCAT itself without IO: disable the EtherCAT device or use project variants; unlinked `AT %I*` read zeros.

Decision: simulate at **process-image level with a PDO/terminal model** derived from the EtherCATConfig export (offsets, per-slave WcState/InfoData, realistic status words, CoE object dictionaries for drives). Frame level is out of scope.

## D. Terminal process data (Beckhoff defaults)

| Terminal | Inputs (TxPDO) | Outputs (RxPDO) | Conf |
|---|---|---|---|
| EL1008/EL1018 | 8 × BOOL `0x6000:01 … 0x6070:01`, TxPDO 0x1A00–0x1A07, 1 byte packed | — | HIGH |
| EL2008 | — | 8 × BOOL `0x7000:01 … 0x7070:01`, RxPDO 0x1600–0x1607 | HIGH |
| EL3004/EL3054/EL3064/EL3104 | per channel "AI Standard" 4 B: Status WORD + Value INT (`0x60n0:11`); Status bit0 Underrange, bit1 Overrange, bits2–3 Limit1, bits4–5 Limit2, bit6 Error, bit13 Sync error, bit14 TxPDO State, bit15 TxPDO Toggle. 0–10 V → 0..32767; 4–20 mA → 0..32767 | — | HIGH (EL32xx differs: bit3/4 and bit7) |
| EL4004/EL4024/EL4034 | — | 4 × INT `0x70n0:01` | MEDIUM, verify subindices |
| EL6224 IO-Link | DeviceState PDOs 0x1A04/0x1A05, port data 0x1A00–0x1A03 | ports 0x1600–0x1603 | HIGH |
| EL6001/EL6021 serial (22-byte mode) | Status WORD `0x3103:01` + 22 Data In bytes | Ctrl WORD `0x3003:01` + 22 Data Out bytes | MEDIUM-HIGH |
| EL7031/EL7041 stepper | ENC/STM/POS status PDOs 0x1A00–0x1A06 | ENC/STM/POS control 0x1600–0x1607 (Enable `0x7010:01`, Velocity `0x7010:21`) | HIGH |
| EL72x1 servo | Statusword `0x6010:01`, Modes display, actuals; DS402 alt `0x6041` | Controlword `0x7010:01`, Modes `0x7010:03`, targets; DS402 `0x6040/0x6060/0x607A/0x60FF`; enable 0x80→0x06→0x07→0x0F | HIGH |
| EL9011/EL9100/EL9186/EL9187 | passive, not in slave list | — | MEDIUM |
| EK1100/EK1110/EK1200 | slaves with no PDOs; InfoData only | — | MEDIUM |

sildarvinnsla-specific (from its `.xti` files, HIGH):
- **ATV320** (vendor 0x0800005a, product 0x389): TxPDO 0x1A00 = ETA 0x6041 UINT, RFR 0x2002:03 INT, LCR 0x2002:05 UINT, DI 0x2016:03 UINT, LFT 0x2029:16 UINT, HMIS 0x2002:29 UINT. RxPDO 0x1600 = CMD 0x6040 UINT, LFR 0x2037:03 INT, OL1R 0x2016:0D UINT, ACC 0x203C:02 UINT, DEC 0x203C:03 UINT. `FB_ATV320` also writes ~15 motor parameters and 0x2032:01 (EEPROM save) over CoE.
- **EL9222-5500** (product 0x24063052): per channel TxPDO Status bits Enabled, Tripped, Hardware Protection, Current Level Warning, Cool Down Lock, Diag, TxPDO State, Input cycle counter (2 bits); RxPDO Control bits Reset, Switch.
- **EP2338-0002/-1002**: 8 DI + 8 DO; **PS2001-2410**: Warning, Error, DC OK, Output voltage, Output current, Input undervoltage; **Festo CTEU**: outputs under `Module 1 (VAEM-L1-S-8-PT [16DO])`; **EL2912**: field-voltage diag under `Module 3 (DEVICEIO)`.

Per-slave TwinCAT pseudo-inputs (HIGH, tcsystemmanager/1089009035): `WcState` BOOL (0 valid); `InfoData.State` WORD: 0x1 INIT, 0x2 PREOP, 0x4 SAFEOP, 0x8 OP, 0x10 error flag, 0x100 slave not present, 0x200 link error, 0x400–0x8000 per-port link flags; `InfoData.AdsAddr` = master NetId (6 bytes) + port UINT (EtherCAT address 1001+).

EtherCAT master inputs (HIGH): `DevState` UINT bits 0x0001 link error, 0x0002 I/O locked, 0x0004 redundancy link error, 0x0008 missing frame, 0x0010 out of send resources, 0x0020 watchdog, 0x0040 driver not found, 0x0080 I/O reset active, 0x0100 slave INIT, 0x0200 PRE-OP, 0x0400 SAFE-OP, 0x0800 slave error, 0x1000 DC not synchronised. `Frm0State`/`FrmXState`, `Frm0WcState` (bit 0x8000 frame missing), `SlaveCount`, `ChangeCount`, `InfoData.AmsNetId`. InfoData is updated acyclically and can lag a cycle.

Open points: verify `TcUaDaConfig.xml` schema from the mirrored TF6100 PDF; confirm awcullen DataTypeDefinition exposure in a spike; check EL40xx subindices against the PDF before hard-coding.
