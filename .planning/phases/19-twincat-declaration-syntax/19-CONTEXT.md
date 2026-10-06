# Phase 19: TwinCAT Declaration Syntax - Context

**Gathered:** 2026-10-05
**Status:** Ready for planning
**Mode:** Autonomous smart discuss (recommended answers accepted; yolo mode, overnight run)

<domain>
## Phase Boundary

TwinCAT declaration-level constructs parse into the AST and survive `stc fmt` / `stc emit` round-trips: `{attribute ...}` pragmas retained on declarations, bare `VAR_GLOBAL` files as GVL declarations with `qualified_only` enforcement, `AT %I*`/`%Q*` on STRUCT members and FB variables without warnings, empty formal call arguments, and `ACTION ... END_ACTION` blocks callable from their POU. Expression-level gaps (bit access, named function arguments, qualified enums, `REF=`, `THIS^`) are Phase 20. Runtime instantiation of GVLs and binding of wildcard AT members to I/O is Phases 22 to 24. Acceptance oracle: the flattened sildarvinnsla sources in `/Users/jonb/Projects/beckhoff-docs/stc-probes/` (`st301.st`, `svncorecomponents.st`, and the one-construct probes `gvl1.st`, `gvl2.st`, `structat.st`, `structpragma.st`, `prog.st`, `action.st`, `link.st`).

</domain>

<decisions>
## Implementation Decisions

### Attribute pragmas in the AST
- Add `ast.Attribute{NodeBase; Name string; Value string; HasValue bool}` and an `Attributes []*Attribute` field on VarDecl, StructMember, EnumValue, TypeDecl, ProgramDecl, FunctionBlockDecl, FunctionDecl, MethodDecl, PropertyDecl, InterfaceDecl and the new GVLDecl. Non-attribute pragmas (`{warning disable C0001}`, `{region}`, `{endregion}`, `{text ...}`) are kept verbatim as `ast.PragmaNode{Text}` in a sibling `Pragmas []*PragmaNode` field on the same nodes so fmt round-trips them; preprocessor directives stay with `pkg/preprocess`.
- The parser accumulates every Pragma token (and intervening comments/blank lines) until the next declaration token and attaches them to that declaration. The existing `skipPragmas()` becomes `collectPragmas() ([]*ast.Attribute, []*ast.PragmaNode)`; `parseDeclaration` no longer discards pragmas.
- Attribute names and values accept single or double quotes; `''` inside a single-quoted value unescapes to `'`. `{attribute 'qualified_only'}` without `:=` sets `HasValue=false`. The emitter always re-emits `{attribute 'name' := 'value'}` with single quotes and re-escapes `'` as `''`.
- JSON: `"attributes": [{"name": "OPC.UA.DA", "value": "1"}]` on each node; `stc parse --format json` on `ECT.TcGVL`-derived ST must show `TcLinkTo`, `OPC.UA.DA`, `OPC.UA.DA.StructuredType`, `qualified_only` on the right nodes.

### GVL files
- New `ast.GVLDecl{NodeBase; Name *Ident; Blocks []*VarBlock; Attributes; Pragmas}` produced when a file's top level contains `VAR_GLOBAL [PERSISTENT] [RETAIN] [CONSTANT] ... END_VAR` blocks. All top-level VAR_GLOBAL blocks of one file aggregate into one GVLDecl; mixing with other declarations in the same file is allowed (the GVL takes the file name).
- The GVL name defaults to the file basename without extension (`ECT.st` -> `ECT`); a `--gvl-name` flag on `parse`, `check`, `fmt`, `emit` overrides it for single-file invocations. (TcGVL XML `Name=` will feed this in Phase 21.)
- Checker: GVL variables are registered in a global scope and resolvable as `GVL.x` always, and as bare `x` unless the GVL carries `{attribute 'qualified_only'}`; unqualified access to a qualified_only GVL variable is a new error diagnostic SEMA033 ("GVL 'X' is qualified_only; use X.var"). `VAR_GLOBAL CONSTANT` members are constants; `PERSISTENT`/`RETAIN` are recorded as flags on VarBlock (no runtime semantics yet).
- Interpreter: GVL declarations are tolerated (no crash) and `GVL.x` resolves when a GVL is present in the same analysis unit via a global env layer; full task-level instantiation and persistence are Phase 23. Keep this minimal and tested.

### Wildcard AT addresses in structs and FBs
- `StructMember` gains `AtAddress *Ident` like VarDecl; `parseStructMember` accepts `name AT %I* : TYPE`.
- Checker SEMA031 (AT in FB) is emitted only for explicit addresses (`%IX0.0`, `%QW4`, ...) inside FUNCTION_BLOCK/FUNCTION; wildcard `%I*`/`%Q*`/`%M*` in FB `VAR`/`VAR_INPUT`/`VAR_OUTPUT` and in STRUCT members produce no diagnostic. An explicit address inside a STRUCT is SEMA031 as well (TwinCAT rejects it).
- `FB_ATV320`-style inputs (`i_uETA AT %I* : UINT` in VAR_INPUT) must check clean. Runtime binding of these members to process-image slots is out of scope (Phase 24, RUNT-07).

### Empty call arguments and ACTIONs
- `parseCallArg` accepts `name :=` and `name =>` followed directly by `,` or `)`; the resulting `ast.CallArg` has `Value == nil` and is kept in the AST so `stc fmt` reproduces the source. Checker and interpreter skip nil-valued arguments (no assignment, no output write).
- New `ast.ActionDecl{NodeBase; Name *Ident; Body []Statement}` with `Actions []*ActionDecl` on ProgramDecl and FunctionBlockDecl. Parser accepts `ACTION name ... END_ACTION` blocks appearing after `END_PROGRAM` / `END_FUNCTION_BLOCK` (CODESYS text export form) and attaches them to the immediately preceding POU; a leading ACTION with no preceding POU is a diagnostic. TcPOU `<Action>` XML attachment is Phase 21.
- Checker: inside the owning POU, `A100_input();` resolves to the action as a parameterless call statement; actions share the POU's variable scope. From outside, `inst.ActionName()` on an FB instance is accepted as a call statement. Interpreter executes the action body in the owner's environment (same env as the POU body, so edge FBs and timers inside actions keep state).
- `stc emit` and `stc fmt` print actions after the POU in the same text form.

### Claude's Discretion
- Exact field names, diagnostic message wording, and where the pragma accumulator lives in the parser.
- Whether `Attributes` lives on `VarBlock` too (useful for `{attribute 'qualified_only'}` placed before a block).
- Coverage strategy: pkg/parser and pkg/interp gates are 95%; pkg/interp sits at 94.0% on main today, so this phase must add interpreter tests to clear the gate.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/lexer` already emits one `Pragma` token for `{...}` (lexer.go `scanPragma`), including the `DirectAddr` token for `%I*`.
- `ast.PragmaNode{Text}` exists in `pkg/ast/var.go:83` and is handled by `json.go:547` but is never constructed.
- `VarDecl.AtAddress *Ident` and `iomap.ParseAddress` (handles `%I*` wildcards, `IsWildcard`) exist; `checker/check.go:71 checkATAddresses` emits SEMA030/031/032.
- `parseCallArgs`/`parseCallArg` in `pkg/parser/stmt.go:387-430` handle `:=` and `=>`; the empty-value case is the only addition.
- `pkg/emit` and `pkg/format` re-emit from the AST; adding attribute/pragma/action printing there gives round-trip for free.

### Established Patterns
- Parser: hand-written recursive descent with `p.at()`, `p.match()`, `p.advance()`, error recovery via `recoverDeclaration`; pragmas skipped at `decl.go:14`, `var.go:22/25/84`, `types.go:179`, `stmt.go:19`, `decl.go:237`.
- AST nodes embed `NodeBase`, implement `Children()`, and are marshalled by hand in `pkg/ast/json.go` (new fields need JSON cases).
- Checker diagnostics use `SEMAnnn` codes with vendor profiles; tests are table-driven in `pkg/checker/*_test.go`.
- Tests: Go unit tests per package plus `stc test` ST suites under `tests/` and the st-tests CI workflow.

### Integration Points
- `parseDeclaration` (decl.go:12) dispatch: add `KwVarGlobal` -> GVL and `KwAction` -> attach to previous POU.
- `pkg/analyzer` facade and `pkg/checker/resolve.go` symbol collection: register GVL scope and action symbols.
- `pkg/interp/scan.go initializeEnv` already binds AT vars from PROGRAM VarBlocks; GVL env layer slots in above the program env.
- CLI flags in `cmd/stc/*.go` (cobra) for `--gvl-name`.
- MCP `stc_parse` returns the JSON AST, so attribute output appears there automatically.

</code_context>

<specifics>
## Specific Ideas

- Gate progress with the flattened probes: `stc parse` on `svncorecomponents.st` currently reports 1430 diagnostics and on `st301.st` 2711; after this phase the remaining errors must all be Phase 20 constructs (bit access, named function args, qualified enum CASE labels, enum base types, `REF=`).
- Real-world pragma shapes to cover in tests (from sildarvinnsla): `{attribute 'TcLinkTo' := '.I1 := TIID^Device 1 (EtherCAT)^ST301.A1.00 (EK1200)^ST301.A1.03 (EL1008)^Channel 1^Input; .I2 := ...'}` (long multi-link value containing `^`, `;`, `:=`, spaces, parentheses), `{attribute "qualified_only"}` (double quotes), `{attribute 'OPC.UA.DA.Description' := 'Time q_xDetected has been TRUE (caps at 1 day)'}`, attributes on enum values inside `( ... ) UINT;` (value-level attributes must at least parse), attribute before `FUNCTION_BLOCK`, attribute before `TYPE`, blank line between attribute and declaration, `VAR PERSISTENT RETAIN` and `VAR_GLOBAL PERSISTENT RETAIN` blocks, `VAR_IN_OUT` members with attributes.
- MAIN shape to test for actions: `PROGRAM MAIN VAR ... END_VAR A050_ModbusCall(); A100_input(); ... END_PROGRAM ACTION A050_ModbusCall ... END_ACTION ACTION A100_input ... END_ACTION`.
- Keep `stc fmt` idempotent with pragmas present (format twice, identical output).

</specifics>

<deferred>
## Deferred Ideas

- Bit access `w.3`, named arguments in function-call expressions, qualified enum values and CASE labels, enum base types, `REF=`, `THIS^`/`SUPER^` -> Phase 20.
- TcPOU/TcGVL/TcDUT XML import (including `<Action>` elements and GVL `Name=`) -> Phase 21.
- Runtime GVL instantiation per task, PERSISTENT state file, typed AT binding -> Phases 22-24.
- Using `TcLinkTo` attribute values for anything -> Phase 24.

</deferred>
