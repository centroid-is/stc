# Open-Source ST Parse Corpus

Real-world IEC 61131-3 / Structured Text files collected from permissively-licensed
open-source projects. Used as a parse-fuzz corpus to exercise the `stc` parser
against code it has never seen before.

## Sources

### structured-text-utilities (MIT)
- Repository: https://github.com/WengerAG/structured-text-utilities
- License: MIT
- Notes: A collection of utility function blocks written in idiomatic
  Structured Text — arrays, math, strings, time, byte manipulation.

### agents4plc (Apache-2.0)
- Repository: https://github.com/Luoji-zju/Agents4PLC_release
- License: Apache-2.0
- Notes: AI-generated Structured Text benchmark programs for PLC control
  tasks (multi-pump control, special stack).

## Excluded Sources

### echidna (BSD-2-Clause)
- Repository: https://github.com/61131/echidna
- Excluded because the test files use Instruction List (IL) syntax, not
  Structured Text. The parser does not support IL and hangs on these files.
