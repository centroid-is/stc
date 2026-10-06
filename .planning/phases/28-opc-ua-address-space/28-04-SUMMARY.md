---
phase: 28-opc-ua-address-space
plan: 04
subsystem: opcua
tags: [opcua, tf6100, serve, cli, symtree, runtime, golden, st301]

requires:
  - phase: 28-03
    provides: "Build(root, src), Publish, opcuatest.BrowseSnapshot, st301_shape.json golden"
  - phase: 22
    provides: "symtree.Build/Tree/Node, interp.Runtime NewRuntime/Tick/Get/Set"
  - phase: 21
    provides: "twincat.Import, analyzer.AnalyzeProject"
provides:
  - "pkg/opcua/bind: Root(*symtree.Tree) opcua.SymbolNode"
  - "pkg/opcua/bind: RuntimeSource (NewRuntimeSource, Read, Snapshot, Write, ApplyPending, Pending, MaxPending)"
  - "interp.Runtime.CheckSet(path, v) error and GetMany(paths) ([]Value, error)"
  - "stc serve <project|.st|dir> --opcua --security --cert --key --pki-dir --cycle --realtime --run-for"
  - "tests/opcua_golden/st301_shape/*.st parsed ST fixture"
  - "docs/OPCUA.md"
affects: [23, 29]

tech-stack:
  added: []
  patterns:
    - "Adapters live in pkg/opcua/bind so pkg/opcua never imports symtree or interp"
    - "Writes validated with Runtime.CheckSet at submit time, queued, applied by ApplyPending before each Tick"
    - "Reads are deep copies taken under the runtime lock; Snapshot is one critical section"
    - "serve keeps serving the last image when the scan fails; analysis errors are reported, not fatal"

key-files:
  created:
    - pkg/opcua/bind/symtree.go
    - pkg/opcua/bind/symtree_test.go
    - pkg/opcua/bind/runtime.go
    - pkg/opcua/bind/runtime_test.go
    - pkg/interp/runtime_batch.go
    - pkg/interp/runtime_batch_test.go
    - cmd/stc/serve_cmd.go
    - cmd/stc/serve_cmd_test.go
    - tests/opcua_golden/st301_shape/types.st
    - tests/opcua_golden/st301_shape/fbs.st
    - tests/opcua_golden/st301_shape/GVL_BatchLines.st
    - tests/opcua_golden/st301_shape/sensors.st
    - tests/opcua_golden/st301_shape/GVL_Roe.st
    - tests/opcua_golden/st301_shape/main.st
    - docs/OPCUA.md
  modified:
    - cmd/stc/main.go
    - pkg/parser/pragma.go
    - pkg/parser/pragma_test.go
    - pkg/ast/attribute.go
    - pkg/ast/attribute_test.go
    - docs/CLI_REFERENCE.md
    - .planning/phases/28-opc-ua-address-space/28-VALIDATION.md
    - .planning/phases/28-opc-ua-address-space/deferred-items.md

key-decisions:
  - "serve reports analysis errors but keeps going; only load, instantiate, flag and listener failures exit 1"
  - "A Tick error stops the scan and serve keeps publishing the last image until SIGINT or --run-for"
  - "Phase 23 is not merged, so serve paces Tick with its own time.Ticker loop; --realtime uses a locked OS thread with absolute deadlines"
  - "IEC $ escapes in attribute pragma values are kept raw by the parser, and opcua unescapes Descriptions"
  - "The golden was not edited; the parsed fixture matches it byte for byte"

requirements-completed: [OPCUA-01, OPCUA-02, OPCUA-03, OPCUA-04, OPCUA-05, OPCUA-06]

duration: 75min
completed: 2026-10-06
---

# Phase 28 Plan 04: stc serve and Project Adapters Summary

**`stc serve` runs a parsed project's scan and serves it as a TF6100 OPC UA address space. symtree and the interp Runtime are bound in through pkg/opcua/bind, and the parsed ST301-shaped fixture browses byte-identical to the 28-03 golden.**

## Performance

- Duration: about 75 minutes
- Completed: 2026-10-06
- Tasks: 4 of 4
- Files: 15 created, 8 modified

## Accomplishments

- `bind.Root` adapts any symtree to `opcua.SymbolNode`. Kinds map one to one, attributes come type-level first, and array elements are named `arr[i]`.
- `bind.RuntimeSource` serves the Runtime with deep-copied reads and a single-lock Snapshot. Writes are validated at submit time and applied between scans.
- `stc serve` loads a .tsproj/.plcproj, .st files or directories. It prints the endpoint, namespace 4 and the node count, and with `--format json` a one-line start-up object.
- `TestServeST301Parity` serves `tests/opcua_golden/st301_shape/` and compares the browse with `st301_shape.json` byte for byte. It then writes `Conveyor.xRun` and `Cfg.rMax` and sees the scan-computed `rSpeed` follow after the TON elapses.
- Basic256Sha256 secure-only serving is tested with a client certificate, and SecurityPolicy None is refused.

## Task Commits

1. **Task 1: symtree adapter** - `1b9bd63` (feat)
2. **Task 2: Runtime NodeSource adapter, CheckSet/GetMany** - `f3ecf79` (feat)
3. **Task 3: pragma $-escape fix** - `8ee949f` (fix)
4. **Task 3: stc serve, ST fixture, parity and exec tests** - `e2306dc` (feat)
5. **Task 3/4: project and JSON-event serve tests** - `cd91f4e` (test)
6. **Task 4: VALIDATION sign-off, docs** - `bacecde` (docs)

## API Name Mapping

| Plan expectation | Actual on main |
|------------------|----------------|
| `Runtime.RunFree(ctx, cycle)` (Phase 23) | Not merged. serve has its own `scanLoop` that calls ApplyPending, then Tick, then waits |
| Snapshot under the runtime lock | New `Runtime.GetMany(paths)`, one critical section, returns Clone()d values |
| Write validation without applying | New `Runtime.CheckSet(path, v)` uses the coerce dry-run |
| sim's project loader | `twincat.Import` plus `analyzer.AnalyzeProject`, because symtree needs the AnalysisResult |

## Smoke Test (ST301)

Command: `stc serve ".../ST301 solution.tsproj" --opcua 127.0.0.1:48400`, then a browse with `opcuatest.Take`.

| Result | Value |
|--------|-------|
| i=2259 | 0 (Running) |
| Published nodes | 13475 (14195 browsed incl. DataTypes) |
| GVL roots under PLC1 | CVS03, ECT, ECT_Diag, EPW01, FPW01, GVL_3rd, SPB03, ST303, STM03, buttons, jam, section, sensors |
| Analysis errors reported | 12 (SIZEOF, ADR, UINT_TO_WORD, FB_TwoWayConveyor, ST_LineRecipe member) |
| Scan | stopped on the first Tick: `undefined function: SIZEOF`; values from initialisation kept serving |

Values such as `ns=4;s=CVS03.OptimarInfeedPermitted` read Good. Phase 23 adds SIZEOF/ADR, after which the scan should keep running.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Attribute values with IEC `$'` escapes were dropped**
- **Found during:** Task 3 (parity test)
- **Issue:** The parser's pragma value scanner ended the string at the quote of `$'`. `{attribute 'OPC.UA.DA.Description' := 'Batches on line $'1$' ($$ counted)'}` was therefore kept as a raw pragma, and the node lost its Description.
- **Fix:** `$x` pairs are now kept raw for the consumer, which is opcua's unescapeIEC. `Attribute.String` copies them verbatim, so the round trip holds. A trailing lone `$` renders as `$$`.
- **Files modified:** pkg/parser/pragma.go, pkg/ast/attribute.go and their tests
- **Commit:** 8ee949f

**2. [Rule 2 - Missing functionality] Validate-without-write and consistent multi-read on Runtime**
- **Issue:** Runtime had no dry-run Set and no multi-path read under one lock. Get also returns array slices that the scan keeps mutating.
- **Fix:** Added `CheckSet` and `GetMany` in a new file, pkg/interp/runtime_batch.go. runtime.go was not touched, to avoid conflicts with Phase 23.
- **Commit:** f3ecf79

**3. [Rule 2 - Security] Bounded write queue**
- **Issue:** T-28-13 covers DoS. An unbounded queue between scans would let a client grow memory.
- **Fix:** `MaxPending` is 4096. Writes beyond it fail.
- **Commit:** f3ecf79

### Other adjustments

- Analysis errors do not abort serve, unlike the plan's "exit 1 with diagnostics". The team lead asked that the ST301 address space be published even when analysis or the runtime fails. Only error diagnostics are listed in the start-up report.
- The project can be given positionally or with `--project`. `--realtime` and `--pki-dir` were added; `--pki-dir` keeps tests out of the user cache.
- The fixture has one file per GVL (sensors.st, GVL_Roe.st), because .st GVL names come from the file basename. FB-internal values are produced by FB bodies, because the checker forbids writing FB internals from MAIN.
- The plan's struct-member write of `p_cmd_JogFwd` is replaced by scalar writes. HMI is a StructuredType Variable, so it has no member node. 28-02 covers struct writes.
- Tests run serve in-process (cobra `ExecuteContext`) so coverage counts. One exec test covers the built binary's `--help`, its JSON line and a clean exit after `--run-for`.

## Deferred Issues

- pkg/interp cannot initialise TOD, DATE or DT variables from literals. This is logged in deferred-items.md.
- OPC UA build diagnostics print with zero positions (`:0:0: warning: ...`). This is cosmetic.
- cmd/stc/main.go has a gofmt alignment issue that predates this plan.

## Known Stubs

None.

## Threat Flags

None. All new surface (the listener, remote writes, CLI paths) is covered by T-28-12..15.

## Verification

- `go test ./... -count=1` passes.
- `go test -race ./pkg/opcua/... ./cmd/stc -count=1` passes.
- `bash scripts/coverage-gate.sh` exits 0, with 96.62% in total.
- Coverage is 95.4% for pkg/opcua, 100.0% for pkg/opcua/bind, 100% for CheckSet/GetMany, and 67-100% per function in serve_cmd.go.
- `go list -deps ./pkg/opcua | grep -c stc/pkg/symtree` prints 0.

## Self-Check: PASSED
