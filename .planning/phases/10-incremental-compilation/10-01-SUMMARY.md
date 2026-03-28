---
phase: 10-incremental-compilation
plan: 01
subsystem: incremental
tags: [dependency-graph, file-cache, symbol-table, incremental]
dependency_graph:
  requires: []
  provides: [DepGraph, FileCache, PurgeFile, SymbolsByFile]
  affects: [analyzer, checker]
tech_stack:
  added: [crypto/sha256, encoding/json]
  patterns: [BFS transitive closure, content-addressed caching, disk index persistence]
key_files:
  created:
    - pkg/incremental/depgraph.go
    - pkg/incremental/depgraph_test.go
    - pkg/incremental/filecache.go
    - pkg/incremental/filecache_test.go
  modified:
    - pkg/symbols/table.go
    - pkg/symbols/table_test.go
decisions:
  - SHA-256 for content hashing (deterministic, fast, no collision risk)
  - Disk index stores hashes only, ASTs stay in memory per invocation
  - Case-insensitive POU matching via strings.ToUpper for IEC 61131-3 compliance
metrics:
  duration: 228s
  completed: "2026-03-28T21:06:39Z"
---

# Phase 10 Plan 01: Incremental Compilation Infrastructure Summary

File-level dependency graph with BFS transitive closure, per-file symbol purge on the symbol table, and on-disk file cache with SHA-256 content hashing for cross-invocation reuse.

## What Was Built

### DepGraph (pkg/incremental/depgraph.go)
- `NewDepGraph()` creates an empty file-level dependency graph
- `AddFile(filename, declares, references)` stores POU declarations and references per file with case-insensitive name matching
- `RemoveFile(filename)` removes a file and its edges from the graph
- `Dependents(filename)` returns files that reference POUs declared in the given file
- `AllDirty(changed)` computes transitive closure via BFS -- finds all files affected by changes
- `ScanFile(file, filename)` extracts POU names and type references from AST, filtering out elementary types

### FileCache (pkg/incremental/filecache.go)
- `ContentHash(content)` returns hex-encoded SHA-256 of file content
- `NewFileCache()` creates an in-memory cache keyed by filename
- `Store/Load/Remove` manage cached ParseResults
- `IsStale(filename, hash)` detects changed files by hash comparison
- `NeedsParse(filename)` identifies entries loaded from disk index that need re-parsing
- `SaveIndex(dir)` writes `.stc-cache/index.json` with filename-to-hash mappings
- `LoadIndex(dir)` restores hash index from disk (ParseResults nil until re-parsed)

### Symbol Table Extensions (pkg/symbols/table.go)
- `PurgeFile(filename)` removes all symbols from global scope where Pos.File matches, cleans POU registry, removes child scopes, and updates file list
- `SymbolsByFile(filename)` returns all global-scope symbols declared in a file

## Test Coverage

- 6 DepGraph tests: AddFile, Dependents, AllDirty transitive closure, case-insensitive matching, replace, remove
- 8 FileCache tests: ContentHash, Store/Load, IsStale, Remove, Files, disk index round-trip, NeedsParse, missing index
- 3 Symbol table tests: PurgeFile, PurgeFile child scope cleanup, SymbolsByFile

All 17 new tests pass. Zero regressions in analyzer, checker, and symbols packages.

## Deviations from Plan

None - plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 (RED) | aeb0e81 | Failing tests for DepGraph and per-file symbol purge |
| 1 (GREEN) | f949085 | Implement DepGraph and per-file symbol purge |
| 2 (RED) | 9f2b86b | Failing tests for FileCache and content hashing |
| 2 (GREEN) | b4bf077 | Implement FileCache with disk persistence |

## Known Stubs

None - all functionality is fully wired and tested.

## Self-Check: PASSED
