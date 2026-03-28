# Phase 6: Simulation - Context

**Gathered:** 2026-03-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can run closed-loop simulations injecting sensor waveforms and defining plant models, all deterministic and replayable. Extends the scan cycle engine with plant model feedback loops.

</domain>

<decisions>
## Implementation Decisions

### Simulation Architecture
- `PlantModel` Go interface: `Update(inputs map[string]Value, dt Duration) map[string]Value` — simple, extensible
- 3 built-in plant models: `MotorModel` (inertia/speed), `ValveModel` (flow/position), `CylinderModel` (position/force)
- `WaveformGenerator` with Step, Ramp, Sine, Square patterns — configurable amplitude, frequency, offset
- Extends test runner pattern — simulation runs are deterministic and replayable for regression testing
- Simulations use the existing ScanCycleEngine with plant model outputs fed back as inputs each cycle

### Claude's Discretion
- Plant model parameter naming
- Waveform configuration format
- Internal simulation loop implementation
- Test fixture organization

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `pkg/interp/scan.go` — ScanCycleEngine with Tick(dt), I/O table
- `pkg/interp/interpreter.go` — Full interpreter
- `pkg/testing/runner.go` — Test runner pattern (discovery, execution, reporting)
- `pkg/interp/value.go` — Value types for I/O mapping

### Integration Points
- Plant models feed outputs back as scan cycle inputs
- Simulation results can use existing JUnit/JSON reporting
- `stc sim` or integrated into `stc test` with sim fixtures

</code_context>

<specifics>
## Specific Ideas

- All simulations must be deterministic — no randomness, no wall-clock
- Replayability is key — same config = same results every time

</specifics>

<deferred>
## Deferred Ideas

None

</deferred>
