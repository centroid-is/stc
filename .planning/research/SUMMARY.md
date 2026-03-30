# Research Summary: v1.1 Vendor Libraries, I/O Mapping & Mock Framework

**Domain:** IEC 61131-3 vendor library integration for ST compiler toolchain
**Researched:** 2026-03-30
**Overall confidence:** MEDIUM-HIGH

## Executive Summary

The v1.1 milestone -- vendor library stubs, I/O address mapping, and mock framework -- is architecturally straightforward because stc v1.0 already contains all the foundational infrastructure. The parser handles AT addresses and empty-body FUNCTION_BLOCKs. The checker has vendor profiles and symbol registration. The interpreter has FB instance factories and scan cycle engines. What is missing is the glue: loading stub files before user code, mapping AT addresses to a byte-level I/O table, and auto-generating zero-value mock instances for body-less FBs.

The I/O memory model follows the IEC 61131-3 standard directly: three flat byte arrays (%I, %Q, %M) with typed access at bit, byte, word, and double-word granularity. This is exactly how real PLCs implement their I/O image -- a contiguous byte buffer that hardware terminals write to and PLC programs read from. stc does not need to model specific EtherCAT terminals; it only needs to model the I/O image and resolve AT addresses to offsets within it.

Vendor library extraction is feasible through TwinCAT `.TcPOU` XML files, which contain the ST declaration in a CDATA block. Go's `encoding/xml` handles this trivially. The proprietary `.library` and `.compiled-library` formats from CODESYS/Beckhoff should NOT be parsed -- they are undocumented binary formats that change between versions. The recommended primary path is hand-written `.st` stub files (the TypeScript `.d.ts` analogy), with TcPOU extraction as a convenience tool.

Allen Bradley requires the most careful handling. AB uses Add-On Instructions (AOIs) instead of FUNCTION_BLOCKs, has no ENUM type, no OOP, no POINTER/REFERENCE, no AT addressing (tag-based instead), and different timer types. AB stubs should still be written as IEC 61131-3 FUNCTION_BLOCKs for stc's checker, with the emitter handling the AOI translation. AB support is deferred to a later milestone per PROJECT.md, but the stub format should be designed to accommodate it now.

## Key Findings

**Stack:** Zero new Go dependencies needed. `encoding/xml` (stdlib) for TcPOU extraction, everything else uses existing stc infrastructure.
**Architecture:** IOTable with three flat byte arrays (%I, %Q, %M), AT address parser, vendor stub loader in checker pass 1, auto-stub FBs in interpreter.
**Critical pitfall:** Do NOT try to parse `.library` files. Do NOT model EtherCAT terminals. Do NOT implement AB tag-based addressing -- use standard IEC addressing for stubs.

## Implications for Roadmap

Based on research, suggested phase structure:

1. **I/O Address Parser & Table** - Foundation for everything else
   - Addresses: AT address parsing (`%IX0.0`, `%QW4`, `%MD12`, `%I*`), IOTable data structure
   - Avoids: Over-engineering terminal models
   - Rationale: Other features depend on I/O mapping

2. **Vendor Stub Loading** - Enables type-checking of vendor FB code
   - Addresses: `.st` stub file parsing, symbol table registration before user code, `stc.toml` library_paths
   - Avoids: Proprietary format parsing
   - Rationale: Type-checking is prerequisite for testing and simulation

3. **Mock Framework** - Enables host-based testing with vendor FBs
   - Addresses: Auto-stub zero-value FBs, `[test.mock_paths]` config, mock override loading
   - Avoids: Go-based mock API (keeps everything in ST)
   - Rationale: Depends on stub loading being complete

4. **Shipped Stubs & TcPOU Extractor** - Convenience and completeness
   - Addresses: Top-20 Beckhoff + top-10 Schneider stubs, `stc vendor extract` command
   - Avoids: Community repository (v2 feature)
   - Rationale: Polish, can be done in parallel once framework is solid

5. **STC_TEST/STC_SIM Auto-Define** - Small integration feature
   - Addresses: Auto-define preprocessor symbols in `stc test` and `stc sim` commands
   - Avoids: Nothing -- this is trivial
   - Rationale: Enables conditional compilation for test/production code paths

**Phase ordering rationale:**
- I/O table must exist before AT addresses can be resolved in the interpreter
- Stub loading must work before mocks can override stubs
- Mock framework depends on both I/O table and stub loading
- Shipped stubs and extractor are independent of each other, parallel with mock framework

**Research flags for phases:**
- Phase 1: Standard pattern, unlikely to need further research
- Phase 2: Standard pattern, existing `VENDOR_LIBRARIES.md` has full design
- Phase 3: Mock override precedence rules need careful implementation but design is clear
- Phase 4: Beckhoff FB signatures need verification against Infosys docs during stub writing
- Phase 5: Trivial -- one-line change in `stc test` and `stc sim` command setup

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| I/O Memory Model | HIGH | IEC 61131-3 standard is clear; Beckhoff InfoSys confirms |
| AT Keyword Semantics | HIGH | Standard syntax, verified with Beckhoff docs |
| Library File Formats | MEDIUM | TcPOU XML verified; .library binary format confirmed as undocumented |
| Allen Bradley Differences | MEDIUM | Multiple sources agree on major gaps; minor details may be incomplete |
| Mock Framework Design | HIGH | Design validated against existing stc architecture in VENDOR_LIBRARIES.md |
| EtherCAT Terminal Mapping | MEDIUM | Product docs show process data; exact byte offsets vendor-configured |

## Gaps to Address

- Allen Bradley timer instruction details (TONR vs TON semantics differences) -- needed when writing AB stubs
- Schneider-specific FB signatures (READ_VAR, WRITE_VAR parameter details) -- needed when writing Schneider stubs
- PLCopen XML import as alternative stub source -- deferred to future milestone
- CODESYS scripting API for `.library` export -- documented but requires CODESYS IDE, not suitable for stc
- AB v35+ firmware may have added ENUM support -- needs verification when AB milestone starts
