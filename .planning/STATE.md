---
gsd_state_version: 1.0
milestone: v1.2
milestone_name: TwinCAT Import, EtherCAT Simulation & OPC UA
status: executing
stopped_at: Completed 20-09-PLAN.md
last_updated: "2026-10-06T07:18:10.265Z"
last_activity: 2026-10-06
progress:
  total_phases: 29
  completed_phases: 16
  total_plans: 67
  completed_plans: 60
  percent: 55
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-30)

**Core value:** Write ST once, validate it instantly on your machine, and deploy to any supported PLC vendor -- no hardware required for development and testing.
**Current focus:** Phase 28 — OPC UA Address Space

## Current Position

Phase: 28 (OPC UA Address Space) — EXECUTING
Plan: 4 of 4
Status: Ready to execute
Last activity: 2026-10-06

Progress: [█████████░] 90%

## Performance Metrics

**Velocity:**

- Total plans completed: 51 (v1.0) + 10 (v1.1) = 42
- Average duration: ~4.5 min
- Total execution time: ~2.4 hours (v1.0) + ~1 hour (v1.1)

**v1.1 Phase Execution (phases 12-18):**

- Phase 12: 2 plans (I/O address parser)
- Phase 13: 2 plans (vendor stub loading)
- Phase 14: 2 plans (mock framework)
- Phase 15: 1 plan (Beckhoff stubs)
- Phase 16: 1 plan (Schneider/AB stubs)
- Phase 17: 1 plan (behavioral mocks)
- Phase 18: 1 plan (auto-defines + TcPOU extractor)

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [v1.0]: All v1.0 decisions remain valid (see STATE.md archive)
- [v1.1 Research]: IOTable uses three flat byte arrays (%I, %Q, %M) per IEC 61131-3 standard
- [v1.1 Research]: Stub files are hand-written .st declarations (TypeScript .d.ts analogy), NOT parsed from .library
- [v1.1 Research]: TcPOU XML extraction is a convenience tool, not the primary stub path
- [v1.1 Research]: AB stubs written as IEC 61131-3 FUNCTION_BLOCKs for checker, emitter handles AOI translation later
- [Phase 12]: IOTable is pure byte-level storage with no interp.Value dependency
- [Phase 12]: AT-bound input vars synced before staged inputs in Tick for test override flexibility
- [Phase 13]: Variadic ResolveOpts pattern preserves backward compatibility for CollectDeclarations callers
- [Phase 13]: First-library-wins deduplication for duplicate vendor FB names; user code silently overrides library symbols
- [Phase 13]: Variadic AnalyzeOpts pattern preserves backward compatibility for all existing Analyze callers
- [Phase 13]: Cross-vendor detection uses string-contains heuristic on library path keys (VEND010)
- [Phase 14]: Mock symbols registered with isLibrary=false so they override library stubs as real implementations
- [Phase 14-mock-framework]: SET_IO/GET_IO use string area identifiers for ST developer ergonomics
- [Phase 14-mock-framework]: IOTable created per test case for isolation; auto-stub warnings aggregated at run level
- [Phase 15]: MC_Power.Override parameter renamed to Override_V to avoid conflict with OVERRIDE keyword in stc lexer
- [Phase 18]: RunOpts.Defines field threads preprocessor defines through test runner to pipeline.Parse
- [v1.2 Roadmap]: Dialect parity split into declaration syntax (Phase 19) and expression semantics + parse gate (Phase 20); nothing downstream starts until flattened ST301 parses clean
- [v1.2 Roadmap]: Symbol tree with dotted-path Get/Set (Phase 22) is the shared backbone for EtherCAT binding, OPC UA, ST test built-ins and MCP tools; design it so an ADS server (v2) is a thin adapter
- [v1.2 Roadmap]: EtherCAT simulated at process-image/PDO level only; ATV320 + Tc2_EtherCAT mocks isolated in Phase 26 as the highest-fidelity risk
- [v1.2 Roadmap]: Phase 28 opens with an awcullen/opcua spike for struct DataTypeDefinition (fallback NodeSet2 import) before building the address space
- [Phase 19]: 19-01: scripts/coverage-gate.sh is the per-plan coverage check (thresholds hard-coded from .testcoverage.yml) — Reproduces CI merged profile locally; go-test-coverage opt-in via STC_COVER_TOOL=1
- [Phase 19]: 19-02: Attribute.String() quotes name and value with '' doubling; JSON value key present iff HasValue
- [Phase 19]: 19-02: Attributes/Pragmas also on VarBlock; new NodeKinds appended after KindVarDecl (=40, pinned by test)
- [Phase 19]: 19-03: trailing pragmas before END_VAR attach to the VarBlock, before END_STRUCT or ) to the last member/value; ownerless pragmas (EOF, empty struct/enum) are dropped
- [Phase 19]: 19-03: printers emit owner attributes (with their own comments) before the owner's leading trivia; attributes are emitted for every vendor target
- [Phase 19]: 19-05: GVL access rules (qualified_only, constants) live on the KindGVL symbol as GVLInfo so PurgeFile cannot leave stale entries
- [Phase 19]: 19-05: GVLs resolve in a deferred pendingGVLs pass after all TYPEs are registered
- [Phase 19]: 19-06: GVL member writes to undeclared names are RuntimeErrors; non qualified_only GVL envs chain as parents of program/test/FUNCTION envs via Interpreter.GlobalParent
- [Phase 19]: ACTION bodies stop at END_ACTION, the next ACTION, the POU end or any top-level declaration keyword
- [Phase 19]: FB methods are FunctionType symbols in the FB scope so actions and the FB body can call them unqualified
- [Phase 19]: DIAL-08 stays pending until 19-08 adds runtime action execution
- [Phase 19]: 19-10: --gvl-name is applied after parsing (after ia.Parse in check), never in the incremental parse path; more than one input file is a usage error, a JSON {error} object under --format json
- [Phase 19]: 19-09: gofmt is not CI-enforced; pre-existing non-gofmt files left untouched
- [Phase 19]: 19-09: LSP resolves qualified_only GVL variables via the GVL struct type, same-file GVL first
- [Phase 20]: 20-01: ast.EnumOrdinals is the single enum numbering routine; Known=false propagates to implicit successors of a non-literal value
- [Phase 20]: 20-01: JSON kinds for Phase 20 nodes and CallArg are forced in nodeToMap (CallStmt/CallExpr args now report CallArg, not SourceFile)
- [Phase 20]: 20-02: Parser.stmtHead keeps fb(name := ...) a CallStmt only at statement head; expression calls split args into Args (leading positional) and NamedArgs (rest, source order)
- [Phase 20]: 20-02: w.3.1 lexes as Dot RealLiteral; parser splits it into nested BitAccessExprs and reports bit access on a bit
- [Phase 20]: Enum base types accept only integer and bit-string keywords
- [Phase 20]: Struct initialiser needs '(' Ident ':='; any other '(' stays ParenExpr
- [Phase 20]: Repetition count is an integer literal and never expanded; initialiser nesting capped at 64
- [Phase 20]: Typed literals stop at ':'; only time, date and TOD literals keep colons
- [Phase 20]: resolveTypeSpec consults a pointer-stable forward map (owning declaration: first user/mock, else first library) before the global scope
- [Phase 20]: EXTENDS re-parents the derived POU scope onto the base scope instead of copying symbols
- [Phase 20]: Standard FBs are library symbols with Tc2_Standard names first and IEC aliases appended; stubs and user code override them silently
- [Phase 20]: SEMA037 is reported once per NamedType node; unknown names inside library declarations are not reported
- [Phase 20]: 20-05: runtime constant-index bit access accepts any integer symbol as index; checker SEMA035 must enforce CONSTANT
- [Phase 20]: 20-05: bit writes keep the target IECType, mask unsigned kinds and sign-extend signed kinds
- [Phase 20]: 20-05: an enum's zero value is its first declared value typed by its base type
- [Phase 20]: 20-06: constant bit index needs a VAR_GLOBAL CONSTANT or a VAR CONSTANT with an integer literal initialiser, on an integer or bit-string object
- [Phase 20]: 20-06: all-positional calls must supply every parameter; once any argument is named, omitted inputs take defaults; positional args bind by slot index (ruling A1)
- [Phase 20]: 20-06: REFERENCE TO T inputs bind a value of exactly T without widening
- [Phase 20]: 20-06: POU-scope methods and actions bind before same-named built-ins; global functions still lose to built-ins
- [Phase 20]: Positional arguments bind by index in the full argument list at runtime; omitted and empty inputs take their declared default
- [Phase 20]: SUPER resolves relative to the declaring FB of the running code (Env.selfDecl); unqualified and THIS^ method calls are virtual
- [Phase 20]: REF= and REF() build path references (RefPath) with indices evaluated at bind time; a path that stops resolving is a dangling-reference RuntimeError
- [Phase 20]: Built-in functions reject named arguments at runtime instead of dropping them
- [Phase 20]: 20-09: strict enums reject arithmetic, implicit integer conversion and cross-type comparison (SEMA036); <X>_TO_<Y> and TO_<Y> conversions take them explicitly; non-strict enums act as their base integer (ruling A4)
- [Phase 20]: 20-09: THIS^.m and SUPER^.m resolve through the FB scope chain; SEMA038 texts mirror the interpreter's runtime errors
- [Phase 20]: 20-09: initialisers type-check literal values only; non-literal values are walked for undeclared names (Phase 22 owns literal typing)
- [Phase 20]: 20-09: types.ArrayDimension.Known marks literal bounds; too many initialisers is reported only with known bounds and literal repetition counts
- [Phase 20]: Phase 20 oracle asserts 0 parse diagnostics (parser.Parse) and 0 P001 (analyzer.Analyze) for st301 and svncore; semantic errors are logged as templated buckets only
- [Phase 20]: TwinCAT dialect check tolerates only 'cannot assign DINT to INT' (Phase 22 RUNT-05), held in one named allowlist
- [Phase 28]: 28-01: awcullen v1.4.0 always advertises secured policies once a cert loads; EnableBasic256Sha256 records intent and AllowNone=false gives a secure-only server
- [Phase 28]: 28-01: awcullen ListenAndServe binds all interfaces and Close sleeps 3 s; Endpoint port 0 is rejected

### Pending Todos

None yet.

### Blockers/Concerns

- awcullen/opcua support for StructureDefinition DataTypeDefinitions is unverified (Phase 28 spike)
- Phase 29 needs a stored browse fixture from the real ST301 TF6100 server, captured once from the plant network
- The Flutter HMI may require SignAndEncrypt; Phase 28 must support Basic256Sha256 with self-signed certs

## Session Continuity

Last session: 2026-10-06T07:18:10.260Z
Stopped at: Completed 20-09-PLAN.md
Resume file: None

## Operator Next Steps

- Plan the first v1.2 phase with /gsd:plan-phase 19
