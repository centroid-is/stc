# Phase 22 Deferred Items

## From 22-02

- **Method call arguments are not type-checked.** `fb.M(w := 'x')` with a WORD input checks clean (pre-existing, found while writing TestUntypedLiteral). Function and FB-call arguments go through `bindCallArgs`/`checkInputArg`; method calls do not. Out of scope for literal typing.
- **Bitwise AND/OR/XOR on ANY_INT operands** (`UINT AND UINT`, 5 oracle errors) still require BOOL. 22-02 enabled bitwise only for BYTE..LWORD (IEC ANY_BIT). CODESYS also allows integers; research Open Question 1 keeps this out of Phase 22.
