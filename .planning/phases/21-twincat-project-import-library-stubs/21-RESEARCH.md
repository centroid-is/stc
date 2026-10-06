# Phase 21: TwinCAT Project Import & Library Stubs - Research

**Researched:** 2026-10-06
**Domain:** TwinCAT 3 project file formats (tsproj, xti, plcproj, TcPOU/TcGVL/TcDUT/TcTTO), library resolution, Beckhoff library signatures, stc analyzer/CLI integration
**Confidence:** HIGH for file formats, measured baselines and Beckhoff signatures (all read from local files and the offline Beckhoff manuals); MEDIUM for TwinCAT transitive-library visibility and the double-quoted attribute rule (inferred from production code that compiles)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Project model and import entry point
- New package `pkg/project/twincat` (or `pkg/vendor/twincat`; planner's call) with `Import(path) (*Model, []diag.Diagnostic, error)`. `Model{Name, AmsPort, Tasks []Task{Name, CycleTime time.Duration, Priority}, Sources []Source{Path, Kind (POU/GVL/DUT/ITF), ST text, line map back to the TcPOU XML}, Libraries []LibraryRef{Name, DefaultResolution, Namespace, ResolvedFrom (project|sibling|stub|unresolved), Position}}`.
- `.tsproj` → `<Plc><Project File="X.xti"/>` → `_Config/PLC/X.xti` `<Project Name PrjFilePath TmcFilePath AmsPort>` → the `.plcproj`. `.plcproj` directly is also accepted (no task info then; cycle time defaults to 10 ms with a warning). `<Task CycleTime>` is in 100 ns units (10000 = 1 ms).
- Each TcPOU/TcGVL/TcDUT is converted to ST text per object (declaration + implementation, methods as `METHOD ... END_METHOD` inside the FB with their own declaration/implementation, actions as `ACTION ... END_ACTION` inside the POU, properties as `PROPERTY ... GET ... SET ... END_PROPERTY`), with a source map so diagnostics point at the TcPOU file and the XML line of the CDATA text. GVL name comes from the TcGVL `Name=` attribute (feeds `GVLDecl.Name`). The existing `pkg/vendor/extract.go` TcPOU reader is extended, not duplicated.
- `stc vendor import <path>` prints a summary (POUs, GVLs, DUTs, libraries, tasks) in text and JSON; `--out <dir>` writes the converted `.st` files plus a generated `stc.toml` so the rest of the CLI works on the flattened project. `stc check <x.tsproj|x.plcproj>` and `stc test --project <x.tsproj>` accept a project path directly and import on the fly (same code path).

#### Library resolution
- Order: (1) the project's own POUs/DUTs/GVLs; (2) sibling library plcproj: for each `<PlaceholderReference Include="X">` look for `X/X.plcproj`, `X/*/X.plcproj` or `*/X.plcproj` under the solution's parent directories (sildarvinnsla has `SVNCoreComponents/SVNCoreComponents/SVNCoreComponents.plcproj` beside `ST301/`), plus `[build.library_paths]` entries in stc.toml; (3) shipped stubs under `stdlib/vendor/beckhoff/<lib>.st` matched by library name case-insensitively (`Tc2_EtherCAT` → `tc2_ethercat.st`); (4) unresolved → diagnostic `VEND020 unresolved library reference 'X'` at the `<PlaceholderReference>` position in the plcproj, non-fatal.
- Library sources are analysed with `isLibrary=true` (no SEMA037 for names inside libraries, no unused warnings), same as stubs today. Namespace-qualified names `Tc2_EtherCAT.ST_EcSlaveState` resolve through the library's declarations.
- Tc2_Standard and Tc3_Module resolve to the built-in standard FBs plus a small stub (`Tc3_Module` is empty for our purposes; emit a stub file with a comment).

#### Shipped stubs (hand-written `.st`, declaration only, bodies empty)
- `tc2_ethercat.st`: FB_EcGetSlaveState, FB_EcGetAllSlaveStates, FB_EcSetSlaveState, FB_EcGetMasterState, FB_EcGetAllSlaveCrcErrors, FB_EcGetSlaveCrcErrorEx, FB_EcCoESDoRead, FB_EcCoESDoWrite, FB_EcPhysicalWriteCmd, FB_EcGetSlaveCrcError, FB_EcGetAllSlaveAddr, with `ST_EcSlaveState{deviceState : BYTE; linkState : BYTE}`, `E_EcSlaveState`, `EcDiagParam`-style constants if Tc2 defines them (check InfoSys mirror at `/Users/jonb/Projects/beckhoff-docs/`). Parameter names and types must match Beckhoff docs (sNetId : T_AmsNetId, nSlaveAddr : UINT, bExecute : BOOL, tTimeout : TIME, bBusy/bError : BOOL, nErrId : UDINT, pDstBuf : PVOID/POINTER TO BYTE, cbBufLen : UDINT, nIndex : WORD, nSubIndex : BYTE, etc.).
- `tc2_system.st` additions: `AMSADDR{netId : AMSNETID; port : AMSPORT}`, `AMSNETID`/`T_AmsNetIdArr` (ARRAY[0..5] OF USINT), `AMSPORT` (UINT), `F_CreateAmsNetId`, `MEMCPY/MEMSET/MEMCMP` (already partly there), `FB_LocalSystemTime`, `RTC`, `ADSREAD/ADSWRITE` (exists).
- `tc2_modbussrv.st`: FB_MBReadRegs, FB_MBReadWriteRegs, FB_MBReadCoils, FB_MBReadInputs, FB_MBWriteRegs, FB_MBWriteCoils, FB_MBWriteSingleReg, FB_MBWriteSingleCoil, FB_MBReadInputRegs (Tc2_ModbusSrv is the server library; the client FBs are Tc2_ModbusRtu / TF6250 Tc2_ModbusTcp. Check which library sildarvinnsla actually references for FB_MBReadWriteRegs and ship the stub under the referenced library name so resolution works).
- `tc2_serialcom.st`: SerialLineControl, ComBuffer, SendString, ReceiveString, SendByte, ReceiveByte, ClearComBuffer, ST_ComBuffer types, E_ComMode etc., for Baader.
- `tc3_module.st`: empty with a header comment; `Tc3_IPCDiag` (Baader) likewise as a minimal stub so the reference resolves.
- Stub parameter typing: use exactly the Beckhoff declared types; where a Beckhoff type is unknown to stc (PVOID, __XWORD), add `TYPE PVOID : POINTER TO BYTE; END_TYPE` and `__XWORD : LWORD` aliases in `common_types.st`.

#### `stc vendor extract` fixes (IMPT-05)
- Emits `END_FUNCTION_BLOCK`/`END_PROGRAM`/`END_FUNCTION`, includes methods, actions and properties, handles TcGVL and TcDUT entries (as GVL text with the XML `Name=`, and TYPE text), and never silently skips an entry (unknown extensions produce a warning). Output parses with `stc parse`.

#### Checker/CLI integration
- `analyzer.Analyze` gains a project-aware entry (`AnalyzeProject(model)`) that registers library sources first, then user sources, keeping file-level diagnostics mapped to the original TcPOU paths.
- Exit criterion: `stc check` on the imported ST301 (with SVNCoreComponents resolved as a sibling library and Tc2 stubs) reports zero errors, after this phase's import plus Phase 20's checker. Known Phase 22 residuals (literal typing `x / 5.0`, `n := n + 1` on INT) are expected to appear; the plan must bucket them and the success criterion counts errors excluding the SEMA001/SEMA021 literal-typing class only if that class is explicitly listed in the SUMMARY. The 3 genuine SVNCore findings (SEMA024 on `settings.p_stat_Batches`) are reported, not hidden.
- JSON output of `stc vendor import` includes `plc_name`, `ams_port`, `tasks[].cycle_time_ns`, `libraries[].resolved_from`, and the source list.

### Claude's Discretion
- Package naming/location, source-map representation, how `stc check` detects a project path (extension), exact VEND code numbers, whether converted `.st` files are cached under `.stc-cache`.
- Whether to model `<Folder>`/virtual folders (not needed).

### Deferred Ideas (OUT OF SCOPE)
- Loading `.xti` EtherCAT config / `TcLinkTo` resolution → Phase 24.
- Task scheduling at runtime, PERSISTENT state → Phase 23.
- Literal typing / integer wrap → Phase 22.
- TwinSAFE `.splcproj` and `.tsproj` NC/IO sections → out of scope.
- `.library` / `.compiled-library` parsing → out of scope (stubs instead).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| IMPT-01 | `stc vendor import <x.tsproj\|x.plcproj>` reads TcPOU (declaration, implementation, methods, actions, properties), TcGVL and TcDUT files listed in the plcproj and builds one project model stc can check and run | File shapes (section "TwinCAT file formats"), line-preserving converter prototype (Pattern 2, verified zero parse diagnostics on FB_ATV320, MAIN, FB_SerialFramer, FB_Wagon), PROPERTY access-modifier parser gap (Pitfall 4), measured check baseline (section "Measured baseline") |
| IMPT-02 | Library placeholder references resolve in order: project POUs, sibling library plcproj (SVNCoreComponents), shipped stubs; unresolved references are reported as diagnostics | PlaceholderReference shapes, sibling search layout, stub dependency closure (Tc2_EtherCAT needs Tc2_Utilities), first-library-wins ordering, position capture with `xml.Decoder.InputPos` (Pattern 3) |
| IMPT-03 | Task cycle time and PLC project name are read from the `.tsproj` and used by the runtime and the OPC UA namespace | tsproj `<Task CycleTime>` (100 ns units), inline vs `File=` PLC project, `PlcTask.TcTTO` (`<CycleTime>` in microseconds, `<PouCall>` program list); measured ST301 = 1 ms, Baader = 20 ms |
| IMPT-04 | Shipped stubs cover Tc2_EtherCAT, Tc2_System additions, Tc2_ModbusSrv, Tc3_Module and Tc2_SerialCom so the sildarvinnsla projects type-check | Full signature list (section "Stub signatures"), each read from the Beckhoff manuals; `E_EcSlaveState` is an SVNCoreComponents type, not Beckhoff; AMSADDR layout read from the tsproj type system; PVOID semantics |
| IMPT-05 | `stc vendor extract` emits stubs that parse (closing keywords, methods included) and no longer silently skips TcGVL/TcDUT entries | Measured: current extract on SVNCoreComponents writes 27 of 65 objects and `FB_ATV320.st` fails with "expected KwEndFunctionBlock, got EOF"; map iteration makes stdout order random |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Go stdlib only for XML: use `encoding/xml`. No new dependencies (no etree). [CITED: CLAUDE.md "Recommended Stack"]
- No Java at runtime. All compiler core in Go.
- Error recovery: the parser must produce partial ASTs from broken code.
- Determinism: all test execution must be deterministic, no wall-clock dependencies. This also rules out map-iteration order in CLI output and library registration order.
- Machine-readable output: every CLI command supports `--format json`.
- Do not parse `.library` binaries; TcPOU XML extraction and hand-written `.st` stubs only.
- No EtherCAT terminal models and no hardware address validation in this phase (CLAUDE.md "What NOT to Use").
- GSD workflow: edits only through GSD commands.
- User memory: work on main, keep CI green on macOS/Windows/Linux, run `scripts/coverage-gate.sh` before pushing; core packages need 95%+ statement coverage, exec-based CLI tests must capture coverage through GOCOVERDIR.

## Summary

Multi-file loading alone removes almost all of the Phase 20 "missing library" residue. A throwaway converter (scratchpad only) loaded every plcproj-listed TcPOU, TcGVL and TcDUT through the current `parser.Parse` and `analyzer.Analyze`. ST301 went from 1782 errors (flattened oracle) to 262 with its own files, to 42 with SVNCoreComponents passed as `LibraryFiles`. Adding draft Tc2 stubs left 42, but every remaining Tc2 SEMA037 disappeared. The rest are Phase 22 literal typing and built-ins (34), five SEMA033 caused by a double-quoted `qualified_only`, and three genuine findings: sildarvinnsla HEAD 549e3a9 renamed `FB_TwoWayConveyor` to `FB_BatchConveyor` in SVNCoreComponents but ST301's `SPB03` GVL still declares `FB_TwoWayConveyor`, and MAIN writes `ST_LineRecipe.stopDistanceFromEnd`, which no longer exists. A literal "zero errors" exit is therefore not reachable on HEAD. The gate must allow-list the Phase 22 class and list the genuine findings by name.

The file formats are simple and fully covered by `encoding/xml`. Two shapes of tsproj exist in the oracle: ST101/ST201/ST301 reference `_Config/PLC/X.xti` (whose `PrjFilePath` is relative to the xti), while Baader embeds `<Plc><Project Name PrjFilePath AmsPort>` inline (path relative to the tsproj). Every plcproj also lists `PlcTask.TcTTO`, which names the program the task calls (`<PouCall><Name>MAIN`) and repeats the cycle time in microseconds. A converter that writes each CDATA segment at its own XML line and column gives ST text whose line and column equal the TcPOU XML position, so no source-map remapping is needed (prototype verified on four real files with zero parse diagnostics).

The stubs need care in four places. `E_EcSlaveState` is declared by SVNCoreComponents, not Beckhoff. `T_AmsNetIdArr` is `ARRAY[0..5] OF BYTE`, not USINT. The TwinCAT type system in the Baader tsproj defines `AMSADDR` as `netId : AMSNETID (ARRAY[0..5] OF BYTE); port : WORD`. `FB_LocalSystemTime`, `RTC`, `TIMESTRUCT` and `SYSTEMTIME_TO_DT` are Tc2_Utilities, which no sildarvinnsla plcproj references, so stub resolution needs a dependency closure (Tc2_EtherCAT depends on Tc2_Utilities). Baader does not use Tc2_SerialCom at all; it has its own `FB_SerialFramer` over the EL6001 process image. Two checker/parser gaps block a clean run once stubs load: `fb();` with empty arguments reports SEMA022 "not callable" for every FB instance, and `PROPERTY PUBLIC P : INT` (TwinCAT's default property header) fails to parse.

**Primary recommendation:** Build `pkg/vendor/twincat` with a line-preserving TcPOU-to-ST converter, an ordered library resolver backed by `//go:embed` stubs with a dependency closure, and `Import()` returning a model the CLI feeds straight into `analyzer.Analyze` (sibling libraries before stubs). Fix the empty-call SEMA022, the PROPERTY access modifier and the double-quoted attribute rule in this phase, and gate the oracle on explicit per-owner buckets.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| tsproj / xti / plcproj / TcTTO reading | `pkg/vendor/twincat` (import layer) | — | Pure file-format work; no semantic knowledge needed |
| TcPOU/TcGVL/TcDUT/TcIO → ST text | `pkg/vendor/twincat` | `pkg/vendor/extract.go` (thin wrapper, stub mode) | One converter, two renderers (full body for import, declarations for extract) |
| Library resolution (sibling, library_paths, embedded stubs) | `pkg/vendor/twincat` | `stdlib/vendor/beckhoff` (embed.FS) | Resolver owns order and diagnostics; stubs own content |
| Parsing converted text | `pkg/parser` / `pkg/pipeline` | — | Unchanged entry points |
| Declaration registration, first-library-wins | `pkg/checker/resolve.go` | `pkg/analyzer` | Already supports `LibraryFiles` with `isLibrary=true` |
| Empty-call FB check, PROPERTY modifier, quote rule | `pkg/checker` / `pkg/parser` / `pkg/ast` | — | Gaps exposed once types resolve |
| CLI (`vendor import`, `check <proj>`, `test --project`, `sim <proj>`) | `cmd/stc` | `pkg/testing` (RunOpts.ProjectFiles) | CLI chooses path mode by extension |
| Task cycle time / PLC name exposure | `pkg/vendor/twincat` Model | `pkg/project` Config (`--out` stc.toml) | Model is the source; Phase 23/28 consume it |

## TwinCAT file formats (all read from /Users/jonb/Projects/sildarvinnsla at 549e3a9)

### tsproj
```xml
<TcSmProject ... TcVersion="3.1.4026.22">
  <DataTypes> ... </DataTypes>                 <!-- Baader only: system types AMSNETID, AMSADDR, IO types -->
  <Project ProjectGUID=... TargetNetId="10.1.177.66.1.1" Target64Bit="true">
    <System>
      <Tasks>
        <Task Id="3" Priority="20" CycleTime="10000" AmsPort="350" AdtTasks="true">
          <Name>PlcTask</Name>
        </Task>
      </Tasks>
    </System>
    <Plc>
      <Project File="ST301.xti"/>              <!-- ST101/ST201/ST301 form -->
    </Plc>
    <Safety><Project File="Line3.xti"/></Safety>   <!-- TwinSAFE: skip with a note -->
```
- `TcSmProject/Project/System/Tasks/Task@CycleTime` is in 100 ns units: ST101/ST201/ST301 `10000` = 1 ms, Baader `200000` = 20 ms. [VERIFIED: grep of the four tsproj files]
- Baader form (inline, no `_Config/PLC`): `<Plc><Project GUID=... Name="gagnasofnun" PrjFilePath="gagnasofnun\gagnasofnun.plcproj" TmcFilePath=... AmsPort="851">` at line 509 of `Baader/Sildarvinnsla Baader.tsproj`. PrjFilePath is relative to the tsproj directory. [VERIFIED]
- `File=` form: `ST301/_Config/PLC/ST301.xti` line 3: `<Project GUID=... Name="ST301" PrjFilePath="..\..\ST301\ST301.plcproj" TmcFilePath="..\..\ST301\ST301.tmc" AmsPort="851">`. PrjFilePath is relative to the xti file (`_Config/PLC/` → `ST301/ST301/ST301.plcproj`). Backslashes must be normalised. [VERIFIED]
- The xti lookup path is `<tsproj dir>/_Config/PLC/<File>`. [VERIFIED for ST101/ST201/ST301]
- Baader's tsproj `<DataTypes>` holds the TwinCAT system types: `AMSNETID` = 6 x BYTE (48 bits) and `AMSADDR` = `netId : AMSNETID` (bit 0) + `port : WORD` (bit 48). [VERIFIED: Baader tsproj lines 3-57]

### plcproj (MSBuild, default namespace `http://schemas.microsoft.com/developer/msbuild/2003`)
- `<Compile Include="GVLs\ECT.TcGVL">`, `POUs\MAIN.TcPOU`, `POUs\wagon\ET_WagonLocation.TcDUT`, `PlcTask.TcTTO`. Counts: ST301 3 TcDUT, 17 TcGVL, 3 TcPOU, 1 TcTTO; ST101 15/4/1 GVL/POU/TTO; ST201 14/2/1; Baader 3 DUT, 4 GVL, 17 POU, 1 TTO; SVNCoreComponents 36 DUT, 2 GVL, 27 POU. No `.TcIO` (interfaces) and no properties anywhere in the oracle. [VERIFIED]
- Only plcproj-listed files belong to the project: `Baader/gagnasofnun/GVLs/GVL_Testing.TcGVL` exists on disk but is not listed. [VERIFIED]
- Library references:
```xml
<PlaceholderReference Include="Tc2_EtherCAT">
  <DefaultResolution>Tc2_EtherCAT, * (Beckhoff Automation GmbH)</DefaultResolution>
  <Namespace>Tc2_EtherCAT</Namespace>
</PlaceholderReference>
<PlaceholderReference Include="Tc3_Module"> ... <SystemLibrary>true</SystemLibrary></PlaceholderReference>
<PlaceholderResolution Include="Tc2_EtherCAT"><Resolution>Tc2_EtherCAT, * (Beckhoff Automation GmbH)</Resolution></PlaceholderResolution>
```
- References per project [VERIFIED]:

| Project | PlaceholderReference |
|---------|---------------------|
| ST101, ST201, ST301 | SVNCoreComponents, Tc2_EtherCAT, Tc2_ModbusSrv, Tc2_Standard, Tc2_System, Tc3_Module |
| Baader `gagnasofnun` | SVNCoreComponents, Tc2_Standard, Tc2_System, Tc3_IPCDiag, Tc3_Module |
| SVNCoreComponents | Tc2_EtherCAT, Tc2_Standard, Tc2_System |

- No `<LibraryReference Include=...>` items in any oracle plcproj; only the `<LibraryReferences>{guid}</LibraryReferences>` property. [VERIFIED]
- Sibling library layout: `sildarvinnsla/SVNCoreComponents/SVNCoreComponents/SVNCoreComponents.plcproj` beside `sildarvinnsla/ST301/`. A duplicate copy sits under `sildarvinnsla/.claude/worktrees/track-stations/` and must never be picked up. [VERIFIED]

### TcPOU / TcGVL / TcDUT / TcTTO
- Root `<TcPlcObject Version="1.1.0.1" ProductVersion=...>` with one child: `<POU Name Id SpecialFunc>`, `<GVL Name Id [ParameterList="True"]>`, `<DUT Name Id>`, `<Task Name Id>` (TcTTO), `<Itf Name Id>` (TcIO, not in oracle). [VERIFIED]
- Many files start with a UTF-8 BOM; `encoding/xml` decoded them without error in a test run. Files are LF on the macOS checkout; `.gitattributes` has `* text=auto`, so Windows checkouts are CRLF. [VERIFIED]
- POU children, in file order: `<Declaration><![CDATA[...]]></Declaration>`, `<Implementation><ST><![CDATA[...]]></ST></Implementation>`, then interleaved `<Action Name Id><Implementation><ST>`, `<Method Name Id><Declaration>METHOD ...</Declaration><Implementation><ST>`. `FB_ATV320.TcPOU` interleaves Action, Action, Method, Method, Action. [VERIFIED lines 197-977]
- Declarations carry no `END_FUNCTION_BLOCK`/`END_PROGRAM`/`END_FUNCTION`; method declarations carry no `END_METHOD`; actions have no header. [VERIFIED]
- In every oracle file, a CDATA segment never shares a line with the next one; there is always at least one tag-only line between segments. All implementations are `<ST>`; no FBD/LD/SFC/CFC. [VERIFIED by grep over all five projects]
- Property shape (from TwinCAT; no oracle instance): `<Property Name Id><Declaration>PROPERTY PUBLIC P : INT</Declaration><Get Name="Get" Id><Declaration>VAR END_VAR</Declaration><Implementation><ST>...</ST></Implementation></Get><Set ...>...</Set></Property>`. [ASSUMED: element names follow the Method pattern; fixture must be hand-written]
- TcGVL `Name=` attribute is the GVL name (`ECT_Diag`, `EcDiagParam`, `BatchLineParameterList`). [VERIFIED]
- TcTTO: `<Task Name="PlcTask"><CycleTime>1000</CycleTime>` (comment says microseconds) `<Priority>20</Priority><PouCall><Name>MAIN</Name></PouCall>`. ST101/ST201/ST301 1000 µs, Baader 20000 µs, all call MAIN. Agrees with the tsproj in all four projects. [VERIFIED]

## Measured baseline (current branch, throwaway converter in the scratchpad, nothing written to the repo)

Method: for each plcproj `Compile` item of kind TcPOU/TcGVL/TcDUT, concatenate declaration, implementation, methods (`END_METHOD` added), actions (`ACTION n:` ... `END_ACTION`) and the closing keyword; `parser.Parse(tcpouPath, text)`; `ast.SetGVLName(file, Name)` for GVLs; `analyzer.Analyze(user, nil, AnalyzeOpts{LibraryFiles: libs})`. Errors only.

| Run | Errors |
|-----|--------|
| st301.st flattened (Phase 20 oracle) | 1782 |
| ST301 plcproj, own files only | 262 (235 SEMA037) |
| ST301 + SVNCoreComponents as library | 42 |
| ST301 + SVNCoreComponents + draft Tc2 stubs | 42 (all Tc2 SEMA037 gone; FB_MBReadWriteRegs now exposes 10 Phase 22 literal SEMA021) |
| SVNCoreComponents alone, no stubs | 131 |
| SVNCoreComponents + draft stubs | 130 |
| ST101 + SVN + stubs | 51 |
| ST201 + SVN + stubs | 43 |
| Baader + SVN + stubs | 273 |

The whole ST301 + SVN + stubs analysis runs in well under a second.

### ST301 + SVNCoreComponents + stubs, 42 errors

| Count | Code | Message | Owner |
|-------|------|---------|-------|
| 5 | SEMA033 | GVL 'SPB03' is qualified_only; use SPB03.gate | Phase 21: SPB03 uses `{attribute "qualified_only"}` with double quotes (see Open Question 3) |
| 4 | SEMA010 | undeclared identifier "ADR" | Phase 22 built-ins |
| 4 | SEMA010 | undeclared identifier "SIZEOF" | Phase 22 built-ins |
| 1 | SEMA010 | undeclared identifier "UINT_TO_WORD" | Phase 22 conversions |
| 16 | SEMA021 | cannot pass DINT as input `nUnitID/nReadQuantity/nMBReadAddr/...` (expected BYTE/WORD), DINT as `PT` (expected REAL), LREAL as `i_rAutoSPOverRide` (expected REAL) | Phase 22 literal typing |
| 9 | SEMA001 | cannot assign LREAL to REAL/UINT/UDINT, DINT to REAL/INT | Phase 22 literal typing |
| 1 | SEMA037 | undeclared type 'FB_TwoWayConveyor' (SPB03.TcGVL) | Genuine: renamed to FB_BatchConveyor in SVNCore commit 549e3a9 |
| 2 | SEMA024 | type ST_LineRecipe has no member "stopDistanceFromEnd" (MAIN) | Genuine: field moved out of ST_LineRecipe at 549e3a9 |

Projected after Phase 21 with the three fixes below: 37 errors, 34 Phase 22 + 3 genuine.

### SVNCoreComponents checked as its own project + stubs, 130 errors

| Count | Code | Message | Owner |
|-------|------|---------|-------|
| 18 | SEMA021 | cannot pass WORD as `nSlaveAddr` (expected UINT) (`amsaddr.port` is WORD) | Phase 22 implicit conversion |
| 30 | SEMA021 | DINT literal as `nIndexOffset`/`nIndex` (BYTE/WORD) | Phase 22 literal typing |
| 2 | SEMA021 | DINT literal as `reqState` (WORD) | Phase 22 |
| 47 | SEMA001 | LREAL→REAL/UINT/UDINT/USINT, LREAL array index, DINT→BYTE/INT, WORD↔UINT, AND/OR on integers | Phase 22 |
| 9 | SEMA003 | cannot compare BYTE and DINT | Phase 22 literal typing |
| 16 | SEMA010 | ADR 6, SIZEOF 5, WORD_TO_UINT 2, UINT_TO_WORD, BOOL_TO_UINT, SHL | Phase 22 built-ins |
| 5 | SEMA022 | "fbSetState"/"fbCoERead"/"fbCoEWrite" is not callable (type FB_...) | Phase 21: empty-call checker bug (Pitfall 3) |
| 1 | SEMA037 | undeclared type 'ST_Batch' (FB_Conveyor.TcPOU, also FB_Reposition) | Genuine: ST_Batch is declared nowhere at 549e3a9 |
| 2 | SEMA024 | ST_LineRecipe has no member "stopDistanceFromEnd" / "drivePastForDelivery" (FB_Conveyor) | Genuine |

### ST101 / ST201 / Baader

- ST101 (51): 8 SEMA033 double-quoted `SPB01`; 15 SEMA010 (ADR, SIZEOF, SHL, UINT_TO_WORD); 28 Phase 22 literal SEMA001/SEMA021.
- ST201 (43): 10 SEMA033 double-quoted `SPB02`; 1 SEMA037 FB_TwoWayConveyor and 1 SEMA024 stopDistanceFromEnd (genuine drift, same as ST301); 1 P001 + 1 P002 chained assignment at MAIN 245 (Phase 22 per Phase 20 deferred list); rest Phase 22.
- Baader (273): 16 SEMA037 for `MDP5001_600_*`, `Status_FBD35181_Plc`, `Ctrl_903458A3_Plc`, which are TwinCAT IO-generated types defined in per-box `_Config/IO/.../(EL6001).xti` `<DataTypes>` (Phase 24), plus 36 cascading "type Invalid does not support member access"; 43 SEMA023 pointer and STRING indexing, 39 ADR, 24 REAL_TO_UDINT and other conversions, 12 FB_Fifo method/member access (member method calls, Phase 22/23); 1 SEMA022 empty call `Mach01()`. No Tc2 library symbol is unresolved once stubs load.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/xml` | Go stdlib (go1.26.0 local, go.mod 1.25.0) | tsproj/xti/plcproj/TcPOU/TcTTO decoding, `Decoder.InputOffset`/`InputPos` for positions | CLAUDE.md mandates it; no DTD/external-entity processing [VERIFIED: local build] |
| `embed` | Go stdlib | Ship `stdlib/vendor/beckhoff/*.st` inside the binary | Stubs must resolve for an installed `stc` with no repo checkout [ASSUMED: no embed exists today, verified by grep] |
| `path/filepath`, `io/fs` | Go stdlib | Backslash normalisation, sibling glob, embedded FS reads | — |
| Existing `pkg/parser`, `pkg/pipeline`, `pkg/analyzer`, `pkg/checker` | repo HEAD 355eac0 | Parse converted text, register libraries with `isLibrary=true` | Reuse; `AnalyzeOpts.LibraryFiles` already exists |
| `github.com/spf13/cobra`, `github.com/BurntSushi/toml` | already in go.mod | CLI and `--out` stc.toml | Existing dependencies |

### Supporting
None. No new modules.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Line-preserving layout | Explicit line map (like `preprocess.SourceMap`) and post-hoc remapping of every diagnostic | Remapping has to touch parser, checker, usage and LSP diagnostics; layout gets exact positions for free |
| `//go:embed` stubs | Locate `stdlib/vendor/beckhoff` relative to the executable or cwd | Breaks for `go install` binaries and CI temp dirs |
| Struct `xml.Unmarshal` for TcPOU | Token loop | Unmarshal loses per-segment offsets; the layout needs the token loop anyway |

**Installation:** none.

## Package Legitimacy Audit

No external packages are installed in this phase. slopcheck not run: nothing to check.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| (none) | — | — | — | — | — | — |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
 x.tsproj ──► read Tasks (CycleTime/100ns) ──► PLC project: inline <Project PrjFilePath> or File=X.xti ──┐
                                                                                                         ▼
 x.plcproj (direct: warn, 10 ms default) ───────────────────────────────────────────────────────► read plcproj
                                                                                                         │
             ┌────────────────────────── Compile items ──────────────────────────────┬────── PlaceholderReference (+line) ──┐
             ▼                                                                       ▼                                    ▼
   TcPOU/TcGVL/TcDUT/TcIO ─► line-preserving converter ─► ST text (path = TcPOU)   TcTTO ─► task PouCall, µs cycle   library resolver
   unknown ext / non-ST / .splcproj ─► VEND warning                                                                        │
                                                                                     (1) own project names (skip lib)     │
                                                                                     (2) sibling X/X.plcproj, X/*/X.plcproj, */X.plcproj,
                                                                                         walking parent dirs; [build.library_paths]
                                                                                     (3) embedded stub + dependency closure
                                                                                     (4) VEND020 at plcproj line
                                                                                                         │
                                                       Model{Name, AmsPort, Tasks, Sources, Libraries, LibrarySources}
                                                                                                         │
             ┌────────────────────────────┬──────────────────────────────┬───────────────────────────────┤
             ▼                            ▼                              ▼                               ▼
   stc vendor import (summary,    stc check <proj>:              stc test --project:            stc sim <proj>:
   --format json, --out dir +     parse sources, Analyze(user,   RunOpts.ProjectFiles +          program from TcTTO PouCall,
   stc.toml)                      LibraryFiles = sibling then    LibraryFiles                    --dt default = task cycle
                                  stubs) ─► diags at TcPOU:line
```

### Recommended Project Structure
```
pkg/vendor/twincat/
├── model.go        # Model, Task, Source, LibraryRef, Kind enums, JSON tags
├── tsproj.go       # tsproj + xti + inline PLC project, Tasks
├── plcproj.go      # Compile items, PlaceholderReference with positions, TcTTO
├── convert.go      # TcPOU/TcGVL/TcDUT/TcIO token loop → segments → layout / compact renderers
├── libs.go         # sibling search, library_paths, embedded stubs + dependency closure
├── import.go       # Import(path) orchestration, diagnostics (VEND02x)
└── testdata/       # synthetic tsproj/xti/plcproj/TcPOU fixtures (no customer code), incl. CRLF + BOM copies
pkg/vendor/extract.go   # thin wrapper over twincat converter in declaration-only mode
stdlib/vendor/beckhoff/
├── embed.go        # package beckhoff; //go:embed *.st; var FS embed.FS; Deps map
├── tc2_ethercat.st tc2_modbussrv.st tc2_serialcom.st tc3_module.st tc3_ipcdiag.st (new)
└── tc2_system.st tc2_utilities.st common_types.st (extended)
cmd/stc/vendor_cmd.go   # add `import`
cmd/stc/check.go, test_cmd.go, sim_cmd.go   # project-path mode
tests/twincat_import_test.go   # STC_SILD_DIR env-gated oracle
```
Import direction: `pkg/vendor` → `pkg/vendor/twincat` → {`pkg/parser`, `pkg/pipeline`, `pkg/ast`, `pkg/diag`, `pkg/source`, `pkg/project`, `stdlib/vendor/beckhoff`}. `twincat` must not import `pkg/vendor` (cycle). `cmd/stc` calls `analyzer.Analyze` with the model's files; an `analyzer.AnalyzeProject` wrapper is optional sugar.

### Pattern 1: Model and JSON
```go
// Source: CONTEXT.md decisions; JSON keys required by CONTEXT
type Task struct {
    Name       string        `json:"name"`
    CycleTime  time.Duration `json:"-"`
    CycleNs    int64         `json:"cycle_time_ns"`
    Priority   int           `json:"priority"`
    Programs   []string      `json:"programs"`   // TcTTO <PouCall><Name>
}
type LibraryRef struct {
    Name, DefaultResolution, Namespace string
    ResolvedFrom string     `json:"resolved_from"` // project|sibling|library_path|stub|builtin|unresolved
    Path         string     `json:"path,omitempty"`
    Pos          source.Pos `json:"pos"`           // plcproj line of <PlaceholderReference>
}
type Model struct {
    PlcName string `json:"plc_name"`; AmsPort int `json:"ams_port"`
    Tasks []Task `json:"tasks"`; Sources []Source `json:"sources"`; Libraries []LibraryRef `json:"libraries"`
    LibrarySources []Source `json:"-"`   // ordered: siblings first, then stubs
}
```
Keep slices ordered by plcproj order (sources) and by reference order (libraries) for deterministic output.

### Pattern 2: Line-preserving layout (prototype verified)
Write every CDATA segment at the XML line and column where its text starts, pad the first line with spaces, and write synthesized keywords on tag-only lines. ST positions then equal TcPOU XML positions, and `parser.Parse(tcpouPath, text)` reports diagnostics at the real file location. The prototype in the scratchpad parsed FB_ATV320, ST301 MAIN, Baader FB_SerialFramer and ST301 FB_Wagon with 0 diagnostics, and `fbSetState();` landed on XML lines 440 and 498, the same as in the XML.

| XML event | Write at XML line of the event |
|-----------|-------------------------------|
| CharData under `Declaration` or `ST` | the text, starting at line/col of `<![CDATA[` + 9 |
| `<Action Name="n">` start | `ACTION n:` |
| `</Action>` | `END_ACTION` |
| `</Method>` | `END_METHOD` |
| `<Get>` / `</Get>` | `GET` / `END_GET` |
| `<Set>` / `</Set>` | `SET` / `END_SET` |
| `</Property>` | `END_PROPERTY` |
| `</POU>` | `END_PROGRAM` / `END_FUNCTION_BLOCK` / `END_FUNCTION` |
| `</Itf>` | `END_INTERFACE` |
| `<Method>` inside `<Itf>` | declaration then `END_METHOD` at `</Method>` (no body) |

```go
// Source: scratchpad prototype layout/main.go (verified 2026-10-06)
off := d.InputOffset()            // before d.Token(): start of the token in the raw bytes
tok, _ := d.Token()
if cd, ok := tok.(xml.CharData); ok && (parent == "Declaration" || parent == "ST") {
    line, col := lineCol(raw, off)   // count '\n' in raw[:off]
    if bytes.HasPrefix(raw[off:], []byte("<![CDATA[")) { col += len("<![CDATA[") }
    lay.put(line, col, string(cd))
}
```
Determine the closing keyword from the POU declaration with the stc lexer: skip pragmas and comments, take the first of PROGRAM / FUNCTION_BLOCK / FUNCTION / INTERFACE. Do not use a regex; `{attribute ...}` lines and `(* ... *)` comments precede the keyword in real files. Fallback when a segment would overlap a previous line (not seen in the oracle): append on the next free line and emit a warning, accepting an offset.

The `--out` renderer writes compact text (no padding) with a `// source: <relative TcPOU path>` header, because those files are read on their own.

### Pattern 3: plcproj with positions
```go
// Source: encoding/xml docs; position = line of the start tag
for {
    tok, err := d.Token()
    if err == io.EOF { break }
    se, ok := tok.(xml.StartElement)
    if !ok { continue }
    line, _ := d.InputPos()  // after the start tag; single-line tags in plcproj
    switch se.Name.Local {
    case "Compile":              var c compileItem; d.DecodeElement(&c, &se); c.Line = line
    case "PlaceholderReference": var p placeholderRef; d.DecodeElement(&p, &se); p.Line = line
    }
}
```
Match on `Name.Local` only; the plcproj default namespace is the MSBuild one.

### Pattern 4: Library resolution order and registration order
1. A reference whose name equals the project itself is ignored.
2. Sibling: for each ancestor directory D of the plcproj, starting at the solution directory and walking up at most 3 levels, try `D/X/X.plcproj`, `D/X/*/X.plcproj` (sorted), `D/*/X.plcproj` (sorted). Never descend into dot-directories. First hit wins; more than one hit at the same level is a warning naming both.
3. `[build.library_paths]` from an `stc.toml` found by `project.FindConfig(plcprojDir)`, keyed by library name case-insensitively.
4. Embedded stub `strings.ToLower(X) + ".st"` plus its dependency closure; `Tc2_Standard` maps to `builtin` (standard FBs already in the checker).
5. Otherwise VEND020 (warning) at the `<PlaceholderReference>` line.

Sibling libraries are converted with the same converter and appended to `LibraryFiles` before stub files, so a type declared by both (none found today) resolves to the real library under first-library-wins. Build `LibraryFiles` as an ordered slice; never iterate a map. Sibling libraries resolve their own PlaceholderReferences recursively with a visited set (SVNCoreComponents → Tc2_EtherCAT, Tc2_Standard, Tc2_System).

Stub dependency closure (Go map in `embed.go`):

| Stub | Files | Depends on |
|------|-------|-----------|
| tc2_system | common_types.st, tc2_system.st | — |
| tc2_utilities | tc2_utilities.st | tc2_system |
| tc2_ethercat | tc2_ethercat.st | tc2_system, tc2_utilities |
| tc2_modbussrv | tc2_modbussrv.st | tc2_system |
| tc2_serialcom | tc2_serialcom.st | tc2_system |
| tc2_mc2 | tc2_mc2.st | tc2_system |
| tc3_module, tc3_ipcdiag, tc3_eventlogger | own file | tc2_system |
| tc2_standard | (builtin) | — |

Evidence for the Tc2_EtherCAT → Tc2_Utilities edge: the Tc2_EtherCAT manual declares `TYPE T_DCTIME : T_ULARGE_INTEGER;` and `EC_DCTIME_DELTA_OFFSET : T_ULARGE_INTEGER`, and `T_ULARGE_INTEGER` is a Tc2_Utilities type. SVNCoreComponents uses `FB_LocalSystemTime` and `SYSTEMTIME_TO_DT` (Tc2_Utilities) while referencing only Tc2_EtherCAT, Tc2_Standard and Tc2_System, and it compiles in TwinCAT. [VERIFIED: manual lines 8470, 8628; Tc2_Utilities line 23243] [ASSUMED: TwinCAT exposes transitively referenced libraries; inferred from production code]

### Pattern 5: CLI project mode
- `stc check <path>`: if exactly one argument ends in `.tsproj` or `.plcproj` (case-insensitive), import it and analyse; reject mixing project and `.st` arguments and reject `--gvl-name` with a project. JSON stays the bare diagnostic array.
- Parse imported sources with `pipeline.Parse(path, text, defines)` directly, or add `IncrementalAnalyzer.ParseSources([]Source)` keyed by the TcPOU path and the hash of the converted text. The existing `ia.Parse(filenames)` reads raw files from disk and would parse XML.
- `stc test --project <x>`: add `RunOpts.ProjectFiles []*ast.SourceFile`. Today `buildExternalContext` only takes TYPE, FUNCTION and FUNCTION_BLOCK from library files, ignores GVLs and PROGRAMs, and flags every library FB as auto-stubbed even when it has a body. Project files must merge like the test file's own declarations, without auto-stub warnings.
- `stc sim <x>`: choose the PROGRAM named by the first task's `PouCall` and default `--dt` to the task cycle time unless `--dt` was given; collect GVLs from every source. Full multi-task scheduling stays in Phase 23.
- `stc vendor import <x> [--out dir]`: text summary and JSON (`plc_name`, `ams_port`, `tasks[].cycle_time_ns`, `libraries[].resolved_from`, `sources[]`). `--out` writes compact `.st` per object under the same relative folders, converted sibling libraries and used stubs under `<out>/libs/<Name>/`, and an `stc.toml` whose `library_paths` point there.

### Anti-Patterns to Avoid
- Concatenating all objects into one file (the Phase 20 flattener). It merged 17 GVLs into one and produced 24 false SEMA033.
- Deriving the GVL name from the file basename. Use the `Name=` attribute with `ast.SetGVLName` after parsing, never inside a cached parse.
- Loading every `.TcPOU` under the folder. Only plcproj-listed items count.
- Iterating `cfg.Build.LibraryPaths` (a map) to build `LibraryFiles`. `vendor.LoadLibraries` does this today, which makes first-library-wins nondeterministic; sort keys.
- Resolving stubs from the filesystem relative to cwd.

## Stub signatures (copy into the `.st` files)

All parameter names and types are from the Beckhoff manuals in `/Users/jonb/Projects/beckhoff-docs/pdf-text/` unless tagged. Stubs keep `END_FUNCTION_BLOCK` / `END_FUNCTION` and empty bodies.

### tc2_ethercat.st (TwinCAT_3_PLC_Lib_Tc2_EtherCAT_EN.txt, v1.10.0) [CITED]
```
TYPE E_EcAdressingType : (eAdressingType_AutoInc := 1, eAdressingType_Fixed, eAdressingType_Broadcast); END_TYPE
TYPE ST_EcSlaveState : STRUCT deviceState : BYTE; linkState : BYTE; END_STRUCT END_TYPE
TYPE ST_EcCrcError   : STRUCT portA : UDINT; portB : UDINT; portC : UDINT; END_STRUCT END_TYPE
TYPE ST_EcCrcErrorEx : STRUCT portA : UDINT; portB : UDINT; portC : UDINT; portD : UDINT; END_STRUCT END_TYPE

VAR_GLOBAL CONSTANT   (* §14.1, subset *)
    EC_AMSPORT_MASTER : UINT := 16#FFFF;   EC_MAX_SLAVES : UINT := 16#FFFF;
    EC_DEVICE_STATE_MASK : BYTE := 16#0F;  EC_DEVICE_STATE_INIT : BYTE := 16#01;  EC_DEVICE_STATE_PREOP : BYTE := 16#02;
    EC_DEVICE_STATE_BOOTSTRAP : BYTE := 16#03;  EC_DEVICE_STATE_SAFEOP : BYTE := 16#04;  EC_DEVICE_STATE_OP : BYTE := 16#08;
    EC_DEVICE_STATE_ERROR : BYTE := 16#10;  EC_DEVICE_STATE_INVALID_VPRS : BYTE := 16#20;  EC_DEVICE_STATE_INITCMD_ERROR : BYTE := 16#40;
    EC_LINK_STATE_OK : BYTE := 16#00;  EC_LINK_STATE_NOT_PRESENT : BYTE := 16#01;  EC_LINK_STATE_LINK_WITHOUT_COMM : BYTE := 16#02;
    EC_LINK_STATE_MISSING_LINK : BYTE := 16#04;  EC_LINK_STATE_ADDITIONAL_LINK : BYTE := 16#08;
    EC_LINK_STATE_PORT_A : BYTE := 16#10;  EC_LINK_STATE_PORT_B : BYTE := 16#20;  EC_LINK_STATE_PORT_C : BYTE := 16#40;  EC_LINK_STATE_PORT_D : BYTE := 16#80;
END_VAR
```
Common FB shape: `VAR_INPUT sNetId : T_AmsNetId; ... bExecute : BOOL; tTimeout : TIME := DEFAULT_ADS_TIMEOUT; END_VAR VAR_OUTPUT bBusy : BOOL; bError : BOOL; nErrId : UDINT; ... END_VAR`.

| FB | Extra VAR_INPUT (in order) | Extra VAR_OUTPUT |
|----|---------------------------|------------------|
| FB_EcPhysicalWriteCmd (§3.2) | `sNetId; adp : UINT; ado : UINT; len : UDINT; eType : E_EcAdressingType := eAdressingType_Fixed; pSrcBuf : PVOID; bExecute; tTimeout` | `wkc : UINT` |
| FB_EcGetAllSlaveAddr (§4.3) | `pAddrBuf : POINTER TO ARRAY[0..EC_MAX_SLAVES] OF UINT; cbBufLen : UDINT` | `nSlaves : UINT` |
| FB_EcGetAllSlaveCrcErrors (§4.4) | `pCrcErrorBuf : POINTER TO ARRAY[0..EC_MAX_SLAVES] OF DWORD; cbBufLen : UDINT` | `nSlaves : UINT` |
| FB_EcGetSlaveCrcError (§4.12) | `nSlaveAddr : UINT` | `crcError : ST_EcCrcError` |
| FB_EcGetSlaveCrcErrorEx (§4.13) | `nSlaveAddr : UINT`; note `tTimeout : TIME` has no default in the manual | `CrcError : ST_EcCrcErrorEx` |
| FB_EcGetAllSlaveStates (§5.1) | `pStateBuf : POINTER TO ARRAY[0..EC_MAX_SLAVES] OF ST_EcSlaveState; cbBufLen : UDINT` | `nSlaves : UINT` |
| FB_EcGetMasterState (§5.3) | — | `state : WORD` |
| FB_EcGetSlaveState (§5.4) | `nSlaveAddr : UINT` | `state : ST_EcSlaveState` |
| FB_EcSetSlaveState (§5.9) | `nSlaveAddr : UINT; bExecute; tTimeout : TIME := T#10S; reqState : WORD` (reqState after tTimeout) | `currState : ST_EcSlaveState` |
| FB_EcCoeSdoRead (§7.1) | `nSlaveAddr : UINT; nSubIndex : BYTE; nIndex : WORD; pDstBuf : PVOID; cbBufLen : UDINT` | `cbRead : UDINT` |
| FB_EcCoeSdoWrite (§7.3) | `nSlaveAddr : UINT; nSubIndex : BYTE; nIndex : WORD; pSrcBuf : PVOID; cbBufLen : UDINT` | — |

Name spelling: Beckhoff writes `FB_EcCoeSdoRead` in headings and `FB_EcCoESdoRead` in examples; sildarvinnsla writes `FB_EcCoESDoRead`. stc names are case-insensitive, so any spelling resolves. Use `FB_EcCoESdoRead`.

**Not a Beckhoff type:** `E_EcSlaveState` is declared by `SVNCoreComponents/ECT/Diag/E_EcSlaveState.TcDUT` as `(Unknown := 0, Init := 1, PreOp := 2, Boot := 3, SafeOp := 4, Op := 8) USINT`. The Tc2_EtherCAT data-types chapter (§13.1-13.22) has no such type. `EcDiagParam` is an SVNCoreComponents GVL (`MAX_EC_SLAVES : UINT := 128`) and `FB_EcDeviceDiag` is an SVNCoreComponents FB. [VERIFIED]

### tc2_system.st additions (TwinCAT_3_PLC_Lib_Tc2_System_EN.txt, v1.17.3) [CITED unless tagged]
```
TYPE T_AmsNetIdArr : ARRAY[0..5] OF BYTE; END_TYPE          (* §5, line 7327: BYTE, not USINT *)
TYPE AMSNETID : ARRAY[0..5] OF BYTE; END_TYPE                (* [VERIFIED: Baader tsproj DataTypes, 6 x BYTE] *)
TYPE AMSADDR : STRUCT netId : AMSNETID; port : WORD; END_STRUCT END_TYPE   (* [VERIFIED: Baader tsproj, port is WORD] *)
TYPE ST_AmsAddr : STRUCT netId : T_AmsNetIdArr; port : T_AmsPort; END_STRUCT END_TYPE   (* §5.11 *)
VAR_GLOBAL CONSTANT
    DEFAULT_ADS_TIMEOUT : TIME := T#5S;     (* value [ASSUMED]; name cited, value not in the manual *)
    MAX_STRING_LENGTH   : UDINT := 255;     (* §5.20 *)
END_VAR
FUNCTION F_CreateAmsNetId : T_AmsNetId  VAR_INPUT nIds : T_AmsNetIdArr; END_VAR  END_FUNCTION   (* §4.2.4 *)
FUNCTION MEMCPY  : UDINT VAR_INPUT destAddr : PVOID; srcAddr : PVOID; n : UDINT; END_VAR END_FUNCTION
FUNCTION MEMMOVE : UDINT VAR_INPUT destAddr : PVOID; srcAddr : PVOID; n : UDINT; END_VAR END_FUNCTION
FUNCTION MEMSET  : UDINT VAR_INPUT destAddr : PVOID; fillByte : USINT; n : UDINT; END_VAR END_FUNCTION
FUNCTION MEMCMP  : DINT  VAR_INPUT pBuf1 : PVOID; pBuf2 : PVOID; n : UDINT; END_VAR END_FUNCTION
ADSREAD / ADSWRITE: DESTADDR / SRCADDR : PVOID; TMOUT : TIME := DEFAULT_ADS_TIMEOUT   (* §ADSREAD line 2057, ADSWRITE line 2218 *)
```
- `T_AmsNetId` (`STRING(23)`), `T_AmsPort` (`UINT`) and `T_MaxString` already live in `common_types.st`. `AMSPORT : UINT` can be added for completeness; `AMSADDR.port` must stay WORD.
- `MEMCPY`/`MEMSET`/`MEMMOVE` in today's `tc2_system.st` take UDINT addresses and `MEMSET.fillByte` is BYTE; switch to PVOID/USINT. `stdlib/mocks/beckhoff/ads_mock.st` declares `DESTADDR : UDINT` and must change with the stub, because mocks override stubs.

### tc2_utilities.st additions (TwinCAT_3_PLC_Lib_Tc2_Utilities_EN.txt, v2.18.2) [CITED]
```
TYPE TIMESTRUCT : STRUCT wYear : WORD; wMonth : WORD; wDayOfWeek : WORD; wDay : WORD; wHour : WORD; wMinute : WORD; wSecond : WORD; wMilliseconds : WORD; END_STRUCT END_TYPE
TYPE E_TimeZoneID : (eTimeZoneID_Invalid := -1, eTimeZoneID_Unknown := 0, eTimeZoneID_Standard := 1, eTimeZoneID_Daylight := 2); END_TYPE
FUNCTION_BLOCK FB_LocalSystemTime   (* §3.47 *)
VAR_INPUT  sNetID : T_AmsNetID := ''; bEnable : BOOL; dwCycle : DWORD(1..86400) := 5; dwOpt : DWORD := 1; tTimeout : TIME := DEFAULT_ADS_TIMEOUT; END_VAR
VAR_OUTPUT bValid : BOOL; systemTime : TIMESTRUCT; tzID : E_TimeZoneID := eTimeZoneID_Invalid; END_VAR
END_FUNCTION_BLOCK
FUNCTION SYSTEMTIME_TO_DT : DT  VAR_INPUT TIMESTR : TIMESTRUCT; END_VAR  END_FUNCTION   (* §4.1.18 *)
```
`RTC` (Tc2_Utilities) appears in sildarvinnsla only inside strings and comments; ship it only if the planner wants completeness. The negative enum value and `DWORD(1..86400)` subrange both parse and check clean today (tested in the scratchpad). `SYSTEMTIME_TO_DT` is listed as a Phase 22 missing built-in in the Phase 20 deferred list; shipping it in the Tc2_Utilities stub resolves the svncore SEMA010 for it here.

### tc2_modbussrv.st (TF6250_TC3_Modbus_TCP_EN.txt §6.1.2) [CITED]
The TF6250 Modbus TCP client FBs live in the PLC library named **Tc2_ModbusSrv** (§6.1: "The defined modbus functions are implemented in the PLC library Tc2_ModbusSrv"), which is the name ST101/ST201/ST301 reference. Ship them as `tc2_modbussrv.st`.
```
VAR_GLOBAL CONSTANT MODBUS_TCP_PORT : UINT := 502; END_VAR     (* value [ASSUMED]: Modbus TCP standard port *)
Common inputs: sIPAddr : STRING(15); nTCPPort : UINT := MODBUS_TCP_PORT; nUnitID : BYTE := 16#FF; ... bExecute : BOOL; tTimeout : TIME;
Common outputs: bBUSY : BOOL; bError : BOOL; nErrId : UDINT;
```
| FB | Inputs between nUnitID and bExecute | Extra output |
|----|-------------------------------------|--------------|
| FB_MBReadCoils (fn 1), FB_MBReadInputs (fn 2), FB_MBReadRegs (fn 3), FB_MBReadInputRegs (fn 4) | `nQuantity : WORD; nMBAddr : WORD; cbLength : UDINT; pDestAddr : POINTER TO BYTE` | `cbRead : UDINT` |
| FB_MBWriteSingleCoil (fn 5), FB_MBWriteSingleReg (fn 6) | `nMBAddr : WORD; nValue : WORD` | — |
| FB_MBWriteCoils (fn 15), FB_MBWriteRegs (fn 16) | `nQuantity : WORD; nMBAddr : WORD; cbLength : UDINT; pSrcAddr : POINTER TO BYTE` | — |
| FB_MBReadWriteRegs (fn 23) | `nReadQuantity : WORD; nMBReadAddr : WORD; nWriteQuantity : WORD; nMBWriteAddr : WORD; cbDestLength : UDINT; pDestAddr : POINTER TO BYTE; cbSrcLength : UDINT; pSrcAddr : POINTER TO BYTE` | `cbRead : UDINT` |

### tc2_serialcom.st (TF6340_TC3_Serial_Communication_EN.txt §5) [CITED]
```
TYPE ComBuffer : STRUCT Buffer : ARRAY[0..300] OF BYTE; RdIdx : INT; WrIdx : INT; Count : INT; FreeByte : INT; Error : INT; blocked : BOOL; END_STRUCT END_TYPE
TYPE ComSerialLineMode_t : (SERIALLINEMODE_DEFAULT, SERIALLINEMODE_KL6_3B_ALTERNATIVE, SERIALLINEMODE_KL6_5B_STANDARD, SERIALLINEMODE_KL6_22B_STANDARD, SERIALLINEMODE_PC_COM_PORT, SERIALLINEMODE_EL6_22B, SERIALLINEMODE_IE6_11B); END_TYPE
TYPE ComError_t : (COMERROR_NOERROR := 0, COMERROR_PARAMETERCHANGED := 1, COMERROR_TXBUFFOVERRUN := 2, COMERROR_STRINGOVERRUN := 10, COMERROR_ZEROCHARINVALID := 11, COMERROR_INVALIDPOINTER := 20, COMERROR_INVALIDRXPOINTER := 21, COMERROR_INVALIDRXLENGTH := 22, COMERROR_DATASIZEOVERRUN := 23, COMERROR_INVALIDPROCESSDATASIZE := 24, COMERROR_MODENOTSUPPORTED := 16#0101, COMERROR_INVALIDCHANNELNUMBER := 16#0102, COMERROR_INVALIDBAUDRATE := 16#1001, COMERROR_INVALIDNUMDATABITS := 16#1002, COMERROR_INVALIDNUMSTOPBITS := 16#1003, COMERROR_INVALIDPARITY := 16#1004, COMERROR_INVALIDHANDSHAKE := 16#1005, COMERROR_INVALIDNUMREGISTERS := 16#1006, COMERROR_INVALIDREGISTER := 16#1007, COMERROR_TIMEOUT := 16#1008); END_TYPE
FUNCTION_BLOCK SerialLineControl  VAR_INPUT Mode : ComSerialLineMode_t; pComIn : POINTER TO BYTE; pComOut : POINTER TO BYTE; SizeComIn : INT; END_VAR
    VAR_IN_OUT TxBuffer : ComBuffer; RxBuffer : ComBuffer; END_VAR  VAR_OUTPUT Error : BOOL; ErrorID : ComError_t; END_VAR
FUNCTION_BLOCK SendByte      VAR_INPUT SendByte : BYTE; END_VAR   VAR_IN_OUT TxBuffer : ComBuffer; END_VAR  VAR_OUTPUT Busy : BOOL; Error : ComError_t; END_VAR
FUNCTION_BLOCK SendString    VAR_INPUT SendString : STRING; END_VAR VAR_IN_OUT TxBuffer : ComBuffer; END_VAR VAR_OUTPUT Busy : BOOL; Error : ComError_t; END_VAR
FUNCTION_BLOCK ReceiveByte   VAR_IN_OUT RXBuffer : ComBuffer; END_VAR  VAR_OUTPUT ByteReceived : BOOL; ReceivedByte : BYTE; Error : ComError_t; END_VAR
FUNCTION_BLOCK ReceiveString VAR_INPUT Prefix : STRING; Suffix : STRING; Timeout : TIME; Reset : BOOL; END_VAR
    VAR_IN_OUT ReceivedString : STRING; RXBuffer : ComBuffer; END_VAR  VAR_OUTPUT StringReceived : BOOL; busy : BOOL; Error : ComError_t; RxTimeout : BOOL; END_VAR
FUNCTION_BLOCK ClearComBuffer VAR_IN_OUT Buffer : ComBuffer; END_VAR
```
CONTEXT's `ST_ComBuffer` and `E_ComMode` do not exist in TF6340; the real names are `ComBuffer`, `ComSerialLineMode_t` and `ComError_t`. An FB input named like its FB (`SendString.SendString`, `SendByte.SendByte`) checks clean in stc (scratchpad test). No sildarvinnsla project references Tc2_SerialCom; Baader's `framer.SendString(...)` is a METHOD of its own `FB_SerialFramer`. The stub is exercised by a committed fixture only.

### tc3_module.st, tc3_ipcdiag.st
Header comment plus one harmless declaration so `TestBeckhoffStubsParse` (which requires at least one declaration) passes, for example `TYPE T_Tc3ModuleStubMarker : BOOL; END_TYPE` [ASSUMED: name is arbitrary]. No sildarvinnsla code uses either library's symbols. [VERIFIED by grep]

### common_types.st (locked decision)
`TYPE PVOID : POINTER TO BYTE; END_TYPE` and `TYPE __XWORD : LWORD; END_TYPE` parse and resolve today (scratchpad test). The TwinCAT PLC manual §16.5.11 defines PVOID as UXINT (ULINT on 64-bit, UDINT on 32-bit) and XWORD/__XWORD as LWORD/DWORD, XINT as LINT/DINT. With PVOID as `POINTER TO BYTE`, stc rejects `POINTER TO INT` and `UDINT` arguments ("cannot pass POINTER TO INT as input parameter p (expected POINTER TO BYTE)"). This does not bite in Phase 21 because `ADR` is still undeclared (Invalid type, no SEMA021), but Phase 22 must make any pointer and the `ADR` result assignable to PVOID. Record that in deferred-items. Also add `UXINT`, `__UXINT`, `XINT`, `__XINT`, `XWORD` aliases if the planner wants parity; no oracle file uses them directly. [CITED: TwinCAT_3_PLC_EN.txt line 44870]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| XML parsing, CDATA, entities, BOM, CRLF | String slicing on `<![CDATA[` | `encoding/xml` Decoder (`Token`, `InputOffset`, `InputPos`, `DecodeElement`) | Handles entities, BOM and line-end normalisation |
| Diagnostic position remapping | A second map applied to every diagnostic producer | Line-preserving layout | Exact positions, no changes in checker/usage/LSP |
| Shipping stub files | Runtime lookup of `stdlib/` next to the binary | `//go:embed` in `stdlib/vendor/beckhoff/embed.go` | Works for installed binaries and CI |
| Finding the POU kind | Regex over declaration text | `pkg/lexer` tokens, skipping pragmas and comments | Attributes and comments precede the keyword |
| Library declaration precedence | New precedence logic | Existing first-library-wins in `pkg/checker/resolve.go` with an ordered `LibraryFiles` slice | Already tested in Phase 13/19/20 |

**Key insight:** The checker already does what TwinCAT library resolution needs. The new work is ordering inputs correctly and keeping positions honest.

## Runtime State Inventory

Not a rename or migration phase. One state item matters: `.stc-cache` index entries from `stc check` are keyed by file path and content hash. If imported sources go through the incremental cache, the key must be the TcPOU path with the hash of the converted text, not of the raw XML; otherwise a converter change would serve stale ASTs. Stored data, live service config, OS-registered state, secrets: none (verified: stc keeps no other state).

## Common Pitfalls

### Pitfall 1: "Zero errors" is not reachable on sildarvinnsla HEAD
**What goes wrong:** The gate demands 0 errors and the plan stalls or starts hiding errors.
**Why it happens:** HEAD 549e3a9 renamed `FB_TwoWayConveyor` and removed `ST_LineRecipe.stopDistanceFromEnd`/`drivePastForDelivery` and `ST_Batch` in SVNCoreComponents without migrating ST201/ST301 (the remote has a branch `fix/migrate-st201-st301-batchconveyor`). Phase 22 owns ADR/SIZEOF/conversions and literal typing.
**How to avoid:** Gate on explicit buckets: Phase 21-owned classes (SEMA037/SEMA010 for library symbols, SEMA033 from GVL merging or quoting, SEMA022 empty calls, parse errors) must be 0; a named Phase 22 allowlist (message templates); a named genuine list (FB_TwoWayConveyor, stopDistanceFromEnd, drivePastForDelivery, ST_Batch). Report all three in the SUMMARY.
**Warning signs:** Any SEMA037 naming a Tc2 or SVNCore symbol, or any SEMA033.

### Pitfall 2: Double-quoted attribute names
**What goes wrong:** 5 (ST301), 8 (ST101) and 10 (ST201) SEMA033 errors for bare access to a GVL marked `{attribute "qualified_only"}`.
**Why it happens:** stc honours the double-quoted form. The TwinCAT PLC manual gives the syntax as `{attribute 'attribute'}` and uses single quotes in all 315 examples; the three production line projects access these GVLs bare and compile in TwinCAT. `ast.Attribute.String()` also re-quotes with single quotes, so `stc fmt` would silently turn an ignored attribute into an active one.
**How to avoid:** Record the quote character in the attribute node, ignore double-quoted attribute names for semantics, warn ("attribute name in double quotes is ignored by TwinCAT; use single quotes"), and keep the original quotes in fmt output. See Open Question 3.

### Pitfall 3: `fb();` reports SEMA022 "not callable"
**What goes wrong:** 5 SVNCore and 1 Baader errors once the FB types resolve (before, they were hidden as "type Invalid").
**Why it happens:** `parseAssignOrCall` parses `a()` at statement head as a `CallExpr` expression statement, not a `CallStmt`; `checkCallExpr` (`pkg/checker/check.go:765-812`) only accepts `*types.FunctionType`. Reproduced with a user FB: `a();` → `"a" is not callable (type FB_A)`. The interpreter runs it correctly (`stc test` passes a two-call counter test).
**How to avoid:** In `checkCallExpr`, accept a symbol whose type is an FB instance and bind arguments the way `checkCallStmt` does (or have the parser emit `CallStmt` for a statement-level call). Add checker tests for `fb();` at top level and inside a CASE arm.

### Pitfall 4: `PROPERTY PUBLIC P : INT` does not parse
**What goes wrong:** "expected identifier, got KwPublic". TwinCAT writes an access modifier in property declarations by default.
**Why it happens:** `parseProperty` (`pkg/parser/decl.go:559`) calls `parseIdent` right after PROPERTY. Methods already skip PUBLIC/PRIVATE/PROTECTED/INTERNAL (`decl.go:315`).
**How to avoid:** Accept the same modifier set after PROPERTY (and ABSTRACT/FINAL). Fixture: a TcPOU with a Property element whose Get and Set both have declarations and bodies.

### Pitfall 5: Library order and map iteration
**What goes wrong:** A type defined by both a sibling library and a stub resolves differently between runs.
**Why it happens:** first-library-wins plus `range cfg.Build.LibraryPaths` (a map) in `vendor.LoadLibraries`.
**How to avoid:** Build `LibraryFiles` as an ordered slice: siblings in reference order, then stubs in closure order, then `library_paths` sorted by key. Fix `LoadLibraries` to sort keys while touching it.

### Pitfall 6: Shipping E_EcSlaveState in the Tc2_EtherCAT stub
**What goes wrong:** A second, different `E_EcSlaveState` (Beckhoff has none) competes with SVNCore's `USINT` enum; whichever registers first wins.
**How to avoid:** Do not declare it in `tc2_ethercat.st`. Satisfy the IMPT-04 wording by asserting in the import test that `E_EcSlaveState` resolves from SVNCoreComponents (`resolved_from: sibling`). See Open Question 1.

### Pitfall 7: Stub-only checks miss stub type errors
**What goes wrong:** Library files are only registered (`CollectDeclarations`), never body- or initialiser-checked, and SEMA037 is suppressed inside them. A typo such as `tTimeout : TIME := DEFAULT_ADS_TIMEOUTT` would go unnoticed.
**How to avoid:** Extend `stdlib/vendor/beckhoff/stubs_test.go` to run `analyzer.Analyze` on each stub's dependency closure as user files and require zero errors (the CONTEXT rule "every stub must parse and check clean").

### Pitfall 8: PVOID typing
See the common_types.st note: stub PVOID as `POINTER TO BYTE` is stricter than TwinCAT. Harmless until Phase 22 types `ADR`.

### Pitfall 9: Paths
`PrjFilePath` uses backslashes and is relative to the xti (file form) or tsproj (inline form). `Compile Include` is relative to the plcproj. Use `filepath.FromSlash(strings.ReplaceAll(p, "\\", "/"))`. Windows CI: test fixtures with CRLF and BOM must decode the same as LF.

### Pitfall 10: TwinSAFE and other compile items
ST101/ST201/ST301 tsproj have `<Safety><Project File="CVS01.xti|Line2.xti|Line3.xti"/>` pointing at `.splcproj`. Skip with an info diagnostic. Unknown `Compile` extensions (`.TcTLO`, `.TcVIS`, `.TcGTLO`, ...) get a warning, never a silent skip. Non-ST implementations get a warning and keep their declaration.

### Pitfall 11: `stc vendor extract` determinism and coverage of kinds
Today it returns `map[string]string` and prints in map order, skips TcGVL/TcDUT, drops methods and omits the END keyword (measured: 27 of 65 objects; `FB_ATV320.st` fails to parse). Return an ordered slice, use the shared converter in declaration mode (methods as `METHOD ... END_METHOD` with VAR blocks, no bodies; properties with empty GET/SET), write GVLs with the `Name=` and DUTs as TYPE text.

## Code Examples

### tsproj + xti reading
```go
// Source: shapes verified in ST301 solution.tsproj, _Config/PLC/ST301.xti, Baader tsproj
type tsproj struct {
    Project struct {
        Tasks []struct {
            Priority  int    `xml:"Priority,attr"`
            CycleTime int64  `xml:"CycleTime,attr"` // 100 ns units
            Name      string `xml:"Name"`
        } `xml:"System>Tasks>Task"`
        Plc []plcProjectRef `xml:"Plc>Project"`
    } `xml:"Project"`
}
type plcProjectRef struct {
    File        string `xml:"File,attr"`        // "ST301.xti" → <tsproj dir>/_Config/PLC/ST301.xti
    Name        string `xml:"Name,attr"`        // inline form
    PrjFilePath string `xml:"PrjFilePath,attr"` // inline: relative to tsproj dir
    AmsPort     int    `xml:"AmsPort,attr"`
}
// xti: <TcSmItem><Project Name PrjFilePath AmsPort/></TcSmItem>; PrjFilePath relative to the xti
cycle := time.Duration(t.CycleTime) * 100 * time.Nanosecond // 10000 → 1ms
```

### Import into analysis
```go
// Source: pkg/analyzer/analyzer.go:44 (existing API)
m, diags, err := twincat.Import(path, twincat.Options{Defines: defines})
user := parseAll(m.Sources)            // pipeline.Parse(src.Path, src.Text, defines); SetGVLName for GVLs
libs := parseAll(m.LibrarySources)     // siblings first, then stubs (ordered)
res := analyzer.Analyze(user, cfg, analyzer.AnalyzeOpts{LibraryFiles: libs})
```

### Env-gated oracle test
```go
// tests/twincat_import_test.go
func TestSildarvinnslaImport(t *testing.T) {
    dir := os.Getenv("STC_SILD_DIR")
    if dir == "" { t.Skip("STC_SILD_DIR not set; sildarvinnsla is local-only") }
    // import ST301 solution.tsproj; assert tasks[0].CycleTime == time.Millisecond, PlcName == "ST301",
    // SVNCoreComponents resolved_from == "sibling", Tc2_EtherCAT == "stub", Tc2_Standard == "builtin";
    // bucket errors by templated message; fail on any Phase-21-owned bucket; log Phase 22 and genuine buckets.
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Flattened single-file oracle (`stc-probes/st301.st`) | Real tsproj import | This phase | Removes 1740 of 1782 ST301 errors |
| Stubs found only through `[build.library_paths]` on disk | Embedded stubs resolved by library name | This phase | `stc check x.tsproj` works with no stc.toml |
| `vendor extract` FB declarations only | Shared converter, all object kinds | This phase | Output parses |

**Deprecated/outdated:** `vendor.ExtractStub`/`ExtractProject` map-returning API; keep thin wrappers for existing tests or update the tests.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | TwinCAT makes symbols of transitively referenced libraries visible (SVNCore uses Tc2_Utilities FBs without referencing it) | Pattern 4 | Without the closure, FB_LocalSystemTime and SYSTEMTIME_TO_DT stay SEMA037/SEMA010 in svncore |
| A2 | TwinCAT ignores `{attribute "..."}` with double quotes | Pitfall 2, OQ3 | If wrong, the three line projects have 23 genuine SEMA033 that TwinCAT would also report, which contradicts them building |
| A3 | `DEFAULT_ADS_TIMEOUT` = T#5S and `MODBUS_TCP_PORT` = 502 | Stub signatures | Only default values; no checker effect |
| A4 | TcPOU Property XML uses `<Property><Declaration/><Get><Declaration/><Implementation><ST/></Implementation></Get><Set>...` | TcPOU formats | Property import untested against real files; no oracle file has a property |
| A5 | TwinSAFE `.splcproj` can be skipped without losing PLC symbols | Pitfall 10 | Safety GVLs are not referenced by grep in PLC code |
| A6 | `//go:embed` is the right delivery for stubs (no existing embed in repo) | Standard Stack | Alternative is filesystem lookup, which fails for installed binaries |
| A7 | Sibling search depth of 3 ancestor levels is enough | Pattern 4 | Deeper layouts need `[build.library_paths]` |

## Open Questions (RESOLVED)

1. **IMPT-04 lists `E_EcSlaveState` under the Tc2_EtherCAT stubs, but it is an SVNCoreComponents type.**
   - What we know: Declared in `SVNCoreComponents/ECT/Diag/E_EcSlaveState.TcDUT`; absent from the Tc2_EtherCAT manual's data types.
   - RESOLVED: Do not ship it in `tc2_ethercat.st`. The import test asserts it resolves with `resolved_from: sibling`, and the SUMMARY states why. Same for `EcDiagParam` and `FB_EcDeviceDiag`.

2. **CONTEXT says `T_AmsNetIdArr`/`AMSNETID` are `ARRAY[0..5] OF USINT` and `AMSADDR.port : AMSPORT (UINT)`.**
   - What we know: The Tc2_System manual says `T_AmsNetIdArr : ARRAY[0..5] OF BYTE`. The TwinCAT type system in the Baader tsproj defines AMSNETID as 6 BYTE and AMSADDR.port as WORD. With USINT, `F_CreateAmsNetId(nIds := amsaddr.netId)` reports "cannot pass ARRAY OF USINT ... (expected ARRAY OF BYTE)".
   - RESOLVED: Use BYTE for both arrays and WORD for `AMSADDR.port`, as Beckhoff declares them (CONTEXT also says "use exactly the Beckhoff declared types"). The resulting 18 svncore "WORD as nSlaveAddr (expected UINT)" SEMA021 are Phase 22 implicit conversion, already bucketed by Phase 20.

3. **Should stc honour `{attribute "qualified_only"}` with double quotes?**
   - What we know: Documented syntax is single quotes only; 23 bare accesses across ST101/ST201/ST301 to such GVLs; the projects compile in TwinCAT. Phase 20 deferred this question.
   - RESOLVED: Treat double-quoted attribute names as unrecognised with a warning (new code in the SEMA range or a lint-style warning), preserve the quote character through fmt, and add a checker test. Mark A2 in the SUMMARY so the user can confirm.

4. **Is the success criterion "zero errors" literal?**
   - RESOLVED: No. Gate on owner buckets (Pitfall 1): Phase 21-owned buckets must be 0; Phase 22 buckets and the four genuine sildarvinnsla drift findings are listed by message template in the test and the SUMMARY.

5. **Where do Baader's IO-generated types (`MDP5001_600_*`, `Status_FBD35181_Plc`, `Ctrl_903458A3_Plc`) come from?**
   - What we know: Per-box `_Config/IO/.../(EL6001).xti` `<DataTypes>`; a few `_I_`/`_O_` variants also appear in the tsproj `<DataTypes>`, but not the names the code uses.
   - RESOLVED: Out of scope (Phase 24 loads xti). The Baader gate allow-lists these SEMA037 and their "type Invalid" cascades as Phase 24.

6. **Tc2_SerialCom is not referenced by any oracle project.**
   - RESOLVED: Ship the stub anyway (IMPT-04) and cover it with a committed fixture plcproj that references it; the Baader check needs no SerialCom symbols.

7. **Should the empty-call SEMA022 and PROPERTY access modifier be fixed here?**
   - RESOLVED: Yes. Both block Phase 21 criteria (zero Phase-21-owned errors; properties in IMPT-01). Each is a small, isolated change with tests in `pkg/checker` and `pkg/parser`.

8. **Should `stc test --project` reuse `RunOpts.LibraryFiles`?**
   - RESOLVED: No. Add `RunOpts.ProjectFiles`, merged like the test file's own declarations including GVLs, with no auto-stub warnings. Library and stub files stay in `LibraryFiles`.

9. **Where do task cycle time and the program list come from when both tsproj and TcTTO exist?**
   - RESOLVED: Cycle time and priority from the tsproj task (system configuration, authoritative at runtime), program list from the TcTTO `PouCall`, matched by task name. If they disagree, warn and keep the tsproj value. With a bare plcproj, use the TcTTO (µs) if present, else 10 ms with a warning.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test | yes | go1.26.0 darwin/arm64 (go.mod 1.25.0) | — |
| sildarvinnsla checkout | env-gated oracle | yes, local only | `/Users/jonb/Projects/sildarvinnsla` at 549e3a9 (untracked SVNCore files present in working tree) | CI skips; synthetic fixtures cover formats |
| Beckhoff manuals mirror | stub signatures | yes | `/Users/jonb/Projects/beckhoff-docs/pdf-text/` (Tc2_EtherCAT 1.10.0, Tc2_System 1.17.3, Tc2_Utilities 2.18.2, TF6250, TF6340, Tc3_Module) | — |
| gsd-sdk | commit | yes | `/opt/homebrew/bin/gsd-sdk` | git |

**Missing dependencies with no fallback:** none.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing`; exec-based CLI tests in `cmd/stc` (`runStc`); `stc test` ST runner |
| Config file | `.testcoverage.yml`, `scripts/coverage-gate.sh`, `.github/workflows/{ci,coverage,st-tests}.yml` |
| Quick run command | `go test ./pkg/vendor/... ./stdlib/vendor/beckhoff/ -count=1` (or the touched package) |
| Full suite command | `go test ./... -count=1 && go run ./cmd/stc test tests/ && bash scripts/coverage-gate.sh` |
| Oracle | `STC_SILD_DIR=/Users/jonb/Projects/sildarvinnsla go test ./tests -run TestSildarvinnsla -count=1 -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| IMPT-01 | tsproj (File= and inline), xti, plcproj, TcTTO read; all Compile kinds; unknown ext warns | unit | `go test ./pkg/vendor/twincat -run 'TestTsproj|TestPlcproj|TestTcTTO' -count=1` | No, Wave 0 |
| IMPT-01 | Converter: decl+impl, methods, actions, properties (GET/SET), Itf, GVL Name, DUT; positions equal XML line/col; BOM and CRLF fixtures | unit | `go test ./pkg/vendor/twincat -run TestConvert -count=1` | No |
| IMPT-01 | `PROPERTY PUBLIC P : INT` parses | unit | `go test ./pkg/parser -run TestPropertyAccessModifier -count=1` | No |
| IMPT-01 | `fb();` and `fb();` in CASE arm check clean; still error for non-FB | unit | `go test ./pkg/checker -run TestEmptyFBCall -count=1` | No |
| IMPT-01 | `stc check fixture.tsproj` reports diagnostics at TcPOU path and XML line; `stc vendor import --format json`; `--out` then `stc check` on output | exec CLI | `go test ./cmd/stc -run 'TestVendorImport|TestCheckProject' -count=1` | No |
| IMPT-02 | sibling found (and dot-dirs ignored), library_paths, stub + closure, builtin Tc2_Standard, VEND020 with plcproj line; ordered LibraryFiles | unit | `go test ./pkg/vendor/twincat -run TestResolve -count=1` | No |
| IMPT-03 | ST301-shaped fixture: cycle 1 ms, priority 20, programs [MAIN], plc_name, ams_port 851; bare plcproj default 10 ms + warning; `sim <proj>` uses task dt | unit + exec | `go test ./pkg/vendor/twincat -run TestTasks`; `go test ./cmd/stc -run TestSimProject` | No |
| IMPT-04 | Every stub parses and its dependency closure checks clean as user code; per-library declaration counts | unit | `go test ./stdlib/vendor/beckhoff -count=1` | Partial (`stubs_test.go` parses only) |
| IMPT-04 | Fixture program using each stubbed FB with Beckhoff parameter names checks clean | unit | `go test ./pkg/vendor/twincat -run TestStubUsage -count=1` | No |
| IMPT-04 | Double-quoted attribute ignored with warning; fmt keeps quotes | unit | `go test ./pkg/checker ./pkg/format -run TestDoubleQuotedAttribute -count=1` | No |
| IMPT-05 | extract on fixture plcproj: ordered output, END keywords, methods/properties as signatures, GVL/DUT included, every file passes `parser.Parse` with 0 errors | unit + exec | `go test ./pkg/vendor -run TestExtract`; `go test ./cmd/stc -run TestVendorExtract` | Partial (edit existing) |
| All | Oracle: ST301, ST101, ST201, Baader, SVNCore bucketed by owner; Phase-21 buckets = 0 | oracle (env-gated) | `STC_SILD_DIR=... go test ./tests -run TestSildarvinnsla -v` | No |

### Sampling Rate
- **Per task commit:** quick run command for the touched package plus `go vet ./...`.
- **Per wave merge:** full suite command.
- **Phase gate:** full suite green, coverage gate passes, oracle run locally with buckets recorded in the SUMMARY.

### Wave 0 Gaps
- [ ] `pkg/vendor/twincat/testdata/` synthetic solution: `Demo solution.tsproj` (File= form), `_Config/PLC/Demo.xti`, `Demo/Demo.plcproj` (GVL, DUT, POU with method/action/property, TcTTO, unknown item, missing library), sibling `DemoLib/DemoLib/DemoLib.plcproj`, inline-form tsproj, CRLF+BOM copies. No customer code.
- [ ] `pkg/vendor/twincat/*_test.go` for tsproj, plcproj, convert, resolve, import.
- [ ] `cmd/stc/vendor_import_test.go`, `check_project_test.go`, sim/test project-mode cases.
- [ ] `tests/twincat_import_test.go` (STC_SILD_DIR).
- [ ] Extend `stdlib/vendor/beckhoff/stubs_test.go` (check-clean per closure, counts for new files); update `stdlib/mocks/beckhoff/ads_mock.st` to PVOID.
- [ ] Add `pkg/vendor/twincat` (and `pkg/vendor`) to `.testcoverage.yml` and the `coverage-gate.sh` awk table at 95%.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | `encoding/xml` (no DTD or external entity expansion); reject unknown root elements with a diagnostic; bound sibling search depth |
| V6 Cryptography | no | — |
| V12 Files and Resources | yes | `--out` writes only below the output directory: clean each relative Include path and reject `..` and absolute paths |

### Known Threat Patterns for the import

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal through `Compile Include="..\..\x"` or `PrjFilePath` when writing `--out` | Tampering | `filepath.Clean`, require the result to stay under the out dir; reading outside the project is allowed (TwinCAT allows it) but writing is not |
| XML entity expansion / XXE | Denial of service / disclosure | Go `encoding/xml` does not fetch external entities or expand custom DTD entities |
| Symlink loops in sibling search | Denial of service | Shallow globs only, depth limit, skip dot-directories |
| Very large files | Denial of service | Acceptable for a local CLI; no network input |

## Sources

### Primary (HIGH confidence)
- `/Users/jonb/Projects/sildarvinnsla` @ 549e3a9: `ST301/ST301 solution.tsproj`, `ST301/_Config/PLC/ST301.xti`, `ST301/ST301/ST301.plcproj`, `PlcTask.TcTTO` (all projects), `SVNCoreComponents/.../FB_ATV320.TcPOU`, `E_EcSlaveState.TcDUT`, `EcDiagParam.TcGVL`, `Baader/Sildarvinnsla Baader.tsproj` (inline PLC, DataTypes), `Baader/_Config/IO/.../(EL6001).xti`.
- `/Users/jonb/Projects/beckhoff-docs/pdf-text/`: `TwinCAT_3_PLC_Lib_Tc2_EtherCAT_EN.txt` (§3.2, 4.3, 4.4, 4.12, 4.13, 5.1, 5.3, 5.4, 5.9, 7.1, 7.3, 13, 14.1), `TwinCAT_3_PLC_Lib_Tc2_System_EN.txt` (MEMCPY/MEMSET/MEMMOVE/MEMCMP, F_CreateAmsNetId §4.2.4, ADSREAD/ADSWRITE, §5.11, T_AmsNetIdArr, T_MaxString), `TwinCAT_3_PLC_Lib_Tc2_Utilities_EN.txt` (§3.47 FB_LocalSystemTime, §4.1.18, TIMESTRUCT, E_TimeZoneID, T_ULARGE_INTEGER), `TF6250_TC3_Modbus_TCP_EN.txt` (§6.1, 6.1.2.1-6.1.2.9), `TF6340_TC3_Serial_Communication_EN.txt` (§5.1.1, 5.1.3, 5.3), `TwinCAT_3_PLC_EN.txt` (§16.5.11 PVOID/XWORD, §16.8.2 attribute syntax).
- stc source at 355eac0: `pkg/vendor/extract.go`, `loader.go`, `pkg/analyzer/analyzer.go`, `pkg/checker/resolve.go` (1114-1244), `pkg/checker/check.go` (765-812), `pkg/parser/decl.go` (204-300, 559-640), `pkg/parser/stmt.go` (110-160), `pkg/incremental/analyzer.go`, `pkg/testing/runner.go`, `cmd/stc/{check,vendor_cmd,test_cmd,sim_cmd,gvlname}.go`.
- Scratchpad experiments (not committed): converter + analyzer baseline, line-preserving layout prototype, repro of `fb();` SEMA022, PROPERTY PUBLIC parse failure, PVOID assignability, negative enum and subrange parsing.

### Secondary (MEDIUM confidence)
- [Beckhoff InfoSys Tc2_System T_AmsNetID](https://infosys.beckhoff.de/content/1033/tcplclib_tc2_system/31059723.html): T_AmsNetIdArr as byte array (web search summary).

### Tertiary (LOW confidence)
- None used for decisions.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — stdlib only, verified locally.
- File formats and resolution: HIGH — read from the oracle files; transitive library visibility MEDIUM (A1).
- Stub signatures: HIGH — copied from manuals; two default values ASSUMED.
- Pitfalls and baseline: HIGH — measured on this branch.

**Research date:** 2026-10-06
**Valid until:** 2026-11-05 (sildarvinnsla HEAD may migrate ST201/ST301 and change the genuine-finding list)
