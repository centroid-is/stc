package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/twincat"
	"github.com/centroid-is/stc/pkg/vendor"
	"github.com/spf13/cobra"
)

func newVendorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vendor",
		Short: "Vendor library tools",
		Long:  "Tools for working with vendor-specific PLC libraries.",
	}

	cmd.AddCommand(newVendorExtractCmd(), newVendorImportCmd())
	return cmd
}

// printJSON writes v as indented JSON to stdout.
func printJSON(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON marshal error: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(out))
	return nil
}

// nonNilDiags returns ds, or an empty slice so JSON prints [] instead of null.
func nonNilDiags(ds []diag.Diagnostic) []diag.Diagnostic {
	if ds == nil {
		return []diag.Diagnostic{}
	}
	return ds
}

// hasErrors reports whether ds contains an Error diagnostic.
func hasErrors(ds []diag.Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == diag.Error {
			return true
		}
	}
	return false
}

// exitWithFailure exits 1 after output was already printed.
func exitWithFailure(cmd *cobra.Command) {
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	os.Exit(1)
}

func newVendorImportCmd() *cobra.Command {
	var outDir string

	cmd := &cobra.Command{
		Use:   "import <x.tsproj|x.plcproj>",
		Short: "Import a TwinCAT project and report its structure",
		Long: `Read a TwinCAT solution (.tsproj) or PLC project (.plcproj) and print the
PLC name, AMS port, tasks, sources and how each referenced library resolved
(sibling project, library_paths, embedded stub, built-in or unresolved).

With --out, write the project as plain .st files (compact form), its
libraries under libs/, and an stc.toml so stc check/test work on the copy.
Exit code 1 when the import fails or reports an error diagnostic.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			defineFlags, _ := cmd.Flags().GetStringSlice("define")

			m, ds, err := twincat.Import(args[0], twincat.Options{Defines: pipeline.ParseDefines(defineFlags)})
			if err != nil {
				for _, d := range ds {
					fmt.Fprintln(os.Stderr, d.String())
				}
				return fmt.Errorf("importing %s: %w", args[0], err)
			}

			// Status output goes to stderr in JSON mode so stdout stays valid JSON.
			status := io.Writer(os.Stdout)
			if format == "json" {
				status = os.Stderr
			}
			if outDir != "" {
				if err := twincat.WriteOut(m, outDir); err != nil {
					return fmt.Errorf("writing %s: %w", outDir, err)
				}
				fmt.Fprintf(status, "Wrote %d source(s) and %d library file(s) to %s\n",
					len(m.Sources), len(m.LibrarySources), outDir)
			}

			if format == "json" {
				if err := printJSON(struct {
					*twincat.Model
					Diagnostics []diag.Diagnostic `json:"diagnostics"`
				}{m, nonNilDiags(ds)}); err != nil {
					return err
				}
			} else {
				printImportText(m, ds)
			}
			if hasErrors(ds) {
				exitWithFailure(cmd)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outDir, "out", "", "Write the project as .st files, libs/ and stc.toml into this directory")
	cmd.Flags().StringSliceP("define", "D", nil, "Define preprocessor symbols (can be repeated)")
	return cmd
}

// printImportText prints the human-readable import summary to stdout and
// the diagnostics to stderr.
func printImportText(m *twincat.Model, ds []diag.Diagnostic) {
	fmt.Printf("PLC %s (AMS port %d)\n", m.PlcName, m.AmsPort)
	fmt.Printf("Project: %s\n", m.ProjectPath)
	for _, t := range m.Tasks {
		fmt.Printf("Task %s: cycle %s, priority %d, programs %s\n", t.Name, t.CycleTime, t.Priority, joinOrNone(t.Programs))
	}
	counts := map[twincat.Kind]int{}
	for _, s := range m.Sources {
		counts[s.Kind]++
	}
	fmt.Printf("Sources: %d POU(s), %d GVL(s), %d DUT(s), %d interface(s)\n",
		counts[twincat.KindPOU], counts[twincat.KindGVL], counts[twincat.KindDUT], counts[twincat.KindITF])
	fmt.Printf("Libraries: %d\n", len(m.Libraries))
	for _, l := range m.Libraries {
		if l.Path != "" {
			fmt.Printf("  %s: %s (%s)\n", l.Name, l.ResolvedFrom, l.Path)
		} else {
			fmt.Printf("  %s: %s\n", l.Name, l.ResolvedFrom)
		}
	}
	fmt.Printf("%d task(s), %d source(s), %d librar(ies), %d library source file(s)\n",
		len(m.Tasks), len(m.Sources), len(m.Libraries), len(m.LibrarySources))
	for _, d := range ds {
		fmt.Fprintf(os.Stderr, "%s [%s]\n", d.String(), d.Code)
	}
}

func joinOrNone(ss []string) string {
	if len(ss) == 0 {
		return "(none)"
	}
	out := ss[0]
	for _, s := range ss[1:] {
		out += ", " + s
	}
	return out
}

func newVendorExtractCmd() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "extract <path.plcproj>",
		Short: "Extract FB stubs from a TwinCAT project",
		Long: `Parse a TwinCAT .plcproj file and extract every POU, GVL, DUT and interface
it lists as a declaration-only .st stub (methods and properties keep their
signatures, implementation bodies are dropped), in plcproj order.

With --format json, print {"stubs": [{name, rel_path, kind, text}], "diagnostics": [...]}.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			plcprojPath := args[0]

			stubs, diags, err := vendor.ExtractProject(plcprojPath)
			if format != "json" || err != nil {
				for _, d := range diags {
					fmt.Fprintln(os.Stderr, d.String())
				}
			}
			if err != nil {
				return fmt.Errorf("extracting stubs: %w", err)
			}

			if format == "json" && outputDir == "" {
				if stubs == nil {
					stubs = []vendor.ExtractedStub{}
				}
				return printJSON(struct {
					Stubs       []vendor.ExtractedStub `json:"stubs"`
					Diagnostics []diag.Diagnostic      `json:"diagnostics"`
				}{stubs, nonNilDiags(diags)})
			}

			if len(stubs) == 0 {
				fmt.Fprintln(os.Stderr, "No POU declarations found in project.")
				return nil
			}

			// Ensure output directory exists
			if outputDir != "" {
				if err := os.MkdirAll(outputDir, 0o755); err != nil {
					return fmt.Errorf("creating output directory: %w", err)
				}
			}

			for _, st := range stubs {
				name, stub := st.Name, st.Text
				if outputDir == "" {
					// Print to stdout
					fmt.Printf("(* %s *)\n%s\n", name, stub)
				} else {
					// Write to individual .st files
					outPath := filepath.Join(outputDir, name+".st")
					if err := os.WriteFile(outPath, []byte(stub), 0o644); err != nil {
						return fmt.Errorf("writing %s: %w", outPath, err)
					}
					fmt.Printf("Extracted: %s -> %s\n", name, outPath)
				}
			}

			if outputDir == "" {
				fmt.Fprintf(os.Stderr, "\n%d POU(s) extracted to stdout. Use --output to write files.\n", len(stubs))
			} else {
				fmt.Fprintf(os.Stderr, "%d POU(s) extracted to %s\n", len(stubs), outputDir)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory for extracted .st files (default: stdout)")
	return cmd
}
