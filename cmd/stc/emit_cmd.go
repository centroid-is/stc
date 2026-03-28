package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/emit"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/spf13/cobra"
)

// emitOutput is the JSON output structure for a single emitted file.
type emitOutput struct {
	File        string            `json:"file"`
	Code        string            `json:"code"`
	Target      string            `json:"target"`
	Diagnostics []diag.Diagnostic `json:"diagnostics"`
	HasErrors   bool              `json:"has_errors"`
}

func newEmitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "emit [file...]",
		Short: "Emit vendor-specific ST",
		Long: `Emit vendor-specific Structured Text from parsed source files.

Parses one or more ST source files and re-emits them with vendor-specific
formatting and filtering. Supports Beckhoff (full CODESYS OOP), Schneider
(no OOP/pointers/references), and Portable (additionally no 64-bit types).`,
		RunE: runEmit,
	}

	cmd.Flags().String("target", "portable", "Vendor target: beckhoff, schneider, portable")

	return cmd
}

func runEmit(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: no input files specified")
		fmt.Fprintln(os.Stderr, "usage: stc emit <file.st> [file2.st ...] [--target <vendor>]")
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	format, _ := cmd.Flags().GetString("format")
	targetName, _ := cmd.Flags().GetString("target")

	target := emit.LookupTarget(targetName)
	opts := emit.Options{
		Target:            target,
		Indent:            "    ",
		UppercaseKeywords: true,
	}

	hasErrors := false
	var outputs []emitOutput

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

		switch format {
		case "json":
			code := ""
			if result.File != nil {
				code = emit.Emit(result.File, opts)
			}
			outputs = append(outputs, emitOutput{
				File:        filename,
				Code:        code,
				Target:      string(target),
				Diagnostics: result.Diags,
				HasErrors:   fileHasErrors,
			})

		default: // text
			if fileHasErrors {
				// Print diagnostics to stderr
				for _, d := range result.Diags {
					fmt.Fprintln(os.Stderr, d.String())
				}
				continue
			}

			if result.File != nil {
				// Print file separator for multiple files
				if len(args) > 1 && i > 0 {
					fmt.Fprintf(os.Stdout, "// --- file: %s ---\n", filepath.Base(filename))
				}
				if len(args) > 1 && i == 0 {
					fmt.Fprintf(os.Stdout, "// --- file: %s ---\n", filepath.Base(filename))
				}
				code := emit.Emit(result.File, opts)
				fmt.Fprint(os.Stdout, code)
			}
		}
	}

	// JSON output
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
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	return nil
}
