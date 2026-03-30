# Stack Research: Vendor Libraries, I/O Mapping & Mock Framework

**Domain:** IEC 61131-3 vendor library integration, PLC I/O address mapping, host-based mock framework
**Researched:** 2026-03-30
**Confidence:** MEDIUM-HIGH (I/O addressing from IEC standard + Beckhoff docs; library formats from vendor docs; AB dialect from multiple community sources)

## Recommended Stack

### Core Technologies (No New Dependencies)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go `encoding/xml` | stdlib | Parse TwinCAT `.TcPOU` XML for stub extraction | No external dep needed; TcPOU is simple XML with CDATA |
| Go `encoding/json` | stdlib | I/O table serialization for mock framework | Already used throughout stc for `--format json` |
| Existing stc parser | v1.0 | Parse `.st` stub files for vendor library loading | Stubs use plain ST syntax -- no new parser needed |
| Existing stc interpreter | v1.0 | Execute mock FBs written in ST | `FBInstance` + `StandardFB` interface already handles user-defined FBs |

**Key insight: This milestone requires ZERO new Go dependencies.** Everything builds on `encoding/xml` (stdlib) and existing stc infrastructure.

### Supporting Libraries (None Required)

No external Go packages are needed. The entire feature set builds on:

1. **Parser** -- already handles `FUNCTION_BLOCK` with empty body, `AT` keyword, `VarConfig` section
2. **Checker** -- already has `VendorProfile` with feature gates, `BuiltinFunctions` map
3. **Interpreter** -- already has `StdlibFBFactory`, `FBInstance`, `ScanCycleEngine`
4. **Project config** -- already has `[build.library_paths]` in `stc.toml`

### New Internal Packages/Files

| Package/File | Purpose | Notes |
|--------------|---------|-------|
| `pkg/iomap/iomap.go` | I/O address table: `%I*`, `%Q*`, `%M*` areas with typed read/write | Flat byte arrays with typed access, not a full PLC memory model |
| `pkg/iomap/address.go` | Parse AT addresses: `%IX0.0`, `%QW4`, `%MD12`, `%I*` | Regex-based parser for `%[IQM][XBWD]?(\*\|\d+(\.\d+)*)` |
| `pkg/vendor/loader.go` | Load `.st` stub files from `[build.library_paths]` config | Calls parser, registers declarations in symbol table before user code |
| `pkg/vendor/extract.go` | Extract FB signatures from TwinCAT `.TcPOU` XML files | `stc vendor extract` command |
| `pkg/interp/mock.go` | Auto-stub generation for declaration-only FBs | Zero-value outputs, no-op Execute |

## I/O Memory Model: %I, %Q, %M Areas

### IEC 61131-3 Address Format (HIGH confidence -- from standard)

```
%<area><size><position>

Area:  I = Input, Q = Output, M = Memory (flag/marker)
Size:  X = Bit, B = Byte, W = Word (16-bit), D = Double word (32-bit)
       (omitted = Bit for BOOL, Byte for others -- vendor-dependent)
Position: <byte>[.<bit>]  or  * (auto-assign)
```

**Examples:**

| Address | Meaning | Data Width |
|---------|---------|------------|
| `%IX0.0` | Input bit 0 of byte 0 | 1 bit |
| `%IX0.7` | Input bit 7 of byte 0 | 1 bit |
| `%IB0` | Input byte 0 | 8 bits |
| `%IW0` | Input word at byte 0 | 16 bits |
| `%IW2` | Input word at byte 2 | 16 bits |
| `%ID0` | Input double word at byte 0 | 32 bits |
| `%QX0.0` | Output bit 0 of byte 0 | 1 bit |
| `%QB4` | Output byte 4 | 8 bits |
| `%QW8` | Output word at byte 8 | 16 bits |
| `%MD48` | Memory double word at byte 48 | 32 bits |
| `%I*` | Auto-assigned input (TwinCAT/CODESYS wildcard) | Per variable type |
| `%Q*` | Auto-assigned output | Per variable type |

**Overlap rules:** `%IW0` occupies bytes 0-1. `%IW2` occupies bytes 2-3. `%ID0` occupies bytes 0-3 and overlaps with `%IW0` and `%IW2`. This is intentional and how real PLCs work.

### AT Keyword Semantics (HIGH confidence -- from IEC 61131-3 + Beckhoff InfoSys)

The `AT` keyword binds a symbolic variable name to a physical I/O address:

```iec
VAR
    startButton AT %IX0.0 : BOOL;        (* Input bit 0.0 *)
    motorSpeed  AT %QW4   : INT;         (* Output word at byte 4 *)
    recipe      AT %MD100 : DWORD;       (* Memory double word at byte 100 *)
END_VAR
```

**Where AT is valid (per IEC 61131-3):**
- `VAR` blocks in `PROGRAM` declarations
- `VAR_GLOBAL` blocks (GVL in TwinCAT)
- `VAR_CONFIG` blocks (configuration-level linking)
- NOT valid in `FUNCTION_BLOCK` or `FUNCTION` VAR blocks (FBs are hardware-independent)

**TwinCAT wildcard addressing:**
```iec
VAR
    sensor1 AT %I* : BOOL;   (* TwinCAT assigns address at link time *)
    output1 AT %Q* : BOOL;   (* Linked to PDO via System Manager *)
END_VAR
```

The wildcard `*` means "allocate from the I/O image but let the IDE link it to specific hardware." In stc, wildcards should allocate from the mock I/O table sequentially.

### Memory Model for stc Mock I/O

Use three flat byte arrays, one per area:

```go
type IOTable struct {
    I []byte // Input image  (default 1024 bytes, growable)
    Q []byte // Output image (default 1024 bytes, growable)
    M []byte // Memory/flag area (default 4096 bytes, growable)
}

// Typed access methods
func (t *IOTable) GetBit(area byte, byteOffset, bitOffset int) bool
func (t *IOTable) SetBit(area byte, byteOffset, bitOffset int, v bool)
func (t *IOTable) GetByte(area byte, offset int) byte
func (t *IOTable) SetByte(area byte, offset int, v byte)
func (t *IOTable) GetWord(area byte, offset int) uint16
func (t *IOTable) SetWord(area byte, offset int, v uint16)
func (t *IOTable) GetDWord(area byte, offset int) uint32
func (t *IOTable) SetDWord(area byte, offset int, v uint32)
```

This mirrors how real PLCs work: the I/O image is a contiguous byte buffer. Variables declared with AT addresses are views into this buffer. The scan cycle copies physical I/O into/out of these buffers.

## EtherCAT Terminal I/O Mapping (MEDIUM confidence -- from Beckhoff product docs)

### How TwinCAT Maps Terminals to Addresses

EtherCAT terminals are mapped to the PLC I/O image through Process Data Objects (PDOs). The mapping is done in the TwinCAT System Manager, not in ST code. The ST code only sees AT addresses.

| Terminal | Type | Channels | Process Data | Typical AT Address |
|----------|------|----------|-------------|-------------------|
| EL1008 | 8-ch digital input | 8 x BOOL | 1 byte (8 bits packed) | `%IB0` or `%IX0.0` thru `%IX0.7` |
| EL2008 | 8-ch digital output | 8 x BOOL | 1 byte (8 bits packed) | `%QB0` or `%QX0.0` thru `%QX0.7` |
| EL3064 | 4-ch analog input 0-10V, 12-bit | 4 x INT | 8 bytes (4 x 16-bit) | `%IW0`, `%IW2`, `%IW4`, `%IW6` |
| EL4034 | 4-ch analog output, 16-bit | 4 x INT | 8 bytes (4 x 16-bit) | `%QW0`, `%QW2`, `%QW4`, `%QW6` |

**EL3064 detail:** Each channel has a 16-bit value (12-bit ADC, upper 4 bits = status in compact mode or zero-padded). The mapping puts Channel 1 at the first word, Channel 2 at the next, etc. With compact PDO mapping, it is 4 x INT (8 bytes total).

**What stc needs:** stc does not need to know about specific terminal models. The user declares AT addresses in their GVL, and stc maps them to the IOTable. The terminal model determines what addresses exist, but that is a TwinCAT System Manager concern, not an stc concern. stc just needs to handle `AT %IX0.0 : BOOL` and `AT %IW4 : INT` correctly.

### Implications for stc

Do NOT model EtherCAT terminals. Model the I/O image (byte arrays) that terminals write to. Users declare AT addresses and stc resolves them to offsets in the I/O table. During testing, users set I/O values via `SetInput` on the scan cycle engine or via a mock I/O table API.

## Beckhoff .library File Format (MEDIUM confidence -- from Beckhoff InfoSys + CODESYS docs)

### Format Assessment

| Format | Internal Structure | Extractable? | Notes |
|--------|-------------------|-------------|-------|
| `.library` (source) | Proprietary binary container with embedded ST source | Partially -- CODESYS scripting API can export | Contains full source but format is undocumented |
| `.compiled-library` | Proprietary binary, source stripped | NO -- signatures present but binary format undocumented | Used for distribution, no public parser exists |
| `.TcPOU` (TwinCAT) | Simple XML with CDATA-wrapped ST | YES -- trivial to parse | Each POU in its own file, declaration in `<Declaration>` element |
| `.plcproj` (TwinCAT) | MSBuild XML | YES -- lists all `.TcPOU` references | Standard XML, references POU files |
| PLCopen XML | Standardized IEC 61131-10 XML | YES -- public XSD schema | Can represent POUs with or without bodies |

### Recommended Extraction Path

**Primary:** Parse `.TcPOU` files directly. Structure is trivial:

```xml
<?xml version="1.0" encoding="utf-8"?>
<TcPlcObject Version="1.1.0.1" ProductVersion="3.1.4024.6">
  <POU Name="FB_MyBlock" Id="{guid}" SpecialFunc="None">
    <Declaration><![CDATA[
FUNCTION_BLOCK FB_MyBlock
VAR_INPUT
    bExecute : BOOL;
    nValue   : INT;
END_VAR
VAR_OUTPUT
    bDone  : BOOL;
    bError : BOOL;
END_VAR
    ]]></Declaration>
    <Implementation>
      <ST><![CDATA[
        (* implementation code *)
      ]]></ST>
    </Implementation>
  </POU>
</TcPlcObject>
```

Go parsing is trivial with `encoding/xml`:

```go
type TcPlcObject struct {
    XMLName xml.Name `xml:"TcPlcObject"`
    POU     struct {
        Name        string `xml:"Name,attr"`
        Declaration string `xml:"Declaration"`
        Implementation struct {
            ST string `xml:"ST"`
        } `xml:"Implementation"`
    } `xml:"POU"`
}
```

Extract `Declaration` CDATA, strip/ignore `Implementation`, write to `.st` stub file. The stc parser then handles the stub file natively.

**Secondary:** PLCopen XML import (already planned as a future feature).

**Do NOT attempt:** Parsing `.library` or `.compiled-library` files. The binary format is proprietary, undocumented, and changes between CODESYS versions. The cost/benefit is terrible.

## Allen Bradley Dialect Differences (MEDIUM confidence -- from Rockwell docs + community sources)

### Feature Comparison: AB Logix 5000 vs IEC 61131-3 / CODESYS

| Feature | Beckhoff (CODESYS) | Schneider (CODESYS-derived) | Allen Bradley (Logix 5000) |
|---------|-------------------|---------------------------|---------------------------|
| `FUNCTION_BLOCK` | Full support | Full support | **NO** -- uses Add-On Instructions (AOI) |
| `FUNCTION` | Full support | Full support | Full support |
| `PROGRAM` | Full support | Full support | **Tasks + Routines** model instead |
| OOP (METHOD, INTERFACE, PROPERTY) | Full support | No | **No** |
| POINTER TO | Yes | No | **No** |
| REFERENCE TO | Yes | No | **No** |
| ENUM types | Full support | Full support | **No native ENUM** -- use DINT constants |
| LREAL (64-bit float) | Yes | Yes | **Yes** (newer firmware v31+) |
| LINT/ULINT (64-bit int) | Yes | Yes | **LINT yes, ULINT no** |
| WSTRING | Yes | Yes | **No** |
| STRING literal assignment | Yes | Yes | **No** -- cannot assign `'hello'` to STRING tag directly |
| AT %I/%Q/%M addressing | Yes | Yes | **No** -- tag-based, no direct addresses |
| VAR_GLOBAL (GVL) | Yes | Yes | **Controller-scoped tags** instead |
| REPEAT..UNTIL | Yes | Yes | **Not documented** |
| CASE with ranges | Yes | Yes | **Integer values only, no ranges** |
| Multi-dimensional arrays | Yes | Yes | **3 dimensions max** |
| Timer types | TON, TOF, TP (IEC) | TON, TOF, TP (IEC) | **TONR, TOF** -- different names/behavior |

### AB-Specific Concepts Needing Stubs

| AB Concept | IEC Equivalent | Stub Strategy |
|------------|---------------|---------------|
| Add-On Instruction (AOI) | FUNCTION_BLOCK | Declare as FUNCTION_BLOCK in stub, note in metadata |
| User-Defined Type (UDT) | TYPE..STRUCT..END_TYPE | Same syntax, no change needed |
| Controller-scoped tag | VAR_GLOBAL | Use VAR_GLOBAL in stubs |
| MSG instruction | ADSREAD/ADSWRITE equivalent | Stub as FUNCTION_BLOCK with Done/Error/EN/DN outputs |
| PID instruction | PID FB | Stub with AB-specific parameters (SP, PV, CV, etc.) |
| TONR (retentive timer) | No IEC equivalent | Custom stub FB |
| GSV/SSV (Get/Set System Value) | No IEC equivalent | Stub as FUNCTION with class/attribute params |

### Implications for stc Stubs

AB stubs should be written as standard IEC 61131-3 FUNCTION_BLOCK declarations even though AB uses AOIs internally. The stub provides the type signature for stc's checker. When emitting for AB target, the emitter maps FUNCTION_BLOCK to AOI syntax. This is already the pattern used for Beckhoff/Schneider FB differences.

## Go XML Parsing for TwinCAT Files (HIGH confidence -- Go stdlib)

No external libraries needed. `encoding/xml` handles TcPOU files perfectly:

```go
// Parse a .TcPOU file
func ParseTcPOU(r io.Reader) (*TcPOU, error) {
    var obj TcPlcObject
    if err := xml.NewDecoder(r).Decode(&obj); err != nil {
        return nil, err
    }
    return &obj.POU, nil
}

// Parse a .plcproj file to find all .TcPOU references
func ParsePlcProj(r io.Reader) ([]string, error) {
    // .plcproj is MSBuild XML with <Compile Include="path.TcPOU" /> elements
    // Use xml.Decoder to extract Include attributes
}
```

For walking a TwinCAT project directory: `filepath.WalkDir` + filter for `.TcPOU` extension.

## Alternatives Considered

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| Plain `.st` stubs | YAML/TOML/JSON stub format | Engineers know ST syntax; parser already handles it; zero new formats |
| Go `encoding/xml` | `github.com/beevik/etree` | etree is nicer API but adds dependency for simple XML; stdlib is sufficient |
| Flat byte array I/O table | Structured I/O model per terminal | Over-engineering; real PLCs use flat byte images too |
| Zero-value auto-stubs | Require explicit mocks for all FBs | Too much friction; most tests only care about a few FBs |
| Extract from `.TcPOU` XML | Parse `.library` binary files | `.library` format is proprietary and undocumented; `.TcPOU` is trivial XML |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| EtherCAT terminal models in stc | stc is not a System Manager; terminal config is IDE concern | Model I/O image (byte arrays) only |
| `.library` binary parsing | Proprietary format, undocumented, changes between CODESYS versions | `.TcPOU` XML extraction or hand-written `.st` stubs |
| Complex mock framework in Go | Forces engineers to learn Go for testing | ST-based mocks using existing parser/interpreter |
| Address validation against hardware config | stc has no hardware config file | Trust AT addresses; validate format only |
| Allen Bradley tag-based addressing in stc | Fundamentally different paradigm from IEC %I/%Q/%M | AB stubs use standard IEC FUNCTION_BLOCK; no AT addresses |

## Sources

- [Beckhoff InfoSys: AT-Declaration](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/11948825611.html) -- AT syntax grammar, wildcard addressing (HIGH confidence)
- [Beckhoff InfoSys: Addresses](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/2529360523.html) -- %I/%Q/%M format, size prefixes (HIGH confidence)
- [Beckhoff InfoSys: Library creation](https://infosys.beckhoff.com/content/1033/tc3_plc_intro/4189255051.html) -- .library vs .compiled-library formats (MEDIUM confidence)
- [Beckhoff EL1008 product page](https://www.beckhoff.com/en-us/products/i-o/ethercat-terminals/el-ed1xxx-digital-input/el1008.html) -- 8-ch DI process data (HIGH confidence)
- [Beckhoff EL3064 product page](https://www.beckhoff.com/en-us/products/i-o/ethercat-terminals/el-ed3xxx-analog-input/el3064.html) -- 4-ch AI 0-10V 12-bit (HIGH confidence)
- [Rockwell Logix 5000 IEC 61131-3 Compliance (1756-PM018)](https://literature.rockwellautomation.com/idc/groups/literature/documents/pm/1756-pm018_-en-p.pdf) -- AB dialect restrictions (MEDIUM confidence, PDF partially accessible)
- [IEC 61131-3 Wikipedia](https://en.wikipedia.org/wiki/IEC_61131-3) -- Standard addressing overview (MEDIUM confidence)
- [Fernhill Software IEC 61131-3 Variable Declarations](https://www.fernhillsoftware.com/help/iec-61131/common-elements/variable-declaration.html) -- AT keyword syntax (MEDIUM confidence)
- [CODESYS Library Development](https://content.helpme-codesys.com/en/CODESYS%20Development%20System/_cds_library_development_information.html) -- Library file types (MEDIUM confidence)
- [PLCopen XML Exchange](https://www.plcopen.org/standards/xml-echange/) -- IEC 61131-10 format (MEDIUM confidence)
- [GitHub: Beckhoff-PLC-TwinCAT TcPOU example](https://github.com/a-m-shotorbani/Beckhoff-PLC-TwinCAT/blob/main/FB_myFileRead.TcPOU) -- TcPOU XML structure (HIGH confidence)
- [Solisplc: Add-On Instructions](https://www.solisplc.com/tutorials/add-on-instructions-programming-aoi-rslogix-studio-5000-plc-programming-tutorial-example-logic) -- AOI vs FUNCTION_BLOCK (MEDIUM confidence)

---
*Stack research for: Vendor Libraries, I/O Mapping & Mock Framework*
*Researched: 2026-03-30*
