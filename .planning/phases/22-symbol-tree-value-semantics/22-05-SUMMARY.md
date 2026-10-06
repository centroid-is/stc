---
phase: 22-symbol-tree-value-semantics
plan: 05
subsystem: interp
tags: [runtime, get-set, coercion, tojson, sim, fixture, oracle, coverage-gate, runt-01, runt-02, runt-05, runt-06]

requires:
  - phase: 22-01
    provides: "storeAs typed-store choke point, integer wrap"
  - phase: 22-02
    provides: "untyped literal adoption in the checker (oracle literal class 0)"
  - phase: 22-03
    provides: "pkg/symtree Build/Lookup/Walk and the path grammar"
  - phase: 22-04
    provides: "instantiateVar, evalInit, constant bounds, RegisterGVLs two-pass, InitErrors"
provides:
  - "interp.Runtime: every TYPE, enum, FUNCTION, FB, GVL and PROGRAM of a file set on one interpreter, instantiated eagerly"
  - "Mutex-serialised Runtime.Tick / Get / Set / Snapshot, plus ToJSON"
  - "Interpreter.RegisterFiles and NewScanCycleEngineWith"
  - "stc sim --set PATH=VALUE / --get PATH (text and JSON)"
  - "ARRAY OF FB and struct FB members as live instances; fbs[i](...) calls"
  - "ST301-shaped fixture proving SC1-SC4 end to end; oracle literal-class zero assertion"
affects: [phase-23-tasks, phase-28-opcua, mcp, hmi]

tech-stack:
  added: []
  patterns:
    - "Current value is the type witness for Set coercion (interp never imports symtree)"
    - "Dry-run then apply for map writes into FB instances (all-or-nothing)"
    - "orderedObject for deterministic JSON in declared order"

key-files:
  created:
    - pkg/interp/register.go
    - pkg/interp/runtime.go
    - pkg/interp/runtime_path.go
    - pkg/interp/coerce.go
    - pkg/interp/tojson.go
    - pkg/interp/runtime_test.go
    - pkg/sim/engine_with_test.go
    - tests/runtime/st301_shape/EcDiagParam.st
    - tests/runtime/st301_shape/ECT_Diag.st
    - tests/runtime/st301_shape/GVL.st
    - tests/runtime/st301_shape/pous.st
    - tests/runtime_fixture_test.go
  modified:
    - pkg/interp/scan.go
    - pkg/interp/fb_instance.go
    - pkg/interp/init_value.go
    - pkg/interp/interpreter.go
    - pkg/sim/engine.go
    - pkg/sim/result.go
    - cmd/stc/sim_cmd.go
    - cmd/stc/sim_cmd_test.go
    - tests/twincat_probes_test.go
    - .planning/phases/22-symbol-tree-value-semantics/22-VALIDATION.md

key-decisions:
  - "Runtime.Tick advances the shared interpreter clock once per cycle, then runs every PROGRAM in source order"
  - "External integer writes are range-checked and rejected, never wrapped (research A2)"
  - "Map writes into FB instances run a validation pass first, so a failing Set changes nothing"
  - "ToJSON renders enum value names whenever an ordinal matches, independent of {attribute 'to_string'}"
  - "stc sim now builds an interp.Runtime, so initialiser errors are fatal ('initialisation error')"

requirements-completed: [RUNT-01, RUNT-02, RUNT-05, RUNT-06]

duration: 40min
completed: 2026-10-06
---

# Phase 22 Plan 05: Live Runtime Get/Set, sim flags and phase gate Summary

**interp.Runtime with mutex-serialised Tick/Get/Set/Snapshot over one shared interpreter, JSON-style coercion with range checks, deterministic ToJSON, `stc sim --set/--get`, and an ST301-shaped fixture proving SC1-SC4 end to end.**

## Performance

- **Duration:** about 40 min
- **Completed:** 2026-10-06
- **Tasks:** 5
- **Files modified:** 22

## Runtime API (integration point for Phase 28 OPC UA NodeSource)

```go
type RuntimeOpts struct{ LibraryFiles []*ast.SourceFile }
func NewRuntime(files []*ast.SourceFile, opts ...RuntimeOpts) (*Runtime, error)
func (r *Runtime) Tick(dt time.Duration) error      // mutex; clock once, PROGRAMs in source order
func (r *Runtime) Get(path string) (Value, error)   // mutex; follows a leaf reference
func (r *Runtime) Set(path string, v any) error     // mutex; coerced, visible on next Tick
func (r *Runtime) Snapshot() any                    // mutex; {GVL...,PROGRAM...: {var: json}}
func (r *Runtime) ToJSON(v Value) any               // JSON-ready, ordered, deterministic
func (r *Runtime) Engine(name string) *ScanCycleEngine // single-goroutine callers (stc sim)
func (r *Runtime) Interpreter() *Interpreter
const MaxRuntimePathLen = 1024
func (interp *Interpreter) RegisterFiles(files []*ast.SourceFile) error
func NewScanCycleEngineWith(interp *Interpreter, prog *ast.ProgramDecl) *ScanCycleEngine
```

Paths: `ROOT.var.member[3].bit`, case-insensitive, GVL roots before PROGRAM roots, same grammar as `symtree.ParsePath`. Get/Set accept bool, all Go int/uint/float kinds, `json.Number`, strings (IEC literals `16#FF`, `INT#5`, `T#5s`, `D#...`, `'quoted $N'`, enum `Run`/`E.Run`/`E#Run`), `[]any` for arrays (from ArrayLow) and `map[string]any` for structs and FBs. Set rejects unknown paths, CONSTANT members (GVL and PROGRAM), pointers, standard FB outputs, unbound references, out-of-range integers, NaN/Inf and REAL overflow.

## Task Commits

1. **Task 1: RegisterFiles, shared scan engines, Runtime skeleton** - `cad33b3` (test), `3e342a7` (feat)
2. **Task 2: path resolution, Get, ToJSON, Snapshot** - `aa488d9` (test), `512bd68` (feat)
3. **Task 3: Set with coercion, access rules, race safety** - `fddd6d3` (test), `7f0145d` (feat)
4. **Task 4: stc sim --set/--get** - `36e8ab1` (feat, tests included)
5. **Task 5: fixture, oracle assertion, validation, gate** - `eb81eae` (fix), `5ad3991` (test), `7b35e90` (test), `6f202b9` (fix), `7cabd97` (docs)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] ARRAY OF FB elements were INT zeros and `fbs[i]()` could not be called**
- **Found during:** Task 5. The agreement subtest failed on every `GVL.fb[n]...` path.
- **Issue:** `zeroFromType` only resolved TYPE names, so `fb : ARRAY[1..2] OF FB_ATV320` held integers. A call on an index expression was an "unsupported call target".
- **Fix:** `typeCtx.fbInstance` builds a fresh stdlib or user FB instance per element or struct member, with the same parent env a plain FB variable gets. `execCallStmt` accepts `fbs[i](IN := x)`.
- **Files modified:** pkg/interp/fb_instance.go, init_value.go, interpreter.go
- **Commits:** eb81eae, 6f202b9

**2. [Rule 1 - Bug] Several programs on one interpreter would advance the clock once each**
- **Fix:** `ScanCycleEngine.tick(dt, advance)`. `Runtime.Tick` calls `SetDt` once. Commit 3e342a7.

**3. [Rule 2 - Missing functionality] Snapshot added**
- The team lead asked for a snapshot entry point for Phase 28. `Runtime.Snapshot()` renders every root in order. Commit 512bd68.

**4. Oracle assertion uses a regular expression**
- The argument messages carry the parameter name, so the nine-message list is a regex over parsed messages, with its own unit test (`TestLiteralClassPattern`).

### TDD gate note
Task 4 has no separate RED commit; its exec tests landed with the implementation in 36e8ab1. Tasks 1-3 follow test-then-feat.

## Verification

- `go test ./... -count=1`: all packages pass.
- `go run ./cmd/stc test tests/`: 246 passed, 0 failed.
- `STC_PROBES_DIR=... go test ./tests -run 'TwinCAT|TestRuntimeFixture'`: pass, literal class 0 on both oracle files.
- `go test -race ./pkg/interp -run TestRuntimeConcurrent`: pass.
- `bash scripts/coverage-gate.sh`: interp 98.77%, checker 98.57%, parser 98.34%, lexer 97.60%, emit 97.32%, types 100%, total 96.39%. All new files in pkg/interp have no uncovered statements. pkg/symtree is 98.5% standalone; the gate script does not print it.

## Remaining non-literal oracle buckets (from 22-02, out of scope)

| Count | Message class |
|------:|---------------|
| 15 | cannot pass WORD as input parameter (expected UINT) |
| 1 | cannot assign WORD to UINT |
| 5 | AND/OR requires BOOL operands, got UINT and UINT |
| 30 | SEMA010 built-ins: ADR 10, SIZEOF 5, SHL 3, conversions 12 |
| 35 | SEMA037 undeclared types and other flattening artefacts |

st301.st stays dominated by flattening errors (undeclared identifiers, SEMA037, SEMA033).

## Known Differences

- **Research A1:** stc wraps typed intermediates, so `i + 1 > i` with i = 32767 is FALSE. CODESYS may compare at register width. Stores are identical. A TwinCAT oracle check is still open.
- **Research A5:** STRING lengths are not tracked, so Set accepts strings longer than `STRING(n)`.
- Subrange bounds are not enforced by Set.
- `Runtime.Engine` bypasses the mutex. It is meant for single-goroutine callers such as `stc sim`.
- Only the first array dimension is modelled, as before.

## Next Phase Readiness
- Phase 28 can wrap Get/Set/Snapshot as the OPC UA NodeSource. Use symtree for browse metadata and Runtime for values; the agreement test guarantees every scalar tree leaf resolves.
- Phase 23 replaces Runtime.Tick's "all programs in source order" with task scheduling.

## Self-Check: PASSED
