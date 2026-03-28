package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/format"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/spf13/cobra"
)

// fmtOutput is the JSON output structure for a single formatted file.
type fmtOutput struct {
	File        string            `json:"file"`
	Code        string            `json:"code"`
	Diagnostics []diag.Diagnostic `json:"diagnostics"`
	HasErrors   bool              `json:"has_errors"`
}

func newFmtCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fmt [file...]",
		Short: "Format ST source files",
		Long: `Format one or more Structured Text source files with consistent style.

Parses each file and re-emits with normalized indentation, keyword casing,
and spacing. Comments attached to AST nodes are preserved.`,
		RunE: runFmt,
	}

	cmd.Flags().String("indent", "    ", "Indentation string (default: 4 spaces)")
	cmd.Flags().Bool("uppercase-keywords", true, "Use uppercase keywords (default: true)")

	return cmd
}

func runFmt(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: no input files specified")
		fmt.Fprintln(os.Stderr, "usage: stc fmt <file.st> [file2.st ...] [--indent <string>] [--uppercase-keywords]")
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	outputFormat, _ := cmd.Flags().GetString("format")
	indent, _ := cmd.Flags().GetString("indent")
	uppercaseKeywords, _ := cmd.Flags().GetBool("uppercase-keywords")

	opts := format.FormatOptions{
		Indent:            indent,
		UppercaseKeywords: uppercaseKeywords,
	}

	hasErrors := false
	var outputs []fmtOutput

	for i, filename := range args {
		content, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			hasErrors = true
			continue
		}

		result := parser.Parse(filename, string(content))

		// Check for parse errors
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

		switch outputFormat {
		case "json":
			code := ""
			if result.File != nil {
				code = format.Format(result.File, opts)
			}
			outputs = append(outputs, fmtOutput{
				File:        filename,
				Code:        code,
				Diagnostics: result.Diags,
				HasErrors:   fileHasErrors,
			})

		default: // text
			if fileHasErrors {
				for _, d := range result.Diags {
					fmt.Fprintln(os.Stderr, d.String())
				}
				continue
			}

			if result.File != nil {
				if len(args) > 1 {
					fmt.Fprintf(os.Stdout, "// --- file: %s ---\n", filepath.Base(filename))
				}
				code := format.Format(result.File, opts)
				fmt.Fprint(os.Stdout, code)
			}
			_ = i // suppress unused
		}
	}

	// JSON output
	if outputFormat == "json" {
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
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	return nil
}
