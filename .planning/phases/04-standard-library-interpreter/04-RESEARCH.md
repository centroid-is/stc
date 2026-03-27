# Phase 4: Standard Library & Interpreter - Research

**Researched:** 2026-03-27
**Domain:** IEC 61131-3 tree-walking interpreter + standard library FBs/functions in Go
**Confidence:** HIGH

## Summary

Phase 4 builds two tightly coupled systems: (1) a tree-walking AST interpreter in Go that executes typed ASTs with PLC scan-cycle semantics, and (2) the full IEC 61131-3 standard library (timers, counters, edge detection, bistable, math, string, type conversion functions). The interpreter receives a typed AST + symbol table from the Phase 3 analyzer and evaluates it inside a Tick(dt) loop that reads inputs, executes program body, writes outputs, and advances a deterministic virtual clock. Standard library FBs are Go structs that implement a common interface and accept injected time deltas for deterministic testing.

The existing codebase provides a solid foundation: the AST node hierarchy (pkg/ast/) covers all ST control structures including OOP, the type system (pkg/types/) has full widening/lattice rules with built-in function signatures already declared, and the symbol table (pkg/symbols/) provides hierarchical scope lookup. The interpreter needs to bridge from these typed ASTs to runtime values and execution. The biggest implementation challenge is correct timer semantics (TON/TOF/TP have subtle edge cases around re-triggering, elapsed time capping, and reset behavior) and FB instance lifecycle management (persistent state across scan cycles, per-instance scope).

**Primary recommendation:** Build the interpreter core (value system + expression/statement evaluation) first, then layer on FB instance management and scan cycle, then implement stdlib FBs as Go structs behind a common interface, then connect math/string/conversion functions. This bottom-up approach lets each layer be tested independently.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Tree-walking AST interpreter in Go -- simplest approach, matches "interpreter only" project decision
- Explicit `Tick(dt)` scan cycle: read inputs -> execute program body -> write outputs -> advance time
- Each FB instance gets its own scope with persistent state across scan cycles (standard PLC behavior)
- Go `time.Duration` for all time values -- deterministic, injected via `Tick(dt)`, no wall-clock dependency
- All standard library FBs and functions implemented in Go -- full control, deterministic, easier to test
- Microsecond-level timer precision (match PLC behavior, compare accumulated vs preset)
- Fixed-length strings (STRING[80] default per IEC) backed by Go strings with length validation at boundaries
- Full IEC standard library coverage per REQUIREMENTS.md (timers, counters, edge detection, bistable, math, string, type conversion)
- All FBs accept injected time for deterministic testing

### Claude's Discretion
- Internal interpreter value representation
- Standard function parameter naming
- Test fixture organization
- Error message formatting for runtime errors

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| STLB-01 | Timers implemented with correct IEC semantics (TON, TOF, TP) | Timer FB specs from IEC 61131-3 via Fernhill reference; Go time.Duration for accumulation; Tick(dt) injection |
| STLB-02 | Counters implemented with correct IEC semantics (CTU, CTD, CTUD) | Counter specs from IEC standard; rising-edge detection on CU/CD inputs; reset priority rules |
| STLB-03 | Edge detection implemented (R_TRIG, F_TRIG) | Simple prev-state comparison FBs; single BOOL output CLK/Q |
| STLB-04 | Bistable FBs implemented (SR, RS) | Set/Reset dominant bistable logic per IEC |
| STLB-05 | Standard math functions (ABS, SQRT, MIN, MAX, SEL, MUX, LIMIT, etc.) | Go math stdlib covers all; type-generic via runtime dispatch on Value types |
| STLB-06 | Standard string functions (LEN, LEFT, RIGHT, MID, CONCAT, FIND, etc.) | Go string operations; 1-based indexing per IEC; length validation at STRING[N] boundaries |
| STLB-07 | Standard type conversion functions (INT_TO_REAL, BOOL_TO_INT, etc.) | Runtime value coercion; banker's rounding for REAL->INT per IEC spec |
| STLB-08 | All standard library FBs support deterministic time injection | Tick(dt) passed to all FBs; no time.Now() anywhere in interpreter |
| INTP-01 | Interpreter executes typed AST with scan-cycle semantics | Tick(dt) loop: ReadInputs -> Eval body -> WriteOutputs -> advance clock |
| INTP-02 | Interpreter supports deterministic time advancement | Virtual clock incremented by dt on each Tick; time.Duration throughout |
| INTP-03 | Interpreter can set/get inputs and outputs programmatically | IOTable maps variable names to Values; SetInput/GetOutput API |
| INTP-04 | Interpreter handles all ST control structures, expressions, and FB instance calls | Type-switch eval for all ast.Statement and ast.Expr variants |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `math` | 1.22+ | Math functions (Sqrt, Abs, Sin, Cos, etc.) | Direct mapping to IEC math functions |
| Go stdlib `time` | 1.22+ | time.Duration for TIME values | Microsecond precision, deterministic arithmetic |
| Go stdlib `strings` | 1.22+ | String manipulation backing IEC string functions | UTF-8 safe, handles all IEC string operations |
| Go stdlib `strconv` | 1.22+ | Number<->string conversions for type conversion functions | Standard Go numeric formatting |
| Go stdlib `math/big` | 1.22+ | Potential for precise integer overflow detection | Only if overflow semantics need exact control |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify` | 1.11.1 | Test assertions (already in go.mod) | All unit tests for interpreter and stdlib |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Tagged union Value type | Interface-based Value | Tagged union is faster (no heap alloc for primitives), more explicit; interface is more Go-idiomatic but slower |
| map[string]Value for FB state | struct per FB type | Map is flexible but slower; typed structs are faster but require per-FB code -- use typed structs since FB set is fixed |

**Installation:**
```bash
# No new dependencies needed -- all Go stdlib + existing testify
```

## Architecture Patterns

### Recommended Project Structure
```
pkg/
  interp/
    value.go          # Value type (tagged union for runtime values)
    env.go            # Environment/scope for variable storage
    interpreter.go    # Core interpreter (eval expressions, exec statements)
    fb_instance.go    # FB instance management (persistent state, scope)
    scan.go           # ScanCycleEngine: Tick(dt), IOTable, virtual clock
    stdlib_timers.go  # TON, TOF, TP implementations
    stdlib_counters.go # CTU, CTD, CTUD implementations
    stdlib_edge.go    # R_TRIG, F_TRIG implementations
    stdlib_bistable.go # SR, RS implementations
    stdlib_math.go    # ABS, SQRT, MIN, MAX, SEL, MUX, LIMIT, EXPT, trig
    stdlib_string.go  # LEN, LEFT, RIGHT, MID, CONCAT, FIND, INSERT, DELETE, REPLACE
    stdlib_convert.go # *_TO_* type conversion functions
    errors.go         # Runtime error types
    # Test files alongside each source file
    value_test.go
    interpreter_test.go
    scan_test.go
    stdlib_timers_test.go
    stdlib_counters_test.go
    # etc.
  interp/testdata/    # ST source fixtures for integration tests
```

### Pattern 1: Tagged Union Value Type
**What:** A single `Value` struct with a `Kind` tag and fields for each possible value type. Avoids interface dispatch overhead for the hot path (expression evaluation).
**When to use:** All runtime value representation in the interpreter.
**Example:**
```go
// pkg/interp/value.go
type ValueKind int

const (
    ValBool ValueKind = iota
    ValInt            // int64 backing for all integer types
    ValReal           // float64 backing for REAL/LREAL
    ValString
    ValTime           // time.Duration
    ValDate
    ValDateTime
    ValTod
    ValArray
    ValStruct
    ValFBInstance     // Reference to a function block instance
)

type Value struct {
    Kind    ValueKind
    Bool    bool
    Int     int64
    Real    float64
    Str     string
    Time    time.Duration
    Array   []Value
    Struct  map[string]Value  // Field name -> value (case-insensitive keys)
    FBRef   *FBInstance       // For FB instance references
    IECType types.TypeKind    // Tracks the precise IEC type for conversions
}

// Zero returns the zero value for a given IEC type kind
func Zero(kind types.TypeKind) Value { ... }
```

### Pattern 2: Environment Chain for Variable Scoping
**What:** An `Env` struct holding variable bindings with a parent pointer for scope chain lookup. Mirrors the symbol table's hierarchical scoping but holds runtime values instead of type information.
**When to use:** Variable storage during interpretation.
**Example:**
```go
// pkg/interp/env.go
type Env struct {
    parent  *Env
    vars    map[string]Value  // UPPERCASE keys for case-insensitive lookup
}

func NewEnv(parent *Env) *Env { ... }
func (e *Env) Get(name string) (Value, bool) { ... }  // walks parent chain
func (e *Env) Set(name string, v Value) bool { ... }   // sets in declaring scope
func (e *Env) Define(name string, v Value) { ... }     // defines in current scope
```

### Pattern 3: StandardFB Interface
**What:** All standard library FBs implement a common interface for the interpreter to call. Each FB is a Go struct with persistent state.
**When to use:** All standard library FBs (timers, counters, edge detectors, bistables).
**Example:**
```go
// pkg/interp/fb_instance.go
type StandardFB interface {
    // Execute runs one scan cycle of the FB with the given time delta.
    Execute(dt time.Duration)
    // GetInput returns the current value of an input by name.
    GetInput(name string) Value
    // SetInput sets the value of an input by name.
    SetInput(name string, v Value)
    // GetOutput returns the current value of an output by name.
    GetOutput(name string) Value
}

// FBInstance wraps a StandardFB with its scope and type info.
type FBInstance struct {
    TypeName string
    FB       StandardFB     // nil for user-defined FBs
    Env      *Env           // Variable scope (for user-defined FBs)
    Decl     *ast.FunctionBlockDecl  // AST for user-defined FBs
}
```

### Pattern 4: Tick(dt) Scan Cycle Engine
**What:** The top-level execution loop that implements PLC scan semantics. External code calls Tick(dt) to advance one scan cycle.
**When to use:** All program execution.
**Example:**
```go
// pkg/interp/scan.go
type ScanCycleEngine struct {
    interp   *Interpreter
    inputs   map[string]Value  // External -> program inputs
    outputs  map[string]Value  // Program outputs -> external
    clock    time.Duration     // Accumulated virtual time
    program  *ast.ProgramDecl  // The program AST
    env      *Env              // Program-level environment
}

func (e *ScanCycleEngine) Tick(dt time.Duration) error {
    // 1. Copy inputs into program environment
    e.readInputs()
    // 2. Execute program body
    err := e.interp.execStatements(e.env, e.program.Body)
    // 3. Copy outputs from program environment
    e.writeOutputs()
    // 4. Advance virtual clock
    e.clock += dt
    return err
}

func (e *ScanCycleEngine) SetInput(name string, v Value)  { ... }
func (e *ScanCycleEngine) GetOutput(name string) Value     { ... }
func (e *ScanCycleEngine) Clock() time.Duration            { ... }
```

### Pattern 5: Expression Evaluation via Type Switch
**What:** The interpreter evaluates expressions by type-switching on AST node types. Each case returns a `Value`.
**When to use:** All expression evaluation.
**Example:**
```go
func (i *Interpreter) evalExpr(env *Env, expr ast.Expr) (Value, error) {
    switch e := expr.(type) {
    case *ast.Literal:
        return i.evalLiteral(e)
    case *ast.Ident:
        return i.evalIdent(env, e)
    case *ast.BinaryExpr:
        return i.evalBinary(env, e)
    case *ast.UnaryExpr:
        return i.evalUnary(env, e)
    case *ast.CallExpr:
        return i.evalCall(env, e)
    case *ast.MemberAccessExpr:
        return i.evalMemberAccess(env, e)
    case *ast.IndexExpr:
        return i.evalIndex(env, e)
    case *ast.DerefExpr:
        return i.evalDeref(env, e)
    case *ast.ParenExpr:
        return i.evalExpr(env, e.Inner)
    case *ast.ErrorNode:
        return Value{}, &RuntimeError{Msg: "cannot evaluate error node"}
    default:
        return Value{}, &RuntimeError{Msg: fmt.Sprintf("unsupported expression type: %T", expr)}
    }
}
```

### Anti-Patterns to Avoid
- **Wall-clock time anywhere:** Never use `time.Now()` or `time.Sleep()`. All time flows through `Tick(dt)`.
- **Shared mutable state between FB instances:** Each FB instance MUST have its own state. Two TON instances must be independent.
- **Ignoring IEC 1-based string indexing:** IEC strings use 1-based positions. MID(s, 3, 2) means "3 chars starting at position 2" -- not Go's 0-based indexing.
- **Floating-point equality for timer comparisons:** Use `>=` not `==` when comparing elapsed time to preset (accumulated durations may overshoot).
- **Forgetting to cap ET at PT:** TON.ET must not exceed TON.PT. Once ET >= PT, ET stays at PT.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Math functions (SQRT, SIN, COS, etc.) | Custom implementations | Go `math` stdlib | Proven, fast, IEEE 754 compliant |
| Duration arithmetic | Custom time tracking | Go `time.Duration` | Nanosecond precision, overflow-safe arithmetic |
| String manipulation | Custom string ops | Go `strings` package | Handles Unicode, efficient, well-tested |
| Number formatting | Custom formatters | Go `strconv` package | Handles all edge cases in numeric conversion |
| Banker's rounding | Custom rounding | `math.RoundToEven()` | Exact IEC 61131-3 REAL_TO_INT rounding behavior |

**Key insight:** The interpreter value system and FB lifecycle are the custom parts. Everything else delegates to Go stdlib.

## IEC 61131-3 Standard Library Specifications

### Timers (STLB-01)

**TON (On-Delay Timer)**
- Inputs: IN (BOOL), PT (TIME)
- Outputs: Q (BOOL), ET (TIME)
- Behavior: When IN rises, ET starts accumulating. When ET >= PT, Q becomes TRUE. When IN falls, ET resets to 0, Q becomes FALSE.
- Edge cases: If IN pulses shorter than PT, Q never fires. ET is capped at PT (never exceeds it). Re-triggering (IN falls then rises) restarts from ET=0.

**TOF (Off-Delay Timer)**
- Inputs: IN (BOOL), PT (TIME)
- Outputs: Q (BOOL), ET (TIME)
- Behavior: When IN rises, Q immediately becomes TRUE, ET=0. When IN falls, ET starts accumulating. When ET >= PT, Q becomes FALSE. If IN rises again before PT, ET resets.
- Edge cases: Rapid IN toggling faster than PT keeps Q TRUE.

**TP (Pulse Timer)**
- Inputs: IN (BOOL), PT (TIME)
- Outputs: Q (BOOL), ET (TIME)
- Behavior: Rising edge on IN starts pulse: Q=TRUE for exactly PT duration. While pulse is active, changes to IN are ignored. After PT, Q returns to FALSE.
- Edge cases: Re-trigger during active pulse is ignored. ET resets only after pulse completes AND IN is FALSE.

### Counters (STLB-02)

**CTU (Count Up)**
- Inputs: CU (BOOL R_EDGE), R (BOOL), PV (INT)
- Outputs: Q (BOOL), CV (INT)
- Behavior: Rising edge on CU increments CV. When CV >= PV, Q = TRUE. R resets CV to 0.
- Reset takes priority over count.

**CTD (Count Down)**
- Inputs: CD (BOOL R_EDGE), LD (BOOL), PV (INT)
- Outputs: Q (BOOL), CV (INT)
- Behavior: Rising edge on CD decrements CV. When CV <= 0, Q = TRUE. LD loads PV into CV.
- Load takes priority over count.

**CTUD (Count Up/Down)**
- Inputs: CU (BOOL R_EDGE), CD (BOOL R_EDGE), R (BOOL), LD (BOOL), PV (INT)
- Outputs: QU (BOOL), QD (BOOL), CV (INT)
- Behavior: R resets to 0; LD loads PV; CU increments; CD decrements. QU = (CV >= PV), QD = (CV <= 0).
- Priority: R > LD > CU/CD.

### Edge Detection (STLB-03)

**R_TRIG (Rising Edge)**
- Input: CLK (BOOL)
- Output: Q (BOOL)
- Behavior: Q = TRUE for one scan when CLK transitions FALSE->TRUE. Requires internal prev_state.

**F_TRIG (Falling Edge)**
- Input: CLK (BOOL)
- Output: Q (BOOL)
- Behavior: Q = TRUE for one scan when CLK transitions TRUE->FALSE. Requires internal prev_state.

### Bistable (STLB-04)

**SR (Set Dominant)**
- Inputs: S1 (BOOL), R (BOOL)
- Output: Q1 (BOOL)
- Behavior: Q1 = S1 OR (NOT R AND Q1). Set dominates.

**RS (Reset Dominant)**
- Inputs: S (BOOL), R1 (BOOL)
- Output: Q1 (BOOL)
- Behavior: Q1 = NOT R1 AND (S OR Q1). Reset dominates.

### Math Functions (STLB-05)
All operate on ANY_NUM unless noted:
- **ABS(IN)** -> same type. Go: `math.Abs` for reals, manual for ints.
- **SQRT(IN)** -> ANY_REAL. Go: `math.Sqrt`.
- **SIN/COS/TAN/ASIN/ACOS/ATAN(IN)** -> ANY_REAL. Go: `math.Sin`, etc.
- **LN(IN)** -> ANY_REAL. Go: `math.Log`.
- **LOG(IN)** -> ANY_REAL. Go: `math.Log10`.
- **EXP(IN)** -> ANY_REAL. Go: `math.Exp`.
- **EXPT(IN, N)** -> ANY_REAL. Go: `math.Pow`.
- **MIN(IN1, IN2)** -> ANY_NUM. Compare and return smaller.
- **MAX(IN1, IN2)** -> ANY_NUM. Compare and return larger.
- **LIMIT(MN, IN, MX)** -> ANY_NUM. `MAX(MN, MIN(IN, MX))`.
- **SEL(G, IN0, IN1)** -> ANY. If G then IN1 else IN0.
- **MUX(K, IN0, IN1, ...)** -> ANY. Select INK by index.
- **MOVE(IN)** -> ANY. Identity (used for forcing evaluation order).

### String Functions (STLB-06)
All use 1-based indexing per IEC:
- **LEN(IN)** -> INT. String length.
- **LEFT(IN, L)** -> STRING. First L characters.
- **RIGHT(IN, L)** -> STRING. Last L characters.
- **MID(IN, L, P)** -> STRING. L characters starting at position P (1-based).
- **CONCAT(IN1, IN2)** -> STRING. Concatenation.
- **FIND(IN1, IN2)** -> INT. Position of IN2 in IN1 (0 if not found).
- **INSERT(IN1, IN2, P)** -> STRING. Insert IN2 into IN1 at position P.
- **DELETE(IN, L, P)** -> STRING. Delete L characters at position P.
- **REPLACE(IN1, IN2, L, P)** -> STRING. Replace L chars at P with IN2.

Edge cases: L=0 returns empty; P beyond length returns input unchanged; result truncated to STRING max length at boundary.

### Type Conversion Functions (STLB-07)
The full matrix of *_TO_* functions. Key rules:
- **REAL_TO_INT**: Banker's rounding (round half to even). Go: `math.RoundToEven`.
- **INT_TO_REAL**: Exact for values that fit in float64 mantissa.
- **BOOL_TO_INT**: FALSE=0, TRUE=1.
- **INT_TO_BOOL**: 0=FALSE, nonzero=TRUE.
- **INT_TO_STRING/STRING_TO_INT**: Decimal representation.
- **Overflow**: Truncate to target type range (modular for unsigned, saturate or wrap per vendor).

Generate all valid combinations from the type matrix: {BOOL, BYTE, WORD, DWORD, LWORD, SINT, INT, DINT, LINT, USINT, UINT, UDINT, ULINT, REAL, LREAL, STRING, TIME} x same set. Not all combinations are valid; implement the standard subset plus commonly used extensions.

## Common Pitfalls

### Pitfall 1: FB Instance State Not Persisting Across Scans
**What goes wrong:** FB instances lose their internal state (timer elapsed time, counter values, edge detector prev_state) between scan cycles.
**Why it happens:** Creating new FB instances each scan or failing to maintain per-instance environments.
**How to avoid:** FB instances are created once when the POU scope is initialized, stored in the Env, and reused on every Tick. The interpreter must distinguish between "initialize variables" (once) and "execute body" (every scan).
**Warning signs:** Timers never fire because ET resets every scan. Counters always read 0.

### Pitfall 2: Timer ET Exceeding PT
**What goes wrong:** TON.ET continues accumulating past PT, causing incorrect behavior in downstream logic that reads ET.
**Why it happens:** Forgetting to cap: `ET = min(ET + dt, PT)`.
**How to avoid:** Always cap ET at PT in timer Execute(). After ET >= PT, stop accumulating.
**Warning signs:** ET values larger than PT in test assertions.

### Pitfall 3: IEC 1-Based String Indexing
**What goes wrong:** MID("HELLO", 2, 1) returns "EL" instead of "HE". FIND returns 0-based position.
**Why it happens:** Go uses 0-based indexing; IEC uses 1-based.
**How to avoid:** All string functions convert P parameter with `goIdx = iecPos - 1` before delegating to Go strings. FIND returns `goIdx + 1` (or 0 for not-found).
**Warning signs:** Off-by-one errors in string tests.

### Pitfall 4: Missing RETURN Value Handling in Functions
**What goes wrong:** FUNCTION declarations that assign to the function name (the IEC way of returning values) silently return zero.
**Why it happens:** IEC functions return their value by assigning to a variable with the same name as the function. The interpreter must create this variable in the function's scope and read it after execution.
**How to avoid:** When executing a FunctionDecl, create a variable named after the function in the local scope. After body execution, read that variable as the return value.
**Warning signs:** All user-defined functions return 0/empty.

### Pitfall 5: Forgetting Rising Edge Detection on Counter Inputs
**What goes wrong:** CTU increments on every scan while CU is TRUE, not just on the rising edge.
**Why it happens:** CU is defined as `BOOL R_EDGE` -- the counter must internally detect the rising edge, not just read the current value.
**How to avoid:** Counter FBs store prev_CU/prev_CD state. Increment only when current=TRUE AND prev=FALSE.
**Warning signs:** Counter overflows immediately when input is held TRUE.

### Pitfall 6: CallStmt vs CallExpr FB Invocation
**What goes wrong:** FB instance calls via CallStmt (named parameter style `myTimer(IN := x, PT := T#5s)`) work, but FB calls via expression position don't, or vice versa.
**Why it happens:** The AST has two call forms: `CallStmt` (with named `CallArg` including IsOutput) and `CallExpr` (positional args in expression context). FB calls primarily use CallStmt. Function calls use CallExpr.
**How to avoid:** Handle both call forms. CallStmt maps named args to FB inputs/outputs. CallExpr maps positional args to function parameters. The interpreter must dispatch correctly based on whether the callee is a FB instance or a function.
**Warning signs:** FB calls produce "not callable" errors. Function calls lose argument binding.

## Code Examples

### Timer Implementation (TON)
```go
// pkg/interp/stdlib_timers.go
type TON struct {
    // Inputs
    IN bool
    PT time.Duration
    // Outputs
    Q  bool
    ET time.Duration
    // Internal
    running bool
}

func (t *TON) Execute(dt time.Duration) {
    if !t.IN {
        // Input FALSE: reset
        t.Q = false
        t.ET = 0
        t.running = false
        return
    }
    // Input TRUE
    if !t.running {
        // Rising edge: start timing
        t.running = true
        t.ET = 0
    }
    // Accumulate time
    t.ET += dt
    if t.ET >= t.PT {
        t.ET = t.PT // Cap at PT
        t.Q = true
    }
}

func (t *TON) SetInput(name string, v Value) {
    switch strings.ToUpper(name) {
    case "IN":
        t.IN = v.Bool
    case "PT":
        t.PT = v.Time
    }
}

func (t *TON) GetOutput(name string) Value {
    switch strings.ToUpper(name) {
    case "Q":
        return Value{Kind: ValBool, Bool: t.Q, IECType: types.KindBOOL}
    case "ET":
        return Value{Kind: ValTime, Time: t.ET, IECType: types.KindTIME}
    }
    return Value{}
}
```

### Counter Implementation (CTU)
```go
type CTU struct {
    CU     bool
    R      bool
    PV     int64
    Q      bool
    CV     int64
    prevCU bool // for rising edge detection
}

func (c *CTU) Execute(dt time.Duration) {
    if c.R {
        c.CV = 0
        c.Q = false
    } else if c.CU && !c.prevCU { // Rising edge on CU
        if c.CV < c.PV {
            c.CV++
        }
    }
    c.Q = c.CV >= c.PV
    c.prevCU = c.CU
}
```

### Scan Cycle Integration Test Pattern
```go
func TestTONFiresAfterPreset(t *testing.T) {
    engine := NewScanCycleEngine(program, env)
    engine.SetInput("StartBtn", BoolValue(true))

    // Run 5 scans of 100ms each = 500ms
    for i := 0; i < 5; i++ {
        engine.Tick(100 * time.Millisecond)
    }

    // TON with PT=T#500ms should have fired
    q := engine.GetOutput("MotorRunning")
    assert.True(t, q.Bool, "motor should be running after 500ms")
}
```

### String Function Example (MID with 1-based indexing)
```go
func iecMID(in string, length int, pos int) string {
    // IEC: MID(IN, L, P) - L chars starting at 1-based position P
    if pos < 1 || length < 1 || pos > len(in) {
        return ""
    }
    start := pos - 1 // Convert to 0-based
    end := start + length
    if end > len(in) {
        end = len(in)
    }
    return in[start:end]
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Wall-clock timers in test | Deterministic virtual clock | STruC++ ADVANCE_TIME pattern | All timer tests are instant and deterministic |
| Separate FB state management | FB instances as first-class scope objects | Established PLC pattern | Correct FB lifecycle = correct programs |
| C++ transpilation for host exec | Tree-walking interpreter | Project decision | Simpler, debuggable, adequate performance for dev testing |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify 1.11.1 |
| Config file | None needed (Go convention) |
| Quick run command | `go test ./pkg/interp/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STLB-01 | TON/TOF/TP correct timing | unit | `go test ./pkg/interp/ -run TestTON -count=1` | Wave 0 |
| STLB-02 | CTU/CTD/CTUD counting | unit | `go test ./pkg/interp/ -run TestCTU -count=1` | Wave 0 |
| STLB-03 | R_TRIG/F_TRIG detection | unit | `go test ./pkg/interp/ -run TestRTRIG -count=1` | Wave 0 |
| STLB-04 | SR/RS bistable logic | unit | `go test ./pkg/interp/ -run TestSR -count=1` | Wave 0 |
| STLB-05 | Math functions correctness | unit | `go test ./pkg/interp/ -run TestMath -count=1` | Wave 0 |
| STLB-06 | String functions with 1-based idx | unit | `go test ./pkg/interp/ -run TestString -count=1` | Wave 0 |
| STLB-07 | Type conversion with banker's round | unit | `go test ./pkg/interp/ -run TestConvert -count=1` | Wave 0 |
| STLB-08 | Deterministic time injection | integration | `go test ./pkg/interp/ -run TestDeterministic -count=1` | Wave 0 |
| INTP-01 | Scan cycle semantics | integration | `go test ./pkg/interp/ -run TestScanCycle -count=1` | Wave 0 |
| INTP-02 | Deterministic time advancement | integration | `go test ./pkg/interp/ -run TestClock -count=1` | Wave 0 |
| INTP-03 | Programmatic I/O access | integration | `go test ./pkg/interp/ -run TestIOAccess -count=1` | Wave 0 |
| INTP-04 | All control structures | integration | `go test ./pkg/interp/ -run TestControl -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./pkg/interp/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before verification

### Wave 0 Gaps
- [ ] `pkg/interp/` directory -- entire package is new
- [ ] Test infrastructure: table-driven tests following project convention (testdata/ directories)
- [ ] No new framework install needed (Go testing + testify already available)

## Open Questions

1. **Integer overflow semantics**
   - What we know: IEC 61131-3 doesn't specify overflow behavior uniformly; vendors differ (some wrap, some saturate)
   - What's unclear: Whether to match a specific vendor's behavior or use Go's natural wrap-around for integer types
   - Recommendation: Use Go's natural int64 arithmetic (wrap on overflow) with no special handling. Document this. Vendor-specific overflow can be a later enhancement.

2. **EXPT (power) function for integer types**
   - What we know: IEC defines EXPT for ANY_NUM but integer exponentiation can overflow
   - What's unclear: Return type when both operands are integers (INT**INT -> INT or REAL?)
   - Recommendation: EXPT always returns REAL (consistent with math.Pow). If both args are int, convert to float64 first.

3. **STRING max length enforcement**
   - What we know: Decision says STRING[80] default backed by Go strings with length validation at boundaries
   - What's unclear: Exactly which boundaries (assignment? concatenation? function return?)
   - Recommendation: Validate and truncate on assignment to STRING variables. Functions return Go strings of arbitrary length; truncation happens when stored.

## Sources

### Primary (HIGH confidence)
- Fernhill Software IEC 61131-3 reference -- [TON](https://www.fernhillsoftware.com/help/iec-61131/common-elements/standard-function-blocks/on-delay-timer.html), [TOF](https://www.fernhillsoftware.com/help/iec-61131/common-elements/standard-function-blocks/off-delay-timer.html), [TP](https://www.fernhillsoftware.com/help/iec-61131/common-elements/standard-function-blocks/timer-pulse.html), [CTU](https://www.fernhillsoftware.com/help/iec-61131/common-elements/standard-function-blocks/up-counter.html), [CTD](https://www.fernhillsoftware.com/help/iec-61131/common-elements/standard-function-blocks/down-counter.html), [String functions](https://www.fernhillsoftware.com/help/iec-61131/common-elements/string-functions/index.html), [Type conversions](https://www.fernhillsoftware.com/help/iec-61131/common-elements/conversion-functions/type-casts.html)
- Existing codebase: pkg/ast/, pkg/types/, pkg/symbols/, pkg/checker/, pkg/analyzer/ -- all read and analyzed directly
- Go standard library documentation -- math, time, strings, strconv packages

### Secondary (MEDIUM confidence)
- [CODESYS Standard Library documentation](https://content.helpme-codesys.com/en/libs/Standard/Current/index.html) -- cross-referenced timer/counter behavior
- [ControlByte CODESYS timer guide](https://controlbyte.tech/blog/codesys-timers-ton-tof-tp-guide/) -- timing diagram descriptions verified against Fernhill

### Tertiary (LOW confidence)
- [OpenPLC standard library DeepWiki](https://deepwiki.com/thiagoralves/OpenPLC_v3/7.1-iec-61131-3-standard-library) -- implementation reference, not authoritative for semantics

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all Go stdlib, no external dependencies needed
- Architecture: HIGH -- tree-walking interpreter is well-understood; patterns derived from existing codebase conventions
- IEC FB semantics: HIGH -- cross-verified across Fernhill reference, CODESYS docs, and multiple implementations
- Pitfalls: HIGH -- derived from IEC spec edge cases and prior art research in PITFALLS.md

**Research date:** 2026-03-27
**Valid until:** 2026-04-27 (stable domain, IEC standard doesn't change frequently)
