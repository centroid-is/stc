# Architecture

Technical architecture of the stc compiler toolchain for developers.

## System Overview

```
                         stc.toml
                            |
                            v
  .st source ----> [Preprocessor] ----> [Lexer] ----> [Parser] ----> AST
                   (pkg/preprocess)    (pkg/lexer)   (pkg/parser)   (pkg/ast)
                                                                      |
                   +--------------------------------------------------+
                   |                    |                    |
                   v                    v                    v
              [Checker]            [Formatter]          [Emitter]
           (pkg/checker)         (pkg/format)          (pkg/emit)
                   |
          +--------+--------+
          |                 |
          v                 v
     [Interpreter]     [Analyzer]
    (pkg/interp)     (pkg/analyzer)
          |                 |
     +----+----+            v
     |         |       [Incremental]
     v         v      (pkg/incremental)
  [Testing] [Sim]
(pkg/testing)(pkg/sim)
```

### Data Flow

1. **Source text** enters the preprocessor, which evaluates `{IF}` / `{DEFINE}` directives and produces preprocessed text with a source map for position remapping.

2. The **lexer** tokenizes the preprocessed text into a stream of tokens, capturing trivia (whitespace, comments) for CST fidelity.

3. The **parser** consumes tokens via recursive descent with Pratt expression parsing. It produces a concrete syntax tree (`ast.SourceFile`) with error recovery -- broken code yields partial ASTs with `ErrorNode` markers plus diagnostics.

4. The **checker** runs two passes over the AST:
   - **Pass 1 (Resolver)**: Collects all declarations (PROGRAMs, FUNCTION_BLOCKs, FUNCTIONs, TYPEs) into the symbol table. Library stub files are loaded before user code.
   - **Pass 2 (Type Checker)**: Walks expression and statement bodies, resolves types, checks assignments, validates FB parameter usage, and emits vendor-aware warnings.

5. The **interpreter** executes the AST directly with PLC scan-cycle semantics: read inputs, execute body, write outputs. It maintains an environment (`Env`) per scope with variable storage, and `FBInstance` objects for function block state.

6. The **testing** package discovers `*_test.st` files, parses `TEST_CASE` declarations, and executes them through the interpreter with assertion collection.

7. The **sim** package runs closed-loop simulations with waveform generators driving inputs and optional plant models providing feedback.

8. The **emitter** walks the AST and produces vendor-flavored ST text (Beckhoff, Schneider, or portable).

9. The **formatter** walks the AST and re-emits ST with normalized style (indentation, keyword casing, spacing), preserving comments attached to nodes.

## Package Dependency Graph

Arrows indicate "imports" direction (A --> B means A imports B).

```
cmd/stc -------> pkg/analyzer, pkg/ast, pkg/diag, pkg/ecat, pkg/emit, pkg/format,
                 pkg/incremental, pkg/lint, pkg/lsp, pkg/parser, pkg/pipeline,
                 pkg/preprocess, pkg/project, pkg/sim, pkg/testing, pkg/vendor,
                 pkg/version

cmd/stc-mcp ---> pkg/analyzer, pkg/ast, pkg/diag, pkg/emit, pkg/format,
                 pkg/lint, pkg/parser, pkg/pipeline, pkg/testing

pkg/analyzer --> pkg/ast, pkg/checker, pkg/diag, pkg/project
pkg/checker ---> pkg/ast, pkg/diag, pkg/symbols, pkg/types
pkg/ecat ------> pkg/ast, pkg/diag, pkg/iomap, pkg/source, pkg/types
pkg/emit ------> pkg/ast
pkg/format ----> pkg/ast
pkg/incremental> pkg/ast, pkg/diag, pkg/parser, pkg/pipeline, pkg/source
pkg/interp ----> pkg/ast, pkg/ecat, pkg/iomap, pkg/types
pkg/iomap -----> pkg/ast
pkg/lexer -----> pkg/ast, pkg/source
pkg/lint ------> pkg/ast, pkg/diag
pkg/lsp -------> pkg/analyzer, pkg/ast, pkg/checker, pkg/diag, pkg/format,
                 pkg/parser, pkg/pipeline, pkg/symbols
pkg/parser ----> pkg/ast, pkg/lexer, pkg/source
pkg/pipeline --> pkg/ast, pkg/diag, pkg/parser, pkg/preprocess
pkg/preprocess > pkg/diag, pkg/source
pkg/sim -------> pkg/ast, pkg/interp, pkg/pipeline
pkg/symbols ---> pkg/ast, pkg/types
pkg/testing ---> pkg/ast, pkg/interp, pkg/iomap, pkg/pipeline
pkg/vendor ----> pkg/ast, pkg/parser, pkg/project
```

Foundation packages with no internal dependencies: `pkg/source`, `pkg/diag`, `pkg/types`, `pkg/version`.

## Key Interfaces and Types

### Node (pkg/ast)

Every AST node implements the `Node` interface:

```go
type Node interface {
    Kind() NodeKind       // Discriminator (KindProgramDecl, KindIfStmt, etc.)
    Pos() Pos             // Source position (file, line, column)
    Children() []Node     // Child nodes for tree traversal
}
```

Nodes are categorized into declarations (`ProgramDecl`, `FunctionBlockDecl`, `FunctionDecl`, `InterfaceDecl`, `MethodDecl`, `PropertyDecl`, `TypeDecl`, `ActionDecl`, `TestCaseDecl`), statements (`AssignStmt`, `CallStmt`, `IfStmt`, `CaseStmt`, `ForStmt`, `WhileStmt`, `RepeatStmt`, `ReturnStmt`, `ExitStmt`, `ContinueStmt`), and expressions (`BinaryExpr`, `UnaryExpr`, `CallExpr`, `MemberAccessExpr`, `IndexExpr`, `LiteralExpr`, `IdentExpr`).

The `SourceFile` is the root node containing a slice of `Declaration` nodes.

### Value (pkg/interp)

Runtime values in the interpreter:

```go
type Value struct {
    Kind     ValueKind   // Bool, Int, Real, String, Time, Array, Struct, Enum, ...
    Bool     bool
    Int      int64
    Real     float64
    Str      string
    Dur      time.Duration
    Elements []Value           // For arrays
    Fields   map[string]Value  // For structs
}
```

### StandardFB (pkg/interp)

Interface for built-in function blocks (timers, counters, edge detectors, bistables):

```go
type StandardFB interface {
    Execute(dt time.Duration)
    SetInput(name string, v Value)
    GetOutput(name string) Value
    GetInput(name string) Value
}
```

`StdlibFBFactory` is a `map[string]func() StandardFB` that maps type names to constructors.

### FBInstance (pkg/interp)

Wraps either a `StandardFB` (for stdlib FBs) or an `Env` + `Decl` pair (for user-defined FBs):

```go
type FBInstance struct {
    TypeName   string
    FB         StandardFB              // Non-nil for stdlib FBs
    Env        *Env                    // Non-nil for user-defined FBs
    Decl       *ast.FunctionBlockDecl  // AST declaration
    ParentDecl *ast.FunctionBlockDecl  // For EXTENDS chain
}
```

### PlantModel (pkg/sim)

Interface for simulated physical systems:

```go
type PlantModel interface {
    Update(inputs map[string]interp.Value, dt time.Duration) map[string]interp.Value
}
```

Built-in models: `MotorModel`, `ValveModel`, `CylinderModel`.

### EtherCAT topology and link binding (pkg/ecat)

`pkg/ecat` lets a program run against the I/O image a real TwinCAT EtherCAT master would present, wired through the same `TcLinkTo` pragmas the project uses on the target.

- **Loader** (`topology.go`, `tree.go`). `LoadProject` reads TwinCAT EtherCATConfig exports (`Device N.xml`), one `Master` per file. Nesting and link paths are a port of `generate_gvl.py`: a box's role comes from its port media (`Info/Physics`), terminals nest under the EK coupler or CX head they are daisy-chained to, and `LinkPath` builds `TIID^master^coupler^box^pdo^entry`.
- **Layout** (`image.go`). Every PDO entry and pseudo-input gets a `Slot` (master, direction, byte, bit, bit length). Offsets from the export's `<ProcessImage>` are preferred; without one the layout is computed from the PDO order. `Topology.Slot(path)` and `Paths()` look slots up by link path, and `Images` holds one input and one output byte array per master.
- **Links** (`link.go`, `collect.go`, `resolve.go`). `ParseTcLinkTo` parses single and multi-member values. `CollectLinks` walks GVLs and PROGRAMs on the AST, following struct and FB members, and emits one `LinkedVar` per linked leaf. `Resolve` binds each to its slot and reports ECAT001 to ECAT007 (see `stc ecat validate` in CLI_REFERENCE.md).
- **Network** (`network.go`). `Network` owns the images, writes healthy Beckhoff pseudo-inputs every step (slave `State` = OP, `WcState` clear, `SlaveCount`, `AmsNetId`, `AdsAddr`), and runs one `Device` per slave from a `Registry` keyed by vendor and product id. `Passthrough` is the default device. A fault API (`SetSlaveState`, `SetWcState`, `SetDevState`, `ClearFaults`) overrides the pseudo-inputs.

The interpreter side is `interp.IOBinder`, attached with `ScanCycleEngine.SetIOBinder`. Each `Tick` calls it at two points:

1. After AT-address inputs are synced from the I/O table, `preScan` steps the network (pseudo-inputs and device models) and copies every input binding from the image into its variable.
2. After AT-address outputs are written back, `postScan` copies every output binding from its variable into the image.

The codec decodes by declared type and slot width and never reads past a slot. Bindings that cannot be decoded are dropped and reported through `IOBinder.Errors()`. A nil binder leaves the scan cycle unchanged.

Device models for specific terminals and drives arrive in Phases 25 and 26 through the `Registry`. Scenario scripting on top of the fault API arrives in Phase 27.

### Project runtime (pkg/interp)

`LoadProject(ProjectSpec)` builds one `Runtime` (one interpreter, all
files and libraries instantiated once) and binds every task of the spec to
the engines of its PROGRAMs. A spec comes from `cmd/stc`'s loader: a
`.tsproj`/`.plcproj` gives its tasks (cycle, priority, programs); `.st`
files get one 10 ms MAIN task.

- **Scheduling.** The base tick is the GCD of the task cycles. `Tick`
  advances the virtual clock by one base tick and runs every due task in
  priority order (lower number first, then task name), under the
  Runtime mutex, so it is serialised with `Get`/`Set` (OPC UA, `--get`).
  `Advance(d)` runs whole base ticks.
- **I/O.** `AT %I*/%Q*` wildcards get slots in a flat process image after
  the explicit addresses; `SetIOBinder` attaches an EtherCAT network. Values
  are decoded and encoded by declared type.
- **Free-running.** `Run(ctx, RunOpts)` ticks once per base tick of a
  monotonic `WallClock`, sleeping (never busy-waiting) until each tick is
  due. A late tick counts one overrun per task due in the missed window and
  realigns instead of bursting catch-up ticks. `OnTick` runs on the scan
  goroutine after each tick; `stc serve` applies queued OPC UA writes and
  periodic `--persist` saves there.
- **State file.** `SaveState`/`LoadState` persist PERSISTENT and RETAIN
  variables as `{"version": 1, "values": {"<GVL>.<path>": <json>}}` with
  sorted keys. Unknown or mistyped entries are warnings; another version
  is an error and nothing is applied.

### Config (pkg/project)

Project configuration loaded from `stc.toml`:

```go
type Config struct {
    Project ProjectConfig  // name, version
    Build   BuildConfig    // source_roots, vendor_target, library_paths
    Lint    LintConfig     // naming_convention
    Test    TestConfig     // mock_paths
}
```

## How to Add a New ST Language Feature

1. **Lexer** (`pkg/lexer/`): If the feature introduces new keywords or token types, add them to the keyword table and token type constants.

2. **AST** (`pkg/ast/`): Define new node types in the appropriate file (`decl.go` for declarations, `stmt.go` for statements, `expr.go` for expressions). Implement `Kind()`, `Pos()`, `Children()`, and the marker method (`declNode()`, `stmtNode()`, or `exprNode()`). Add JSON marshaling support in `json.go`.

3. **Parser** (`pkg/parser/`): Add parsing logic in the recursive descent parser. Follow the existing pattern of `parseXxx` methods. Add error recovery if the construct can appear in a position where recovery is needed.

4. **Checker** (`pkg/checker/`): Add type-checking logic in the appropriate pass. Pass 1 (`resolve.go`) for declarations, Pass 2 for expressions and statements.

5. **Interpreter** (`pkg/interp/`): Add evaluation logic in `interpreter.go` for expressions or statement execution.

6. **Emitter** (`pkg/emit/`): Add emission logic to reproduce the construct in vendor-flavored ST.

7. **Formatter** (`pkg/format/`): Add formatting logic to produce consistently styled output.

8. **Tests**: Add unit tests at each level (parser, checker, interpreter) and integration tests in `tests/`.

## How to Add a New CLI Command

1. Create a new file in `cmd/stc/` (e.g., `mycommand.go`).

2. Define a `newMyCmd() *cobra.Command` function following the existing pattern:
   - Set `Use`, `Short`, `Long` descriptions
   - Add command-specific flags
   - Implement `RunE` handler
   - Support `--format json` via the persistent `format` flag

3. Register the command in `cmd/stc/main.go` by adding `newMyCmd()` to the `rootCmd.AddCommand(...)` call.

4. Add tests in `cmd/stc/mycommand_test.go`.

5. If the command should be available via MCP, add a tool handler in `cmd/stc-mcp/tools.go`.

## How to Add a New Stdlib Function Block

1. Create a new file `pkg/interp/stdlib_myblock.go`.

2. Define a struct implementing `StandardFB`:
   ```go
   type MyBlock struct {
       // Internal state
   }
   func (b *MyBlock) Execute(dt time.Duration) { /* ... */ }
   func (b *MyBlock) SetInput(name string, v Value) { /* ... */ }
   func (b *MyBlock) GetOutput(name string) Value { /* ... */ }
   func (b *MyBlock) GetInput(name string) Value { /* ... */ }
   ```

3. Register the constructor in `StdlibFBFactory` via an `init()` function:
   ```go
   func init() {
       StdlibFBFactory["MY_BLOCK"] = func() StandardFB { return &MyBlock{} }
   }
   ```

4. Add the type signature in `pkg/types/builtin.go` so the checker knows the FB's inputs and outputs.

5. Add tests in `pkg/interp/stdlib_myblock_test.go`.

## How to Add Vendor Stubs

1. Create a `.st` file with `FUNCTION_BLOCK` declarations that have `VAR_INPUT`, `VAR_OUTPUT`, `VAR_IN_OUT` blocks but no body statements. Place it in `stdlib/vendor/<vendor>/`.

2. Supporting types (structs, enums) go in the same file or a separate `common_types.st`.

3. Add a test in `stdlib/vendor/<vendor>/stubs_test.go` that parses the stub file and verifies it produces no parse errors.

4. Users reference stubs via `[build.library_paths]` in their `stc.toml`.

## How to Add an EtherCAT Device Model

`pkg/ecat` simulates the EtherCAT side of a TwinCAT project from its EtherCATConfig exports. Every slave gets a device model that reads its outputs and writes its inputs in the process image each cycle.

**Contract.** A model implements `ecat.Device`:

```go
type Device interface {
    Init(s *Slave)                                  // once, with the slave from the export
    Step(dt time.Duration, out []byte, in []byte)   // every cycle
}
```

`out` is the master-to-slave view and `in` is the slave-to-master view, both limited to the byte span of the slave's own PDO entries. Models that also implement `ecat.Binder` receive their `*ecat.Layout` after `Init`. The Layout resolves entries by PDO and entry name exactly as they appear in the export, for example `Layout.Field("Channel 1", "Status")`. `FindEntry` and `Fields` give per-direction lookups when names vary between revisions. Embed `devices.Base` to get `Init`, `Bind` and the generic `Set(pdo, entry, v)` and `Get(pdo, entry)` stimulus.

**Registration.** `ecat.Registry` maps `(VendorId, ProductCode)` to a factory. An exact identity always wins. When none matches, vendor-scoped model-name patterns registered with `RegisterModel` are tried in order, which covers other revisions and synthetic fixtures. Add new products to `knownIDs` in `pkg/ecat/devices/ids.go` with a product constant taken from a real export, plus a fallback regex if useful. Couplers and terminals without process data use the `passive` factory.

**Unmatched slaves.** A slave with no model runs as `ecat.Passthrough`, which never touches the image. `NewNetwork` reports it as an `ECAT010` warning through `Network.Diagnostics()`, with the slave name, vendor and product, so the missing registration is easy to add.

**Package layout.** `pkg/ecat/devices` keeps one file per family: `base.go`, `passive.go`, `digital.go`, `analog.go`, `el9222.go`, `psu.go`, `safety.go`, `el6001.go` and `serial_peer.go`. Each family has a matching `_test.go` built on fixtures in `tests/ecat_fixtures`. The package registers itself into `ecat.DefaultRegistry` from `init`, so consumers that pass a nil registry must blank-import it:

```go
import _ "github.com/centroid-is/stc/pkg/ecat/devices"
```

**Shipped models and stimulus.** Tests and scenarios reach a model with `Network.Device(master, slave)` or `Network.DeviceByName(name)` and type-assert it.

| Model | Products | Stimulus and inspection |
|-------|----------|-------------------------|
| `Passive` | EK1100, EK1110, EK1200, EL6070, EL9011, CU2508 | `Base.Set`, `Base.Get` |
| `DigitalIO` | EL1008, EL1018, EL2008, EP2338, Festo CTEU | `SetInput(ch, v)`, `Output(ch)` |
| `Analog` | EL3054 (4-20 mA), EL3064 (0-10 V) | `SetCurrent(ch, mA)`, `SetVoltage(ch, v)`, `SetRaw(ch, raw)` |
| `EL9222` | EL9222-5500 | `Trip`, `SetWarning`, `SetCoolDown`, `SetHardwareProtection`, `SetLoadCurrent`, `Enabled`, `Tripped` |
| `PSU` | PS2001-2410 | `SetPSU(state)`, `State()` |
| `SafetyDiag` | EL1904, EL2912, EP1918-0002 | `SetFieldVoltage(under, over)` |
| `EL6001` | EL6001, EL6002 | `SetPeer(p)` with `ScriptedPeer`, `LoopbackPeer` or `BaaderPeer()`, `SetErrors` |

Network-level faults that apply to any slave use `SetSlaveState`, `SetWcState`, `SetDevState` and `ClearFaults`.

**Real-data gate.** `tests/ecat_models_test.go` loads every local sildarvinnsla export when `STC_SILD_DIR` is set and fails on any unmodelled slave except the ATV320 drives, which Phase 26 models.

## Testing Strategy

### Layers

- **Unit tests**: Each `pkg/` package has `*_test.go` files testing individual functions and types. Run with `go test ./...`.

- **Integration tests**: `cmd/stc/*_test.go` tests the CLI commands end-to-end by invoking the binary with test fixtures in `cmd/stc/testdata/`.

- **ST test suites**: `tests/` contains ST test files exercising the full pipeline (parse, check, interpret, assert). Run with `stc test tests/` or `go run ./cmd/stc test tests/`.

- **Corpus tests**: `tests/corpus/` contains real-world ST files for parse-only validation. `tests/corpus_test.go` verifies they parse without panics.

- **Adversarial tests**: `tests/adversarial/` and `pkg/interp/adversarial_test.go` test edge cases and malformed inputs.

### Coverage

Coverage thresholds are enforced via `.testcoverage.yml`:
- Overall: 85%
- Critical packages (parser, lexer, checker, interp, types, emit): 94-95%

CI runs coverage checks on every PR via the `coverage.yml` workflow.

### CI Workflows

| Workflow | File | Purpose |
|----------|------|---------|
| CI | `ci.yml` | Build, test, vet on Linux/macOS/Windows; golangci-lint |
| Coverage | `coverage.yml` | Coverage thresholds with Codecov upload |
| Release | `release.yml` | Cross-platform binaries on GitHub release |
| ST Tests | `st-tests.yml` | Run ST test suites on all platforms |
