# Phase 3: Semantic Analysis - Research

**Researched:** 2026-03-27
**Domain:** IEC 61131-3 type system, symbol resolution, vendor-aware diagnostics
**Confidence:** HIGH

## Summary

Phase 3 builds the semantic analysis layer for the STC compiler: a hierarchical symbol table, a two-pass type checker following MATIEC's proven fill-candidates/narrow-candidates algorithm, cross-file symbol resolution, and vendor-aware diagnostics. This phase transforms raw ASTs from the parser into typed, resolved programs with actionable error messages.

The existing codebase provides a solid foundation: the AST node hierarchy in `pkg/ast/` already covers all IEC 61131-3 + CODESYS OOP constructs, the `pkg/diag/` package has a Collector with severity levels and diagnostic codes, and the `pkg/project/` package reads `stc.toml` with `VendorTarget` configuration. The parser produces `(File, []Diagnostic)` tuples that the semantic analyzer will consume directly.

The hardest sub-problem is the ANY type hierarchy resolution for overloaded standard functions. MATIEC's three-stage approach (fill_candidate_datatypes, narrow_candidate_datatypes, print_datatype_errors) is the proven solution. The user has locked the decision to use IEC-defined implicit widening only (no CODESYS-permissive mode), which simplifies the type lattice but requires strict validation.

**Primary recommendation:** Build three new packages (`pkg/types/`, `pkg/symbols/`, `pkg/checker/`) plus a `pkg/analyzer/` facade, following the Go compiler's scope-chain pattern for symbol tables and MATIEC's two-pass algorithm for type resolution. Wire into CLI via `stc check` command.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- IEC-defined implicit widening only (INT->DINT->LINT, REAL->LREAL) -- strict, catches real bugs
- Two-pass candidate/narrow approach for ANY type hierarchy overloaded standard functions (proven by MATIEC)
- Parse OOP fully but defer inheritance/interface type checking to v1.x -- check method signatures and FB instance calls only
- Parse and represent POINTER TO/REFERENCE TO in symbol table, skip dereferencing validation in v1
- Use stc.toml `source_roots` to find .st files, then build dependency graph from POU references
- Hierarchical symbol table: global scope -> POU scope -> method scope -> block scope, each with parent reference for name lookup
- Two-pass analysis: first pass collects all POU declarations and type signatures, second pass type-checks bodies (handles forward references)
- Go structs with feature flags (SupportsOOP, SupportsPointerTo, MaxStringLen, etc.)
- Vendor diagnostics are warnings, not errors -- code may be valid for current vendor, just not portable
- Three built-in profiles: `beckhoff`, `schneider`, `portable` (intersection). User sets via stc.toml or --vendor flag

### Claude's Discretion
- Internal error representation details
- Specific diagnostic message wording
- Symbol table caching strategy
- Test fixture organization

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SEMA-01 | User gets type mismatch errors with file:line:col and actionable messages | Diagnostic type already has Pos/EndPos/Code/Message fields; type checker produces diagnostics via diag.Collector |
| SEMA-02 | Type checker resolves all IEC primitive types (BOOL through TOD, 20+ types) | Type lattice data structure maps all elementary types with widening rules; see IEC type hierarchy below |
| SEMA-03 | Type checker handles arrays, structs, enums, FB instances, method calls | AST already has ArrayType, StructType, EnumType, FunctionBlockDecl, MethodDecl, CallExpr, MemberAccessExpr nodes |
| SEMA-04 | Type checker detects undeclared variables, unused variables, unreachable code | Symbol table tracks declarations and usage counts; control flow analysis for unreachable code after RETURN/EXIT |
| SEMA-05 | Type checker handles cross-file symbol resolution (multi-file projects) | stc.toml source_roots for file discovery; two-pass: collect declarations across files first, then type-check bodies |
| SEMA-06 | CLI command `stc check <files...> --format json` outputs diagnostics | Existing stub in cmd/stc/stubs.go; wire analyzer.Check() to CLI; JSON marshaling already works on Diagnostic type |
| SEMA-07 | Vendor-aware diagnostics warn when using constructs unsupported by target vendor | VendorProfile Go structs with feature flags; vendor checks emit Warning-severity diagnostics |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib | 1.26 | Language runtime | Already in use, all packages are pure Go |
| github.com/stretchr/testify | (existing) | Test assertions | Already used in parser tests, require/assert pattern |
| github.com/BurntSushi/toml | (existing) | Config parsing | Already used for stc.toml |
| github.com/spf13/cobra | (existing) | CLI framework | Already used for stc command |

### Supporting
No new external dependencies needed. The semantic analysis is pure Go code operating on existing AST types.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom type lattice | Generic constraint solver | Overkill for IEC's finite type set; custom lattice is simpler and faster |
| Map-based symbol table | Tree-sitter-style incremental | Not needed until Phase 10 (incremental); maps are correct and fast for batch |

**Installation:**
No new packages to install.

## Architecture Patterns

### Recommended Project Structure
```
pkg/
  types/             # IEC 61131-3 type system
    types.go         # Type interface, concrete types (Primitive, Array, Struct, Enum, FB, Function, Pointer, Reference)
    lattice.go       # Type widening/narrowing rules, ANY hierarchy, compatibility checks
    builtin.go       # Built-in type constants (TypeBOOL, TypeINT, etc.) and standard function signatures
  symbols/           # Symbol table and scoping
    scope.go         # Scope type with parent chain, Insert/Lookup methods
    symbol.go        # Symbol type (name, type, kind, span, used flag)
    table.go         # SymbolTable facade: global scope, POU registry, file tracking
  checker/           # Type checking passes
    resolve.go       # Pass 1: Walk AST, collect all declarations into symbol table
    check.go         # Pass 2: Walk AST bodies, type-check expressions, resolve calls
    candidates.go    # ANY type candidate enumeration and narrowing (MATIEC algorithm)
    vendor.go        # Vendor profile definitions and vendor-aware diagnostic checks
    diag_codes.go    # Semantic diagnostic code constants (SEMA001, SEMA002, etc.)
  analyzer/          # Public facade
    analyzer.go      # Analyze(files, config) -> AnalysisResult{SymbolTable, TypeInfo, Diagnostics}
cmd/stc/
  check.go           # stc check command implementation (replaces stub)
```

### Pattern 1: Hierarchical Scope Chain (Go compiler pattern)
**What:** Each scope holds a map of names to symbols and a pointer to its parent scope. Name lookup walks up the chain until found or reaches the universe/global scope.
**When to use:** All name resolution.
**Example:**
```go
// pkg/symbols/scope.go
type Scope struct {
    parent   *Scope
    symbols  map[string]*Symbol
    children []*Scope
    kind     ScopeKind  // Global, POU, Method, Block
    name     string     // POU/method name for this scope
}

type ScopeKind int
const (
    ScopeGlobal ScopeKind = iota
    ScopePOU              // PROGRAM, FUNCTION_BLOCK, FUNCTION
    ScopeMethod           // METHOD inside FB
    ScopeBlock            // FOR/IF/etc block (for future block-scoped vars)
)

func (s *Scope) Lookup(name string) *Symbol {
    // Case-insensitive lookup (IEC 61131-3 identifiers are case-insensitive)
    upper := strings.ToUpper(name)
    if sym, ok := s.symbols[upper]; ok {
        return sym
    }
    if s.parent != nil {
        return s.parent.Lookup(name)
    }
    return nil
}

func (s *Scope) Insert(sym *Symbol) error {
    upper := strings.ToUpper(sym.Name)
    if existing, ok := s.symbols[upper]; ok {
        return fmt.Errorf("redeclaration of %q (first declared at %s)", sym.Name, existing.Pos)
    }
    s.symbols[upper] = sym
    return nil
}
```

### Pattern 2: Two-Pass Type Resolution (MATIEC algorithm)
**What:** Pass 1 walks all declarations and registers POU signatures, type aliases, and global variables. Pass 2 walks bodies, resolves identifiers to symbols, and type-checks expressions using candidate/narrow for overloaded functions.
**When to use:** Always -- handles forward references (FB used before declared).
**Example:**
```go
// pkg/checker/resolve.go -- Pass 1
func (r *Resolver) CollectDeclarations(files []*ast.SourceFile) {
    for _, file := range files {
        for _, decl := range file.Declarations {
            switch d := decl.(type) {
            case *ast.ProgramDecl:
                r.registerPOU(d.Name.Name, d, symbols.KindProgram)
            case *ast.FunctionBlockDecl:
                r.registerPOU(d.Name.Name, d, symbols.KindFunctionBlock)
            case *ast.FunctionDecl:
                r.registerFunction(d)
            case *ast.TypeDecl:
                r.registerType(d)
            case *ast.InterfaceDecl:
                r.registerInterface(d)
            }
        }
    }
}

// pkg/checker/check.go -- Pass 2
func (c *Checker) CheckBodies(files []*ast.SourceFile) {
    for _, file := range files {
        for _, decl := range file.Declarations {
            switch d := decl.(type) {
            case *ast.ProgramDecl:
                c.checkPOUBody(d.Name.Name, d.VarBlocks, d.Body)
            case *ast.FunctionBlockDecl:
                c.checkFBBody(d)
            case *ast.FunctionDecl:
                c.checkFunctionBody(d)
            }
        }
    }
}
```

### Pattern 3: Type Lattice with Explicit Widening Rules
**What:** The IEC 61131-3 ANY type hierarchy is encoded as a data structure, not code. Widening rules are a lookup table. This makes the rules auditable, testable, and vendor-configurable.
**When to use:** All type compatibility checks, implicit conversions, operator resolution.
**Example:**
```go
// pkg/types/lattice.go
// IEC-defined implicit widening: smaller -> larger within same category
var wideningRules = map[TypeKind][]TypeKind{
    KindSINT:  {KindINT, KindDINT, KindLINT, KindREAL, KindLREAL},
    KindINT:   {KindDINT, KindLINT, KindREAL, KindLREAL},
    KindDINT:  {KindLINT, KindLREAL},
    KindLINT:  {KindLREAL},
    KindUSINT: {KindUINT, KindUDINT, KindULINT, KindREAL, KindLREAL},
    KindUINT:  {KindUDINT, KindULINT, KindREAL, KindLREAL},
    KindUDINT: {KindULINT, KindLREAL},
    KindULINT: {KindLREAL},
    KindREAL:  {KindLREAL},
    KindBYTE:  {KindWORD, KindDWORD, KindLWORD},
    KindWORD:  {KindDWORD, KindLWORD},
    KindDWORD: {KindLWORD},
}

func CanWiden(from, to TypeKind) bool {
    targets, ok := wideningRules[from]
    if !ok {
        return false
    }
    for _, t := range targets {
        if t == to {
            return true
        }
    }
    return false
}

// CommonType finds the smallest type both a and b can widen to.
// Returns (type, true) or (Invalid, false) if incompatible.
func CommonType(a, b TypeKind) (TypeKind, bool) {
    if a == b { return a, true }
    if CanWiden(a, b) { return b, true }
    if CanWiden(b, a) { return a, true }
    // Search for common supertype in widening graph
    // ...
    return KindInvalid, false
}
```

### Pattern 4: Vendor Profile as Go Struct
**What:** Each vendor profile is a Go struct with boolean feature flags and numeric limits. Vendor checks are a separate pass that emits Warning-level diagnostics.
**Example:**
```go
// pkg/checker/vendor.go
type VendorProfile struct {
    Name           string
    SupportsOOP    bool   // METHOD, INTERFACE, PROPERTY
    SupportsPointerTo bool
    SupportsReferenceTo bool
    Supports64Bit  bool   // LINT, LREAL, LWORD, ULINT
    SupportsWString bool
    MaxStringLen   int    // 0 = unlimited
    SupportsPragmas []string // supported pragma names
}

var Beckhoff = VendorProfile{
    Name:              "beckhoff",
    SupportsOOP:       true,
    SupportsPointerTo: true,
    SupportsReferenceTo: true,
    Supports64Bit:     true,
    SupportsWString:   true,
    MaxStringLen:      0,
}

var Schneider = VendorProfile{
    Name:              "schneider",
    SupportsOOP:       false, // partial; controller-dependent
    SupportsPointerTo: false,
    SupportsReferenceTo: false,
    Supports64Bit:     true,
    SupportsWString:   true,
    MaxStringLen:      254,
}

var Portable = VendorProfile{
    Name:              "portable",
    SupportsOOP:       false, // intersection of all vendors
    SupportsPointerTo: false,
    SupportsReferenceTo: false,
    Supports64Bit:     true,
    SupportsWString:   false,
    MaxStringLen:      254,
}
```

### Anti-Patterns to Avoid
- **Single-pass type checking:** Cannot handle forward references. The user locked two-pass. Always collect declarations first.
- **Case-sensitive symbol lookup:** IEC 61131-3 identifiers are case-insensitive. Store and look up using uppercased names; preserve original casing for display.
- **Monolithic checker:** Mixing name resolution, type checking, and vendor checks in one pass. Separate concerns into distinct passes for testability.
- **Hardcoded widening rules in if/else chains:** Use data-driven lookup tables. Makes rules auditable and testable.
- **Failing fast on first error:** Continue checking to report multiple errors. Use the existing `(result, []Diagnostic)` pattern.

## IEC 61131-3 Type System Reference

### Complete ANY Type Hierarchy
```
ANY
  ANY_DERIVED          (user-defined types: structs, enums, FBs, aliases)
  ANY_ELEMENTARY
    ANY_MAGNITUDE
      ANY_NUM
        ANY_REAL       REAL, LREAL
        ANY_INT
          ANY_SIGNED   SINT, INT, DINT, LINT
          ANY_UNSIGNED USINT, UINT, UDINT, ULINT
      TIME
    ANY_BIT            BOOL, BYTE, WORD, DWORD, LWORD
    ANY_CHARS
      ANY_CHAR         CHAR, WCHAR
      ANY_STRING       STRING, WSTRING
    ANY_DATE           DATE, DT (DATE_AND_TIME), TOD (TIME_OF_DAY)
```

### Elementary Type Sizes
| Type | Bits | Go Representation |
|------|------|-------------------|
| BOOL | 1 (stored as 8) | bool |
| BYTE | 8 | uint8 |
| WORD | 16 | uint16 |
| DWORD | 32 | uint32 |
| LWORD | 64 | uint64 |
| SINT | 8 | int8 |
| INT | 16 | int16 |
| DINT | 32 | int32 |
| LINT | 64 | int64 |
| USINT | 8 | uint8 |
| UINT | 16 | uint16 |
| UDINT | 32 | uint32 |
| ULINT | 64 | uint64 |
| REAL | 32 | float32 |
| LREAL | 64 | float64 |
| STRING | variable | string (with max length) |
| WSTRING | variable | string (UTF-16, with max length) |
| TIME | 64 | time.Duration |
| DATE | 64 | time.Time |
| DT | 64 | time.Time |
| TOD | 64 | time.Duration |

### IEC Implicit Widening Rules (locked: IEC-only, no CODESYS permissive)
Widening is allowed only within the same numeric category, from smaller to larger:

**Signed integers:** SINT -> INT -> DINT -> LINT
**Unsigned integers:** USINT -> UINT -> UDINT -> ULINT
**Reals:** REAL -> LREAL
**Cross-category (INT to REAL):** SINT/INT -> REAL, DINT -> LREAL (precision-preserving only)
**Bit types:** BYTE -> WORD -> DWORD -> LWORD (within ANY_BIT only)

**NOT allowed implicitly:**
- Signed to unsigned or vice versa (SINT -> USINT = error, requires explicit conversion)
- ANY_BIT to ANY_INT or vice versa (BYTE -> INT = error)
- ANY_DATE types (no implicit conversions)
- STRING types (no implicit conversions)
- BOOL to anything except itself

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Type hierarchy membership | Switch/case chains for "is X an ANY_INT?" | Bitmask or set-based category membership | 20+ types across 10+ categories; switch chains are error-prone |
| Standard function signatures | Manually coded function stubs | Data-driven signature table loaded at init | 80+ standard functions; table is auditable and extensible |
| Diagnostic message formatting | fmt.Sprintf everywhere | Structured diagnostic builder with code + template | Consistency across 50+ diagnostic messages; enables i18n later |
| Case-insensitive string comparison | strings.ToUpper at every call site | Normalize once at symbol insertion; store canonical form | Forgetting case folding at one site = bug that passes 99% of tests |

**Key insight:** The type system has a fixed, finite set of types and rules defined by the IEC standard. Encode the rules as data (tables, maps), not code (if/else chains). This makes the implementation auditable against the standard.

## Common Pitfalls

### Pitfall 1: ANY Type Candidate Explosion
**What goes wrong:** An integer literal `42` could be SINT, INT, DINT, LINT, USINT, UINT, UDINT, ULINT, REAL, or LREAL. In `ADD(42, someVar)`, the number of candidate type combinations for the expression grows multiplicatively with nesting depth.
**Why it happens:** Naive candidate enumeration generates all possible type assignments. With 3+ operands in nested expressions, candidate lists grow exponentially.
**How to avoid:** Prune candidates early using context. If the target variable is DINT, prune candidates to types that can widen to DINT. Limit candidate set to 16 entries max. MATIEC handles this by narrowing top-down after filling bottom-up.
**Warning signs:** Type checker hangs or takes >1s on expressions with 4+ mixed-type operands.

### Pitfall 2: Case-Insensitive But Case-Preserving Identifiers
**What goes wrong:** IEC 61131-3 specifies that identifiers are case-insensitive. `myVar`, `MYVAR`, and `MyVar` all refer to the same symbol. But diagnostics should show the original casing from the declaration.
**Why it happens:** Developers store identifiers as-is and compare case-sensitively. Works for test cases with consistent casing; fails on real production code.
**How to avoid:** Normalize to uppercase for storage/lookup keys. Store original name in the Symbol struct for display in diagnostics. Use the declared name in error messages.

### Pitfall 3: Forward References Across Files
**What goes wrong:** File A declares `FB_Motor` and uses `FB_Pump` (from file B). File B declares `FB_Pump` and uses `FB_Motor` (from file A). Single-pass analysis of A fails because `FB_Pump` is not yet known.
**Why it happens:** Analyzing files in order, one at a time, without a pre-registration step.
**How to avoid:** Two-pass is locked. Pass 1 collects all POU names and signatures from all files. Pass 2 resolves references and type-checks bodies. Order of files does not matter after pass 1.

### Pitfall 4: Missing VAR_IN_OUT Semantics
**What goes wrong:** `VAR_IN_OUT` parameters are passed by reference. Assigning a literal to a VAR_IN_OUT parameter is illegal (no address to reference). Type-checking VAR_IN_OUT requires checking that the argument is an lvalue.
**Why it happens:** Treating all parameters as pass-by-value for type checking purposes.
**How to avoid:** Track parameter direction (Input, Output, InOut) in the symbol table. For InOut parameters, verify the argument is an addressable expression (variable, array element, struct member), not a literal or computed expression.

### Pitfall 5: Enum Qualified Access
**What goes wrong:** IEC 61131-3 Ed.3 supports qualified enum access: `E_Color.Red` or `E_Color#Red`. The type checker must resolve `E_Color` as a type, then look up `Red` within that type's scope, not the enclosing scope.
**Why it happens:** Treating enum values as global names (which works for unqualified access) but failing on qualified access.
**How to avoid:** Each enum type gets its own scope containing its values. MemberAccessExpr on an enum type name resolves to the enum scope. Support both `EnumType.Value` and `EnumType#Value` syntax.

### Pitfall 6: FB Instance vs FB Type Confusion
**What goes wrong:** `motor : FB_Motor;` declares an instance. `FB_Motor(...)` is a function call syntax error for FBs (FBs are called via their instance: `motor(IN := TRUE);`). Type checker must distinguish "this identifier refers to a type" from "this identifier refers to an instance."
**Why it happens:** Functions and FBs look syntactically similar but have different call semantics. Functions are called by name; FBs are called by instance name.
**How to avoid:** Symbol kind field distinguishes FunctionBlock (type) from Variable (instance of FB). Function calls resolve to Function symbols; FB calls resolve to Variable symbols whose type is a FunctionBlock.

## Code Examples

### Diagnostic Code Convention
```go
// pkg/checker/diag_codes.go
const (
    // Type errors
    CodeTypeMismatch     = "SEMA001" // type mismatch in assignment/comparison
    CodeNoImplicitConv   = "SEMA002" // no implicit conversion from X to Y
    CodeIncompatibleOp   = "SEMA003" // operator not defined for type

    // Name resolution
    CodeUndeclared       = "SEMA010" // undeclared identifier
    CodeRedeclared       = "SEMA011" // identifier already declared in scope
    CodeUnusedVar        = "SEMA012" // declared variable never referenced
    CodeUnreachableCode  = "SEMA013" // code after RETURN/EXIT

    // Structural
    CodeWrongArgCount    = "SEMA020" // wrong number of arguments
    CodeWrongArgType     = "SEMA021" // argument type mismatch
    CodeNotCallable      = "SEMA022" // identifier is not a function/FB
    CodeNotIndexable     = "SEMA023" // type does not support indexing
    CodeNoMember         = "SEMA024" // type has no member with this name
    CodeInOutRequiresVar = "SEMA025" // VAR_IN_OUT requires variable, not literal

    // Vendor warnings
    CodeVendorOOP        = "VEND001" // OOP not supported by target vendor
    CodeVendorPointer    = "VEND002" // POINTER TO not supported
    CodeVendorReference  = "VEND003" // REFERENCE TO not supported
    CodeVendorStringLen  = "VEND004" // string length exceeds vendor limit
    CodeVendor64Bit      = "VEND005" // 64-bit type not supported
    CodeVendorWString    = "VEND006" // WSTRING not supported
)
```

### Analyzer Public API
```go
// pkg/analyzer/analyzer.go
package analyzer

type AnalysisResult struct {
    Symbols *symbols.Table
    Diags   []diag.Diagnostic
}

// Analyze runs semantic analysis on parsed source files.
// Pass a nil config for default (portable) behavior.
func Analyze(files []*ast.SourceFile, cfg *project.Config) AnalysisResult {
    table := symbols.NewTable()
    diags := diag.NewCollector()

    // Pass 1: Collect all declarations
    resolver := checker.NewResolver(table, diags)
    resolver.CollectDeclarations(files)

    // Pass 2: Type-check bodies
    ch := checker.NewChecker(table, diags)
    ch.CheckBodies(files)

    // Pass 3 (optional): Vendor-aware warnings
    if cfg != nil && cfg.Build.VendorTarget != "" {
        profile := checker.LookupVendor(cfg.Build.VendorTarget)
        if profile != nil {
            checker.CheckVendorCompat(files, table, profile, diags)
        }
    }

    return AnalysisResult{
        Symbols: table,
        Diags:   diags.All(),
    }
}
```

### Expression Type Checking (with candidate/narrow)
```go
// pkg/checker/check.go
func (c *Checker) checkBinaryExpr(expr *ast.BinaryExpr) types.Type {
    left := c.checkExpr(expr.Left)
    right := c.checkExpr(expr.Right)

    if left == types.Invalid || right == types.Invalid {
        return types.Invalid // propagate errors, don't cascade
    }

    op := expr.Op.Text
    switch {
    case isArithmeticOp(op): // +, -, *, /
        common, ok := types.CommonType(left.Kind(), right.Kind())
        if !ok {
            c.diags.Errorf(expr.Op.Span.Start.ToSourcePos(), CodeIncompatibleOp,
                "operator %s not defined for types %s and %s", op, left, right)
            return types.Invalid
        }
        return types.PrimitiveType(common)

    case isComparisonOp(op): // =, <>, <, >, <=, >=
        _, ok := types.CommonType(left.Kind(), right.Kind())
        if !ok {
            c.diags.Errorf(expr.Op.Span.Start.ToSourcePos(), CodeIncompatibleOp,
                "cannot compare %s and %s", left, right)
            return types.Invalid
        }
        return types.Bool

    case isBooleanOp(op): // AND, OR, XOR
        if left.Kind() != types.KindBOOL || right.Kind() != types.KindBOOL {
            c.diags.Errorf(expr.Op.Span.Start.ToSourcePos(), CodeTypeMismatch,
                "boolean operator %s requires BOOL operands, got %s and %s", op, left, right)
            return types.Invalid
        }
        return types.Bool
    }
    return types.Invalid
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Single-pass type inference | Two-pass candidate/narrow (MATIEC) | Proven since 2013 | Handles overloaded standard functions correctly |
| Case-sensitive identifiers | Case-insensitive (IEC spec) | Standard requirement | Must normalize at all lookup points |
| Skip OOP in type system | Parse OOP, defer deep checking | Project decision | Method signatures and FB calls checked; inheritance deferred |
| Global flat namespace | Hierarchical scopes | Standard practice | Correct name shadowing and POU-local resolution |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify |
| Config file | go.mod (existing) |
| Quick run command | `go test ./pkg/types/... ./pkg/symbols/... ./pkg/checker/... ./pkg/analyzer/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SEMA-01 | Type mismatch errors with positions | unit | `go test ./pkg/checker/... -run TestTypeMismatch -count=1` | Wave 0 |
| SEMA-02 | All IEC primitive types resolved | unit | `go test ./pkg/types/... -run TestPrimitiveTypes -count=1` | Wave 0 |
| SEMA-03 | Arrays, structs, enums, FB instances, method calls | unit | `go test ./pkg/checker/... -run TestComplex -count=1` | Wave 0 |
| SEMA-04 | Undeclared/unused vars, unreachable code | unit | `go test ./pkg/checker/... -run TestDiagnostics -count=1` | Wave 0 |
| SEMA-05 | Cross-file symbol resolution | integration | `go test ./pkg/analyzer/... -run TestCrossFile -count=1` | Wave 0 |
| SEMA-06 | stc check CLI with JSON output | integration | `go test ./cmd/stc/... -run TestCheck -count=1` | Wave 0 |
| SEMA-07 | Vendor-aware diagnostics | unit | `go test ./pkg/checker/... -run TestVendor -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./pkg/types/... ./pkg/symbols/... ./pkg/checker/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `pkg/types/types_test.go` -- type lattice, widening rules, ANY hierarchy membership
- [ ] `pkg/types/lattice_test.go` -- CommonType, CanWiden for all type combinations
- [ ] `pkg/symbols/scope_test.go` -- scope chain lookup, case-insensitive, shadowing
- [ ] `pkg/checker/resolve_test.go` -- pass 1 declaration collection
- [ ] `pkg/checker/check_test.go` -- pass 2 type checking for all expression/statement types
- [ ] `pkg/checker/vendor_test.go` -- vendor profile diagnostics
- [ ] `pkg/analyzer/analyzer_test.go` -- end-to-end with multi-file ST fixtures
- [ ] `cmd/stc/check_test.go` -- CLI integration test for `stc check`
- [ ] Test fixtures in `pkg/checker/testdata/` -- ST files for each error case

## Open Questions

1. **Pos type conversion between packages**
   - What we know: ast.Pos, lexer.Pos, and source.Pos are separate types with identical fields (to avoid circular imports, as documented in Phase 1 decisions)
   - What's unclear: Will the checker need to convert between these Pos types, or can it work entirely with source.Pos?
   - Recommendation: The checker should work with source.Pos for diagnostics. Add a `ToSourcePos()` helper on ast.Pos if needed. Keep it simple -- three identical struct types is a minor inconvenience, not a design flaw.

2. **Standard function signature registry**
   - What we know: Phase 4 (Standard Library) will implement the actual function bodies. Phase 3 needs just the type signatures for type checking.
   - What's unclear: How many standard functions need signatures in Phase 3?
   - Recommendation: Include signatures for the 20 most-used standard functions (ADD, SUB, MUL, DIV, MOD, ABS, MIN, MAX, SEL, MUX, LIMIT, MOVE, type conversion functions). Full coverage comes in Phase 4. Register them with ANY_NUM/ANY_INT/ANY_REAL generic types.

3. **Symbol table as Phase 9 (LSP) input**
   - What we know: The LSP will need to query the symbol table for go-to-definition, hover, completions.
   - What's unclear: Does the symbol table API need to support position-based lookup (find symbol at offset X)?
   - Recommendation: Design the Symbol struct with Span (declaration site) from the start. Add a `SymbolAt(file, offset)` method stub that Phase 9 can implement. Do not over-engineer for LSP now -- batch analysis is the primary consumer.

## Sources

### Primary (HIGH confidence)
- [MATIEC compiler documentation](https://openplcproject.gitlab.io/matiec/) -- Two-pass type checking algorithm (fill_candidate_datatypes, narrow_candidate_datatypes, print_datatype_errors)
- [Fernhill IEC 61131-3 Generic Data Types](https://www.fernhillsoftware.com/help/iec-61131/common-elements/datatypes-generic.html) -- Complete ANY type hierarchy
- [Fernhill IEC 61131-3 Elementary Data Types](https://www.fernhillsoftware.com/help/iec-61131/common-elements/datatypes-elementary.html) -- All elementary type specifications
- [PLCnext Implicit Type Conversion](https://engineer.plcnext.help/2022.0_LTS_en/DataTypes_ImpliciteTypeConversion.htm) -- IEC implicit widening rules
- [Go type checker source](https://go.dev/src/go/types/check.go) -- Scope chain and type checking patterns
- [Go types package documentation](https://pkg.go.dev/go/types) -- Scope, Object, Type interfaces

### Secondary (MEDIUM confidence)
- [Go compiler internals - type checker](https://internals-for-interns.com/posts/the-go-type-checker/) -- Scope chain walkthrough
- [Go gotypes example](https://go.googlesource.com/example/+/HEAD/gotypes/README.md) -- Symbol table usage patterns

### Tertiary (LOW confidence)
- Existing project code (`pkg/ast/`, `pkg/diag/`, `pkg/project/`) -- Direct inspection, HIGH confidence but noted here for completeness

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- pure Go, no new dependencies, existing patterns
- Architecture: HIGH -- MATIEC two-pass algorithm is well-documented and proven; Go compiler patterns are canonical
- Type system: HIGH -- IEC 61131-3 type hierarchy is fixed and well-specified in the standard
- Pitfalls: HIGH -- documented in prior research and confirmed against MATIEC source

**Research date:** 2026-03-27
**Valid until:** 2026-04-27 (stable domain, IEC standard does not change frequently)
