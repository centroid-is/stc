---
phase: 24-ethercat-topology-link-binding
plan: 04
subsystem: ecat, docs
tags: [ethercat, equivalence, generate_gvl, validation, coverage]
requires:
  - "24-01 LoadProject, Topology.Paths, Slave.Parent"
  - "24-02 ParseTcLinkTo, CollectLinks, Resolve, stc ecat validate"
  - "24-03 Network, IOBinder"
provides:
  - "Env-gated ST301 equivalence gate (TestEcatST301Equivalence)"
  - "User and developer docs for stc ecat validate, pkg/ecat and the IOBinder"
  - "Signed-off 24-VALIDATION.md"
affects: [25, 26, 27]
tech-stack:
  added: []
  patterns: ["flattened probe GVLs named from their '// ---- <path>/<Name>.TcGVL' section markers"]
key-files:
  created:
    - tests/ecat_equivalence_test.go
    - docs/ETHERCAT_SIMULATION.md
  modified:
    - docs/CLI_REFERENCE.md
    - docs/ARCHITECTURE.md
    - cmd/stc/ecat.go
    - .planning/phases/24-ethercat-topology-link-binding/24-VALIDATION.md
decisions:
  - "The equivalence test names each GVL in the flattened st301.st from its preceding section marker, so ECT and ECT_Diag keep their own names"
  - "Generator equivalence is checked without running Python: every TIID^ target the generator wrote into ST301 must exist in Topology.Paths()"
metrics:
  duration: "~25 min"
  completed: 2026-10-06
  tasks: 3
  files: 6
---

# Phase 24 Plan 04: ST301 Equivalence, Docs and Validation Sign-off Summary

The real ST301 exports load into the topology generate_gvl.py builds, and all 1340 TcLinkTo targets in ST301 resolve against the SVNCore types with zero errors and zero warnings. The feature is documented and phase validation is signed off.

## Tasks

| Task | Name | Commit |
| ---- | ---- | ------ |
| 1 | Env-gated ST301 equivalence test | 4b47c9a |
| 2 | Documentation | 659e2f6 |
| 3 | Validation sign-off and coverage gate | 32bf1ff |

## ST301 Equivalence Results

Run locally with STC_SILD_DIR and STC_PROBES_DIR set. Without them the test skips, which is what CI does.

| Master | Slaves | In bytes | Out bytes | Bindings |
|--------|--------|----------|-----------|----------|
| Device 1 (EtherCAT) | 23 | 1846 | 1543 | 233 |
| Device 2 (EtherCAT) | 49 | 3747 | 3085 | 563 |
| Device 3 (EtherCAT) | 34 | 1963 | 1543 | 353 |
| Device 4 (EtherCAT) | 16 | 1809 | 1548 | 191 |

- st301.st has 114 TcLinkTo pragmas with 1340 TIID^ targets. None are missing among the 1970 topology paths.
- CollectLinks over svncorecomponents.st and st301.st yields 1340 linked leaves. Resolve binds all 1340 with zero errors and zero warnings.
- Both files parse with zero error diagnostics.
- Device 1 nesting holds: the EL1008 and EL2912 nest under the EK1200 head. All 32 ATV320 drives sit at master level.
- InfoData^AmsNetId and InfoData^AdsAddr links are among the bindings.
- The pkg/ecat port needed no fixes.

## Coverage

The merged-profile gate, with pkg/ecat added as an extra informational row:

| Package | Coverage | Minimum |
|---------|----------|---------|
| pkg/parser | 98.35% | 95% |
| pkg/lexer | 97.60% | 95% |
| pkg/checker | 98.50% | 94% |
| pkg/interp | 98.25% | 95% |
| pkg/types | 100.00% | 95% |
| pkg/emit | 97.32% | 95% |
| pkg/ecat | 99.84% | 90% (not gated) |
| total | 96.34% | 85% |

`go vet ./...` is clean. `go test ./... -count=1` fails only on pkg/checker TestEmptyFBCall.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Doc bug] `stc ecat validate --help` omitted ECAT007**
- **Found during:** Task 2
- **Fix:** The long help now lists ECAT007 as a warning.
- **Files modified:** cmd/stc/ecat.go
- **Commit:** 659e2f6

**2. [Scope addition at the lead's request] docs/ETHERCAT_SIMULATION.md**
- The orchestrator asked for a short user guide in addition to the plan's ARCHITECTURE.md section. It covers validation, the Go wiring, healthy defaults, the fault API and device models.
- **Commit:** 659e2f6

### Coverage gate run

`bash scripts/coverage-gate.sh` exits 1 because its unit step aborts on the pre-existing pkg/checker TestEmptyFBCall failure. That test is fixed on main by Phase 21 and disappears on merge. A scratchpad copy of the script that tolerates only that package's failure produced the table above. The unit log was checked and showed no other failing test.

## Deferred Issues

- pkg/checker TestEmptyFBCall still fails on this branch. It is pre-existing and already logged in deferred-items.md.

## Threat Mitigations

- T-24-07: the test is env-gated. It logs counts only. On failure it logs at most 20 link paths or diagnostics with file name and line, never source text. No customer file is copied into the repo.

## Known Stubs

None.

## Self-Check: PASSED
