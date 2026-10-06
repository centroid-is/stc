package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/sim"
	"github.com/centroid-is/stc/pkg/twincat"
	"github.com/spf13/cobra"
)

func newSimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sim <file.st|x.tsproj|x.plcproj>",
		Short: "Run closed-loop simulation of an ST program",
		Long: `Run a deterministic simulation of a Structured Text program with waveform
injection and optional plant model feedback. The simulation runs the program's
scan cycle for a specified number of iterations at a fixed time step.

Given a TwinCAT project, the program called by the first task runs (or the
first PROGRAM when the task calls none) with all project GVLs, and --dt
defaults to the task cycle time. Programs that instantiate user FBs from other
files are not supported yet.`,
		Args: cobra.ExactArgs(1),
		RunE: runSim,
	}

	cmd.Flags().Int("cycles", 100, "Number of scan cycles to run")
	cmd.Flags().String("dt", "10ms", "Cycle time as Go duration (e.g., 10ms, 100us)")
	cmd.Flags().StringSlice("wave", nil, `Waveform bindings: INPUT_NAME:KIND:AMPLITUDE:FREQUENCY
  KIND: step, ramp, sine, square
  Example: --wave "SENSOR:sine:100.0:0.5"`)
	cmd.Flags().StringSliceP("define", "D", nil, "Define preprocessor symbols (can be repeated)")

	return cmd
}

func runSim(cmd *cobra.Command, args []string) error {
	filename := args[0]
	format, _ := cmd.Flags().GetString("format")

	defineFlags, _ := cmd.Flags().GetStringSlice("define")
	defines := pipeline.ParseDefines(defineFlags)
	// Auto-define STC_SIM preprocessor symbol
	if defines == nil {
		defines = make(map[string]bool)
	}
	defines["STC_SIM"] = true

	var prog *ast.ProgramDecl
	var gvls []*ast.GVLDecl
	var taskDt time.Duration
	if isProjectPath(filename) {
		p, g, dt, err := loadSimProject(filename, defines)
		if err != nil {
			return err
		}
		prog, gvls, taskDt = p, g, dt
	} else {
		// Read and parse the ST file (with preprocessing)
		content, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("cannot read file: %w", err)
		}
		result := pipeline.Parse(filename, string(content), defines)
		prog, gvls = selectProgram([]*ast.SourceFile{result.File}, nil)
	}
	if prog == nil {
		return fmt.Errorf("no PROGRAM declaration found in %s", filename)
	}

	// Parse flags
	cycles, _ := cmd.Flags().GetInt("cycles")
	dtStr, _ := cmd.Flags().GetString("dt")
	dt, err := time.ParseDuration(dtStr)
	if err != nil {
		return fmt.Errorf("invalid --dt value %q: %w", dtStr, err)
	}
	if taskDt > 0 && !cmd.Flags().Changed("dt") {
		dt = taskDt
	}

	waveStrs, _ := cmd.Flags().GetStringSlice("wave")

	// Parse waveform bindings
	waveforms, err := parseWaveFlags(waveStrs)
	if err != nil {
		return err
	}

	cfg := sim.SimConfig{
		Program:   prog,
		GVLs:      gvls,
		NumCycles: cycles,
		CycleDt:   dt,
		Waveforms: waveforms,
	}

	engine := sim.NewSimulationEngine(cfg)
	simResult, err := engine.Run()
	if err != nil {
		return fmt.Errorf("simulation error: %w", err)
	}

	switch format {
	case "json":
		return outputJSON(simResult)
	default:
		return outputText(simResult)
	}
}

// selectProgram returns the PROGRAM named by want (case-insensitive) or,
// when want names none, the first PROGRAM; and every GVL in files.
func selectProgram(files []*ast.SourceFile, want []string) (*ast.ProgramDecl, []*ast.GVLDecl) {
	var first, named *ast.ProgramDecl
	var gvls []*ast.GVLDecl
	for _, f := range files {
		for _, d := range f.Declarations {
			switch d := d.(type) {
			case *ast.ProgramDecl:
				if first == nil {
					first = d
				}
				if named == nil && d.Name != nil && len(want) > 0 && strings.EqualFold(d.Name.Name, want[0]) {
					named = d
				}
			case *ast.GVLDecl:
				gvls = append(gvls, d)
			}
		}
	}
	if named != nil {
		return named, gvls
	}
	return first, gvls
}

// loadSimProject imports a TwinCAT project for simulation and returns the
// first task's program, all user GVLs and the task cycle time. Error
// diagnostics abort.
func loadSimProject(path string, defines map[string]bool) (*ast.ProgramDecl, []*ast.GVLDecl, time.Duration, error) {
	m, ds, err := twincat.Import(path, twincat.Options{Defines: defines})
	if err != nil {
		return nil, nil, 0, fmt.Errorf("importing %s: %w", path, err)
	}
	user, _, pds := twincat.ParseModel(m, defines)
	ds = append(ds, pds...)
	if hasErrors(ds) {
		for _, d := range ds {
			fmt.Fprintln(os.Stderr, d.String())
		}
		return nil, nil, 0, fmt.Errorf("project %s has errors", path)
	}
	var want []string
	var dt time.Duration
	if len(m.Tasks) > 0 {
		want, dt = m.Tasks[0].Programs, m.Tasks[0].CycleTime
	}
	prog, gvls := selectProgram(user, want)
	return prog, gvls, dt, nil
}

// parseWaveFlags parses --wave flag values into WaveformBinding objects.
// Format: INPUT_NAME:KIND:AMPLITUDE:FREQUENCY
func parseWaveFlags(flags []string) ([]sim.WaveformBinding, error) {
	var bindings []sim.WaveformBinding

	for _, f := range flags {
		parts := strings.Split(f, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid --wave format %q: expected INPUT_NAME:KIND[:AMPLITUDE[:FREQUENCY]]", f)
		}

		inputName := parts[0]
		kindStr := strings.ToLower(parts[1])

		var kind sim.WaveformKind
		switch kindStr {
		case "step":
			kind = sim.WaveStep
		case "ramp":
			kind = sim.WaveRamp
		case "sine":
			kind = sim.WaveSine
		case "square":
			kind = sim.WaveSquare
		default:
			return nil, fmt.Errorf("unknown waveform kind %q (expected step, ramp, sine, square)", kindStr)
		}

		cfg := sim.WaveformConfig{Kind: kind}

		if len(parts) >= 3 {
			amp, err := strconv.ParseFloat(parts[2], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid amplitude %q in --wave %q: %w", parts[2], f, err)
			}
			cfg.Amplitude = amp
		}

		if len(parts) >= 4 {
			freq, err := strconv.ParseFloat(parts[3], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid frequency %q in --wave %q: %w", parts[3], f, err)
			}
			cfg.Frequency = freq
		}

		bindings = append(bindings, sim.WaveformBinding{
			InputName: inputName,
			Generator: sim.NewWaveformGenerator(cfg),
		})
	}

	return bindings, nil
}

// outputJSON marshals the simulation result as indented JSON.
func outputJSON(result *sim.SimResult) error {
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON marshal error: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(out))
	return nil
}

// outputText prints a human-readable table of simulation results.
func outputText(result *sim.SimResult) error {
	fmt.Printf("Simulation: %d cycles, duration %s\n\n", result.NumCycles, result.Duration)

	if len(result.Cycles) == 0 {
		fmt.Println("(no cycles recorded)")
		return nil
	}

	// Collect all output names from first cycle
	var outputNames []string
	for name := range result.Cycles[0].Outputs {
		outputNames = append(outputNames, name)
	}

	// Print header
	fmt.Printf("%-8s %-12s", "Cycle", "Time")
	for _, name := range outputNames {
		fmt.Printf(" %-16s", name)
	}
	fmt.Println()
	fmt.Printf("%-8s %-12s", "-----", "----")
	for range outputNames {
		fmt.Printf(" %-16s", "----------------")
	}
	fmt.Println()

	// Print rows (max 50, with ellipsis)
	maxRows := 50
	cycles := result.Cycles
	truncated := false
	if len(cycles) > maxRows {
		truncated = true
		cycles = cycles[:maxRows]
	}

	for _, c := range cycles {
		fmt.Printf("%-8d %-12s", c.Cycle, c.Time)
		for _, name := range outputNames {
			if v, ok := c.Outputs[name]; ok {
				fmt.Printf(" %-16s", v.String())
			} else {
				fmt.Printf(" %-16s", "-")
			}
		}
		fmt.Println()
	}

	if truncated {
		fmt.Printf("... (%d more cycles not shown)\n", len(result.Cycles)-maxRows)
	}

	return nil
}
