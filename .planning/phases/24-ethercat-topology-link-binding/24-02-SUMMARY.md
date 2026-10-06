---
phase: 24-ethercat-topology-link-binding
plan: 02
subsystem: ecat
tags: [ethercat, twincat, tclinkto, link-validation, cli]
requires:
  - "24-01 pkg/ecat LoadProject, Topology.Slot/Paths, Slot, Dir"
provides:
  - "ParseTcLinkTo/Link: single and multi-member TcLinkTo parsing"
  - "CollectLinks/LinkedVar: AST walk of GVLs and PROGRAMs for linked leaves with width, direction, position, upper-cased Steps"
  - "Resolve/Binding: ECAT001-005 diagnostics, sorted bindings; SortDiagnostics"
  - "stc ecat validate --io <Device*.xml>... <st files> (text and JSON)"
  - "tests/ecat_fixtures/demo_types.st, demo_ect.st, demo_bad.st"
affects: [24-03, 24-04, 27]
tech-stack:
  added: []
  patterns: ["AST walk instead of analyzer for link collection", "diagnostic codes as constants in pkg/ecat"]
key-files:
  created:
    - pkg/ecat/link.go
    - pkg/ecat/link_test.go
    - pkg/ecat/collect.go
    - pkg/ecat/collect_test.go
    - pkg/ecat/resolve.go
    - pkg/ecat/resolve_test.go
    - cmd/stc/ecat.go
    - cmd/stc/ecat_test.go
    - tests/ecat_fixtures/demo_types.st
    - tests/ecat_fixtures/demo_ect.st
    - tests/ecat_fixtures/demo_bad.st
  modified:
    - cmd/stc/main.go
decisions:
  - "GVL names come from file stems (parser behaviour); demo_ect.st yields GVL demo_ect, so Go tests call ast.SetGVLName(file, \"ECT\") and the CLI test copies it to ECT.st"
  - "Variables with ECAT001/003/004 errors are not bound; ECAT005 duplicates keep both bindings"
  - "Unknown widths (library types not loaded, TIME, pointers) skip the ECAT003 check; STRING links only to 48-bit slots"
  - "Added ECAT006 (malformed TcLinkTo, error) and ECAT007 (linked leaf without AT %I*/%Q*, warning)"
metrics:
  duration: "~35 min"
  completed: 2026-10-06
  tasks: 4
  files: 12
---

# Phase 24 Plan 02: TcLinkTo Parsing, Link Resolution and `stc ecat validate` Summary

TcLinkTo pragmas in GVLs and programs are parsed, walked through struct and FB members to typed leaves, resolved to process-image slots with ECAT001-007 diagnostics at the attribute position, and exposed through `stc ecat validate` in text and JSON.

## Tasks

| Task | Name | Commits |
| ---- | ---- | ------- |
| 1 | ParseTcLinkTo | 232065d (test), b744eab (feat) |
| 2 | ST fixtures and CollectLinks | adbd507 (test), a1aa4a3 (feat) |
| 3 | Resolve with ECAT001-005 | 519e354 (test), c906142 (feat) |
| 4 | stc ecat validate command | 281fb9d (feat) |

## Exported API (for 24-03)

- `type Link struct{ Member, Target string }`, `ParseTcLinkTo(value string) ([]Link, error)`.
- `type LinkedVar struct{ Path string; Steps []string; Link, TypeName string; Type types.Type; BitWidth int; Dir Dir; HasAT bool; Pos, EndPos source.Pos }`. `Steps` is upper-cased `[GVL or PROGRAM, var, member...]`.
- `CollectLinks(files []*ast.SourceFile) ([]LinkedVar, []diag.Diagnostic)`.
- `type Binding struct{ Var LinkedVar; Slot Slot }`, `Resolve(topo *Topology, vars []LinkedVar) ([]Binding, []diag.Diagnostic)`, `SortDiagnostics([]diag.Diagnostic)`.
- Codes: `CodeUnresolved` ECAT001, `CodeMemberNotDeclared` ECAT002, `CodeSizeMismatch` ECAT003, `CodeDirMismatch` ECAT004, `CodeDuplicate` ECAT005, `CodeBadLink` ECAT006, `CodeNoAT` ECAT007.

## Verification

- `go test ./pkg/ecat -count=1 -cover`: pass, 99.8%.
- `go test ./cmd/stc -run TestEcat -count=1`: pass. Under GOCOVERDIR every block of cmd/stc/ecat.go is covered.
- `stc ecat validate` on Demo Device 1/2 with demo_types.st and demo_ect.st exits 0 with 35 bindings. With demo_bad.st it exits 1 and reports ECAT001 to ECAT006 with `demo_bad.st:line:col` positions.
- `go build ./...` and `go vet` are clean.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] GVL name of demo_ect.st**
- **Found during:** Task 2
- **Issue:** The parser derives a GVL name from the file stem, so `demo_ect.st` becomes `demo_ect`, not `ECT`.
- **Fix:** Kept the planned fixture name because 24-03 references it. Go tests rename the GVL with `ast.SetGVLName(file, "ECT")`. The CLI test copies the file to a temp `ECT.st`, which matches how a real `ECT.TcGVL` flattens.
- **Commits:** adbd507, 281fb9d

**2. [Rule 2 - Missing functionality] ECAT007 for linked leaves without AT**
- **Found during:** Task 2
- **Issue:** The plan asks for a warning when a linked leaf has no AT address and names it ECAT007. A `%M` address also has no I/O direction.
- **Fix:** Missing AT and `%M` addresses both warn with ECAT007 and set `HasAT=false`, which skips the direction check.
- **Commit:** a1aa4a3

**3. [Rule 1 - Quality] Shared diagnostic sort**
- **Found during:** Task 4
- **Fix:** Moved the position sort into exported `SortDiagnostics`, used by Resolve and the CLI, with a unit test.
- **Commit:** 281fb9d

## Deferred Issues

- `pkg/checker` `TestEmptyFBCall` fails on this branch. It is pre-existing and unrelated, since pkg/checker does not depend on any file changed here. Logged in deferred-items.md.

## Threat Mitigations

- T-24-03: member paths longer than 16 segments raise ECAT002 "too deep". Alias chains and width recursion stop at depth 16. A visited set breaks FB EXTENDS cycles. Tests cover each case.

## Known Stubs

None.

## Self-Check: PASSED
