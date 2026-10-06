# Phase 21: TwinCAT Project Import & Library Stubs - Context

**Gathered:** 2026-10-06
**Status:** Ready for planning
**Mode:** Autonomous smart discuss (recommended answers accepted; overnight run, user unavailable)

<domain>
## Phase Boundary

A TwinCAT solution on disk becomes one stc project model: `stc vendor import <x.tsproj|x.plcproj>` reads every TcPOU (declaration, implementation, methods, actions, properties), TcGVL and TcDUT listed in the plcproj, resolves library placeholder references (project POUs → sibling library plcproj such as SVNCoreComponents → shipped `.st` stubs), reads the task cycle time and PLC project name from the `.tsproj`, and exposes the model to `stc check`, `stc test`, `stc sim` and `--format json`. Ships the Beckhoff stubs sildarvinnsla needs (Tc2_EtherCAT, Tc2_System additions, Tc2_ModbusSrv, Tc3_Module, Tc2_SerialCom) and fixes `stc vendor extract`. Acceptance oracle: `/Users/jonb/Projects/sildarvinnsla` (ST101, ST201, ST301, Baader `gagnasofnun`, SVNCoreComponents). Not in scope: EtherCAT `.xti`/export loading and `TcLinkTo` resolution (Phase 24), runtime task scheduling (Phase 23), OPC UA (Phase 28), `.library` binaries, TwinSAFE `.splcproj` projects (skip with a note).

</domain>

<decisions>
## Implementation Decisions

### Project model and import entry point
- New package `pkg/project/twincat` (or `pkg/vendor/twincat`; planner's call) with `Import(path) (*Model, []diag.Diagnostic, error)`. `Model{Name, AmsPort, Tasks []Task{Name, CycleTime time.Duration, Priority}, Sources []Source{Path, Kind (POU/GVL/DUT/ITF), ST text, line map back to the TcPOU XML}, Libraries []LibraryRef{Name, DefaultResolution, Namespace, ResolvedFrom (project|sibling|stub|unresolved), Position}}`.
- `.tsproj` → `<Plc><Project File="X.xti"/>` → `_Config/PLC/X.xti` `<Project Name PrjFilePath TmcFilePath AmsPort>` → the `.plcproj`. `.plcproj` directly is also accepted (no task info then; cycle time defaults to 10 ms with a warning). `<Task CycleTime>` is in 100 ns units (10000 = 1 ms).
- Each TcPOU/TcGVL/TcDUT is converted to ST text per object (declaration + implementation, methods as `METHOD ... END_METHOD` inside the FB with their own declaration/implementation, actions as `ACTION ... END_ACTION` inside the POU, properties as `PROPERTY ... GET ... SET ... END_PROPERTY`), with a source map so diagnostics point at the TcPOU file and the XML line of the CDATA text. GVL name comes from the TcGVL `Name=` attribute (feeds `GVLDecl.Name`). The existing `pkg/vendor/extract.go` TcPOU reader is extended, not duplicated.
- `stc vendor import <path>` prints a summary (POUs, GVLs, DUTs, libraries, tasks) in text and JSON; `--out <dir>` writes the converted `.st` files plus a generated `stc.toml` so the rest of the CLI works on the flattened project. `stc check <x.tsproj|x.plcproj>` and `stc test --project <x.tsproj>` accept a project path directly and import on the fly (same code path).

### Library resolution
- Order: (1) the project's own POUs/DUTs/GVLs; (2) sibling library plcproj: for each `<PlaceholderReference Include="X">` look for `X/X.plcproj`, `X/*/X.plcproj` or `*/X.plcproj` under the solution's parent directories (sildarvinnsla has `SVNCoreComponents/SVNCoreComponents/SVNCoreComponents.plcproj` beside `ST301/`), plus `[build.library_paths]` entries in stc.toml; (3) shipped stubs under `stdlib/vendor/beckhoff/<lib>.st` matched by library name case-insensitively (`Tc2_EtherCAT` → `tc2_ethercat.st`); (4) unresolved → diagnostic `VEND020 unresolved library reference 'X'` at the `<PlaceholderReference>` position in the plcproj, non-fatal.
- Library sources are analysed with `isLibrary=true` (no SEMA037 for names inside libraries, no unused warnings), same as stubs today. Namespace-qualified names `Tc2_EtherCAT.ST_EcSlaveState` resolve through the library's declarations.
- Tc2_Standard and Tc3_Module resolve to the built-in standard FBs plus a small stub (`Tc3_Module` is empty for our purposes; emit a stub file with a comment).

### Shipped stubs (hand-written `.st`, declaration only, bodies empty)
- `tc2_ethercat.st`: FB_EcGetSlaveState, FB_EcGetAllSlaveStates, FB_EcSetSlaveState, FB_EcGetMasterState, FB_EcGetAllSlaveCrcErrors, FB_EcGetSlaveCrcErrorEx, FB_EcCoESDoRead, FB_EcCoESDoWrite, FB_EcPhysicalWriteCmd, FB_EcGetSlaveCrcError, FB_EcGetAllSlaveAddr, with `ST_EcSlaveState{deviceState : BYTE; linkState : BYTE}`, `E_EcSlaveState`, `EcDiagParam`-style constants if Tc2 defines them (check InfoSys mirror at `/Users/jonb/Projects/beckhoff-docs/`). Parameter names and types must match Beckhoff docs (sNetId : T_AmsNetId, nSlaveAddr : UINT, bExecute : BOOL, tTimeout : TIME, bBusy/bError : BOOL, nErrId : UDINT, pDstBuf : PVOID/POINTER TO BYTE, cbBufLen : UDINT, nIndex : WORD, nSubIndex : BYTE, etc.).
- `tc2_system.st` additions: `AMSADDR{netId : AMSNETID; port : AMSPORT}`, `AMSNETID`/`T_AmsNetIdArr` (ARRAY[0..5] OF USINT), `AMSPORT` (UINT), `F_CreateAmsNetId`, `MEMCPY/MEMSET/MEMCMP` (already partly there), `FB_LocalSystemTime`, `RTC`, `ADSREAD/ADSWRITE` (exists).
- `tc2_modbussrv.st`: FB_MBReadRegs, FB_MBReadWriteRegs, FB_MBReadCoils, FB_MBReadInputs, FB_MBWriteRegs, FB_MBWriteCoils, FB_MBWriteSingleReg, FB_MBWriteSingleCoil, FB_MBReadInputRegs (Tc2_ModbusSrv is the server library; the client FBs are Tc2_ModbusRtu / TF6250 Tc2_ModbusTcp. Check which library sildarvinnsla actually references for FB_MBReadWriteRegs and ship the stub under the referenced library name so resolution works).
- `tc2_serialcom.st`: SerialLineControl, ComBuffer, SendString, ReceiveString, SendByte, ReceiveByte, ClearComBuffer, ST_ComBuffer types, E_ComMode etc., for Baader.
- `tc3_module.st`: empty with a header comment; `Tc3_IPCDiag` (Baader) likewise as a minimal stub so the reference resolves.
- Stub parameter typing: use exactly the Beckhoff declared types; where a Beckhoff type is unknown to stc (PVOID, __XWORD), add `TYPE PVOID : POINTER TO BYTE; END_TYPE` and `__XWORD : LWORD` aliases in `common_types.st`.

### `stc vendor extract` fixes (IMPT-05)
- Emits `END_FUNCTION_BLOCK`/`END_PROGRAM`/`END_FUNCTION`, includes methods, actions and properties, handles TcGVL and TcDUT entries (as GVL text with the XML `Name=`, and TYPE text), and never silently skips an entry (unknown extensions produce a warning). Output parses with `stc parse`.

### Checker/CLI integration
- `analyzer.Analyze` gains a project-aware entry (`AnalyzeProject(model)`) that registers library sources first, then user sources, keeping file-level diagnostics mapped to the original TcPOU paths.
- Exit criterion: `stc check` on the imported ST301 (with SVNCoreComponents resolved as a sibling library and Tc2 stubs) reports zero errors, after this phase's import plus Phase 20's checker. Known Phase 22 residuals (literal typing `x / 5.0`, `n := n + 1` on INT) are expected to appear; the plan must bucket them and the success criterion counts errors excluding the SEMA001/SEMA021 literal-typing class only if that class is explicitly listed in the SUMMARY. The 3 genuine SVNCore findings (SEMA024 on `settings.p_stat_Batches`) are reported, not hidden.
- JSON output of `stc vendor import` includes `plc_name`, `ams_port`, `tasks[].cycle_time_ns`, `libraries[].resolved_from`, and the source list.

### Claude's Discretion
- Package naming/location, source-map representation, how `stc check` detects a project path (extension), exact VEND code numbers, whether converted `.st` files are cached under `.stc-cache`.
- Whether to model `<Folder>`/virtual folders (not needed).

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/vendor/extract.go`: `ParseTcPOU`, `ParsePlcProj` (Compile Include entries), `ExtractStub`; `pkg/vendor/loader.go` `LoadLibraries(cfg, dir)` for `[build.library_paths]`; `pkg/vendor/mock.go`.
- `pkg/project` stc.toml config (`FindConfig`, `[build.library_paths]`, `[test].mock_paths`).
- Phase 19/20: GVLDecl with `SetGVLName`, ACTION parsing inside POUs, attributes, namespace-qualified types (`NamedType.Namespace`), SEMA037 with library exemption, `isLibrary` registration and first-library-wins dedupe in `pkg/checker/resolve.go`, pointer-stable two-pass registration.
- `tests/twincat_probes/` and `tests/twincat_probes_test.go` (oracle via `STC_PROBES_DIR`); `scripts/coverage-gate.sh`.

### Established Patterns
- Cobra commands in `cmd/stc/*.go` with `--format json`; exec-based CLI tests with GOCOVERDIR coverage.
- XML via `encoding/xml` only (CLAUDE.md); CDATA `<Declaration>`/`<Implementation><ST>` already parsed.
- Diagnostics carry `source.Pos{File, Line, Col}`; use the TcPOU path so LSP/editors open the right file.

### Integration Points
- `cmd/stc/vendor.go` (extract subcommand) → add `import`; `cmd/stc/check.go`/`test.go` project-path detection.
- `pkg/analyzer` facade → `AnalyzeProject`; `pkg/incremental` cache keyed by TcPOU path.
- `stdlib/vendor/beckhoff/*.st` + `stubs_test.go` (every stub must parse and check clean).
- `docs/VENDOR_LIBRARIES.md` and `docs/CLI_REFERENCE.md` updates.

</code_context>

<specifics>
## Specific Ideas

- Real layout: `ST301/ST301 solution.tsproj` → `<Plc><Project File="ST301.xti"/>` → `ST301/_Config/PLC/ST301.xti` with `PrjFilePath="..\..\ST301\ST301.plcproj"` (Windows backslashes; normalise) and `AmsPort="851"`; `<Task Id="3" Priority="20" CycleTime="10000" AmsPort="350">` (1 ms). ST301.plcproj has 24 `Compile Include` entries (GVLs\*.TcGVL, POUs\*.TcPOU, DUTs\*.TcDUT) and PlaceholderReferences: SVNCoreComponents (DefaultResolution "SVNCoreComponents, * (Centroid)"), Tc2_EtherCAT, Tc2_ModbusSrv, Tc2_Standard, Tc2_System, Tc3_Module. Baader's `gagnasofnun.plcproj` adds Tc3_IPCDiag. SVNCoreComponents has 71 files incl. methods and actions in `FB_ATV320.TcPOU` (`<Method Name=...>`, `<Action Name=...>`).
- Vendor FBs actually used (count): EcDiagParam 31 (project GVL), FB_EcDeviceDiag 28 (project FB), REAL_TO_UDINT 24, F_CreateAmsNetId 13, FB_EcGetAllSlaveStates 13, T_AmsNetIdArr 12, MEMCPY 10, E_EcSlaveState 9, FB_MBReadWriteRegs 5, ST_EcSlaveState 5, FB_EcGetSlaveCrcErrorEx 4, FB_EcSetSlaveState 3, FB_MBReadCoils 3, RTC 2, FB_EcGetSlaveState 2, FB_EcGetMasterState 2, FB_EcCoESDoRead/Write 2, FB_MBReadRegs 1, FB_LocalSystemTime 1, FB_EcPhysicalWriteCmd 1, FB_EcGetAllSlaveCrcErrors 1, AMSADDR 1, ADSREAD/ADSWRITE 1.
- The flattened probes (`/Users/jonb/Projects/beckhoff-docs/stc-probes/st301.st`) merged 17 GVLs into one and lost library context; after this phase the oracle should become the real tsproj import: add an env-gated test `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla` that imports ST301 and SVNCoreComponents and asserts the error count by code, with the expected Phase 22 buckets allow-listed explicitly.
- Beckhoff docs mirror for signatures: `/Users/jonb/Projects/beckhoff-docs/` (grep `pdf-text/` first; Tc2_EtherCAT and Tc2_System PDFs are there).

</specifics>

<deferred>
## Deferred Ideas

- Loading `.xti` EtherCAT config / `TcLinkTo` resolution → Phase 24.
- Task scheduling at runtime, PERSISTENT state → Phase 23.
- Literal typing / integer wrap → Phase 22.
- TwinSAFE `.splcproj` and `.tsproj` NC/IO sections → out of scope.
- `.library` / `.compiled-library` parsing → out of scope (stubs instead).

</deferred>
