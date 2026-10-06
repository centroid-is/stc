---
phase: 22-symbol-tree-value-semantics
plan: 03
subsystem: analysis
tags: [symtree, symbol-tree, analyzer, cli, json, opc-ua-prep]

requires:
  - phase: 20-dialect-and-attributes
    provides: ast.Attribute on decls, EnumType ordinals and flags, GVL struct symbols
provides:
  - pkg/symtree static symbol tree (Build, Lookup, Walk, JSON, Text, ParsePath)
  - AnalysisResult.Files and AnalysisResult.LibraryFiles
  - stc check --symbols (text and --format json)
  - pkg/symtree 95% coverage gate in .testcoverage.yml
affects: [22-05 runtime Get/Set path grammar, 28 OPC UA address space builder, LSP, MCP]

tech-stack:
  added: []
  patterns:
    - "Tree order from the AST, types from the symbol table; symtree never imports interp"
    - "Arrays are lazy: an element factory synthesises [i] on demand after bounds checks"
    - "Ordered JSON via fixed structs; enum strings marshalled in ascending ordinal order"

key-files:
  created:
    - pkg/symtree/symtree.go
    - pkg/symtree/path.go
    - pkg/symtree/lookup.go
    - pkg/symtree/json.go
    - pkg/symtree/symtree_test.go
    - pkg/symtree/lookup_test.go
    - pkg/symtree/edge_test.go
  modified:
    - pkg/analyzer/analyzer.go
    - pkg/analyzer/analyzer_test.go
    - cmd/stc/check.go
    - cmd/stc/check_test.go
    - .testcoverage.yml

key-decisions:
  - "RUNT-01 layout is delivered as declaration order, VAR sections and array bounds; byte offsets are an ADS concern for v2"
  - "Multi-dimensional or non-constant-bound arrays are Array nodes with Bounded=false and no elements"
  - "A derived FB variable with a base variable's name replaces it in place; EXTENDS chain is walked base first with a visited guard"
  - "VAR_TEMP and VAR_EXTERNAL are not part of the tree (no instance state)"
  - "A user GVL replaces a library GVL of the same name; roots are GVLs (library then user) then PROGRAMs"
  - "Bit segments (x.3) resolve to a synthetic BOOL node on integer and bit-string scalars only"
  - "RUNT-01 left unchecked: plan 22-05 owns its end-to-end proof"

patterns-established:
  - "Path grammar documented in pkg/symtree package doc; interp mirrors it without importing symtree"

requirements-completed: []

duration: 8min
completed: 2026-10-06
---

# Phase 22 Plan 03: Symbol Tree Summary

**Static `pkg/symtree` tree of every GVL and PROGRAM with FB instances, struct members and lazy array elements, carrying IEC type, declared type name, enum strings and merged attributes, addressable by case-insensitive dotted path and printed by `stc check --symbols`.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-10-06T06:56:38Z
- **Completed:** 2026-10-06T07:05:00Z
- **Tasks:** 3
- **Files modified:** 12

## Accomplishments
- `GVL.fb[2].HMI.p_stat_State` resolves to an Enum node with `*types.EnumType`, TypeName `E_State`, EnumStrings `{0:Idle,1:Run}` and attributes type-level then instance-level.
- FB children follow the EXTENDS chain base first, with VAR sections and CONSTANT/RETAIN/PERSISTENT flags; standard FBs expose their params (TON: IN, PT, Q, ET).
- Cycle guard by type-name stack; recursive structs and FBs stop at a childless node.
- `stc check --symbols` prints indented text to stdout, or `{"diagnostics": [...], "symbols": {...}}` with `--format json`. Default output is unchanged.
- pkg/symtree coverage 98.5%; fuzzed ParsePath for 10s without findings.

## Public API (pkg/symtree)

```go
type Kind int // KindGVL, KindProgram, KindFBInstance, KindStruct, KindArray,
              // KindScalar, KindEnum, KindReference, KindPointer
func (k Kind) String() string // "gvl", "program", "fb_instance", ...

type Node struct {
    Name, Path  string           // declared case; "[3]" for elements, "3" for bits
    Kind        Kind
    Type        types.Type       // from the symbol table; nil if unresolved
    TypeName    string           // "FB_Drive", "ARRAY[1..3] OF FB_Drive", "STRING(80)"
    Attributes  []*ast.Attribute // type-level then instance-level
    EnumStrings map[int64]string
    Section     ast.VarSection
    Constant, Retain, Persistent bool
    Pos         ast.Pos
    Low, High   int
    Bounded     bool             // one-dimensional constant bounds known
}
func (n *Node) Children() []*Node // arrays synthesise Low..High on each call

type Tree struct{ Roots []*Node }
func Build(res analyzer.AnalysisResult) (*Tree, error)
func (t *Tree) Lookup(path string) (*Node, error)
type WalkOpts struct{ ExpandArrays bool }
func (t *Tree) Walk(fn func(*Node) bool, opts WalkOpts)
func (t *Tree) JSON() ([]byte, error) // {"roots":[...]}; arrays: low, high, element "[*]"
func (t *Tree) Text() string          // "path : TYPE {attr := 'v', flag}" lines

const MaxPathLen = 1024
type Segment struct{ Name string; Index int; IsIndex, IsBit bool; Bit int }
func ParsePath(p string) ([]Segment, error)
```

Analyzer: `AnalysisResult.Files []*ast.SourceFile` and `AnalysisResult.LibraryFiles []*ast.SourceFile`.

## Task Commits

1. **Task 1: AnalysisResult files and symtree Build** - `7057de4` (test), `bf7938b` (feat), `561ef9a` (style)
2. **Task 2: ParsePath, Lookup, Children, Walk, JSON** - `4b85373` (test), `e23b1e4` (feat)
3. **Task 3: stc check --symbols and coverage gate** - `5d18baf` (test), `104cc98` (feat)

## Files Created/Modified
- `pkg/symtree/symtree.go` - Kind, Node, Tree, Build, type naming, cycle guard
- `pkg/symtree/path.go` - Segment, ParsePath, MaxPathLen
- `pkg/symtree/lookup.go` - Children, Lookup, Walk, bit width
- `pkg/symtree/json.go` - deterministic JSON and Text renderers
- `pkg/analyzer/analyzer.go` - Files and LibraryFiles on AnalysisResult
- `cmd/stc/check.go` - `--symbols` flag
- `.testcoverage.yml` - `^pkg/symtree$` threshold 95

## Decisions Made
See key-decisions. The JSON element prototype uses the label `[*]` so nested template paths read `GVL.fb[*].HMI.p_stat_State`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing functionality] Text renderer and Bounded flag added to the API**
- **Found during:** Tasks 1 and 3
- **Issue:** The plan's Node had no way to tell unknown bounds from `0..0`, and the CLI text format needed a renderer.
- **Fix:** Added exported `Node.Bounded` and `Tree.Text()` (tested in pkg/symtree, so the CLI branch stays small).
- **Committed in:** `bf7938b`, `e23b1e4`

**2. [Rule 1 - Bug] Unresolved array variables rendered an empty type name**
- **Found during:** Task 2 coverage work
- **Fix:** `specArrayName` renders `ARRAY[0..?] OF INT` from the spec when the symbol table has no type.
- **Committed in:** `e23b1e4`

**Total deviations:** 2 auto-fixed. No scope creep.

## Issues Encountered
- `cmd/stc/main.go` is not gofmt-clean on this branch; it predates this plan and was left alone.

## Next Phase Readiness
- 22-05: mirror the path grammar from the pkg/symtree package doc (ident, `[int]` with optional minus, `.digits` bit, 1024-byte cap, `a[1,2]` rejected). The ST301-shaped end-to-end check and the RUNT-01 checkbox belong there.
- Phase 28: read `Node.Attributes` (type-level first) and `EnumStrings`; use `Walk` with `ExpandArrays:true` or `Children()` for array elements.

---
*Phase: 22-symbol-tree-value-semantics*
*Completed: 2026-10-06*

## Self-Check: PASSED
