# Architecture: Vendor Libraries, I/O Mapping, and Mock Framework

**Domain:** IEC 61131-3 Structured Text Compiler Toolchain -- v1.1 Integration
**Researched:** 2026-03-30
**Confidence:** HIGH (all recommendations verified against existing codebase)

## Recommended Architecture

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| `pkg/iomap` (NEW) | I/O address parsing, IOTable data structure | Interpreter (read/write I/O), Checker (AT validation) |
| `pkg/vendor` (NEW) | Load .st stubs, extract from TcPOU XML | Parser (parse stubs), Checker (register symbols), Project config |
| `pkg/interp` (MODIFY) | Auto-stub generation for body-less FBs, IOTable integration | IOMap (resolve AT vars), Vendor (stub declarations) |
| `pkg/checker` (MODIFY) | Accept library declarations before user code | Vendor loader (stub declarations), Symbols table |
| `pkg/project` (MODIFY) | Add `[test.mock_paths]` config | Test runner, Vendor loader |

### Data Flow

**Type-checking flow (stc check):**
```
stc.toml [build.library_paths]
    |
    v
pkg/vendor/loader.go: parse .st stub files
    |
    v
pkg/checker/resolve.go: register stub declarations in pass 1 (before user code)
    |
    v
pkg/checker/check.go: type-check user code (vendor FBs now resolve)
```

**Test execution flow (stc test):**
```
stc.toml [build.library_paths] + [test.mock_paths]
    |
    v
1. Parse stub files (declarations only)
2. Parse mock files (declarations + bodies) -- overrides stubs
3. Parse user source files
4. Parse test files
    |
    v
pkg/checker: resolve all symbols (mocks replace stubs in symbol table)
    |
    v
pkg/interp: create FBInstance for each FB instantiation
    |-- Has body (user code or mock)? --> NewUserFBInstance (existing)
    |-- No body (stub only)?          --> NewAutoStubFBInstance (NEW: zero-value outputs)
    |
    v
pkg/interp/scan.go: ScanCycleEngine with IOTable
    |-- AT variables resolved to IOTable offsets
    |-- Inputs copied from IOTable.I at cycle start
    |-- Outputs copied to IOTable.Q at cycle end
```

## Patterns to Follow

### Pattern 1: AT Address Resolution

**What:** Parse AT address strings into structured data for IOTable access.

**Data structure:**

```go
// pkg/iomap/address.go

type Area byte
const (
    AreaInput  Area = 'I'
    AreaOutput Area = 'Q'
    AreaMemory Area = 'M'
)

type Size byte
const (
    SizeBit   Size = 'X'  // 1 bit
    SizeByte  Size = 'B'  // 8 bits
    SizeWord  Size = 'W'  // 16 bits
    SizeDWord Size = 'D'  // 32 bits
)

type IOAddress struct {
    Area       Area
    Size       Size
    ByteOffset int
    BitOffset  int   // 0-7, only meaningful for SizeBit
    IsWildcard bool  // true for %I*, %Q*, %M*
}

// ParseAddress parses "%IX0.0", "%QW4", "%MD12", "%I*" etc.
func ParseAddress(s string) (IOAddress, error)
```

**When:** Every AT address in a VarDecl needs parsing during checker pass 1 and interpreter initialization.

### Pattern 2: Stub Loading Priority

**What:** Multi-layer symbol resolution with explicit override semantics.

**Priority (highest to lowest):**
1. User-written mock FBs from `[test.mock_paths]` (test mode only)
2. User source FBs (declarations with bodies)
3. Built-in stc standard library FBs (TON, TOF, CTU, etc.)
4. Vendor stub FBs from `[build.library_paths]` (declarations without bodies)

**Implementation:**

```go
// In checker/resolve.go, modify CollectDeclarations:

func (r *Resolver) CollectDeclarations(files []*ast.SourceFile, opts ResolveOpts) {
    // 1. Register vendor stub declarations (lowest priority)
    for _, stubFile := range opts.LibraryFiles {
        r.collectFile(stubFile, SymbolSourceLibrary)
    }

    // 2. Register user source declarations
    for _, file := range files {
        r.collectFile(file, SymbolSourceUser)
    }

    // 3. Register mock declarations (highest priority, test mode only)
    for _, mockFile := range opts.MockFiles {
        r.collectFile(mockFile, SymbolSourceMock) // allows override
    }
}
```

**Key rule:** Mock files can override library-sourced declarations without triggering the "redeclaration" error. This requires tracking the symbol source.

### Pattern 3: IOTable Integration with ScanCycleEngine

**What:** Bind AT-declared variables to IOTable offsets during env initialization.

```go
// In interp/scan.go, extend initializeEnv:

func (e *ScanCycleEngine) initializeEnv() {
    // ... existing code ...

    for _, vb := range e.program.VarBlocks {
        for _, vd := range vb.Declarations {
            if vd.AtAddress != nil {
                addr, err := iomap.ParseAddress(vd.AtAddress.Name)
                if err != nil {
                    continue // or report error
                }
                // Register this variable as I/O-mapped
                e.ioBindings = append(e.ioBindings, IOBinding{
                    VarName: vd.Names[0].Name,
                    Address: addr,
                })
            }
        }
    }
}

// In Tick(), add I/O copy steps:
func (e *ScanCycleEngine) Tick(dt time.Duration) error {
    // 0. Copy I/O table inputs into env (NEW)
    for _, b := range e.ioBindings {
        if b.Address.Area == iomap.AreaInput {
            e.env.Set(b.VarName, e.ioTable.Read(b.Address))
        }
    }

    // 1-4. Existing scan cycle steps...

    // 5. Copy env outputs to I/O table (NEW)
    for _, b := range e.ioBindings {
        if b.Address.Area == iomap.AreaOutput {
            if v, ok := e.env.Get(b.VarName); ok {
                e.ioTable.Write(b.Address, v)
            }
        }
    }
}
```

### Pattern 4: Auto-Stub FB Instance

**What:** When interpreter encounters a body-less FB type, create a "do nothing" instance.

```go
// In interp/mock.go

type AutoStubFB struct {
    typeName string
    inputs   map[string]Value  // zero-valued
    outputs  map[string]Value  // zero-valued
}

func (fb *AutoStubFB) Execute(dt time.Duration) {} // no-op
func (fb *AutoStubFB) SetInput(name string, v Value) { fb.inputs[name] = v }
func (fb *AutoStubFB) GetOutput(name string) Value   { return fb.outputs[name] }
func (fb *AutoStubFB) GetInput(name string) Value     { return fb.inputs[name] }
```

This satisfies the existing `StandardFB` interface with zero behavior. All outputs remain at their zero values unless the user provides a mock with real logic.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Terminal-Aware I/O Model
**What:** Building data structures for specific EtherCAT terminal types (EL1008 channel model, etc.)
**Why bad:** Creates unbounded maintenance burden; every new terminal needs a model; not stc's responsibility
**Instead:** Model the flat I/O image. Let AT addresses resolve to byte offsets. The terminal type determines what addresses exist, but that is the System Manager's job.

### Anti-Pattern 2: Binary Library Parsing
**What:** Reverse-engineering .library or .compiled-library file formats
**Why bad:** Undocumented, proprietary, changes between CODESYS versions, legally questionable
**Instead:** Use .TcPOU XML extraction (simple, documented) or hand-written .st stubs

### Anti-Pattern 3: Go-Based Mock Registration
**What:** Requiring Go code to register mock FB implementations
**Why bad:** Forces PLC engineers to write Go; breaks the "one language" value proposition
**Instead:** ST-based mocks that the parser/interpreter handle natively. Mock FBs are just FUNCTION_BLOCKs with bodies placed in a mock directory.

### Anti-Pattern 4: Validating Addresses Against Hardware Config
**What:** Checking that `%IX0.0` actually corresponds to a configured terminal
**Why bad:** stc has no hardware configuration file; addresses are user-declared
**Instead:** Validate the address FORMAT (`%IX0.0` is valid, `%ZZ99` is not). Do not validate address existence.

## Scalability Considerations

| Concern | 10 stubs | 100 stubs | 1000 stubs |
|---------|----------|-----------|------------|
| Parse time | <10ms | <100ms | ~500ms (may need caching) |
| Symbol table size | Negligible | Negligible | ~2MB for 1000 FB types |
| LSP responsiveness | No impact | No impact | May need lazy loading |
| Mock override resolution | O(n) acceptable | O(n) acceptable | Consider hash-based lookup |

For v1.1, the expected stub count is ~50-100. No scaling concerns.

## Sources

- Existing stc codebase: `pkg/ast/var.go`, `pkg/interp/scan.go`, `pkg/checker/vendor.go`
- [Beckhoff InfoSys: AT-Declaration](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/11948825611.html)
- [Beckhoff InfoSys: Addresses](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/2529360523.html)
- [VENDOR_LIBRARIES.md](../../../docs/VENDOR_LIBRARIES.md)
