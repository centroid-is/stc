# Feature Landscape: Vendor Libraries, I/O Mapping & Mock Framework

**Domain:** IEC 61131-3 Vendor Library Support for STC Compiler Toolchain
**Researched:** 2026-03-30
**Milestone:** v1.1 Vendor Libraries & I/O
**Confidence:** HIGH (Beckhoff FBs well-documented via Infosys; Schneider MEDIUM -- docs less accessible; AB MEDIUM -- significant dialect differences confirmed)

## Context

This research covers features for the v1.1 milestone only. The v1.0 toolchain (parser, checker, interpreter, test runner, simulation, LSP, MCP) is already shipped. This document identifies:
- Which vendor FBs to stub first
- What I/O terminal types matter
- What mock patterns PLC engineers need
- How AB's ST dialect differs

---

## Table Stakes

Features users expect when a tool claims "vendor library support." Missing these = feature feels half-built.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Stub loading from .st declaration files** | Users write code referencing vendor FBs; stc must resolve them without SEMA010 errors | MEDIUM | Design doc already specifies this. Uses existing parser -- no new format needed |
| **Tc2_MC2 motion control stubs** | PLCopen MC_* blocks are used in nearly every motion application. This is the #1 library users will hit | LOW | 10 core FBs. Signatures are standardized by PLCopen. Already drafted in design doc |
| **Tc2_System stubs (ADS, file I/O, memory)** | ADSREAD/ADSWRITE, FB_FileOpen/Read/Write, MEMCPY are in most TwinCAT projects | LOW | 8-10 core FBs. Signatures well-documented on Beckhoff Infosys |
| **Zero-value auto-stubs for bodiless declarations** | Code must compile and run even if user hasn't written a custom mock. Return zero/FALSE by default | LOW | Interpreter already handles empty-body FBs via NewUserFBInstance |
| **%I*/%Q*/%M* address resolution** | Production code uses AT declarations with I/O addresses. stc must parse and type-check these | MEDIUM | Parser already handles AT; checker needs mock I/O table for simulation |
| **STC_TEST and STC_SIM auto-defines** | Engineers expect conditional compilation to work without manual config | LOW | Preprocessor already exists; just needs auto-define in test/sim commands |
| **Mock override via mocks/ directory** | Engineers need custom mock behavior for testing vendor FBs | MEDIUM | Same-name FB with body overrides bodiless stub. Resolver needs override logic |
| **Schneider basic communication stubs** | READ_VAR, WRITE_VAR are Schneider's equivalent of ADS. Required for Schneider target users | LOW | 4-6 core FBs. Well-documented in Schneider manuals |
| **stc.toml library_paths configuration** | Users must be able to point stc at their vendor stubs | LOW | Config schema already exists per design doc |

## Differentiators

Features that go beyond basic stub loading and set stc apart from vendor IDEs.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **ST-based mock framework (no Go required)** | Engineers write mocks in ST, the language they know. No TcUnit-style "run on PLC" required | MEDIUM | Design doc pattern: mock FB with same name + body overrides stub. Cycle-count simulation for timing |
| **I/O mock table for simulation** | Inject sensor values into %I* addresses, read actuator outputs from %Q* addresses during test/sim | MEDIUM | Extends existing plant model infrastructure. Maps address strings to interpreter variables |
| **Recording mocks with call tracking** | __mock_call_count, __mock_last_* variables for assertion. Like jest.fn() for PLC code | HIGH | v1.1 stretch goal. Zero-value stubs + user mocks cover 90% of need |
| **stc vendor init command** | Scaffolds vendor stub files from built-in registry. `stc vendor init beckhoff tc2_mc2` | LOW | Quality-of-life. Copies from embedded stdlib/vendor/ directory |
| **stc vendor extract from .TcPOU** | Auto-extract FB signatures from existing TwinCAT projects | MEDIUM | Parse XML, extract VAR_INPUT/VAR_OUTPUT, write stub .st files |
| **Allen Bradley dialect awareness** | Warn about AB-incompatible constructs when vendor_target is AB. Flag OOP, POINTER TO, etc. | MEDIUM | Extends existing vendor profile system. Critical for "write once" story |
| **Error injection mocks** | Mocks that simulate Error := TRUE, ErrorID := specific codes. Tests error handling paths | LOW | Just ST code in mock FBs. Document the pattern, not a framework feature |

## Anti-Features

Features to explicitly NOT build for v1.1.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Real ADS/Modbus communication** | stc is a development tool, not a runtime. Real communication requires hardware/network | Stub FBs that accept/return data. Mock communication results |
| **Hardware-specific I/O terminal simulation** | Simulating EL3xxx analog scaling, EL7xxx stepper profiles is unbounded scope | Mock I/O table with raw values. Users write plant models for signal conditioning |
| **Auto-generation of mocks from vendor compiled-libraries** | CODESYS .compiled-library format is proprietary binary | Support .st stub files that users create or extract from .TcPOU XML |
| **Full AB AOI system** | AOIs have a fundamentally different structure than FUNCTION_BLOCK (different parameter model, no OOP) | Provide AB dialect restrictions in vendor profile. Full AOI support deferred to v2 |
| **Tc3_BA building automation library** | Niche vertical. FB_BA_AI, FB_BA_Toggle etc. only relevant to BMS projects | Ship core libraries first. Users can write their own stubs for vertical-specific libs |

---

## Vendor Function Block Catalog

### Beckhoff TwinCAT 3 -- Top 30 Most-Used FBs

Confidence: HIGH -- sourced from Beckhoff Infosys official documentation.

#### Tc2_MC2 (PLCopen Motion Control) -- 10 FBs

Every motion application uses these. Signatures are PLCopen-standardized.

| FB | Signature Summary | Priority |
|----|-------------------|----------|
| MC_Power | Enable/Disable axis. Inputs: Axis:AXIS_REF, Enable:BOOL, Enable_Positive:BOOL, Enable_Negative:BOOL, Override:LREAL. Outputs: Status, Busy, Active, Error, ErrorID | P1 |
| MC_MoveAbsolute | Move to position. Inputs: Axis, Execute, Position:LREAL, Velocity:LREAL, Acceleration:LREAL, Deceleration:LREAL, Jerk:LREAL, Direction:MC_Direction. Outputs: Done, Busy, Active, CommandAborted, Error, ErrorID | P1 |
| MC_MoveRelative | Move relative distance. Same output pattern as MoveAbsolute. Input: Distance instead of Position | P1 |
| MC_MoveVelocity | Continuous velocity move. Inputs: Axis, Execute, Velocity, Acceleration, Deceleration, Jerk, Direction. Outputs: InVelocity, Busy, Active, CommandAborted, Error, ErrorID | P1 |
| MC_Stop | Stop axis. Inputs: Axis, Execute, Deceleration, Jerk. Outputs: Done, Busy, Active, CommandAborted, Error, ErrorID | P1 |
| MC_Home | Home/reference axis. Inputs: Axis, Execute, Position. Outputs: Done, Busy, Active, CommandAborted, Error, ErrorID | P1 |
| MC_Reset | Reset axis error. Inputs: Axis, Execute. Outputs: Done, Busy, Error, ErrorID | P1 |
| MC_ReadActualPosition | Read position. Inputs: Axis, Enable. Outputs: Valid, Busy, Error, ErrorID, Position:LREAL | P1 |
| MC_ReadActualVelocity | Read velocity. Inputs: Axis, Enable. Outputs: Valid, Busy, Error, ErrorID, ActualVelocity:LREAL | P1 |
| MC_ReadStatus | Read axis state machine. Inputs: Axis, Enable. Outputs: Valid, Busy, Error, ErrorID, plus individual state BOOLs (Errorstop, Disabled, Stopping, StandStill, DiscreteMotion, ContinuousMotion, SynchronizedMotion, Homing) | P2 |

Supporting types required:
- `AXIS_REF` -- STRUCT with NcToPlc, PlcToNc fields (opaque for stubs)
- `MC_Direction` -- ENUM (mcPositiveDirection, mcNegativeDirection, mcCurrentDirection, mcShortestWay)

#### Tc2_System (System Services) -- 8 FBs

Used in most TwinCAT projects for ADS communication and file I/O.

| FB/Function | Signature Summary | Priority |
|-------------|-------------------|----------|
| ADSREAD | Inputs: NETID:T_AmsNetId, PORT:T_AmsPort, IDXGRP:UDINT, IDXOFFS:UDINT, LEN:UDINT, DESTADDR:UDINT, READ:BOOL, TMOUT:TIME. Outputs: BUSY, ERR, ERRID | P1 |
| ADSWRITE | Inputs: NETID, PORT, IDXGRP, IDXOFFS, LEN, SRCADDR, WRITE:BOOL, TMOUT:TIME. Outputs: BUSY, ERR, ERRID | P1 |
| FB_FileOpen | Inputs: sNetId, sPathName, nMode:DWORD, ePath:E_OpenPath, bExecute, tTimeout. Outputs: bBusy, bError, nErrId, hFile:UINT | P2 |
| FB_FileClose | Inputs: sNetId, hFile, bExecute, tTimeout. Outputs: bBusy, bError, nErrId | P2 |
| FB_FileRead | Inputs: sNetId, hFile, pReadBuff, cbReadLen, bExecute, tTimeout. Outputs: bBusy, bError, nErrId, cbRead, bEOF | P2 |
| FB_FileWrite | Inputs: sNetId, hFile, pWriteBuff, cbWriteLen, bExecute, tTimeout. Outputs: bBusy, bError, nErrId, cbWrite | P2 |
| MEMCPY | FUNCTION(destAddr:UDINT, srcAddr:UDINT, n:UDINT):UDINT | P1 |
| MEMSET | FUNCTION(destAddr:UDINT, fillByte:BYTE, n:UDINT):UDINT | P2 |

Supporting types:
- `T_AmsNetId` -- STRING(23)
- `T_AmsPort` -- UINT
- `T_MaxString` -- STRING(255)
- `E_OpenPath` -- ENUM (PATH_GENERIC, PATH_BOOTPATH, PATH_BOOTPRJPATH)

#### Tc2_Utilities (Utility Functions) -- 8 FBs

The most commonly used utility FBs across production projects.

| FB/Function | Purpose | Priority |
|-------------|---------|----------|
| FB_FormatString | Printf-style string formatting. Widely used for logging/HMI | P1 |
| FB_FormatString2 | Extended formatting with more placeholders | P2 |
| FB_BasicPID | Simple PID controller. Common in process control | P2 |
| FB_CSVMemBufferReader | Parse CSV data from memory buffer | P3 |
| FB_CSVMemBufferWriter | Write CSV data to memory buffer | P3 |
| NT_GetTime | Read system time. Used in logging, scheduling | P2 |
| FB_LocalSystemTime | Get local time as structured type | P2 |
| FB_WritePersistentData | Force write of persistent variables to storage | P3 |

#### Tc3_JsonXml (JSON/XML Processing) -- 4 FBs

Increasingly common in Industry 4.0 / IIoT projects.

| FB | Purpose | Priority |
|----|---------|----------|
| FB_JsonDomParser | Parse and create JSON documents. Methods: ParseDocument, GetDocumentRoot, HasMember, GetValueByName, etc. | P2 |
| FB_JsonSaxWriter | Stream-write JSON documents | P3 |
| FB_JsonReadWriteDataType | Auto-serialize/deserialize PLC data types to JSON via attributes | P2 |
| FB_XmlDomParser | Parse and create XML documents | P3 |

Note: These FBs use METHOD calls extensively. Stub declarations need method signatures, not just VAR_INPUT/VAR_OUTPUT.

#### Tc2_EtherCAT (EtherCAT Services) -- 4 FBs

Used when projects need runtime EtherCAT diagnostics or parameter access.

| FB | Purpose | Priority |
|----|---------|----------|
| FB_EcCoESdoRead | Read slave parameters via CoE SDO protocol | P2 |
| FB_EcCoESdoWrite | Write slave parameters via CoE SDO protocol | P2 |
| FB_EcGetAllSlaveStates | Read EtherCAT state of all slaves. Used for diagnostics | P3 |
| FB_EcGetSlaveState | Read EtherCAT state of single slave | P3 |

#### Tc3_EventLogger -- 2 FBs

Standard logging in modern TwinCAT projects.

| FB | Purpose | Priority |
|----|---------|----------|
| FB_TcEventLogger | Create and send system events | P2 |
| FB_TcAlarm | Alarm management with confirm/reset semantics | P3 |

### Schneider EcoStruxure -- Top 12 Most-Used FBs

Confidence: MEDIUM -- documentation less publicly accessible than Beckhoff. Based on Schneider community forums, official guides, and common project patterns.

#### Communication (Modbus/Serial/TCP) -- 6 FBs

| FB/Function | Purpose | Priority |
|-------------|---------|----------|
| READ_VAR | Modbus read register(s). Inputs: ADR (address from ADDM), OBJ:STRING, NUM:INT, management table. Outputs: data buffer | P1 |
| WRITE_VAR | Modbus write register(s). Same addressing model | P1 |
| SEND_RECV_MSG | Send and/or receive user-defined messages on serial/TCP. Used for ASCII protocols, custom framing | P2 |
| ADDM | Address Manager -- constructs communication address string. Returns formatted address for READ_VAR/WRITE_VAR | P1 |
| DATA_EXCH | Generic data exchange for non-Modbus protocols | P2 |
| PRINT_CHAR | Send characters to serial port | P3 |

#### Motion (PLCopen-derived) -- 3 FBs

Schneider uses PLCopen FBs with vendor-specific parameter extensions.

| FB | Difference from Beckhoff | Priority |
|----|--------------------------|----------|
| MC_Power | Different parameter names, additional ControllerMode input | P1 |
| MC_MoveAbsolute | Similar to PLCopen standard, different AXIS_REF structure | P1 |
| MC_Stop | Similar to PLCopen standard | P1 |

#### System Utilities -- 3 FBs

| FB/Function | Purpose | Priority |
|-------------|---------|----------|
| GetBit | Extract bit N from WORD/DWORD. Widely used in bit-level I/O manipulation | P1 |
| SetBit | Set/clear bit N in WORD/DWORD | P1 |
| RTC | Read real-time clock. Returns structured date/time | P2 |

### Allen Bradley Logix 5000 -- ST Dialect Differences

Confidence: MEDIUM -- based on Rockwell Automation official documentation (1756-pm007, 1756-pm018).

#### Key Dialect Restrictions (vs. IEC 61131-3 / CODESYS)

| Feature | IEC/CODESYS | Allen Bradley | Impact on stc |
|---------|-------------|---------------|---------------|
| FUNCTION_BLOCK keyword | Supported | NOT supported. Uses AOI (Add-On Instruction) | stc must map FB concepts to AOI syntax in AB emit mode |
| INTERFACE / METHOD / PROPERTY | Supported (OOP) | NOT supported. No OOP | Vendor profile must flag OOP as incompatible |
| POINTER TO | Supported | NOT supported | Vendor profile must flag |
| REFERENCE TO | Supported | NOT supported | Vendor profile must flag |
| EXTENDS / IMPLEMENTS | Supported (inheritance) | NOT supported | Vendor profile must flag |
| LREAL (64-bit float) | Supported | Supported (since firmware v20+) | Safe to use, but older firmware may not support |
| LINT (64-bit int) | Supported | Supported but limited instruction coverage | Flag as partially supported |
| STRING length declaration | STRING(N) | STRING with 82-char default, or STRING[N] with brackets | Emit-time syntax difference |
| ENUM types | TYPE name : (a, b, c) | DINT-based, no native ENUM keyword | Must emit as DINT constants for AB |
| FOR loop | FOR i := 1 TO 10 DO | FOR i := 1 TO 10 DO (same syntax) | Compatible |
| CASE statement | CASE x OF 1: ... | CASE x OF 1: ... (same syntax) | Compatible |
| Timer/Counter | TON, TOF, CTU (IEC standard) | TONR, TOFR, CTU (vendor-specific names, UDT-based) | Emit-time name mapping required |
| PID | Not standard | PIDE instruction (enhanced PID, UDT-based) | AB-specific; needs its own stub |

#### AB Built-In Instructions Available in ST

| Instruction | Type | Purpose | stc Stub Priority |
|-------------|------|---------|-------------------|
| TONR | Timer | Timer On-Delay (retentive) | P2 |
| TOFR | Timer | Timer Off-Delay (retentive) | P2 |
| RTOR | Timer | Retentive Timer On | P2 |
| CTU | Counter | Count Up | P2 |
| CTD | Counter | Count Down | P2 |
| CTUD | Counter | Count Up/Down | P2 |
| PIDE | Process | Enhanced PID with auto-tune support | P2 |
| MSG | Communication | Send/receive messages between controllers | P2 |
| GSV | System | Get System Value -- read controller attributes | P3 |
| SSV | System | Set System Value -- write controller attributes | P3 |
| COP | Data | Copy array/structure | P2 |
| FLL | Data | Fill array with value | P3 |
| SIZE | Data | Get array/structure size | P2 |
| FIND | String | Find substring | P2 |
| MID | String | Extract substring | P2 |
| CONCAT | String | Concatenate strings | P2 |

#### AB AOI Structure (for reference)

AOIs are the AB equivalent of FUNCTION_BLOCK. Key differences:
- Parameters are Input, Output, InOut, Local (similar to FB)
- Cannot use POINTER TO or REFERENCE TO
- Can contain Ladder, FBD, or ST logic internally
- Are called like instructions: `MyAOI(InTag, OutTag);`
- UDTs serve as the "struct" mechanism (no OOP)
- Tags are global by default; no VAR_GLOBAL equivalent (everything is a tag)

---

## I/O Terminal Type Catalog

### Beckhoff EtherCAT Terminal Families

Confidence: HIGH -- official Beckhoff product catalog.

The most commonly used terminals that stc's I/O mock table should understand:

#### Digital Input (EL1xxx) -- Most Common

| Terminal | Description | Data Type | Mock Value |
|----------|-------------|-----------|------------|
| EL1004 | 4-ch digital input, 24V DC, 3ms | BOOL per channel | TRUE/FALSE |
| EL1008 | 8-ch digital input, 24V DC, 3ms | BOOL per channel | TRUE/FALSE |
| EL1018 | 8-ch digital input, 24V DC, 10us (fast) | BOOL per channel | TRUE/FALSE |
| EL1014 | 4-ch digital input, 24V DC, 10us | BOOL per channel | TRUE/FALSE |
| EL1809 | 16-ch digital input, 24V DC | BOOL per channel | TRUE/FALSE |

**Mock pattern:** Map to %IX address. Each channel = one BOOL. `%IX0.0` through `%IX0.7` for EL1008.

#### Digital Output (EL2xxx) -- Most Common

| Terminal | Description | Data Type | Mock Value |
|----------|-------------|-----------|------------|
| EL2004 | 4-ch digital output, 24V DC, 0.5A | BOOL per channel | Read back TRUE/FALSE |
| EL2008 | 8-ch digital output, 24V DC, 0.5A | BOOL per channel | Read back TRUE/FALSE |
| EL2809 | 16-ch digital output, 24V DC, 0.5A | BOOL per channel | Read back TRUE/FALSE |
| EL2624 | 4-ch relay output, 125V AC/30V DC | BOOL per channel | Read back TRUE/FALSE |

**Mock pattern:** Map to %QX address. Write TRUE/FALSE to activate outputs. `%QX0.0` through `%QX0.7` for EL2008.

#### Analog Input (EL3xxx) -- Most Common

| Terminal | Description | Data Type | Mock Value |
|----------|-------------|-----------|------------|
| EL3001 | 1-ch analog input, +/-10V, 12-bit | INT (raw) | -32768..32767 |
| EL3004 | 4-ch analog input, +/-10V, 12-bit | INT per channel | -32768..32767 |
| EL3008 | 8-ch analog input, +/-10V, 12-bit | INT per channel | -32768..32767 |
| EL3021 | 1-ch analog input, 4-20mA, 12-bit | INT (raw) | 0..32767 |
| EL3024 | 4-ch analog input, 4-20mA, 12-bit | INT per channel | 0..32767 |
| EL3064 | 4-ch analog input, 0-10V, 12-bit | INT per channel | 0..32767 |
| EL3102 | 2-ch analog input, +/-10V, 16-bit | INT per channel | -32768..32767 |
| EL3202 | 2-ch PT100 RTD temperature input | INT per channel | Temp x 10 |
| EL3314 | 4-ch thermocouple input | INT per channel | Temp x 10 |

**Mock pattern:** Map to %IW address. INT value represents raw ADC reading. Users apply scaling in ST code or plant model. `%IW0` for first channel.

#### Analog Output (EL4xxx) -- Most Common

| Terminal | Description | Data Type | Mock Value |
|----------|-------------|-----------|------------|
| EL4001 | 1-ch analog output, 0-10V, 12-bit | INT (raw) | 0..32767 |
| EL4004 | 4-ch analog output, 0-10V, 12-bit | INT per channel | 0..32767 |
| EL4024 | 4-ch analog output, 4-20mA, 12-bit | INT per channel | 0..32767 |
| EL4034 | 4-ch analog output, +/-10V, 16-bit | INT per channel | -32768..32767 |

**Mock pattern:** Map to %QW address. INT value is raw DAC output. `%QW0` for first channel.

#### Communication (EL6xxx) -- Common

| Terminal | Description | Notes |
|----------|-------------|-------|
| EL6001 | Serial interface RS232 | Used with SEND_RECV_MSG / FB_FileRead in ST |
| EL6021 | Serial interface RS422/RS485 | Modbus RTU over serial |
| EL6224 | IO-Link master, 4 ports | IO-Link device communication |
| EL6731 | PROFIBUS DP master | Legacy fieldbus gateway |
| EL6751 | CANopen master | CANopen device communication |

**Mock pattern:** Communication terminals are not directly addressable as %I/%Q in most cases. Their FBs (serial read/write) are mocked at the FB level, not I/O level.

#### Motion (EL7xxx) -- Common

| Terminal | Description | Notes |
|----------|-------------|-------|
| EL7031 | Stepper motor, 1.5A, incremental encoder | Controlled via NC axis, not direct I/O |
| EL7041 | Stepper motor, 5A, incremental encoder | Same NC-axis control model |
| EL7047 | Stepper motor, 5A, BiSS-C encoder | Higher-end stepper with absolute encoder |
| EL7201 | Servo motor, 4.5A RMS, resolver | Servo control via NC axis |
| EL7211 | Servo motor, 4.5A RMS, resolver, OCT | With One Cable Technology |

**Mock pattern:** Motion terminals are controlled through NC axis configuration, accessed via MC_* FBs. Mock at MC_Power / MC_MoveAbsolute level, not at terminal level.

### I/O Address Space Design for Mock Table

The stc mock I/O table should support these address patterns:

| Address Pattern | Type | Size | Example |
|-----------------|------|------|---------|
| %IX{byte}.{bit} | Digital input bit | BOOL | %IX0.0, %IX0.7, %IX1.0 |
| %QX{byte}.{bit} | Digital output bit | BOOL | %QX0.0, %QX0.7 |
| %IW{word} | Analog/word input | INT/WORD | %IW0, %IW1, %IW100 |
| %QW{word} | Analog/word output | INT/WORD | %QW0, %QW1 |
| %ID{dword} | Double-word input | DINT/DWORD | %ID0, %ID4 |
| %QD{dword} | Double-word output | DINT/DWORD | %QD0, %QD4 |
| %MD{dword} | Memory marker (dword) | DINT/DWORD | %MD0, %MD100 |
| %MX{byte}.{bit} | Memory marker (bit) | BOOL | %MX0.0 |
| %MW{word} | Memory marker (word) | INT/WORD | %MW0 |

---

## Mock Patterns for PLC Engineers

### Pattern 1: Zero-Value Auto-Stub (Default)

**When:** Code references a vendor FB but no custom mock exists.
**Behavior:** All outputs return zero/FALSE. Execute is a no-op.
**Use case:** 80% of test scenarios where you just need code to compile and run.

```iec
(* No mock written -- auto-stub returns zero *)
mover(Execute := TRUE, Position := 100.0);
(* mover.Done = FALSE, mover.Busy = FALSE, mover.Error = FALSE *)
```

### Pattern 2: Cycle-Count Simulation Mock

**When:** Testing state machine logic that depends on FB completion timing.
**Behavior:** After rising edge of Execute, go Busy for N cycles, then Done.
**Use case:** Motion sequences, communication timeouts, process steps.

```iec
(* Mock MC_MoveAbsolute: Done after 5 cycles *)
FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT ... END_VAR
VAR_OUTPUT ... END_VAR
VAR
    prevExecute : BOOL;
    cycleCount  : INT;
END_VAR

IF Execute AND NOT prevExecute THEN
    Busy := TRUE;  Done := FALSE;  cycleCount := 0;
END_IF;
IF Busy THEN
    cycleCount := cycleCount + 1;
    IF cycleCount >= 5 THEN
        Done := TRUE;  Busy := FALSE;
    END_IF;
END_IF;
prevExecute := Execute;
END_FUNCTION_BLOCK
```

### Pattern 3: Error Injection Mock

**When:** Testing error handling paths (fault recovery, alarm management).
**Behavior:** After Execute, set Error := TRUE with specific ErrorID.

```iec
(* Mock that simulates axis fault *)
FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT ... END_VAR
VAR_OUTPUT ... END_VAR
VAR
    prevExecute : BOOL;
END_VAR

IF Execute AND NOT prevExecute THEN
    Error := TRUE;
    ErrorID := 16#4FFF;  (* Drive fault *)
    Busy := FALSE;
    Done := FALSE;
END_IF;
prevExecute := Execute;
END_FUNCTION_BLOCK
```

### Pattern 4: Configurable Response Mock

**When:** Same mock needs to return different results based on test scenario.
**Behavior:** Use a global variable or additional input to control mock behavior.

```iec
(* Configurable mock -- test sets MockMode before calling *)
VAR_GLOBAL
    MockMode_MoveAbsolute : INT;  (* 0=succeed, 1=error, 2=timeout *)
END_VAR

FUNCTION_BLOCK MC_MoveAbsolute
VAR_INPUT ... END_VAR
VAR_OUTPUT ... END_VAR
VAR prevExecute : BOOL; cycles : INT; END_VAR

IF Execute AND NOT prevExecute THEN
    CASE MockMode_MoveAbsolute OF
        0: Busy := TRUE; Done := FALSE; Error := FALSE; cycles := 0;
        1: Error := TRUE; ErrorID := 16#4FFF; Busy := FALSE;
        2: Busy := TRUE; Done := FALSE; Error := FALSE; cycles := 0;
    END_CASE;
END_IF;
IF Busy THEN
    cycles := cycles + 1;
    CASE MockMode_MoveAbsolute OF
        0: IF cycles >= 5 THEN Done := TRUE; Busy := FALSE; END_IF;
        2: (* Never completes -- simulates timeout *);
    END_CASE;
END_IF;
prevExecute := Execute;
END_FUNCTION_BLOCK
```

### Pattern 5: Communication Stub (Modbus/ADS)

**When:** Testing code that reads/writes via ADSREAD or READ_VAR.
**Behavior:** Returns pre-configured data buffer. Simulates BUSY/DONE cycle.

```iec
(* Mock ADSREAD that returns simulated data *)
FUNCTION_BLOCK ADSREAD
VAR_INPUT ... END_VAR
VAR_OUTPUT ... END_VAR
VAR
    prevRead : BOOL;
    cycles   : INT;
END_VAR

IF READ AND NOT prevRead THEN
    BUSY := TRUE; ERR := FALSE; cycles := 0;
END_IF;
IF BUSY THEN
    cycles := cycles + 1;
    IF cycles >= 2 THEN
        BUSY := FALSE;
        (* Data would be "written" to DESTADDR in real TwinCAT.
           In test, the test case pre-populates the target variable. *)
    END_IF;
END_IF;
prevRead := READ;
END_FUNCTION_BLOCK
```

### Pattern 6: I/O Injection via Mock Table

**When:** Testing code that reads sensor inputs or writes actuator outputs.
**Behavior:** Test injects values into %I* addresses before running logic.

```iec
{test}
TEST_CASE 'Temperature alarm triggers above threshold'
VAR
    controller : TemperatureController;
END_VAR

(* Inject mock sensor value: 85.0 degrees via analog input *)
(* stc's mock I/O table maps %IW10 to an injectable INT value *)
__SET_IO(%IW10, 8500);  (* Raw value = temp * 100 *)

(* Run one scan cycle *)
controller();

(* Check that alarm output is set *)
ASSERT_TRUE(__GET_IO(%QX2.0), 'High temp alarm should be active');

END_TEST_CASE
```

Note: `__SET_IO` and `__GET_IO` are stc-specific test helpers (not IEC standard). They are only available when `STC_TEST` is defined.

### How TcUnit Handles Hardware Dependencies

Confidence: HIGH -- documented on tcunit.org and AllTwinCAT blog.

TcUnit's approach to mocking hardware:
1. **Disable all I/Os:** TcUnit-Runner automatically disables all I/O links before running tests. This prevents hardware state from affecting test results.
2. **Protected write access:** TcUnit provides `TEST_ORDERED` with special functions to write to variables that are normally read-only (e.g., %I* inputs).
3. **Multi-cycle tests:** `TEST_FINISHED()` allows tests to span multiple PLC scan cycles, essential for testing FB state machines.
4. **No mock framework:** TcUnit does NOT provide a mock framework. Engineers typically test at the application layer, avoiding direct hardware FB calls in test code. This is the gap stc fills.

stc's advantage: Because stc runs on host (not PLC), it can provide genuine mock FBs with configurable behavior. TcUnit users must either avoid vendor FBs in test code or wrap them in abstraction layers.

---

## Feature Dependencies

```
[Stub File Loading]
    |
    +--enables--> [Type Checking vendor FB calls]
    |                 |
    |                 +--enables--> [LSP completion for vendor FBs]
    |
    +--enables--> [Zero-Value Auto-Stubs in interpreter]
    |                 |
    |                 +--enables--> [Mock Override Loading]
    |                                   |
    |                                   +--enables--> [Custom Mock FBs]
    |
    +--enables--> [I/O Mock Table]
                      |
                      +--enables--> [__SET_IO / __GET_IO helpers]
                      |
                      +--enables--> [Plant Model I/O binding]

[STC_TEST/STC_SIM auto-defines]
    +--enables--> [Conditional compilation in test vs production]

[Vendor Profile Extensions]
    +--enables--> [AB dialect warnings]
```

## MVP Recommendation for v1.1

### Must Have (ship with milestone)

1. **Stub file loading** -- stc.toml library_paths, resolver loads before user code
2. **Tc2_MC2 stubs** -- 10 PLCopen motion FBs with AXIS_REF and MC_Direction types
3. **Tc2_System stubs** -- ADSREAD, ADSWRITE, FB_File*, MEMCPY
4. **Zero-value auto-stubs** -- empty body = no-op execute, zero outputs
5. **Mock override via mocks/ directory** -- same-name FB with body replaces stub
6. **STC_TEST / STC_SIM auto-defines** -- preprocessor symbols set automatically
7. **I/O address resolution in checker** -- %I*, %Q*, %M* don't produce errors
8. **Mock I/O table** -- basic set/get for %IX, %QX, %IW, %QW addresses

### Should Have (stretch goals for v1.1)

9. **Tc2_Utilities stubs** -- FB_FormatString, NT_GetTime, FB_BasicPID
10. **Schneider communication stubs** -- READ_VAR, WRITE_VAR, ADDM, GetBit, SetBit
11. **__SET_IO / __GET_IO test helpers** -- inject/read I/O values in tests
12. **stc vendor init command** -- scaffold stub files from built-in registry
13. **AB vendor profile restrictions** -- warn about OOP, POINTER TO, etc.

### Defer to v1.2+

14. **Tc3_JsonXml stubs** -- requires METHOD support in stub declarations
15. **Tc2_EtherCAT stubs** -- niche, only needed for EtherCAT diagnostics
16. **Tc3_EventLogger stubs** -- useful but not blocking for most tests
17. **Recording mocks** -- __mock_call_count pattern
18. **stc vendor extract** -- .TcPOU XML extraction
19. **Full AB AOI emit** -- needs separate research phase

## Sources

- [Beckhoff Tc2_MC2 Documentation](https://infosys.beckhoff.com/content/1033/tcplclib_tc2_mc2/index.html)
- [Beckhoff Tc2_System Documentation](https://infosys.beckhoff.com/content/1033/tcplclib_tc2_system/index.html)
- [Beckhoff Tc2_Utilities Function Blocks](https://infosys.beckhoff.com/content/1033/tcplclib_tc2_utilities/34965771.html)
- [Beckhoff Tc3_JsonXml Function Blocks](https://infosys.beckhoff.com/content/1033/tcplclib_tc3_jsonxml/4219229195.html)
- [Beckhoff Tc2_EtherCAT Overview](https://infosys.beckhoff.com/content/1033/tcplclib_tc2_ethercat/56993291.html)
- [Beckhoff Tc3_BA_Common](https://infosys.beckhoff.com/content/1033/tcplclib_tc3_ba_common/5030568587.html)
- [Beckhoff EtherCAT Terminals Product Overview](https://www.beckhoff.com/en-us/products/i-o/ethercat-terminals/tabular-product-overview/)
- [Beckhoff FB_init/FB_exit Methods](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/5044757003.html)
- [TcUnit Testing Framework](https://tcunit.org/)
- [TcUnit DeepWiki Overview](https://deepwiki.com/tcunit/TcUnit)
- [AllTwinCAT: Unit Testing in Industrial Automation](https://alltwincat.com/2021/02/16/unit-testing-in-the-world-of-industrial-automation/)
- [Rockwell Logix 5000 Structured Text Manual (1756-pm007)](https://literature.rockwellautomation.com/idc/groups/literature/documents/pm/1756-pm007_-en-p.pdf)
- [Rockwell Logix 5000 IEC 61131-3 Compliance (1756-pm018)](https://literature.rockwellautomation.com/idc/groups/literature/documents/pm/1756-pm018_-en-p.pdf)
- [Rockwell PIDE Instruction Documentation](https://www.rockwellautomation.com/en-us/docs/studio-5000-logix-designer/37-00/contents-ditamap/instruction-set/process-control-instructions/pide.html)
- [Schneider SEND_RECV_MSG Documentation](https://product-help.schneider-electric.com/Machine%20Expert/V1.1/en/m2xxcom/m2xxcom/Function_Block_Descriptions/Function_Block_Descriptions-7.htm)
- [Schneider Communication Services Guide](https://iportal2.schneider-electric.com/Contents/docs/SQD-BMENOC0301_User%20guide.pdf)
- [Schneider EcoStruxure Machine Expert Generic Libraries](https://www.se.com/us/en/download/document/EIO0000003289/)
- [STC Vendor Libraries Design Doc](docs/VENDOR_LIBRARIES.md)

---
*Feature research for: v1.1 Vendor Libraries, I/O Mapping & Mock Framework*
*Researched: 2026-03-30*
