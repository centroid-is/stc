package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/scenario"
	stctesting "github.com/centroid-is/stc/pkg/testing"
	"github.com/centroid-is/stc/pkg/twincat"
	"github.com/centroid-is/stc/pkg/vendor"
	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	var projectPaths, ioFlags []string
	cmd := &cobra.Command{
		Use:   "test [dir]",
		Short: "Run ST unit tests",
		Long: `Discover and run *_test.st test files in the specified directory (default: current directory).

With --project <x.tsproj|x.plcproj>, the TwinCAT project is imported and its
POUs, GVLs, DUTs and interfaces (plus sibling library projects) are available
to the tests; embedded Beckhoff library stubs are auto-stubbed.

Project (plant) mode: with --io <Device N.xml glob>, or with --project given
.st files (repeat --project, or pass a directory), every TEST_CASE runs
against a fresh simulated plant: the whole project with its task schedule
and, with --io, the EtherCAT network. Test bodies use SET, GET,
SIM_SET_LINK, SIM_TRIP, SIM_SLAVE_STATE, SIM_ANALOG, SIM_DRIVE_FAULT,
SIM_SERIAL_PEER, SIM_RAMP, RUN_CYCLES and ADVANCE_TIME (whole scans), and
read GVLs directly. Test files then hold only TEST_CASEs.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			format, _ := cmd.Flags().GetString("format")

			// Auto-define STC_TEST preprocessor symbol
			opts := stctesting.RunOpts{
				Defines: map[string]bool{"STC_TEST": true},
			}
			if configPath, err := project.FindConfig(dir); err == nil {
				cfg, err := project.LoadConfig(configPath)
				if err != nil {
					return fmt.Errorf("loading config: %w", err)
				}
				projectDir := filepath.Dir(configPath)

				// Load library files
				libFiles, err := vendor.LoadLibraries(cfg, projectDir)
				if err != nil {
					return fmt.Errorf("loading libraries: %w", err)
				}
				opts.LibraryFiles = libFiles

				// Load mock files
				mockFiles, err := vendor.LoadMocks(cfg, projectDir)
				if err != nil {
					return fmt.Errorf("loading mocks: %w", err)
				}
				opts.MockFiles = mockFiles
			}
			switch {
			case len(ioFlags) > 0 && len(projectPaths) == 0:
				return fmt.Errorf("--io requires --project")
			case isPlantMode(projectPaths, ioFlags):
				spec, err := loadTestPlant(cmd, projectPaths, ioFlags, opts.Defines, format)
				if err != nil {
					return err
				}
				opts.Plant = spec
			case len(projectPaths) == 1:
				if err := loadTestProject(projectPaths[0], format, &opts); err != nil {
					return err
				}
			}

			result, err := stctesting.RunWithOpts(dir, opts)
			if err != nil {
				return err
			}

			switch format {
			case "junit":
				out, fmtErr := stctesting.FormatJUnit(result)
				if fmtErr != nil {
					return fmtErr
				}
				fmt.Fprint(os.Stdout, string(out))
			case "json":
				out, fmtErr := stctesting.FormatJSON(result)
				if fmtErr != nil {
					return fmtErr
				}
				fmt.Fprintln(os.Stdout, string(out))
			default: // "text"
				printTextResults(result)
			}

			if result.HasFailures() {
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&projectPaths, "project", nil, "TwinCAT project (.tsproj or .plcproj) whose sources the tests run against, or .st files/directories of a project to simulate (repeatable)")
	cmd.Flags().StringSliceVar(&ioFlags, "io", nil, "EtherCATConfig export (Device N.xml, globs allowed) simulated under the project in plant mode (repeatable)")
	return cmd
}

// isPlantMode reports whether stc test runs in project (plant) mode: any
// --io, or a --project list that is not a single .tsproj/.plcproj.
func isPlantMode(projectPaths, ioFlags []string) bool {
	if len(projectPaths) == 0 {
		return false
	}
	return len(ioFlags) > 0 || len(projectPaths) > 1 || !isProjectPath(projectPaths[0])
}

// loadTestPlant loads the project and the --io exports into a PlantSpec,
// once for the whole run (each TEST_CASE builds its own Plant from it).
// Load and resolve warnings go to stderr in text mode; errors always.
func loadTestPlant(cmd *cobra.Command, projectPaths, ioFlags []string, defines map[string]bool, format string) (*scenario.PlantSpec, error) {
	errOut := cmd.ErrOrStderr()
	emit := func(ds []diag.Diagnostic) {
		for _, d := range ds {
			if d.Severity == diag.Error || format != "json" {
				fmt.Fprintln(errOut, d.String())
			}
		}
	}
	src, ds, err := loadProjectSpec(projectPaths, defines)
	emit(ds)
	if err != nil {
		return nil, err
	}
	ioFiles, err := expandIOGlobs(ioFlags)
	if err != nil {
		return nil, err
	}
	spec, err := scenario.BuildPlantSpec(src, ioFiles)
	emit(spec.Diagnostics)
	if err != nil {
		if spec.Topology == nil {
			return nil, fmt.Errorf("--io: %w", err)
		}
		return nil, fmt.Errorf("EtherCAT links do not resolve: %d error(s)", countErrors(spec.Diagnostics))
	}
	return &spec, nil
}

// loadTestProject imports a TwinCAT project into opts: project and sibling
// library sources become RunOpts.ProjectFiles, stub files are appended to
// RunOpts.LibraryFiles. Error diagnostics from the import or parse abort the
// run; warnings are printed to stderr in text mode.
func loadTestProject(path, format string, opts *stctesting.RunOpts) error {
	if !isProjectPath(path) {
		return fmt.Errorf("--project expects a .tsproj or .plcproj file, got %s", path)
	}
	m, ds, err := twincat.Import(path, twincat.Options{Defines: opts.Defines})
	if err != nil {
		return fmt.Errorf("importing %s: %w", path, err)
	}
	projectFiles, stubs, pds := twincat.ParseForTest(m, opts.Defines)
	ds = append(ds, pds...)
	for _, d := range ds {
		if d.Severity == diag.Error || format != "json" {
			fmt.Fprintln(os.Stderr, d.String())
		}
	}
	if hasErrors(ds) {
		return fmt.Errorf("project %s has errors", path)
	}
	opts.ProjectFiles = projectFiles
	opts.LibraryFiles = append(opts.LibraryFiles, stubs...)
	return nil
}

// printTextResults prints human-readable test results to stdout.
func printTextResults(result *stctesting.RunResult) {
	for _, suite := range result.Suites {
		fmt.Printf("=== RUN  %s\n", suite.Name)
		for _, tr := range suite.Tests {
			if tr.Passed {
				fmt.Printf("--- PASS: %s (%.3fs)\n", tr.Name, tr.Duration.Seconds())
			} else {
				fmt.Printf("--- FAIL: %s (%.3fs)\n", tr.Name, tr.Duration.Seconds())
				for _, a := range tr.Assertions {
					if !a.Passed {
						if a.Position != "" {
							fmt.Printf("    %s: %s\n", a.Position, a.Message)
						} else {
							fmt.Printf("    %s\n", a.Message)
						}
					}
				}
				if tr.Error != "" {
					fmt.Printf("    error: %s\n", tr.Error)
				}
			}
		}
	}

	// Print fidelity warnings
	if len(result.Warnings) > 0 {
		fmt.Println()
		fmt.Println("Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  [fidelity] %s\n", w)
		}
	}

	fmt.Println()
	if result.HasFailures() {
		fmt.Println("FAIL")
	} else {
		fmt.Println("ok")
	}
	fmt.Printf("%d tests, %d passed, %d failed\n", result.Total, result.Passed, result.Failed)
}
