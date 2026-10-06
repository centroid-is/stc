package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/incremental"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/twincat"
	"github.com/centroid-is/stc/pkg/vendor"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [file... | x.tsproj | x.plcproj]",
		Short: "Type-check ST source files",
		Long: `Run semantic analysis on one or more IEC 61131-3 Structured Text source files.

Reports type errors, undeclared variables, unused variables, unreachable code,
and vendor compatibility warnings. Exit code 1 if errors found, 0 otherwise.

A single TwinCAT .tsproj or .plcproj argument is imported on the fly and
checked with its library stubs; diagnostics point at TcPOU files and lines.`,
		RunE: runCheck,
	}

	cmd.Flags().String("vendor", "", "Vendor target for compatibility checking (beckhoff, schneider, portable)")
	cmd.Flags().StringSliceP("define", "D", nil, "Define preprocessor symbols (can be repeated)")
	addGVLNameFlag(cmd)

	return cmd
}

func runCheck(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	vendorFlag, _ := cmd.Flags().GetString("vendor")
	defineFlags, _ := cmd.Flags().GetStringSlice("define")
	defines := pipeline.ParseDefines(defineFlags)

	if err := validateGVLName(cmd, args, format); err != nil {
		return err
	}

	if hasProjectArg(args) {
		return runCheckProject(cmd, args, format, vendorFlag, defines)
	}

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: no input files specified")
		fmt.Fprintln(os.Stderr, "usage: stc check <file.st> [file2.st ...]")
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	// Try to find and load project config
	var cfg *project.Config
	var configPath string
	if cp, err := project.FindConfig("."); err == nil {
		configPath = cp
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

	// Load vendor library stubs from configured library paths
	var libFiles []*ast.SourceFile
	if cfg != nil && len(cfg.Build.LibraryPaths) > 0 && configPath != "" {
		projectDir := filepath.Dir(configPath)
		var err error
		libFiles, err = vendor.LoadLibraries(cfg, projectDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: loading libraries: %v\n", err)
		}
	}

	// Determine cache directory for incremental analysis
	cacheDir := "."
	if configPath != "" {
		cacheDir = filepath.Dir(configPath)
	}

	// Run incremental parse (skips parsing unchanged files)
	ia := incremental.NewIncrementalAnalyzer(cacheDir)
	if defines != nil {
		ia.SetDefines(defines)
	}
	incrResult := ia.Parse(args)
	stats := incrResult.Stats

	// Rename after the incremental parse, never inside it: cached ASTs are
	// shared across runs, so renaming here keeps cold runs and cache hits
	// identical. validateGVLName guarantees a single file.
	for _, f := range incrResult.Files {
		applyGVLName(cmd, f)
	}

	// Run semantic analysis on all parsed files (with library stubs if available)
	analysisResult := analyzer.Analyze(incrResult.Files, cfg, analyzer.AnalyzeOpts{LibraryFiles: libFiles})

	// Combine parse diagnostics with analysis diagnostics
	allDiags := make([]diag.Diagnostic, 0, len(incrResult.Diags)+len(analysisResult.Diags))
	allDiags = append(allDiags, incrResult.Diags...)
	allDiags = append(allDiags, analysisResult.Diags...)

	return reportDiags(cmd, format, allDiags,
		fmt.Sprintf("(%d/%d files re-parsed)", stats.StaleFiles, stats.TotalFiles))
}

// isProjectPath reports whether p names a TwinCAT solution or PLC project.
func isProjectPath(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".tsproj", ".plcproj":
		return true
	}
	return false
}

// hasProjectArg reports whether any argument is a TwinCAT project path.
func hasProjectArg(args []string) bool {
	for _, a := range args {
		if isProjectPath(a) {
			return true
		}
	}
	return false
}

// failUsage reports a usage error: {"error": "..."} on stdout under
// --format json, otherwise "Error: ..." on stderr; then exits 1.
func failUsage(cmd *cobra.Command, format string, err error) {
	if format == "json" {
		out, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintln(os.Stdout, string(out))
	} else {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	exitWithFailure(cmd)
}

// runCheckProject imports a TwinCAT project and checks it through
// analyzer.AnalyzeProject. It bypasses the incremental cache: conversion is
// cheap and the cache is keyed on raw .st files, not on TcPOU XML.
func runCheckProject(cmd *cobra.Command, args []string, format, vendorFlag string, defines map[string]bool) error {
	if len(args) != 1 {
		failUsage(cmd, format, fmt.Errorf("a project path (.tsproj or .plcproj) must be the only argument"))
	}
	if cmd.Flags().Changed(gvlNameFlag) {
		failUsage(cmd, format, fmt.Errorf("--%s cannot be used with a project path; GVL names come from the TcGVL objects", gvlNameFlag))
	}

	m, importDiags, err := twincat.Import(args[0], twincat.Options{Defines: defines})
	if err != nil {
		failUsage(cmd, format, fmt.Errorf("importing %s: %w", args[0], err))
	}

	// Import already failed on a broken stc.toml next to the project, so a
	// config found here loads.
	var cfg *project.Config
	if cp, err := project.FindConfig(filepath.Dir(m.ProjectPath)); err == nil {
		cfg, _ = project.LoadConfig(cp)
	}
	if vendorFlag != "" {
		if cfg == nil {
			cfg = &project.Config{}
		}
		cfg.Build.VendorTarget = vendorFlag
	}

	res := analyzer.AnalyzeProject(m, cfg, defines)
	allDiags := append(append([]diag.Diagnostic{}, importDiags...), res.Diags...)
	return reportDiags(cmd, format, allDiags,
		fmt.Sprintf("(%d source(s), %d library source(s) from %s)", len(m.Sources), len(m.LibrarySources), m.PlcName))
}

// reportDiags prints diagnostics as a JSON array (stdout) or text (stderr)
// with an error/warning summary and a trailing note line, then exits 1 when
// any error was reported.
func reportDiags(cmd *cobra.Command, format string, allDiags []diag.Diagnostic, note string) error {
	if allDiags == nil {
		allDiags = []diag.Diagnostic{}
	}
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
		fmt.Fprintln(os.Stderr, note)
	}

	// Exit code: 1 if errors, 0 if warnings-only or clean
	if errorCount > 0 {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	return nil
}
