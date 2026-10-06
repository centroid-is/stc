---
phase: 20-twincat-expression-semantics
plan: 07
subsystem: interp
tags: [interp, named-arguments, functions, methods, this, super, references, ref-assign, twincat]

requires:
  - phase: 20-twincat-expression-semantics
    plan: 05
    provides: "BitAccessExpr in assignToTarget and isAssignable, enum Value.Enum, enum-aware TO_STRING hook"
  - phase: 20-twincat-expression-semantics
    plan: 02
    provides: "CallExpr.NamedArgs, RefAssignStmt, ThisExpr, SuperExpr"
  - phase: 19-twincat-declaration-syntax
    provides: "fbDeclChain, EnterCall/ExitCall, Env actions"
provides:
  - "bindArgs: named, positional, mixed and empty arguments, declared defaults, => outputs and VAR_IN_OUT write-back"
  - "Interpreter.FuncDecls, RegisterFunctionDecl and CallFunction (user FUNCTIONs run in pkg/interp)"
  - "Env.self, Env.selfDecl, Env.CurrentFB; LookupAction stops at the FB instance env"
  - "evalCall dispatch: action, method of current FB, ADR, REF, LocalFunctions, FuncDecls, stdlib, zero-arg FB"
  - "THIS (pointer to the current instance), SUPER (base declaration relative to the running code), SUPER^.M(), SUPER^()"
  - "RefPath references for REF= and REF() on variables, struct members, array elements, GVL members and FB members"
  - "Index and member writes through a reference variable write the target"
affects: [20-08, 20-09, 21, 22]

tech-stack:
  added: []
  patterns:
    - "Call statements whose callee is not an FB instance go through evalCall, so statement and expression calls share one binder"
    - "A reference is a path walked from its root on every read and write; index expressions are evaluated once at bind time"
    - "Code running for an FB carries its instance and declaring FB on the env (self, selfDecl)"

key-files:
  created:
    - pkg/interp/call_args.go
    - pkg/interp/functions.go
    - pkg/interp/ref_path.go
    - pkg/interp/call_args_test.go
    - pkg/interp/this_super_test.go
    - pkg/interp/ref_path_test.go
    - pkg/testing/runner_functions_test.go
  modified:
    - pkg/interp/interpreter.go
    - pkg/interp/env.go
    - pkg/interp/value.go
    - pkg/interp/fb_instance.go
    - pkg/interp/new_features_test.go
    - pkg/testing/runner.go

key-decisions:
  - "Positional entries bind by their index in the full argument list (ruling A1); duplicate bindings, unknown names, => on a non-output and too many arguments are RuntimeErrors"
  - "Omitted and empty inputs take their declared initial value, evaluated in the callee env, else the type's zero value"
  - "SUPER resolves relative to the declaring FB of the running code (Env.selfDecl), so a three-level chain calls each level once"
  - "Method calls are virtual: an unqualified or THIS^ call uses the instance's most-derived method; SUPER^.M() is not virtual"
  - "REF= checks at bind time that the target exists; a path that stops resolving later is a 'dangling reference' RuntimeError"
  - "REF() of a plain variable keeps the PtrEnv/PtrVar form; paths with steps use Value.Ref"
  - "Built-in and runner functions reject named arguments with a RuntimeError instead of dropping them"

patterns-established:
  - "New call paths run under EnterCall/ExitCall and bind through bindArgs"

requirements-completed: [DIAL-06]  # DIAL-09 runtime half is done; the checker half (SEMA038) is 20-09

duration: 16min
completed: 2026-10-06
---

# Phase 20 Plan 07: Interpreter call and reference semantics Summary

**User FUNCTIONs now run inside pkg/interp with named, positional, mixed and default arguments. Unqualified METHOD calls, `THIS^` and `SUPER^` run against the right instance, and `REF=` binds path references that read and write through members and array elements.**

## Performance

- **Duration:** about 16 min
- **Started:** 2026-10-06T02:59Z
- **Completed:** 2026-10-06T03:15Z
- **Tasks:** 3 of 3
- **Files created:** 7, modified: 6

## Accomplishments

- `F_X(a := 1, b := 2)`, `F_X(1, 2)`, `F_X(b := 2, a := 1)`, `F_X(1, b := 2)` and `F_X(a := 1, 2)` all return 3. `F_X(a := 1)` returns 11 with `b : INT := 10`. The old runner closure ignored that default and returned 1.
- `n := F(a := 1, q => flag, io := v);` writes flag and v after the call returns. `q => s.w.2` writes one bit.
- `fb(q => G.s.m);` and `fb(q => arr[1].w.0);` now write their output. Before, only identifier and member targets were handled.
- A FUNCTION called as a statement with named arguments (`F_Inc(io := v, step := 5);`) runs. The parser makes that a CallStmt, which used to report "undefined".
- `Inc();` and `Inc(step := 5);` inside an FB body or one of its actions call the instance's method, inherited methods included. This closes the 19-08 deferred item.
- An FB body no longer runs an ACTION that only the enclosing PROGRAM defines. That call is now "undefined function". This closes the second 19-08 deferred item.
- `THIS^.n := 7;`, `THIS^.Get()` and `r REF= THIS^.n;` work in bodies, actions and methods.
- `SUPER^.M()` calls the base method and `SUPER^()` runs the base body, both on the same instance. In C EXTENDS B EXTENDS A, where each M returns `SUPER^.M()` plus its own constant, `c.M()` returns 111.
- `r REF= n;`, `r REF= s.m;`, `r REF= arr[i];`, `r REF= G.cfg.limit;`, `r REF= inst.v;` and `r REF= p^;` all read and write through. Changing i afterwards does not move the reference.
- `batches[1] := one;` and `rs.x := 5;` on reference variables write the target. Before, the reference was replaced by a copy (the research anti-pattern).
- `r2 REF= r;` binds r2 to r's target. `REF(s.p_stat_Batches[1].x)` gives the same path reference.
- Recursion through FUNCTIONs, methods, `SUPER^.M()` and `SUPER^()` stops at MaxCallDepth with a RuntimeError.

## Task Commits

1. **Task 1: Shared argument binder and user FUNCTIONs**
   - `3810984` test(20-07): failing tests (RED)
   - `6664ecc` feat(20-07): implementation (GREEN)
2. **Task 2: Current-FB pointer, unqualified METHOD dispatch, THIS^ and SUPER^**
   - `84c85c9` test(20-07): failing tests (RED, build failure on CurrentFB)
   - `ab8e88a` feat(20-07): implementation (GREEN)
3. **Task 3: Path-based REF= references and wave gate**
   - `767f817` test(20-07): failing tests (RED, build failure on RefPath)
   - `5ed1bbf` feat(20-07): implementation, plus edge-case coverage tests (GREEN)

## Verification

- `go test ./... -count=1` passes after each task.
- `go run ./cmd/stc test tests/` reports 216 passed, 0 failed. `go run ./cmd/stc test tests/twincat_dialect` reports 12 passed.
- `grep -n "func callUserFunction" pkg/testing/runner.go` has no match. `grep -n "RegisterFunctionDecl" pkg/testing/runner.go` matches. `grep -n "func (e \*Env) CurrentFB" pkg/interp/env.go` matches.
- `go vet` is clean on pkg/interp and pkg/testing. gofmt is clean on every file this plan touched.
- Every block in call_args.go, functions.go, ref_path.go and the new env.go functions is covered. Every new or edited block in interpreter.go is covered.
- `bash scripts/coverage-gate.sh` exits 0. This closes wave 4.

| package | coverage | min |
|---------|----------|-----|
| pkg/parser | 98.17% | 95% |
| pkg/lexer | 97.52% | 95% |
| pkg/checker | 98.18% | 94% |
| pkg/interp | 98.06% | 95% |
| pkg/types | 100.00% | 95% |
| pkg/emit | 97.73% | 95% |
| total | 95.81% | 85% |

pkg/interp rose from 97.09% to 98.06%.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Call statements of FUNCTIONs and METHODs**
- **Found during:** Tasks 1 and 2
- **Issue:** At statement head the parser turns `F(a := 1);`, `Inc(step := 5);`, `inst.M(v := 1);` and `SUPER^.Put(v := 1);` into a CallStmt. execCallStmt accepted only FB instances, so these failed with "undefined" or "is not a function block instance".
- **Fix:** If the callee is not an FB instance, or is a member that names a METHOD, execCallStmt evaluates the call as a CallExpr with `NamedArgs = s.Args`. A non-FB member that is not a method keeps the old "is not a function block instance" error.
- **Files modified:** pkg/interp/interpreter.go
- **Commits:** 6664ecc, ab8e88a

**2. [Rule 1 - Bug] Named arguments to built-in functions were silently dropped**
- **Found during:** Task 1
- **Issue:** `LIMIT(MN := 0, IN := 5, MX := 10)` ran LIMIT with zero arguments.
- **Fix:** Built-in and LocalFunctions calls with NamedArgs are a RuntimeError ("does not accept named arguments"). Logged in deferred-items.md, since binding them needs the IEC parameter names.
- **Commit:** 6664ecc

**3. [Rule 1 - Bug] SUPER in a multi-level chain**
- **Found during:** Task 2
- **Issue:** Resolving SUPER from the instance's type gives the wrong base when B's method runs on a C instance, and `SUPER^.M()` would loop back into B.
- **Fix:** Env.selfDecl records the declaring FB on method envs and SUPER^() body envs. SUPER is the next declaration after it in fbDeclChain.
- **Files modified:** pkg/interp/env.go, pkg/interp/interpreter.go
- **Commit:** ab8e88a

**4. [Rule 2 - Missing functionality] REF() on paths and bind-time validation**
- **Found during:** Task 3
- **Issue:** REF() accepted only a plain identifier, and the plan did not say what `REF=` to a missing member or an index out of range should do.
- **Fix:** REF() uses the same path builder as `REF=`. Both report "reference target X does not exist" at bind time. A path that stops resolving later is a "dangling reference" error.
- **Commit:** 5ed1bbf

**5. Signature and shape details.** `CallFunction(env, decl, posArgs, named, pos)` and `evalMethodCall(env, member, posArgs, named)` take the argument lists rather than the plan's order or a CallExpr. `RefStep.Index` is a single int, because interpreter arrays are one-dimensional and evalIndex uses only the first index. Five call sites in new_features_test.go were updated for the new evalMethodCall signature.

**6. Fixture parameter name.** The plan's `Inc` example uses a parameter named `by`. BY is a keyword (FOR ... BY), so the tests use `step`.

## Issues Encountered

None blocking. New out-of-scope items are logged in deferred-items.md:
- VAR_IN_OUT and `=>` targets are evaluated twice: once to read, once at write-back. This was already true for FB call statements.
- References stored in struct or GVL members do not auto-dereference.
- Writing to an unbound REFERENCE TO replaces it, as before.
- Built-in functions reject named arguments at runtime.

The two 19-08 deferred items, unqualified METHOD calls and the action lookup boundary, are marked RESOLVED in the Phase 19 deferred-items.md.

## Notes for 20-08 and 20-09

- **DIAL-06** is marked complete. The parser half is 20-02, the checker half 20-06, and the interpreter half this plan.
- **DIAL-09** is not marked. `REF=`, `THIS^` and `SUPER^` execute, but the checker side (SEMA038 for `REF=` to a non-lvalue, THIS outside an FB, SUPER without EXTENDS) belongs to 20-09. Mark DIAL-09 there.
- **ST suites:** 20-08 owns tests/twincat_dialect/named_args_test.st and ref_this_super_test.st. Every shape in this summary runs in the runner. Use `step`, not `by`, as a parameter name.
- **Runtime error texts** that 20-09's checker messages can mirror:
  - "THIS used outside a function block"
  - "SUPER used in FB_X, which does not EXTEND another function block"
  - "reference target is not a variable: *ast.BinaryExpr"
  - "reference target S.NOPE does not exist"
- **Engines outside the runner** must call `RegisterFunctionDecl` for each FUNCTION. The runner does this, but ScanCycleEngine has no file context.
- **Deferred 20-02 item still open:** `fb(1, b := 2);` parses as an expression statement over a CallExpr with NamedArgs, and evalCall does not treat it as an FB call.

## Self-Check: PASSED

- All 7 created files exist. All 6 task commits are in `git log`.
- `grep -n "bindArgs" pkg/interp/call_args.go`, `grep -n "CallFunction" pkg/interp/functions.go`, `grep -n "RefPath" pkg/interp/ref_path.go`, `grep -n "CurrentFB" pkg/interp/interpreter.go` and `grep -n "RegisterFunctionDecl" pkg/testing/runner.go` all match.
