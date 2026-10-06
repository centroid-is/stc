---
phase: 20
reviewed: 2026-10-06
depth: deep
status: fixed
files_reviewed: 96
files_reviewed_list:
  - "git diff main...HEAD --name-only -- pkg cmd tests scripts .github (96 files; pkg/ast, pkg/checker, pkg/emit, pkg/format, pkg/interp, pkg/lexer, pkg/lint, pkg/parser, pkg/symbols, pkg/testing, pkg/types, tests/)"
findings:
  critical: 0
  high: 2
  medium: 4
  low: 8
  total: 14
---

# Phase 20: Code Review Report

**Reviewed:** 2026-10-06
**Depth:** deep (source read plus executed probes against a branch build and a main build)
**Status:** issues_found

## Summary

Scope was every source file in `git diff main...HEAD` under pkg, cmd, tests, scripts and .github. Items listed in `deferred-items.md` and the CONTEXT deferrals were not counted as findings.

Verification performed:

- `go test ./...` and `go vet ./pkg/...` pass on the branch.
- `stc test` on every `*_test.st` directory gives identical results on main and on the branch. The TwinCAT dialect suites pass, with 33 of 33 tests.
- Every truncation prefix of a file holding all new constructs went through `stc check`, `stc fmt` and `stc emit`. None panicked.
- Initialisers nested 100,000 deep, both `[` and `(a :=`, are stopped by `maxInitDepth` without stack exhaustion.
- `stc fmt` is idempotent on every new construct: bit access, REF=, THIS^/SUPER^, struct and array initialisers, enum base types, TYPE defaults, namespaced types and trailing commas.
- Error counts from `stc check` on all 123 repo `.st` files were compared between main and the branch. Every increase is the intended SEMA037 on a standalone stub or fixture, apart from H1, which was found separately.

Every finding below was reproduced by running a probe unless it is marked "by inspection".

## High

### HI-01: Standard FB registration breaks previously clean code with false redeclaration and type errors

**File:** `pkg/checker/stdlib_fbs.go:64-86` (collisions surface at `pkg/checker/resolve.go:322-337` and `pkg/checker/resolve.go:801-811`)

**Issue:** `registerStdFBs` inserts TON, TOF, TP, CTU, CTD, CTUD, R_TRIG, F_TRIG, SR and RS into the global scope as library symbols. The `isStdFB` override exists only in the POU and TYPE resolvers. Two other global inserts collide with these names:

- A bare GVL variable named after a standard FB, such as `TON : INT;`, now reports `redeclaration of "TON" (previously declared at :0:0)`. The position is empty.
- An enum value named after a standard FB, such as `TYPE E_Flip : (SR, RS, TP);`, fails to insert silently. `e := TP;` then resolves `TP` to the FB type and reports `cannot assign TP to E_Flip`. Short names like SR, RS and TP are realistic enum values.

Both files check clean on main with 0 errors and fail on the branch with 3 errors. This is a regression on previously clean code.

Reproduction:
```
TYPE E_Flip : (SR, RS, TP); END_TYPE
VAR_GLOBAL TON : INT; END_VAR
PROGRAM P VAR e : E_Flip; i : INT; END_VAR e := TP; i := TON; END_PROGRAM
```

**Fix:** Let any user or library global replace a standard FB entry, not only POUs and TYPEs. Do this before inserting in `resolveGVL` (bare and qualified symbols) and in the enum-value loop of `resolveTypeDecl`:
```go
if existing := global.LookupLocal(val); existing != nil && r.isStdFB(existing) {
    r.table.RemovePOU(existing.Name) // drop the std FB, user name wins
}
```
A cleaner option is to keep the standard FBs in a scope below global, as the parent of the global scope, so user globals shadow them without any removal. Add regression tests for an enum value and a GVL variable named TP, TON or RS.

### HI-02: SUPER^ inside an inherited ACTION resolves against the wrong base and runs the wrong method

**File:** `pkg/interp/interpreter.go:1348-1359` (`execAction`), `pkg/interp/env.go:183` (`currentFBDecl`), `pkg/interp/interpreter.go:1555` (`evalSuper`)

**Issue:** `execAction` runs the action body directly in the owner env, which is the instance env, without recording the FB that declares the action. `currentFBDecl` therefore returns `inst.Decl`, the most-derived type. Take an ACTION declared in FB_B, where FB_B EXTENDS FB_A and FB_C EXTENDS FB_B. When it runs on an FB_C instance, `SUPER^.M()` calls FB_B.M instead of FB_A.M. Nothing reports an error, so execution is silently wrong. The checker types this correctly because `currentFB` is the declaring FB, so the static and runtime meanings disagree.

The probe `c.Act()` with `trace := SUPER^.M()` in FB_B's action expected 1 and got 2. Property accessors share the same gap, since their env has a nil `selfDecl`.

**Fix:** Track the declaring FB of each action and run it in a child env with `selfDecl` set:
```go
// findAction / LookupAction return the declaring *ast.FunctionBlockDecl too
func (interp *Interpreter) execAction(owner *Env, act *ast.ActionDecl, decl *ast.FunctionBlockDecl, pos ast.Pos) error {
    ...
    run := owner
    if inst := owner.CurrentFB(); inst != nil && decl != nil {
        run = NewEnv(owner); run.self = inst; run.selfDecl = decl
    }
    return interp.execStatements(run, act.Body) // keep ErrReturn handling
}
```
Do the same for property get and set envs. Add an interp test with a three-level chain that calls `SUPER^.M()` from an inherited action.

## Medium

### ME-01: Enum tag leaks into integer variables, so TO_STRING of an INT prints an enum name

**File:** `pkg/interp/enum.go:33`, `pkg/interp/interpreter.go:1327`, `pkg/interp/call_args.go:126-133`

**Issue:** Enum values carry `Value.Enum`. Assignment and argument binding copy the Value unchanged. After `i := e;` with `i : INT`, or after passing `e` to an INT VAR_INPUT, `TO_STRING(i)` returns `'blue'` instead of `'2'`. This is the reverse of the deferred "tag lost on integer assignment" item and is not covered by it.

Probe output: `expected '2', got 'blue'` for both the assignment and the parameter case.

**Fix:** Strip the tag when the declared type of the destination is not that enum. One way is to clear `Enum` in `assignToTarget` and `bindArgs` unless the target's declared type spec resolves to the same enum. A cheaper way is to resolve TO_STRING from the argument's static type, for example by having the checker annotate the call, instead of from a runtime tag.

### ME-02: Enum values initialised from a constant are silently misnumbered at runtime

**File:** `pkg/ast/enum_ordinals.go:52-59`, `pkg/checker/resolve.go:934`, `pkg/interp/enum.go:52`

**Issue:** `EnumOrdinals` returns `Known=false` with a positional guess for `(ka := C_BASE, kb)`. Its documentation says callers that can evaluate constants should do so, but neither the checker nor the interpreter does. The checker reports nothing, and at runtime `E_K.kb` is 1 instead of 11.

Probe: `ASSERT_EQ(TO_INT(E_K.kb), 11)` gives `expected 11, got 1`, and `stc check` reports 0 diagnostics. CASE labels, comparisons with hardware values and persisted values then use wrong numbers without any warning.

**Fix:** In `resolveEnumSpec`, resolve a non-literal value through `Symbol.HasConstInt` for an identifier or `GVL.C` constant, and feed the result into numbering. The interpreter should evaluate the expression in the global env when it registers the enum. If the value is still unknown, report a diagnostic such as SEMA036 "enum value must be a constant integer expression" rather than guessing.

### ME-03: VAR_IN_OUT bound to a literal or expression is accepted and silently not written back

**File:** `pkg/checker/check_calls.go:102-127`, `pkg/interp/call_args.go:134`

**Issue:** `bindCallArgs` checks an in-out argument with `checkInputArg`, which never requires an lvalue. The probe `F_Add(a := 1, io := 5)` checks clean. At runtime `isAssignable` is false, so the write-back is skipped and the callee's changes are lost. IEC 61131-3 and CODESYS require a variable for VAR_IN_OUT. Omitting a VAR_IN_OUT is also accepted without a diagnostic.

**Fix:** In `bindCallArgs`, when `p.Direction == types.DirInOut`, require `isLValue(a.Value)` and report SEMA021 "VAR_IN_OUT %q must be bound to a variable". Also report an unbound in-out once any argument is named.

### ME-04: emit prints REF=, THIS^ and SUPER^ for targets that strip references and OOP

**File:** `pkg/emit/emit.go:720-725` (RefAssignStmt), and the ThisExpr and SuperExpr cases in `emitExpr`

**Issue:** For `--target schneider` or `portable`, the emitter drops the `r : REFERENCE TO INT;` declaration but still prints `r REF= v;`, `THIS^.v := 1;` and `SUPER^();`. The output names an undeclared variable and uses constructs the target lacks, so it will not compile on the target. `r := REF(v)` had the same gap before this phase, but REF=, THIS and SUPER are new Phase 20 output.

**Fix:** When `!e.opts.Target.supportsOOP()` or references are unsupported, either refuse with a diagnostic, which is preferred and matches the "no OOP/pointers/references" contract, or skip these statements with a comment. Add emit tests for Schneider and portable that contain these nodes.

## Low

### LO-01: A positional argument after a `=>` binding takes the wrong slot

**File:** `pkg/interp/call_args.go:102-106`, `pkg/checker/check_calls.go:54-61`

**Issue:** Positional arguments bind `slots[i]` and `fn.Params[i]`, where `i` is the index in the full argument list including `=>` outputs. `F_Sum(a := 1, q => x, 2)` reports "too many arguments" in both the checker and the interpreter, although ruling A1 accepts any mix.

**Fix:** Count only non-output arguments when computing the positional slot index.

### LO-02: Binding both spellings of a standard FB input is not detected

**File:** `pkg/checker/stdlib_fbs.go:47-55`

**Issue:** `ctr(CU := x, RESET := a, R := b)` checks clean. At runtime the last binding wins because both names set the same field.

**Fix:** Map each alias to its canonical name in `checkCallStmt` and report "parameter bound more than once" when both spellings appear.

### LO-03: SEMA037 misses non-type global names used as types

**File:** `pkg/checker/resolve.go:1089-1100` (`lookupTypeName`)

**Issue:** The fallback accepts any global symbol that has a Type. `b : red;`, an enum value, becomes E_Col. `c : F_One;`, a FUNCTION, and `d : P;`, a PROGRAM used inside itself, are also accepted without SEMA037.

**Fix:** Accept only `KindType`, `KindFunctionBlock`, `KindInterface` and the standard FB symbols in the fallback, and in the `forward` map exclude FUNCTION and PROGRAM shells as variable types.

### LO-04: Typed based literals accept invalid bases and empty digits

**File:** `pkg/lexer/lexer.go:351-359`

**Issue:** `WORD#3#10` lexes, checks clean and evaluates in base 3, giving 3. IEC allows only bases 2, 8 and 16. `DINT#16#` checks clean and fails only at runtime with "invalid integer literal".

**Fix:** In the lexer, accept the second `#` only when the base is 2, 8 or 16, and require at least one digit after it. Otherwise emit an Illegal token so the parser reports it.

### LO-05: execCallStmt evaluates the member callee object up to three times

**File:** `pkg/interp/interpreter.go:1009`

**Issue:** By inspection, the method probe, `evalMemberAccess` and `evalMethodCall` each evaluate `c.Object`. A callee such as `arr[Next()].M(x := 1);` runs its side effects more than once.

**Fix:** Evaluate the object once and pass the value into the method and instance paths.

### LO-06: A strict-enum error in a built-in call leaves later arguments unchecked

**File:** `pkg/checker/check.go:820-823`

**Issue:** By inspection, the loop returns `Invalid` on the first strict-enum argument. Variables in later arguments are never marked used, which gives false "declared but never used" warnings.

**Fix:** Record the error, keep checking the remaining arguments, and return Invalid after the loop.

### LO-07: selfMember type assertion is unchecked

**File:** `pkg/checker/check_ref.go:118`

**Issue:** By inspection, `fb := objType.(*types.FunctionBlockType)` panics if THIS^ or SUPER^ ever types as anything else. It is safe today only because `fbPointer` returns either an FB pointer or Invalid. That holds within this phase, but a future change to `checkDerefExpr` could break it and crash the LSP.

**Fix:** Use `fb, ok := objType.(*types.FunctionBlockType); if !ok { return types.Invalid, true }`.

### LO-08: ArrayType.Equal ignores the new Known flag

**File:** `pkg/types/types.go:185`

**Issue:** By inspection, `ARRAY[1..GVL.A]` and `ARRAY[1..GVL.B]` both store `Low=1, High=0` with `Known=false`. They compare equal to each other and to a literal `ARRAY[1..0]`, so mismatched array assignments pass silently.

**Fix:** Treat two dimensions as equal only when both are Known with equal bounds, or both are unknown with the same bound expression text.

---

## Fixes applied

Every fix has a regression test. All HIGH and MEDIUM findings and six of the eight LOW findings are fixed. LO-05 and LO-06 are deferred in `deferred-items.md`, together with follow-ups found while fixing.

| Finding | Commit | Fix |
|---------|--------|-----|
| HI-01 | 7e1c12e | GVL variables, GVL names and enum values replace a standard FB of the same name. |
| HI-02 | 323ccaf | Actions and property accessors run with the declaring FB as selfDecl, so SUPER^ resolves from it. |
| ME-01 | b861fc6 | A stored integer takes the enum tag of its destination. |
| ME-02 | e219b6d | Enum values from constants are evaluated in the checker and interpreter. An unresolvable value reports SEMA036. |
| ME-03 | 3ff2a98 | A VAR_IN_OUT bound to a non-variable, or left unbound in a named call, reports SEMA021. |
| ME-04 | b5a14ae | On schneider and portable, statements using REF=, THIS^, SUPER^ or a dropped variable are replaced by a removal comment. |
| LO-01 | 63d5c81 | `=>` bindings do not take a positional slot. |
| LO-02 | 1bbc19f | Binding a standard FB input and its alias is reported. |
| LO-03 | 7f21ff5 | Enum values, FUNCTIONs, PROGRAMs and variables are not accepted as types. |
| LO-04 | d7a32b3 | Based literals need base 2, 8 or 16 and at least one digit. |
| LO-05 | d577cbf | Deferred. |
| LO-06 | d577cbf | Deferred. |
| LO-07 | c45a49a | selfMember checks its type assertion. |
| LO-08 | c7554be | Array bounds that name constants are resolved. Unknown bounds compare by expression. |

---

_Reviewed: 2026-10-06_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
