---
phase: 21-twincat-project-import-library-stubs
plan: 03
subsystem: vendor/twincat
tags: [twincat, tsproj, plcproj, tctto, tcpou, converter, import]

requires:
  - phase: 21-01
    provides: PROPERTY access modifiers parse, statement-level fb() calls
provides:
  - pkg/vendor/twincat Model, Task, Source, LibraryRef, Kind and ResolvedFrom constants
  - VEND020-VEND027 diagnostic codes
  - ReadTsproj (File= and inline forms), ReadPlcproj (items + PlaceholderReferences with positions), ReadTcTTO, mergeTasks
  - ConvertFile/Convert with ModeLayout, ModeCompact, ModeDecl
  - Synthetic Demo/DemoLib/Inline fixtures
affects: [21-04, 21-05, 21-06, 23, 28]

tech-stack:
  added: []
  patterns:
    - "Line-preserving layout: CDATA text placed at XML line/col (InputOffset before Token, +9 for <![CDATA[)"
    - "Synthesized keywords recorded before the element stack is popped so they keep action/accessor context"
    - "Positions from raw bytes via lineCol(raw, InputOffset) for plcproj start tags"

key-files:
  created:
    - pkg/vendor/twincat/model.go
    - pkg/vendor/twincat/codes.go
    - pkg/vendor/twincat/xmlutil.go
    - pkg/vendor/twincat/tsproj.go
    - pkg/vendor/twincat/plcproj.go
    - pkg/vendor/twincat/convert.go
    - pkg/vendor/twincat/model_test.go
    - pkg/vendor/twincat/helpers_test.go
    - pkg/vendor/twincat/tsproj_test.go
    - pkg/vendor/twincat/plcproj_test.go
    - pkg/vendor/twincat/convert_test.go
    - pkg/vendor/twincat/testdata/
  modified: []

key-decisions:
  - "Malformed XML is a typed *BadXMLError (VEND027 in its message); plan 04 turns it into a diagnostic"
  - "PlcprojInfo.Items carry a by-extension Kind (TcTTO is KindTask); the converter decides the final kind from the XML root"
  - "PlaceholderRef.Pos is the start-tag line and column, not column 1"
  - "Default task with neither tsproj nor TcTTO is named PlcTask with an empty program list"
  - "ModeCompact and ModeDecl both start with a // source: header"

requirements-completed: []
requirements-contributed: [IMPT-01, IMPT-03]

duration: 25min
completed: 2026-10-06
---

# Phase 21 Plan 03: TwinCAT project readers and line-preserving converter Summary

**`pkg/vendor/twincat` reads both tsproj shapes, xti, plcproj and TcTTO with positions, and converts TcPOU/TcGVL/TcDUT/TcIO to ST whose line and column equal the XML position.**

## Performance

- **Duration:** about 25 min
- **Tasks:** 3/3
- **Files created:** 11 Go files plus fixtures

## Accomplishments

- The File= Demo tsproj yields PLC name Demo, AMS port 851, PlcTask at 1 ms, priority 20, and a VEND022 info for the TwinSAFE project. The inline tsproj yields 20 ms.
- ReadPlcproj returns Compile items in file order and PlaceholderReferences with plcproj positions. Visu\Screen.TcVIS gets a VEND021 warning at its line.
- The converter places every CDATA segment at its XML line and column, verified token by token for FB_Motor, MAIN, I_Motor, F_Add and GVL_Main.
- CRLF+BOM rewrites decode identically to LF for tsproj, xti, plcproj, TcTTO and TcPOU.
- Package coverage is 96.8%.
- A throwaway check (not committed) converted all 119 real TcPOU/TcGVL/TcDUT objects in SVNCoreComponents, ST301 and Baader in layout and declaration mode with zero parse diagnostics and no converter diagnostics.

## Exported API for plans 04 and 05

| Name | Purpose |
|------|---------|
| `ReadTsproj(path) (*TsprojInfo, []diag.Diagnostic, error)` | `TsprojInfo{PlcName, AmsPort, PlcprojPath, Tasks}` |
| `ReadPlcproj(path) (*PlcprojInfo, []diag.Diagnostic, error)` | `PlcprojInfo{Path, Items []Item{RelPath, AbsPath, Ext, Kind, Line}, Refs []PlaceholderRef{Name, DefaultResolution, Namespace, Pos}}` |
| `ReadTcTTO(path) (Task, error)` | TcTTO name, µs cycle, priority, PouCall programs |
| `mergeTasks(ts []Task, ttos []ttoFile, projectPath)` | unexported, used by Import in plan 04 |
| `ConvertFile(path, relPath, mode)` / `Convert(path, relPath, raw, mode)` | returns `Converted{Kind, Name, Text}` |
| `ModeLayout`, `ModeCompact`, `ModeDecl` | renderers |
| `KindPOU/GVL/DUT/ITF`, `KindTask` | object kinds |
| `FromProject/Sibling/LibraryPath/Stub/Builtin/Unresolved` | resolution values |
| `*BadXMLError{Path, Err}` | VEND027 |
| `Code*` constants | VEND020-VEND027 |

## Task Commits

1. **Task 1: Model, codes, fixtures:** `f0f9229` (feat)
2. **Task 2: Readers:** `3bd9cd7` (test, RED), `ef12d13` (feat, GREEN)
3. **Task 3: Converter:** `146497a` (test, RED), `fd13358` (feat, GREEN)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] End keywords lost their action/accessor context**
- **Found during:** Task 3 GREEN
- **Issue:** END_ACTION was recorded after the element stack was popped, so declaration-only mode kept it outside the dropped action.
- **Fix:** Record closing keywords before popping the stack.
- **Files modified:** pkg/vendor/twincat/convert.go
- **Commit:** fd13358

### Additions

- `xmlutil.go` holds `BadXMLError`, `normPath`, `slashPath`, `lineCol` and `decodeXML`, shared by the readers and the converter.
- `KindTask` was added for TcTTO plcproj items because the plan's item kinds included TcTTO.
- The Demo fixture FB_Motor does not declare `IMPLEMENTS I_Motor`, so the interface conformance check cannot block the zero-error Demo import in plan 04.
- `testdata/convert/FB_Fbd.TcPOU` was added for the non-ST implementation case.

## TDD Gate Compliance

Both TDD tasks have a `test(21-03)` commit followed by a `feat(21-03)` commit.

## Known Stubs

None.

## Next Phase Readiness

Plan 04 can build `Import()` on `ReadTsproj`, `ReadPlcproj`, `ReadTcTTO`, `mergeTasks` and `ConvertFile`, and rewire `pkg/vendor/extract.go` to `ModeDecl`. The Demo fixture references DemoLib (sibling), Tc2_EtherCAT, Tc2_Standard, Tc2_System, Tc3_Module, Tc2_SerialCom and the unresolvable Tc2_Missing. The `sln/.hidden/DemoLib.plcproj` decoy must never be picked.

## Self-Check: PASSED

- All created files exist.
- Commits f0f9229, 3bd9cd7, ef12d13, 146497a and fd13358 are on main.
