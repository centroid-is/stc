# Phase 28: OPC UA Address Space - Context

**Gathered:** 2026-10-06
**Status:** Ready for planning
**Mode:** Autonomous (user asked for maximum speed; parallel track D in worktree ../stc-wt-28, branch gsd/phase-28-opcua, based on main at 4007da7). The awcullen/opcua spike is DONE (28-RESEARCH.md = the spike report, verdict GO, prototype at /private/tmp/claude-501/-Users-jonb-Projects-stc/3a97ec58-502a-4334-8f74-c078fd46a32a/scratchpad/opcua-spike/). Success criterion 1 (spike) is therefore satisfied before planning.

<domain>
## Phase Boundary

`pkg/opcua`: an OPC UA server (awcullen/opcua v1.4.0, MIT) that publishes a TF6100-shaped address space from an abstract symbol model: Beckhoff namespace `urn:BeckhoffAutomation:Ua:PLC1` at index 4, string NodeIds `ns=4;s=GVL.fb[2].HMI.p_stat_State` with declared case, exposure by `OPC.UA.DA` rules ('1' inherits, '0' prunes, '2' flattens, type-level attributes apply to every instance, PROGRAM MAIN publishes nothing unless marked), `OPC.UA.DA.Access` → AccessLevel (missing = read/write), `OPC.UA.DA.Description` → Description, PLCopen OPC 30000 type mapping, StructuredType structs as ExtensionObjects with served StructureDefinition (reflect.StructOf runtime Go types registered per encoding id) while members stay individually readable, enums Int32 + EnumStrings, arrays as single nodes with ArrayDimensions, `i=2259` Running, SecurityPolicy None + Anonymous (writes allowed via role permissions) and optional Basic256Sha256 with self-signed certs.

Dependency isolation (binding, because Phases 21/22/23 are being built concurrently on other worktrees): `pkg/opcua` must compile and be fully tested WITHOUT pkg/symtree, pkg/vendor/twincat or an interp Runtime. It consumes two small interfaces defined in pkg/opcua: `SymbolNode` (Path, Name, Kind, IEC type `types.Type`, Attributes []ast.Attribute incl. inherited type-level ones, EnumStrings, Children()) and `NodeSource` (Read(path) (any, error), Write(path, any) error, Snapshot() for consistent reads). Plan 28-04 (adapters symtree→SymbolNode, interp.Runtime→NodeSource, `stc serve --project` CLI, free-running scan) is planned now but executed only after Phases 21, 22 and 23 are merged into main; the orchestrator will run it on main.

Not in scope: subscriptions wired to the live scan and HMI acceptance (Phase 29, though awcullen's poll-based monitored items already work through the read handler), RPC methods, AnalogItemType, LegacyArrayHandling, ADS.

</domain>

<decisions>
## Implementation Decisions

### Library and server core (OPCUA-01, OPCUA-02)
- Dependency: `github.com/awcullen/opcua v1.4.0` (pin exactly; note the Dec 2024 release and the possibility of forking). Tests use awcullen's own client package (no gopcua dependency). Binary growth ~8 MB accepted.
- `pkg/opcua.Server{Config{Endpoint ":4840", ApplicationURI "urn:stc:opcua", PLCNamespace "urn:BeckhoffAutomation:Ua:PLC1", AllowNone bool (default true), AllowAnonymous (default true), CertFile/KeyFile (auto-generate a self-signed pair under the user cache dir or a temp dir when None is used; required by the library even for None), EnableBasic256Sha256 bool}}`. Namespace index 4 is forced with filler namespaces `urn:stc:filler:2`, `urn:stc:filler:3` (as in the spike). Anonymous gets write permission through `WithRolePermissions`. `Start()`, `Stop()`, `Endpoint()`.
- Standard Server object served by the library; `i=2259` must read Running.

### Address-space builder (OPCUA-02, OPCUA-03, OPCUA-04)
- `Build(root SymbolNode, src NodeSource) (*Space, []Diagnostic)`: walks GVLs and PROGRAMs; node published iff `exposed(node)`: own `OPC.UA.DA` = '1' or '2', or an ancestor instance exposed with '1' (inheritance), or the declaring type (FB/STRUCT member) carries `OPC.UA.DA := '1'` (type-level); `'0'` on a node prunes it and its subtree; `'2'` publishes the node as a Variable (ExtensionObject) without member child nodes; intermediate Object nodes (FB instances, struct instances, arrays of structs) are created when any descendant is exposed. Objects: GVL → Object under `Objects/DeviceSet/PLC1` (DI DeviceType shape: Model "TwinCAT 3 PLC emulated by stc", DeviceManual, DeviceRevision "3.1", SerialNumber, DeviceState); FB instance → Object; struct instance → Variable with child member Variables when StructuredType, else Object with child Variables (decision: struct instance exposed WITHOUT StructuredType is an Object whose members are Variables; WITH StructuredType it is a Variable of the custom DataType whose children are also Variables, matching the HMI reading `GVL.fb.HMI` as one ExtensionObject and `GVL.fb.HMI.p_stat_Error` individually).
- NodeIds: `ns=4;s=<path>` with declared case and `[i]` indices as in the symbol path; BrowseName `4:<name>`; DisplayName = name. Description = `OPC.UA.DA.Description` value (unescaped).
- AccessLevel/UserAccessLevel: `OPC.UA.DA.Access` '1' → CurrentRead; '2' → CurrentWrite; '3' or missing → CurrentRead|CurrentWrite. Writes to read-only return BadNotWritable.

### Data type mapping and structured types (OPCUA-05, OPCUA-06)
- `pkg/opcua/typemap.go`: BOOL→Boolean, SINT→SByte, USINT/BYTE→Byte, INT→Int16, UINT/WORD→UInt16, DINT→Int32, UDINT/DWORD→UInt32, LINT→Int64, ULINT/LWORD→UInt64, REAL→Float, LREAL→Double, STRING/WSTRING→String, TIME→Int64 (ms), LTIME→Int64 (ns), DATE/DT→DateTime, TOD→UInt32 (ms of day), enums→Int32 with an EnumStrings property on the Variable (and an Enumeration DataType node per enum type under the PLC namespace), arrays→single Variable with ValueRank 1 and ArrayDimensions, REFERENCE/POINTER→skipped with a diagnostic.
- Structs with StructuredType: one DataType node per ST struct type (`ns=4;s=DT.<TypeName>` and encoding `ns=4;s=TE.<TypeName>`, matching the spike naming until TF6100's real names are verified) with `ua.StructureDefinition`; runtime Go type via `reflect.StructOf` with type-qualified field names to avoid the global registry collision; nested structs/arrays/enums/strings supported; whole-struct writes split into member writes through NodeSource.
- Value conversion helpers both directions (`toUA(iecType, any)`, `fromUA(iecType, ua.Variant)`), with TIME in ms.

### NodeSource and consistency
- `NodeSource.Snapshot() (map[path]any)`-style read snapshot per poll so handler goroutines read a consistent scan image; writes queued to apply between scans (the interp Runtime adapter in 28-04 implements this with the Phase 22/23 mutex).
- Test fake `MapSource` for all unit tests.

### CLI and adapters (plan 28-04, deferred execution)
- `stc serve --project <tsproj|plcproj|.st files> --opcua :4840 [--security basic256sha256] [--cert --key] [--cycle 10ms]` runs import (Phase 21) → analysis → symtree (Phase 22) → Runtime free-running scan (Phase 23) → opcua Server. JSON `--format json` prints endpoint, namespace index, node count.
- Golden test: build the address space for the committed ST301-shaped fixture (from Phase 22's `tests/`), browse it with the client and snapshot node ids/types/access levels to `tests/opcua_golden/st301_shape.json`.

### Claude's Discretion
- Package layout inside pkg/opcua, certificate generation details (crypto/x509 self-signed, 10-year), exact DI node property values, diagnostic texts.
- Coverage: pkg/opcua is not gated but target >= 85% (the library's goroutines make some paths hard; integration tests with a real listener on 127.0.0.1:0 are acceptable and must be deterministic with generous timeouts).

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Spike prototype (scratchpad) with working code for every item: namespace forcing, DataTypeNode with StructureDefinition, reflect.StructOf registration, Set*ValueHandler hooks, role permissions, client decode procedure; copy patterns from it.
- `pkg/types` (types.Type kinds incl. EnumType with ordinals/strings after Phase 20, StructType, ArrayType with bounds, FunctionBlockType), `pkg/ast.Attribute`.

### Established Patterns
- Go stdlib + minimal deps (go.mod currently has cobra, glsp, toml, mcp go-sdk, testify); adding awcullen/opcua is the one approved new dependency.
- Table-driven tests; CLI exec tests; deterministic ordering (sort node children by declaration order, never map iteration).

### Integration Points
- `pkg/opcua` ← SymbolNode/NodeSource adapters (28-04) ← pkg/symtree (22), pkg/interp Runtime (22/23), pkg/vendor/twincat import (21); `cmd/stc/serve.go` new command.

</code_context>

<specifics>
## Specific Ideas

- HMI expectations (from .planning/research/v1.2/SILDARVINNSLA-OPCUA-USAGE.md): node ids like `GVL_BatchLines.Drives_Line1[1].HMI` (whole struct as ExtensionObject) and `GVL_BatchLines.Drives_Line1[1].HMI.p_stat_Error`; `sensors.EPW01_WA01_IS11.HMI.p_stat_xRaw`; enum values decoded to names (`rdy(2)`); `ns=0;i=2259`.
- Attribute usage: `OPC.UA.DA` 474, `.Access` 398 ('1' 391, '3' 7), `.StructuredType` 328 (also on FB/TYPE headers), `.Description` 235, `'0'` twice; exposure often decided inside the FB type (`FB_Sensor` has `VAR PERSISTENT RETAIN HMI : ST_Sensor_HMI` with `OPC.UA.DA := '1'`, instances unmarked).
- Beckhoff type mapping and semantics: .planning/research/v1.2/BECKHOFF-REFERENCE.md section A.

</specifics>

<deferred>
## Deferred Ideas

- Live subscriptions tied to scan ticks, HMI acceptance, CI browse diff against the real TF6100 → Phase 29.
- TcRpcEnable methods, AnalogItemType, Alias, LegacyArrayHandling, username/password + trust list → v2.
- ADS server → v2.

</deferred>
