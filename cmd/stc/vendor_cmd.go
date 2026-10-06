package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/centroid-is/stc/pkg/vendor"
	"github.com/spf13/cobra"
)

func newVendorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vendor",
		Short: "Vendor library tools",
		Long:  "Tools for working with vendor-specific PLC libraries.",
	}

	cmd.AddCommand(newVendorExtractCmd())
	return cmd
}

func newVendorExtractCmd() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "extract <path.plcproj>",
		Short: "Extract FB stubs from a TwinCAT project",
		Long: `Parse a TwinCAT .plcproj file and extract every POU, GVL, DUT and interface
it lists as a declaration-only .st stub (methods and properties keep their
signatures, implementation bodies are dropped), in plcproj order.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plcprojPath := args[0]

			stubs, diags, err := vendor.ExtractProject(plcprojPath)
			for _, d := range diags {
				fmt.Fprintln(os.Stderr, d.String())
			}
			if err != nil {
				return fmt.Errorf("extracting stubs: %w", err)
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
