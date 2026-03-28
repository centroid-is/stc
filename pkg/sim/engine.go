package sim

import (
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/interp"
)

// WaveformBinding maps a waveform generator to a program input variable.
type WaveformBinding struct {
	InputName string
	Generator *WaveformGenerator
}

// PlantBinding connects a plant model to program outputs and inputs for
// closed-loop simulation. OutputToPlant maps program output names to plant
// input names. PlantToInput maps plant output names to program input names.
type PlantBinding struct {
	Model         PlantModel
	OutputToPlant map[string]string // program output name -> plant input name
	PlantToInput  map[string]string // plant output name -> program input name
}

// SimConfig configures a simulation run.
type SimConfig struct {
	Program        *ast.ProgramDecl // The ST program to simulate
	NumCycles      int              // Number of scan cycles to run
	CycleDt        time.Duration    // Time step per cycle (e.g., 10ms)
	Waveforms      []WaveformBinding
	Plants         []PlantBinding
	RecordInterval int // Record every N cycles (default 1)
}

// SimulationEngine orchestrates scan cycles with waveform injection and
// plant model feedback for deterministic closed-loop simulation.
type SimulationEngine struct {
	cfg    SimConfig
	scan   *interp.ScanCycleEngine
	cycles []CycleRecord
}

// NewSimulationEngine creates a simulation engine from the given config.
func NewSimulationEngine(cfg SimConfig) *SimulationEngine {
	if cfg.RecordInterval <= 0 {
		cfg.RecordInterval = 1
	}
	scan := interp.NewScanCycleEngine(cfg.Program)
	scan.Initialize()
	return &SimulationEngine{
		cfg:  cfg,
		scan: scan,
	}
}

// Run executes the simulation for NumCycles iterations and returns the result.
// The simulation loop:
//  1. Inject waveform values as program inputs
//  2. Execute one scan cycle (Tick)
//  3. Collect program outputs
//  4. Feed outputs to plant models, collect plant outputs
//  5. Feed plant outputs back as program inputs for the NEXT cycle
//
// Plant outputs are fed back as inputs on the next cycle (not same cycle),
// matching real PLC behavior where inputs are read at scan start.
func (s *SimulationEngine) Run() (*SimResult, error) {
	// plantFeedback holds values from plant models to inject at the start
	// of the next cycle.
	plantFeedback := make(map[string]interp.Value)

	outputNames := s.scan.OutputNames()

	for cycle := 0; cycle < s.cfg.NumCycles; cycle++ {
		t := time.Duration(cycle) * s.cfg.CycleDt

		// Collect all inputs for this cycle (for recording)
		currentInputs := make(map[string]interp.Value)

		// 1a. Inject waveform values
		for _, wb := range s.cfg.Waveforms {
			val := wb.Generator.Generate(t)
			key := strings.ToUpper(wb.InputName)
			s.scan.SetInput(key, val)
			currentInputs[key] = val
		}

		// 1b. Inject plant feedback from previous cycle
		for name, val := range plantFeedback {
			s.scan.SetInput(name, val)
			currentInputs[name] = val
		}

		// 2. Execute scan cycle
		if err := s.scan.Tick(s.cfg.CycleDt); err != nil {
			return nil, err
		}

		// 3. Collect program outputs
		currentOutputs := make(map[string]interp.Value)
		for _, name := range outputNames {
			currentOutputs[name] = s.scan.GetOutput(name)
		}

		// 4. Feed outputs to plant models, collect plant feedback for next cycle
		plantFeedback = make(map[string]interp.Value) // reset
		for _, pb := range s.cfg.Plants {
			// Build plant inputs from program outputs
			plantInputs := make(map[string]interp.Value)
			for progOut, plantIn := range pb.OutputToPlant {
				key := strings.ToUpper(progOut)
				if v, ok := currentOutputs[key]; ok {
					plantInputs[plantIn] = v
				}
			}

			// Run plant model
			plantOutputs := pb.Model.Update(plantInputs, s.cfg.CycleDt)

			// Map plant outputs back to program input names for next cycle
			for plantOut, progIn := range pb.PlantToInput {
				if v, ok := plantOutputs[plantOut]; ok {
					plantFeedback[strings.ToUpper(progIn)] = v
				}
			}
		}

		// 5. Record cycle if interval matches
		if cycle%s.cfg.RecordInterval == 0 {
			s.cycles = append(s.cycles, CycleRecord{
				Cycle:   cycle,
				Time:    t,
				Inputs:  currentInputs,
				Outputs: currentOutputs,
			})
		}
	}

	return &SimResult{
		Cycles:    s.cycles,
		Duration:  time.Duration(s.cfg.NumCycles) * s.cfg.CycleDt,
		NumCycles: s.cfg.NumCycles,
	}, nil
}
