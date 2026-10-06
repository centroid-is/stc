# Vendor-Specific Function Block Libraries in stc

Research document -- 2026-03-30

## Problem Statement

Production IEC 61131-3 Structured Text code relies heavily on vendor-specific
function blocks that do not exist in the IEC standard. A Beckhoff TwinCAT 3
project may reference hundreds of FBs from Tc2_System, Tc2_MC2, Tc2_EtherCAT,
and Tc3_EventLogger. A Schneider EcoStruxure project uses READ_VAR, WRITE_VAR,
and vendor-specific MC_* variants. Today, stc cannot parse, check, interpret,
or test any code that instantiates these FBs -- they resolve as "undeclared
identifier" (SEMA010) and fail at interpretation with "undefined function."

This document researches the landscape, evaluates approaches, and recommends a
concrete design for stc.

---

## 1. How Other ST Tools Handle This

### RuSTy (PLC-lang/rusty)

RuSTy uses a `plc.json` project file with a `"libraries"` section. Each library
entry has a name, path, package behavior, and include paths. External functions
are declared with an `{external}` pragma:

```
{external} FUNCTION puts : DINT
VAR_INPUT {ref}
    text : STRING;
END_VAR
END_FUNCTION
```

Any C function can be linked through external declarations. The compiler sees
the signature for type-checking but does not require an ST body. This is the
closest analog to what stc needs -- declaration-only stubs.

Source: [RuSTy Libraries Documentation](https://plc-lang.github.io/rusty/libraries.html)

### MATIEC / OpenPLC

MATIEC is an IEC 61131-3 compiler that converts ST to C. It supports the
standard library but has no formal mechanism for vendor-specific extensions.
Users who need vendor FBs must either avoid them or write C stubs that get
linked at compile time. MATIEC's approach is essentially "standard only."

Source: [MATIEC on GitHub](https://github.com/nucleron/matiec)

### CODESYS Library System

CODESYS uses two library file formats:

- `.library` -- source format, contains full ST source, human-readable
- `.compiled-library` -- binary format, source stripped, contains only
  signatures and compiled code; used for distribution via Package Manager

A `.compiled-library` file preserves the FB/FUNCTION/PROGRAM declarations
(names, parameter types, return types) but strips implementation bodies. This
is conceptually identical to what stc needs: type signatures without bodies.

The internal format is proprietary and not publicly documented at the binary
level. Extracting signatures programmatically requires the CODESYS IDE's
scripting interface.

Source: [CODESYS Library Development](https://content.helpme-codesys.com/en/CODESYS%20Development%20System/_cds_library_development_information.html)

### PLCopen XML Exchange Format (IEC 61131-10)

PLCopen XML is an XML-based format for exchanging IEC 61131-3 projects between
different programming environments. It was standardized as IEC 61131-10 in 2019.
The format can represent POUs with full source or as interface-only declarations.
Vendor-specific attributes are allowed but filtered on import.

PLCopen XML could serve as an import source for extracting FB signatures from
vendor projects, but it is not widely used for library distribution -- vendors
prefer their proprietary formats.

Source: [PLCopen XML Exchange](https://www.plcopen.org/standards/xml-echange/)

### TwinCAT Project Files

TwinCAT 3 projects use `.plcproj` (MSBuild XML) files that reference `.TcPOU`
files for each POU. A `.TcPOU` file contains the full ST declaration and body
in XML. These files are parseable and could be used to extract FB signatures
automatically, though this is a secondary extraction path.

Source: [Beckhoff Infosys](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/2526208651.html)

---

## 2. How Other Language Ecosystems Solve This

### TypeScript: `.d.ts` Declaration Files

TypeScript's approach is the gold standard for this pattern. `.d.ts` files
contain only type declarations (no implementation). The `declare` keyword
signals "this exists at runtime but I'm not providing code for it." A massive
community project (DefinitelyTyped) provides type declarations for thousands
of JavaScript libraries.

**Key lessons for stc:**
- Declarations use the same language syntax, just without bodies
- A community repository can scale the effort
- Tooling can auto-generate declarations from existing code

### Rust Embedded: HAL Traits + Mock Crate

The `embedded-hal` crate defines traits (interfaces) for hardware peripherals.
Driver crates program against these traits, not against specific hardware.
The `embedded-hal-mock` crate provides mock implementations for testing:

```rust
// Production: uses real I2C hardware
let sensor = Sensor::new(i2c_peripheral);

// Test: uses mock with expected transactions
let expectations = [I2cTransaction::write(0x48, vec![0x01])];
let mock = I2cMock::new(&expectations);
let sensor = Sensor::new(mock);
```

**Key lessons for stc:**
- Define an interface (trait) that both real and mock implementations satisfy
- Mock implementations record expected calls and verify them
- The same driver code works with real hardware and mocks

### C/C++ Hardware Abstraction Layers

C/C++ embedded projects use HAL layers with conditional compilation:

```c
#ifdef TARGET_STM32
#include "stm32_gpio.h"
#else
#include "mock_gpio.h"
#endif
```

This is directly analogous to stc's existing preprocessor `{IF defined(...)}`.

### MATLAB/Simulink Plant + Controller Co-simulation

Simulink uses "S-Functions" as the boundary between controller logic and plant
models. The controller references an S-Function block with defined I/O ports.
During simulation, a plant model sits behind the S-Function interface. During
deployment, real hardware sits there instead.

**Key lesson for stc:** stc already has `PlantModel` in `pkg/sim/` -- this is
the same pattern. Vendor FBs need the same treatment: an interface at the
boundary, with mock/stub behind it during testing.

---

## 3. Existing stc Infrastructure

### What We Have

| Component | Location | Relevance |
|-----------|----------|-----------|
| Vendor profiles | `pkg/checker/vendor.go` | Language feature gates (OOP, pointers, 64-bit) per vendor |
| Symbol resolution | `pkg/checker/resolve.go` | Two-pass: collect declarations, then type-check bodies |
| Built-in functions | `pkg/types/builtin.go` | `BuiltinFunctions` map of `*FunctionType` with parameter signatures |
| FB interface | `pkg/interp/fb_instance.go` | `StandardFB` interface: Execute, SetInput, GetOutput, GetInput |
| FB factory | `pkg/interp/fb_instance.go` | `StdlibFBFactory` map of constructor functions |
| Function dispatch | `pkg/interp/interpreter.go` | `StdlibFunctions` map + `LocalFunctions` per-instance overrides |
| Project config | `pkg/project/config.go` | `stc.toml` with `[build.library_paths]` section |
| Plant models | `pkg/sim/plant.go` | `PlantModel` interface for closed-loop simulation |
| Scan cycle engine | `pkg/interp/scan.go` | `ScanCycleEngine` with deterministic time |

### Key Observation

stc already has all the infrastructure needed:

1. **Type-checker registration:** `BuiltinFunctions` is a `map[string]*FunctionType`.
   Vendor FB signatures can be registered the same way. The resolver already
   creates `FunctionBlockType` with `Inputs`, `Outputs`, and `InOuts` slices.

2. **Interpreter FB dispatch:** `StdlibFBFactory` maps type names to constructor
   functions returning `StandardFB`. Vendor mock FBs implement the same interface.

3. **Per-test overrides:** `LocalFunctions` on the interpreter allows per-test
   function overrides. The same pattern extends to per-test FB overrides.

4. **Project config:** `stc.toml` already has `[build.library_paths]` with a
   map of library name to path (e.g., `oscat = "vendor/oscat/"`).

---

## 4. Recommended Design

### 4.1 Stub Library Format: `.st` Declaration Files

Use plain ST files with FB/FUNCTION declarations that have **no body**. This is
the simplest approach and requires zero new file formats. The parser already
handles this -- a `FUNCTION_BLOCK` with `VAR_INPUT`/`VAR_OUTPUT` blocks and an
empty body is valid syntax.

**File naming convention:** `*.st` files in a library directory, one file per
library or grouped logically.

**Example: `vendor/beckhoff/tc2_mc2.st`**

```iec
(* Tc2_MC2 -- PLCopen Motion Control function blocks for Beckhoff TwinCAT 3 *)
(* Stub declarations for stc type-checking. No implementation bodies. *)

FUNCTION_BLOCK MC_Power
VAR_INPUT
    Axis        : AXIS_REF;     (* Axis reference -- opaque struct *)
    Enable      : BOOL;
    Enable_Positive : BOOL;
    Enable_Negative : BOOL;
    Override    : LREAL := 100.0;
END_VAR
VAR_OUTPUT
    Status      : BOOL;
    Busy        : BOOL;
    Active      : BOOL;
    Error       : BOOL;
    ErrorID     : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis        : AXIS_REF;
    Execute     : BOOL;
    Position    : LREAL;
    Velocity    : LREAL;
    Acceleration : LREAL;
    Deceleration : LREAL;
    Jerk        : LREAL;
    Direction   : MC_Direction;
END_VAR
VAR_OUTPUT
    Done        : BOOL;
    Busy        : BOOL;
    Active      : BOOL;
    CommandAborted : BOOL;
    Error       : BOOL;
    ErrorID     : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_MoveRelative
VAR_INPUT
    Axis        : AXIS_REF;
    Execute     : BOOL;
    Distance    : LREAL;
    Velocity    : LREAL;
    Acceleration : LREAL;
    Deceleration : LREAL;
    Jerk        : LREAL;
END_VAR
VAR_OUTPUT
    Done        : BOOL;
    Busy        : BOOL;
    Active      : BOOL;
    CommandAborted : BOOL;
    Error       : BOOL;
    ErrorID     : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_Stop
VAR_INPUT
    Axis        : AXIS_REF;
    Execute     : BOOL;
    Deceleration : LREAL;
    Jerk        : LREAL;
END_VAR
VAR_OUTPUT
    Done        : BOOL;
    Busy        : BOOL;
    Active      : BOOL;
    CommandAborted : BOOL;
    Error       : BOOL;
    ErrorID     : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_Home
VAR_INPUT
    Axis        : AXIS_REF;
    Execute     : BOOL;
    Position    : LREAL;
END_VAR
VAR_OUTPUT
    Done        : BOOL;
    Busy        : BOOL;
    Active      : BOOL;
    CommandAborted : BOOL;
    Error       : BOOL;
    ErrorID     : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_ReadActualPosition
VAR_INPUT
    Axis    : AXIS_REF;
    Enable  : BOOL;
END_VAR
VAR_OUTPUT
    Valid   : BOOL;
    Busy    : BOOL;
    Error   : BOOL;
    ErrorID : UDINT;
    Position : LREAL;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK MC_ReadActualVelocity
VAR_INPUT
    Axis    : AXIS_REF;
    Enable  : BOOL;
END_VAR
VAR_OUTPUT
    Valid   : BOOL;
    Busy    : BOOL;
    Error   : BOOL;
    ErrorID : UDINT;
    ActualVelocity : LREAL;
END_VAR
END_FUNCTION_BLOCK

(* Supporting types *)
TYPE AXIS_REF :
STRUCT
    NcToPlc     : DINT;     (* Opaque -- real TwinCAT uses a complex struct *)
    PlcToNc     : DINT;
END_STRUCT
END_TYPE

TYPE MC_Direction : (
    mcPositiveDirection,
    mcNegativeDirection,
    mcCurrentDirection,
    mcShortestWay
);
END_TYPE
```

**Example: `vendor/beckhoff/tc2_system.st`**

```iec
(* Tc2_System -- System function blocks for Beckhoff TwinCAT 3 *)

TYPE T_AmsNetId : STRING(23); END_TYPE
TYPE T_AmsPort : UINT; END_TYPE
TYPE T_MaxString : STRING(255); END_TYPE
TYPE E_OpenPath : (PATH_GENERIC, PATH_BOOTPATH, PATH_BOOTPRJPATH); END_TYPE

FUNCTION_BLOCK ADSREAD
VAR_INPUT
    NETID    : T_AmsNetId;
    PORT     : T_AmsPort;
    IDXGRP   : UDINT;
    IDXOFFS  : UDINT;
    LEN      : UDINT;
    DESTADDR : UDINT;       (* PVOID simplified to UDINT for stc *)
    READ     : BOOL;
    TMOUT    : TIME := T#5S;
END_VAR
VAR_OUTPUT
    BUSY  : BOOL;
    ERR   : BOOL;
    ERRID : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK ADSWRITE
VAR_INPUT
    NETID    : T_AmsNetId;
    PORT     : T_AmsPort;
    IDXGRP   : UDINT;
    IDXOFFS  : UDINT;
    LEN      : UDINT;
    SRCADDR  : UDINT;
    WRITE    : BOOL;
    TMOUT    : TIME := T#5S;
END_VAR
VAR_OUTPUT
    BUSY  : BOOL;
    ERR   : BOOL;
    ERRID : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_FileOpen
VAR_INPUT
    sNetId    : T_AmsNetId;
    sPathName : T_MaxString;
    nMode     : DWORD;
    ePath     : E_OpenPath := PATH_GENERIC;
    bExecute  : BOOL;
    tTimeout  : TIME := T#5S;
END_VAR
VAR_OUTPUT
    bBusy  : BOOL;
    bError : BOOL;
    nErrId : UDINT;
    hFile  : UINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_FileClose
VAR_INPUT
    sNetId   : T_AmsNetId;
    hFile    : UINT;
    bExecute : BOOL;
    tTimeout : TIME := T#5S;
END_VAR
VAR_OUTPUT
    bBusy  : BOOL;
    bError : BOOL;
    nErrId : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_FileRead
VAR_INPUT
    sNetId   : T_AmsNetId;
    hFile    : UINT;
    pReadBuff : UDINT;
    cbReadLen : UDINT;
    bExecute : BOOL;
    tTimeout : TIME := T#5S;
END_VAR
VAR_OUTPUT
    bBusy     : BOOL;
    bError    : BOOL;
    nErrId    : UDINT;
    cbRead    : UDINT;
    bEOF      : BOOL;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_FileWrite
VAR_INPUT
    sNetId    : T_AmsNetId;
    hFile     : UINT;
    pWriteBuff : UDINT;
    cbWriteLen : UDINT;
    bExecute  : BOOL;
    tTimeout  : TIME := T#5S;
END_VAR
VAR_OUTPUT
    bBusy     : BOOL;
    bError    : BOOL;
    nErrId    : UDINT;
    cbWrite   : UDINT;
END_VAR
END_FUNCTION_BLOCK

FUNCTION MEMCPY : UDINT
VAR_INPUT
    destAddr : UDINT;
    srcAddr  : UDINT;
    n        : UDINT;
END_VAR
END_FUNCTION

FUNCTION MEMSET : UDINT
VAR_INPUT
    destAddr : UDINT;
    fillByte : BYTE;
    n        : UDINT;
END_VAR
END_FUNCTION

FUNCTION MEMMOVE : UDINT
VAR_INPUT
    destAddr : UDINT;
    srcAddr  : UDINT;
    n        : UDINT;
END_VAR
END_FUNCTION
```

**Why plain ST and not YAML/TOML/JSON?**

- Engineers already know ST syntax. No new format to learn.
- The stc parser already handles these declarations. No new parser needed.
- Stub files can be copy-pasted from vendor documentation.
- Stub files can be generated from TwinCAT `.TcPOU` files with simple XML extraction.
- The same files work for type-checking, LSP completion, and documentation.

This is directly analogous to TypeScript's `.d.ts` pattern but using the
language's own syntax.

### 4.2 Library Discovery: `stc.toml` Configuration

The existing `stc.toml` already has `[build.library_paths]`. Extend this:

```toml
[project]
name = "my-machine"
version = "1.0.0"

[build]
source_roots = ["src/"]
vendor_target = "beckhoff"

[build.library_paths]
tc2_mc2    = "vendor/beckhoff/tc2_mc2.st"
tc2_system = "vendor/beckhoff/tc2_system.st"
tc2_utilities = "vendor/beckhoff/tc2_utilities.st"
oscat      = "vendor/oscat/"
```

**Resolution rules:**

1. If the path ends in `.st`, treat it as a single stub file.
2. If the path is a directory, parse all `*.st` files in it.
3. Paths are relative to the `stc.toml` location.
4. Library declarations are loaded into the symbol table in Pass 1 (Resolver)
   before user code, so user code can reference them.

**Directory convention:**

```
my-project/
  stc.toml
  src/
    main.st
    motion.st
  vendor/
    beckhoff/
      tc2_mc2.st
      tc2_system.st
      tc2_utilities.st
      tc2_ethercat.st
      tc3_eventlogger.st
    schneider/
      modbus.st
      motion.st
  tests/
    motion_test.st
  mocks/
    mc_mock.st
```

### 4.3 Mock Framework: ST-Based Mocks with Interpreter Registration

Mocks should be written in ST, not Go. This keeps everything in one language
and lets PLC engineers write their own mocks without knowing Go.

**Approach:** When stc loads a stub FB (declaration with no body), it creates
a "pass-through" `FBInstance` that:

1. Accepts all declared inputs via `SetInput`
2. Returns zero-valued outputs via `GetOutput`
3. Does nothing on `Execute`

This is the **default mock** -- it silently does nothing, which is sufficient
for many test scenarios where you just need the code to compile and run.

**Custom mocks** are ST function blocks that the user writes with the same
name, placed in a `mocks/` directory or referenced via the test configuration.
When running tests, stc loads mock implementations in preference to stubs.

**Example: `mocks/mc_mock.st`**

```iec
(* Mock MC_MoveAbsolute that simulates motion completion after Execute *)
FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT
    Axis        : AXIS_REF;
    Execute     : BOOL;
    Position    : LREAL;
    Velocity    : LREAL;
    Acceleration : LREAL;
    Deceleration : LREAL;
    Jerk        : LREAL;
    Direction   : MC_Direction;
END_VAR
VAR_OUTPUT
    Done        : BOOL;
    Busy        : BOOL;
    Active      : BOOL;
    CommandAborted : BOOL;
    Error       : BOOL;
    ErrorID     : UDINT;
END_VAR
VAR
    prevExecute : BOOL;
    cycleCount  : INT;
END_VAR

(* Simple mock: after Execute rising edge, go Busy for 5 cycles, then Done *)
IF Execute AND NOT prevExecute THEN
    Busy := TRUE;
    Active := TRUE;
    Done := FALSE;
    cycleCount := 0;
END_IF;

IF Busy THEN
    cycleCount := cycleCount + 1;
    IF cycleCount >= 5 THEN
        Done := TRUE;
        Busy := FALSE;
        Active := FALSE;
    END_IF;
END_IF;

prevExecute := Execute;
END_FUNCTION_BLOCK
```

**Example test using the mock: `tests/motion_test.st`**

```iec
{test}
TEST_CASE 'MC_MoveAbsolute completes after execute'
VAR
    mover : MC_MoveAbsolute;
END_VAR

(* Trigger move *)
mover(Execute := TRUE, Position := 100.0, Velocity := 500.0);
ASSERT_TRUE(mover.Busy, 'Should be busy after execute');
ASSERT_FALSE(mover.Done, 'Should not be done yet');

(* Run 4 more cycles *)
mover(Execute := TRUE);
mover(Execute := TRUE);
mover(Execute := TRUE);
mover(Execute := TRUE);

ASSERT_TRUE(mover.Done, 'Should be done after 5 cycles');
ASSERT_FALSE(mover.Busy, 'Should no longer be busy');

END_TEST_CASE
```

**Mock loading priority (highest to lowest):**

1. User-written mock FBs in `mocks/` directory (or paths from config)
2. Built-in stc standard library FBs (TON, TOF, CTU, etc.)
3. Auto-generated zero-value stubs from vendor library declarations

This means: if a user writes their own mock for `MC_MoveAbsolute`, it overrides
the auto-generated stub. If they don't write one, the stub silently accepts
all inputs and returns zeros.

### 4.4 Conditional Compilation for Production vs. Test

stc's existing preprocessor supports `{IF defined(X)}`. Use this for code
that must behave differently in production vs. test:

```iec
{IF defined(STC_TEST)}
(* Use simplified motion -- mock FB handles the simulation *)
mover(Execute := startMove, Position := targetPos);
{ELSE}
(* Production: full parameterization *)
mover(
    Axis := GVL.Axis1,
    Execute := startMove,
    Position := targetPos,
    Velocity := GVL.MaxVelocity,
    Acceleration := GVL.MaxAccel,
    Deceleration := GVL.MaxDecel
);
{END_IF}
```

stc should automatically define `STC_TEST` when running `stc test` and
`STC_SIM` when running `stc sim`. This requires no user configuration.

### 4.5 Implementation Plan

The implementation touches four subsystems:

**Step 1: Library loading in the Resolver (checker pass 1)**

Modify `pkg/checker/resolve.go` so that `CollectDeclarations` accepts an
additional list of "library source files" that are parsed and resolved before
user code. These files come from `[build.library_paths]` in `stc.toml`.

The resolver already handles `FunctionBlockDecl` with empty bodies -- it just
registers the name, inputs, and outputs in the symbol table. No change to the
resolver logic itself is needed.

**Step 2: Auto-stub generation in the interpreter**

When the interpreter encounters an FB type name that exists in the symbol table
(from a stub declaration) but has no `StdlibFBFactory` entry and no user-defined
FB declaration with a body, create a `FBInstance` with:

- `Env` populated with zero-valued inputs and outputs from the type's parameter list
- `Decl` set to the stub declaration (empty body)
- `Execute` is a no-op (empty body means no statements to run)

This is the auto-generated zero-value mock. The existing `NewUserFBInstance`
already handles this correctly -- an FB with an empty body will have an env
with variables but `Execute` will run zero statements.

**Step 3: Mock override loading**

Add a `[test.mock_paths]` section to `stc.toml`:

```toml
[test]
mock_paths = ["mocks/"]
```

When running `stc test`, the test runner:

1. Parses stub library files (declarations only)
2. Parses mock files (declarations with bodies)
3. Parses user source files
4. Parses test files

Mock declarations override stub declarations in the symbol table (same name
replaces earlier registration). The resolver already errors on redeclaration,
so this requires a small change: when loading mocks, allow override of
library-sourced declarations.

**Step 4: Ship starter stub libraries**

Ship stub files for the most commonly used vendor libraries in a `stdlib/vendor/`
directory within the stc distribution:

```
stdlib/vendor/beckhoff/tc2_mc2.st
stdlib/vendor/beckhoff/tc2_system.st
stdlib/vendor/beckhoff/tc2_utilities.st
stdlib/vendor/beckhoff/tc2_ethercat.st
stdlib/vendor/beckhoff/tc3_eventlogger.st
stdlib/vendor/schneider/modbus.st
stdlib/vendor/schneider/motion.st
stdlib/vendor/schneider/system.st
```

Users can reference these via stc.toml or copy them into their project's
`vendor/` directory for customization.

---

## 5. Most Commonly Used Vendor FBs

Based on production code patterns and vendor documentation, these are the FBs
that would cover approximately 80% of real-world usage:

### Beckhoff TwinCAT 3 -- Top 20

**Tc2_MC2 (Motion Control -- PLCopen):**

| FB | Purpose |
|----|---------|
| MC_Power | Enable/disable axis |
| MC_MoveAbsolute | Move to absolute position |
| MC_MoveRelative | Move relative distance |
| MC_MoveVelocity | Continuous velocity move |
| MC_Stop | Stop axis motion |
| MC_Home | Home/reference axis |
| MC_Reset | Reset axis error |
| MC_ReadActualPosition | Read current position |
| MC_ReadActualVelocity | Read current velocity |
| MC_ReadStatus | Read axis state machine |

**Tc2_System (System Services):**

| FB/Function | Purpose |
|-------------|---------|
| ADSREAD | Read data via ADS protocol |
| ADSWRITE | Write data via ADS protocol |
| FB_FileOpen | Open file |
| FB_FileClose | Close file |
| FB_FileRead | Read from file |
| FB_FileWrite | Write to file |
| MEMCPY | Memory copy |
| MEMSET | Memory fill |

**Tc2_Utilities:**

| FB/Function | Purpose |
|-------------|---------|
| FB_FormatString | Printf-style string formatting |
| CRC16 | CRC-16 checksum |

**Tc3_EventLogger:**

| FB | Purpose |
|----|---------|
| FB_TcEventLogger | System event logging |
| FB_TcAlarm | Alarm management |

### Schneider EcoStruxure -- Top 10

| FB/Function | Purpose |
|-------------|---------|
| READ_VAR | Modbus read register |
| WRITE_VAR | Modbus write register |
| SEND_REQ | Send communication request |
| RCV_REQ | Receive communication request |
| MC_Power | Enable axis (different params from Beckhoff) |
| MC_MoveAbsolute | Move to position (different params from Beckhoff) |
| MC_Stop | Stop axis |
| GetBit | Extract bit from word |
| SetBit | Set bit in word |
| RTC | Read real-time clock |

---

## 6. Complete User Workflow Example

### Project Setup

```
my-conveyor-project/
  stc.toml
  src/
    conveyor.st          # Production code using MC_MoveAbsolute
  vendor/
    beckhoff/
      tc2_mc2.st         # Stub declarations (from stc stdlib or hand-written)
  mocks/
    mc_mock.st           # Custom mock that simulates motion timing
  tests/
    conveyor_test.st     # Unit tests
```

**stc.toml:**

```toml
[project]
name = "my-conveyor"
version = "1.0.0"

[build]
source_roots = ["src/"]
vendor_target = "beckhoff"

[build.library_paths]
tc2_mc2 = "vendor/beckhoff/tc2_mc2.st"

[test]
mock_paths = ["mocks/"]
```

### Development Commands

```bash
# Type-check production code (vendor stubs provide type info)
stc check src/conveyor.st

# Run tests (mocks override stubs for execution)
stc test tests/

# Simulate with plant model (stc sim already exists)
stc sim tests/conveyor_sim.st

# Emit vendor-specific ST for deployment
stc emit src/conveyor.st --target beckhoff

# LSP provides completion for MC_MoveAbsolute inputs/outputs
# because the stub declarations are in the symbol table
```

### What Happens Under the Hood

1. `stc check`: Parser reads `vendor/beckhoff/tc2_mc2.st`, registers
   `MC_MoveAbsolute` with its inputs/outputs in the symbol table. Parser then
   reads `src/conveyor.st`, which instantiates `MC_MoveAbsolute`. The checker
   resolves the type, validates input parameter names and types. No body
   needed for checking.

2. `stc test`: Same as above, but also reads `mocks/mc_mock.st`. The mock
   `MC_MoveAbsolute` (with body) replaces the stub `MC_MoveAbsolute` (without
   body) in the symbol table. The interpreter creates `FBInstance` objects
   using the mock declaration, which has executable body statements. Tests
   call the FB and assert on outputs.

3. `stc emit`: Reads and checks the code, then emits vendor-flavored ST.
   Vendor FB calls are emitted as-is (they're real vendor FBs on the target).
   The emitter does not need to know about the mock.

---

## 7. Future Extensions

### `stc vendor init` Command

A CLI command that scaffolds vendor library stubs:

```bash
stc vendor init beckhoff tc2_mc2
# Creates vendor/beckhoff/tc2_mc2.st with all FB stubs

stc vendor init schneider modbus
# Creates vendor/schneider/modbus.st with all FB stubs
```

This downloads from a built-in registry of known vendor FB signatures.

### `stc vendor extract` Command

Extract FB signatures from existing TwinCAT projects:

```bash
stc vendor extract path/to/MyProject.plcproj --output vendor/custom/
# Parses .TcPOU XML files, extracts declarations, writes stub .st files
```

### Auto-Mock Generation

For stubs without a user-written mock, generate "recording mocks" that capture
all SetInput calls and return configurable outputs:

```iec
(* Auto-generated mock -- records calls for assertion *)
mover(Execute := TRUE, Position := 100.0);
ASSERT_EQUAL(mover.__mock_call_count, 1, 'Should have been called once');
ASSERT_EQUAL(mover.__mock_last_Position, 100.0, 'Position should be 100');
```

This is a v2 feature -- the zero-value stub + user-written mock approach
covers the immediate need.

### Community Library Repository

Following the DefinitelyTyped model, a Git repository of vendor FB stub
declarations that the community maintains:

```
stc-vendor-stubs/
  beckhoff/
    tc2_mc2.st
    tc2_system.st
    tc2_utilities.st
    tc2_ethercat.st
    tc3_eventlogger.st
    tc3_module.st
  schneider/
    modbus.st
    motion.st
    system.st
    communication.st
  oscat/
    oscat_basic.st
    oscat_network.st
```

Users install via: `stc vendor install beckhoff/tc2_mc2`

---

## 8. Summary of Recommendations

| Decision | Recommendation | Rationale |
|----------|---------------|-----------|
| Stub format | Plain `.st` files with declarations, no body | Uses existing parser, no new format, engineers already know ST |
| Library discovery | `[build.library_paths]` in `stc.toml` | Already exists in config schema, just needs loading logic |
| Mock approach | User-written ST function blocks with full bodies | Same language, full control, no Go knowledge needed |
| Default behavior | Zero-value auto-stubs for unimplemented FBs | Code compiles and runs without mocks; tests can assert on outputs |
| Mock loading | `[test.mock_paths]` in `stc.toml`, overrides stubs | Clear precedence, no magic |
| Shipped stubs | Top-20 Beckhoff + top-10 Schneider FBs | Covers 80% of production code |
| Conditional compilation | Auto-define `STC_TEST` and `STC_SIM` | Zero-config switching between production and test paths |

The implementation requires changes to four files:

1. `pkg/project/config.go` -- add `TestConfig` struct with `MockPaths`
2. `pkg/checker/resolve.go` -- accept library files in `CollectDeclarations`
3. `pkg/interp/scan.go` -- resolve stub FBs as zero-value instances
4. `cmd/stc/test_cmd.go` -- load libraries and mocks before running tests

No new packages, no new file formats, no new parser features. The design
builds entirely on existing infrastructure.

---

## TwinCAT project import

`stc vendor import`, `stc check`, `stc test --project` and `stc sim` accept a
`.tsproj` or `.plcproj` directly. Every `PlaceholderReference` in the plcproj
resolves in this order; the first hit wins:

1. **Project.** POUs, GVLs and DUTs listed in the plcproj itself.
2. **Sibling plcproj.** A `<Library>.plcproj` near the project (for example
   `SVNCoreComponents/SVNCoreComponents/SVNCoreComponents.plcproj`). Its
   sources load as real code, not as stubs.
3. **`[build.library_paths]`.** A directory named after the library in
   `stc.toml`.
4. **Embedded stubs.** Declaration-only `.st` files shipped inside the `stc`
   binary (`stdlib/beckhoff`), loaded with their dependency closure.
5. **Unresolved.** Reported as a VEND020 warning.

Shipped libraries:

| Library | Closure | Notes |
|---------|---------|-------|
| Tc2_Standard | built in | The checker already provides TON, TOF, R_TRIG and friends |
| Tc2_System | common types, Tc2_System | `PVOID` is `POINTER TO BYTE` for now |
| Tc2_Utilities | + Tc2_System | FB_LocalSystemTime, RTC, TIMESTRUCT |
| Tc2_EtherCAT | + Tc2_System, Tc2_Utilities | CoE SDO read/write, slave state FBs |
| Tc2_ModbusSrv | + Tc2_System | Also ships the Modbus TCP client FBs (TF6250) |
| Tc2_SerialCom | + Tc2_System | |
| Tc2_MC2 | + Tc2_System | |
| Tc3_Module | + Tc2_System | Marker stub |
| Tc3_IPCDiag | + Tc2_System | Marker stub |
| Tc3_EventLogger | + Tc2_System | |

`E_EcSlaveState`, `EcDiagParam` and `FB_EcDeviceDiag` are SVNCoreComponents
types, not Beckhoff ones, so they come from the sibling project and are not in
the Tc2_EtherCAT stub.

## Sources

- [RuSTy Libraries Documentation](https://plc-lang.github.io/rusty/libraries.html)
- [RuSTy External Functions (Issue #101)](https://github.com/PLC-lang/rusty/issues/101)
- [MATIEC on GitHub](https://github.com/nucleron/matiec)
- [CODESYS Library Development](https://content.helpme-codesys.com/en/CODESYS%20Development%20System/_cds_library_development_information.html)
- [PLCopen XML Exchange](https://www.plcopen.org/standards/xml-echange/)
- [Beckhoff Tc2_MC2 Documentation](https://infosys.beckhoff.com/content/1033/tcplclib_tc2_mc2/index.html)
- [Beckhoff Tc2_System Documentation](https://download.beckhoff.com/download/document/automation/twincat3/TwinCAT_3_PLC_Lib_Tc2_System_EN.pdf)
- [Beckhoff TwinCAT PLCopen Import/Export](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/2526208651.html)
- [Rust embedded-hal](https://github.com/rust-embedded/embedded-hal)
- [embedded-hal-mock](https://github.com/dbrgn/embedded-hal-mock)
- [TypeScript Declaration Files](https://www.typescriptlang.org/docs/handbook/declaration-files/introduction.html)
- [DefinitelyTyped](https://github.com/DefinitelyTyped/DefinitelyTyped)
