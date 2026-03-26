package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/spf13/cobra"
)

// parseOutput is the JSON output structure for a single parsed file.
type parseOutput struct {
	File        string            `json:"file"`
	AST         json.RawMessage   `json:"ast"`
	Diagnostics []diag.Diagnostic `json:"diagnostics"`
	HasErrors   bool              `json:"has_errors"`
}

func newParseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "parse [file...]",
		Short: "Parse ST source files and output AST",
		Long:  "Parse one or more IEC 61131-3 Structured Text source files and output the abstract syntax tree.",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runParse,
	}
}

func runParse(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	hasErrors := false

	var outputs []parseOutput

	for _, filename := range args {
		content, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			hasErrors = true
			continue
		}

		result := parser.Parse(filename, string(content))

		// Check if any diagnostic is an error.
		fileHasErrors := false
		for _, d := range result.Diags {
			if d.Severity == diag.Error {
				fileHasErrors = true
				break
			}
		}
		if fileHasErrors {
			hasErrors = true
		}

		switch format {
		case "json":
			astJSON, marshalErr := ast.MarshalNode(result.File)
			if marshalErr != nil {
				fmt.Fprintf(os.Stderr, "error marshaling AST: %v\n", marshalErr)
				hasErrors = true
				continue
			}
			outputs = append(outputs, parseOutput{
				File:        filename,
				AST:         json.RawMessage(astJSON),
				Diagnostics: result.Diags,
				HasErrors:   fileHasErrors,
			})
		default: // text
			// Print diagnostics to stderr.
			for _, d := range result.Diags {
				fmt.Fprintln(os.Stderr, d.String())
			}
			// Print summary to stdout.
			nDecls := 0
			if result.File != nil {
				nDecls = len(result.File.Declarations)
			}
			fmt.Fprintf(os.Stdout, "Parsed %d declaration(s), %d diagnostic(s) in %s\n",
				nDecls, len(result.Diags), filename)
		}
	}

	// In JSON mode, output the collected results.
	if format == "json" {
		var out []byte
		var err error
		if len(outputs) == 1 {
			out, err = json.MarshalIndent(outputs[0], "", "  ")
		} else {
			out, err = json.MarshalIndent(outputs, "", "  ")
		}
		if err != nil {
			return fmt.Errorf("JSON marshal error: %w", err)
		}
		fmt.Fprintln(os.Stdout, string(out))
	}

	if hasErrors {
		// Signal error via exit code without printing extra message
		// (Cobra would print the error, so we use SilenceErrors).
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	return nil
}
