# Phase 19: TwinCAT Declaration Syntax - Research

**Researched:** 2026-10-05
**Domain:** Go recursive-descent parser / AST / checker / tree-walking interpreter for IEC 61131-3 ST (TwinCAT dialect)
**Confidence:** HIGH (all code paths read and probed on HEAD 79eda31; TwinCAT semantics MEDIUM)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Attribute pragmas in the AST
- Add `ast.Attribute{NodeBase; Name string; Value string; HasValue bool}` and an `Attributes []*Attribute` field on VarDecl, StructMember, EnumValue, TypeDecl, ProgramDecl, FunctionBlockDecl, FunctionDecl, MethodDecl, PropertyDecl, InterfaceDecl and the new GVLDecl. Non-attribute pragmas (`{warning disable C0001}`, `{region}`, `{endregion}`, `{text ...}`) are kept verbatim as `ast.PragmaNode{Text}` in a sibling `Pragmas []*PragmaNode` field on the same nodes so fmt round-trips them; preprocessor directives stay with `pkg/preprocess`.
- The parser accumulates every Pragma token (and intervening comments/blank lines) until the next declaration token and attaches them to that declaration. The existing `skipPragmas()` becomes `collectPragmas() ([]*ast.Attribute, []*ast.PragmaNode)`; `parseDeclaration` no longer discards pragmas.
- Attribute names and values accept single or double quotes; `''` inside a single-quoted value unescapes to `'`. `{attribute 'qualified_only'}` without `:=` sets `HasValue=false`. The emitter always re-emits `{attribute 'name' := 'value'}` with single quotes and re-escapes `'` as `''`.
- JSON: `"attributes": [{"name": "OPC.UA.DA", "value": "1"}]` on each node; `stc parse --format json` on `ECT.TcGVL`-derived ST must show `TcLinkTo`, `OPC.UA.DA`, `OPC.UA.DA.StructuredType`, `qualified_only` on the right nodes.

#### GVL files
- New `ast.GVLDecl{NodeBase; Name *Ident; Blocks []*VarBlock; Attributes; Pragmas}` produced when a file's top level contains `VAR_GLOBAL [PERSISTENT] [RETAIN] [CONSTANT] ... END_VAR` blocks. All top-level VAR_GLOBAL blocks of one file aggregate into one GVLDecl; mixing with other declarations in the same file is allowed (the GVL takes the file name).
- The GVL name defaults to the file basename without extension (`ECT.st` -> `ECT`); a `--gvl-name` flag on `parse`, `check`, `fmt`, `emit` overrides it for single-file invocations. (TcGVL XML `Name=` will feed this in Phase 21.)
- Checker: GVL variables are registered in a global scope and resolvable as `GVL.x` always, and as bare `x` unless the GVL carries `{attribute 'qualified_only'}`; unqualified access to a qualified_only GVL variable is a new error diagnostic SEMA033 ("GVL 'X' is qualified_only; use X.var"). `VAR_GLOBAL CONSTANT` members are constants; `PERSISTENT`/`RETAIN` are recorded as flags on VarBlock (no runtime semantics yet).
- Interpreter: GVL declarations are tolerated (no crash) and `GVL.x` resolves when a GVL is present in the same analysis unit via a global env layer; full task-level instantiation and persistence are Phase 23. Keep this minimal and tested.

#### Wildcard AT addresses in structs and FBs
- `StructMember` gains `AtAddress *Ident` like VarDecl; `parseStructMember` accepts `name AT %I* : TYPE`.
- Checker SEMA031 (AT in FB) is emitted only for explicit addresses (`%IX0.0`, `%QW4`, ...) inside FUNCTION_BLOCK/FUNCTION; wildcard `%I*`/`%Q*`/`%M*` in FB `VAR`/`VAR_INPUT`/`VAR_OUTPUT` and in STRUCT members produce no diagnostic. An explicit address inside a STRUCT is SEMA031 as well (TwinCAT rejects it).
- `FB_ATV320`-style inputs (`i_uETA AT %I* : UINT` in VAR_INPUT) must check clean. Runtime binding of these members to process-image slots is out of scope (Phase 24, RUNT-07).

#### Empty call arguments and ACTIONs
- `parseCallArg` accepts `name :=` and `name =>` followed directly by `,` or `)`; the resulting `ast.CallArg` has `Value == nil` and is kept in the AST so `stc fmt` reproduces the source. Checker and interpreter skip nil-valued arguments (no assignment, no output write).
- New `ast.ActionDecl{NodeBase; Name *Ident; Body []Statement}` with `Actions []*ActionDecl` on ProgramDecl and FunctionBlockDecl. Parser accepts `ACTION name ... END_ACTION` blocks appearing after `END_PROGRAM` / `END_FUNCTION_BLOCK` (CODESYS text export form) and attaches them to the immediately preceding POU; a leading ACTION with no preceding POU is a diagnostic. TcPOU `<Action>` XML attachment is Phase 21.
- Checker: inside the owning POU, `A100_input();` resolves to the action as a parameterless call statement; actions share the POU's variable scope. From outside, `inst.ActionName()` on an FB instance is accepted as a call statement. Interpreter executes the action body in the owner's environment (same env as the POU body, so edge FBs and timers inside actions keep state).
- `stc emit` and `stc fmt` print actions after the POU in the same text form.

### Claude's Discretion
- Exact field names, diagnostic message wording, and where the pragma accumulator lives in the parser.
- Whether `Attributes` lives on `VarBlock` too (useful for `{attribute 'qualified_only'}` placed before a block).
- Coverage strategy: pkg/parser and pkg/interp gates are 95%; pkg/interp sits at 94.0% on main today, so this phase must add interpreter tests to clear the gate.

### Deferred Ideas (OUT OF SCOPE)
- Bit access `w.3`, named arguments in function-call expressions, qualified enum values and CASE labels, enum base types, `REF=`, `THIS^`/`SUPER^` -> Phase 20.
- TcPOU/TcGVL/TcDUT XML import (including `<Action>` elements and GVL `Name=`) -> Phase 21.
- Runtime GVL instantiation per task, PERSISTENT state file, typed AT binding -> Phases 22-24.
- Using `TcLinkTo` attribute values for anything -> Phase 24.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DIAL-01 | `{attribute 'name' := 'value'}` pragmas (single/double quoted, `''` escapes, blank lines before the declaration) retained on VarDecl, StructMember, EnumValue, TypeDecl and POU nodes, in `stc parse --format json`, round-trip through `stc fmt`/`stc emit` | Pragma is already a non-trivia lexer token (`lexer.go:206 scanPragma`); 6 skip sites identified; trivia-attachment interaction and JSON/emit/format sites mapped (Patterns 1-2, Pitfalls 1-3) |
| DIAL-02 | Top-level `VAR_GLOBAL [PERSISTENT] [RETAIN] [CONSTANT]` parses as GVL named from file or `--gvl-name`; `qualified_only` enforced | `parseVarBlock` already parses modifiers; resolver/checker integration via a `types.StructType` symbol (Pattern 3); qualified_only side-table to avoid name collisions (Pitfall 4) |
| DIAL-03 | `AT %I*`/`%Q*` accepted on STRUCT members and FB VAR/VAR_INPUT/VAR_OUTPUT without warnings; explicit `%IX..` in FBs keeps SEMA031 | `check.go:71 checkATAddresses` warns unconditionally for non-PROGRAM; `iomap.ParseAddress` already returns `IsWildcard` (Pattern 4) |
| DIAL-05 | Empty formal args (`PT := ,`, `Q => ,`) parse and are ignored at runtime | `stmt.go:404 parseCallArg`; checker already guards `arg.Value != nil`; interp input loop `interpreter.go:971` does not (Pattern 5) |
| DIAL-08 | `ACTION name ... END_ACTION` blocks parse and are callable as `name()` inside their POU | `ast.ActionDecl`, `KindActionDecl`, `KwAction`/`KwEndAction`, JSON, emit and format cases already exist; zero-arg calls parse as `AssignStmt{Target: CallExpr}` so dispatch goes through `checkCallExpr`/`evalCall` (Pattern 6). Real sources place ACTIONs **inside** the POU (Pitfall 6) |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Go only for compiler core; **stdlib only**, no new dependencies (CLAUDE.md "Core Technologies (No New Dependencies)").
- Parser must not require Java; parser must produce partial ASTs from broken code (error recovery is mandatory for LSP).
- All test execution deterministic, no wall-clock dependencies.
- Every CLI command supports `--format json` (the new `--gvl-name` flag must not break this; errors for misuse must be emitted in JSON when `--format json`).
- Must handle CODESYS extensions to parse production code.
- GSD workflow enforcement: edits go through `/gsd:execute-phase`.
- User memory: always use GitHub PRs with multi-platform CI and agent PR reviews; never ship without full branch coverage verified in CI.

## Summary

Every construct in this phase lands in code that already exists and is half-wired. The lexer emits one `Pragma` token per `{...}` and the parser discards it at six sites. `ast.ActionDecl`, `KindActionDecl`, `KwAction`/`KwEndAction`, the JSON case and emit/format printers already exist but the parser never builds an ActionDecl. `ast.PragmaNode` exists but is never constructed (and has no NodeKind). `VarBlock` already carries `IsConstant/IsRetain/IsPersistent` and `parseVarBlock` already parses `VAR_GLOBAL PERSISTENT RETAIN`. `iomap.ParseAddress` already flags wildcards. The checker already skips nil call-arg values. So the work is mostly wiring, not new machinery.

Three findings change the plan relative to CONTEXT.md. First, in the real flattened sources every ACTION sits **inside** its POU, before `END_PROGRAM`/`END_FUNCTION_BLOCK` (16 in ST301 MAIN, 4 inside `FB_ATV320` after its METHODs); only the one-construct probe `action.st` uses the after-POU form. The parser must accept both or the oracle files do not improve. Second, a zero-argument call `A1();` parses as an expression statement (`AssignStmt{Target: CallExpr}`), not a `CallStmt`, so action dispatch belongs in `checkCallExpr` and `evalCall`, not `checkCallStmt`/`execCallStmt`. Third, `CallStmt` JSON omits `args` entirely today, so criterion 4 cannot be verified through `stc parse --format json` until that is added.

Measured baselines: `stc parse` reports 2711 diagnostics on st301.st and 1430 on svncorecomponents.st. A textual simulation of Phase 19 (wrap GVLs, drop struct AT, fill empty args, neutralise actions) leaves roughly 715-965 on st301 and roughly 620-650 on svncorecomponents, and the residual classes are all Phase 20 items plus two gaps no requirement owns (array/struct initialisers, `BYTE#16#10`). Coverage under the CI method (`-coverpkg=./...`) is interp 94.02% (1494/1589, needs 1510), parser 95.88%, emit 95.55% (2 statements of slack), checker 94.77%.

**Primary recommendation:** Implement in five vertical slices (attributes, GVL, AT, empty args, actions), each touching lexer-free parser + AST/JSON + checker + interp + emit/format with tests, and start the phase with a pure-test plan that lifts pkg/interp above 95% using the uncovered stdlib error paths.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Attribute/pragma text parsing (`{attribute 'n' := 'v'}`) | Parser (`pkg/parser`) | Lexer (token boundaries only) | Lexer already produces a single Pragma token; splitting name/value is syntactic |
| Attribute attachment to declarations | Parser | AST (`Children`, JSON) | Attachment depends on what declaration follows; the AST just stores it |
| Attribute round-trip printing | Emit + Format | AST | Both printers re-emit from AST; no source slicing |
| GVL naming (file basename, `--gvl-name`) | Parser (default from filename) | CLI `cmd/stc` (override after parse) | Parser already receives the filename; override must happen after incremental cache |
| GVL symbol registration, qualified_only (SEMA033) | Checker (`resolve.go`, `check.go`) | Symbols (`pkg/symbols`) | Name resolution is checker work; symbol table stores |
| GVL runtime access (`GVL.x`) | Interpreter (`interpreter.go`, `scan.go`, `env.go`) | Test runner (`pkg/testing`) | Minimal global env layer; full instantiation is Phase 23 |
| Wildcard AT acceptance (SEMA031 rules) | Checker | `pkg/iomap` (`IsWildcard`) | Pure diagnostic policy |
| Empty call args | Parser | Interpreter (skip nil), Emit/Format | Checker already tolerant |
| ACTION parsing and attachment | Parser | AST | Two source shapes (inside POU, after POU) |
| ACTION resolution (`A1()`, `inst.A1()`) | Checker (scope symbol) | — | Shares POU scope |
| ACTION execution in owner env | Interpreter | — | Must run in POU/FB env so timers keep state |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go toolchain | go.mod `go 1.25.0`; local `go1.26.0` | Build and test | Project language [VERIFIED: go.mod, `go version`] |
| Go stdlib (`strings`, `path/filepath`, `unicode`) | stdlib | Attribute text parsing, GVL name from basename | CLAUDE.md forbids new deps [VERIFIED: CLAUDE.md] |
| `github.com/spf13/cobra` | v1.10.2 (already in go.mod) | `--gvl-name` flag on parse/check/fmt/emit | Already used by every command [VERIFIED: go.mod] |
| `github.com/stretchr/testify` | v1.11.1 (already in go.mod) | Unit test assertions | Already used throughout tests [VERIFIED: go.mod] |

### Supporting

None. No package is added in this phase.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-parsing attribute text from the Pragma token | Re-lexing the pragma body with `lexer.Tokenize` | Re-lexing turns `'` strings into StringLiteral tokens with `''` handling for free, but `{attribute "x"}` double-quoted names lex as WStringLiteral and unknown pragmas like `{warning disable C0139}` lex fine too. A 40-line hand scanner over the pragma text is simpler and fully coverable. Recommend hand scanner. |
| Extending the declaration span to include its attributes | Making `Attribute` a child node with its own span | Span extension changes LSP hover/definition ranges and diagnostic anchors. Child nodes keep existing spans and let comment trivia attach to the attribute (Pitfall 2). Recommend child nodes. |

**Installation:** none.

## Package Legitimacy Audit

No external packages are installed in this phase. slopcheck was not run because there is nothing to check.

| Package | Registry | Age | Downloads | Source Repo | slopcheck | Disposition |
|---------|----------|-----|-----------|-------------|-----------|-------------|
| (none) | — | — | — | — | — | — |

**Packages removed due to slopcheck [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
 .st file ──► preprocess (pipeline.Parse) ──► lexer.Tokenize
                                                │  Pragma tokens are NON-trivia
                                                ▼
                                  parser.parseSourceFile(filename)
             ┌──────────────────────────────────┼──────────────────────────────────┐
             ▼                                  ▼                                  ▼
  collectPragmas() before every        VAR_GLOBAL at top level            ACTION at top level or
  decl / var / member / enum value     ──► GVLDecl (name = basename)       inside PROGRAM/FB body
  ──► []*Attribute, []*PragmaNode      (one per file, blocks appended)     ──► POU.Actions
             │                                  │                                  │
             └──────────────► *ast.SourceFile ◄─┴──────────────────────────────────┘
                                   │  attachTrivia (comments ─► nearest node, incl. Attribute nodes)
         ┌──────────────┬──────────┼───────────────────┬─────────────────────┐
         ▼              ▼          ▼                   ▼                     ▼
  ast.MarshalNode   format/emit   cmd: --gvl-name    checker                interp
  (stc parse json)  (round-trip)  renames GVLDecl    resolve: GVL symbol    ScanCycleEngine / test runner:
                                  after parse/cache  (StructType), action   GVL env layer, actions
                                                     symbols in POU scope   defined on POU/FB Env
                                                     check: SEMA031 rules,  evalCall: action lookup
                                                     SEMA033, action bodies execCallStmt: skip nil args
```

### Recommended Change Map (file by file)

```
pkg/ast/
  var.go        Attribute node; PragmaNode gets a NodeKind; VarDecl/VarBlock gain Attributes, Pragmas
  decl.go       GVLDecl; Attributes/Pragmas on POUs/TypeDecl/Method/Property/Interface; Actions on Program/FB
  types.go      StructMember.AtAddress, Attributes, Pragmas; EnumValue.Attributes, Pragmas
  node.go       KindGVLDecl, KindAttribute, KindPragma appended at END of the iota list
  json.go       cases: Attribute, GVLDecl; fields: attributes, pragmas, actions, at_address on StructMember,
                args on CallStmt (missing today); helper marshalAttrs(m, attrs, pragmas)
pkg/parser/
  parser.go     skipPragmas -> collectPragmas; attribute text scanner; gvlNameFromFilename
  decl.go       parseDeclaration: Pragma (collect then dispatch), KwVarGlobal (GVL), KwAction;
                parseProgram loops like parseFunctionBlock to accept inner ACTIONs; FB loop: case KwAction
  var.go        attach pragmas to VarBlock and VarDecl
  types.go      parseStructMember AT; pragmas on struct members and enum values; parseTypeDecls attrs
  stmt.go       parseCallArg empty value; parseStatements stop at KwAction
  error.go      declarationStarts += KwVarGlobal, KwAction, Pragma
  trivia.go     nodeBaseOf: GVLDecl, Attribute, PragmaNode, StructMember(optional)
pkg/checker/
  diag_codes.go CodeGVLQualifiedOnly = "SEMA033"
  resolve.go    resolveGVL; action symbols in POU scope
  check.go      CheckBodies: GVL AT check, action bodies; checkATAddresses wildcard rule; struct AT check;
                checkIdent / checkCallExpr: SEMA033 instead of SEMA010
  usage.go      checkUnreachableDecl walks action bodies
pkg/symbols/    symbol.go: KindAction, KindGVL (no coverage gate on this package)
pkg/interp/
  env.go        actions map + DefineAction / LookupAction (returns owner env)
  interpreter.go evalCall: action dispatch + depth guard; evalMethodCall: action fallback;
                execCallStmt: skip nil arg.Value; Globals map for GVL.x in evalMemberAccess/execAssignMember
  scan.go       ScanCycleEngine: SetGlobals(gvls); define program actions on env
  fb_instance.go define FB (and EXTENDS parent) actions on inst.Env
pkg/emit, pkg/format  print attributes/pragmas, GVLDecl, StructMember AT, POU actions, empty args
pkg/testing/runner.go register GVLDecls with the interpreter (so ST suites can test GVL.x)
pkg/incremental/depgraph.go ScanFile: GVLDecl declares its name
cmd/stc/      parse.go, check.go, fmt_cmd.go, emit_cmd.go: --gvl-name (single file only)
```

### Pattern 1: Collect pragmas, then attach to the next declaration

**What:** Replace every `skipPragmas()`/`p.advance()` on Pragma with a collector returning attributes and other pragmas, and pass them into the node constructor that follows.

**Where pragmas are dropped today [VERIFIED: grep]:**

| Site | Context | New owner |
|------|---------|-----------|
| `decl.go:14` | Between top-level declarations | Next POU/TypeDecl/GVLDecl (or first VarBlock of the GVL) |
| `var.go:22,25` | Before VAR blocks | VarBlock (recommend `Attributes` on VarBlock: yes) |
| `var.go:84` | Before a VarDecl | VarDecl |
| `types.go:179` | Before a struct member | StructMember |
| `decl.go:237` | Inside INTERFACE | MethodSignature/PropertySignature (not required; may keep dropping) |
| `stmt.go:19` | Between statements | Not a declaration. Keep skipping (see Open Question 3) |
| `parseEnumType` | Not handled at all; `{attribute}` before an enum value is a parse error today | EnumValue |
| `parseFunctionBlock` loop | Pragmas before METHOD/PROPERTY swallowed by `parseStatements` | MethodDecl/PropertyDecl |

**Example (shape, adapted from existing parser style):**
```go
// parser.go
func (p *Parser) collectPragmas() (attrs []*ast.Attribute, other []*ast.PragmaNode) {
	for p.at(lexer.Pragma) {
		tok := p.advance()
		if a, ok := parseAttributeText(tok); ok {
			attrs = append(attrs, a)
		} else {
			other = append(other, &ast.PragmaNode{
				NodeBase: ast.NodeBase{NodeKind: ast.KindPragma,
					NodeSpan: ast.SpanFrom(astPos(tok.Pos), astPos(tok.EndPos))},
				Text: tok.Text,
			})
		}
	}
	return attrs, other
}
```

`parseAttributeText` scans `tok.Text`: `{`, optional spaces, case-insensitive `attribute`, spaces, a quoted name (`'` or `"`), optional `:=` and a quoted value. In a `'`-quoted value `''` becomes `'`; apply the same doubling rule for `"`. If the shape does not match, return `ok=false` so the text survives verbatim as a PragmaNode.

Real-data survey of all 1550 attributes in the probes [VERIFIED: grep over stc-probes]: every one matches `{attribute ['"]name['"]( := '...')?}`; none contains `}` inside a value; 3 contain `''`; the longest is 2988 bytes; values contain non-ASCII (em dash). Names seen: TcLinkTo (675), OPC.UA.DA (370), OPC.UA.DA.Access (299), OPC.UA.DA.StructuredType (263), OPC.UA.DA.Description (185), qualified_only (59 single-quoted, 3 double-quoted), to_string (9), strict (9). Non-attribute pragmas: `{warning disable C0139}` (15), all at the start of ACTION bodies.

### Pattern 2: Attribute as a child node (trivia-safe)

**What:** Put `Attributes` and `Pragmas` first in the owner's `Children()`, and add `*ast.Attribute`/`*ast.PragmaNode` cases to `nodeBaseOf` in `pkg/parser/trivia.go`.

**Why:** `attachTrivia` maps each comment to the innermost node containing the next non-trivia token. A Pragma token is non-trivia. Today a comment above `{attribute ...}` lands on the enclosing VarBlock, so the formatter would print it above `VAR_GLOBAL` once attributes are re-emitted. With Attribute nodes in the walk, that comment attaches to the Attribute and prints in place. The ECT GVL is full of this shape (`// ==== Device 1 (EtherCAT) ====` then `{attribute 'TcLinkTo' ...}` then the var).

Printers emit, in order: owner leading comments, each attribute/pragma with its own leading trivia on its own line at the owner's indent, then the owner. Re-emit attributes as `{attribute 'name' := 'value'}` with `'` doubled; `HasValue=false` prints `{attribute 'name'}`.

### Pattern 3: GVL in the checker as a struct-typed symbol

**What:** In `resolve.go`, add `resolveGVL(d *ast.GVLDecl)`:
1. Build `&types.StructType{Name: gvlName, Members: ...}` from all blocks (resolve each VarDecl type with the existing `resolveTypeSpec`).
2. Insert a global symbol `{Name: gvlName, Kind: symbols.KindGVL (new) or KindVariable, Type: structType}`. Redeclaration against an existing POU/type name uses the existing CodeRedeclared path.
3. If the GVL is **not** qualified_only, also insert each variable into the global scope (`Kind: KindVariable, ParamDir: ast.VarGlobal`) so bare `x` resolves.
4. If it **is** qualified_only, record `qualifiedOnly[upper(var)] = append(..., gvlName)` in a side table (on Resolver/Table, or a `symbols.Table` field) and do **not** insert the bare names.

Then `checkIdent` and `checkCallExpr` fall back on a miss: if the name is in the qualified_only table, emit SEMA033 `GVL 'ECT' is qualified_only; use ECT.x` instead of SEMA010.

**Why it works with no new checker machinery:** `checkIdent("ECT")` returns the StructType and the existing `checkMemberAccessExpr` StructType case resolves `ECT.X` and nested `ECT.X.q_wDigitalInputs`. `isInterfaceVar` already treats `VarGlobal` as an interface point, so no unused-variable warnings fire.

qualified_only detection: the GVL is qualified_only if any attribute on the GVLDecl or on any of its VarBlocks is `qualified_only` (case-insensitive). With aggregation, the attribute before the first VAR_GLOBAL becomes `GVLDecl.Attributes` and attributes before later blocks go to those `VarBlock.Attributes`, so printing reproduces the source.

AT checks for GVLs: call `checkATAddresses(blocks, "GVL")` and treat "GVL" like "PROGRAM" (AT is valid in VAR_GLOBAL per IEC and CLAUDE.md).

### Pattern 4: SEMA031 policy

```go
// check.go checkATAddresses, after ParseAddress succeeds
if pouType != "PROGRAM" && pouType != "GVL" && !addr.IsWildcard {
	c.diags.Warnf(pos, CodeATNotAllowedHere, ...)
}
```
Add a struct pass in `CheckBodies` over `*ast.TypeDecl` whose Type is `*ast.StructType`: invalid format is SEMA030, explicit address is SEMA031 warning, wildcard is silent. Keep SEMA031 a **warning** (it is `Warnf` today and existing tests key on the code, not severity).

Wording: Beckhoff InfoSys says explicit AT on structure or FB components is allowed but "all instances use the same memory" [CITED: infosys.beckhoff.com/content/1033/tc3_plc_intro/11948825611.html]. CONTEXT.md's rationale "TwinCAT rejects it" is inaccurate. The locked behaviour (warn) is still sensible; recommend the message say that all instances share the address.

`fbat.st` today: `stc check` gives 2 SEMA031 warnings for `AT %I*`/`AT %Q*` in an FB [VERIFIED: probe]. Existing tests `TestATAddressNotAllowedInFunctionBlock/Function` use `%IX0.0` and stay green.

### Pattern 5: Empty call arguments

Parser (`stmt.go` parseCallArg): after consuming `:=` or `=>`, if `p.at(lexer.Comma) || p.at(lexer.RParen)`, build the CallArg with `Value: nil` (an untyped nil interface, never a typed nil pointer). Span ends at the operator token.

Expression-level detection: `isNamedArgCall` (`expr.go:399`) only checks `( Ident :=|=>`, so `t(IN := b, PT := , ...)` is already routed to the statement parser. No change there.

Checker: `checkCallStmt` already guards `if arg.Value != nil` (`check.go:408`) and still validates the parameter name. No change except tests.

Interpreter: the input loop at `interpreter.go:963-976` calls `evalExpr(env, nil)`, which returns "unsupported expression type: <nil>". Add `if arg.Value == nil { continue }`. The in-out and output loops already skip nil.

Printers: both print `PT := ` then nothing, giving `PT := , Q => );`. That reparses and is idempotent. Optionally trim the trailing space when Value is nil.

JSON: add `args` to the `CallStmt` case in `json.go` (missing today [VERIFIED: probe JSON shows only `callee`]); CallArg JSON already omits a nil value.

Real-data note: TwinCAT auto-complete writes `PT:= ,` and `ET=> );` [VERIFIED: probes]; the lexer tokenises `:=` and `=>` independent of spacing.

### Pattern 6: ACTIONs

**Parser, two shapes (both required):**
1. **Inside the POU** (all 16+4 real occurrences [VERIFIED: st301.st line 1317, svncorecomponents.st lines 4228-4716]):
   ```
   PROGRAM MAIN VAR ... END_VAR
   A050_ModbusCall(); ...
   ACTION A050_ModbusCall
     ...
   END_ACTION
   END_PROGRAM
   ```
   and `FUNCTION_BLOCK FB_ATV320 ... METHOD...END_METHOD ... ACTION coe ... END_ACTION ... END_FUNCTION_BLOCK`.
   `parseProgram` currently calls `parseStatements(KwEndProgram)`; rewrite it as a loop like `parseFunctionBlock` with `case lexer.KwAction`. Add `lexer.KwAction` to the FB loop and to both `parseStatements` stop sets.
2. **After the POU** (`action.st`, CODESYS text export): in `parseSourceFile`, when the next token (after collected pragmas) is `KwAction`, parse it and append to the last declaration if it is `*ast.ProgramDecl` or `*ast.FunctionBlockDecl`; otherwise emit a P-code error ("ACTION without a preceding PROGRAM or FUNCTION_BLOCK") and keep it as a top-level `*ast.ActionDecl` so fmt/emit (which already handle top-level ActionDecl) still print it.

`parseAction`: `ACTION`, ident, optional `:` (the existing emitters print `ACTION name:`), optional `;`, then `parseStatements(KwEndAction)`, `END_ACTION`, optional `;`. A `{warning ...}` pragma at the start of the body is skipped by parseStatements today.

Add `Actions` to `ProgramDecl.Children()` and `FunctionBlockDecl.Children()` so trivia, LSP `Inspect`, and lint walks see them; add `"actions"` to JSON.

**Call shape:** `A1();` parses as `AssignStmt{Target: CallExpr{Callee: Ident A1}, Value: nil}` [VERIFIED: `stc parse --format json`]. `inst.A1();` parses as `AssignStmt{Target: CallExpr{Callee: MemberAccessExpr}}`.

**Checker:** in `resolveProgram`/`resolveFunctionBlock`, after `resolveVarBlocksInScope`, insert each action into `pouScope` as `{Name, Kind: symbols.KindAction, Type: &types.FunctionType{Name: act, ReturnType: types.TypeVOID}}`. A clash with a variable name yields the existing redeclaration error. `checkCallExpr` then resolves `A1()` through `checkUserFuncCall` with zero params. In `CheckBodies`, check each action body with `c.checkPOUBody(pouName, action.Body)` so variables used only inside actions are marked used. `inst.A1()` is already silently accepted because `exprName` returns "" for member-access callees (`check.go:804`); no change is needed to satisfy "accepted".

**Interpreter (recommended: env-based):**
```go
// env.go
type Env struct { /* existing */ actions map[string]*ast.ActionDecl }
func (e *Env) DefineAction(a *ast.ActionDecl) { ... key := strings.ToUpper(a.Name.Name) ... }
func (e *Env) LookupAction(name string) (*ast.ActionDecl, *Env) { /* walk parents, return owner */ }

// interpreter.go evalCall, Ident branch, before ADR/REF/LocalFunctions
if act, owner := env.LookupAction(calleeName); act != nil {
	return Value{}, interp.execAction(owner, act) // ErrReturn swallowed; depth guard
}
```
- `ScanCycleEngine.initializeEnv` defines `program.Actions` on `e.env`.
- `newUserFBInstanceDepth` defines `decl.Actions` (and the EXTENDS parent's) on `inst.Env`.
- `evalMethodCall`: when `findMethod` returns nil, look for an action on `fbInst.Decl`/`ParentDecl` and execute it in `fbInst.Env`.
- The body runs in the **owner** env, so `R_TRIG`/`TON` instances declared in the POU keep state across scans.
- Add a call-depth counter on Interpreter (e.g. 256) that returns a `RuntimeError`. Nothing guards recursion today [VERIFIED: grep], and a self-calling action would otherwise blow the Go stack fatally.

### Pattern 7: GVL in the interpreter (minimal)

- Add `Globals map[string]*Env` (upper GVL name to env) to `Interpreter`, plus `RegisterGVL(decl *ast.GVLDecl)` that builds the env with the same zero-value/init logic as `initializeEnv` (stdlib FBs via `StdlibFBFactory`, user FBs via `FBDecls`).
- `evalMemberAccess`/`execAssignMember`: when `target.Object` is an `*ast.Ident` that is **not** found in env and names a GVL, read or write the member in that GVL env. Nested `GVL.s.a := 1` works because struct values are shared Go maps.
- Unqualified access: make non-qualified_only GVL envs the parent of the program env (chain them). The checker already enforces qualified_only, so the interpreter need not.
- `ScanCycleEngine.SetGlobals(gvls)` before the first Tick; `cmd/stc/sim_cmd.go` passes GVLDecls from the file; `pkg/testing/runner.go` collects GVLDecls into `fileContext` and registers them per test case. This makes GVLs testable from ST suites.

### Pattern 8: GVL name and `--gvl-name`

- Default: `strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))`, sanitised to an identifier (non `[A-Za-z0-9_]` to `_`, leading digit gets `_` prefix, empty gives `GVL`). MCP passes `input.st`, so its GVL is named `input`.
- Override after parsing, not inside `parser.Parse`. `stc check` goes through `incremental.IncrementalAnalyzer`, which caches parse results keyed by content hash; a rename inside the parser would be invisible on a cache hit. A helper such as `ast.SetGVLName(file, name) bool` called in each command keeps the 3 `parser.Parse` call sites unchanged.
- `--gvl-name` with more than one input file is a usage error, reported in JSON when `--format json`.

### Anti-Patterns to Avoid

- **Inserting qualified_only GVL variables into the global scope:** ST301 has `WA01 : FB_Wagon` in both EPW01 and FPW01 GVLs; bare insertion produces false redeclaration errors. Use the side table.
- **Inserting new NodeKinds in the middle of the iota list:** append at the end so existing numeric values do not shift.
- **Dispatching actions in `execCallStmt`:** zero-arg calls never reach it.
- **Renaming the GVL inside the incremental parse path:** stale on cache hits.
- **Silently dropping a pragma that precedes `END_VAR`/`END_STRUCT`:** attach it to the block as a trailing pragma or at least do not crash. It is rare in real data.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| I/O address validation and wildcard detection | A new regex in the checker | `iomap.ParseAddress` and `IOAddress.IsWildcard` | Already handles `%I*`, `%QX0.0`, `%MD48` and error messages (SEMA030) |
| GVL member type checking | A GVL-specific member-access path | `types.StructType` symbol plus existing `checkMemberAccessExpr` | Nested access, unknown-member SEMA024 come free |
| Zero values for GVL variables | New zero-value code | `zeroFromTypeSpecWith`, `StdlibFBFactory`, `NewUserFBInstance` | Same code as program and FB envs |
| Comment placement around pragmas | Ad hoc printer logic | `attachTrivia` with Attribute nodes in `nodeBaseOf` | Deterministic, offset-based |
| String escape in printers | Custom escaping per printer | One `ast.(*Attribute).String()` or shared helper | Emit and format must agree for idempotence |

**Key insight:** almost every subsystem this phase touches already has a generic mechanism; adding a new node kind and wiring it in is cheaper and safer than special paths.

## Runtime State Inventory

Not a rename/refactor/migration phase. Omitted.

## Common Pitfalls

### Pitfall 1: PragmaNode has no NodeKind
**What goes wrong:** `ast.PragmaNode` exists but nothing sets `NodeKind`, so `Kind()` returns 0 = `SourceFile` and JSON prints `"kind": "SourceFile"`.
**How to avoid:** add `KindPragma`, `KindAttribute`, `KindGVLDecl` and names in `nodeKindNames`; set them in the constructors. `pkg/ast/node_test.go` likely checks names; update it.

### Pitfall 2: Comments before an attribute move to the wrong place
**What goes wrong:** with attributes printed, comments that preceded them reappear above the enclosing VAR block.
**How to avoid:** Pattern 2. Add a format test with `// c1`, `{attribute ...}`, `// c2`, `x : BOOL;` and assert order and idempotence.

### Pitfall 3: VarBlock modifier order is not preserved
**What goes wrong:** source `VAR_GLOBAL PERSISTENT RETAIN` prints as `VAR_GLOBAL RETAIN PERSISTENT` (printers emit CONSTANT, RETAIN, PERSISTENT in fixed order). Output is idempotent and semantically equal but not byte-identical.
**How to avoid:** accept it and state it in tests (assert idempotence, not byte equality), or record modifier order on VarBlock. Recommend accepting.

### Pitfall 4: Flattened probes merge many GVLs into one
**What goes wrong:** st301.st contains 12+ GVLs separated by `// ---- ST301/GVLs/X.TcGVL` comments. Under the locked decision they aggregate into one GVL named `st301`, so `stc check` on the flattened file will report redeclarations (duplicate `WA01`) and unresolved `ECT.x` paths.
**How to avoid:** measure the oracle with `stc parse` only in this phase. For check-level verification use small per-GVL fixtures (`ECT.st`, `GVL_LocalSettings.st`). Real per-file names arrive with TcGVL import in Phase 21.

### Pitfall 5: Coverage gates are tight
**What goes wrong:** CI runs `go test -coverpkg=./... ./...` and gates per package. Numbers measured that way on HEAD:

| Package | Covered | Gate | Headroom |
|---------|---------|------|----------|
| pkg/interp | 1494/1589 = 94.02% | 95 | needs +16 covered statements before new code |
| pkg/emit | 515/539 = 95.55% | 95 | about 2 statements |
| pkg/checker | 688/726 = 94.77% | 94 | about 5 statements |
| pkg/parser | 838/874 = 95.88% | 95 | about 7 statements |
| pkg/lexer | 228/234 = 97.44% | 95 | about 5 statements |

Per-package `go test -cover` shows parser and interp at 93.0%; that number is **not** what CI gates.
**How to avoid:** every new branch in parser, emit, checker and interp needs a test in the same plan. Lift interp first with cheap error-path tests (see Validation Architecture).

### Pitfall 6: ACTIONs inside the POU
**What goes wrong:** implementing only the after-POU form leaves ST301's MAIN and FB_ATV320 failing exactly as today.
**How to avoid:** Pattern 6 shape 1, with fixtures copied from both real shapes.

### Pitfall 7: Zero-arg action calls are not CallStmts
**What goes wrong:** handling actions in `checkCallStmt`/`execCallStmt` has no effect on `A1();`.
**How to avoid:** dispatch in `checkCallExpr` and `evalCall`/`evalMethodCall`.

### Pitfall 8: Criterion 4 cannot be checked end to end through `stc check` with TON
**What goes wrong:** `stc check` reports `TON has no input parameter "IN"` today (bug B1, requirement RUNT-08, Phase 20) [VERIFIED: probe m.st].
**How to avoid:** verify empty-arg checking with a user-defined FB, and verify runtime with TON through the interpreter or an ST suite (`stc test` does not run the checker).

### Pitfall 9: `stc test` never executes PROGRAMs
**What goes wrong:** the test runner collects TYPE/FB/FUNCTION/INTERFACE/TEST_CASE only; PROGRAM actions cannot be exercised from `*_test.st`.
**How to avoid:** test PROGRAM actions through `interp.NewScanCycleEngine` in Go tests; test FB actions (internal `A1()` and external `inst.A1()`) in an ST suite.

### Pitfall 10: Recovery swallows the next declaration
**What goes wrong:** `declarationStarts` (`error.go:24`) lacks `KwVarGlobal`, `KwAction`, and `Pragma`, so panic-mode recovery can skip past them and eat a following GVL, action, or the attributes of the next POU.
**How to avoid:** add all three.

## Code Examples

### Existing pragma skip to replace (`pkg/parser/decl.go:12`)
```go
case lexer.Pragma:
	// Skip pragmas between declarations (attach as trivia in future)
	p.advance()
	if !p.atEnd() {
		return p.parseDeclaration()
	}
	return nil
```

### Existing unconditional SEMA031 (`pkg/checker/check.go:90`)
```go
if pouType != "PROGRAM" {
	c.diags.Warnf(pos, CodeATNotAllowedHere,
		"AT address declarations are only valid in PROGRAM blocks, not %s", pouType)
}
```

### Existing input loop that crashes on nil (`pkg/interp/interpreter.go:963`)
```go
for _, arg := range s.Args {
	if arg.IsOutput { continue }
	if arg.Name == nil { continue }
	argVal, err := interp.evalExpr(env, arg.Value) // nil -> "unsupported expression type: <nil>"
```

### Acceptance fixtures (from the real probes)
```
TYPE ST_EL1008 :
STRUCT
	{attribute 'OPC.UA.DA.Access' := '1'}
	I1 AT %I* : BOOL;
	I2 : BOOL;
END_STRUCT
END_TYPE
```
```
{attribute "qualified_only"}
VAR_GLOBAL PERSISTENT RETAIN
	{attribute 'OPC.UA.DA.Description' := 'The slave controller''s own lost-link count'}

	rFullPalletWagonSpeed : REAL := 25.0;
END_VAR
```
```
TYPE E_State :
(
  {attribute 'OPC.UA.DA.Description' := 'Ready'}
  rdy := 2,
  nst := 3
);
END_TYPE
```
The enum example parses only for the attribute; `(...) UINT` base types stay Phase 20.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Pragmas discarded by parser | Attributes as AST nodes | This phase | Phase 24 (`TcLinkTo`) and Phase 28 (OPC UA exposure) read them from the AST |
| `ACTION name:` printed by emit/format | `ACTION name` (colon optional on input) | This phase | Existing coverage tests only assert the substring `ACTION MyAction`, so they keep passing |

**TwinCAT semantics relied on:**
- `qualified_only` forces access through the GVL namespace [CITED: infosys.beckhoff.com/content/1031/tc3_plc_intro/12049335819.html].
- Actions have no own variables and run on the parent POU's data; called as `A_Inputs();` inside the POU [CITED: industrialmonitordirect.com knowledge base; MEDIUM, not Beckhoff primary].
- Explicit AT on FB/struct components is allowed but shared across instances; `*` placeholders are recommended [CITED: infosys.beckhoff.com/content/1033/tc3_plc_intro/11948825611.html].

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Calling an FB action from outside with input assignments (`inst.Act(IN := x)`) is not needed for the probes | Pattern 6 | Low. Not seen in probes; would need CallStmt member-callee support |
| A2 | `RETURN` inside an action returns from the action only | Pattern 6 | Low. Behaviour matches methods today |
| A3 | Double-quoted attribute values use `""` doubling | Pattern 1 | Very low. No real data uses double-quoted values |
| A4 | The phase-19 residual estimate (st301 about 715-965, svncore about 620-650) is approximate | Summary | Medium. It is a textual simulation, not an implementation. Use the residual-classification gate below rather than a hard number |

## Open Questions (RESOLVED)

1. **Exact diagnostic target for the oracle files.**
   - What we know: 2711 (st301) and 1430 (svncore) today; simulation suggests about a 2-3x reduction.
   - Recommendation: gate on classification, not a count. After the phase, no `stc parse` diagnostic on st301.st or svncorecomponents.st may sit on a line containing `{attribute`, `{warning`, `VAR_GLOBAL`, `ACTION`, `END_ACTION`, ` AT %`, `:= ,`, `:=,`, `=> ,`, `=>,` or `=> )`. Record the remaining count in the SUMMARY for Phase 20.
   - **RESOLVED:** Adopted as the classification gate in 19-09 (Task 1 owned-token check in TestTwinCATProbeOracle, Task 3 records before/after counts against the 19-01 baseline). No numeric target.

2. **Unowned parse gaps found while probing.**
   - Array and struct initialisers `:= [(a := 1, ...), ...]` and `s : ST := (a := 1)` do not parse at all [VERIFIED: probe], although ANALYSIS.md 2.1 item 11 says they do. RUNT-06 (Phase 22) only covers applying them.
   - `BYTE#16#10` (typed literal with a based number) fails to parse.
   - Recommendation: not Phase 19 scope. Flag both to the roadmap owner so Phase 20 (whose success criterion 4 is zero parse errors) absorbs them.
   - **RESOLVED:** Out of Phase 19 scope. 19-09 Task 3 buckets every remaining oracle diagnostic by construct (including array/struct initialisers and typed based literals such as BYTE#16#10) and hands the buckets to Phase 20 in the SUMMARY.

3. **Statement-level pragmas.**
   - `{warning disable C0139}` appears 15 times, always at the top of an ACTION body. It is skipped today and remains lost in fmt.
   - Recommendation: allowed under the locked decision (declarations only). Optionally keep a `Pragmas` slice on ActionDecl for pragmas that precede the first statement, which covers every real occurrence for a few lines of code.
   - **RESOLVED:** Adopted. 19-02 adds ActionDecl.Pragmas (with JSON), and 19-07 fills it in parseAction via collectPragmas and prints it as the first body line in fmt/emit.

4. **Ordering of aggregated GVL blocks in fmt.**
   - When VAR_GLOBAL blocks are interleaved with POUs, fmt prints all blocks at the GVL's first position. Idempotent but reorders.
   - Recommendation: accept and document.
   - **RESOLVED:** Accepted by design. 19-05 Task 1 aggregates all blocks into one GVLDecl printed at the first block's position; output is idempotent and the reordering is documented in the 19-05 SUMMARY.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build, tests | yes | go1.26.0 (go.mod 1.25.0) | — |
| Probe sources `/Users/jonb/Projects/beckhoff-docs/stc-probes/` | acceptance oracle | yes (local only, outside repo) | — | Copy the one-construct probes into `testdata/`; CI cannot see the large files |
| vladopajic/go-test-coverage | CI gate | CI only | v2 action | Reproduce locally with the merge script below |

**Missing dependencies with no fallback:** none.
**Note:** the large flattened files are customer code outside the repo. Do not commit them. Commit minimal fixtures that reproduce each construct.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` + testify v1.11.1; ST suites via `stc test` |
| Config file | `.testcoverage.yml` (gates), `.github/workflows/{ci,coverage,st-tests}.yml` |
| Quick run command | `go test ./pkg/parser/ ./pkg/ast/ ./pkg/checker/ ./pkg/interp/ ./pkg/emit/ ./pkg/format/ -count=1` |
| Full suite command | `go test ./... -count=1 && go run ./cmd/stc test tests/` |
| Gate-equivalent coverage | `go test -coverprofile=unit.txt -covermode=atomic -coverpkg=./... ./... -count=1` then merge duplicate blocks per package (any count > 0 is covered) and compare with `.testcoverage.yml` |

CI facts: ci.yml runs build, `go test ./...`, vet and `go run ./cmd/stc test tests/` on the OS matrix. coverage.yml runs the coverpkg profile plus exec-based `cmd/stc` coverage and enforces the gates. st-tests.yml runs five named suites and `TestCorpusParse`; a new suite directory under `tests/` is picked up by ci.yml's `stc test tests/` but **not** by st-tests.yml unless a step is added.

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DIAL-01 | attribute text scanner: single/double quotes, `''`, no value, non-attribute pragma | unit | `go test ./pkg/parser -run TestAttribute -count=1` | no, Wave 0 |
| DIAL-01 | attributes attach to VarDecl, StructMember, EnumValue, TypeDecl, POU, VarBlock, GVL; blank line before decl | unit | `go test ./pkg/parser -run TestPragmaAttach` | no |
| DIAL-01 | JSON contains `attributes` with name/value on each node | unit | `go test ./pkg/ast -run TestMarshalAttributes` | no |
| DIAL-01 | fmt and emit re-emit attributes; fmt twice is identical; comment order kept | unit | `go test ./pkg/format ./pkg/emit -run Attribute` | no |
| DIAL-01 | `stc parse --format json` shows attributes | exec (cmd) | `go test ./cmd/stc -run TestParseJSONAttributes` | no |
| DIAL-02 | top-level VAR_GLOBAL (with PERSISTENT RETAIN CONSTANT) becomes one GVLDecl named from file | unit | `go test ./pkg/parser -run TestGVL` | no |
| DIAL-02 | `--gvl-name` overrides; rejected with 2 files | exec | `go test ./cmd/stc -run TestGVLNameFlag` | no |
| DIAL-02 | `GVL.x` resolves; bare `x` OK when not qualified_only; SEMA033 when qualified_only; duplicate names across two qualified_only GVLs do not error | unit | `go test ./pkg/checker -run TestGVL` | no |
| DIAL-02 | interpreter reads/writes `GVL.x` and nested `GVL.s.a`; unqualified access | unit + ST | `go test ./pkg/interp -run TestGVL`; `go run ./cmd/stc test tests/twincat_dialect` | no |
| DIAL-03 | struct member `AT %I*` parses, JSON `at_address`, printers emit it | unit | `go test ./pkg/parser ./pkg/emit ./pkg/format -run StructAT` | no |
| DIAL-03 | wildcard in FB VAR/VAR_INPUT/VAR_OUTPUT and struct: no diagnostics; `%IX0.0` in FB and struct: SEMA031 | unit | `go test ./pkg/checker -run TestATWildcard` | partially (existing FB/FUNCTION tests) |
| DIAL-05 | `PT := ,`, `Q => ,`, `ET => )` parse with nil Value; JSON `args` present | unit | `go test ./pkg/parser ./pkg/ast -run EmptyArg` | no |
| DIAL-05 | checker validates names of empty args, no type error | unit | `go test ./pkg/checker -run EmptyArg` | no |
| DIAL-05 | TON call with empty args runs identically to omitted args | unit + ST | `go test ./pkg/interp -run EmptyArg`; ST suite | no |
| DIAL-08 | ACTION inside PROGRAM, inside FB after METHODs, after END_PROGRAM, after END_FUNCTION_BLOCK, optional colon; orphan ACTION diagnostic | unit | `go test ./pkg/parser -run TestAction` | no |
| DIAL-08 | `A1()` resolves in POU; variable used only in action not reported unused; `inst.A1()` accepted | unit | `go test ./pkg/checker -run TestAction` | no |
| DIAL-08 | MAIN of action calls runs via ScanCycleEngine; R_TRIG/TON in action keep state across Ticks; recursion guard | unit | `go test ./pkg/interp -run TestAction` | no |
| DIAL-08 | FB action called internally and as `inst.A1()` from TEST_CASE | ST | `go run ./cmd/stc test tests/twincat_dialect` | no |
| all | probe residual classification (Open Question 1) | manual/local script | `/tmp/stc-probe parse <probe>` plus grep | local only |

### Interp coverage lift (do first)

Needs at least 16 newly covered statements before any new interp code. Cheapest targets, all direct calls to `StdlibFunctions[...]` with bad arguments [VERIFIED: uncovered blocks in merged profile]:

| File | Uncovered statements | What to call |
|------|---------------------|--------------|
| `stdlib_array.go` | 10 | `UPPER_BOUND` with no args, a non-array, a non-int dim, dim 2, and a valid array (only the ST suite covers it today, which the gate does not count) |
| `stdlib_convert.go` | 7 | `UDINT_TO_REAL`, `REAL_TO_LREAL`, `LREAL_TO_REAL`, `TIME_TO_REAL` and others at lines 40-120 with zero args |
| `stdlib_string.go` | 6 | `DELETE`/`REPLACE`/`INSERT` with too few args and out-of-range positions (lines 114-165) |
| `stdlib_math.go` | 5 | error branches at lines 89-121 |
| `scan.go` | 5 | `readIOValue`/`writeIOValue` byte and dword sizes (lines 124-140, 213-216) |

That is about 33 statements, which also leaves room for any hard-to-reach defensive branch in the new action/GVL code.

### Sampling Rate
- **Per task commit:** quick run command above.
- **Per wave merge:** full suite plus the gate-equivalent coverage computation.
- **Phase gate:** full suite green, all five gated packages at or above threshold under the coverpkg method, and probe residual classification clean.

### Wave 0 Gaps
- [ ] `pkg/interp/stdlib_errors_test.go` for the coverage lift.
- [ ] `pkg/parser/pragma_test.go`, `gvl_test.go`, `action_test.go`, `callarg_test.go`.
- [ ] `pkg/checker/gvl_test.go`, `action_test.go`, AT wildcard cases in `check_test.go`.
- [ ] `pkg/interp/action_test.go`, `gvl_test.go`, empty-arg case.
- [ ] `pkg/ast` JSON tests for Attribute, GVLDecl, actions, CallStmt args.
- [ ] `pkg/emit`, `pkg/format` round-trip and idempotence tests with attributes, GVLs, struct AT, actions, empty args.
- [ ] `tests/twincat_dialect/*_test.st` and a matching step in st-tests.yml.
- [ ] Fixtures in `testdata/` copied from the one-construct probes (`gvl1.st`, `gvl2.st`, `structat.st`, `structpragma.st`, `action.st`, `fbat.st`, plus an in-POU action fixture).

## Security Domain

`security_enforcement` is absent from `.planning/config.json`, so treated as enabled. This phase is offline source processing with no network, auth, sessions or crypto.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | yes | Bounded, panic-free parsing of untrusted ST; existing fuzz tests in `pkg/lexer/fuzz_test.go` |
| V6 Cryptography | no | — |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unterminated `{` pragma or huge attribute value | Denial of service | Lexer already stops at EOF; attribute scanner must be linear and index-safe |
| Self-recursive ACTION | Denial of service (fatal Go stack overflow) | Interpreter call-depth guard returning RuntimeError |
| Parser infinite loop on new recovery paths | Denial of service | Keep the existing "no forward progress, advance" guards in every new loop; extend adversarial tests in `pkg/parser/adversarial_test.go` |
| GVL name from hostile filename | Tampering of output | Sanitise to an identifier |

## Sources

### Primary (HIGH confidence)
- Codebase at HEAD 79eda31: `pkg/parser/{parser,decl,var,types,stmt,expr,error,trivia}.go`, `pkg/ast/{decl,var,types,stmt,node,json,visitor}.go`, `pkg/checker/{resolve,check,usage,diag_codes}.go`, `pkg/interp/{interpreter,scan,env,fb_instance,stdlib_*}.go`, `pkg/emit/emit.go`, `pkg/format/format.go`, `pkg/testing/runner.go`, `cmd/stc/*.go`, `cmd/stc-mcp/tools.go`, `.testcoverage.yml`, `.github/workflows/*.yml`.
- Probe runs of a fresh `go build -o /tmp/stc-probe ./cmd/stc` against `/Users/jonb/Projects/beckhoff-docs/stc-probes/*.st`.
- CI-equivalent coverage profile (`-coverpkg=./...`) computed locally, 2026-10-05.

### Secondary (MEDIUM confidence)
- [Beckhoff InfoSys: AT declaration](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/11948825611.html): placeholders recommended; explicit AT on FB/struct components shares memory across instances.
- [Beckhoff InfoSys: Global variable lists](https://infosys.beckhoff.com/content/1031/tc3_plc_intro/12049335819.html): `qualified_only` enforces GVL namespace.

### Tertiary (LOW confidence)
- [Industrial Monitor Direct: Calling Beckhoff TwinCAT Actions from Programs](https://industrialmonitordirect.com/blogs/knowledgebase/calling-beckhoff-twincat-actions-from-programs): actions share parent POU variables, called as `name();`.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH. No new dependencies; versions read from go.mod.
- Architecture: HIGH. Every integration point read and probed.
- Pitfalls: HIGH for code-derived pitfalls; MEDIUM for TwinCAT semantics.
- Oracle residual estimate: LOW-MEDIUM. Textual simulation only.

**Research date:** 2026-10-05
**Valid until:** 2026-11-04, or until Phase 19 code lands.
