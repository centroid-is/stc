package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [file...]",
		Short: "Type-check ST source files",
		Long: `Run semantic analysis on one or more IEC 61131-3 Structured Text source files.

Reports type errors, undeclared variables, unused variables, unreachable code,
and vendor compatibility warnings. Exit code 1 if errors found, 0 otherwise.`,
		RunE: runCheck,
	}

	cmd.Flags().String("vendor", "", "Vendor target for compatibility checking (beckhoff, schneider, portable)")

	return cmd
}

func runCheck(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	vendorFlag, _ := cmd.Flags().GetString("vendor")

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: no input files specified")
		fmt.Fprintln(os.Stderr, "usage: stc check <file.st> [file2.st ...]")
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	// Try to find and load project config
	var cfg *project.Config
	if configPath, err := project.FindConfig("."); err == nil {
		if loaded, err := project.LoadConfig(configPath); err == nil {
			cfg = loaded
		}
	}

	// Apply --vendor flag override
	if vendorFlag != "" {
		if cfg == nil {
			cfg = &project.Config{}
		}
		cfg.Build.VendorTarget = vendorFlag
	}

	// Run analysis
	result := analyzer.AnalyzeFiles(args, cfg)

	// Count errors and warnings
	errorCount := 0
	warningCount := 0
	for _, d := range result.Diags {
		switch d.Severity {
		case diag.Error:
			errorCount++
		case diag.Warning:
			warningCount++
		}
	}

	switch format {
	case "json":
		// JSON output to stdout
		out, err := json.MarshalIndent(result.Diags, "", "  ")
		if err != nil {
			return fmt.Errorf("JSON marshal error: %w", err)
		}
		fmt.Fprintln(os.Stdout, string(out))

	default: // text
		// Print each diagnostic to stderr
		for _, d := range result.Diags {
			fmt.Fprintln(os.Stderr, d.String())
		}
		// Print summary to stderr
		fmt.Fprintf(os.Stderr, "%d error(s), %d warning(s)\n", errorCount, warningCount)
	}

	// Exit code: 1 if errors, 0 if warnings-only or clean
	if errorCount > 0 {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	return nil
}
