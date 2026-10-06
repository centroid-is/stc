# Phase 22: Symbol Tree & Value Semantics - Research

**Researched:** 2026-10-06
**Domain:** Go IEC 61131-3 interpreter value model, checker literal typing, symbol tree
**Confidence:** HIGH (all findings from this repo at HEAD 8d13297 in worktree `../stc-wt-22`, measured with `go build -o /tmp/stc22 ./cmd/stc`)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Symbol tree (RUNT-01)
- New package `pkg/symtree`. `Build(analysis *analyzer.Result) (*Tree, error)` produces a tree of `Node{Name, Path, Kind (GVL, Program, FBInstance, Struct, Array, Scalar, Enum, Reference, Pointer), Type types.Type, Attributes []ast.Attribute (merged: type-level attributes of the declaring FB/STRUCT plus instance-level), EnumStrings map[int64]string (for enum-typed scalars), Children (lazy for arrays: bounds known from the type), Decl position}`. Paths use declared case (`GVL.fb[2].HMI.p_stat_State`), lookups are case-insensitive, array indices are the declared bounds (1-based if declared `[1..N]`), struct members and FB variables (VAR_INPUT/OUTPUT/VAR/VAR_IN_OUT) are children in declaration order.
- Root children: every GVL (by name), every PROGRAM (by name). FB instances expand into their declared variables (type-level recursion with a cycle guard); standard FBs (TON etc.) expose their inputs/outputs (IN, PT, Q, ET...).
- `Tree.Lookup(path)`, `Tree.Walk(fn)`, and `Tree.JSON()` for `--format json`; `stc check --symbols` (or `stc symbols <files>`) prints the tree. Attributes are kept verbatim so Phase 28 can read `OPC.UA.DA`, `OPC.UA.DA.Access`, `.StructuredType`, `.Description`.

#### Live Get/Set (RUNT-02)
- `interp.Runtime` (wrapping ScanCycleEngine + GVL env layer) gains `Get(path string) (Value, error)` and `Set(path string, v any) error`, resolving paths against the live environments (GVL env, program env, FB instance envs, struct values, array elements, bit access `x.3` allowed in paths). `Set` coerces Go `bool`, `int*`, `float64`, `json.Number`, `string` (enum value name or literal text like `T#5s`, `16#FF`, `'text'`), `[]any`, `map[string]any` into the declared IEC type, with range checks and wrap per the type; errors for unknown paths or incompatible values. Values flow back as `Value` plus a `ToJSON()` helper (enum → name when the type has enum strings, TIME → ms number plus ISO string, struct → object, array → list).
- Thread-safety: `Runtime` serialises Get/Set with the scan via a mutex (Phase 23 free-running mode will call Tick under the same lock). Sets between scans take effect at the next scan.
- Test built-ins `SET(path, value)` / `GET(path)` in `*_test.st` and MCP tools are Phase 27/29; here only the Go API plus one exec-level CLI hook (`stc sim --set PATH=VALUE --get PATH`) so it is observable end to end.

#### Integer wrap and literal typing (RUNT-05)
- Interpreter: arithmetic results are wrapped to the result IEC type (INT 32767+1 = -32768, UINT 0-1 = 65535, SINT/USINT/DINT/UDINT/LINT/ULINT, BYTE/WORD/DWORD/LWORD likewise) at the point the value is stored into a typed variable AND when produced by a typed binary expression (operand type after IEC promotion). Division by zero stays a runtime error. REAL/LREAL unaffected. Initialised integer variables must carry their declared IECType (fixes the 20-05 deferred "w : WORD := 16#9 has width 32").
- Checker: untyped integer literals get an "untyped int" type that adopts the expected type in assignments, binary operations with a typed operand, call arguments, CASE labels, initialisers and comparisons; out-of-range literals for the adopted type are an error (SEMA021 message variant); default DINT when nothing constrains them. Untyped real literals adopt REAL/LREAL similarly (`x / 5.0` with REAL x stays REAL). Mixed typed operands keep the existing widening lattice. Remove the Phase 20 allow-list entry for `cannot assign DINT to INT` in tests/twincat_dialect_check_test.go and the oracle allowlists.
- The remaining Phase 20 deferred checker items that are literal-typing related ("unsigned + untyped literal widens to LREAL", "r := 0 errors while initialiser accepts it") are closed by this.

#### Initialisers at instantiation (RUNT-06)
- `StructInit`/`ArrayInit` (incl. repetition `3(0)`, nested, omitted fields → type defaults, TYPE defaults for alias/enum types) are evaluated when a variable, GVL member, FB instance variable or struct member is instantiated; constant-expression array bounds (`ARRAY[1..GVL.CONST]`, `C_MAX*2`) are evaluated from GVL/POU constants at instantiation (GVLs instantiate before POUs; constants resolve across GVLs). FB VAR initialisers apply per instance (incl. inherited EXTENDS variables). `w : WORD := 16#9` yields a WORD-typed value.
- `Get("ECT_Diag.Device_1_SlaveInfo[3].p_stat_sName")` at cycle 0 returns the initialiser value.

### Claude's Discretion
- Whether `pkg/symtree` depends on `pkg/interp` (prefer: symtree depends on analyzer/types only; interp exposes an adapter that maps tree nodes to live values).
- Exact JSON shapes, error texts, SEMA code numbering for literal range errors.
- Coverage strategy; gates: parser/lexer/interp/types/emit >= 95%, checker >= 94%, total >= 85% (current parser 98.3, checker 98.5, interp 98.1, emit 97.3).

### Deferred Ideas (OUT OF SCOPE)
- Task scheduling, free-running mode, PERSISTENT state → Phase 23.
- ST test built-ins SET/GET and MCP tools → Phases 27/29.
- Pointer arithmetic, `__NEW`, UNION, BIT type → out of v1.2.

Also from CONTEXT: do not edit pkg/vendor, cmd/stc/vendor*.go, stdlib/vendor, stdlib/, docs/VENDOR_LIBRARIES.md (Phase 21 runs in parallel on main). Keep .planning edits to this phase directory plus STATE/ROADMAP plan-progress lines.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RUNT-01 | Symbol tree for every GVL, PROGRAM, FB instance, struct member and array element with IEC type, layout, enum strings and attributes, addressable by dotted path | GVL symbol `Type` is an ordered `*types.StructType` (`pkg/checker/resolve.go:370-404`); POU var types via `table.LookupPOU(name).LookupLocal(v).Type`; attributes only in AST (`ast.VarDecl.Attributes`, FB/TYPE decl attributes); `AnalysisResult` lacks files (`pkg/analyzer/analyzer.go:19-24`) so add `Files`. See "Pattern 4". |
| RUNT-02 | Live `Get(path)`/`Set(path, value)` with type coercion | No `Runtime` type exists today. `RefPath`/`readRef`/`writeRef` (`pkg/interp/ref_path.go:22-138`) is the path-walk primitive; `fbMemberRoot` rebases into FB envs. Current value is the type witness for coercion. See "Pattern 5" and the coercion table. |
| RUNT-05 | Integer wrap per declared type; untyped literals adopt context type | `evalBinaryInt` always returns `IntValue` (DINT) and never wraps (`pkg/interp/interpreter.go:451-484`); every typed store already funnels through `adoptEnumTag` (7 sites) which becomes the wrap point. Checker types literals as DINT/LREAL (`pkg/checker/check.go:654-684`), `isLiteralCompatible` excludes bit types (`check.go:1060`). Measured 85 literal-class errors on the flattened oracles. |
| RUNT-06 | Array/struct/struct-array initialisers with constant-expression bounds applied at instantiation | Interp has no case for `*ast.ArrayInit`/`*ast.StructInit` in `evalExpr` (`interpreter.go:134-165`); `initVarDecl` swallows the error and keeps the zero value (`scan.go:225-229`). `evalConstInt` only reads literals, so `ARRAY[1..N]` allocates 1 slot (`fb_instance.go:474-534`). `ast.ConstIntValue(x, lookup)` already folds names, `G.C`, `+ - *` (`pkg/ast/enum_ordinals.go:187`). |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Go stdlib only for compiler core; no new third-party dependencies (cobra already used by the CLI).
- No Java runtime. Determinism: no wall-clock in tests; the Runtime mutex must not introduce timing-dependent behaviour.
- Every CLI command supports `--format json` (applies to `stc check --symbols` and `stc sim --get`).
- Parser error recovery must not regress (no parser work expected here).
- GSD workflow: work happens through `/gsd:execute-phase`.
- User memory: every change lands through a GitHub PR with multi-platform CI and agent review; never ship without full branch coverage verified in CI.

## Summary

The phase is four mostly independent changes in existing packages plus one new package. The interpreter today has no notion of a value's *declared* type beyond the `IECType` tag on zero values: integer literals evaluate as DINT, `evalBinaryInt` returns DINT unconditionally, and initialisers overwrite the correctly typed zero value with the literal's DINT value. Every typed store already passes through one helper, `adoptEnumTag(dst, v)` (7 call sites), which is the natural single choke point: generalise it into `storeAs(dst, v)` that adopts the enum tag, converts INT→REAL where the slot is real, and wraps integers to `dst.IECType`. Combined with typed binary results (literal operands adopt the other operand's type) this delivers RUNT-05 at runtime with a small diff.

Array and struct initialisers are silently ignored (probe: `arr : ARRAY[1..3] OF INT := [1,2,3]` reads `arr[2] = 0`; `ARRAY[1..N]` with a constant `N` gets one slot and `sa[1]` is "index out of bounds"). RUNT-06 needs a type-directed initialiser evaluator (`evalInit(typeSpec, expr)`) and constant-bound evaluation through the existing `ast.ConstIntValue` with a lookup into GVL and POU constants. The FB-instance path (`newUserFBInstanceDepth`) duplicates `initVarDecl`; unify them so initialisers, cloning and coercion behave identically everywhere.

In the checker, the 85 literal-class errors on the two flattened oracles (80 svncore, 5 st301; all verified as literal-caused) come from three roots: `checkLiteral` returns DINT/LREAL so `CommonType(UDINT, DINT)` = LREAL; `isLiteralCompatible` excludes BYTE..LWORD; and CASE/comparison use `CommonType` with no literal rule. An AST-level "untyped constant" predicate plus adoption in `checkBinaryExpr`, assignment, call arguments, CASE labels and comparisons removes all of them. The symbol tree is AST-ordered with types from the symbol table, and the Runtime resolves paths directly against live envs using the current value as the type witness, so `pkg/symtree` never needs to import `pkg/interp`.

**Primary recommendation:** Five plans. Wave 1 runs three in parallel: interp value semantics (`storeAs`, wrap, typed binaries), checker untyped literals, and `pkg/symtree` with `stc check --symbols`. Wave 2 adds initialisers and constant bounds in interp. Wave 3 adds `interp.Runtime` Get/Set/ToJSON, `stc sim --set/--get`, the ST301-shaped fixture and the oracle/coverage gate.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Symbol tree (static shape, types, attributes, enum strings) | `pkg/symtree` (new) | `pkg/analyzer` (exposes Files + Symbols) | Static, built once after analysis; consumers (OPC UA, LSP, MCP) need it without a running interpreter |
| Live Get/Set by path | `pkg/interp` (`Runtime`) | `pkg/symtree` (optional validation) | Values live in `Env`/`FBInstance`; the current value carries kind/IECType/enum tag as the coercion witness |
| Integer wrap, int→real store conversion | `pkg/interp` (`storeAs`, `evalBinary`) | — | Runtime semantics |
| Untyped literal typing, range errors | `pkg/checker` | `pkg/types` (helper predicates only) | Static semantics; keep the untyped notion private to the checker so hover/emit never see it |
| Initialisers and constant bounds | `pkg/interp` (instantiation) | `pkg/ast.ConstIntValue` | Evaluated at instantiation, GVLs first |
| CLI exposure | `cmd/stc/check.go` (`--symbols`), `cmd/stc/sim_cmd.go` (`--set/--get`) | `pkg/sim` | `--format json` everywhere |

## Current Code Facts (file:line evidence)

### pkg/analyzer
- `AnalysisResult{Symbols *symbols.Table; Diags []diag.Diagnostic}` only (`pkg/analyzer/analyzer.go:19-24`). No AST is kept. `Analyze(files, cfg, opts...)` (`:44`). **Add `Files []*ast.SourceFile` and `LibraryFiles []*ast.SourceFile`** to the result (non-breaking) so `symtree.Build` can read attributes and declaration order.
- GVL symbol: `Kind: KindGVL`, `Type: *types.StructType` whose `Members` are in declaration order with resolved types (`pkg/checker/resolve.go:370-404`). Non-qualified GVL vars are also inserted bare into the global scope.
- POU scopes: `r.table.RegisterPOU(name, KindProgram|KindFunctionBlock, pos)` (`resolve.go:454, 566`); vars resolved by `resolveVarBlocksInScope`. `table.LookupPOU(name).LookupLocal(v)` returns the variable symbol with `Type types.Type`. Scope symbols are a map, so **order comes from the AST**.
- `types.FunctionBlockType` holds only Inputs/Outputs/InOuts (`pkg/types/types.go:244-253`), not VAR locals or EXTENDS. Walk `ast.FunctionBlockDecl.VarBlocks` along the `Extends` chain for children.
- `types.EnumType{Values, Ordinals, Qualified, Strict, ToString}` (`types.go:219-231`) gives enum strings directly. `types.ArrayDimension{Low, High, Known, Text}` (`types.go:132-139`); bounds over GVL constants are `Known` because the resolver uses `enumConst` → `ast.ConstIntValue` (`resolve.go:1144-1159`).
- Attributes: `[]*ast.Attribute{Name, Value, HasValue}` (`pkg/ast/attribute.go:9-14`) on ProgramDecl, FunctionBlockDecl, VarDecl, StructMember, TypeDecl, GVLDecl, VarBlock.

### pkg/interp
- `Value` tagged union with `IECType types.TypeKind` and `Enum string` (`pkg/interp/value.go:59-82`). `IntValue(n)` sets DINT, `RealValue` LREAL (`:211-219`). Arrays are `[]Value`, structs `map[string]Value` with upper-case keys (order and declared case lost).
- `Zero(kind)` sets the right IECType (`value.go:178-204`), but STRING zero from `zeroFromTypeSpecWith` has no IECType (`fb_instance.go:425-427`).
- Literals: `parseLitInt` returns `IECType: KindDINT` always (`interpreter.go:189-213`).
- `evalBinary` (`interpreter.go:353-449`): reals promote; `evalBinaryInt` (`:451-484`) returns `IntValue(l op r)`, never wraps, ignores operand IECType. `**` always LREAL.
- Store sites all call `adoptEnumTag(dst, v)` (`pkg/interp/enum.go:221`): `assignToTarget` ident (`interpreter.go:671`), array element (`:738`), struct member (`:1176`), deref (`:1211`), GVL member (`gvl.go:136`), FB `SetInput` (`fb_instance.go:265`), function param defaults (`call_args.go:150`). Gaps that bypass it: `writeRef` (`ref_path.go:108-138`), initialisers in `initVarDecl` (`scan.go:225-229`) and `newUserFBInstanceDepth` (`fb_instance.go:128-134`), bit writes (`bits.go`), FOR loop counter updates.
- Instantiation: `initVarDecl(env, fbParent, vd)` (`scan.go:206-237`) for programs and GVLs; `newUserFBInstanceDepth` (`fb_instance.go:67-150`) is a near-duplicate for FB members (no `Clone`, same swallowed initialiser error).
- Arrays: `zeroArrayWith` allocates `high+1` slots for **direct indexing** (`fb_instance.go:470-497`), only the first dimension, capped at 10000; negative lower bounds unsupported. `evalConstInt` reads only integer literals and unary minus (`fb_instance.go:516-545`), so a constant bound evaluates to 0. `evalIndex` uses `Indices[0]` only (`interpreter.go:544-568`).
- GVLs: `RegisterGVL(decl)` builds one `Env` per GVL; non-`qualified_only` GVLs chain as `unqualified` parents (`gvl.go:35-80`). `lookupGVL(name)` (`:88`).
- `ScanCycleEngine` (`scan.go:22-40`) creates its own interpreter with `New()` (`:42-51`), runs one PROGRAM, lazily initialises. `SetGlobals` registers GVLs (`:194-198`). There is **no** constructor that takes a prepared interpreter, and `pkg/sim` never sets `FBDecls`/`TypeDecls`/enums (`pkg/sim/engine.go:46-58`), so sim cannot run user FBs today.
- The full declaration registration (functions, enums, TypeDecls, FBDecls, GVLs) exists only in `pkg/testing/runner.go:303-320, 529-557`.
- `RefPath{Env, Var, Steps []RefStep}` with `readRef`/`writeRef` (`ref_path.go:22-138`) and `fbMemberRoot(inst, member)` (`:204`) already implement "walk a member/index path from an env root".
- Initialiser AST: `ast.StructInit{Fields []*FieldInit{Name, Value}}`, `ast.ArrayInit{Elements []*ArrayInitElem{Count, Value}}` (`pkg/ast/expr.go:226-293`).

### pkg/checker
- `checkLiteral` returns DINT for `LitInt`, LREAL for `LitReal`, prefix type for `LitTyped` (`check.go:654-684`).
- `checkBinaryExpr` (`check.go:686-735`): enum handling first (`checkEnumBinary`), then `types.CommonType` for arithmetic and comparisons. `CommonType(UDINT, DINT)` = LREAL and `CommonType(BYTE, DINT)` fails, which is the root of most residuals.
- Assignment literal rule: `isLiteralExpr(s.Value) && isLiteralCompatible(...)` (`check.go:288`); `isLiteralExpr` only accepts a bare literal or `-literal` (`:1044-1055`), so `x + 1` never qualifies. `isLiteralCompatible` allows int→ANY_INT and real→ANY_REAL only (`:1060-1068`), not BYTE..LWORD, not int→REAL.
- Same rule at FB call args (`check.go:517-519`), function args (`check_calls.go:153-156`), enum args (`check_enum.go:152-155`). Initialisers use the looser `initLiteralCompatible` (`check_init.go:286-297`), hence the "r := 0 errors but r : REAL := 0 passes" inconsistency.
- CASE labels: `caseLabelCompatible` uses `CommonType` only (`check_enum.go:182-205`).
- Array index: `checkIndexExpr` requires `IsAnyInt(idxType)` (`check.go:936-961`); LREAL arrives from `i - 1` with UINT `i`.
- Codes: `CodeTypeMismatch` (SEMA001) for assignment, `CodeWrongArgType` = SEMA021 (`diag_codes.go:20`). Highest code in use is SEMA038.
- The checker records no per-expression types for other packages (no `ExprTypes` map), so an untyped notion stays private.

### Tests that encode today's behaviour
- `tests/twincat_dialect_check_test.go:19-21` allow-lists `"cannot assign DINT to INT"` (the entry to remove).
- `tests/twincat_probes_test.go:201` uses the message only as a `templateMessage` example (keep); its combined run logs Phase 21/22 buckets (`:~270`) and asserts nothing about them.
- 14 `pkg/interp/*_test.go` lines mention `KindDINT`/`IECType` and may need updating once binary results are typed.

## Measured Baseline (literal-typing class to remove)

Command: `go build -o /tmp/stc22 ./cmd/stc && /tmp/stc22 check $STC_PROBES_DIR/<file> --format json`, errors bucketed by message.

| File | Message | Count | Example source line |
|------|---------|------:|---------------------|
| svncorecomponents.st | cannot assign LREAL to REAL | 16 | `bpm.avgBPM5Minute := UDINT_TO_REAL(counter.Minute5) / 5.0;` |
| svncorecomponents.st | cannot pass DINT as input parameter (expected BYTE) | 15 | `nIndexOffset:=16#02,` |
| svncorecomponents.st | cannot pass DINT as input parameter (expected WORD) | 15 | `nIndex:=16#2001,` |
| svncorecomponents.st | cannot assign LREAL to UDINT | 11 | `HMI.p_stat_uMissed := HMI.p_stat_uMissed + 1;` |
| svncorecomponents.st | array index must be an integer type, got LREAL | 11 | `aCrcBuf[i - 1]`, `buffer[(240 + start - per1min) MOD 240]` |
| svncorecomponents.st | cannot assign LREAL to UINT | 9 | `index := (index + 1) MOD 50;` |
| svncorecomponents.st | cannot assign LREAL to USINT | 1 | `a[nPort] := a[nPort] + 1;` |
| svncorecomponents.st | cannot assign DINT to INT | 1 | `CounterCnf := CounterCnf + 1;` |
| svncorecomponents.st | cannot compare BYTE and DINT | 1 | `nLinkState <> 0` |
| st301.st | cannot assign LREAL to UINT / UDINT / REAL, DINT to INT | 2 / 1 / 1 / 1 | `uRunning := uRunning + 1;`, `... * 100.0` |
| **Total** | | **85** | svncore 80, st301 5 |

All 85 were inspected and every one is caused by an untyped literal operand. Phase 21 research measured 34 on the *imported project* run (ST301 + SVNCore as library + stubs), which this branch cannot reproduce because import lands on main in parallel. The gate should assert zero for this message class on the flattened oracle files here, and the orchestrator re-checks the project count after merge.

**Not literal typing, so they stay after this phase (state in SUMMARY):** `cannot pass WORD as input parameter (expected UINT)` 15 and `cannot assign WORD to UINT` 1 (typed WORD↔UINT implicit conversion), `boolean operator AND/OR requires BOOL operands` 9 (bitwise on integers), SEMA010 built-ins (ADR, SIZEOF, SHL, conversions), undeclared identifiers/types from flattening. Phase 21 research labelled some of these "Phase 22", but CONTEXT scopes Phase 22 to RUNT-01/02/05/06. See Open Question 1.

Runtime probe (scratch `probe_test.st` run with `stc test`), current behaviour:

```
i : INT := 32767;  i := i + 1;        -> 32768   (want -32768)
u : UINT;          u := u - 1;        -> -1      (want 65535)
arr : ARRAY[1..3] OF INT := [1,2,3];  arr[2] -> 0 (want 2)
sa : ARRAY[1..N] OF ST_A := [...];    sa[1].a -> "array index out of bounds: 1 (length 1)"
```

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib (`encoding/json`, `strconv`, `strings`, `sync`, `math`, `math/bits`) | Go 1.26.0 (local toolchain; go.mod declares 1.25) | JSON coercion, literal parsing, mutex, wrap | Project rule: stdlib only |
| Existing `pkg/ast.ConstIntValue` | in repo | Constant-expression array bounds | Already used by the checker resolver for the same bounds, so interp and checker agree |
| Existing `pkg/interp` `RefPath` | in repo | Path walk for Get/Set | Already handles member/index steps and write-back |

No new packages are installed, so the Package Legitimacy Audit is not applicable.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| AST predicate for "untyped constant" (private to checker) | New `types.KindUntypedInt/Real` in the lattice | A new kind leaks into `IsAnyInt`, hover, emit and every `switch Kind`; dozens of sites must learn it. The AST predicate touches only the 6 literal-rule sites. |
| Current value as coercion witness in `Runtime.Set` | Look up the declared type via `pkg/symtree` | Would make interp depend on symtree→analyzer→checker. The witness approach is sufficient once IECType is correct everywhere (this phase fixes that). |
| Keep direct-index arrays (`high+1` slots) | Store `Low` offset and allocate `high-low+1` | Changing indexing touches every array access, bits, RefPath and tests. Keep direct indexing and add `ArrayLow` metadata only. Negative lower bounds remain unsupported (none in oracles). |

## Architecture Patterns

### System Architecture Diagram

```
 .st files ──parse──> []*ast.SourceFile ──analyzer.Analyze──> AnalysisResult{Symbols, Diags, Files(new)}
                                                              │
                         ┌────────────────────────────────────┤
                         v                                    v
               symtree.Build(result)                 checker (untyped literal adoption)
               Tree{Roots: GVLs, PROGRAMs}           -> diags (85 literal errors gone)
               Lookup/Walk/JSON                       
                         │ (static shape, attrs, enum strings)
                         v
   stc check --symbols [--format json]

 []*ast.SourceFile ──interp.NewRuntime──> register TYPEs, enums, FUNCTIONs, FBs
                                          -> RegisterGVL (constants first; evalInit, const bounds)
                                          -> instantiate every PROGRAM env (evalInit per var)
                                          │
   Go test / stc sim --set ──Set(path,v)──> mutex ─> parse path ─> root (GVL env | PROGRAM env)
                                          │            ─> steps: member | FB env hop | [i] | .bit
                                          │            ─> coerce(v, witness=current value) ─> storeAs
   Tick(dt) ──mutex──> each PROGRAM body (declaration order) ─> evalBinary (typed, wrapped) ─> storeAs (wrap)
                                          │
   Get(path) ──mutex──> same walk ─> Value ─> ToJSON(enum name, TIME {ms, iso}, arrays low..high, structs declared case)
```

### Recommended Project Structure
```
pkg/symtree/
├── symtree.go        # Node, Kind, Tree, Build
├── path.go           # ParsePath (shared grammar; interp copies or imports nothing)
├── lookup.go         # Lookup, lazy array elements, Walk
├── json.go           # Tree.JSON()
└── symtree_test.go
pkg/interp/
├── store.go          # storeAs (replaces adoptEnumTag), wrapInt, literal-operand typing helpers
├── init_value.go     # evalInit(typeSpec, expr), constBounds, instantiateVar (shared by program/GVL/FB)
├── runtime.go        # Runtime, NewRuntime, Tick, Get, Set
├── runtime_path.go   # path parse + resolve against envs
├── coerce.go         # Go any -> Value per witness
└── tojson.go         # Value -> JSON-ready any
pkg/checker/
└── untyped.go        # untypedConst(expr) (kind, value, ok), adopt(), range check
```

### Pattern 1: Single store choke point with wrap (interp)

Replace `adoptEnumTag(dst, v)` with `storeAs(dst, v)` and route the gap sites through it (`writeRef`, initialisers, bit writes return path, FOR counter).

```go
// storeAs converts v for storage in a slot whose current value is dst.
func storeAs(dst, v Value) Value {
	switch {
	case dst.Kind == ValInt && v.Kind == ValInt:
		v.Int = wrapInt(v.Int, dst.IECType)
		v.IECType = dst.IECType
		v.Enum = dst.Enum // existing adoptEnumTag rule
	case dst.Kind == ValReal && v.Kind == ValInt:
		v = Value{Kind: ValReal, Real: float64(v.Int), IECType: dst.IECType}
	case dst.Kind == ValReal && v.Kind == ValReal:
		v.IECType = dst.IECType
	}
	return v
}

func wrapInt(n int64, k types.TypeKind) int64 {
	switch k {
	case types.KindSINT:
		return int64(int8(n))
	case types.KindUSINT, types.KindBYTE:
		return int64(uint8(n))
	case types.KindINT:
		return int64(int16(n))
	case types.KindUINT, types.KindWORD:
		return int64(uint16(n))
	case types.KindDINT:
		return int64(int32(n))
	case types.KindUDINT, types.KindDWORD:
		return int64(uint32(n))
	default: // LINT, ULINT, LWORD: int64 bit pattern; ULINT/LWORD render via uint64(n)
		return n
	}
}
```

Initialised scalars: `val = storeAs(zero, iv)` so `w : WORD := 16#9` keeps IECType WORD (closes 20-05 deferred item).

### Pattern 2: Typed binary results (interp)

In `evalBinary`, compute the result kind before `evalBinaryInt`:
- If one operand AST is an untyped integer literal (`*ast.Literal` with `LitInt`, or unary minus/paren of one), the result kind is the other operand's `IECType`.
- Otherwise the kind is `types.CommonType(l.IECType, r.IECType)`; when that fails (signed vs unsigned, bit vs int) use the wider of the two by size. CODESYS behaviour for mixed signedness was not verified (see Pitfall 2).
- Arithmetic results get `wrapInt(result, kind)` and `IECType: kind`. Comparisons stay BOOL. ULINT/LWORD use `uint64` arithmetic for `/`, `MOD` and comparisons.

### Pattern 3: Untyped constants in the checker

```go
// untypedConst reports whether e is an untyped numeric constant expression:
// an integer or real literal without a type prefix, possibly parenthesised,
// negated, or combined with another untyped constant by + - * / MOD.
// For integers, val is the folded value when it fits int64.
func untypedConst(e ast.Expr) (k untypedKind, val int64, hasVal bool)
```

Adoption sites, all before the existing `CommonType`/`CanWiden` checks:
1. `checkBinaryExpr` (arithmetic and comparison): exactly one side untyped and the other side typed → the untyped side takes the typed side's type when allowed (int → ANY_INT, BYTE..LWORD, ANY_REAL; real → ANY_REAL). Untyped real with a typed integer keeps today's result (LREAL). Both untyped → defaults DINT/LREAL. Place it after `checkEnumBinary` so enum rules still run first.
2. `checkAssignStmt`, FB call args (`check.go:517`), function/method args (`check_calls.go:153`), enum args (`check_enum.go:152`): replace `isLiteralExpr && isLiteralCompatible` with `untypedAssignable(e, target)` and report range errors.
3. `caseLabelCompatible`: an untyped label adopts the selector's type (fixes `CASE byteExpr AND 16#0F OF`).
4. `initLiteralCompatible`: reuse `untypedAssignable` so assignment and initialiser rules are one rule (closes 20-09 deferred item).
5. Range check when the adopted type is integer or bit-string and the value is known: error "constant 300 out of range for BYTE" with SEMA021 at argument sites and SEMA001 at assignment sites (consistent with the surrounding message codes). Negative constant into unsigned is out of range.

Expected type of the expression is not otherwise propagated top-down; bottom-up adoption in the binary node plus the literal rule at sinks is enough for every measured residual (`x / 5.0`, `u + 1`, `(240 + u - v) MOD 240`, `i - 1` as index, `16#02` to BYTE param, `b <> 0`).

### Pattern 4: Symbol tree built from AST order + symbol-table types

```go
package symtree

type Kind int // GVL, Program, FBInstance, Struct, Array, Scalar, Enum, Reference, Pointer

type Node struct {
	Name        string            // declared case; "[3]" for array elements
	Path        string            // "GVL.fb[2].HMI.p_stat_State"
	Kind        Kind
	Type        types.Type
	TypeName    string            // declared type name, e.g. "ST_Drive_HMI", "FB_ATV320"
	Attributes  []*ast.Attribute  // type-level (FB/STRUCT/TYPE decl) then instance-level (VarDecl/member)
	EnumStrings map[int64]string  // enum scalars
	Section     ast.VarSection    // VAR_INPUT/OUTPUT/IN_OUT/VAR for FB children
	Pos         source.Pos
	Low, High   int               // arrays (from types.ArrayDimension; Known required)
	children    []*Node           // nil for arrays until requested
	elemProto   func(i int) *Node // arrays: lazy element factory
}

type Tree struct{ Roots []*Node /* GVLs then PROGRAMs, source order */ }

func Build(res analyzer.AnalysisResult) (*Tree, error)
func (t *Tree) Lookup(path string) (*Node, error) // case-insensitive, synthesises array elements
func (n *Node) Children() []*Node                 // expands arrays lazily
func (t *Tree) Walk(fn func(*Node) bool, opts WalkOpts) // opts.ExpandArrays default false
func (t *Tree) JSON() ([]byte, error)              // arrays emit {low, high, element: <proto>} not every element
```

- Type sources: GVL symbol `.Type.(*types.StructType).Members`; PROGRAM/FB vars via `table.LookupPOU(pou).LookupLocal(name).Type`; struct members from `*types.StructType`; FB children by walking `ast.FunctionBlockDecl.VarBlocks` along `Extends` (base first, derived overrides); standard FBs from `checker` stdlib `FunctionBlockType` Inputs/Outputs.
- Build `map[upperName]*ast.TypeDecl` and `map[upperName]*ast.FunctionBlockDecl` once from `res.Files` + `res.LibraryFiles` for type-level attributes and member declarations.
- Cycle guard: a stack of FB/struct type names on the current path; a repeat yields a leaf with Kind FBInstance/Struct and no children.
- Enum strings: from `types.EnumType.Values`/`Ordinals`, also for enum-typed array elements and alias-to-enum.
- `stc check --symbols`: after diagnostics, print the tree as indented text (`path : TYPE {attrs}`), or JSON under `"symbols"` with `--format json`. Keep diagnostics output unchanged when the flag is absent.

### Pattern 5: Runtime and path resolution

```go
type Runtime struct {
	mu       sync.Mutex
	interp   *Interpreter
	programs []*programRun // name, env, engine; source order
}

func NewRuntime(files []*ast.SourceFile) (*Runtime, error) // registers TYPEs, enums, FUNCTIONs, FBs, GVLs, PROGRAM envs
func (r *Runtime) Tick(dt time.Duration) error            // runs PROGRAM bodies in source order; Phase 23 replaces with tasks
func (r *Runtime) Get(path string) (Value, error)
func (r *Runtime) Set(path string, v any) error
func (r *Runtime) ToJSON(v Value) any
func (r *Runtime) Interpreter() *Interpreter
```

- Add `NewScanCycleEngineWith(interp *Interpreter, prog *ast.ProgramDecl)` so all programs share one interpreter and one GVL layer. Initialise every program at construction ("cycle 0" values visible before the first Tick).
- Path grammar: `root ('.' ident | '[' int ']' | '.' digits)*`. Root: GVL name first (`interp.lookupGVL`), then PROGRAM name; anything else is "unknown root". A trailing `.N` on an integer is bit access (reuse `readBit`/bit write helpers); a multi-index `[i,j]` is an error ("multi-dimensional arrays not supported").
- Steps: env variable → value; `ValFBInstance` user → hop to `inst.Env` (`fbMemberRoot`); stdlib → `GetMember` for read, `SetInput` for inputs, outputs read-only error; `ValStruct` → upper-case key; `ValArray` → direct index with bounds check against `ArrayLow..len-1`; `ValReference` → `readRef`.
- Writes use a `RefPath` rooted at the deepest env (program/GVL/FB env) and `writeRef` with `storeAs` at the leaf.

### Set coercion table (witness = current value at path)

| Witness | Accepted Go input | Rule |
|---------|-------------------|------|
| `ValBool` | `bool`; integer 0/1; `json.Number` 0/1; string `TRUE/FALSE/true/false/1/0` | else error |
| `ValInt`, enum tag set | string value name (`v`, `E.v`, `E#v`, case-insensitive) via `EnumDefs`; integer equal to a declared ordinal | strict enum rejects unknown ordinals |
| `ValInt` | `int*`, `uint*`, `json.Number`, `float64` only if integral, string IEC literal (`16#FF`, `2#1010`, `-3`, `1_000`, `INT#5`) | range check against IECType, error when out of range (no silent wrap on external writes); then `storeAs` |
| `ValReal` | `float64`, `float32`, `int*`, `json.Number`, string (`1.5`, `REAL#1.5`, `1e3`) | REAL range ±3.4028235e38 checked; NaN/Inf rejected |
| `ValString` | `string`; surrounding single quotes stripped, `$` escapes per IEC | no declared length is tracked; document max 255 for STRING |
| `ValTime`/`ValDate`/`ValDateTime`/`ValTod` | string literal (`T#5s`, `TIME#1m`, `D#2026-01-01`, `TOD#12:00`) via existing `parseLitTime`/typed parsers; numbers = milliseconds for TIME | |
| `ValArray` | `[]any` with `len <= High-Low+1`, element i goes to slot `Low+i` | elements coerced recursively; shorter lists leave the tail unchanged |
| `ValStruct` | `map[string]any`, keys case-insensitive | unknown key error; missing keys untouched; recursive |
| `ValFBInstance` (user) | `map[string]any` of variable names | recursive per member; stdlib FB maps only inputs |
| `ValReference` | write-through to target | unbound reference → error |
| `ValPointer` | none | "pointer not writable by path" |

### ToJSON shapes

| Value | JSON |
|-------|------|
| BOOL | `true/false` |
| integers | number; ULINT/LWORD rendered from `uint64(n)` |
| enum-tagged int | value name string when an ordinal matches, else the number |
| REAL/LREAL | number; NaN/Inf as string `"NaN"`, `"+Inf"` |
| STRING | string |
| TIME | `{"ms": 5000, "iso": "PT5S"}` |
| DATE/DT/TOD | IEC literal string |
| ARRAY | list of elements `Low..High` (skip unused direct-index slots) |
| STRUCT | object in declaration order with declared-case keys |
| FB instance | object of its variables in declaration order |

To make arrays and structs render correctly without symtree, add two metadata fields set at instantiation and preserved by `Clone`: `Value.ArrayLow int` and `Value.Fields []string` (declared-case member names in order). Both are read-only metadata, so sharing the slice in `Clone` is safe.

### Pattern 6: Initialisers and constant bounds (interp)

```go
// evalInit evaluates an initialiser against its declared type.
func (interp *Interpreter) evalInit(env *Env, ts ast.TypeSpec, init ast.Expr, zero Value) (Value, error)
```
- `*ast.ArrayInit` on an array type: walk elements, expanding `Count(Value)` repetitions (Count via `ast.ConstIntValue`), place element j at slot `Low+j`; error if more elements than `High-Low+1`; each element through `evalInit` with the element type (nested `ArrayInit` and `StructInit` recurse). Missing tail keeps the element zero/TYPE default.
- `*ast.StructInit` on a struct type: start from the struct zero (which already applies member defaults from the TYPE once member initialisers are honoured), set named fields, unknown field error.
- Anything else: `evalExpr` then `storeAs(zero, v)`.
- Struct TYPE member defaults (`STRUCT a : INT := 5; END_STRUCT`) must apply in `zeroStructWith`: evaluate member `InitValue` there with an env that can see constants.
- Constant bounds: replace `evalSubrangeConst` in the zero builders with `interp.constInt(expr, env)` = `ast.ConstIntValue(expr, lookup)`, where lookup(qual, name) reads `lookupGVL(qual).GetLocal(name)` for qualified names and walks the instantiating env chain for bare names; only `ValInt` values count. The zero builders are currently free functions taking a `TypeResolver`; turn the resolver into a small struct carrying both type resolution and constant lookup.
- Ordering: GVLs register in source order and `RegisterGVL` already gives later GVLs the earlier unqualified chain. For qualified constants referenced across GVLs (`EcDiagParam.MAX_EC_SLAVES` used in `ECT_Diag`), register `VAR_GLOBAL CONSTANT`-only GVLs first, or do a two-pass registration (all constant blocks of all GVLs, then the rest). Library GVLs (stubs, SVNCore as library) must register before project GVLs.
- One shared `instantiateVar(env, fbParent, vb, vd)` used by `initVarDecl` and `newUserFBInstanceDepth` (EXTENDS chain included), with `Clone` per name and `storeAs` on initialisers.

### Anti-Patterns to Avoid
- **Wrapping only in `evalBinaryInt`:** misses `i := 70000` style stores, function returns and FB inputs. Wrap at the store choke point and also in typed binaries.
- **Adding an untyped kind to `types.TypeKind`:** leaks into hover, emit and `IsAnyInt`; keep it inside the checker.
- **Expanding every array element in the symbol tree:** `ARRAY[1..128] OF ST_EcSlaveInfo` across many instances explodes JSON size; keep arrays lazy and emit a prototype.
- **Making `pkg/interp` import `pkg/symtree` or `pkg/analyzer`:** risks an import cycle and contradicts the CONTEXT preference.
- **Silently ignoring initialiser evaluation errors:** today `initVarDecl` drops them. At least surface them as `NewRuntime` errors so a broken initialiser is visible in Get tests.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Constant folding for bounds and repetition counts | A new evaluator | `ast.ConstIntValue(x, lookup)` | Already handles names, `G.C`, parens, unary, `+ - *` with overflow checks; the checker uses it for the same bounds |
| Path walking/write-back into aggregates | A new walker | `RefPath` + `writeRef` + `fbMemberRoot` | Already correct for nested struct/array write-back |
| Enum name ↔ ordinal | New maps | `interp.EnumDefs` / `types.EnumType.Values+Ordinals` | Single source of numbering (`ast.EnumOrdinals`) |
| Literal text parsing in Set | New parsers | `parseLitInt`, `parseLitReal`, `parseLitTime`, `parseLitTyped` | Same lexemes as source code |
| Declaration registration for Runtime | Copy of runner code | Extract a `RegisterFiles(files)` helper in interp and call it from Runtime (runner migration optional) | One place that knows TYPEs, enums, FUNCTIONs, FBs |

## Common Pitfalls

### Pitfall 1: IECType is wrong on many values today
**What goes wrong:** Wrapping at store uses `dst.IECType`; if the slot was initialised from a literal it says DINT and nothing wraps.
**How to avoid:** Fix initialisers first (`storeAs(zero, iv)`) in the same plan as `storeAs`. Add a test that `w : WORD := 16#9` has `IECType == KindWORD` and `w.16` is a runtime error.

### Pitfall 2: Intermediate-width semantics
**What goes wrong:** CONTEXT locks wrapping typed binary results, so `IF i + 1 > i` with `i = 32767` is FALSE. Real CODESYS may compute INT arithmetic in register width (DINT) and only truncate on store `[ASSUMED]`.
**How to avoid:** Implement CONTEXT as written; record the CODESYS register-width question as a deferred item for a TwinCAT oracle check. Tests should pin store-wrap cases, which are unambiguous.

### Pitfall 3: ULINT/LWORD in int64
**What goes wrong:** Values above 2^63 are negative int64; division, MOD, comparisons and JSON go wrong.
**How to avoid:** Use `uint64` paths for those kinds in compare/div/mod and ToJSON. Keep a test with `16#FFFF_FFFF_FFFF_FFFF`.

### Pitfall 4: New range errors in existing corpora
**What goes wrong:** Range checks on adopted literals can produce new errors (`b : BYTE := 256;`, `u := -1;`) in `tests/` corpora or oracle files.
**How to avoid:** Run `go run ./cmd/stc check` over `tests/**/*.st` and both oracle files before and after, and diff error buckets. Every new error must be a genuine out-of-range constant.

### Pitfall 5: Constant-bound arrays sized 1 today
**What goes wrong:** `ARRAY[1..GVL.N]` silently has one slot; tests that pass today may depend on that (unlikely, but index errors would surface).
**How to avoid:** Implement constant bounds and lift or keep the 10000 cap explicitly (`MAX_EC_SLAVES = 128`, oracle max is small); report bound evaluation failure as a NewRuntime error rather than falling back to 1.

### Pitfall 6: GVL constant ordering across GVLs
**What goes wrong:** `ECT_Diag` references `EcDiagParam.MAX_EC_SLAVES`, declared in another GVL (in SVNCore, a library). If `ECT_Diag` registers first, the bound is unresolved.
**How to avoid:** Two-pass GVL registration (constants of all GVLs, then everything), library files before project files.

### Pitfall 7: Struct keys lose case and order
**What goes wrong:** `Value.Struct` keys are upper-case and map order is random, so ToJSON output would be non-deterministic and unlike TwinCAT names.
**How to avoid:** `Value.Fields` metadata (declared case, order) and sorted fallback. JSON determinism is a CLAUDE.md constraint.

### Pitfall 8: Merge conflicts with Phase 21 on main
**What goes wrong:** Phase 21 also edits `tests/twincat_probes_test.go` and possibly `cmd/stc/check.go`, `pkg/analyzer`.
**How to avoid:** Keep edits to those files minimal and isolated (one allow-list removal, one flag registration, one result field). The final plan's gate runs after merging main into the branch.

### Pitfall 9: Comparison typing of untyped constants
**What goes wrong:** `b <> 0` with BYTE `b` must not become `CommonType(BYTE, DINT)` = fail; `u - 1 < 0` with UINT becomes always FALSE after wrap.
**How to avoid:** Adopt in comparisons too (locked). Accept the wrap consequence (it matches typed UINT arithmetic).

## Code Examples

### storeAs replacing adoptEnumTag call sites
```go
// pkg/interp/interpreter.go:671 (assignToTarget, Ident case)
val = storeAs(existing, val)
// pkg/interp/ref_path.go writeRef leaf
cur.Array[last.Index] = storeAs(cur.Array[last.Index], val)
```

### Constant lookup for bounds
```go
func (interp *Interpreter) constLookup(env *Env) func(qual, name string) (int64, bool) {
	return func(qual, name string) (int64, bool) {
		var v Value
		var ok bool
		if qual != "" {
			g := interp.lookupGVL(qual)
			if g == nil {
				return 0, false
			}
			v, ok = g.GetLocal(name)
		} else if env != nil {
			v, ok = env.Get(name)
		}
		if !ok || v.Kind != ValInt {
			return 0, false
		}
		return v.Int, true
	}
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Literal typed DINT/LREAL, special-cased only when the whole RHS is a literal | Untyped constant adopts context type (Go-style untyped constants) | This phase | Removes 85 oracle errors, unifies initialiser and assignment rules |
| int64 arithmetic without wrap | Wrap to IEC width at store and typed binaries | This phase | Matches PLC overflow behaviour |
| Initialisers for aggregates ignored | Type-directed `evalInit` | This phase | Cycle-0 state matches TwinCAT download state |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | CODESYS may evaluate INT arithmetic at register width and truncate only on store | Pitfall 2 | Expressions comparing an overflowing intermediate differ from the PLC; store results are unaffected |
| A2 | External `Set` of an out-of-range number should be rejected rather than wrapped | Coercion table | If users expect wrap, Set errors instead; easy to relax |
| A3 | Mixed signed/unsigned typed operands at runtime take the wider operand's kind | Pattern 2 | Rare in oracles; result width may differ from TwinCAT |
| A4 | Running all PROGRAM bodies in source order per Tick is acceptable until Phase 23 tasks | Pattern 5 | Only affects multi-PROGRAM projects; Phase 23 replaces it |
| A5 | STRING length is not tracked, so Set does not enforce declared `STRING(n)` | Coercion table | Over-long strings accepted; Phase 28 may need lengths from symtree |

## Open Questions (RESOLVED)

1. **Do WORD↔UINT implicit conversions, bitwise AND/OR on integers and the missing built-ins (ADR, SIZEOF, SHL, conversions) belong to this phase?**
   - What we know: Phase 21 research labels them "Phase 22", but CONTEXT limits Phase 22 to RUNT-01/02/05/06 and only lists literal typing for the checker.
   - RESOLVED: Out of scope. The gate asserts only the literal-typing class (85 → 0). The final SUMMARY lists the remaining buckets by name with counts so the orchestrator can route them.

2. **Where does the "untyped" notion live?**
   - RESOLVED: A private AST predicate in `pkg/checker/untyped.go`; no new `types.TypeKind`. The interpreter uses its own tiny "is untyped literal operand" check in `evalBinary`.

3. **How does `Runtime.Set` know the declared type without symtree?**
   - RESOLVED: The current value at the path is the witness (Kind, IECType, Enum tag, ArrayLow, Fields). This is reliable once initialisers use `storeAs`. `pkg/symtree` stays independent of interp.

4. **How do arrays with non-zero lower bounds render and accept list writes?**
   - RESOLVED: Keep direct indexing and add `Value.ArrayLow`. Get/Set/ToJSON use `ArrayLow..len-1`. Negative lower bounds stay unsupported (none in the oracles) and produce an instantiation error instead of a silent wrong size.

5. **How is "cycle 0" defined for `Get`?**
   - RESOLVED: `NewRuntime` instantiates every GVL and PROGRAM env eagerly. `Get` before any `Tick` returns initialiser values.

6. **Which CLI surface prints the tree?**
   - RESOLVED: `stc check --symbols` (text and `--format json`), no new top-level command. `stc sim` gains repeatable `--set PATH=VALUE` (applied after instantiation, before cycle 1) and `--get PATH` (read after the last cycle, printed as `"get": {path: json}` in JSON mode). sim builds a `Runtime` from the parsed file so user FBs, types and enums work.

7. **Range errors: which code?**
   - RESOLVED: Reuse SEMA001 at assignment/initialiser sites and SEMA021 at argument sites, with message `constant <n> out of range for <TYPE>`. No new code number.

8. **ST301 success criterion 1 needs `GVL.fb[2].HMI.p_stat_State` but the flattened st301 depends on SVNCore.**
   - RESOLVED: Commit a reduced, anonymised fixture under `tests/runtime/` (ST301-shaped: `EcDiagParam` constant GVL, `ECT_Diag` struct-array initialiser over `ARRAY[1..EcDiagParam.MAX_EC_SLAVES]`, `FB_ATV320`-like FB with `HMI : ST_Drive_HMI`, `Drives_Line1 : ARRAY[1..2] OF FB_ATV320`, enum `hmis_e` with `to_string`, `OPC.UA.DA` attributes). The oracle test additionally builds the tree from `svncorecomponents.st` + `st301.st` together when `STC_PROBES_DIR` is set and logs a sample of paths.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test | ✓ | go1.26.0 darwin/arm64 | — |
| Oracle sources (`STC_PROBES_DIR`) | oracle gate | ✓ locally at `/Users/jonb/Projects/beckhoff-docs/stc-probes` | — | Test skips when unset (CI) |
| python3 | measurement scripts only | ✓ | — | not needed by plans |

No missing blocking dependencies.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | `go test` (testify already used) + `stc test` ST suites |
| Config file | `.testcoverage.yml`, `scripts/coverage-gate.sh` |
| Quick run command | `go test ./pkg/interp ./pkg/checker ./pkg/symtree -count=1` |
| Full suite command | `go test ./... -count=1 && go run ./cmd/stc test tests/ && bash scripts/coverage-gate.sh` |
| Oracle command | `STC_PROBES_DIR=/Users/jonb/Projects/beckhoff-docs/stc-probes go test ./tests -run 'TwinCAT' -count=1 -v` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RUNT-05 | `wrapInt` table for all 12 integer kinds, both directions | unit | `go test ./pkg/interp -run TestWrapInt -count=1` | ❌ Wave 0 (`store_test.go`) |
| RUNT-05 | INT 32767+1 = -32768, UINT 0-1 = 65535, SINT/USINT/BYTE/WORD/DWORD/UDINT/LINT/ULINT/LWORD wrap, div by zero error, REAL unchanged | unit + ST | `go test ./pkg/interp -run TestIntegerWrap -count=1`; `go run ./cmd/stc test tests/type_system/` | ❌ Wave 0 (`tests/type_system/wrap_test.st`) |
| RUNT-05 | `w : WORD := 16#9` keeps IECType WORD; `w.16` runtime error | unit | `go test ./pkg/interp -run TestInitialisedIECType -count=1` | ❌ |
| RUNT-05 | `a := a + 1` on INT, `u := u + 1` on UDINT, `x / 5.0` REAL, `16#02` to BYTE param, `CASE b AND 16#0F OF`, `b <> 0`, `arr[i - 1]` with UINT i check clean; `b := 300` errors | unit | `go test ./pkg/checker -run TestUntypedLiteral -count=1` | ❌ (`untyped_test.go`) |
| RUNT-05 | dialect gate without the allow-list | integration | `go test ./tests -run TestTwinCATDialectCheck -count=1` | ✅ (edit) |
| RUNT-05 | literal class 85 → 0 on oracles | oracle | Oracle command above, new subtest asserting zero for the literal message set | ✅ (extend `twincat_probes_test.go`) |
| RUNT-06 | ArrayInit, repetition, nested, StructInit with omitted fields, TYPE member defaults, alias/enum defaults, FB VAR per instance incl. EXTENDS, constant bounds `G.C`, `C*2`, `G.C - 1` | unit | `go test ./pkg/interp -run 'TestInit|TestConstBounds' -count=1` | ❌ (`init_value_test.go`) |
| RUNT-01 | Tree roots, paths, declared case, case-insensitive Lookup, lazy arrays, enum strings, merged attributes, cycle guard, stdlib FB children, JSON determinism | unit | `go test ./pkg/symtree -count=1` | ❌ (`symtree_test.go`) |
| RUNT-01 | `stc check --symbols` text and JSON | CLI exec | `go test ./cmd/stc -run TestCheckSymbols -count=1` | ❌ |
| RUNT-02 | Get/Set by path on fixture: `Set("GVL.x.p_cmd_Start", true)`, enum name, `json.Number`, `T#5s`, `16#FF`, struct map, array list, bit `.3`, errors for unknown path/out of range/pointer | unit | `go test ./pkg/interp -run TestRuntime -count=1` | ❌ (`runtime_test.go`) |
| RUNT-02/06 | `Get("ECT_Diag.Device_1_SlaveInfo[3].p_stat_sName")` at cycle 0 on ST301-shaped fixture | integration | `go test ./tests -run TestRuntimeFixture -count=1` | ❌ (`tests/runtime/`) |
| RUNT-02 | `stc sim --set P=V --get P --format json` | CLI exec | `go test ./cmd/stc -run TestSimSetGet -count=1` | ❌ |
| RUNT-02 | Concurrent Get/Set/Tick under `-race` | unit | `go test -race ./pkg/interp -run TestRuntimeConcurrent -count=1` | ❌ |

### Sampling Rate
- **Per task commit:** the quick run command for the touched package.
- **Per wave merge:** full suite command.
- **Phase gate:** full suite + oracle command + `bash scripts/coverage-gate.sh` green; add `^pkg/symtree$` threshold 95 to `.testcoverage.yml`.

### Wave 0 Gaps
- [ ] `pkg/interp/store_test.go`, `init_value_test.go`, `runtime_test.go`
- [ ] `pkg/checker/untyped_test.go`
- [ ] `pkg/symtree/symtree_test.go`
- [ ] `tests/runtime/st301_shape.st` fixture + `tests/runtime_fixture_test.go`
- [ ] `tests/type_system/wrap_test.st`
- [ ] `.testcoverage.yml` entry for `pkg/symtree`

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | partial | Set rejects pointers and stdlib FB outputs; CONSTANT GVL members rejected by Set (check `GVLInfo.Constants` or a `const` flag on the env entry) |
| V5 Input Validation | yes | Path grammar parser with explicit errors; range checks; no panics on any input (fuzz `ParsePath` and `Set`) |
| V6 Cryptography | no | — |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path with huge index or deep nesting from a remote client (Phase 28/29) | DoS | Bounds check before allocation; lazy tree arrays; reject paths over a fixed length |
| Writes racing the scan | Tampering | Single mutex around Tick/Get/Set; `-race` test |
| Writes to `VAR_GLOBAL CONSTANT` members | Tampering | Reject in Set with a clear error |
| Oracle source leakage in logs | Information disclosure | Keep using `templateMessage` for oracle logs (T-19-18, T-20-22) |

## Recommended Plan Split (5 plans, 3 waves)

| Plan | Wave | Scope | Requirements | Files |
|------|------|-------|--------------|-------|
| 22-01 Interp value semantics | 1 | `storeAs` + `wrapInt` replacing `adoptEnumTag` (7 sites + `writeRef` + bit write + FOR); typed binary results with literal-operand adoption; ULINT/LWORD uint64 paths; initialised scalars via `storeAs(zero, iv)`; update interp tests that pin DINT | RUNT-05 | `pkg/interp/store.go`, `interpreter.go`, `enum.go`, `ref_path.go`, `bits.go`, `scan.go`, `fb_instance.go`, tests, `tests/type_system/wrap_test.st` |
| 22-02 Checker untyped literals | 1 | `untypedConst`, adoption in binary/compare, assignment, FB/function/enum args, CASE labels, initialisers (unify `initLiteralCompatible`), range errors; remove dialect allow-list; corpora diff | RUNT-05 | `pkg/checker/untyped.go`, `check.go`, `check_calls.go`, `check_enum.go`, `check_init.go`, `tests/twincat_dialect_check_test.go` |
| 22-03 Symbol tree | 1 | `AnalysisResult.Files/LibraryFiles`; `pkg/symtree` Build/Lookup/Walk/JSON, lazy arrays, merged attributes, enum strings, stdlib FB children; `stc check --symbols`; coverage entry | RUNT-01 | `pkg/analyzer/analyzer.go`, `pkg/symtree/*`, `cmd/stc/check.go`, `.testcoverage.yml` |
| 22-04 Initialisers and constant bounds | 2 (after 22-01) | `evalInit` for ArrayInit/StructInit/repetition/nesting; struct TYPE member defaults; `ArrayLow`/`Fields` metadata; constant bounds via `ast.ConstIntValue` with GVL/env lookup; two-pass GVL registration; shared `instantiateVar` for program/GVL/FB (EXTENDS) | RUNT-06 | `pkg/interp/init_value.go`, `fb_instance.go`, `scan.go`, `gvl.go`, `value.go` |
| 22-05 Runtime Get/Set + gate | 3 | `NewScanCycleEngineWith`, `RegisterFiles`, `Runtime` (mutex, Tick, Get, Set, ToJSON), coercion table, path parser, `stc sim --set/--get`, ST301-shaped fixture, oracle literal-class zero assertion, symtree-vs-runtime path agreement test (every leaf path from the tree resolves in the Runtime), coverage gate | RUNT-02 (+ SC1, SC2, SC4 end to end) | `pkg/interp/runtime*.go`, `coerce.go`, `tojson.go`, `pkg/sim/engine.go`, `cmd/stc/sim_cmd.go`, `tests/runtime/*`, `tests/twincat_probes_test.go` |

## Sources

### Primary (HIGH confidence)
- Repository code at HEAD 8d13297 (worktree `/Users/jonb/Projects/stc-wt-22`), file:line references above.
- Measured `stc check --format json` on `/Users/jonb/Projects/beckhoff-docs/stc-probes/{svncorecomponents,st301}.st`.
- Runtime probe with `stc test` on a scratch file (results quoted above).
- `.planning/phases/20-twincat-expression-semantics/deferred-items.md`, `.planning/research/v1.2/ANALYSIS.md` §2-3, `.planning/phases/21-*/21-RESEARCH.md` (residual buckets).

### Tertiary (LOW confidence)
- CODESYS intermediate arithmetic width (A1): training knowledge only, not verified.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH (stdlib only, existing helpers verified in code)
- Architecture: HIGH (choke points and gaps located by file:line; probe confirms behaviour)
- Pitfalls: MEDIUM-HIGH (A1/A3 are TwinCAT-behaviour assumptions)

**Research date:** 2026-10-06
**Valid until:** 2026-11-05 (or until Phase 21 merges into this branch; re-measure the project-mode count then)
