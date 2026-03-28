package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/lint"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/spf13/cobra"
)

// lintOutput is the JSON output structure for lint results.
type lintOutput struct {
	File         string            `json:"file"`
	Diagnostics  []diag.Diagnostic `json:"diagnostics"`
	HasErrors    bool              `json:"has_errors"`
	WarningCount int               `json:"warning_count"`
	ErrorCount   int               `json:"error_count"`
}

func newLintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint [file...]",
		Short: "Lint ST source files",
		Long: `Run lint checks on one or more IEC 61131-3 Structured Text source files.

Reports PLCopen coding guideline violations and naming convention issues.
Exit code 1 if parse errors exist, 0 otherwise (lint warnings do not cause exit 1).`,
		RunE: runLint,
	}

	return cmd
}

func runLint(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: no input files specified")
		fmt.Fprintln(os.Stderr, "usage: stc lint <file.st> [file2.st ...]")
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

	// Build lint options
	opts := lint.DefaultLintOptions()
	if cfg != nil && cfg.Lint.NamingConvention != "" {
		opts.NamingConvention = cfg.Lint.NamingConvention
	}

	var allDiags []diag.Diagnostic
	var perFile []lintOutput
	hasParseErrors := false

	for _, filename := range args {
		content, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			os.Exit(1)
		}

		// Parse the file
		parseResult := parser.Parse(filename, string(content))

		// Check for parse errors
		parseErrorCount := 0
		for _, d := range parseResult.Diags {
			if d.Severity == diag.Error {
				parseErrorCount++
				hasParseErrors = true
			}
		}

		// Run lint rules on the parsed file
		lintResult := lint.LintFile(parseResult.File, opts)

		// Combine parse diagnostics + lint diagnostics
		fileDiags := append(parseResult.Diags, lintResult.Diags...)

		// Count
		errorCount := 0
		warningCount := 0
		for _, d := range fileDiags {
			switch d.Severity {
			case diag.Error:
				errorCount++
			case diag.Warning:
				warningCount++
			}
		}

		allDiags = append(allDiags, fileDiags...)
		perFile = append(perFile, lintOutput{
			File:         filename,
			Diagnostics:  fileDiags,
			HasErrors:    errorCount > 0,
			WarningCount: warningCount,
			ErrorCount:   errorCount,
		})
	}

	// Count totals
	totalErrors := 0
	totalWarnings := 0
	for _, d := range allDiags {
		switch d.Severity {
		case diag.Error:
			totalErrors++
		case diag.Warning:
			totalWarnings++
		}
	}

	switch format {
	case "json":
		out, err := json.MarshalIndent(allDiags, "", "  ")
		if err != nil {
			return fmt.Errorf("JSON marshal error: %w", err)
		}
		fmt.Fprintln(os.Stdout, string(out))

	default: // text
		for _, d := range allDiags {
			fmt.Fprintln(os.Stderr, d.String())
		}
		fmt.Fprintf(os.Stderr, "%d warning(s), %d error(s)\n", totalWarnings, totalErrors)
	}

	// Exit code: 1 if parse errors exist, 0 otherwise
	if hasParseErrors {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	return nil
}
