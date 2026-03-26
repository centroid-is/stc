# Phase 1: Project Bootstrap & Parser - Research

**Researched:** 2026-03-26
**Domain:** Go project scaffold, IEC 61131-3 Structured Text lexer/parser, CLI foundation
**Confidence:** HIGH

## Summary

Phase 1 bootstraps the entire STC project: Go module, hand-written lexer, recursive descent parser with error recovery, AST types, CLI skeleton with `stc parse`, and CI pipeline. This is a greenfield Go project targeting IEC 61131-3 Edition 3 Structured Text with CODESYS OOP extensions. The parser must produce a CST-first tree that preserves all tokens (for future formatter/LSP), handle error recovery via panic-mode synchronization, and output JSON AST via the CLI.

The core technical challenge is building a hand-written recursive descent parser for a language whose grammar is not cleanly LALR(1)-parseable as specified in the IEC standard. Identifier ambiguity (is `FOO` a variable, type, function, or enum value?) requires the parser to defer resolution to semantic analysis -- parsing must be purely syntactic. The Pratt parsing technique handles expression operator precedence cleanly. Error recovery synchronizes at `;`, `END_*`, and `VAR` boundaries to produce partial ASTs with error nodes.

**Primary recommendation:** Build lexer first with exhaustive keyword table, then parser with Pratt expressions, using golden file tests throughout. Design CST node types to carry trivia (whitespace, comments) from day one. Use Cobra for CLI, BurntSushi/toml for stc.toml config. Keep all compiler logic in `pkg/` -- CLI in `cmd/stc/` is a thin wrapper.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Go module path: `github.com/centroid-is/stc`
- Package layout: `cmd/stc/`, `pkg/lexer/`, `pkg/parser/`, `pkg/ast/` -- following the agent plan structure
- Build system: Makefile with `build`, `test`, `lint`, `install` targets
- Go version: 1.22+ (latest stable)
- CST-first approach: preserve all tokens (whitespace, comments) in the tree -- needed for formatter (Phase 8) and LSP, avoids painful retrofit. Based on rust-analyzer precedent.
- Error recovery: panic-mode with synchronization at `;`, `END_*`, `VAR` boundaries -- proven in Go/Rust compilers
- CODESYS OOP: parse all OOP syntax (METHOD, INTERFACE, PROPERTY, EXTENDS, IMPLEMENTS) but defer type-checking to Phase 3
- Hand-written lexer in Go with keyword table lookup
- CI matrix: macOS + Windows + Linux with Go 1.22, run on every PR
- Linting: golangci-lint with default + govet + errcheck + staticcheck
- Branch strategy: feature branches to PRs to main, no direct pushes
- Test coverage: `go test -coverprofile` with coverage badge in README

### Claude's Discretion
- Specific CST node type naming conventions
- Internal error representation details
- Test fixture organization
- Makefile target naming beyond the core four

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PARS-01 | Parse IEC 61131-3 Ed.3 ST source files (PROGRAM, FUNCTION_BLOCK, FUNCTION, TYPE declarations) | Core parser architecture with recursive descent; declaration parsing functions per POU type |
| PARS-02 | Parse CODESYS OOP extensions (METHOD, INTERFACE, PROPERTY, EXTENDS, IMPLEMENTS) | OOP node types in AST; parser functions for each OOP construct; defer type-checking |
| PARS-03 | Parse CODESYS pointer/reference types (POINTER TO, REFERENCE TO) | Type expression parser handles compound type specifiers |
| PARS-04 | Parse 64-bit types (LINT, LREAL, LWORD, ULINT) | Keyword table includes all 64-bit type keywords; type specifier parser handles them |
| PARS-05 | Produce partial ASTs from broken code with error nodes (error recovery for LSP) | Panic-mode error recovery; ErrorNode AST type; synchronization points |
| PARS-06 | Output AST as JSON via `stc parse <file> --format json` | Cobra CLI with parse subcommand; JSON marshaling on all AST nodes |
| PARS-07 | Parse all ST control structures (IF/CASE/FOR/WHILE/REPEAT) with full operator precedence | Pratt parser for expressions; recursive descent for statements; IEC precedence table |
| PARS-08 | Parse VAR sections (VAR, VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT, VAR_TEMP, VAR_GLOBAL) | Variable declaration block parser with section-type tracking |
| PARS-09 | Parse arrays, structs, enums, subranges, typed literals | Type declaration parser; literal parser handles typed literals (INT#5, T#5s, 16#FF) |
| PARS-10 | Parse {attribute '...'} pragmas (Beckhoff-style) | Lexer emits pragma tokens; parser attaches them to following declarations |
| CLI-01 | Single binary `stc` with subcommands: parse (+ stubs for future: check, test, emit, lint, fmt, pp) | Cobra command tree; stub subcommands return "not yet implemented" |
| CLI-02 | Every subcommand supports `--format json` for machine-readable output | Global `--format` flag on root command; JSON output path in parse |
| CLI-03 | `stc --version` outputs version information | Cobra version flag; ldflags injection at build time |
| CLI-04 | Project manifest (stc.toml) defines source roots, vendor target, library paths | BurntSushi/toml for config parsing; config struct in pkg/project/ |
| CLI-05 | All diagnostics include file:line:col with actionable error messages | Diagnostic type with Pos{File, Line, Col}; structured error messages |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.26.0 (installed) | Implementation language | Single static binary, fast compilation. Go 1.22+ required by CONTEXT.md but 1.26.0 is what is installed and fully backward compatible. |
| `spf13/cobra` | v1.10.x | CLI command framework | The standard for Go CLIs. Used by kubectl, docker, gh, hugo. Provides subcommands, flags, help, shell completion. |
| `github.com/BurntSushi/toml` | v1.5.0 | TOML config parsing | The standard Go TOML library. stc.toml is the project manifest format per CLI-04. |
| `github.com/stretchr/testify` | v1.10.x | Test assertions | `assert` and `require` packages for cleaner test assertions. Table-driven test pattern. |
| `github.com/google/go-cmp` | v0.6.x | Deep comparison | Better diff output than `reflect.DeepEqual` for AST comparison in tests. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` | stdlib | JSON AST output | For `--format json` on all CLI commands |
| `fmt`, `os`, `path/filepath` | stdlib | File I/O and paths | Source file reading, path resolution |
| `unicode`, `unicode/utf8` | stdlib | Character classification | Lexer needs Unicode-aware identifier parsing |
| `strings`, `strconv` | stdlib | String/number parsing | Literal value parsing in lexer |
| `embed` | stdlib | Embedded test fixtures | Golden test files embedded in test binary |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `BurntSushi/toml` | `spf13/viper` | Viper is heavier, pulls in many dependencies. TOML-only config does not need Viper's env/flag binding. |
| `spf13/cobra` | `urfave/cli` v2 | Both mature. Cobra wins on ecosystem size and documentation. |
| `stretchr/testify` | stdlib `testing` only | Testify adds cleaner assertions and require (fail-fast). Worth the dependency for a project this size. |

**Installation:**
```bash
go mod init github.com/centroid-is/stc
go get github.com/spf13/cobra@latest
go get github.com/BurntSushi/toml@latest
go get github.com/stretchr/testify@latest
go get github.com/google/go-cmp@latest
```

## Architecture Patterns

### Recommended Project Structure
```
cmd/
  stc/
    main.go           # Cobra root command, subcommand registration
    parse.go          # parse subcommand
    version.go        # version subcommand
pkg/
  ast/
    node.go           # Base Node interface, NodeKind enum
    decl.go           # POU declarations: Program, FunctionBlock, Function, Interface, Type
    stmt.go           # Statements: If, Case, For, While, Repeat, Assignment, Call, Return, Exit
    expr.go           # Expressions: Binary, Unary, Literal, Ident, Call, MemberAccess, Index, Deref
    types.go          # Type specifiers: Named, Array, Pointer, Reference, String, Subrange, Enum, Struct
    var.go            # Variable declarations and VAR sections
    trivia.go         # Whitespace and comment trivia tokens attached to nodes
    visitor.go        # Visitor interface
    json.go           # JSON marshaling for all node types
  lexer/
    lexer.go          # Scanner: Scan() returns next Token
    token.go          # Token struct and TokenKind enum
    keywords.go       # Keyword lookup table (map[string]TokenKind)
    position.go       # Pos struct (File, Line, Col, Offset)
  parser/
    parser.go         # Parser struct with advance/expect/peek helpers
    decl.go           # POU parsing: parseProgram, parseFunctionBlock, parseFunction, parseInterface, parseType
    stmt.go           # Statement parsing: parseIf, parseCase, parseFor, parseWhile, parseRepeat
    expr.go           # Pratt expression parser: parseExpr with precedence levels
    types.go          # Type specifier parsing: parseTypeSpec
    var.go            # VAR section parsing
    error.go          # Error recovery: synchronize(), ErrorNode creation
  diag/
    diagnostic.go     # Diagnostic struct: Severity, Pos, EndPos, Message, Code
    collector.go      # DiagCollector accumulates diagnostics
  project/
    config.go         # stc.toml parsing: SourceRoots, VendorTarget, LibPaths
  source/
    source.go         # SourceFile: filename, content, line offsets for position lookup
```

### Pattern 1: CST-First with Trivia Attachment

**What:** Every token (including whitespace and comments) is preserved in the tree. Each AST node has `LeadingTrivia` and `TrailingTrivia` fields. The parser produces a lossless representation of the source.

**When to use:** Always -- this is a locked decision. The CST enables the formatter (Phase 8) and LSP (Phase 9) without retrofitting.

**Example:**
```go
// pkg/ast/node.go
type Node interface {
    Kind() NodeKind
    Span() source.Span      // Start/end position in source
    Children() []Node        // Child nodes for tree walking
}

// Trivia represents whitespace, comments, and other non-semantic tokens
type Trivia struct {
    Kind  TriviaKind  // Whitespace, LineComment, BlockComment
    Text  string
    Span  source.Span
}

// Every concrete node embeds NodeBase
type NodeBase struct {
    NodeKind      NodeKind
    NodeSpan      source.Span
    LeadingTrivia []Trivia
    TrailingTrivia []Trivia
}
```

### Pattern 2: Error-Tolerant Pipeline (T, []Diagnostic)

**What:** The parser always returns an AST, even from broken code. Errors become ErrorNode entries in the AST and diagnostics in the result. This is the rust-analyzer pattern: `(T, Vec<Error>)` not `Result<T, Error>`.

**When to use:** Every pipeline stage. Critical for LSP.

**Example:**
```go
// pkg/parser/parser.go
type ParseResult struct {
    File  *ast.SourceFile  // Always non-nil, may contain ErrorNodes
    Diags []diag.Diagnostic
}

// Parser always succeeds -- errors are collected, not fatal
func Parse(src *source.SourceFile) ParseResult {
    p := newParser(src)
    file := p.parseSourceFile()
    return ParseResult{File: file, Diags: p.diags.All()}
}
```

### Pattern 3: Pratt Parsing for Expressions

**What:** Use top-down operator precedence (Pratt) parsing for the expression sub-grammar. Recursive descent handles statements and declarations; Pratt handles expressions with correct precedence.

**When to use:** All expression parsing -- binary operators, unary operators, function calls, member access, array indexing.

**IEC 61131-3 Operator Precedence Table (highest to lowest):**

| Precedence | Operators | Description |
|------------|-----------|-------------|
| 11 | `()` | Parenthesized expressions |
| 10 | Function call, `.` member, `[]` index, `^` deref | Postfix operations |
| 9 | `NOT`, `-` (unary) | Unary prefix |
| 8 | `**` | Exponentiation |
| 7 | `*`, `/`, `MOD` | Multiplicative |
| 6 | `+`, `-` | Additive |
| 5 | `<`, `>`, `<=`, `>=` | Relational |
| 4 | `=`, `<>` | Equality |
| 3 | `AND`, `&` | Logical AND |
| 2 | `XOR` | Logical XOR |
| 1 | `OR` | Logical OR |

**Example:**
```go
// pkg/parser/expr.go
func (p *Parser) parseExpr(minPrec int) ast.Expr {
    left := p.parseUnaryExpr()
    for {
        prec := p.infixPrecedence(p.peek().Kind)
        if prec < minPrec {
            break
        }
        op := p.advance()
        // Right-associative for ** only
        nextPrec := prec + 1
        if op.Kind == token.Power {
            nextPrec = prec
        }
        right := p.parseExpr(nextPrec)
        left = &ast.BinaryExpr{Left: left, Op: op, Right: right}
    }
    return left
}
```

### Pattern 4: Panic-Mode Error Recovery

**What:** When the parser encounters an unexpected token, it enters recovery mode: wraps consumed tokens in an ErrorNode, then skips tokens until it finds a synchronization point (`;`, `END_*`, `VAR_*`, or declaration keyword).

**When to use:** At statement level and declaration level.

**Synchronization tokens:**
- Statement level: `;`, `END_IF`, `END_FOR`, `END_WHILE`, `END_REPEAT`, `END_CASE`
- Declaration level: `END_VAR`, `END_FUNCTION_BLOCK`, `END_FUNCTION`, `END_PROGRAM`, `END_INTERFACE`, `END_TYPE`, `END_METHOD`, `END_PROPERTY`
- Block level: `VAR`, `VAR_INPUT`, `VAR_OUTPUT`, `VAR_IN_OUT`, `VAR_TEMP`, `VAR_GLOBAL`

**Example:**
```go
// pkg/parser/error.go
func (p *Parser) synchronize(stopAt ...token.Kind) {
    stopSet := makeSet(stopAt...)
    for !p.atEnd() {
        if stopSet[p.peek().Kind] {
            return
        }
        // Also stop at any statement or declaration boundary
        if p.isStatementStart() || p.isDeclarationStart() {
            return
        }
        p.advance() // skip token
    }
}

func (p *Parser) recoverStatement() ast.Stmt {
    start := p.peek().Span.Start
    p.addDiag(diag.Error, p.peek().Span, "unexpected %s", p.peek().Kind)
    p.synchronize(token.Semicolon, token.EndIf, token.EndFor,
        token.EndWhile, token.EndRepeat, token.EndCase)
    // Consume the semicolon if present
    if p.peek().Kind == token.Semicolon {
        p.advance()
    }
    return &ast.ErrorNode{NodeSpan: source.SpanFrom(start, p.prev().Span.End)}
}
```

### Pattern 5: Golden File Testing

**What:** Parser tests use golden files: `.st` input files paired with `.ast.json` expected output files. Tests parse the input, marshal the AST to JSON, and compare against the golden file. An `-update` flag regenerates golden files.

**When to use:** All parser and lexer tests.

**Example:**
```go
func TestGolden(t *testing.T) {
    files, _ := filepath.Glob("testdata/*.st")
    for _, f := range files {
        t.Run(filepath.Base(f), func(t *testing.T) {
            src, _ := os.ReadFile(f)
            result := parser.Parse(source.New(filepath.Base(f), string(src)))
            actual, _ := json.MarshalIndent(result.File, "", "  ")
            golden := f + ".json"
            if *update {
                os.WriteFile(golden, actual, 0644)
                return
            }
            expected, _ := os.ReadFile(golden)
            assert.JSONEq(t, string(expected), string(actual))
        })
    }
}
```

### Anti-Patterns to Avoid

- **Compiler logic in CLI:** All parsing, AST types, diagnostics live in `pkg/`. `cmd/stc/` only wires Cobra flags to `pkg/` function calls.
- **Failing on first error:** Parser must ALWAYS return an AST. Use `(result, diagnostics)` pattern, never `(result, error)` that stops on first failure.
- **Parser resolves types during parsing:** The parser must be purely syntactic. Whether `FOO` is a type or variable is resolved in Phase 3 (semantic analysis). Parse it as an identifier and let the resolver figure it out.
- **Skipping OOP syntax:** PARS-02 requires METHOD, INTERFACE, PROPERTY, EXTENDS, IMPLEMENTS from day one. The AST must have these node types. Deferring OOP parsing to a later phase would require a parser rewrite.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CLI argument parsing | Custom flag parser | `spf13/cobra` | Subcommands, help generation, shell completion, flag validation |
| TOML config parsing | Custom TOML parser | `BurntSushi/toml` v1.5.0 | TOML spec is deceptively complex (multiline strings, nested tables, datetime) |
| JSON marshaling | Custom JSON serializer | `encoding/json` stdlib | Standard, tested, handles all Go types |
| Test assertions | `if got != want { t.Errorf(...) }` | `testify/assert` + `testify/require` | Cleaner, better error messages, fail-fast with require |
| AST deep comparison | `reflect.DeepEqual` | `go-cmp` | Readable diffs showing exactly which field differs |
| Unicode handling | Byte-level scanning | `unicode`, `unicode/utf8` stdlib | Correct handling of multi-byte identifiers, BOM detection |

**Key insight:** The lexer and parser are the only things that should be hand-written. Everything surrounding them (CLI, config, testing, JSON) should use proven libraries.

## Complete ST Keyword Table

The lexer must recognize all of these as keywords (case-insensitive):

**POU declarations:** PROGRAM, END_PROGRAM, FUNCTION_BLOCK, END_FUNCTION_BLOCK, FUNCTION, END_FUNCTION, TYPE, END_TYPE, INTERFACE, END_INTERFACE, METHOD, END_METHOD, PROPERTY, END_PROPERTY, ACTION, END_ACTION

**Variable sections:** VAR, VAR_INPUT, VAR_OUTPUT, VAR_IN_OUT, VAR_TEMP, VAR_GLOBAL, VAR_ACCESS, VAR_EXTERNAL, VAR_CONFIG, END_VAR, CONSTANT, RETAIN, PERSISTENT, AT

**Control flow:** IF, THEN, ELSIF, ELSE, END_IF, CASE, OF, END_CASE, FOR, TO, BY, DO, END_FOR, WHILE, END_WHILE, REPEAT, UNTIL, END_REPEAT, EXIT, CONTINUE, RETURN

**OOP:** EXTENDS, IMPLEMENTS, THIS, SUPER, ABSTRACT, FINAL, OVERRIDE, PUBLIC, PRIVATE, PROTECTED, INTERNAL

**Type system:** ARRAY, OF, STRUCT, END_STRUCT, POINTER, REFERENCE, TO, STRING, WSTRING

**Primitive types:** BOOL, BYTE, WORD, DWORD, LWORD, SINT, INT, DINT, LINT, USINT, UINT, UDINT, ULINT, REAL, LREAL, TIME, DATE, TIME_OF_DAY, TOD, DATE_AND_TIME, DT

**Boolean/logical:** TRUE, FALSE, AND, OR, XOR, NOT, MOD

**Operators (as tokens):** `:=`, `=>`, `+`, `-`, `*`, `/`, `**`, `=`, `<>`, `<`, `>`, `<=`, `>=`, `&`, `(`, `)`, `[`, `]`, `.`, `..`, `,`, `;`, `:`, `^`, `#`

**Pragmas:** `{` ... `}` (Beckhoff attribute pragmas, treated as single pragma token)

**Comments:** `//` line comments, `(* ... *)` block comments (nestable per IEC spec)

## Common Pitfalls

### Pitfall 1: IEC Grammar Identifier Ambiguity
**What goes wrong:** The parser cannot tell if `FOO` is a variable, type, function name, or enum value without a symbol table. Teams that try to resolve during parsing create circular dependencies.
**Why it happens:** IEC 61131-3 grammar was written for human understanding, not parser generators.
**How to avoid:** Parser is purely syntactic. Parse identifiers as identifiers. Name resolution happens in Phase 3 (semantic analysis). When parsing a type position (e.g., `VAR x : FOO`), parse `FOO` as a NamedType referencing an unresolved identifier.
**Warning signs:** Parser code importing symbol table packages; parser needing two passes; tests failing when type names match variable names.

### Pitfall 2: Nested Block Comments
**What goes wrong:** Many parsers fail on `(* outer (* inner *) still outer *)` because they treat the first `*)` as closing the comment.
**Why it happens:** IEC 61131-3 specifies that block comments nest.
**How to avoid:** Lexer tracks comment nesting depth. Increment on `(*`, decrement on `*)`. Only end the comment when depth reaches zero.
**Warning signs:** Tests with nested comments produce wrong token streams.

### Pitfall 3: Case-Insensitive Keywords
**What goes wrong:** ST keywords are case-insensitive: `IF`, `If`, `if`, `iF` are all the same keyword. Parsers that do string equality fail on production code with mixed casing.
**Why it happens:** IEC standard specifies case-insensitivity for keywords and identifiers.
**How to avoid:** Lexer uppercases (or lowercases) keyword lookup. The keyword table maps uppercase strings to token kinds. The original casing is preserved in the token text for CST fidelity.
**Warning signs:** Parser fails on code pasted from vendor IDEs that use different casing conventions.

### Pitfall 4: Typed Literals and Number Bases
**What goes wrong:** ST has complex literal syntax: `INT#5`, `REAL#3.14`, `16#FF`, `8#77`, `2#1010`, `T#5s`, `T#1h2m3s`, `D#2024-01-15`, `DT#2024-01-15-12:30:00`, `TOD#12:30:00`. Lexers that handle only simple integers/floats break on production code.
**Why it happens:** IEC 61131-3 has rich literal syntax for typed values, time durations, dates, and multi-base integers.
**How to avoid:** Lexer has dedicated scanning functions for each literal category. Number scanning handles `#` prefix for base specification. Time/date literals are scanned as compound tokens.
**Warning signs:** Parser fails on `T#1h30m`, `16#DEAD_BEEF`, or `LINT#123456789`.

### Pitfall 5: CASE Statement with Ranges and Lists
**What goes wrong:** CASE branches can use ranges (`1..10:`) and comma-separated lists (`1, 3, 5:`). Parsers that only handle single-value case labels miss these.
**Why it happens:** This syntax is in the standard but rarely covered in simple tutorials.
**How to avoid:** Case label parser handles three forms: single value, range (`expr .. expr`), and comma-separated list. The `..` token must be lexed as a distinct operator.
**Warning signs:** Parser fails on `CASE x OF 1..10: ... 20, 30, 40: ... END_CASE`.

### Pitfall 6: Semicolons Are Optional After END_* in Some Contexts
**What goes wrong:** Strict semicolon enforcement rejects valid production code. In practice, a semicolon after `END_IF` is optional before `END_FUNCTION_BLOCK`. Some vendor IDEs generate code without these trailing semicolons.
**Why it happens:** The IEC standard is ambiguous about where semicolons are required vs optional.
**How to avoid:** Be permissive about optional semicolons after `END_*` keywords. Accept both `END_IF;` and `END_IF` followed by another statement or `END_*`.
**Warning signs:** Parser rejects code that compiles fine in TwinCAT or CODESYS.

### Pitfall 7: CST Trivia Attachment Ambiguity
**What goes wrong:** When a comment appears between two statements, it is ambiguous whether it is trailing trivia of the first statement or leading trivia of the second. Incorrect attachment breaks the formatter.
**Why it happens:** Trivia attachment is not specified by the grammar -- it is a heuristic decision.
**How to avoid:** Use the rule: whitespace and comments on the same line as a token are trailing trivia; whitespace and comments on a new line before a token are leading trivia of the next token. This matches the convention used by roslyn (C# compiler) and rust-analyzer.
**Warning signs:** Formatter moves comments to wrong positions after reformatting.

## Code Examples

### Lexer Token Types
```go
// pkg/lexer/token.go
type TokenKind int

const (
    // Literals and identifiers
    Illegal TokenKind = iota
    EOF
    Ident           // variable/type names
    IntLiteral      // 42, 16#FF, 2#1010
    RealLiteral     // 3.14, 1.0E-5
    StringLiteral   // 'hello'
    WStringLiteral  // "hello" (wide string)
    TimeLiteral     // T#5s, T#1h30m
    DateLiteral     // D#2024-01-15
    DateTimeLiteral // DT#2024-01-15-12:30:00
    TodLiteral      // TOD#12:30:00
    TypedLiteral    // INT#5, BOOL#TRUE

    // Punctuation
    LParen    // (
    RParen    // )
    LBracket  // [
    RBracket  // ]
    LBrace    // {  (pragma start)
    RBrace    // }  (pragma end)
    Comma     // ,
    Semicolon // ;
    Colon     // :
    Dot       // .
    DotDot    // ..
    Caret     // ^  (dereference)
    Hash      // #
    Arrow     // =>

    // Operators
    Assign      // :=
    Plus        // +
    Minus       // -
    Star        // *
    Slash       // /
    Power       // **
    Eq          // =
    NotEq       // <>
    Less        // <
    LessEq      // <=
    Greater     // >
    GreaterEq   // >=
    Ampersand   // &

    // Keywords (abbreviated -- full set in keywords.go)
    KwProgram         // PROGRAM
    KwEndProgram      // END_PROGRAM
    KwFunctionBlock   // FUNCTION_BLOCK
    KwEndFunctionBlock // END_FUNCTION_BLOCK
    KwFunction        // FUNCTION
    KwEndFunction     // END_FUNCTION
    // ... all ~80 keywords
    KwIf
    KwThen
    KwElsif
    KwElse
    KwEndIf
    // ... etc.

    // Trivia (preserved for CST)
    Whitespace
    LineComment   // // ...
    BlockComment  // (* ... *)
    Pragma        // { ... }
)
```

### AST Declaration Nodes
```go
// pkg/ast/decl.go

// SourceFile is the root node
type SourceFile struct {
    NodeBase
    Declarations []Declaration
}

type Declaration interface {
    Node
    declNode()
}

type ProgramDecl struct {
    NodeBase
    Name       *Ident
    VarBlocks  []*VarBlock
    Body       []Statement
}

type FunctionBlockDecl struct {
    NodeBase
    Name       *Ident
    Extends    *Ident        // nil if no EXTENDS
    Implements []*Ident      // empty if no IMPLEMENTS
    VarBlocks  []*VarBlock
    Body       []Statement
    Methods    []*MethodDecl
    Properties []*PropertyDecl
}

type InterfaceDecl struct {
    NodeBase
    Name       *Ident
    Extends    []*Ident      // interfaces can extend multiple
    Methods    []*MethodSignature
    Properties []*PropertySignature
}

type MethodDecl struct {
    NodeBase
    AccessModifier AccessModifier  // PUBLIC, PRIVATE, PROTECTED, INTERNAL
    Name           *Ident
    ReturnType     TypeSpec         // nil for void methods
    VarBlocks      []*VarBlock
    Body           []Statement
    IsAbstract     bool
    IsFinal        bool
    IsOverride     bool
}

type TypeDecl struct {
    NodeBase
    Name     *Ident
    TypeSpec TypeSpec  // StructType, EnumType, AliasType, SubrangeType, ArrayType
}
```

### Diagnostic Type
```go
// pkg/diag/diagnostic.go
type Severity int
const (
    Error Severity = iota
    Warning
    Info
    Hint
)

type Diagnostic struct {
    Severity Severity       `json:"severity"`
    Pos      source.Pos     `json:"pos"`       // file:line:col
    EndPos   source.Pos     `json:"end_pos"`
    Code     string         `json:"code"`      // e.g., "E001"
    Message  string         `json:"message"`
}

func (d Diagnostic) String() string {
    return fmt.Sprintf("%s:%d:%d: %s: %s",
        d.Pos.File, d.Pos.Line, d.Pos.Col,
        d.Severity, d.Message)
}
```

### CLI Structure
```go
// cmd/stc/main.go
func main() {
    rootCmd := &cobra.Command{
        Use:     "stc",
        Short:   "IEC 61131-3 Structured Text compiler toolchain",
        Version: version.String(), // injected via ldflags
    }

    // Global flags
    rootCmd.PersistentFlags().String("format", "text", "Output format: text, json")

    // Subcommands
    rootCmd.AddCommand(
        newParseCmd(),
        newCheckCmd(),  // stub
        newTestCmd(),   // stub
        newEmitCmd(),   // stub
        newLintCmd(),   // stub
        newFmtCmd(),    // stub
        newPpCmd(),     // stub
    )

    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### stc.toml Config
```toml
# stc.toml - STC project manifest
[project]
name = "my-plc-project"
version = "1.0.0"

[build]
source_roots = ["src/", "lib/"]
vendor_target = "beckhoff"     # beckhoff, schneider, portable

[build.library_paths]
oscat = "vendor/oscat/"

[lint]
naming_convention = "PascalCase"  # for future use
```

```go
// pkg/project/config.go
type Config struct {
    Project ProjectConfig `toml:"project"`
    Build   BuildConfig   `toml:"build"`
    Lint    LintConfig    `toml:"lint"`
}

type ProjectConfig struct {
    Name    string `toml:"name"`
    Version string `toml:"version"`
}

type BuildConfig struct {
    SourceRoots  []string          `toml:"source_roots"`
    VendorTarget string            `toml:"vendor_target"`
    LibraryPaths map[string]string `toml:"library_paths"`
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Parser generator (ANTLR, yacc) | Hand-written recursive descent | ~2015 (Go, Rust, TS compilers all switched) | Full control over error recovery for LSP |
| AST only (lose comments/whitespace) | CST-first (preserve all tokens) | ~2018 (rust-analyzer, roslyn) | Enables formatter and LSP without retrofit |
| `(result, error)` pattern | `(result, []diagnostics)` pattern | ~2018 (rust-analyzer) | Parser never fails -- always returns partial tree |
| Single-file parsing | Workspace-aware parsing | ~2020 (gopls, rust-analyzer) | Cross-file features from day one |

**Deprecated/outdated:**
- `go.lsp.dev/protocol`: Last published 2022, LSP 3.15.3 -- do not use
- ANTLR4 Go target: Works but generated code is verbose and hard to debug; error recovery fights the generator
- `alecthomas/participle`: Cannot produce partial ASTs from broken code -- disqualified by PARS-05

## Open Questions

1. **Exact CST node naming convention**
   - What we know: Need NodeKind enum, concrete types per construct
   - What is unclear: Whether to use `IfStmt` vs `IfStatement` vs `StmtIf` naming
   - Recommendation: Use `IfStmt`, `ForStmt`, `ProgramDecl`, `FunctionBlockDecl` -- matches Go compiler conventions

2. **Pragma parsing granularity**
   - What we know: Beckhoff uses `{attribute 'qualified_only'}`, preprocessor uses `{IF defined(...)}`
   - What is unclear: Whether to parse pragma content or treat as opaque string in Phase 1
   - Recommendation: Lex `{ ... }` as a Pragma token with raw text. In Phase 1, parser attaches pragma nodes to declarations as opaque strings. Phase 2 (preprocessor) will parse the content.

3. **How much of the `stc.toml` to implement in Phase 1**
   - What we know: CLI-04 requires "source roots and vendor target"
   - What is unclear: Whether to parse the full config or just the minimum
   - Recommendation: Parse the full config struct but only use `source_roots` and `vendor_target` in Phase 1. Other fields are read and stored but not acted on.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | All compilation | Yes | 1.26.0 | -- |
| make | Build system | Yes | 3.81 | -- |
| git | Version control, CI | Yes | 2.39.5 | -- |
| gh | PR creation | Yes | 2.86.0 | -- |
| golangci-lint | CI linting | No | -- | Install via `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` or use CI action |

**Missing dependencies with no fallback:**
- None

**Missing dependencies with fallback:**
- golangci-lint: Not installed locally but easily installable. CI uses the `golangci/golangci-lint-action` GitHub Action, which installs it automatically. Local install: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing stdlib + testify v1.10.x |
| Config file | None needed -- Go convention |
| Quick run command | `go test ./... -count=1` |
| Full suite command | `go test ./... -v -count=1 -race -coverprofile=coverage.out` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PARS-01 | Parse PROGRAM, FB, FUNCTION, TYPE declarations | unit | `go test ./pkg/parser/ -run TestDeclarations -v` | Wave 0 |
| PARS-02 | Parse CODESYS OOP (METHOD, INTERFACE, PROPERTY, EXTENDS, IMPLEMENTS) | unit | `go test ./pkg/parser/ -run TestOOP -v` | Wave 0 |
| PARS-03 | Parse POINTER TO, REFERENCE TO | unit | `go test ./pkg/parser/ -run TestPointerRef -v` | Wave 0 |
| PARS-04 | Parse 64-bit types (LINT, LREAL, LWORD, ULINT) | unit | `go test ./pkg/lexer/ -run TestKeywords64 -v` | Wave 0 |
| PARS-05 | Partial ASTs from broken code with error nodes | unit | `go test ./pkg/parser/ -run TestErrorRecovery -v` | Wave 0 |
| PARS-06 | JSON AST output via CLI | integration | `go test ./cmd/stc/ -run TestParseJSON -v` | Wave 0 |
| PARS-07 | All control structures with operator precedence | unit | `go test ./pkg/parser/ -run TestControlFlow -v && go test ./pkg/parser/ -run TestExprPrecedence -v` | Wave 0 |
| PARS-08 | All VAR sections | unit | `go test ./pkg/parser/ -run TestVarSections -v` | Wave 0 |
| PARS-09 | Arrays, structs, enums, subranges, typed literals | unit | `go test ./pkg/parser/ -run TestTypes -v && go test ./pkg/lexer/ -run TestLiterals -v` | Wave 0 |
| PARS-10 | {attribute '...'} pragmas | unit | `go test ./pkg/parser/ -run TestPragma -v` | Wave 0 |
| CLI-01 | Single binary with subcommands | integration | `go test ./cmd/stc/ -run TestSubcommands -v` | Wave 0 |
| CLI-02 | --format json on every subcommand | integration | `go test ./cmd/stc/ -run TestFormatJSON -v` | Wave 0 |
| CLI-03 | stc --version | integration | `go test ./cmd/stc/ -run TestVersion -v` | Wave 0 |
| CLI-04 | stc.toml manifest parsing | unit | `go test ./pkg/project/ -run TestConfig -v` | Wave 0 |
| CLI-05 | Diagnostics with file:line:col | unit | `go test ./pkg/diag/ -run TestDiagFormat -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./... -count=1`
- **Per wave merge:** `go test ./... -v -count=1 -race -coverprofile=coverage.out`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `pkg/lexer/lexer_test.go` -- tokenization tests for all token kinds
- [ ] `pkg/parser/parser_test.go` -- golden file test infrastructure
- [ ] `pkg/parser/testdata/*.st` -- ST fixture files for all constructs
- [ ] `pkg/ast/json_test.go` -- JSON marshaling round-trip tests
- [ ] `pkg/diag/diagnostic_test.go` -- diagnostic formatting tests
- [ ] `pkg/project/config_test.go` -- stc.toml parsing tests
- [ ] `cmd/stc/main_test.go` -- CLI integration tests

## Sources

### Primary (HIGH confidence)
- [Eli Bendersky: Ungrammar in Go and resilient parsing](https://eli.thegreenplace.net/2023/ungrammar-in-go-and-resilient-parsing/) -- CST and error recovery patterns in Go
- [Thunderseethe: Resilient recursive descent parsing](https://thunderseethe.dev/posts/parser-base/) -- Error recovery techniques
- [matklad: Simple but Powerful Pratt Parsing](https://matklad.github.io/2020/04/13/simple-but-powerful-pratt-parsing.html) -- Pratt parser implementation reference
- [Fernhill IEC 61131-3 Expressions](https://www.fernhillsoftware.com/help/iec-61131/common-elements/expressions.html) -- Operator precedence table
- [Go compiler parser internals](https://internals-for-interns.com/posts/the-go-parser/) -- Go's own recursive descent parser architecture
- [Cobra CLI framework](https://github.com/spf13/cobra) -- v1.10.x, standard Go CLI library
- [BurntSushi/toml](https://github.com/BurntSushi/toml) -- v1.5.0, TOML v1.0 compliant

### Secondary (MEDIUM confidence)
- [A Practical Guide to Building a Parser in Go (Jan 2026)](https://gagor.pro/2026/01/a-practical-guide-to-building-a-parser-in-go/) -- Recent Go parser tutorial
- [CODESYS OOP documentation](https://content.helpme-codesys.com/en/CODESYS%20Development%20System/_cds_implementing_interface.html) -- INTERFACE/IMPLEMENTS syntax
- [tree-sitter-structured-text](https://github.com/tmatijevich/tree-sitter-structured-text) -- Grammar reference for token types

### Tertiary (LOW confidence)
- Cobra v2.3.0 mentioned in web results -- could not verify on pkg.go.dev; using v1.10.x which is confirmed stable

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- Go stdlib + Cobra + testify are battle-tested, versions verified against installed Go
- Architecture: HIGH -- CST-first with Pratt parsing is proven by rust-analyzer, Go compiler, roslyn
- Pitfalls: HIGH -- Confirmed across MATIEC, STruCpp, and IEC 61131-3 grammar research
- Keyword/operator table: MEDIUM -- Compiled from Fernhill docs and IEC references; may miss edge cases in vendor extensions

**Research date:** 2026-03-26
**Valid until:** 2026-04-26 (stable domain, no fast-moving dependencies)
