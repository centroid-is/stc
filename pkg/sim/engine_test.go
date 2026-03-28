package sim

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
)

// parseProgram parses ST source and extracts the first ProgramDecl.
func parseProgram(t *testing.T, src string) *ast.ProgramDecl {
	t.Helper()
	result := parser.Parse("test.st", src)
	for _, d := range result.File.Declarations {
		if p, ok := d.(*ast.ProgramDecl); ok {
			return p
		}
	}
	t.Fatal("no ProgramDecl found")
	return nil
}

// motorProgram is a simple ST program that reads RUN (BOOL) input and writes
// it to MOTOR_RUN (BOOL) output. SPEED input is fed back from plant model.
const motorProgram = `PROGRAM MotorSim
VAR_INPUT
    RUN : BOOL;
    SPEED : REAL;
END_VAR
VAR_OUTPUT
    MOTOR_RUN : BOOL;
    CURRENT_SPEED : REAL;
END_VAR
    MOTOR_RUN := RUN;
    CURRENT_SPEED := SPEED;
END_PROGRAM
`

func TestEngine_ClosedLoop_MotorSpeedIncreases(t *testing.T) {
	prog := parseProgram(t, motorProgram)

	// Step waveform: RUN goes TRUE at t=0 (no delay)
	runWave := NewWaveformGenerator(WaveformConfig{
		Kind:      WaveStep,
		Amplitude: 1.0, // BOOL TRUE = any non-zero
		Delay:     0,
	})

	motor := NewMotorModel(MotorConfig{
		MaxSpeed: 1000.0,
		RampTime: 1 * time.Second,
	})

	cfg := SimConfig{
		Program:   prog,
		NumCycles: 50,
		CycleDt:   10 * time.Millisecond,
		Waveforms: []WaveformBinding{
			{InputName: "RUN", Generator: runWave},
		},
		Plants: []PlantBinding{
			{
				Model: motor,
				OutputToPlant: map[string]string{
					"MOTOR_RUN": "RUN",
				},
				PlantToInput: map[string]string{
					"SPEED": "SPEED",
				},
			},
		},
	}

	engine := NewSimulationEngine(cfg)
	res, err := engine.Run()
	if err != nil {
		t.Fatalf("engine.Run() error: %v", err)
	}

	if res.NumCycles != 50 {
		t.Errorf("expected 50 cycles, got %d", res.NumCycles)
	}

	// Motor speed should increase over 50 cycles at 10ms each (500ms total).
	// Rate = 1000 RPM / 1s = 1000 RPM/s.
	// After 500ms: speed should be ~500 RPM.
	first := res.Cycles[0]
	last := res.Cycles[len(res.Cycles)-1]

	firstSpeed := first.Outputs["CURRENT_SPEED"]
	lastSpeed := last.Outputs["CURRENT_SPEED"]

	// First cycle: motor hasn't had feedback yet, speed should be 0 or very small
	if firstSpeed.Real > 100 {
		t.Errorf("first cycle speed too high: %v", firstSpeed)
	}

	// Last cycle: speed should be significantly higher (near 490 RPM)
	if lastSpeed.Real < 100 {
		t.Errorf("last cycle speed too low: %v (expected ~490 RPM)", lastSpeed)
	}
}

func TestEngine_Determinism(t *testing.T) {
	prog := parseProgram(t, motorProgram)

	makeCfg := func() SimConfig {
		return SimConfig{
			Program:   prog,
			NumCycles: 20,
			CycleDt:   10 * time.Millisecond,
			Waveforms: []WaveformBinding{
				{
					InputName: "RUN",
					Generator: NewWaveformGenerator(WaveformConfig{
						Kind:      WaveStep,
						Amplitude: 1.0,
						Delay:     0,
					}),
				},
			},
			Plants: []PlantBinding{
				{
					Model: NewMotorModel(MotorConfig{
						MaxSpeed: 1000.0,
						RampTime: 1 * time.Second,
					}),
					OutputToPlant: map[string]string{
						"MOTOR_RUN": "RUN",
					},
					PlantToInput: map[string]string{
						"SPEED": "SPEED",
					},
				},
			},
		}
	}

	engine1 := NewSimulationEngine(makeCfg())
	res1, err := engine1.Run()
	if err != nil {
		t.Fatalf("run 1 error: %v", err)
	}

	engine2 := NewSimulationEngine(makeCfg())
	res2, err := engine2.Run()
	if err != nil {
		t.Fatalf("run 2 error: %v", err)
	}

	if len(res1.Cycles) != len(res2.Cycles) {
		t.Fatalf("cycle count mismatch: %d vs %d", len(res1.Cycles), len(res2.Cycles))
	}

	for i, c1 := range res1.Cycles {
		c2 := res2.Cycles[i]
		for key, v1 := range c1.Outputs {
			v2, ok := c2.Outputs[key]
			if !ok {
				t.Errorf("cycle %d: output %s missing in run 2", i, key)
				continue
			}
			if v1.Real != v2.Real || v1.Int != v2.Int || v1.Bool != v2.Bool {
				t.Errorf("cycle %d output %s: run1=%v run2=%v (not deterministic)", i, key, v1, v2)
			}
		}
	}
}

func TestEngine_WaveformOnly_NoPlant(t *testing.T) {
	src := `PROGRAM WaveTest
VAR_INPUT
    SENSOR : REAL;
END_VAR
VAR_OUTPUT
    OUTPUT : REAL;
END_VAR
    OUTPUT := SENSOR * 2.0;
END_PROGRAM
`
	prog := parseProgram(t, src)

	cfg := SimConfig{
		Program:   prog,
		NumCycles: 10,
		CycleDt:   100 * time.Millisecond,
		Waveforms: []WaveformBinding{
			{
				InputName: "SENSOR",
				Generator: NewWaveformGenerator(WaveformConfig{
					Kind:      WaveSine,
					Amplitude: 50.0,
					Frequency: 1.0,
				}),
			},
		},
	}

	engine := NewSimulationEngine(cfg)
	res, err := engine.Run()
	if err != nil {
		t.Fatalf("engine.Run() error: %v", err)
	}

	if res.NumCycles != 10 {
		t.Errorf("expected 10 cycles, got %d", res.NumCycles)
	}

	// At t=0, sine = 0. At t=100ms (cycle 1), sine > 0.
	// Output should be sensor * 2.
	for _, c := range res.Cycles {
		out, ok := c.Outputs["OUTPUT"]
		if !ok {
			t.Errorf("cycle %d: OUTPUT not found", c.Cycle)
			continue
		}
		sensor, ok := c.Inputs["SENSOR"]
		if !ok {
			t.Errorf("cycle %d: SENSOR input not recorded", c.Cycle)
			continue
		}
		expected := sensor.Real * 2.0
		if out.Real != expected {
			t.Errorf("cycle %d: OUTPUT=%v, expected %v (SENSOR=%v * 2)", c.Cycle, out.Real, expected, sensor.Real)
		}
	}
}

func TestEngine_RecordInterval(t *testing.T) {
	src := `PROGRAM Simple
VAR_INPUT
    X : REAL;
END_VAR
VAR_OUTPUT
    Y : REAL;
END_VAR
    Y := X;
END_PROGRAM
`
	prog := parseProgram(t, src)

	cfg := SimConfig{
		Program:        prog,
		NumCycles:      100,
		CycleDt:        1 * time.Millisecond,
		RecordInterval: 10,
		Waveforms: []WaveformBinding{
			{
				InputName: "X",
				Generator: NewWaveformGenerator(WaveformConfig{
					Kind:      WaveRamp,
					Amplitude: 100.0,
					Frequency: 1.0,
				}),
			},
		},
	}

	engine := NewSimulationEngine(cfg)
	res, err := engine.Run()
	if err != nil {
		t.Fatalf("engine.Run() error: %v", err)
	}

	// 100 cycles with record every 10 = 10 recorded cycles
	if len(res.Cycles) != 10 {
		t.Errorf("expected 10 recorded cycles (interval=10), got %d", len(res.Cycles))
	}

	// Verify recorded cycle numbers
	for i, c := range res.Cycles {
		expected := i * 10
		if c.Cycle != expected {
			t.Errorf("recorded cycle %d: expected cycle number %d, got %d", i, expected, c.Cycle)
		}
	}
}
