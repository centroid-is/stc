# ST Language Support

What IEC 61131-3 Structured Text features stc supports, including CODESYS extensions and vendor dialect notes.

## Feature Matrix

### Declarations

| Feature | Supported | Notes |
|---------|-----------|-------|
| PROGRAM | Yes | |
| FUNCTION_BLOCK | Yes | Including EXTENDS |
| FUNCTION | Yes | With return type |
| TYPE (struct) | Yes | STRUCT ... END_STRUCT |
| TYPE (enum) | Yes | Named and typed enumerations |
| TYPE (alias) | Yes | Simple type aliases |
| TYPE (subrange) | Yes | e.g., `INT(0..100)` |
| TYPE (array) | Yes | Single and multi-dimensional |
| INTERFACE | Yes | CODESYS OOP extension |
| METHOD | Yes | On FUNCTION_BLOCK, with access specifiers |
| PROPERTY | Yes | GET/SET accessors |
| ACTION | Yes | Named actions on PROGRAMs and FBs |
| NAMESPACE | Partial | Parsed but not semantically enforced |
| GLOBAL_VAR (GVL) | Yes | VAR_GLOBAL declarations |

### Variable Sections

| Section | Supported | Notes |
|---------|-----------|-------|
| VAR | Yes | Local variables |
| VAR_INPUT | Yes | |
| VAR_OUTPUT | Yes | |
| VAR_IN_OUT | Yes | |
| VAR_GLOBAL | Yes | |
| VAR_TEMP | Yes | |
| VAR_EXTERNAL | Yes | |
| VAR CONSTANT | Yes | |
| VAR RETAIN | Yes | |
| VAR PERSISTENT | Yes | |
| AT address | Yes | `%IX0.0`, `%QW0`, `%MW0`, `%MD0`, etc. |

### Data Types

| Type | Supported | Notes |
|------|-----------|-------|
| BOOL | Yes | |
| BYTE, WORD, DWORD, LWORD | Yes | |
| SINT, INT, DINT, LINT | Yes | LINT is a CODESYS extension |
| USINT, UINT, UDINT, ULINT | Yes | ULINT is a CODESYS extension |
| REAL, LREAL | Yes | |
| STRING | Yes | With optional length `STRING(80)` |
| WSTRING | Partial | Executes; string functions count UTF-8 bytes, so non-ASCII characters count as more than one |
| TIME, DATE, TIME_OF_DAY, DATE_AND_TIME | Yes | |
| LTIME, LDATE, LTOD, LDT | Yes | CODESYS 64-bit time extensions |
| ARRAY | Yes | Single and multi-dimensional, variable-length |
| POINTER TO | Yes | CODESYS OOP extension |
| REFERENCE TO | Yes | CODESYS OOP extension |

### Literals

| Literal | Supported | Example |
|---------|-----------|---------|
| Integer | Yes | `42`, `16#FF`, `8#77`, `2#1010` |
| Real | Yes | `3.14`, `1.0E+3` |
| Boolean | Yes | `TRUE`, `FALSE` |
| String | Yes | `'hello'`, `"hello"` |
| Time | Yes | `T#1s500ms`, `T#2h30m` |
| Date | Yes | `D#2024-01-15` |
| Time of day | Yes | `TOD#14:30:00` |
| Date and time | Yes | `DT#2024-01-15-14:30:00` |
| Typed literal | Yes | `INT#42`, `REAL#3.14` |

### Statements

| Statement | Supported | Notes |
|-----------|-----------|-------|
| Assignment `:=` | Yes | |
| IF / ELSIF / ELSE / END_IF | Yes | |
| CASE / OF / END_CASE | Yes | With integer and enum selectors |
| FOR / TO / BY / DO / END_FOR | Yes | |
| WHILE / DO / END_WHILE | Yes | |
| REPEAT / UNTIL / END_REPEAT | Yes | |
| RETURN | Yes | |
| EXIT | Yes | Loop exit |
| CONTINUE | Yes | Loop continue |
| Function call | Yes | Named and positional parameters |
| FB call | Yes | `fb(Input1 := val, ...)` syntax |
| Method call | Yes | `fb.Method(...)` |
| Member access | Yes | `fb.Output` |
| Array indexing | Yes | `arr[i]`, `arr[i, j]` |

### Expressions

| Expression | Supported | Notes |
|------------|-----------|-------|
| Arithmetic | Yes | `+`, `-`, `*`, `/`, `MOD` |
| Comparison | Yes | `=`, `<>`, `<`, `>`, `<=`, `>=` |
| Logical | Yes | `AND`, `OR`, `XOR`, `NOT` |
| Bitwise | Yes | `AND`, `OR`, `XOR`, `NOT` on integer types |
| Parenthesized | Yes | `(expr)` |
| Unary | Yes | `-`, `NOT` |
| Exponentiation | Yes | `**` |

### Preprocessor Directives

| Directive | Supported | Notes |
|-----------|-----------|-------|
| `{IF defined(X)}` | Yes | Conditional compilation |
| `{IF NOT defined(X)}` | Yes | |
| `{ELSIF defined(X)}` | Yes | |
| `{ELSE}` | Yes | |
| `{END_IF}` | Yes | |
| `{DEFINE X}` | Yes | Define a symbol |
| `{ERROR "msg"}` | Yes | Compilation error directive |

### Pragmas

| Pragma | Supported | Notes |
|--------|-----------|-------|
| `{attribute 'name' := 'value'}` | Yes | Preserved on the declaration and re-emitted. `TcLinkTo` binds EtherCAT I/O, `OPC.UA.DA*` decides OPC UA exposure, `qualified_only` is enforced. See [TWINCAT_IMPORT.md](TWINCAT_IMPORT.md) |
| `{warning 'msg'}` | Parsed | |

### OOP Extensions (CODESYS)

| Feature | Supported | Notes |
|---------|-----------|-------|
| INTERFACE declaration | Yes | |
| IMPLEMENTS | Yes | On FUNCTION_BLOCK |
| EXTENDS | Yes | Single inheritance for FBs |
| METHOD with access specifier | Yes | PUBLIC, PRIVATE, PROTECTED, INTERNAL |
| PROPERTY with GET/SET | Yes | |
| POINTER TO | Yes | Parsed, type-checked and executed (ADR, `^`) |
| REFERENCE TO | Yes | Parsed, type-checked and executed (REF=) |
| THIS | Yes | `THIS^` member and method access executes |
| SUPER | Yes | `SUPER^` calls the base implementation, also from inherited code |

## IEC 61131-3 Ed.3 Compliance

stc targets IEC 61131-3 Edition 3 (2013) for Structured Text. The parser and type checker cover the full ST language as defined in the standard, with the following notes:

- **Full compliance**: All data types, variable sections, statements, expressions, and POU declarations defined in Ed.3 are parsed and type-checked.
- **Standard library**: All standard function blocks (timers, counters, edge detectors, bistables) and standard functions (math, string, conversion) are implemented per Ed.3 semantics.
- **Not implemented**: Ladder Diagram (LD), Function Block Diagram (FBD), Instruction List (IL), and Sequential Function Chart (SFC) are out of scope -- stc handles ST only.

## CODESYS Extensions

stc supports the following CODESYS extensions that go beyond the IEC 61131-3 standard:

- **OOP**: INTERFACE, EXTENDS, IMPLEMENTS, METHOD with access specifiers, PROPERTY with GET/SET
- **64-bit types**: LINT, ULINT, LWORD, LREAL, LTIME, LDATE, LTOD, LDT
- **Pointers and references**: POINTER TO, REFERENCE TO, REF= operator
- **String types**: WSTRING
- **Variable-length arrays**: ARRAY[*] OF type

These extensions are required to parse real-world Beckhoff TwinCAT 3 and Schneider EcoStruxure projects.

## Vendor Dialect Notes

### Beckhoff TwinCAT 3

Beckhoff uses a full CODESYS-derived ST dialect with all OOP extensions enabled. stc's `beckhoff` vendor profile allows all language features. The emitter produces Beckhoff-compatible attribute syntax. stc reads TwinCAT `.tsproj` and `.plcproj` projects directly, including `AT %I*` wildcards, `qualified_only` GVLs, bit access and actions; see [TWINCAT_IMPORT.md](TWINCAT_IMPORT.md).

### Schneider EcoStruxure

Schneider uses a CODESYS-derived dialect but with restrictions. The `schneider` vendor profile warns on:
- POINTER TO
- REFERENCE TO
- OOP features (INTERFACE, EXTENDS, IMPLEMENTS, METHOD, PROPERTY)

### Allen Bradley (Studio 5000)

Allen Bradley uses a highly restricted ST dialect. The `portable` profile (which also serves as a proxy for AB restrictions) warns on:
- All CODESYS OOP features
- POINTER TO, REFERENCE TO
- 64-bit types (LINT, ULINT, LWORD, LTIME)

Allen Bradley support is currently limited to type-checking stubs (timers: TONR, TOFR, RTO; common instructions). Full AB emission is deferred to a future version.

## Known Limitations

- **No native code generation**: stc does not produce PLC runtime binaries. It analyses, tests and simulates ST on the host, and `stc emit` re-emits vendor ST for the vendor IDE. `stc sim` and `stc serve` run whole projects in the interpreter for development, not as a production runtime.
- **Interpreter performance**: The tree-walking interpreter is fast enough for tests and simulation but not for hard real-time. One scan of the ST301 production project takes about 8 ms, so serve it with `--cycle 10ms` instead of its 1 ms task.
- **WSTRING**: Values execute, but LEN, LEFT, RIGHT, MID and FIND count UTF-8 bytes. Results differ from TwinCAT for non-ASCII text.
- **SFC**: Sequential Function Chart is not supported (ST-only toolchain).
- **Cross-reference completeness**: Some complex OOP scenarios (multiple inheritance via interfaces, virtual method dispatch) may have gaps in the checker.

## Standard Library Functions

### Math Functions

ABS, SQRT, SIN, COS, TAN, ASIN, ACOS, ATAN, LN, LOG, EXP, MIN, MAX, LIMIT, SEL, MUX, MOVE

### String Functions

LEN, LEFT, RIGHT, MID, CONCAT, FIND

Note: String indexing is 1-based per IEC 61131-3.

### Type Conversion Functions

INT_TO_REAL, REAL_TO_INT, BOOL_TO_INT, INT_TO_BOOL, INT_TO_STRING, STRING_TO_INT, and other `TYPE_TO_TYPE` conversions.

REAL_TO_INT uses banker's rounding (round half to even) per IEC standard.

### Standard Function Blocks

| FB | Description |
|----|-------------|
| TON | On-delay timer |
| TOF | Off-delay timer |
| TP | Pulse timer |
| CTU | Up counter |
| CTD | Down counter |
| CTUD | Up/down counter |
| R_TRIG | Rising edge detector |
| F_TRIG | Falling edge detector |
| SR | Set-dominant bistable |
| RS | Reset-dominant bistable |
