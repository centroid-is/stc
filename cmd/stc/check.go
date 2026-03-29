package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/incremental"
	"github.com/centroid-is/stc/pkg/pipeline"
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
	cmd.Flags().StringSliceP("define", "D", nil, "Define preprocessor symbols (can be repeated)")

	return cmd
}

func runCheck(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	vendorFlag, _ := cmd.Flags().GetString("vendor")
	defineFlags, _ := cmd.Flags().GetStringSlice("define")
	defines := pipeline.ParseDefines(defineFlags)

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

	// Determine cache directory for incremental analysis
	cacheDir := "."
	if configPath, _ := project.FindConfig("."); configPath != "" {
		cacheDir = filepath.Dir(configPath)
	}

	// Run incremental parse (skips parsing unchanged files)
	ia := incremental.NewIncrementalAnalyzer(cacheDir)
	if defines != nil {
		ia.SetDefines(defines)
	}
	incrResult := ia.Parse(args)
	stats := incrResult.Stats

	// Run semantic analysis on all parsed files
	analysisResult := analyzer.Analyze(incrResult.Files, cfg)

	// Combine parse diagnostics with analysis diagnostics
	allDiags := make([]diag.Diagnostic, 0, len(incrResult.Diags)+len(analysisResult.Diags))
	allDiags = append(allDiags, incrResult.Diags...)
	allDiags = append(allDiags, analysisResult.Diags...)

	// Count errors and warnings
	errorCount := 0
	warningCount := 0
	for _, d := range allDiags {
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
		out, err := json.MarshalIndent(allDiags, "", "  ")
		if err != nil {
			return fmt.Errorf("JSON marshal error: %w", err)
		}
		fmt.Fprintln(os.Stdout, string(out))

	default: // text
		// Print each diagnostic to stderr
		for _, d := range allDiags {
			fmt.Fprintln(os.Stderr, d.String())
		}
		// Print summary to stderr
		fmt.Fprintf(os.Stderr, "%d error(s), %d warning(s)\n", errorCount, warningCount)
		fmt.Fprintf(os.Stderr, "(%d/%d files re-parsed)\n", stats.StaleFiles, stats.TotalFiles)
	}

	// Exit code: 1 if errors, 0 if warnings-only or clean
	if errorCount > 0 {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	return nil
}
