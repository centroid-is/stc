# Phase 20: TwinCAT Expression Semantics - Context

**Gathered:** 2026-10-06
**Status:** Ready for planning
**Mode:** Autonomous smart discuss (recommended answers accepted; overnight run, user unavailable)

<domain>
## Phase Boundary

Expression- and call-level TwinCAT constructs parse, type-check and execute: bit access on integer/bit-string values, named arguments in function-call expressions, qualified enums with base types and the `strict`/`to_string`/`qualified_only` attributes, `REF=`, `THIS^`, `SUPER^`, the checker knowing the ten IEC standard FBs and rejecting undeclared type names. Exit gate: `stc check` on the flattened ST301 + SVNCoreComponents probes reports zero parse errors, so this phase also owns the residual parse gaps Phase 19 bucketed (struct/array initialisers with element initialisers, typed based literals like `BYTE#16#10`, a trailing comma before `)` in calls, an empty `;;` declaration, and namespace-qualified type names like `Tc2_EtherCAT.ST_EcSlaveState`). Not in scope: untyped integer literal typing (`n := n + 1` on INT, RUNT-05, Phase 22), integer wrap (Phase 22), pointer arithmetic, runtime initialiser application (Phase 22, RUNT-06; this phase only needs initialisers to parse and type-check), project import (Phase 21).

Acceptance oracle: `tests/twincat_probes/` fixtures (remove the Phase 20 line allowances for `prog.st` line 9 and `link.st` lines 9-10 in `tests/twincat_probes_test.go`) and the external probes at `/Users/jonb/Projects/beckhoff-docs/stc-probes/` (`STC_PROBES_DIR`): st301.st currently 917 parse diagnostics, svncorecomponents.st 615. Target: 0 parse errors in both, and `stc check` on `tests/twincat_probes/action_inside.st` drops to zero errors other than the RUNT-05 literal-typing message.

</domain>

<decisions>
## Implementation Decisions

### Bit access (DIAL-04)
- New `ast.BitAccessExpr{Target Expr, Index Expr}`; the parser produces it when a postfix `.` is followed by an integer literal (after identifiers, member chains and array indexing: `w.3`, `ECT.X.q_wDigitalInputs.0`, `Modbus.arr[0].3`). The index may also be an identifier that resolves to a CONSTANT (CODESYS allows it); anything else is a parse error.
- Checker: the target type must be BYTE/WORD/DWORD/LWORD or any integer type (SINT..ULINT); BOOL/REAL/STRING/struct targets are an error (new SEMA035 "bit access on non-integer type"). A literal index >= the bit width is an error (SEMA035 as well, distinct message). The expression type is BOOL and it is assignable.
- Interpreter: read returns bit `Index` of the integer value; write sets/clears the bit on the target lvalue (member, array element or GVL member), using the existing lvalue write path. Bit numbering is LSB = 0.
- fmt/emit print `target.N` unchanged; JSON kind `BitAccess`.

### Named arguments in function-call expressions (DIAL-06)
- `parseCallArgs` is shared: in expression position a call may contain `name := expr` and `name => target` arguments (CODESYS allows VAR_OUTPUT binding on function calls). Mixing positional and named is allowed only as "positional first, then named"; duplicates or a named-then-positional sequence are checker errors (SEMA024 family).
- Checker binds named args to the FUNCTION's VAR_INPUT/VAR_IN_OUT by name and `=>` to VAR_OUTPUT; unknown names error; omitted inputs take their declared default. Return type is the function's declared type as today.
- Interpreter: `evalCall` binds by name; `=>` targets are written after the call returns.
- Also fix the trailing comma before `)` in any call (`f(a := 1, )`): accepted and dropped; fmt prints without it.

### Qualified enums, base types, enum attributes (DIAL-07)
- `types.EnumType` gains `Base types.Type` (default INT) parsed from `( ... ) UINT;` and `Qualified bool` from `{attribute 'qualified_only'}` on the TYPE. Enum values are typed by `Base`.
- Resolution: `E.v` resolves anywhere an expression is allowed (assignments, comparisons, CASE labels, label lists `E.a, E.b:`, ranges `E.a..E.c:`, initialisers, call arguments). Bare `v` resolves only when the enum is not `qualified_only`; bare use of a qualified_only enum value is an error (new SEMA036). Duplicate bare value names across non-qualified enums remain the existing error.
- `strict`: arithmetic on enum values and implicit conversion between an enum and an integer are errors (SEMA036 message variant); comparisons and assignments between the same enum type stay fine; explicit `TO_INT`/`UINT_TO_...` conversions stay allowed.
- `to_string`: `TO_STRING(e)` returns the value name at runtime (interp built-in); without the attribute it returns the numeric text, as CODESYS does.
- CASE on enum values compares the underlying integer; the interpreter's qualified-enum lookup is implemented in `evalMemberAccess` by checking `EnumTypes` before treating the left side as a variable.

### REF=, THIS^, SUPER^ and inherited scope (DIAL-09)
- `r REF= x;` is a statement (`ast.RefAssignStmt`) valid only when `r` is `REFERENCE TO T` and `x` is an lvalue of type T; afterwards `r` reads and writes through to `x` (the existing Reference value kind). `REF=` to a non-lvalue is SEMA error.
- `THIS^` evaluates to the current FB instance inside FB bodies, methods, properties and actions; `THIS^.x` and `THIS^.M()` work; using THIS outside an FB is an error. `SUPER^.M()` calls the parent FB's method (and `SUPER^()` runs the parent body); requires EXTENDS.
- The checker now puts inherited methods, properties and variables of the EXTENDS chain into the derived FB's scope (fixes the deferred "inherited methods not in derived scope" item) so `SUPER^` and unqualified inherited calls check. Interp reuses `fbDeclChain` from Phase 19.
- Unqualified METHOD calls inside an FB body/action (`Inc();`) must execute at runtime (closes the 19-08 deferred item): `evalCall` dispatch order becomes action, method-of-current-FB, ADR, REF, LocalFunctions, stdlib, zero-arg FB instance.

### Checker knows the standard FBs; undeclared types are errors (RUNT-08)
- Register the ten IEC FBs as FB types in the symbol table at analyzer start (TON/TOF/TP: IN, PT → Q, ET; CTU: CU, RESET, PV → Q, CV; CTD: CD, LOAD, PV → Q, CV; CTUD: CU, CD, RESET, LOAD, PV → QU, QD, CV; R_TRIG/F_TRIG: CLK → Q; SR: SET1, RESET → Q1; RS: SET, RESET1 → Q1) with the types used by `pkg/interp/stdlib_timers.go` etc. Member access `t.Q`, `t.ET` type-checks; named calls `t(IN := x, PT := T#1s)` check; wrong parameter names stay SEMA024.
- `resolveTypeSpec` no longer manufactures an empty placeholder FB for an unknown name: an undeclared type name is a new error SEMA037 ("undeclared type 'X'"), except names that come from loaded vendor stub libraries or `.library_paths`. Forward references within the analysis unit must still resolve (two-pass), including the deferred GVL pass from Phase 19.
- Namespace-qualified type names `Lib.Type` (e.g. `Tc2_EtherCAT.ST_EcSlaveState`) parse into a `TypeRef` with a namespace; resolution tries the qualified symbol, then falls back to the bare `Type`; if neither exists it is SEMA037.

### Residual parse gaps owned for DIAL-10
- Initialisers: `:= (a := 1, b := 'x')` struct initialiser and `:= [(a := 1), (a := 2)]` array-of-struct initialiser, `:= [3(0)]` repetition, nested arrays, with constant-expression bounds `ARRAY[1..GVL.CONST]` parsing (evaluation of bounds and applying values at runtime is Phase 22). They must type-check field names and element counts where statically known.
- Typed based literals `BYTE#16#10`, `WORD#2#1010`, `DWORD#8#17`, plus `UINT#5` already supported.
- An empty declaration/statement `;;` is tolerated (lexer/parser skip an empty statement, no diagnostic in declarations; a lint warning is fine).
- Re-run the Phase 19 hand-off tests with timers and edge triggers (`pkg/checker/action_test.go`, `tests/twincat_dialect/action_test.st`, `empty_args_test.st`, `pkg/interp/action_test.go`, and `stc check tests/twincat_probes/action_inside.st`).

### Claude's Discretion
- Exact SEMA code numbering beyond the names above, diagnostic wording, AST field names.
- Whether `strict` enum arithmetic is an error or a vendor-profile warning for the portable target.
- How `TO_STRING` on enums interacts with the existing string conversion functions.
- Coverage strategy per package (gates: parser/lexer/interp/types/emit >= 95%, checker >= 94%, total >= 85%; after Phase 19: parser 96.93%, checker 95.92%, interp 96.72%, emit 97.16%).

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- Phase 19 added: `ast.Attribute`, `GVLDecl`, `ActionDecl`, attribute retention; `pkg/parser/pragma.go` `collectPragmas`; shared `parseCallArgs` (`pkg/parser/stmt.go`) with nil-valued CallArg support; `interp.EnterCall/ExitCall` depth guard; `fbDeclChain` for EXTENDS walking; GVL env layer (`RegisterGVL`, `GlobalParent`); `Symbol.GVL`, `Symbol.IsConstant`; the `pendingGVLs` deferred resolver pass in `pkg/checker/resolve.go`; `scripts/coverage-gate.sh`; `tests/twincat_probes/` fixtures and `tests/twincat_probes_test.go` (classification gate with STC_PROBES_DIR oracle).
- `pkg/types` has EnumType with explicit values, REFERENCE TO, POINTER TO; `pkg/interp/value.go` has Reference/Pointer kinds; `ADR`/`^` work.
- `pkg/interp/stdlib_timers.go`, counters, edge FBs implement the standard FBs; their input/output names are the source of truth for the checker signatures.

### Established Patterns
- Parser: recursive descent with Pratt expressions (`pkg/parser/expr.go`), postfix chain parsing for member/index access is where `.N` bit access slots in; error recovery via `declarationStarts`.
- Checker: two-pass (collect then check), `SEMAnnn` codes, vendor profiles; tests table-driven per feature file (`gvl_test.go`, `at_wildcard_test.go`, `action_test.go`).
- Interp: `evalCall`/`evalMethodAccess`/`evalMemberAccess` dispatch; lvalue writes through `assignTo`-style helpers; FBInstance wraps stdlib or user FBs.
- Tests: Go unit tests per package, ST suites under `tests/twincat_dialect/` run by `stc test` in CI (st-tests.yml "TwinCAT dialect" step), probe classification gate in `tests/`.

### Integration Points
- `pkg/parser/expr.go` postfix parsing (bit access, named args), `pkg/parser/types.go` (enum base type, namespace-qualified TypeRef, initialisers), `pkg/parser/stmt.go` (`REF=` statement, `;;`), `pkg/lexer` (typed based literals).
- `pkg/checker/resolve.go` (stdlib FB registration, SEMA037, inherited scope), `pkg/checker/expr.go`-style files for bit access and enum rules.
- `pkg/interp/interpreter.go` evalCall dispatch, `evalMemberAccess` for qualified enums, THIS/SUPER environment binding in `fb_instance.go`.
- `pkg/emit`, `pkg/format` printers for every new node; `pkg/ast/json.go` for JSON cases (note deferred item: several nodes lack NodeKind and serialise as "SourceFile"; add kinds for the new nodes at least).
- `tests/twincat_probes_test.go`: remove the Phase 20 allowances; oracle counts must reach 0 parse errors.

</code_context>

<specifics>
## Specific Ideas

- Residual oracle buckets from 19-09 (diagnostics / lines): st301: initialisers 700/130, bit access 192/64, named fn args 12/2, trailing comma 10/5, `;;` 3/1. svncore: qualified enum CASE labels 326/161, bit access 177/26, named fn args 70/9, initialisers 17/1, enum base type 10/5, namespace-qualified type 7/1, typed based literal 5/1, trailing comma 2/1, REF= 1/1. Plan order should follow impact: initialisers and bit access first, then enums.
- Real shapes to test: `sensors.EPW01_WA01_MA01_IS01.i_xRaw := ECT.EPW01_WA01_FD01.q_wDigitalInputs.0;`, `Modbus.brettakerfi_Write[0].0 := b.Q;`, `n := F_FindStatsRow(sMsg := sMsg, sLabel := 'tray/ok per min', values := v);`, `uDeliverIndex := find_order(prio := deliveries, t := ET_WagonStationType.destination);`, `CASE fault OF lft_e.no_fault: ... lft_e.eef1: ...`, `TYPE hmis_e : (tun := 0, {attribute 'OPC.UA.DA' := '1'} rdy := 2, ...) UINT; END_TYPE` with `{attribute 'qualified_only'} {attribute 'to_string'}` on the TYPE, `Device_1_SlaveInfo : ARRAY[1..EcDiagParam.MAX_EC_SLAVES] OF ST_EcSlaveInfo := [(p_stat_sName := 'ST301.A1.01 (EL6070)', p_stat_sModel := 'EL6070', p_stat_nPhysAddr := 1001, ...), ...];`, `ec: Tc2_EtherCAT.ST_EcSlaveState;`, `r REF= n;`, `THIS^.HMI.p_stat_State`, `SUPER^.FB_init()`.
- Keep `stc fmt` idempotent for every new construct (format twice).
- Add the new ST suites under `tests/twincat_dialect/` (bit_access_test.st, named_args_test.st, enum_test.st, ref_this_super_test.st) so CI exercises them on all three OSes.

</specifics>

<deferred>
## Deferred Ideas

- Untyped integer literal typing (`n := n + 1` on INT is DINT→INT today) and integer wrap: Phase 22 (RUNT-05).
- Applying array/struct initialisers at runtime and evaluating constant-expression bounds: Phase 22 (RUNT-06).
- METHOD bodies are never checked (deferred from 19-07): consider in Phase 22/23 unless cheap while adding inherited scope.
- Pointer arithmetic, `__NEW`/`__DELETE`, LTIME, UNION, BIT type: not in v1.2.
- TcPOU/TcGVL/TcDUT XML import and library placeholder resolution: Phase 21.

</deferred>
