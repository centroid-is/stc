# Phase 22: Symbol Tree & Value Semantics - Context

**Gathered:** 2026-10-06
**Status:** Ready for planning
**Mode:** Autonomous smart discuss (recommended answers accepted; user asked for maximum speed, parallel track B in worktree ../stc-wt-22 on branch gsd/phase-22-symbol-tree)

<domain>
## Phase Boundary

Every live variable in a running project is addressable by dotted path with correct IEC value semantics: a symbol tree built from analysis (GVLs, PROGRAMs, FB instances, struct members, array elements, with IEC type, attributes and enum strings), `Get(path)`/`Set(path, value)` on the live interpreter with coercion from JSON-style values, integer wrap per declared type, untyped literals adopting the context type in the checker, and array/struct/struct-array initialisers (constant-expression bounds) applied at instantiation. Consumers: tests, MCP tools, the OPC UA server (Phase 28), ADS later. Not in scope: multi-task scheduling and PERSISTENT files (Phase 23), project import (Phase 21, running in parallel on main; do not edit pkg/vendor, cmd/stc/vendor*.go, stdlib/vendor), EtherCAT binding (24), OPC UA (28).

</domain>

<decisions>
## Implementation Decisions

### Symbol tree (RUNT-01)
- New package `pkg/symtree`. `Build(analysis *analyzer.Result) (*Tree, error)` produces a tree of `Node{Name, Path, Kind (GVL, Program, FBInstance, Struct, Array, Scalar, Enum, Reference, Pointer), Type types.Type, Attributes []ast.Attribute (merged: type-level attributes of the declaring FB/STRUCT plus instance-level), EnumStrings map[int64]string (for enum-typed scalars), Children (lazy for arrays: bounds known from the type), Decl position}`. Paths use declared case (`GVL.fb[2].HMI.p_stat_State`), lookups are case-insensitive, array indices are the declared bounds (1-based if declared `[1..N]`), struct members and FB variables (VAR_INPUT/OUTPUT/VAR/VAR_IN_OUT) are children in declaration order.
- Root children: every GVL (by name), every PROGRAM (by name). FB instances expand into their declared variables (type-level recursion with a cycle guard); standard FBs (TON etc.) expose their inputs/outputs (IN, PT, Q, ET...).
- `Tree.Lookup(path)`, `Tree.Walk(fn)`, and `Tree.JSON()` for `--format json`; `stc check --symbols` (or `stc symbols <files>`) prints the tree. Attributes are kept verbatim so Phase 28 can read `OPC.UA.DA`, `OPC.UA.DA.Access`, `.StructuredType`, `.Description`.

### Live Get/Set (RUNT-02)
- `interp.Runtime` (wrapping ScanCycleEngine + GVL env layer) gains `Get(path string) (Value, error)` and `Set(path string, v any) error`, resolving paths against the live environments (GVL env, program env, FB instance envs, struct values, array elements, bit access `x.3` allowed in paths). `Set` coerces Go `bool`, `int*`, `float64`, `json.Number`, `string` (enum value name or literal text like `T#5s`, `16#FF`, `'text'`), `[]any`, `map[string]any` into the declared IEC type, with range checks and wrap per the type; errors for unknown paths or incompatible values. Values flow back as `Value` plus a `ToJSON()` helper (enum → name when the type has enum strings, TIME → ms number plus ISO string, struct → object, array → list).
- Thread-safety: `Runtime` serialises Get/Set with the scan via a mutex (Phase 23 free-running mode will call Tick under the same lock). Sets between scans take effect at the next scan.
- Test built-ins `SET(path, value)` / `GET(path)` in `*_test.st` and MCP tools are Phase 27/29; here only the Go API plus one exec-level CLI hook (`stc sim --set PATH=VALUE --get PATH`) so it is observable end to end.

### Integer wrap and literal typing (RUNT-05)
- Interpreter: arithmetic results are wrapped to the result IEC type (INT 32767+1 = -32768, UINT 0-1 = 65535, SINT/USINT/DINT/UDINT/LINT/ULINT, BYTE/WORD/DWORD/LWORD likewise) at the point the value is stored into a typed variable AND when produced by a typed binary expression (operand type after IEC promotion). Division by zero stays a runtime error. REAL/LREAL unaffected. Initialised integer variables must carry their declared IECType (fixes the 20-05 deferred "w : WORD := 16#9 has width 32").
- Checker: untyped integer literals get an "untyped int" type that adopts the expected type in assignments, binary operations with a typed operand, call arguments, CASE labels, initialisers and comparisons; out-of-range literals for the adopted type are an error (SEMA021 message variant); default DINT when nothing constrains them. Untyped real literals adopt REAL/LREAL similarly (`x / 5.0` with REAL x stays REAL). Mixed typed operands keep the existing widening lattice. Remove the Phase 20 allow-list entry for `cannot assign DINT to INT` in tests/twincat_dialect_check_test.go and the oracle allowlists.
- The remaining Phase 20 deferred checker items that are literal-typing related ("unsigned + untyped literal widens to LREAL", "r := 0 errors while initialiser accepts it") are closed by this.

### Initialisers at instantiation (RUNT-06)
- `StructInit`/`ArrayInit` (incl. repetition `3(0)`, nested, omitted fields → type defaults, TYPE defaults for alias/enum types) are evaluated when a variable, GVL member, FB instance variable or struct member is instantiated; constant-expression array bounds (`ARRAY[1..GVL.CONST]`, `C_MAX*2`) are evaluated from GVL/POU constants at instantiation (GVLs instantiate before POUs; constants resolve across GVLs). FB VAR initialisers apply per instance (incl. inherited EXTENDS variables). `w : WORD := 16#9` yields a WORD-typed value.
- `Get("ECT_Diag.Device_1_SlaveInfo[3].p_stat_sName")` at cycle 0 returns the initialiser value.

### Claude's Discretion
- Whether `pkg/symtree` depends on `pkg/interp` (prefer: symtree depends on analyzer/types only; interp exposes an adapter that maps tree nodes to live values).
- Exact JSON shapes, error texts, SEMA code numbering for literal range errors.
- Coverage strategy; gates: parser/lexer/interp/types/emit >= 95%, checker >= 94%, total >= 85% (current parser 98.3, checker 98.5, interp 98.1, emit 97.3).

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Phase 19/20 interpreter: GVL env layer (`RegisterGVL`, `GlobalParent`), `FBInstance` with `fbDeclChain`, `Value{Kind, Int, Real, Str, Time, Array, Struct, FBRef, Enum tag, IECType}`, `assignToTarget`, bit access (`bits.go`), `EnumDef` registry and `EnumOrdinals`, `bindArgs`, `FuncDecls`, path references (`RefPath`).
- Checker: pointer-stable two-pass registration, `Symbol.ConstInt/HasConstInt`, `types.ArrayDimension.Known`, `isLiteralCompatible` (initialiser literal rule), `CanWiden` lattice in pkg/types.
- `scripts/coverage-gate.sh`, `tests/twincat_probes_test.go`, `tests/twincat_dialect_check_test.go` (Phase 22 allowlist entry to remove), oracle at `/Users/jonb/Projects/beckhoff-docs/stc-probes/` (STC_PROBES_DIR).

### Established Patterns
- Tagged-union Value, Env parent chains, per-package table-driven tests; CLI exec tests with GOCOVERDIR.

### Integration Points
- `pkg/analyzer` result → `pkg/symtree.Build`; `pkg/interp` ScanCycleEngine/`Runtime` → Get/Set; `pkg/checker` expression typing (binary ops, assignment, call args, CASE) for untyped literals; `pkg/interp/value.go` arithmetic for wrap; `pkg/interp` variable instantiation (`initVarDecl`) for initialisers and constant bounds; `cmd/stc/sim_cmd.go` for `--set/--get`; `cmd/stc/check.go` for `--symbols`.

</code_context>

<specifics>
## Specific Ideas

- Real shapes: `Device_1_SlaveInfo : ARRAY[1..EcDiagParam.MAX_EC_SLAVES] OF ST_EcSlaveInfo := [(p_stat_sName := 'ST301.A1.01 (EL6070)', p_stat_sModel := 'EL6070', p_stat_nPhysAddr := 1001, ...), ...];`, `HMI : ST_Drive_HMI` inside `FB_ATV320` with `p_cfg_ManualFreq : REAL := 20.0`, `p_stat_xAuto : BOOL := TRUE`, enums `hmis_e` with `to_string`, `Drives_Line1 : ARRAY[1..2] OF FB_ATV320`.
- Paths the HMI uses later: `GVL_BatchLines.Drives_Line1[1].HMI.p_stat_Error`, `sensors.EPW01_WA01_IS11.HMI.p_stat_xRaw`.
- Oracle residuals that this phase should remove: the 34 Phase 22 literal-typing errors on imported ST301 + SVNCore (measured by Phase 21 research), the `x / 5.0` LREAL→REAL class, `n := n + 1` on INT.
- Parallel track: Phase 21 is executing on main concurrently; avoid pkg/vendor, cmd/stc/vendor*.go, stdlib/, docs/VENDOR_LIBRARIES.md. Keep .planning edits to this phase's directory plus STATE/ROADMAP plan-progress lines (the orchestrator merges).

</specifics>

<deferred>
## Deferred Ideas

- Task scheduling, free-running mode, PERSISTENT state → Phase 23.
- ST test built-ins SET/GET and MCP tools → Phases 27/29.
- Pointer arithmetic, `__NEW`, UNION, BIT type → out of v1.2.

</deferred>
