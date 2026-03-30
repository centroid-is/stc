# Phase 12: I/O Address Parser & Table - Context

**Gathered:** 2026-03-30
**Status:** Ready for planning
**Mode:** Auto-generated (infrastructure phase)

<domain>
## Phase Boundary

AT-addressed variables (%IX0.0, %QW4, %MD12) are parsed, validated, and wired to a mock I/O table in the interpreter. I/O values sync at scan cycle boundaries matching real PLC behavior. Address overlap detection warns on conflicts.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices at Claude's discretion — infrastructure phase. Key constraints from research:

- I/O memory model: three flat byte arrays (%I input, %Q output, %M memory)
- AT syntax: `%<area>[<size>]<position>` where area=I/Q/M, size=X/B/W/D/L, position=byte[.bit]
- AT only valid in PROGRAM VAR blocks and VAR_GLOBAL (not in FUNCTION_BLOCK per IEC standard)
- I/O sync: inputs copied FROM IOTable before execution, outputs copied TO IOTable after
- Address overlap: byte-level and bit-level accesses to same address should warn
- The parser already has `AtAddress` field on `VarDecl` in `pkg/ast/var.go`
- The interpreter's `ScanCycleEngine` already has `SetInput`/`GetOutput` — extend with I/O table
- Wildcard `AT %I*` means "linker assigns address" — treat as no specific address in stc

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/ast/var.go` — VarDecl has AtAddress string field
- `pkg/interp/scan.go` — ScanCycleEngine with Tick, SetInput, GetOutput
- `pkg/interp/value.go` — Value type system
- `pkg/lexer/token.go` — AT token already lexed
- `pkg/parser/var.go` — AT parsing already implemented

### Integration Points
- New `pkg/io/` package for IOTable and address parsing
- ScanCycleEngine needs IOTable field and sync logic in Tick()
- Checker needs AT address validation (area/size/position format)

</code_context>

<specifics>
## Specific Ideas

- Research says flat map[string]Value is simplest and sufficient
- Address canonical form: uppercase, e.g., "%IX0.0", "%QW4"

</specifics>

<deferred>
## Deferred Ideas

None

</deferred>
