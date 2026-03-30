# Domain Pitfalls

**Domain:** Vendor Library Stubs, I/O Mapping, and Mock Framework for IEC 61131-3 ST Compiler
**Researched:** 2026-03-30
**Confidence:** HIGH for vendor FB differences (verified against Beckhoff Infosys and Schneider product-help docs), MEDIUM for AB limitations (no OOP features confirmed but specifics from training data), MEDIUM for EtherCAT stubbing limits (architectural reasoning, not empirical)

---

## Critical Pitfalls

Mistakes that cause rewrites, silent correctness bugs, or broken user trust.

---

### Pitfall 1: PLCopen "Same Name, Different Signature" Across Vendors

**What goes wrong:**
Users write code against `MC_MoveAbsolute` using Beckhoff stubs, then switch to Schneider stubs. The code that compiled and tested fine now fails type-checking because the FB signatures are fundamentally different despite sharing the same PLCopen name.

Verified differences for `MC_MoveAbsolute`:

| Parameter | Beckhoff TwinCAT (Tc2_MC2) | Schneider EcoStruxure |
|-----------|---------------------------|----------------------|
| Position | **LREAL** | **DINT** |
| Velocity | **LREAL** | **DINT** |
| Acceleration | LREAL (optional, 0 = axis default) | **Not a parameter** (set via separate FB) |
| Deceleration | LREAL (optional, 0 = axis default) | **Not a parameter** (set via separate FB) |
| Jerk | LREAL (optional) | **Not a parameter** |
| BufferMode | MC_BufferMode (vendor extension) | **Not present** |
| Options | ST_MoveOptions (vendor extension) | **Not present** |
| Axis | AXIS_REF (VAR_IN_OUT) | Axis_Ref (VAR_IN_OUT, different struct) |
| ErrorID output | **UDINT** | **WORD** |

This is not a minor difference -- Position is LREAL (64-bit float) on Beckhoff vs DINT (32-bit int) on Schneider. Code that passes `100.5` as Position compiles on Beckhoff and fails on Schneider. Acceleration and deceleration are direct parameters on Beckhoff but handled by separate vendor-specific FBs (`SetDriveRamp_LXM32`) on Schneider.

PLCopen explicitly permits this: "implementations may exchange basic data types like SINT, INT, DINT or LREAL without being non-compliant, as long as they are consistent for the whole set of Function Blocks." This means the *standard itself* encourages vendor divergence.

**Why it happens:**
PLCopen Part 1 defines FB names, input/output names, and behavioral contracts -- but allows vendors to choose concrete data types. Every vendor optimizes for their drive ecosystem: Beckhoff uses LREAL because TwinCAT NC uses floating-point internally; Schneider uses DINT because their drives use integer-unit positions.

**Consequences:**
- User writes portable-looking code that is silently vendor-locked
- Switching vendor stubs breaks compilation with confusing type errors
- Mock FBs written against one vendor's signature are wrong for another
- Tests pass with wrong-vendor mocks, production code fails

**Prevention:**
1. **Namespace stubs by vendor.** Never have two vendors' `MC_MoveAbsolute` in the same symbol table. Stubs live in `vendor/beckhoff/tc2_mc2.st` and `vendor/schneider/motion.st` -- the user picks one via `stc.toml`. The resolver must reject loading stubs from two vendors that define the same FB name.
2. **Emit a clear error on vendor mismatch.** If the user's `vendor_target` is `schneider` but they load `vendor/beckhoff/tc2_mc2.st`, emit a warning: "stub library tc2_mc2 is for Beckhoff, but vendor_target is schneider."
3. **Document the differences.** Each shipped stub file should include a comment header listing vendor-specific deviations from PLCopen.
4. **Do NOT attempt a vendor-abstraction layer.** An `MC_MoveAbsolute_Portable` wrapper that hides differences will always leak. Users who need portability should use conditional compilation (`{IF defined(VENDOR_BECKHOFF)}`).

**Detection:**
- User loads vendor stubs that don't match `vendor_target` in `stc.toml`
- Type errors appear on FB parameters that "should work" per PLCopen standard
- Same test file fails when switching vendor stub libraries

**Phase to address:** Phase 1 (Stub Loading). Vendor-namespaced loading must be designed from the start. Adding it later means retroactively breaking projects that assumed vendor-agnostic stubs.

---

### Pitfall 2: I/O Address Syntax Edge Cases (%I/%Q/%M Addressing)

**What goes wrong:**
The IEC 61131-3 direct addressing syntax `%IX0.0`, `%IW4`, `%QD2`, `%MB100` has more complexity than it appears. Implementations that handle the "happy path" (`%IX0.0` for a bit, `%IW0` for a word) fail on real production code.

**Edge cases that break naive parsers:**

| Syntax | Meaning | Gotcha |
|--------|---------|--------|
| `%I0.0` | Input bit 0.0 | **X is optional for bits.** `%I0.0` = `%IX0.0`. Many parsers only accept `%IX`. |
| `%IW1.2` | Input word, module 1, channel 2 | **Hierarchical addressing.** The `.` is NOT a bit separator here -- it's a path separator. `%IW1.2` is word 2 of module 1, not "bit 2 of word 1." |
| `%QB0` | Output byte 0 | Valid, but **byte 0 overlaps with bits %QX0.0 through %QX0.7 and word %QW0 bits 0-7.** Overlapping I/O addresses are a major source of bugs in production. |
| `%MD100` | Memory double-word 100 | **Memory addresses have no module hierarchy** on most vendors. `%MD1.2` is invalid on Beckhoff but valid on some CODESYS platforms. |
| `%I*` | Incomplete address (AT %I*) | **Wildcard/placeholder.** Used in FB declarations to say "this will be bound to some input address at configuration time." Must parse but cannot be resolved to a concrete address. |
| `%IL0` | Input long-word (64-bit) | **L prefix for LWORD.** Only valid if vendor supports 64-bit types. Portable profile must reject this. |

**Vendor-specific divergences:**

- **Beckhoff TwinCAT:** Addresses map to EtherCAT process image. `%IB0` is byte 0 of the process image, which corresponds to configured PDO mapping. The same physical terminal can have different addresses depending on configuration. TwinCAT also supports `AT %I*` for automatic address assignment.
- **Schneider:** Uses CODESYS-style addressing but with different process image layout. Module.channel addressing (`%IW1.2`) is common.
- **Allen Bradley:** Does **not use direct addressing at all.** AB uses tag-based addressing. There is no `%I` / `%Q` / `%M` syntax. All I/O is accessed through named tags (e.g., `Local:1:I.Data.0`). This is a fundamental incompatibility.

**Overlapping address problem:**
```
VAR
    byteVal AT %QB0 : BYTE;    (* byte at address 0 *)
    bitVal  AT %QX0.3 : BOOL;  (* bit 3 of the SAME byte *)
END_VAR

byteVal := 16#FF;  (* Sets all bits including bit 3 *)
bitVal := FALSE;    (* Clears bit 3 -- but does it? When? *)
```
The behavior depends on scan cycle output coercion order. On real PLCs, the last write wins, but "last" depends on task execution order and output update phase. A mock I/O table must define deterministic precedence rules for overlapping addresses.

**Why it happens:**
The IEC standard defines the syntax but leaves memory layout and overlap behavior to the implementation. Each vendor's process image mapping is different. Developers test with simple `%IX0.0` examples and never encounter hierarchical or overlapping addresses until production.

**Prevention:**
1. **Parse the full address syntax** including optional X prefix, hierarchical dotted paths, wildcard `*`, and L prefix. The lexer currently handles `AT` followed by an identifier -- it needs to properly tokenize `%` addresses.
2. **Build a mock I/O table** that is flat: map each address to a typed memory cell. Detect and warn on overlapping accesses (byte write overlapping with bit access to same region).
3. **Reject %I/%Q/%M for Allen Bradley vendor target.** Emit a vendor-specific error: "Direct addressing (%I/%Q) is not supported by Allen Bradley. Use tag-based addressing."
4. **Support AT %I* (wildcard)** in FB declarations for type-checking without requiring concrete addresses.

**Detection:**
- Parser rejects valid production code containing `%I0.0` (without X)
- No warning when bit and byte addresses overlap
- Hierarchical addresses (`%IW1.2`) parsed as floating-point numbers

**Phase to address:** Phase 2 (I/O Mapping). The parser already handles `AT` keyword -- the I/O table and overlap detection are new.

---

### Pitfall 3: Mock Fidelity Traps -- Mocks That Create False Confidence

**What goes wrong:**
The default mock behavior (zero-value outputs, no-op Execute) creates subtle false confidence in three specific patterns:

**Trap 1: Instant completion mocks**
```iec
(* User's mock: Done := TRUE on first call after Execute *)
mover(Execute := TRUE, Position := 100.0);
(* Mock immediately sets Done := TRUE *)
IF mover.Done THEN
    (* Test proceeds immediately -- but real motion takes seconds *)
    StartNextStep();
END_IF;
```
Test passes. Production code enters `StartNextStep()` while axis is still moving, causing a collision or interlock violation. The mock hides the fact that real MC_MoveAbsolute takes 50-5000ms depending on velocity and distance.

**Trap 2: Error-free mocks**
The zero-value default stub never sets `Error := TRUE` or `ErrorID` to a non-zero value. Tests never exercise error-handling paths. Production code has untested error recovery that fails silently or crashes.

```iec
(* This error handler is never tested because mock never errors *)
IF mover.Error THEN
    CASE mover.ErrorID OF
        16#4001: HandleAxisNotReady();
        16#4003: HandleFollowingError();
    END_CASE;
END_IF;
```

**Trap 3: State machine bypass**
PLCopen motion FBs have a strict state machine: Standstill -> Discrete Motion -> Standstill. Real FBs reject commands that violate the state machine (e.g., MC_MoveAbsolute while axis is in ErrorStop). Zero-value mocks accept any command sequence, so tests pass with illegal state transitions that would fail on real hardware.

**Trap 4: Timer-dependent logic with instant mocks**
```iec
(* Production code waits for ADS response with timeout *)
adsRead(READ := TRUE, TMOUT := T#5S);
IF adsRead.BUSY THEN
    (* Wait... *)
ELSIF adsRead.ERR THEN
    HandleTimeout();
END_IF;
```
With a zero-value mock, `BUSY` is always FALSE and `ERR` is always FALSE. The read "completes" instantly with no data. The timeout path is never tested.

**Why it happens:**
Zero-value mocks are the easiest default and the right choice for getting code to compile and run. But they create a "green dashboard" effect where all tests pass, giving engineers confidence that the code works. The gap between "compiles and runs" and "behaves correctly" is exactly where production bugs live.

Google's testing blog documents this pattern: "if the actual implementation changes, tests that relied on mocks can still pass, giving a false feeling of safety." In PLC context, this is amplified because the consequences of false confidence are physical -- equipment damage, safety incidents, production downtime.

**Prevention:**
1. **Document mock fidelity levels** in shipped stubs. Each stub should have a comment: `(* MOCK FIDELITY: zero-value. Does NOT simulate timing, errors, or state machine. *)` This forces conscious acknowledgment.
2. **Ship "behavioral" mocks** alongside zero-value stubs for the top-5 motion FBs. The behavioral mock in `docs/VENDOR_LIBRARIES.md` (MC_MoveAbsolute with 5-cycle completion) is the right pattern. Ship it, don't just document it.
3. **Add `ASSERT_CALLED` / `ASSERT_NOT_CALLED` intrinsics.** Let tests verify that an FB was actually invoked without needing behavioral fidelity. This catches "dead code" where the FB call is inside an unreachable branch.
4. **Warn on zero-value mock in test output.** When `stc test` runs and an FB has no user mock (using auto-stub), print a warning: `WARN: MC_MoveAbsolute using zero-value stub (no mock). Error and timing paths untested.`
5. **Never ship a mock timer that returns Done=TRUE immediately.** Timer mocks must respect simulated time via the existing `ScanCycleEngine`. The interpreter already has deterministic time -- timer stubs must use it.

**Detection:**
- All tests pass but production code fails on first real hardware run
- Error handling code has 0% coverage
- No tests verify timing-dependent behavior
- No tests exercise FB error outputs

**Phase to address:** Phase 3 (Mock Framework). The zero-value default is fine for Phase 1 (stub loading) but must be augmented with behavioral mocks and fidelity warnings in Phase 3.

---

### Pitfall 4: Library Version Mismatch Between Stubs and User's Vendor IDE

**What goes wrong:**
stc ships stubs for Tc2_MC2 based on TwinCAT 3.1 Build 4024 documentation. A user has TwinCAT 3.1 Build 4026, which added two new parameters to `MC_MoveAbsolute` (e.g., `MC_BufferMode` default changed, new `Options` struct fields). Their production code uses the new parameters. stc's stubs don't have them. Type-checking rejects valid code.

The reverse is also possible: stc's stubs declare parameters that an older TwinCAT version doesn't have. Code passes stc check but fails in the vendor IDE.

**Specific version differences observed:**

- **Beckhoff Tc2_MC2:** The `Options` parameter (`ST_MoveOptions`) has grown over TwinCAT versions. Older versions have fewer struct fields. The struct is opaque in the PLCopen standard.
- **Beckhoff Tc3_EventLogger:** Completely restructured between TwinCAT 3.1.4022 and 3.1.4024.
- **Schneider SoMachine vs Machine Expert:** Library names changed (`SoMachine Motion` -> `Schneider Electric Motion Control`), FB signatures identical but import paths different.

**Why it happens:**
Vendor libraries are living codebases with their own version histories. Beckhoff releases TwinCAT updates 2-3 times per year, each potentially adding or modifying FB signatures. stc's shipped stubs are a snapshot of one version.

**Consequences:**
- stc rejects code that compiles fine in the vendor IDE (false positive)
- stc accepts code that fails in the vendor IDE (false negative -- worse)
- Users lose trust in stc's type-checking accuracy
- Maintaining version-accurate stubs for all vendor versions is unsustainable

**Prevention:**
1. **Version stubs explicitly.** Each stub file must have a header comment: `(* Tc2_MC2 stubs for TwinCAT 3.1 Build 4024.47 *)`. stc.toml should optionally accept a version: `tc2_mc2 = { path = "vendor/beckhoff/tc2_mc2.st", version = "3.1.4024" }`.
2. **Users override shipped stubs.** Make it trivial to regenerate stubs from the user's actual TwinCAT installation. The `stc vendor extract` command (from VENDOR_LIBRARIES.md section 7) should be a v1.1 feature, not a "future extension."
3. **Warn on unknown parameters, don't error.** If user code calls `mover(NewParam := 42)` and the stub doesn't have `NewParam`, emit a warning (not an error): "Parameter 'NewParam' not found in MC_MoveAbsolute stub (stc stub version: 3.1.4024). The parameter may exist in a newer library version." This lets code pass type-checking with a caveat.
4. **Ship stubs for the latest stable version** of each vendor library. Document which version the stubs represent.

**Detection:**
- User reports "stc rejects my code but it compiles in TwinCAT"
- Stub file has no version header
- No mechanism to regenerate stubs from user's environment

**Phase to address:** Phase 1 (Stub Loading) for version headers and unknown-parameter warnings. Phase 4 (Future) for `stc vendor extract` from TwinCAT projects.

---

### Pitfall 5: Allen Bradley ST Dialect Incompatibilities

**What goes wrong:**
Allen Bradley (Rockwell) Logix 5000 uses a restricted ST dialect that differs from IEC 61131-3 in fundamental ways. stc's compile-checking and emit features will break or produce misleading results if AB differences are not explicitly handled.

**Confirmed AB limitations and differences:**

| Feature | Beckhoff/CODESYS | Allen Bradley | Impact on stc |
|---------|-----------------|---------------|---------------|
| OOP (METHOD, INTERFACE, PROPERTY) | Full support | **Not supported** | Must reject OOP constructs for AB target |
| POINTER TO | Supported | **Not supported** | Must reject pointer types |
| REFERENCE TO | Supported | **Not supported** | Must reject reference types |
| FUNCTION_BLOCK | Standard IEC syntax | **AOI (Add-On Instruction)** -- different declaration syntax, no inheritance | Must emit AOI syntax, not FUNCTION_BLOCK |
| Direct addressing (%I/%Q/%M) | Standard IEC syntax | **Not supported** -- uses tag-based addressing | Must reject or translate to tag references |
| WSTRING | Supported | **Not supported** | Must reject WSTRING for AB |
| Nested comments | Supported | **Varies by firmware version** | May need to flatten comments |
| FOR loop syntax | `FOR i := 0 TO 10 BY 1 DO` | Same syntax but **no BY clause in some versions** | Must warn on BY clause for AB |
| CASE with ranges | `1..10:` | **Not supported** -- must enumerate each value | Must reject or expand ranges for AB |
| Multiple return values | Via VAR_OUTPUT | **Single return tag + parameters** | Structural difference in function design |
| Online editing of FB | Supported | **AOIs cannot be edited online** | Not an stc concern but affects user workflow |
| 64-bit types (LINT, LREAL, ULINT, LWORD) | Supported | **Partial** -- LINT/REAL supported since v32, LREAL depends on controller | Must check controller version for AB |

**Why it happens:**
AB's Logix platform evolved from relay-replacement ladder logic. ST was added later as a secondary language. The platform's core model is "tags in a flat global namespace" rather than "scoped variables in POUs." AOIs are the closest thing to function blocks but lack inheritance, interfaces, and methods. AB has been slowly adding IEC features over firmware versions but remains the most restrictive major vendor.

**Consequences:**
- Code written for Beckhoff cannot be checked against AB stubs without massive rewrites
- Emitting AB-compatible ST requires more than syntax translation -- it requires structural transformation (FUNCTION_BLOCK -> AOI)
- Users who target AB need a completely different coding style, and stc must enforce this early

**Prevention:**
1. **Extend VendorProfile** in `pkg/checker/vendor.go` with AB-specific flags: `SupportsDirectAddressing: false`, `SupportsCaseRanges: false`, `SupportsForBy: false`, `HasAOINotFB: true`.
2. **Defer AB emission** to a later milestone. v1.1 should focus on type-checking and stub loading for AB, not emission. The structural transformation (FB -> AOI) is a significant effort.
3. **Ship AB stubs as AOI-compatible declarations.** AB stubs should only contain FBs that map to common AOIs and built-in instructions. Do not attempt to stub AB's proprietary instruction set (MSG, GSV, SSV, etc.) in v1.1.
4. **Warn early and loudly** when code uses features unsupported by the AB target. The existing `CheckVendorCompat` function handles OOP and pointer warnings -- extend it for the AB-specific features listed above.

**Detection:**
- User sets `vendor_target = "allen_bradley"` and gets no warnings on code using POINTER TO, METHOD, etc.
- AB emit produces FUNCTION_BLOCK syntax instead of AOI format
- AB stubs include FBs that don't exist on AB platform

**Phase to address:** Phase 1 (Vendor Profile extension) for compile-checking. Phase 4+ for emission. AB stubs are lower priority than Beckhoff/Schneider stubs.

---

## Moderate Pitfalls

Mistakes that cause significant rework or user confusion but don't require architectural changes.

---

### Pitfall 6: Circular Dependencies Between Vendor Stubs and User Code

**What goes wrong:**
Vendor stubs define types that user code references, and user code defines types that other user code references. If the resolver processes stubs and user code in the same pass without ordering guarantees, circular references can occur:

```
(* vendor/beckhoff/tc2_mc2.st *)
FUNCTION_BLOCK MC_Power
VAR_INPUT
    Axis : AXIS_REF;    (* AXIS_REF is defined in THIS stub file *)
END_VAR
END_FUNCTION_BLOCK

(* src/motion.st *)
TYPE MyAxisConfig : STRUCT
    axis : AXIS_REF;          (* References type from vendor stub *)
    profile : MotionProfile;  (* References type from user code *)
END_STRUCT
END_TYPE

TYPE MotionProfile : STRUCT
    maxVel : LREAL;
    moveType : MC_Direction;  (* References enum from vendor stub *)
END_STRUCT
END_TYPE
```

This is fine if stubs are resolved first. But if a user creates a type with the same name as a vendor type (e.g., user defines their own `AXIS_REF` struct), the resolver hits a redeclaration error. The current resolver (`resolve.go` line 76) rejects redeclarations unconditionally.

**Prevention:**
1. **Load stubs before user code, in a separate resolution pass.** Stub declarations go into the symbol table first. User declarations are resolved second. If a user declares a type with the same name as a stub type, emit a specific error: "Type 'AXIS_REF' conflicts with vendor library declaration from tc2_mc2.st."
2. **Stubs may NOT reference user-defined types.** This is a hard rule. Stubs reference only IEC elementary types, other stub-defined types, and enum types defined in the same stub file.
3. **Within a single stub file, forward references must work.** The resolver already handles forward references in user code (two-pass). The same mechanism works for stub files -- no extra work needed.

**Detection:**
- Redeclaration errors when loading stubs that define types also defined in user code
- Missing type errors when stub files reference types from other stub files loaded later

**Phase to address:** Phase 1 (Stub Loading). The loading order (stubs -> user code) must be established from the start.

---

### Pitfall 7: AXIS_REF and Other Opaque Vendor Structs

**What goes wrong:**
Beckhoff's `AXIS_REF` is a complex struct with dozens of fields (`NcToPlc`, `PlcToNc`, `Status`, `NcBits`, etc.). The stub in `docs/VENDOR_LIBRARIES.md` simplifies it to two DINT fields. User code that accesses `myAxis.NcToPlc.nActVelo` (a nested struct field) fails type-checking because the stub's `AXIS_REF` doesn't have nested structs.

Schneider's `Axis_Ref` is a completely different struct. Same name (case-insensitive in ST), different contents.

**Why it happens:**
Opaque vendor structs are implementation details that leak through the FB interface. In theory, users should only pass `AXIS_REF` between motion FBs and never access its fields directly. In practice, every TwinCAT project accesses `AXIS_REF.NcToPlc.ActPos` directly because it's the fastest way to read actual position without using `MC_ReadActualPosition`.

**Prevention:**
1. **Stub AXIS_REF with sufficient fields for common access patterns.** Include `NcToPlc.ActPos`, `NcToPlc.ActVelo`, `NcToPlc.nStateDWord`, `PlcToNc.nCommand` at minimum for Beckhoff. This covers 90% of direct access patterns.
2. **Warn on access to unstubbed fields.** If user accesses `myAxis.NcToPlc.SomeObscureField` and the stub doesn't have it, emit a warning: "Field 'SomeObscureField' not found in AXIS_REF stub. May exist in vendor library."
3. **Document that stubs are approximations.** The stub file header should say: `(* Simplified AXIS_REF. For full struct, use stc vendor extract from your TwinCAT project. *)`

**Detection:**
- Type errors on direct AXIS_REF field access that works in TwinCAT
- User code accesses deeply nested struct fields not present in stub

**Phase to address:** Phase 1 (Stub Definition). Ship more complete AXIS_REF stubs than the current two-field version.

---

### Pitfall 8: Mock Override Redeclaration Semantics

**What goes wrong:**
The current resolver rejects redeclarations (`resolve.go` line 76-79). Mock loading requires overriding stub declarations with mock declarations (same name, same signature, but with a body). If the override mechanism is not carefully scoped, it can introduce bugs:

1. **Signature mismatch:** Mock declares `MC_MoveAbsolute` with fewer parameters than the stub. Tests compile, but calling code passes parameters that the mock ignores. Silent data loss.
2. **Scope leakage:** Mock override leaks into non-test compilation. `stc check` (not `stc test`) accidentally picks up mocks, and type-checking passes against mock signatures instead of production stubs.
3. **Partial override:** User mocks one FB but not its dependencies. `MC_MoveAbsolute` mock works, but it internally references `MC_ReadActualPosition` (which is still a zero-value stub). The mock's behavior depends on the uncontrolled zero-value stub.

**Prevention:**
1. **Validate mock signature matches stub signature.** When a mock overrides a stub, verify that all declared inputs and outputs match (same names, same types). Extra `VAR` variables are fine (implementation detail). Missing or type-changed parameters are an error.
2. **Mock loading is test-only.** `[test.mock_paths]` in `stc.toml` is only processed by `stc test`, never by `stc check` or `stc emit`. The resolver needs a flag or mode to control whether mock overrides are active.
3. **Emit a log message for each override.** `stc test --verbose` should print: "Mock: MC_MoveAbsolute from mocks/mc_mock.st overrides stub from vendor/beckhoff/tc2_mc2.st"

**Detection:**
- Tests pass with a mock that has a different parameter list than the stub
- `stc check` behavior changes based on files in `mocks/` directory
- No log output indicating which FBs are mocked vs. stubbed

**Phase to address:** Phase 3 (Mock Framework). Signature validation is critical for preventing silent mock/stub divergence.

---

### Pitfall 9: EtherCAT Terminal Configuration Cannot Be Stubbed

**What goes wrong:**
Users expect vendor stubs to cover everything they need for host testing. But EtherCAT terminal configuration involves concepts that exist outside ST code and cannot be represented as FB stubs:

| Concept | Why It Can't Be Stubbed | Impact |
|---------|------------------------|--------|
| **PDO mapping** | Configured in TwinCAT System Manager (XML), not in ST. Defines which process data objects appear in the process image. Changes the meaning of `%IB0`. | Mock I/O table can't validate that addresses match PDO config. |
| **Distributed clocks (DC)** | EtherCAT synchronization mechanism. Configured per-slave. Affects timing of I/O updates relative to PLC task cycle. | Simulated time can't replicate DC sync jitter or phase shifts. |
| **CoE (CANopen over EtherCAT)** | SDO read/write for terminal parameters (e.g., filter settings, scaling factors). Done via `FB_EcCoeSdoRead/Write` at runtime. | The FB can be stubbed, but what value should the mock return for a specific SDO index? |
| **EtherCAT state machine** | INIT -> PREOP -> SAFEOP -> OP. Transitions affect I/O availability. | A stub can simulate states, but the transition conditions depend on hardware. |
| **Terminal-specific I/O mapping** | An EL3064 (4-channel analog input) maps to specific offsets in the process image. Different terminal = different offsets. | Mock I/O table can provide addresses but can't validate they match actual hardware config. |

**Why it happens:**
ST code and EtherCAT configuration are two separate domains that interact at the process image boundary. TwinCAT projects couple them tightly (`.tsproj` and `.xti` files define the hardware config that determines I/O addresses). stc only sees the ST code.

**Prevention:**
1. **Clearly document the stubbing boundary.** "stc stubs provide type-checking and behavioral mocking for ST-level function blocks. Hardware configuration (PDO mapping, DC, terminal parameters) is outside stc's scope and must be validated on the target platform."
2. **Do NOT attempt to parse or simulate EtherCAT configuration.** This is a rabbit hole that leads to reimplementing TwinCAT System Manager.
3. **Provide a simple mock I/O table** that maps addresses to typed values. Let users define expected I/O values in test files or TOML config. Don't try to derive addresses from hardware config.
4. **Stub EtherCAT diagnostic FBs** (`FB_EcCoeSdoRead`, `FB_EcGetSlaveState`, etc.) as zero-value stubs with a clear fidelity warning. Users who need EtherCAT-level testing should use Beckhoff's TwinCAT simulation.

**Detection:**
- Users open issues asking "why doesn't stc simulate my EtherCAT network?"
- Scope creep into hardware simulation during development
- I/O address validation that depends on knowledge stc doesn't have

**Phase to address:** Phase 2 (I/O Mapping) for documenting the boundary. This pitfall is about scope management, not implementation.

---

## Minor Pitfalls

Issues that cause inconvenience or confusion but are easily fixed.

---

### Pitfall 10: Case Sensitivity in FB Names Across Vendors

**What goes wrong:**
IEC 61131-3 specifies case-insensitive identifiers. But vendor library documentation and code examples use specific casing conventions:

- Beckhoff: `MC_MoveAbsolute`, `ADSREAD`, `FB_FileOpen` (mixed conventions)
- Schneider: `MC_MoveAbsolute`, `READ_VAR` (all caps for system FBs)
- User code: any casing

If the symbol table uses case-sensitive lookup, `mc_moveabsolute` won't match `MC_MoveAbsolute`. If it uses case-insensitive lookup, it must normalize consistently.

**Prevention:**
The existing resolver should already handle case-insensitive lookup (IEC requirement from v1.0). Verify that stub-loaded symbols follow the same normalization. Add test cases: stub declares `MC_MoveAbsolute`, user code references `mc_moveAbsolute`.

**Phase to address:** Phase 1 (Stub Loading). Quick verification, not new work.

---

### Pitfall 11: Stub Files With Syntax That Triggers Parser Edge Cases

**What goes wrong:**
Stub files contain declarations with no body. The parser must handle:
- `FUNCTION_BLOCK` with `VAR_INPUT`/`VAR_OUTPUT` blocks but no statements between the last `END_VAR` and `END_FUNCTION_BLOCK`
- `FUNCTION` with parameters but no body (just `END_FUNCTION`)
- Type declarations using vendor-specific syntax (e.g., Beckhoff `{attribute 'qualified_only'}` pragmas on enums)
- Default values referencing enum members defined in the same file but later in parse order

The parser was tested with complete POUs (declaration + body). Empty-body POUs may trigger untested code paths.

**Prevention:**
1. **Add parser test cases for empty-body POUs.** Specifically: FB with inputs/outputs and no body; function with parameters and no body; function with parameters, no body, and a return type.
2. **Test stub files against the parser before shipping.** Run `stc check` on every shipped stub file as a CI step.
3. **Handle forward-referenced types in stub files.** If `MC_MoveAbsolute` references `MC_Direction` which is defined later in the same file, the two-pass resolver handles it. Verify with a test.

**Phase to address:** Phase 1 (Stub Loading). Test-driven -- add tests for each edge case before implementing.

---

### Pitfall 12: PVOID / Pointer Parameters Simplified as UDINT

**What goes wrong:**
Beckhoff system FBs use `PVOID` (pointer to void) for memory operation parameters (`MEMCPY`, `ADSREAD.DESTADDR`). The current stubs replace `PVOID` with `UDINT` (the raw docs in `VENDOR_LIBRARIES.md` show this). This works for type-checking (both are 32-bit unsigned), but:
- It hides the fact that the parameter is a pointer
- Code passing a regular UDINT value where a pointer is expected will pass type-checking but crash on real hardware
- On 64-bit TwinCAT runtimes, `PVOID` is 64-bit (`ULINT`), not 32-bit

**Prevention:**
1. **Define a stub type alias:** `TYPE PVOID : UDINT; END_TYPE` (or `ULINT` for 64-bit targets)
2. **Add a comment:** `(* PVOID: Pointer to void. Pass ADR(variable) on real TwinCAT. Simplified for stc type-checking. *)`
3. **Consider emitting a warning** when user passes a literal value to a PVOID parameter.

**Phase to address:** Phase 1 (Stub Definition). Low effort, high documentation value.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Stub loading (resolver) | Vendor name collision (two vendors define MC_MoveAbsolute) | Enforce single vendor stub set per project; reject multi-vendor same-name FB loading |
| Stub loading (resolver) | Redeclaration error when loading stubs before user code | Add library-declaration mode to resolver that skips redeclaration checks for stub-then-user ordering |
| I/O address parsing | %I0.0 (no X prefix) rejected by parser | Update lexer to handle optional X in bit addresses |
| I/O address parsing | Hierarchical addressing (%IW1.2) parsed as bit address | Distinguish dotted-path semantics based on size prefix (X = bit.subbyte, B/W/D = module.channel) |
| I/O mock table | Overlapping addresses not detected | Build address range tracker; warn on byte/word writes overlapping bit addresses |
| Mock framework | Mock signature diverges from stub | Validate mock VAR_INPUT/VAR_OUTPUT against stub at load time |
| Mock framework | Zero-value stubs hide error paths | Emit fidelity warning in test output for every unstubbed FB |
| AB vendor support | Users expect full AB compile-checking | Document AB support as "experimental / limited" in v1.1; full support deferred |
| AB vendor support | CASE ranges, FOR BY clause rejected | Add vendor-specific syntax restrictions to checker |
| Shipped stubs | Stubs don't match user's TwinCAT version | Version header in every stub file; `stc vendor extract` for user-specific stubs |
| Shipped stubs | Opaque structs (AXIS_REF) too simplified | Ship more complete struct stubs; warn on access to unstubbed fields |

---

## "Looks Done But Isn't" Checklist for v1.1

- [ ] **Stub loading:** Parser handles FB declaration with completely empty body (no statements, no comments between END_VAR and END_FUNCTION_BLOCK)
- [ ] **Stub loading:** Forward references within a single stub file resolve correctly (enum defined after FB that uses it)
- [ ] **Stub loading:** Case-insensitive matching between stub FB names and user code references
- [ ] **I/O parsing:** `%I0.0` (no X prefix) parses correctly as bit address
- [ ] **I/O parsing:** `%IW1.2` parses as word address with hierarchical path, not as bit 2 of word 1
- [ ] **I/O parsing:** `AT %I*` wildcard syntax accepted in FB declarations
- [ ] **I/O parsing:** `%IL0` rejected when vendor profile does not support 64-bit types
- [ ] **Mock override:** Mock with different parameter types than stub produces error, not silent acceptance
- [ ] **Mock override:** Mocks only active during `stc test`, never during `stc check`
- [ ] **Vendor compat:** Allen Bradley target rejects POINTER TO, REFERENCE TO, METHOD, INTERFACE, PROPERTY, direct addressing
- [ ] **Vendor compat:** Allen Bradley target rejects CASE with ranges (`1..10:`)
- [ ] **Shipped stubs:** Every shipped stub file passes `stc check` without errors
- [ ] **Shipped stubs:** AXIS_REF stub includes NcToPlc.ActPos and PlcToNc fields for Beckhoff
- [ ] **Shipped stubs:** Each stub file has version header comment identifying source vendor library version
- [ ] **Test output:** Warning emitted for each FB using zero-value auto-stub (no user mock)

---

## Sources

- [Beckhoff MC_MoveAbsolute Documentation (Tc2_MC2)](https://infosys.beckhoff.com/content/1033/tcplclib_tc2_mc2/70094731.html) -- verified parameter list: LREAL Position, LREAL Velocity, MC_BufferMode, ST_MoveOptions
- [Schneider MC_MoveAbsolute Documentation](https://product-help.schneider-electric.com/Machine%20Expert/V1.1/en/MotCoLib/MotCoLib/Function_Blocks_-_Single_Axis/Function_Blocks_-_Single_Axis-20.htm) -- verified parameter list: DINT Position, DINT Velocity, no Accel/Decel params
- [PLCopen Motion Control Part 1 Standard](https://plcopen.org/system/files/downloads/plcopen_motion_control_part_1_version_2.0.pdf) -- confirms vendors may choose data types for parameters
- [Rockwell Logix 5000 IEC 61131-3 Compliance](https://literature.rockwellautomation.com/idc/groups/literature/documents/pm/1756-pm018_-en-p.pdf) -- AB compliance documentation
- [Rockwell AOI Documentation](https://literature.rockwellautomation.com/idc/groups/literature/documents/pm/1756-pm010_-en-p.pdf) -- AOI structure and limitations
- [Google Testing Blog: Increase Test Fidelity By Avoiding Mocks](https://testing.googleblog.com/2024/02/increase-test-fidelity-by-avoiding-mocks.html) -- mock fidelity and false confidence
- [Stefan Henneken: IEC 61131-3 Unit Tests](https://stefanhenneken.net/2018/01/24/iec-61131-3-unit-tests/) -- PLC unit testing patterns
- [Unit Testing and PLCs (AllTwinCAT)](https://alltwincat.com/2019/06/18/unit-testing-and-plcs/) -- TwinCAT testing practices
- [IEC 61131-3 Direct Addressing (Wikipedia)](https://en.wikipedia.org/wiki/IEC_61131-3) -- %I/%Q/%M syntax reference
- [Beckhoff EtherCAT System Documentation](https://infosys.beckhoff.com/content/1033/ethercatsystem/2584719371.html) -- PDO mapping and DC configuration
- [CODESYS vs TwinCAT Comparison](https://plcprogramming.io/blog/codesys-vs-twincat-comprehensive-comparison) -- vendor feature differences
- [Allen Bradley AOI Guide (Industrial Monitor Direct)](https://industrialmonitordirect.com/blogs/knowledgebase/allen-bradley-add-on-instructions-complete-aoi-guide) -- AOI limitations vs FUNCTION_BLOCK
- [l5x2ST Project (GitHub)](https://github.com/lagarcia38/l5x2ST) -- AB L5X to ST conversion, demonstrates dialect differences

---
*Pitfalls research for: Vendor Library Stubs, I/O Mapping, and Mock Framework (v1.1 milestone)*
*Researched: 2026-03-30*
