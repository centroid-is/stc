# Phase 20: TwinCAT Expression Semantics - Research

**Researched:** 2026-10-06
**Domain:** Go compiler front end and interpreter for IEC 61131-3 ST (lexer, Pratt parser, two-pass checker, tree-walking interpreter), CODESYS/TwinCAT dialect
**Confidence:** HIGH for code paths and oracle numbers (measured in this session); MEDIUM for TwinCAT semantics (Beckhoff InfoSys); LOW items are tagged `[ASSUMED]`

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Bit access (DIAL-04)
- New `ast.BitAccessExpr{Target Expr, Index Expr}`; the parser produces it when a postfix `.` is followed by an integer literal (after identifiers, member chains and array indexing: `w.3`, `ECT.X.q_wDigitalInputs.0`, `Modbus.arr[0].3`). The index may also be an identifier that resolves to a CONSTANT (CODESYS allows it); anything else is a parse error.
- Checker: the target type must be BYTE/WORD/DWORD/LWORD or any integer type (SINT..ULINT); BOOL/REAL/STRING/struct targets are an error (new SEMA035 "bit access on non-integer type"). A literal index >= the bit width is an error (SEMA035 as well, distinct message). The expression type is BOOL and it is assignable.
- Interpreter: read returns bit `Index` of the integer value; write sets/clears the bit on the target lvalue (member, array element or GVL member), using the existing lvalue write path. Bit numbering is LSB = 0.
- fmt/emit print `target.N` unchanged; JSON kind `BitAccess`.

#### Named arguments in function-call expressions (DIAL-06)
- `parseCallArgs` is shared: in expression position a call may contain `name := expr` and `name => target` arguments (CODESYS allows VAR_OUTPUT binding on function calls). Mixing positional and named is allowed only as "positional first, then named"; duplicates or a named-then-positional sequence are checker errors (SEMA024 family).
- Checker binds named args to the FUNCTION's VAR_INPUT/VAR_IN_OUT by name and `=>` to VAR_OUTPUT; unknown names error; omitted inputs take their declared default. Return type is the function's declared type as today.
- Interpreter: `evalCall` binds by name; `=>` targets are written after the call returns.
- Also fix the trailing comma before `)` in any call (`f(a := 1, )`): accepted and dropped; fmt prints without it.

#### Qualified enums, base types, enum attributes (DIAL-07)
- `types.EnumType` gains `Base types.Type` (default INT) parsed from `( ... ) UINT;` and `Qualified bool` from `{attribute 'qualified_only'}` on the TYPE. Enum values are typed by `Base`.
- Resolution: `E.v` resolves anywhere an expression is allowed (assignments, comparisons, CASE labels, label lists `E.a, E.b:`, ranges `E.a..E.c:`, initialisers, call arguments). Bare `v` resolves only when the enum is not `qualified_only`; bare use of a qualified_only enum value is an error (new SEMA036). Duplicate bare value names across non-qualified enums remain the existing error.
- `strict`: arithmetic on enum values and implicit conversion between an enum and an integer are errors (SEMA036 message variant); comparisons and assignments between the same enum type stay fine; explicit `TO_INT`/`UINT_TO_...` conversions stay allowed.
- `to_string`: `TO_STRING(e)` returns the value name at runtime (interp built-in); without the attribute it returns the numeric text, as CODESYS does.
- CASE on enum values compares the underlying integer; the interpreter's qualified-enum lookup is implemented in `evalMemberAccess` by checking `EnumTypes` before treating the left side as a variable.

#### REF=, THIS^, SUPER^ and inherited scope (DIAL-09)
- `r REF= x;` is a statement (`ast.RefAssignStmt`) valid only when `r` is `REFERENCE TO T` and `x` is an lvalue of type T; afterwards `r` reads and writes through to `x` (the existing Reference value kind). `REF=` to a non-lvalue is SEMA error.
- `THIS^` evaluates to the current FB instance inside FB bodies, methods, properties and actions; `THIS^.x` and `THIS^.M()` work; using THIS outside an FB is an error. `SUPER^.M()` calls the parent FB's method (and `SUPER^()` runs the parent body); requires EXTENDS.
- The checker now puts inherited methods, properties and variables of the EXTENDS chain into the derived FB's scope (fixes the deferred "inherited methods not in derived scope" item) so `SUPER^` and unqualified inherited calls check. Interp reuses `fbDeclChain` from Phase 19.
- Unqualified METHOD calls inside an FB body/action (`Inc();`) must execute at runtime (closes the 19-08 deferred item): `evalCall` dispatch order becomes action, method-of-current-FB, ADR, REF, LocalFunctions, stdlib, zero-arg FB instance.

#### Checker knows the standard FBs; undeclared types are errors (RUNT-08)
- Register the ten IEC FBs as FB types in the symbol table at analyzer start (TON/TOF/TP: IN, PT → Q, ET; CTU: CU, RESET, PV → Q, CV; CTD: CD, LOAD, PV → Q, CV; CTUD: CU, CD, RESET, LOAD, PV → QU, QD, CV; R_TRIG/F_TRIG: CLK → Q; SR: SET1, RESET → Q1; RS: SET, RESET1 → Q1) with the types used by `pkg/interp/stdlib_timers.go` etc. Member access `t.Q`, `t.ET` type-checks; named calls `t(IN := x, PT := T#1s)` check; wrong parameter names stay SEMA024.
- `resolveTypeSpec` no longer manufactures an empty placeholder FB for an unknown name: an undeclared type name is a new error SEMA037 ("undeclared type 'X'"), except names that come from loaded vendor stub libraries or `.library_paths`. Forward references within the analysis unit must still resolve (two-pass), including the deferred GVL pass from Phase 19.
- Namespace-qualified type names `Lib.Type` (e.g. `Tc2_EtherCAT.ST_EcSlaveState`) parse into a `TypeRef` with a namespace; resolution tries the qualified symbol, then falls back to the bare `Type`; if neither exists it is SEMA037.

#### Residual parse gaps owned for DIAL-10
- Initialisers: `:= (a := 1, b := 'x')` struct initialiser and `:= [(a := 1), (a := 2)]` array-of-struct initialiser, `:= [3(0)]` repetition, nested arrays, with constant-expression bounds `ARRAY[1..GVL.CONST]` parsing (evaluation of bounds and applying values at runtime is Phase 22). They must type-check field names and element counts where statically known.
- Typed based literals `BYTE#16#10`, `WORD#2#1010`, `DWORD#8#17`, plus `UINT#5` already supported.
- An empty declaration/statement `;;` is tolerated (lexer/parser skip an empty statement, no diagnostic in declarations; a lint warning is fine).
- Re-run the Phase 19 hand-off tests with timers and edge triggers (`pkg/checker/action_test.go`, `tests/twincat_dialect/action_test.st`, `empty_args_test.st`, `pkg/interp/action_test.go`, and `stc check tests/twincat_probes/action_inside.st`).

### Claude's Discretion
- Exact SEMA code numbering beyond the names above, diagnostic wording, AST field names.
- Whether `strict` enum arithmetic is an error or a vendor-profile warning for the portable target.
- How `TO_STRING` on enums interacts with the existing string conversion functions.
- Coverage strategy per package (gates: parser/lexer/interp/types/emit >= 95%, checker >= 94%, total >= 85%; after Phase 19: parser 96.93%, checker 95.92%, interp 96.72%, emit 97.16%).

### Deferred Ideas (OUT OF SCOPE)
- Untyped integer literal typing (`n := n + 1` on INT is DINT→INT today) and integer wrap: Phase 22 (RUNT-05).
- Applying array/struct initialisers at runtime and evaluating constant-expression bounds: Phase 22 (RUNT-06).
- METHOD bodies are never checked (deferred from 19-07): consider in Phase 22/23 unless cheap while adding inherited scope.
- Pointer arithmetic, `__NEW`/`__DELETE`, LTIME, UNION, BIT type: not in v1.2.
- TcPOU/TcGVL/TcDUT XML import and library placeholder resolution: Phase 21.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DIAL-04 | Bit access `x.N` read/write on BYTE/WORD/DWORD/LWORD, struct members, array elements; checker bounds | Parser slot: `parsePostfix` Dot case (`pkg/parser/expr.go:298`). The lexer already emits `Dot` + `IntLiteral` for `w.3`. Interp writes through `assignToTarget` (`pkg/interp/interpreter.go:643`). Beckhoff type list and range error confirmed. Bucket: st301 192 diagnostics on 64 lines, svncore 177 on 26 |
| DIAL-06 | Named args in FUNCTION calls used as expressions | `isNamedArgCall` early return (`expr.go:399`) is the root cause. User FUNCTIONs execute only through positional closures in `pkg/testing/runner.go:357-430`. Most probe cases are unqualified METHOD calls, not FUNCTION calls |
| DIAL-07 | Qualified enums, CASE labels and lists, base types, `strict`/`to_string` | `isCaseLabelStart` (`stmt.go:303`) rejects `Ident Dot Ident :`. `parseEnumType` (`types.go:248`) stops at `)`. `ast.EnumType.BaseType` exists but is never set or printed. The checker already resolves `E.v`, while the interpreter does not. Inline VAR enums are never registered |
| DIAL-09 | `REF=`, `THIS^`, `SUPER^` | `KwThis`/`KwSuper` are lexed but `parsePrimaryExpr` and `parseStatement` ignore them. `ValReference` only targets a named variable, but every real `REF=` targets a member path. Index and member writes overwrite reference variables |
| DIAL-10 | Zero parse errors on flattened ST301 + SVNCoreComponents | Measured 917 / 615 diagnostics, classified below by construct, line and fix site |
| RUNT-08 | Checker knows the 10 standard FBs; undeclared types are errors | Tc2_Standard confirms the CODESYS names (RESET, LOAD, SET1, RESET1). The interpreter implements the IEC names (R, LD, S1, R1), so aliases are needed on both sides. The forward-reference placeholder bug causes about 460 false "has no member" errors in svncore |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Go stdlib only for compiler core. No new dependencies; this phase needs none.
- No Java runtime. Parser must keep producing partial ASTs from broken code (error recovery is essential for LSP).
- Determinism: all test execution deterministic, no wall-clock.
- Every CLI command supports `--format json`. New AST nodes need JSON kinds (`pkg/ast/json.go`) and the `stc parse --format json` contract must stay backward compatible where possible.
- Must handle CODESYS extensions (OOP, pointers, 64-bit types).
- GSD workflow: edits happen inside `/gsd:execute-phase`.
- User memory: always ship through GitHub PRs with multi-platform CI (macOS, Windows, Linux) and agent PR review. Never ship without full branch coverage verified in CI.

## Summary

Phase 20 is mostly front-end plumbing in an existing, well-tested codebase. There are no new libraries. The oracle confirms the Phase 19 buckets exactly: `stc parse` reports 917 diagnostics on st301.st (202 distinct lines) and 615 on svncorecomponents.st (206 lines). Every remaining diagnostic maps to one of 11 constructs with a single fix site each (table in "Oracle Classification"). Struct/array initialisers (st301, 700 diagnostics) and bit access dominate. Qualified enum CASE labels are a one-function lookahead fix (`isCaseLabelStart`) that removes 326 svncore diagnostics.

The real risk is in the checker, not the parser. `stc check` on svncore today reports 1745 errors, but most are checker artefacts, not genuine problems. The biggest single cause is the forward-reference placeholder in `resolveTypeSpec` (`pkg/checker/resolve.go:537`). Any TYPE or FB used before its declaration in file order becomes an empty `FunctionBlockType`. That produces about 460 "type X has no member" errors in svncore and "cannot assign dir_e to dir_e". RUNT-08's SEMA037 change must therefore be paired with a real two-pass resolver that pre-registers every type name with a pointer-stable type object. A naive SEMA037 would turn those forward references into hundreds of false errors. The stdlib FBs account for about 230 errors in svncore and 66 in st301. Inline VAR enums (`eStep : (E_IDLE, ...)`) never register their values, which accounts for about 60 SEMA010 errors in svncore.

The interpreter needs three structural changes. First, a shared argument binder for named and positional arguments plus `=>` outputs, used by FUNCTION, METHOD and unqualified method calls; this means moving `callUserFunction` from `pkg/testing` into `pkg/interp`. Second, a current-FB pointer on `Env` for `THIS^`, `SUPER^` and unqualified method dispatch. Third, path-based references, because every real `REF=` in the probes targets a member path (`settings.p_stat_Batches`). The current `ValReference` only targets a whole variable.

**Primary recommendation:** Land an AST-contract plan first. Then run parser work (expressions/statements and declarations/lexer, in separate files) in parallel. Then run checker-resolver, checker-expression and interpreter plans. Finish with a gate plan that adds the ST suites, removes the probe allowances and makes the oracle assert zero parse errors.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Typed based literal `BYTE#16#10` | Lexer (`pkg/lexer`) | Parser/interp (already split prefix and value) | Token boundary bug: `scanLiteralValue` stops at the second `#` |
| Bit access, named call args, trailing comma, THIS/SUPER, qualified CASE labels | Parser expr/stmt (`pkg/parser/expr.go`, `stmt.go`) | AST, fmt, emit, JSON | Postfix chain and statement dispatch |
| Initialisers, enum base type, namespace TypeRef, `;;` in VAR | Parser declarations (`pkg/parser/types.go`, `var.go`, `decl.go`) | AST, fmt, emit | Declaration grammar |
| Stdlib FB signatures, SEMA037, forward refs, inherited scope, inline enum values, enum attributes | Checker resolver (`pkg/checker/resolve.go`) | `pkg/types`, `pkg/symbols` | Pass 1 owns symbol registration |
| Bit-access typing, named-arg binding, strict enums, REF= and THIS checks, initialiser field checks, reference auto-deref | Checker body (`pkg/checker/check.go`) | `pkg/types/builtin.go` (TO_STRING) | Pass 2 expression typing |
| Bit read/write, arg binding, method dispatch, qualified enum eval, TO_STRING, REF= paths, THIS/SUPER | Interpreter (`pkg/interp`) | `pkg/testing/runner.go` (registration) | Runtime semantics |
| Probe gate, ST suites | `tests/` | CI workflows | Acceptance |

## Oracle Classification (measured 2026-10-06, binary built from branch HEAD 17fbee8)

`/tmp/stc-probe20 parse` totals: st301.st 917 diagnostics on 202 lines; svncorecomponents.st 615 on 206 lines. Bucket sizes match 19-09-SUMMARY. The fix-site column was verified by reading the code.

| Bucket | st301 diag/lines | svncore diag/lines | Example (file:line) | Fix site |
|---|---|---|---|---|
| Struct/array initialisers | 700 / 130 | 17 / 1 | st301:642 `ARRAY[1..EcDiagParam.MAX_EC_SLAVES] OF ST_EcSlaveInfo := [ (p_stat_sName := ...), ...];`, svncore:2219 `fbTime : FB_LocalSystemTime := (bEnable := TRUE, dwCycle := 1);` | `parseVarDecl` init (`var.go:132`) and `parseStructMember` init (`types.go:211`): new `parseInitializer`. Bounds `1..EcDiagParam.MAX_EC_SLAVES` already parse (the error is at col 80, the `[`) |
| Bit access | 192 / 64 | 177 / 26 | st301:1522 `ECT.EPW01_WA01_FD01.q_wDigitalInputs.0`, svncore:3683 `OL1R.0 := i_xRelay1;`, svncore:3739 `create_cmd.8 := NOT run;`, svncore:4096 VAR init `BOOL := i_uStatusWord.0;` | `parsePostfix` Dot case: `IntLiteral` after Dot gives BitAccessExpr |
| Qualified enum CASE labels / lists | 0 | 326 / 161 | svncore:3760 `lft_e.eef1:`, svncore:4193 `states.ready_to_switch_on, states.switched_on:` | `isCaseLabelStart` (`stmt.go:303`): accept `Ident (Dot Ident)*` followed by `:`, `,` or `..`. The first label of a CASE already parses; only subsequent branches fail |
| Named args in expression calls | 12 / 2 | 70 / 9 | st301:3595 `find_order(prio := fetches, t := ET_WagonStationType.source)`, svncore:3700 `create_cmd(ratherHalt := ..., freq := ...)`, svncore:3738 `transition(current_state := q_state, transition_action.run, i_xAutoResetAllowed := ...)` | `parsePostfix` LParen: stop returning at `isNamedArgCall` except at statement head |
| Trailing comma before `)` | 10 / 5 | 2 / 1 | st301:2634 `SPB03.speedBatcher(..., q_xDropComplete => x,\n);`, svncore:4236 | `parseCallArgs` (`stmt.go:391`): break when `,` is followed by `)` |
| Enum base type | 0 | 10 / 5 | svncore:1775 `) USINT;`, 5042/5078/5220/5237 `) UINT;` | `parseEnumType` (`types.go:248`) after `)` |
| Namespace-qualified type | 0 | 7 / 1 | svncore:3591 `ec: Tc2_EtherCAT.ST_EcSlaveState;` (also st201:1143 `SVNCoreComponents.ST_LineRecipe`) | `parseNamedTypeOrSubrange` (`types.go:304`) |
| Typed based literal | 0 | 5 / 1 | svncore:1934 `SHL(BYTE#16#10, nPort)` | `scanIdentOrKeyword`/`scanLiteralValue` (`lexer.go:331,365`) |
| `;;` empty declaration | 3 / 1 | 0 | st301:1273 `rDropPoint: REAL;;` | `parseVarBlock` loop (`var.go:40`): skip stray `;` |
| `REF=` | 0 | 1 / 1 | svncore:980 `batches REF= settings.p_stat_Batches;` | `parseAssignOrCall` (`stmt.go:109`) |
| THIS^ / SUPER^ | 0 | 0 | none in gate files; baader:2544 `SendString := THIS^.SendBytes(pData := ADR(str), nLen := LEN(str));` | `parsePrimaryExpr` and `parseStatement` |

**Expected result:** with all 11 fixes, both gate files reach 0 parse diagnostics. No "other" bucket exists in either gate file.

**Secondary probes (not gated, same directory):** st101 (661), st201 (598) and baader (345) show the same buckets plus two constructs not on the Phase 20 list:
- **Chained assignment** `gateLeft.i_xReset := gateRight.i_xReset := ECT.ST207_A1_02.I1;` (st201:1340). It is not in a gate file. Record it as deferred.
- **Simple array initialisers** `ARRAY [1..10] OF BOOL := [true, false, ...]` (baader:983, st301:3500). These are covered by the initialiser work.

**Small probe files** in the probes directory are ideal committed fixtures: `case.st`, `enum.st`, `fcall.st` and `ptr.st` exercise exactly CASE label lists, enum base type with strict/qualified_only, named FUNCTION call and `REF=`. Copy them into `tests/twincat_probes/`. They are 8-12 lines each, synthetic, and contain no customer code.

### `stc check` today: errors that are NOT parse errors

st301 total 2384 errors; svncore 1745. Classified (top buckets):

| Cause | svncore | st301 | Genuine? | Owner |
|---|---|---|---|---|
| Forward-referenced TYPE/FB resolved to an empty placeholder FB ("type ST_Drive_HMI has no member", "FB_Parameter has no input parameter", "cannot assign dir_e to dir_e") | about 460 (ST_Drive_HMI 105, FB_Parameter 92, ST_EtherCATLink_HMI 66, ST_EcSlaveDiag 38, ...) | 32 (ST_WagonStation) | No, checker bug | Phase 20, RUNT-08 two-pass |
| Stdlib FB params/members (TON/TOF/R_TRIG/F_TRIG) | about 230 | about 66 | No | Phase 20, RUNT-08 |
| Inline VAR enum values undeclared (`E_IDLE`, `cfgReady`, `READ`, `ON`, `INIT_READ`, ...) | about 60 SEMA010 | 0 | No | Phase 20, DIAL-07 (recommended) |
| `REFERENCE TO ARRAY OF ST_Batch does not support indexing` | 14 | 0 | No (missing auto-deref) | Phase 20, DIAL-09 |
| Library FBs/types without stubs (ADSREAD, FB_EcGetSlaveState, T_AmsNetId, ST_EcSlaveState...) | 17 distinct types | 33 distinct SVNCoreComponents types (FB_ATV320, FB_Sensor, ST_EL1008...) | Yes: missing library | After the change these become one SEMA037 per declaration. Phase 21 resolves them |
| GVLs not in the flattened file (`ECT`, `VFD_Params`, `sensors`, `Modbus`...) | 0 | 1235 SEMA010 | Yes: input incomplete | Phase 21 import |
| Missing conversions and shift functions (`UDINT_TO_REAL`, `BYTE_TO_UDINT`, `UINT_TO_WORD`, `WORD_TO_UINT`, `SHL`, `SYSTEMTIME_TO_DT`) | about 30 | 1 | No, checker gap | Not in scope; see Open Questions |
| `cannot assign DINT to INT` from literal typing | present | present | No | Phase 22 (RUNT-05) |

`stc check tests/twincat_probes/action_inside.st` currently reports 4 errors: line 13 DINT→INT (Phase 22), and `R_TRIG has no input parameter "CLK"` plus TON `IN` and `PT`. After RUNT-08 only the line 13 error should remain.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go toolchain | go1.26.0 (local) [VERIFIED: `go version`] | Build and test | Project language |
| Go stdlib (`strings`, `strconv`, `fmt`, `slices`) | stdlib | All new code | CLAUDE.md mandates stdlib-only core |
| testify | already in go.mod | Existing test assertions | Already used in package tests |

### Supporting
None. No new packages are installed in this phase.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Changing `CallExpr.Args []Expr` to `[]*CallArg` | Keep `Args []Expr` for leading positional args and add `NamedArgs []*CallArg` for everything from the first named arg on, in order | Recommended: the JSON contract for all-positional calls is unchanged, and only 28 test literals construct `ast.CallExpr` (`pkg/interp/assertions_test.go` 14, `new_features_test.go` 11). The full change would touch every consumer and the JSON shape |
| Path-based reference value | Re-evaluate the target expression on every access | Re-evaluation changes meaning when an index variable changes after `REF=`. Resolve indices at bind time |

## Package Legitimacy Audit

No external packages are installed in this phase. slopcheck was not run because there is nothing to check. All work uses the Go stdlib and packages already in go.mod.

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
 .st source
    |
    v
 Lexer ── typed based literal fix (BYTE#16#10 → one TypedLiteral token)
    |
    v
 Parser
   ├─ statements: parseStatement ── KwThis/KwSuper → parseAssignOrCall
   │      parseAssignOrCall: lhs ──┬─ ":="  → AssignStmt
   │                               ├─ "REF" "=" → RefAssignStmt (new)
   │                               ├─ "(" named → CallStmt (unchanged path)
   │                               └─ ";"   → expression statement
   │      CASE: isCaseLabelStart accepts Ident(.Ident)* then ":" "," ".."
   ├─ expressions: parsePostfix
   │      "." IntLiteral → BitAccessExpr (new)    "." Ident → MemberAccessExpr
   │      "(" → parseCallArgs (shared) → CallExpr{Args, NamedArgs}
   │      THIS / SUPER primary → ThisExpr / SuperExpr (new), "^" → DerefExpr
   └─ declarations: parseVarDecl / parseStructMember init → parseInitializer
          "(" Ident ":=" → StructInit (new)     "[" → ArrayInit (new, repeat N(x))
          parseEnumType ")" [base type] ; TYPE ... [:= default]
          parseNamedType: Ident "." Ident → NamedType{Namespace}
    |
    v  AST (+ JSON kinds, fmt, emit)
 Checker pass 1 (resolve.go)
   0. register stdlib FBs (TON..RS, with CODESYS+IEC aliases)
   1. pre-register every TYPE/FB/FUNCTION/PROGRAM/INTERFACE name with a
      pointer-stable empty type object
   2. fill types and scopes (VAR, methods, actions) → SEMA037 for unknown names (deduplicated)
   3. EXTENDS post-pass: copy inherited symbols and FB params into derived
   4. GVL pass (existing pendingGVLs)
    |
 Checker pass 2 (check.go)
   BitAccess → SEMA035 · CallExpr binding → SEMA020/021/024 · enum strict/qualified → SEMA036
   RefAssign · THIS/SUPER · ReferenceType auto-deref · initialiser field/count
    |
    v
 Interpreter (stc test runner; sim later)
   evalExpr: BitAccess read · ThisExpr via env.CurrentFB() · qualified enum in evalMemberAccess
   assignToTarget: BitAccess write (read-modify-write through the target lvalue)
   evalCall dispatch: action → method of current FB → ADR → REF → LocalFunctions
                      → user FUNCTION decl (named binder) → stdlib → zero-arg FB
   execRefAssign: build path reference (root env, var, [member|index]...)
```

### Recommended Plan Structure (file ownership avoids merge conflicts)

| Plan | Owns files | Depends on |
|---|---|---|
| 20-01 AST contracts | `pkg/ast/expr.go`, `stmt.go`, `types.go`, `node.go`, `json.go`; `pkg/format/format.go`; `pkg/emit/emit.go`; `pkg/lint/plcopen.go` (walker) | none |
| 20-02 Parser expressions and statements | `pkg/parser/expr.go`, `stmt.go` | 20-01 |
| 20-03 Lexer and declaration parsing | `pkg/lexer/lexer.go`, `pkg/parser/types.go`, `var.go`, `decl.go` | 20-01 (parallel with 20-02) |
| 20-04 Checker resolver | `pkg/checker/resolve.go`, new `pkg/checker/stdlib_fbs.go`, `pkg/types/types.go`, `diag_codes.go` | 20-02, 20-03 |
| 20-05 Checker expressions | `pkg/checker/check.go` (+ new `check_bits.go`, `check_calls.go`), `pkg/types/builtin.go` | 20-04 |
| 20-06 Interpreter | `pkg/interp/*.go`, `pkg/testing/runner.go` | 20-02, 20-03 (parallel with 20-04/05) |
| 20-07 Acceptance gate | `tests/twincat_dialect/*.st`, `tests/twincat_probes/*`, `tests/twincat_probes_test.go`, new analyzer gate test | all |

### Pattern 1: Bit access in the postfix chain
**What:** In `parsePostfix`, after consuming `Dot`, branch on `IntLiteral` before `parseIdent()`.
**Lexer check:** `w.3` lexes as `Ident Dot IntLiteral`, because `scanNumber` only runs on a digit, and `.` is scanned by `scanOperator`, which yields `Dot` (or `DotDot` for `..`). `arr[0].3`, `.0;`, `.8 :=` and `.0 OR` all tokenise correctly. [VERIFIED: lexer.go:241-284, 377-430]
```go
case lexer.Dot:
    p.advance()
    if p.at(lexer.IntLiteral) {
        idxTok := p.advance()
        idx := &ast.Literal{NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral,
            NodeSpan: ast.SpanFrom(astPos(idxTok.Pos), astPos(idxTok.EndPos))},
            LitKind: ast.LitInt, Value: idxTok.Text}
        expr = &ast.BitAccessExpr{NodeBase: ast.NodeBase{NodeKind: ast.KindBitAccessExpr,
            NodeSpan: ast.SpanFrom(expr.Span().Start, idx.Span().End)}, Target: expr, Index: idx}
        continue
    }
    member := p.parseIdent() // unchanged
```
**Constant-identifier index (`nVar.cEnable`):** the parser cannot tell a constant from a member. Keep it a `MemberAccessExpr`. The checker reinterprets it when the object type is an integer or bit-string type and the member name resolves to an integer CONSTANT; the interpreter mirrors this in `evalMemberAccess` when the object value is `ValInt`. [CITED: infosys.beckhoff.com/content/1033/tc3_plc_intro/2529343371.html, "can be specified by any constant"]

### Pattern 2: Named args in expression calls without breaking CallStmt
`parseAssignOrCall` (`stmt.go:109`) relies on `parsePostfix` returning before `(name :=` so that it builds a `CallStmt`. Keep that behaviour only at statement head:
```go
// Parser gains: stmtHead bool (set by parseAssignOrCall for the lhs only)
case lexer.LParen:
    if p.stmtHead && p.isNamedArgCall() { return expr } // FB/method call statement path unchanged
    p.advance()
    all := p.parseCallArgs()                 // shared; handles trailing comma, empty args
    endTok := p.expect(lexer.RParen)
    args, named := splitArgs(all)            // leading Name==nil → Args (Value); rest → NamedArgs
```
`parseAssignOrCall` sets `p.stmtHead = true` before `lhs := p.parseExpr(0)` and resets it immediately after. The flag must also be cleared on entry to any nested `parseExpr` that sits inside brackets or parentheses, so that `a[f(x := 1)] := 2` still works. The simplest way is to save, clear and restore the flag around argument and index parsing in `parsePostfix`.

**Trailing comma** in `parseCallArgs`:
```go
for {
    args = append(args, p.parseCallArg())
    if !p.match(lexer.Comma) { break }
    if p.at(lexer.RParen) { break } // f(a := 1, ) — dropped
}
```

### Pattern 3: Qualified CASE label lookahead
```go
func (p *Parser) isCaseLabelStart() bool {
    if !(p.at(lexer.IntLiteral) || p.at(lexer.Ident)) { return false }
    saved := p.pos; defer func() { p.pos = saved }()
    p.advance()
    for p.at(lexer.Dot) && p.peekAt(1).Kind == lexer.Ident { p.advance(); p.advance() } // E.v, Lib.E.v
    k := p.peek().Kind
    return k == lexer.Colon || k == lexer.DotDot || k == lexer.Comma
}
```
This is safe because no statement can start with `a.b :` or `a.b ,`. Assignment is the distinct `Assign` token (`:=`). Consider also accepting `TypedLiteral` labels (`E#a:`) and `Minus IntLiteral` (`-1:`) for completeness.

### Pattern 4: Pointer-stable two-pass type registration (RUNT-08 prerequisite)
```go
// Pass 0 (new): for every file, every TypeDecl/FB/FUNCTION/PROGRAM/INTERFACE, Insert a symbol whose
// Type is a fresh, empty object of the right Go type (StructType{Name}, EnumType{Name},
// FunctionBlockType{Name}, FunctionType{Name}); for alias/array/pointer TYPE decls insert a
// symbol with a lazily-filled holder.
// Pass 1 (existing functions): FILL the existing object in place (append Members, Inputs, ...)
// instead of allocating a new one and overwriting sym.Type.
```
Today `resolveFunctionBlock` allocates `fbType := &types.FunctionBlockType{Name: name}` and later sets `sym.Type = fbType`. Any variable resolved earlier keeps the placeholder pointer. `resolveTypeDecl` has the same problem. Filling in place fixes every forward reference at once. Type aliases (`TYPE T : INT; END_TYPE`) and TYPE-of-TYPE chains need either a resolution order (topological) or a lazy holder. Recommend a second sweep: resolve TypeDecls in a loop until no new names resolve, then report SEMA037.

### Pattern 5: Current-FB pointer on Env (THIS, SUPER, unqualified methods)
```go
type Env struct { ...; self *FBInstance } // set in newUserFBInstanceDepth: env.self = inst
func (e *Env) CurrentFB() *FBInstance { for c := e; c != nil; c = c.parent { if c.self != nil { return c.self } }; return nil }
```
The method env's parent is `fbInst.Env` and the action owner env is `fbInst.Env`, so every FB-related scope finds its instance. Use the same boundary to fix the 19-08 deferred bug "action lookup walks past the FB boundary": stop `LookupAction` at the first env with `self != nil`.

### Pattern 6: Shared argument binder (FUNCTION, METHOD, unqualified method)
```go
// pkg/interp/call_args.go
type boundCall struct { inputs map[string]Value; outputs []outBinding; inouts []inoutBinding }
func (interp *Interpreter) bindArgs(env *Env, blocks []*ast.VarBlock, pos []ast.Expr, named []*ast.CallArg) (*boundCall, error)
// positional i → i-th VAR_INPUT/VAR_IN_OUT in declaration order;
// named Name==nil inside NamedArgs → slot = its index in the full arg list (TwinCAT tolerance);
// omitted input → InitValue evaluated in callee env, else ZeroFromTypeSpecWith(...);
// "=>" outputs written with assignToTarget after the body returns.
```
Move `callUserFunction` (`pkg/testing/runner.go:374`) into `pkg/interp` (`interp.FuncDecls map[string]*ast.FunctionDecl` plus `CallFunction`). The runner registers decls instead of closures. This also fixes a latent bug: omitted inputs ignore their declared default today.

### Pattern 7: Path references for REF=
```go
type RefPath struct { Env *Env; Var string; Steps []RefStep } // RefStep{Member string} or {Index int}
// Value gains: Ref *RefPath (Kind ValReference; keep PtrEnv/PtrVar for REF() of a plain variable)
```
Build the path at `REF=` time by evaluating index expressions once. Read by walking from `Env.Get(Var)` through members and indices. Write by walking to the parent, setting the element in place, and writing the root back. Generalise `evalIdent`'s auto-deref (`interpreter.go:315`) and the reference branch of `assignToTarget` (`interpreter.go:643`).

### Anti-Patterns to Avoid
- **Emitting SEMA037 inside `resolveTypeSpec` unguarded.** It is called twice per FB/FUNCTION VAR declaration (`resolveVarBlocksInScope` and the parameter loop). The experiment reported every undeclared type twice. Emit from one site, or dedupe by TypeSpec span.
- **Treating `KindInterface` symbols as undeclared.** Interface symbols have `Type == nil`. `resolveTypeSpec` falls through to the unknown-name path today. Return a placeholder `FunctionBlockType{Name}` (or a new `InterfaceType`) without diagnostics.
- **`env.Set(id.Name, arr)` after an index or member write.** `execAssignIndex` (`interpreter.go:696`) and `execAssignMember` (`interpreter.go:1120`) write the aggregate back by name. When the name is a reference variable, this replaces the reference with a copy. Route the write-back through the reference-aware `assignToTarget` Ident branch.
- **Statically expanding `N(x)` repetition.** `[1000000000(0)]` must be counted, not expanded, in the checker. The runtime application in Phase 22 needs a cap.
- **Checking EnumTypes before env without a guard.** Only treat `X.v` as an enum value when `X` is not a variable in env and `EnumTypes[X]` has `v`. GVL and struct paths must keep working (`gvlRoot` runs first in `evalMemberAccess`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Integer literal parsing in typed literals | New base parser | `interp.parseLitInt` (handles `16#`, `2#`, `_`) | Already handles base prefixes; only the lexer token boundary is wrong |
| EXTENDS walking in interp | New chain walker | `fbDeclChain` / `fbExtendsChain` (`interpreter.go:1331`, `fb_instance.go:149`) | Cycle-safe, depth-bounded |
| Recursion bounds for SUPER^ and methods | Ad-hoc counters | `interp.EnterCall/ExitCall` (MaxCallDepth 256) | Already used by every call path |
| Lvalue writes for bit access and `=>` outputs | Per-node write code | `assignToTarget` | Covers Ident, Index, Member, Deref and GVL members |
| Enum value numbering | `parseInt` in runner.go (`runner.go:642`) | New `interp.EnumValues(spec, eval)`: previous+1 rule, base literals via `parseLitInt` | Current code uses the position index and mis-parses `16#0006` as 160006 |
| Attribute lookup | String scans | `ast.HasAttribute(attrs, "strict")` | Phase 19 helper |

## Common Pitfalls

### Pitfall 1: Forward references become SEMA037 storms
**What goes wrong:** Removing the placeholder without pre-registration turns 32 forward-referenced types in svncore (121 uses) into errors.
**Why:** `CollectDeclarations` registers declarations in file order (`resolve.go:86`).
**How to avoid:** Pattern 4. Keep `TestResolveForwardRef` (the only existing test that fails with naive SEMA037, verified by experiment) and extend it to member access on forward-referenced structs and FBs.
**Warning signs:** svncore `stc check` still shows "type ST_Drive_HMI has no member".

### Pitfall 2: Standard FB parameter names differ between IEC and TwinCAT
**What goes wrong:** The interpreter's CTU accepts `R`, CTD accepts `LD`, SR accepts `S1, R`, and RS accepts `S, R1` (`stdlib_counters.go:34`, `stdlib_bistable.go:24`). Tc2_Standard declares `RESET`, `LOAD`, `SET1, RESET` and `SET, RESET1`, with PV and CV as WORD. A TwinCAT `ctu(CU := x, RESET := r)` is silently ignored at runtime today.
**How to avoid:** The checker signature lists the CODESYS names (locked) and also accepts the IEC names as aliases. The interpreter's `SetInput`/`GetInput` accept both. [VERIFIED: beckhoff-docs/pdf-text/TwinCAT_3_PLC_Lib_Tc2_Standard_EN.txt lines 245-449]

### Pitfall 3: Named-then-positional appears in real TwinCAT code
**What goes wrong:** svncore:3738 `transition(current_state := q_state, transition_action.run, i_xAutoResetAllowed := xResetAllowed)` is a METHOD call in a shipped library. The locked decision makes this a checker error. CODESYS documentation says functions cannot mix the two forms at all.
**How to avoid:** Honour the locked decision in the checker. METHOD bodies are not checked yet, so the oracle is unaffected. The parser accepts the form, and the interpreter binds the positional argument to its slot index so that runtime matches TwinCAT. [CITED: content.helpme-codesys.com/en/CODESYS%20Development%20System/_cds_obj_function.html] [ASSUMED: TwinCAT compiles this form for methods because it is in production library source]

### Pitfall 4: Statement-head CallStmt regression
**What goes wrong:** If the expression parser consumes `fb(IN := x)`, statements become expression statements with a CallExpr. That breaks FB call execution (`execCallStmt`) and checker FB parameter checks.
**How to avoid:** Use the `stmtHead` flag from Pattern 2. Regression tests: every `tests/twincat_dialect/*.st`, `empty_args_test.st` and `pkg/parser` CallStmt tests.

### Pitfall 5: Enum strictness is already implicit
**What goes wrong:** The type lattice never widens enum to or from integer (`types/lattice.go:17-36`). Every enum is effectively strict today, so `strict` adds only a distinct SEMA036 message. Non-strict enums would become *more* permissive (`n := e`, `e = 1`).
**How to avoid:** Decide explicitly (see Open Questions). Keep same-kind comparisons. Note that `CommonType(KindEnum, KindEnum)` is true even for two *different* enum types, so comparing `E1.a = E2.b` passes today. Tighten it with `EnumType.Equal` in comparisons for strict enums.

### Pitfall 6: Enum implicit values and hex literals
**What goes wrong:** `registerEnumTypes` (`runner.go:611`) gives `(tun := 0, rdy := 2, nst)` the value nst=2, but IEC and Beckhoff give 3. It parses `16#0006` as 160006.
**How to avoid:** Use the previous+1 rule ("eGreen := 10, eBlue = 11"). [CITED: infosys.beckhoff.com/content/1033/tc3_plc_intro/2529504395.html] Use one shared function for the checker (value range vs base type) and the interpreter.

### Pitfall 7: Inline anonymous enums
**What goes wrong:** `eStep : (E_IDLE, E_STATES, ...);` in a VAR block resolves to `EnumType{Name:""}`. Its values are never inserted, which gives SEMA010 in the checker and "undefined variable" in the interpreter.
**How to avoid:** Insert the values into the POU scope (checker). Register a synthetic enum `<POU>.<var>` with the interpreter. The runner and FB instance creation must walk VarBlocks for `*ast.EnumType` specs.

### Pitfall 8: Typed literal lexing swallows only one `#`
**What goes wrong:** `BYTE#16#10` lexes as `TypedLiteral("BYTE#16")`, `Hash`, `IntLiteral(10)`.
**How to avoid:** In `scanIdentOrKeyword`'s typed-literal branch, after `scanLiteralValue`, if the consumed value is all digits and the next byte is `#`, consume `#` plus `scanBaseDigits`. `parseTypedLiteral` already splits at the first `#`, so prefix `BYTE` and value `16#10` reach `parseLitTyped` and then `parseLitInt`, which handles the base. No checker change is needed. [VERIFIED: lexer.go:331-375, interpreter.go:172-195,267-312]

### Pitfall 9: fmt and emit silently drop new syntax
**What goes wrong:** The formatter's EnumType case (`format.go:621`) ignores `BaseType`. A formatted `(a := 0) UINT` becomes `(a := 0)`, which changes semantics. The same applies to emit. The probe fixture test checks only idempotence, so a dropped construct still passes it.
**How to avoid:** Add round-trip tests that assert the printed text contains the construct, for every new node: bit access, named args, `REF=`, THIS^/SUPER^, initialisers, namespace types, enum base type and TYPE default.

### Pitfall 10: Reference auto-deref missing in the checker
**What goes wrong:** svncore reports 14 errors like "REFERENCE TO ARRAY OF ST_Batch does not support indexing" (`checkIndexExpr`, `check.go:870`).
**How to avoid:** Add a `derefRef(t types.Type) types.Type` helper and apply it in index, member, bit access, binary operands and assignment targets. `RefAssignStmt` checking uses the raw symbol type.

### Pitfall 11: CallExpr with member callee is unchecked
`checkCallExpr` returns `Invalid` for `inst.M(...)` and `THIS^.M(...)` (`check.go:713`). Named-arg checking on method calls therefore only happens for unqualified calls resolved through scope. This is acceptable for this phase. Do not promise checker coverage for `framer.Drain(pDest := ...)`.

## Code Examples

### Bit read/write in the interpreter
```go
func bitWidth(k types.TypeKind) int { /* BYTE,SINT,USINT 8; WORD,INT,UINT 16; DWORD,DINT,UDINT 32; LWORD,LINT,ULINT 64 */ }

func (interp *Interpreter) evalBitAccess(env *Env, e *ast.BitAccessExpr) (Value, error) {
    t, err := interp.evalExpr(env, e.Target); if err != nil { return Value{}, err }
    n, err := interp.bitIndex(env, e.Index);  if err != nil { return Value{}, err }
    return BoolValue(t.Int&(1<<uint(n)) != 0), nil
}

// in assignToTarget:
case *ast.BitAccessExpr:
    cur, err := interp.evalExpr(env, target.Target); if err != nil { return err }
    n, err := interp.bitIndex(env, target.Index);  if err != nil { return err }
    if val.Bool { cur.Int |= 1 << uint(n) } else { cur.Int &^= 1 << uint(n) }
    return interp.assignToTarget(env, target.Target, cur) // IECType preserved from cur
```
Also add `*ast.BitAccessExpr` to `isAssignable` (`interpreter.go:681`) and to the `=>` output binding switch in `execCallStmt`. That switch only handles Ident and Member today; replace it with `assignToTarget`, as in st301:2634 `q_xDropRequest => SPB03.CN02_MA01_fb.i_xDropRequest`.

### Stdlib FB registration in the checker
```go
// pkg/checker/stdlib_fbs.go — registered by CollectDeclarations before library files (IsLibrary=true
// so user code may override, matching vendor stub semantics)
var stdFBs = []*types.FunctionBlockType{
  {Name: "TON", Inputs: in("IN", BOOL, "PT", TIME), Outputs: out("Q", BOOL, "ET", TIME)}, // TOF, TP same
  {Name: "CTU", Inputs: in("CU", BOOL, "RESET", BOOL, "PV", INT), Outputs: out("Q", BOOL, "CV", INT)},
  {Name: "CTD", Inputs: in("CD", BOOL, "LOAD", BOOL, "PV", INT), Outputs: out("Q", BOOL, "CV", INT)},
  {Name: "CTUD", Inputs: in("CU", BOOL, "CD", BOOL, "RESET", BOOL, "LOAD", BOOL, "PV", INT),
                 Outputs: out("QU", BOOL, "QD", BOOL, "CV", INT)},
  {Name: "R_TRIG", Inputs: in("CLK", BOOL), Outputs: out("Q", BOOL)},   // F_TRIG same
  {Name: "SR", Inputs: in("SET1", BOOL, "RESET", BOOL), Outputs: out("Q1", BOOL)},
  {Name: "RS", Inputs: in("SET", BOOL, "RESET1", BOOL), Outputs: out("Q1", BOOL)},
}
// plus alias map {CTU: R→RESET, CTD: LD→LOAD, CTUD: R→RESET, LD→LOAD, SR: S1→SET1, R→RESET, RS: S→SET, R1→RESET1}
```

### Typed based literal lexer fix
```go
if typedLiteralPrefixes[upper] {
    l.advance() // consume #
    valStart := l.pos
    l.scanLiteralValue()
    if !l.atEnd() && l.peek() == '#' && isAllDigits(l.src[valStart:l.pos]) {
        l.advance()       // second #
        l.scanBaseDigits() // 16#10 → hex digits
    }
    return l.makeToken(TypedLiteral, start)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Unknown type names become an empty FB placeholder | SEMA037, with pointer-stable two-pass registration | This phase | Removes about 460 false errors in svncore; library gaps become explicit |
| User FUNCTIONs as positional closures in pkg/testing | FunctionDecl registry and binder in pkg/interp | This phase (recommended) | Named args, defaults and `=>` outputs; sim can call FUNCTIONs later (RUNT-09) |
| `ValReference` targets one env variable | Path reference | This phase | `REF=` to struct members and array elements |

**Deprecated/outdated:** the `parseInt` helper in `pkg/testing/runner.go` for enum values. Replace it with the shared enum-value function.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | TwinCAT compiles the named, positional, named METHOD call at svncore:3738 | Pitfall 3 | If TwinCAT rejects it, the probe has a latent bug. No change needed, since the locked decision already errors |
| A2 | CTU/CTD/CTUD PV and CV should be typed INT in the checker, not WORD as Tc2_Standard declares | Code Examples | `wCount := ctu.CV` with a WORD target would error under INT. With WORD, `n : INT := ctu.CV` errors instead |
| A3 | Constant-identifier bit index stays a MemberAccessExpr and is reinterpreted semantically | Pattern 1 | The locked text says "anything else is a parse error". The parser cannot detect constness, so this is the only workable reading |
| A4 | Non-strict enums should allow implicit enum to base-integer conversion | Pitfall 5 | If they should not, keep the current strict-everywhere behaviour and use SEMA036 only for qualified_only and strict messages |

## Open Questions (RESOLVED)

1. **How should `CallExpr` represent named arguments?**
   - What we know: `CallStmt.Args` is `[]*CallArg`. `CallExpr.Args` is `[]Expr`, used at about 8 production sites, 28 test literals and in the JSON contract.
   - What's unclear: whether to unify the two.
   - RESOLVED: Keep `Args []Expr` for leading positional args. Add `NamedArgs []*CallArg` containing everything from the first named arg, in source order; a positional arg after a named one has `Name == nil`. JSON gains `named_args` only when present. Also give `CallArg` a NodeKind (`KindCallArg`), closing the 19-02 deferred "SourceFile" kind bug for args.

2. **Is strict-enum arithmetic an error or a vendor warning?**
   - What we know: Beckhoff `strict` makes arithmetic, foreign constant assignment and foreign variable assignment compiler errors. The checker already rejects enum arithmetic for every enum.
   - RESOLVED: Error (SEMA036) for every vendor profile. Non-strict enums allow implicit conversion between the enum and its base integer type only (A4). Comparisons between two different enum types are an error only when either enum is strict.

3. **Should SEMA037 fire for library types when no stubs are loaded?**
   - What we know: st301 alone references 33 SVNCoreComponents types, and svncore references 17 Tc2_* types. `stdlib/vendor/beckhoff` already stubs ADSREAD, ADSWRITE and T_AmsNetId, but only loads them through `[build.library_paths]`.
   - RESOLVED: Yes, once per declaration site (deduplicated). These are genuine "missing library" findings, and Phase 21 owns library resolution. The oracle gate counts parse errors only, so these do not block DIAL-10.

4. **Should the oracle check st301 alone or together with svncore?**
   - RESOLVED: The gate test keeps per-file parse counts, which must reach 0. Add a logged (non-failing) combined `analyzer.Analyze([svncore, st301])` run that prints the top non-parse error buckets, so Phase 21 and 22 can track the "genuine problems only" goal.

5. **Should the missing conversions, SHL/SHR and chained assignment be in scope?**
   - What we know: about 30 svncore SEMA010 errors come from `UDINT_TO_REAL`, `BYTE_TO_UDINT`, `SHL` and similar. Chained assignment appears only in st201.
   - RESOLVED: Out of scope. Record both in `deferred-items.md` for Phase 22 (stdlib parity). The only builtins added in this phase are `TO_STRING` (DIAL-07) and the generic `TO_<int>` forms the strict decision names.

6. **Should METHOD bodies now be checked (cheap with inherited scope)?**
   - RESOLVED: No. It is deferred by CONTEXT. Enabling it would surface the named-then-positional error (Pitfall 3) and unknown method-local scope issues across svncore. Keep it in Phase 22/23.

7. **What types should counter PV and CV have (A2)?**
   - RESOLVED: INT (IEC 61131-3). Literal compatibility covers `PV := 5`. Document the TwinCAT WORD difference in the stub comment.

8. **What happens to `rDropPoint: REAL;;` (lint warning)?**
   - RESOLVED: The parser silently skips stray `;` in VAR blocks and at top-level declaration lists. No lint rule is added in this phase.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | build and test | yes | go1.26.0 darwin/arm64 | — |
| External probes (`STC_PROBES_DIR`) | DIAL-10 oracle | yes, local only | `/Users/jonb/Projects/beckhoff-docs/stc-probes/` (st301.st, svncorecomponents.st, plus case/enum/fcall/ptr/st101/st201/baader) | CI skips the oracle; committed fixtures cover the constructs |
| Beckhoff Tc2_Standard reference text | RUNT-08 signatures | yes | `beckhoff-docs/pdf-text/TwinCAT_3_PLC_Lib_Tc2_Standard_EN.txt` | — |
| GitHub CI (ci.yml, st-tests.yml, coverage.yml) | gate | yes | 3 OS matrix | `scripts/coverage-gate.sh` locally |

**Missing dependencies with no fallback:** none.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + testify; `stc test` ST runner (TEST_CASE / ASSERT_*) |
| Config file | `.testcoverage.yml` (thresholds); `.github/workflows/{ci,st-tests,coverage}.yml` |
| Quick run command | `go test ./pkg/parser/ ./pkg/checker/ ./pkg/interp/ -count=1` |
| Full suite command | `go test ./... -count=1` (about 6 s locally, baseline green on 17fbee8) and `go run ./cmd/stc test tests/` |
| Oracle | `STC_PROBES_DIR=/Users/jonb/Projects/beckhoff-docs/stc-probes go test ./tests -run TestTwinCATProbe -count=1 -v` |
| Coverage gate | `./scripts/coverage-gate.sh` (parser/lexer/interp/types/emit >= 95, checker >= 94, total >= 85) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DIAL-04 | `w.3`, `a.b.c.0`, `arr[0].3` parse to BitAccess | unit | `go test ./pkg/parser -run TestBitAccess -count=1` | No, Wave 0: `pkg/parser/bit_access_test.go` |
| DIAL-04 | SEMA035 on BOOL/REAL/STRING/struct and on index >= width; BOOL result; constant index | unit | `go test ./pkg/checker -run TestBitAccess -count=1` | No: `pkg/checker/bit_access_test.go` |
| DIAL-04 | Read and write through member, array, GVL and method-return targets | unit + ST | `go test ./pkg/interp -run TestBitAccess`; `go run ./cmd/stc test tests/twincat_dialect/bit_access_test.st` | No: both |
| DIAL-06 | `n := F(a := 1, b := 2)` equals `F(1, 2)`; defaults; `=>` outputs; trailing comma; unqualified method with named args | unit + ST | `go test ./pkg/parser ./pkg/checker ./pkg/interp -run TestNamedArgs`; `stc test tests/twincat_dialect/named_args_test.st` | No |
| DIAL-06 | Duplicate, unknown and named-then-positional args are SEMA024-family errors | unit | `go test ./pkg/checker -run TestNamedArgs` | No |
| DIAL-07 | `E.v` in CASE labels, lists and ranges; base type; qualified_only (SEMA036); strict; TO_STRING with and without to_string; implicit numbering; inline enums | unit + ST | `go test ./pkg/... -run TestEnum`; `stc test tests/twincat_dialect/enum_test.st` | No |
| DIAL-09 | `REF=` to variable, member and array element; read and write through; THIS^.x, THIS^.M(), SUPER^.M(), SUPER^(); errors outside FB or without EXTENDS; inherited scope | unit + ST | `go test ./pkg/... -run 'TestRefAssign|TestThis|TestSuper'`; `stc test tests/twincat_dialect/ref_this_super_test.st` | No |
| DIAL-10 | Committed probes parse clean with no allowances; fmt round-trips | integration | `go test ./tests -run TestTwinCATProbeFixtures -count=1` | Yes (edit: remove `allowed` map; add case/enum/fcall/ptr fixtures) |
| DIAL-10 | st301 and svncore have 0 parse diagnostics | oracle | `STC_PROBES_DIR=... go test ./tests -run TestTwinCATProbeOracle -v` | Yes (edit: assert `total == 0`) |
| RUNT-08 | TON..RS members and params check; aliases; SEMA037 on `FB_DoesNotExist`; forward refs; interfaces; namespace fallback; library stubs suppress it | unit | `go test ./pkg/checker -run 'TestStdFB|TestUndeclaredType|TestResolveForwardRef'` | Partial (`resolve_test.go` has TestResolveForwardRef) |
| RUNT-08 | Phase 19 hand-off: action_test.go, action_test.st, empty_args_test.st and action_inside.st are check-clean except line 13 DINT | integration | New `tests/twincat_dialect_check_test.go`: run `analyzer.Analyze` on every `tests/twincat_dialect/*.st` and `tests/twincat_probes/action_inside.st`, and assert no errors except an allowlisted RUNT-05 message | No |

### Sampling Rate
- **Per task commit:** the quick run command for the package touched, plus `go vet ./...`.
- **Per wave merge:** `go test ./... -count=1 && go run ./cmd/stc test tests/` and the oracle command.
- **Phase gate:** full suite green, `./scripts/coverage-gate.sh` passes, oracle reports 0/0, CI green on all three OSes in the PR.

### Wave 0 Gaps
- [ ] `pkg/parser/bit_access_test.go`, `named_args_expr_test.go`, `initializer_test.go`, `enum_base_test.go`, `ref_this_super_test.go`, `pkg/lexer` typed-based-literal cases
- [ ] `pkg/checker/bit_access_test.go`, `stdlib_fb_test.go`, `undeclared_type_test.go`, `enum_attr_test.go`, `named_args_test.go`, `ref_this_test.go`
- [ ] `pkg/interp/bit_access_test.go`, `call_args_test.go`, `enum_runtime_test.go`, `ref_path_test.go`, `this_super_test.go`
- [ ] `tests/twincat_dialect/bit_access_test.st`, `named_args_test.st`, `enum_test.st`, `ref_this_super_test.st` (CI runs them through st-tests.yml "TwinCAT dialect" and ci.yml `stc test tests/`)
- [ ] `tests/twincat_probes/{case,enum,fcall,ptr}.st` fixtures; update `tests/twincat_probes/README.md`
- [ ] `tests/twincat_dialect_check_test.go` (analyzer gate for the hand-off items)
- [ ] fmt and emit round-trip tests asserting each new construct is printed (`pkg/format`, `pkg/emit`)

### Coverage implications
The checker has 1.9 points of headroom (95.92 vs a 94 gate). The new resolver passes and SEMA035/036/037 branches must each get negative tests. The interpreter (96.72) and parser (96.93) need error-path tests for every new RuntimeError and parse error, for example a non-integer bit target at runtime, a dangling reference path, SUPER^ without a base, and a repetition count that is not a literal. Emit is at 97.16; every new printer branch needs a test. Moving `callUserFunction` into `pkg/interp` moves its coverage into a gated package, so port the runner tests that exercise it.

## Security Domain

`security_enforcement` is absent from `.planning/config.json`, so it is treated as enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | The parser and checker treat source as untrusted: bounded recursion, no unbounded allocation from source-controlled counts, and error recovery with no panics (`tests/adversarial_test.go`) |
| V6 Cryptography | no | — |

### Known Threat Patterns for a compiler front end and interpreter

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Deeply nested initialisers `[[[[...` or `((((` causing stack exhaustion | DoS | Add adversarial cases; recursion stays proportional to input. Optionally cap initialiser nesting with a parse error |
| Repetition `[4294967295(0)]` or bit index `w.99999999999999999999` | DoS | Count, don't expand. Parse the index with `strconv` and overflow check, then SEMA035 |
| Self-recursive `SUPER^()` or method recursion | DoS | `EnterCall` / MaxCallDepth on every new call path (method-of-current-FB, SUPER) |
| Reference path to a variable that goes out of scope | Tampering / crash | Return a "dangling reference" RuntimeError, as existing code does for `PtrVar`. Never panic |
| Customer sources in the repo | Information disclosure | The oracle reads only `STC_PROBES_DIR` and logs counts and line numbers, never content (T-19-18). Copied small probes (case/enum/fcall/ptr) are synthetic |

## Sources

### Primary (HIGH confidence)
- Codebase read this session: `pkg/lexer/lexer.go`, `pkg/parser/{expr,stmt,types,var,decl}.go`, `pkg/ast/{expr,types,node,json}.go`, `pkg/checker/{resolve,check,diag_codes}.go`, `pkg/types/{types,lattice,builtin}.go`, `pkg/interp/{interpreter,env,fb_instance,value,stdlib_*}.go`, `pkg/testing/runner.go`, `pkg/analyzer/analyzer.go`, `pkg/format/format.go`, `tests/twincat_probes_test.go`
- Measurements: `stc parse` and `stc check --format json` on the probes; a SEMA037 experiment on a scratch copy (1 failing test, every diagnostic duplicated); `go test ./...` baseline green
- `beckhoff-docs/pdf-text/TwinCAT_3_PLC_Lib_Tc2_Standard_EN.txt`: SR, RS, CTU, CTD, CTUD, TON, TOF, TP, R_TRIG and F_TRIG interfaces

### Secondary (MEDIUM confidence)
- [Beckhoff InfoSys: Bit access to variables](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/2529343371.html): types, constant index, range error, 0-based LSB
- [Beckhoff InfoSys: Enumerations](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/2529504395.html): strict rules, previous+1 numbering, default INT base, allowed base types, `:= default`
- [Beckhoff InfoSys: Attribute 'to_string'](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/5725733771.html): TO_STRING returns the component name (TC3.1 4024+)
- [CODESYS: Function object](https://content.helpme-codesys.com/en/CODESYS%20Development%20System/_cds_obj_function.html): no mixing of explicit and implicit assignments; `=>` outputs on function calls

### Tertiary (LOW confidence)
- None used for decisions.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH. No new dependencies; toolchain verified.
- Architecture and code paths: HIGH. Every fix site was read, and oracle buckets were reproduced.
- TwinCAT semantics: MEDIUM. InfoSys and Tc2_Standard text; A1-A4 need user confirmation.
- Pitfalls: HIGH. Most were reproduced (forward-ref errors, duplicate SEMA037, enum numbering, IEC vs CODESYS names).

**Research date:** 2026-10-06
**Valid until:** 2026-11-05 (stable codebase; re-measure the oracle if Phase 21 lands first)
