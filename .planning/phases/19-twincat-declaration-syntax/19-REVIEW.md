---
phase: 19
reviewed: 2026-10-06
depth: deep
status: fixed
base: main
head: ac300ef
findings:
  critical: 0
  high: 3
  medium: 4
  low: 11
  total: 18
files_reviewed_list:
  - pkg/parser/decl.go
  - pkg/parser/parser.go
  - pkg/parser/pragma.go
  - pkg/parser/error.go
  - pkg/parser/stmt.go
  - pkg/parser/trivia.go
  - pkg/parser/types.go
  - pkg/parser/var.go
  - pkg/lexer/lexer.go
  - pkg/ast/attribute.go
  - pkg/ast/decl.go
  - pkg/ast/json.go
  - pkg/ast/node.go
  - pkg/ast/types.go
  - pkg/ast/var.go
  - pkg/checker/check.go
  - pkg/checker/resolve.go
  - pkg/checker/usage.go
  - pkg/checker/diag_codes.go
  - pkg/symbols/symbol.go
  - pkg/incremental/depgraph.go
  - pkg/emit/emit.go
  - pkg/format/format.go
  - pkg/interp/env.go
  - pkg/interp/fb_instance.go
  - pkg/interp/gvl.go
  - pkg/interp/interpreter.go
  - pkg/interp/scan.go
  - pkg/lsp/navigate.go
  - pkg/sim/engine.go
  - pkg/testing/runner.go
  - cmd/stc/check.go
  - cmd/stc/emit_cmd.go
  - cmd/stc/fmt_cmd.go
  - cmd/stc/parse.go
  - cmd/stc/gvlname.go
  - cmd/stc/sim_cmd.go
  - scripts/coverage-gate.sh
  - .github/workflows/st-tests.yml
  - .gitattributes
  - tests/twincat_probes_test.go
  - tests/twincat_dialect/*.st
  - tests/twincat_probes/*.st
  - "*_test.go added in pkg/ and cmd/stc (test-quality pass only)"
---

# Phase 19: Code Review Report

**Reviewed:** 2026-10-06
**Depth:** deep (diff `main...HEAD`, cross-checked against a `main` build)
**Status:** fixed

## Summary

`go test ./pkg/... ./cmd/... ./tests/` passes, `go vet` is clean, and
`stc test tests/twincat_dialect` passes 12/12. Malformed-input probes
(unterminated pragmas, bare `ACTION`, `VAR_GLOBAL` at EOF, `AT` without an
address, empty enum with pragma, `f(a := , => , b =>)`) produced no panics in
parse, check, fmt or emit.

Every finding below was reproduced with the branch binary unless it says
"by inspection". Items already listed in `deferred-items.md` are not repeated.

The three high findings are:

1. A file that holds both `VAR_GLOBAL` and a POU named after the file loses its GVL.
2. Zero-argument FB calls are not covered by the call-depth guard.
3. A crash in `findMethod` that this phase made reachable from the new ACTION path.

## High

### HI-01: GVL named after the file collides with a POU of the same name and is dropped

**File:** `pkg/checker/resolve.go:118-129`, `pkg/parser/decl.go:92-93`
**Issue:** The GVL takes the file basename. GVLs resolve in a second pass,
after every POU is registered. A file `main.st` containing
`VAR_GLOBAL gCount : DINT; END_VAR` and `PROGRAM Main` is the usual
single-file layout. In that file the GVL meets `Main` in the global scope. It
reports `redeclaration of "main"` and returns before inserting any variable.
Every use of `gCount` then reports SEMA010. CONTEXT.md explicitly allows
mixing GVL blocks with other declarations in one file, so this layout is
supported and fails.

```
main.st:1:1: error: redeclaration of "main" (previously declared at main.st:5:9)
main.st:9:1: error: undeclared identifier "gCount"
...
```

Passing `--gvl-name GVL` makes it check clean. The LSP has no such flag, so
the editor always shows these errors.
**Fix:** A name clash on the derived GVL name should not discard the
variables. Register the bare variables even when the GVL symbol cannot be
inserted. Downgrade the clash to a warning that suggests `--gvl-name`, or
skip the qualified symbol when the name was derived and not explicit. Track
the origin of the name on `GVLDecl`, for example with a `NameDerived bool`
field, so that an explicit name still reports the error.

```go
if existing := r.table.LookupGlobal(name); existing != nil && existing.Kind != symbols.KindGVL && d.NameDerived {
    r.diags.Warnf(pos, CodeRedeclared, "GVL name %q (from file name) clashes with %s; use --gvl-name", name, existing.Kind)
    registerQualified = false // still insert bare vars below
}
```

### HI-02: Call-depth guard does not cover FB instance calls; self-call through a GVL crashes the process

**File:** `pkg/interp/interpreter.go:1293-1297`, `1322-1324`, `1361-1363`,
`1394-1398`, and `execCallStmt` at about line 1030
**Issue:** `EnterCall` wraps ACTION, METHOD and user FUNCTION bodies only.
This phase added several zero-argument FB paths: `fb();`, `G.fb();`, `s.fb();`
and `outer.inner();`. They all call `runFBInstance` with no depth check, and
`execCallStmt` runs `Execute` unguarded as well. An FB that calls its own
instance through a GVL recurses until the Go runtime aborts. The abort is a
fatal error, not a recoverable panic, so the whole `stc test` run dies and
the other test cases report nothing.

```
FUNCTION_BLOCK FB_Self  ... g(); END_FUNCTION_BLOCK
VAR_GLOBAL g : FB_Self; END_VAR
TEST_CASE 't' g(); END_TEST_CASE
-> fatal error: stack overflow
```

**Fix:** Call `EnterCall` and `ExitCall` inside `runFBInstance`. Do the same
around `fbInst.Execute` in `execCallStmt`, so every user code entry point is
bounded. Add a regression test next to `pkg/testing/call_depth_test.go`.

```go
func (interp *Interpreter) runFBInstance(inst *FBInstance) error {
    if err := interp.EnterCall(inst.TypeName, ast.Pos{}); err != nil { return err }
    defer interp.ExitCall()
    return inst.Execute(inst.deltaFor(interp.clock, interp.dt), interp)
}
```

### HI-03: `findMethod` recurses forever on 3-level EXTENDS chains, and the new ACTION path always reaches it

**File:** `pkg/interp/interpreter.go:1384-1392` and `findMethod` at about
line 1486, plus `pkg/interp/fb_instance.go:78-81`
**Issue:** `findMethod` recurses with `{Decl: ParentDecl, ParentDecl:
ParentDecl}`. When the parent itself EXTENDS something, the next call checks
the same declaration again with the same arguments. That repeats until the
stack overflows. The bug in `findMethod` predates this phase. The `main`
binary crashes the same way on `c.Bump()` with C extending B extending A.
This phase makes it worse in two ways:

- `newUserFBInstanceDepth` now sets `ParentDecl` on every user FB instance
  whose chain is known, including instances created by the scan engine and
  inside GVLs. Before this phase, only one runner path set it.
- `evalMethodCall` calls `findMethod` before `findAction`, so `inst.Act()`
  crashes even when `Act` is declared on the instance's own FB.

```
FB_C EXTENDS FB_B, FB_B EXTENDS FB_A; ACTION Act on FB_C; c.Act();
-> fatal error: stack overflow (interp.findMethod ...)
```

**Fix:** Walk the chain iteratively, using `fbExtendsChain` (or
`interp.FBDecls`) so each level points at its own base. Then look up actions
in the same walk.

```go
for _, d := range reverse(fbExtendsChain(inst.Decl, interp)) {
    for _, m := range d.Methods { if strings.EqualFold(m.Name.Name, name) { return m } }
}
return nil
```

## Medium

### ME-01: Trailing pragmas are moved by fmt and emit, which changes `{warning restore}` and `{endregion}` scope

**File:** `pkg/parser/var.go:96-99,126-127`, `pkg/parser/types.go:181-186`,
`pkg/parser/types.go:255-266`
**Issue:** Pragmas before `END_VAR` are stored on the `VarBlock`, and the
printer writes them before the `VAR` keyword. Pragmas before `END_STRUCT` or
the closing `)` of an enum go to the last member, and print before that
member. This was reproduced on a block containing `{warning disable C0195}`,
then `x`, then `{warning restore C0195}`. After `stc fmt`, the restore sits
above `VAR` and the disable sits inside the block. The `{endregion}` lines
move in the same way, and a struct region loses its last member. The output
is a fixed point, so the idempotence tests pass, but the source meaning has
changed. Preserving source through fmt and emit round-trips is a stated
phase goal.
**Fix:** Add `TrailingAttributes` and `TrailingPragmas`, or a single
`EndPragmas` field, to `VarBlock`, `StructType` and `EnumType`. Print them
right before `END_VAR`, `END_STRUCT` or `)`. Extend the format tests to
compare the pragma order around the original anchors, not only idempotence.

### ME-02: `qualified_only` isolation leaks at runtime into FB instances declared in a GVL

**File:** `pkg/interp/scan.go` (`initVarDecl`, the `NewUserFBInstance(..., env)` call), `pkg/interp/gvl.go:52-58`
**Issue:** `RegisterGVL` creates the GVL's FB instances with the GVL env as
their parent. The body of such an FB can therefore read the variables of a
`qualified_only` GVL by bare name, plus every earlier unqualified GVL.
`stc check` rejects this with SEMA033. `stc test` does not run the checker,
so the test passes against behaviour TwinCAT would refuse to compile.

```
{attribute 'qualified_only'} VAR_GLOBAL secret : DINT := 42; peek : FB_Peek; END_VAR
FB_Peek body: seen := secret;     -> runtime reads 42 (expected: unresolved)
```

**Fix:** In `RegisterGVL`, create FB-typed members with
`interp.gvls.unqualified` as their parent, not with the GVL's own env. The
general rule is that an FB env's parent should be the global chain, not the
env that declares it. That also covers the known action-lookup leak listed
in `deferred-items.md`.

### ME-03: fmt and emit silently drop attributes on INTERFACE method and property signatures

**File:** `pkg/parser/decl.go:395-397`, `pkg/ast/decl.go` (`MethodSignature`, `PropertySignature`)
**Issue:** The interface loop still discards `Pragma` tokens with
`p.advance()`. `{attribute 'TcRpcEnable'}` before an interface `METHOD` is
common in TwinCAT OPC UA code, and `stc fmt` deletes it. Reproduced: the
output contains only `METHOD M : BOOL;`. This is data loss on a reformat.
**Fix:** Add `Attributes` and `Pragmas` to `MethodSignature` and
`PropertySignature`. Call `collectPragmas` in `parseInterface`, attach the
result to the next signature, and print and marshal it like `MethodDecl`.

### ME-04: Vendor checks and vendor emit ignore GVLs, ACTIONs and attributes

**File:** `pkg/checker/vendor.go:137-146`, `pkg/emit/emit.go:186-197,450-472`
**Issue:** `checkVendorDecl` has no `*ast.GVLDecl` case and does not visit
ACTION bodies. `POINTER TO`, `REFERENCE TO` and 64-bit types inside
`VAR_GLOBAL` therefore get no VEND00x warning for the Allen Bradley or
Schneider profiles. `stc emit` for those targets writes `{attribute ...}`,
`VAR_GLOBAL`, `AT %I*` struct members and `ACTION` blocks unchanged.
CLAUDE.md lists all of these as unsupported on Logix 5000.
**Fix:** Add a `GVLDecl` case that runs `checkVendorVarBlock` on every block.
Gate attribute and ACTION output on a `Target.supportsAttributes()` /
`supportsActions()` check, either emitting nothing or emitting a comment.
For the AB target, at minimum warn on GVLs and ACTIONs.

## Low

### LO-01: `Attribute.String` does not escape `}`, despite its injection-proof claim

**File:** `pkg/ast/attribute.go:18-36`
**Issue:** The lexer ends a pragma at the first `}`. A value containing `}`
can come from a programmatically built AST, such as the Phase 21 TcPOU
import. Printing it yields `{attribute 'x' := 'a}b'}`. That reparses as a
truncated pragma followed by stray tokens, which is the injection the
comment says cannot happen. gofmt has also rewritten the comment's `''` to
`”`, so the comment now misdescribes the escaping.
**Fix:** Reject or replace `}` in `quoteAttr`, for example by mapping it to
`)` or dropping the attribute with a diagnostic. Alternatively, make
`scanPragma` honour quotes. Restore the comment text, for example by
writing "doubled (two quote characters)".

### LO-02: New redeclaration error when a METHOD shares a name with an FB variable

**File:** `pkg/checker/resolve.go:248-257`
**Issue:** `resolveMethods` inserts methods into the FB scope. An FB with
`VAR cnt` and `METHOD Cnt` checked clean on `main` and now reports
`redeclaration of "Cnt"`. That may be correct for TwinCAT, but it is a
behaviour change for existing code, and it is not in CONTEXT.md or any
summary.
**Fix:** Confirm the behaviour against TwinCAT. If TwinCAT allows it, insert
methods into a separate callable namespace that is consulted only for call
expressions.

### LO-03: `--gvl-name` on `fmt` and `emit` has no effect

**File:** `cmd/stc/fmt_cmd.go:78`, `cmd/stc/emit_cmd.go:80`, `cmd/stc/gvlname.go`
**Issue:** The GVL name is never printed. A GVL is printed as its
`VAR_GLOBAL` blocks, so renaming it before fmt or emit changes nothing in
the output.
**Fix:** Drop the flag from fmt and emit, or document it as accepted for
symmetry. Alternatively, print the name as a leading comment.

### LO-04: LSP definition and hover for `qualified_only` GVL members point at the GVL, and the fallback over-matches

**File:** `pkg/lsp/navigate.go:84-124`
**Issue:** The synthesized symbol uses `g.Pos`, which is the start of the
first `VAR_GLOBAL` block. Go-to-definition therefore lands there, not on the
variable. The fallback also runs for any unresolved identifier, by
inspection. A misspelled struct field or bare access that matches a
`qualified_only` member shows that GVL variable in hover.
**Fix:** Record each member's declaration position in `GVLInfo`, for example
as `map[string]source.Pos`. Run the fallback only when the identifier is the
`Member` of a `MemberAccessExpr` whose object resolves to a `KindGVL`
symbol.

### LO-05: GVL JSON uses `blocks` while every POU uses `var_blocks`

**File:** `pkg/ast/json.go:204-217`, `pkg/ast/attribute.go:52-57`
**Issue:** MCP and `--format json` consumers need a special case for GVLs.
**Fix:** Emit `var_blocks`, or emit both keys during a deprecation window,
and update `json_test.go`.

### LO-06: Statement-level pragmas are still deleted by fmt

**File:** `pkg/parser/decl.go:123-129,240-253`, `pkg/parser/stmt.go:20-26`
**Issue:** The parser collects `{region}`, `{endregion}` and
`{warning ...}` inside POU bodies, and before `END_PROGRAM` or
`END_FUNCTION_BLOCK`, and then discards them. `stc fmt` removes them. This
predates the phase, but the phase built the infrastructure to keep them and
does not apply it here.
**Fix:** Keep body pragmas as a `PragmaStmt` statement node, or as trivia,
and print them in place.

### LO-07: A missing `END_ACTION` inside a FUNCTION_BLOCK swallows the following METHOD

**File:** `pkg/parser/decl.go:160-164`
**Issue:** `actionBodyStops` lacks `KwMethod`, `KwProperty` and the access
modifiers. The METHOD header and body are parsed as broken statements of
the action. Reproduced: four cascading errors, and the method is lost from
the AST, which affects LSP outline and hover.
**Fix:** Add `KwMethod`, `KwProperty`, `KwPublic`, `KwPrivate`,
`KwProtected`, `KwInternal`, `KwAbstract`, `KwFinal` and `KwOverride` to
`actionBodyStops`.

### LO-08: SEMA034 misses output bindings and indexed writes to constants

**File:** `pkg/checker/check.go:589-607`
**Issue:** Only `AssignStmt` targets are checked. The cases below pass
without SEMA034:

- An output binding to a constant, as in `q => c` or `q => G.c`.
- An indexed write, as in `arr[1] := ...` on a `VAR_GLOBAL CONSTANT` array.
- A member write, as in `s.a := ...` on a constant struct.

**Fix:** Call `checkConstantTarget` on `=>` argument values in
`checkCallStmt`. Walk `IndexExpr` and `MemberAccessExpr` down to their root.

### LO-09: Branch-touched Go file is not gofmt-clean

**File:** `pkg/ast/node_test.go:940-941`
**Issue:** `gofmt -l` lists it because the `stoppingVisitor` field alignment
is off.
**Fix:** `gofmt -w pkg/ast/node_test.go`.

### LO-10: `scripts/coverage-gate.sh` hard-codes thresholds that duplicate `.testcoverage.yml`

**File:** `scripts/coverage-gate.sh:50-56`
**Issue:** The local gate and CI will drift when a threshold changes in one
place only. The temporary work directory is also never removed.
**Fix:** Parse `override` and `threshold.total` out of `.testcoverage.yml`
with awk or `go run`. Add `trap 'rm -rf "$WORK"' EXIT`, unless a
`KEEP_WORK` variable is set.

### LO-11: Probe fixture test checks only idempotence, and skips the round-trip for probes with any diagnostic

**File:** `tests/twincat_probes_test.go:139-152`
**Issue:** ME-01 passes this gate because the relocated output is a fixed
point. `prog.st` and `link.st` have Phase 20 diagnostics, so they are never
round-tripped at all. They contain the ACTION and attribute shapes the
phase targets.
**Fix:** Compare the sequence of attribute and pragma texts, plus their
anchor lines, between the source and the first fmt pass. Round-trip probes
that have only allowed diagnostics.

## Fixes applied

| Finding | Commit | Result |
|---------|--------|--------|
| HI-01 | 7bf51e6 | A file-derived GVL name that clashes with a POU now warns and keeps the bare variables. An explicit name still errors. |
| HI-02 | 35e4f76 | `FBInstance.Execute` enters the call-depth guard, covering every FB call path. |
| HI-03 | e3a00d6 | Method, property and action lookup walk the EXTENDS chain iteratively. |
| ME-01 | 75ad199 | Pragmas before `END_VAR`, `END_STRUCT` and an enum's `)` are stored as end pragmas and printed in place. |
| ME-02 | d62bf84 | FB instances in a `qualified_only` GVL get the plain-GVL chain as parent. |
| ME-03 | 9068d63 | Interface signatures keep attributes. fmt and emit print var blocks and `END_METHOD`/`END_PROPERTY`, so the output re-parses. |
| ME-04 | ae84982 | Partial. Vendor checks now cover `VAR_GLOBAL` and TYPE/struct members. ACTION and attribute gating is deferred because no Allen Bradley target exists. |
| LO-01 | f292147 | `Attribute.String` replaces a closing brace with `)`. |
| LO-07 | e3f2807 | An unterminated ACTION body stops at `METHOD` and `PROPERTY`. |
| LO-09 | a624dae | gofmt on `pkg/ast/node_test.go`. |

LO-02, LO-03, LO-04, LO-05, LO-06, LO-08, LO-10 and LO-11, plus the rest of
ME-04, are recorded in `deferred-items.md` (141f125).

## Notes (not findings)

- `stc emit` comments out `VAR_GLOBAL` when a line comment precedes it, as
  in `// Global vars...VAR_GLOBAL`. This is the trivia bug that
  `deferred-items.md` already lists under 19-03, now reached through GVLs.
  It deserves priority because emitted GVL files no longer compile.
- The LSP builds its analysis order from map iteration (`pkg/lsp/document.go:108-114`).
  When two GVLs clash, which file gets the error can change between edits.
  This predates the phase, and GVLs make clashes more likely.
- The CI workflow step (`st-tests.yml`) and `.gitattributes` are valid.
  `* text=auto eol=lf` also normalises unlisted text types, which is
  intended.

---

_Reviewed: 2026-10-06_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
