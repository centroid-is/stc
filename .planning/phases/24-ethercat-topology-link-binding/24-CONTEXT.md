# Phase 24: EtherCAT Topology & Link Binding - Context

**Gathered:** 2026-10-06
**Status:** Ready for planning
**Mode:** Autonomous (user asked for maximum speed; parallel track C in worktree ../stc-wt-24, branch gsd/phase-24-ecat-topology, based on main at 385dced). Research is folded into planning: the planner reads the generator and the real exports directly.

<domain>
## Phase Boundary

`pkg/ecat` loads TwinCAT `EtherCATConfig` exports (`Device N.xml`) into masters, slaves and active PDO entries, reproducing the E-bus nesting, `Module N` segments and link paths of `/Users/jonb/Projects/sildarvinnsla/IO List from ethercat/generate_gvl.py`; parses and resolves `TcLinkTo` pragma strings to (master, byte, bit) slots in per-master input/output process images; copies linked `AT %I*`/`%Q*` variables (GVL scalars, struct members, FB instance members) from/to the images at scan boundaries; publishes slave `WcState`/`InfoData.State`/`InfoData.AdsAddr` and master `DevState`/`SlaveCount`/`Frm0State`/`Frm0WcState`/`InfoData.AmsNetId` with Beckhoff bit semantics; `stc ecat validate` reports unresolved links and size mismatches with GVL positions. Device behaviour models (EL1008 inputs, EL9222 trips, ATV320 CiA402) are Phases 25/26; this phase ships only the `Device` interface, a registry keyed by (VendorId, ProductCode) and a generic byte-passthrough default. Scenario files and `stc sim --io` CLI wiring are Phase 27; the Tc2_EtherCAT behavioural mocks are Phase 26. Project import (Phase 21) and the symbol tree / Get-Set (Phase 22) run in parallel on other worktrees: do not edit pkg/vendor, pkg/symtree, cmd/stc/vendor*.go, or the Phase 22 interp files beyond adding new files; keep interp changes in new files (`pkg/interp/iobind.go`) plus minimal hooks in scan.go.

</domain>

<decisions>
## Implementation Decisions

### Topology loader (ECAT-01)
- `pkg/ecat/topology.go`: `LoadConfig(path) (*Master, error)` parsing `EtherCATConfig` XML v1.3 with `encoding/xml`: `Config/Master/Info/Name`, each `Config/Slave`: `Info/{Name,VendorId,ProductCode,RevisionNo,PhysAddr,Physics}`, `PreviousPort/{PhysAddr,Port}`, `ProcessData` with `Sm/Pdo` assignment (active PDO indices) and `TxPdo`/`RxPdo` `{Index, Name, Entry{Name, Index, SubIndex, BitLen, DataType}}`. Types mirror the generator: `Master{Name, Slaves}`, `Slave{Name, Model, Vendor, Product, Revision, Phys, Physics, PrevPhys, PrevPort, Pdos, Parent *Slave, IsEBus}`, `Pdo{Name, Index, Direction (I/O), Entries}`, `Entry{Name, Index, SubIndex, BitLen, DataType}`.
- Port the generator's rules exactly: `active_pdo_indices` (only PDOs assigned to an enabled SyncManager are placed), `classify_role` by `Physics` (`open` = Ethernet-in/E-bus-out e.g. "YK"/"YKY", `terminal` = "KK", `close` = "KY", `plain` = "YY"; fallback by model prefix EK1100/1101/1200/1501/1521 open, EK1110/1120/1210/1220 close), `assign_parents` (terminals nest under the open coupler they chain from via `PreviousPort`), `slave_base_path` (`TIID^master^[coupler]^slave`), `MODULE_MAP` keyed by (vendor, product, pdo name) → `Module N (...)` segment (EL2912 FIELDVOLTAGE → "Module 3 (DEVICEIO)", Festo CTEU Outputs → "Module 1 (VAEM-L1-S-8-PT [16DO])"), `link_path` (entry names with `__` become `^` levels). Padding entries (`#x0` index, empty name) occupy bits but are not linkable.
- Multiple masters: `LoadProject(paths ...string) (*Topology, error)` returns all masters by name (ST301 has Device 1..4).

### Process image layout (ECAT-03)
- Per master: `Image{In, Out []byte}`. Entries are laid out in bus order (slave order in the export), PDO by PDO: bits packed within a PDO in entry order, each PDO starting byte-aligned; TxPdo → In, RxPdo → Out. Each slave additionally gets pseudo-input slots appended after its PDO entries: `WcState` (1 bit, byte-aligned), `InfoData^State` (16 bits), `InfoData^AdsAddr` (64 bits: 6-byte NetId + 2-byte port). Each master gets pseudo-inputs at the end of its input image: `DevState` (16), `SlaveCount` (16), `Frm0State` (16), `Frm0WcState` (16), `InfoData^AmsNetId` (48), `InfoData^ChangeCount` (16). Exact offsets are an internal choice (ST code only sees links), but they must be stable and reported by `stc ecat validate --format json`.
- `Topology.Slot(linkPath) (Slot{Master, Dir, Byte, Bit, BitLen}, bool)` resolves a `TIID^...` path to a slot.

### TcLinkTo parsing and resolution (ECAT-02)
- `pkg/ecat/link.go`: `ParseTcLinkTo(value string) ([]Link, error)` handles the single form `TIID^Device 1 (EtherCAT)^...` (binds the annotated variable itself) and the multi-member form `.I1 := TIID^...; .I2 := TIID^...` (member paths may be nested `.p_stat_X` or `.sub.member`, whitespace-tolerant, `;`-separated, trailing `;` allowed). Each `Link{Member string (empty for the variable itself), Target string}`.
- `pkg/ecat/resolve.go`: `Resolve(topo *Topology, decls []LinkedVar) ([]Binding, []diag.Diagnostic)` where `LinkedVar{Path string (e.g. "ECT.ST301_A1_03.I1"), Type types.Type, Attr ast.Attribute position}` comes from walking analysed GVLs/POUs for `TcLinkTo` attributes (variable-level, struct-member-level inside a linked struct instance, FB-member-level inside a linked FB instance). Diagnostics: `ECAT001 link target not found` (with the nearest matching prefix in the message), `ECAT002 member not declared`, `ECAT003 size mismatch` (variable bit width vs entry BitLen; BOOL↔1, BYTE/SINT/USINT↔8, INT/UINT/WORD↔16, DINT/UDINT/DWORD/REAL↔32, LINT/ULINT/LWORD/LREAL↔64, enums by base type, structs by summed member width when the target is a composite, `T_AmsNetIdArr`/`AMSNETID` ↔ 48, `AMSADDR` ↔ 64, `T_AmsNetId` STRING ↔ 48 accepted with a conversion rule), `ECAT004 direction mismatch` (output var linked to an input entry), `ECAT005 duplicate binding of the same slot by two variables` (warning).
- Match semantics: exact, case-sensitive segment match after trimming; TwinCAT box names contain spaces and parentheses which must round-trip.

### Scan-boundary copying (ECAT-03) and healthy-network pseudo-inputs (ECAT-07)
- `pkg/interp/iobind.go`: `type IOBinder struct` created from `[]Binding` + a `*ecat.Images`; `SyncIn()` before each scan copies input slots into bound variables (GVL scalars, struct members, FB instance members, with sign extension by declared type, enums by base, AMSADDR/NetId arrays byte-wise), `SyncOut()` after each scan copies bound output variables into output slots. Hook points in `ScanCycleEngine.Tick` (two calls) guarded by nil binder; variable access through the existing env/RefPath machinery (no dependency on Phase 22's symtree).
- `pkg/ecat/network.go`: `Network{Topo, Images, Devices map[slaveIndex]Device}` with `Step(dt)`: for each slave, call `Device.Step(dt, outSlice) inSlice` (generic passthrough default returns zero inputs); then write pseudo-inputs: healthy defaults `InfoData.State = 0x0008` (OP), `WcState = 0`, `InfoData.AdsAddr = {masterNetId, 1001+i}` (use `PhysAddr` from the export when present), `DevState = 0`, `SlaveCount = len(slaves)`, `Frm0State = 0`, `Frm0WcState = 0`, `InfoData.AmsNetId = master NetId` (default 192.168.0.1.1.1; the tsproj `TargetNetId` can be passed in), `ChangeCount = 0`. A `Fault` API (`SetSlaveState(i, state)`, `SetWcState(i, bad)`, `SetDevState(bits)`) exists for Phase 27 scenarios.
- `Device` interface: `type Device interface { Init(s *Slave); Step(dt time.Duration, out []byte, in []byte) }` plus `Registry` keyed by (VendorId, ProductCode) with `Default` passthrough; Phase 25/26 register real models.

### CLI (ECAT-02)
- `stc ecat validate --io <Device*.xml>... <gvl or project files>`: loads topology, analyses the ST (reusing the analyzer; GVL files via `--gvl-name`-less multi-file parse, names from file), resolves links, prints a table (variable, link, slot) or JSON (`bindings[]`, `diagnostics[]`, `images{master: {inBytes, outBytes}}`), exit code 1 on ECAT001-004.

### Claude's Discretion
- Internal layout details, diagnostic wording, whether `pkg/ecat` exposes a `Dump()` for debugging.
- Coverage: pkg/ecat is not gated but target >= 90%; interp gate 95% (currently 98.1%) must hold for iobind.go.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/iomap` (flat byte tables, Get/SetBit/Byte/Word/DWord; wildcard address parsing) can back `Images` or be replaced; `pkg/interp/scan.go` already syncs explicit AT addresses at scan boundaries (the hook pattern to mirror).
- Attributes retained in the AST since Phase 19: `VarDecl.Attributes`, `StructMember.Attributes`, GVLDecl; `Attribute{Name, Value}` with `''` unescaped; checker symbol table with GVL struct types and FB types to walk declarations.
- `pkg/ast` walkers, `pkg/diag` codes, cobra CLI patterns with `--format json`, exec tests via GOCOVERDIR.

### Established Patterns
- `encoding/xml` only; table-driven tests; deterministic ordering (sort slices, never range over maps for output).

### Integration Points
- `cmd/stc/ecat.go` new command; `pkg/interp/scan.go` Tick hooks; `pkg/analyzer` for walking GVL attributes (read-only use).

</code_context>

<specifics>
## Specific Ideas

- Real inputs: `/Users/jonb/Projects/sildarvinnsla/IO List from ethercat/ST301/Device 1.xml` .. `Device 4.xml` (and ST101/ST201/baader), generator at `IO List from ethercat/generate_gvl.py` (dataclasses Entry/Pdo/Slave, `parse`, `classify_role`, `assign_parents`, `slave_base_path`, `MODULE_MAP`, `link_path`, `iec_type`), generated GVLs `ST301/ST301/GVLs/ECT.TcGVL` and `ECT_Diag.TcGVL` (675 `TcLinkTo` pragmas across the projects; shapes: `TIID^Device 1 (EtherCAT)^InfoData^AmsNetId`, `.I1 := TIID^Device 1 (EtherCAT)^ST301.A1.00 (EK1200)^ST301.A1.03 (EL1008)^Channel 1^Input; ...`, `.p_stat_Enabled := TIID^...^ST301.A1.02 (EL9222-5500)^OCP Inputs Channel 1^Status^Enabled; ... .p_cmd_Reset := TIID^...^OCP Outputs Channel 1^Control^Reset`, `.q_uCMD := TIID^Device 1 (EtherCAT)^CVS03.CN01.FD01 (ATV320 EtherCAT)^Outputs^CMD; ... .amsaddr := TIID^...^InfoData^AdsAddr`, `TIID^Device 1 (EtherCAT)^ST301.A1.00 (EK1200)^ST301.A1.09 (EL2912)^Module 3 (DEVICEIO)^FIELDVOLTAGE Field Voltage Status^Fieldvoltage Underrange`, `TIID^Device 2 (EtherCAT)^ST302.A1 (CTEU-EtherCAT Modular)^Module 1 (VAEM-L1-S-8-PT [16DO])^Outputs^C1 Output`).
- Equivalence gate (env-gated `STC_SILD_DIR`): load ST301 Device 1..4.xml, parse ST301's `ECT.TcGVL` + `ECT_Diag.TcGVL` (convert the TcGVL CDATA to ST in the test, or read the flattened probe `/Users/jonb/Projects/beckhoff-docs/stc-probes/st301.st` with STC_PROBES_DIR) plus the SVNCore DUTs `ST_EL1008`, `ST_EL2008`, `ST_EL9222_5500`, `ST_EP2338_0002`, `ST_PS2001_2410` and `FB_ATV320`'s AT members, and assert zero unresolved links. That is the "same as generate_gvl.py" proof without running Python.
- Committed fixture: a synthetic `tests/ecat_fixtures/Demo Device 1.xml` with EK1200 → EL1008, EL2008, EL9222-5500, EL2912 (module), EK1110; a plain ATV320, an EP2338, a PS2001 and a Festo CTEU; plus a matching `demo_ect.st` GVL with TcLinkTo pragmas covering every shape above (single, multi-member, InfoData, module segment, AmsNetId).
- Healthy-network values the PLC code reads (`FB_EcDeviceDiag`, `FB_ATV320` gate on `ec.deviceState = 8`): InfoData.State 0x8, WcState 0, DevState 0, SlaveCount = configured count.

</specifics>

<deferred>
## Deferred Ideas

- Device behaviour models → Phase 25 (digital/analog/EL9222/PSU/EL6001) and Phase 26 (ATV320 + CoE + Tc2_EtherCAT mocks).
- Scenario TOML, fault injection CLI, `stc sim --io` → Phase 27.
- Frame-level EtherCAT, ESC registers, DC → out of scope.

</deferred>
