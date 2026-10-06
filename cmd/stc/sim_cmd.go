package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/sim"
	"github.com/spf13/cobra"
)

func newSimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sim <file.st | x.tsproj | x.plcproj | file.st...>",
		Short: "Run closed-loop simulation of an ST program or a whole project",
		Long: `Run a deterministic simulation of a Structured Text program with waveform
injection and optional plant model feedback. The simulation runs the program's
scan cycle for a specified number of iterations at a fixed time step.

Project mode runs a whole project with its task schedule: every task's
PROGRAMs at the task cycle on one runtime with every POU, GVL and library
stub. It applies to a .tsproj/.plcproj, to several .st files (default task:
MAIN every 10ms), to --project, and whenever --io, --persist, --realtime or
--duration is given. --cycles then counts base ticks (the GCD of the task
cycles) and the JSON result has cycles, sim_time_ns, tasks (runs, overruns),
get, diagnostics and warnings. --realtime runs free-running against the wall
clock for --duration (or until SIGINT/SIGTERM) instead of --cycles.
--wave and --dt apply to single-file mode only.`,
		RunE: runSim,
	}

	cmd.Flags().Int("cycles", 100, "Number of scan cycles to run (base ticks in project mode)")
	cmd.Flags().String("dt", "10ms", "Cycle time as Go duration (e.g., 10ms, 100us); single-file mode")
	cmd.Flags().StringSlice("wave", nil, `Waveform bindings: INPUT_NAME:KIND:AMPLITUDE:FREQUENCY
  KIND: step, ramp, sine, square
  Example: --wave "SENSOR:sine:100.0:0.5"`)
	cmd.Flags().StringArray("set", nil, `Set a variable before cycle 1: PATH=VALUE (repeatable).
  VALUE is JSON when valid (5, true, "Run", [1,2], {"x":1}), else a raw string
  such as T#5s or 16#FF. Example: --set MAIN.limit=5 --set GVL.x.p_cmd_Start=true`)
	cmd.Flags().StringArray("get", nil, "Print a variable after the last cycle: PATH (repeatable)")
	cmd.Flags().StringSliceP("define", "D", nil, "Define preprocessor symbols (can be repeated)")
	cmd.Flags().StringSlice("project", nil, "Run in project mode: a .tsproj/.plcproj or .st files (alternative to positional arguments)")
	cmd.Flags().Bool("realtime", false, "Project mode: run free-running against the wall clock for --duration (or until SIGINT/SIGTERM)")
	cmd.Flags().Duration("duration", 0, "Wall time to run with --realtime (0 = until SIGINT/SIGTERM)")
	addProjectRunFlags(cmd)

	return cmd
}

// simProjectFlags are the flags that select project mode on their own.
var simProjectFlags = []string{"project", "io", "persist", "realtime", "duration"}

// isSimProjectMode reports whether `stc sim` runs args as a project.
func isSimProjectMode(cmd *cobra.Command, args []string) bool {
	if len(args) > 1 || hasProjectArg(args) {
		return true
	}
	for _, f := range simProjectFlags {
		if cmd.Flags().Changed(f) {
			return true
		}
	}
	return false
}

func runSim(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")

	defineFlags, _ := cmd.Flags().GetStringSlice("define")
	defines := pipeline.ParseDefines(defineFlags)
	// Auto-define STC_SIM preprocessor symbol
	if defines == nil {
		defines = make(map[string]bool)
	}
	defines["STC_SIM"] = true

	if isSimProjectMode(cmd, args) {
		return runSimProject(cmd, args, defines, format)
	}
	if len(args) != 1 {
		return fmt.Errorf("accepts 1 arg(s), received %d", len(args))
	}
	filename := args[0]

	// Read and parse the ST file (with preprocessing)
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}
	result := pipeline.Parse(filename, string(content), defines)
	files := []*ast.SourceFile{result.File}
	prog := selectProgram(files, nil)
	if prog == nil {
		return fmt.Errorf("no PROGRAM declaration found in %s", filename)
	}

	setFlags, _ := cmd.Flags().GetStringArray("set")
	getFlags, _ := cmd.Flags().GetStringArray("get")
	sets, err := parseSetFlags(setFlags)
	if err != nil {
		return err
	}

	// All TYPEs, FBs, FUNCTIONs, GVLs and PROGRAMs of the file live on one
	// runtime; the simulation drives prog.
	rt, err := interp.NewRuntime(files)
	if err != nil {
		return fmt.Errorf("initialisation error: %w", err)
	}
	for _, s := range sets {
		if err := rt.Set(s.path, s.value); err != nil {
			return fmt.Errorf("--set %s: %w", s.path, err)
		}
	}

	// Parse flags
	cycles, _ := cmd.Flags().GetInt("cycles")
	dtStr, _ := cmd.Flags().GetString("dt")
	dt, err := time.ParseDuration(dtStr)
	if err != nil {
		return fmt.Errorf("invalid --dt value %q: %w", dtStr, err)
	}

	waveStrs, _ := cmd.Flags().GetStringSlice("wave")

	// Parse waveform bindings
	waveforms, err := parseWaveFlags(waveStrs)
	if err != nil {
		return err
	}

	cfg := sim.SimConfig{
		Program:   prog,
		NumCycles: cycles,
		CycleDt:   dt,
		Waveforms: waveforms,
	}

	engine := sim.NewSimulationEngineWith(cfg, rt.Engine(prog.Name.Name))
	simResult, err := engine.Run()
	if err != nil {
		return fmt.Errorf("simulation error: %w", err)
	}
	if len(getFlags) > 0 {
		simResult.Get = make(map[string]any, len(getFlags))
		for _, p := range getFlags {
			v, err := rt.Get(p)
			if err != nil {
				return fmt.Errorf("--get %s: %w", p, err)
			}
			simResult.Get[p] = rt.ToJSON(v)
		}
	}

	switch format {
	case "json":
		return outputJSON(simResult)
	default:
		if err := outputText(simResult); err != nil {
			return err
		}
		return outputGets(simResult, getFlags)
	}
}

// runSimProject is `stc sim` project mode: --cycles deterministic base
// ticks, or free-running with --realtime, then the project status.
func runSimProject(cmd *cobra.Command, args []string, defines map[string]bool, format string) error {
	projFlag, _ := cmd.Flags().GetStringSlice("project")
	inputs := append(append([]string{}, projFlag...), args...)
	if len(inputs) == 0 {
		return errors.New("no project given: pass a .tsproj/.plcproj or .st files")
	}
	for _, f := range []string{"wave", "dt"} {
		if cmd.Flags().Changed(f) {
			return fmt.Errorf("--%s is not supported in project mode: ticks follow the task cycles", f)
		}
	}
	realtime, _ := cmd.Flags().GetBool("realtime")
	duration, _ := cmd.Flags().GetDuration("duration")
	cycles, _ := cmd.Flags().GetInt("cycles")
	switch {
	case realtime && cmd.Flags().Changed("cycles"):
		return errors.New("--realtime runs for --duration of wall time; --cycles cannot be combined with it")
	case !realtime && cmd.Flags().Changed("duration"):
		return errors.New("--duration requires --realtime; use --cycles for a deterministic run")
	case duration < 0:
		return fmt.Errorf("--duration must not be negative, got %s", duration)
	case cycles < 0:
		return fmt.Errorf("--cycles must not be negative, got %d", cycles)
	}
	setFlags, _ := cmd.Flags().GetStringArray("set")
	getFlags, _ := cmd.Flags().GetStringArray("get")
	sets, err := parseSetFlags(setFlags)
	if err != nil {
		return err
	}
	cmd.SilenceUsage = true
	errOut := cmd.ErrOrStderr()
	r, err := projectSetup(cmd, inputs, defines, projectSetupOpts{Sets: sets}, errOut)
	if err != nil {
		return err
	}
	if realtime {
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err = r.runFree(ctx, duration, nil)
	} else {
		err = errors.Join(r.ticks(cycles), r.save())
	}
	if err != nil {
		return fmt.Errorf("simulation error: %w", err)
	}
	st, err := r.status(getFlags)
	if err != nil {
		return err
	}
	return writeProjectStatus(cmd.OutOrStdout(), errOut, format, st, getFlags)
}

// selectProgram returns the PROGRAM named by want (case-insensitive) or,
// when want names none, the first PROGRAM in files.
func selectProgram(files []*ast.SourceFile, want []string) *ast.ProgramDecl {
	var first, named *ast.ProgramDecl
	for _, f := range files {
		for _, d := range f.Declarations {
			if d, ok := d.(*ast.ProgramDecl); ok {
				if first == nil {
					first = d
				}
				if named == nil && d.Name != nil && len(want) > 0 && strings.EqualFold(d.Name.Name, want[0]) {
					named = d
				}
			}
		}
	}
	if named != nil {
		return named
	}
	return first
}

// simSet is one parsed --set flag.
type simSet struct {
	path  string
	value any
}

// parseSetFlags splits PATH=VALUE flags. VALUE is decoded as JSON (numbers
// as json.Number) when it is valid JSON, otherwise kept as a raw string.
func parseSetFlags(flags []string) ([]simSet, error) {
	out := make([]simSet, 0, len(flags))
	for _, f := range flags {
		path, raw, ok := strings.Cut(f, "=")
		if !ok || strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("invalid --set %q: expected PATH=VALUE", f)
		}
		out = append(out, simSet{path: strings.TrimSpace(path), value: parseSetValue(raw)})
	}
	return out, nil
}

func parseSetValue(raw string) any {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err == nil && v != nil {
		if _, err := dec.Token(); err == io.EOF {
			return v
		}
	}
	return raw
}

// outputGets prints the --get values as "PATH = JSON" lines in flag order.
func outputGets(result *sim.SimResult, paths []string) error {
	if len(paths) > 0 {
		fmt.Println()
	}
	for _, p := range paths {
		b, err := json.Marshal(result.Get[p])
		if err != nil {
			return err
		}
		fmt.Printf("%s = %s\n", p, b)
	}
	return nil
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
