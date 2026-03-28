---
phase: 11-mcp-server-claude-code-skills
plan: 02
subsystem: tooling
tags: [claude-code, skills, structured-text, cli, developer-experience]

requires:
  - phase: 08-linter-formatter
    provides: stc lint CLI command for automated code review
  - phase: 07-emitter-vendor
    provides: stc emit CLI command for vendor-specific emission
provides:
  - Claude Code skill index with auto-invoke for .st files
  - 5 workflow skills covering generate, validate, test, emit, review
affects: []

tech-stack:
  added: []
  patterns:
    - Claude Code SKILL.md index with auto-invoke trigger pattern
    - Self-contained skill files with CLI commands, expected output, and error handling

key-files:
  created:
    - .claude/skills/SKILL.md
    - .claude/skills/st-generate.md
    - .claude/skills/st-validate.md
    - .claude/skills/st-test.md
    - .claude/skills/st-emit.md
    - .claude/skills/st-review.md
  modified: []

key-decisions:
  - "Skills are self-contained markdown files with no cross-skill dependencies"
  - "Auto-invoke triggered by *.st file pattern"
  - "All skills reference --format json for machine-readable output"

patterns-established:
  - "Skill structure: purpose, when to use, workflow steps, expected JSON output, error handling"
  - "Validate-first pattern: all skills recommend running stc parse/check before their primary action"

requirements-completed: [SKIL-01, SKIL-02, SKIL-03, SKIL-04, SKIL-05, SKIL-06]

duration: 3min
completed: 2026-03-28
---

# Phase 11 Plan 02: Claude Code Skills Summary

**5 ST workflow skills for Claude Code with auto-invoke on .st files covering generate, validate, test, emit, and review workflows**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-28T21:23:34Z
- **Completed:** 2026-03-28T21:26:28Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- SKILL.md index with auto-invoke pattern for *.st files and all 5 skills listed
- st-generate skill with IEC 61131-3 code generation workflow and standard library reference
- st-validate skill chaining parse + check + lint pipeline with severity interpretation
- st-test skill covering TEST_CASE structure, assertions, ADVANCE_TIME, and I/O mocking
- st-emit skill documenting all 3 vendor targets (beckhoff, schneider, portable)
- st-review skill combining automated lint with manual review checklist

## Task Commits

Each task was committed atomically:

1. **Task 1: SKILL.md index and auto-invoke configuration** - `fe33b8e` (feat)
2. **Task 2: Create all 5 workflow skill files** - `063cfcd` (feat)

## Files Created/Modified
- `.claude/skills/SKILL.md` - Skill index with auto-invoke pattern and skill inventory
- `.claude/skills/st-generate.md` - ST code generation skill with standard library reference
- `.claude/skills/st-validate.md` - Full parse+check+lint validation pipeline
- `.claude/skills/st-test.md` - Unit testing with assertions and time simulation
- `.claude/skills/st-emit.md` - Vendor-specific emission for beckhoff/schneider/portable
- `.claude/skills/st-review.md` - Automated lint plus manual review checklist

## Decisions Made
- Skills are self-contained markdown files with no cross-skill dependencies required
- Auto-invoke triggered by *.st file pattern in SKILL.md
- All skills reference --format json for machine-readable stc CLI output
- Validate-first pattern: all skills recommend parsing/checking before their primary action

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- All Claude Code skills complete for ST development workflows
- Skills ready for use by Claude Code when working with .st files in this project

---
*Phase: 11-mcp-server-claude-code-skills*
*Completed: 2026-03-28*
