package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/preprocess"
	"github.com/spf13/cobra"
)

// ppOutput is the JSON output structure for a single preprocessed file.
type ppOutput struct {
	File        string            `json:"file"`
	Output      string            `json:"output"`
	SourceMap   []smEntry         `json:"source_map"`
	Diagnostics []diag.Diagnostic `json:"diagnostics"`
	HasErrors   bool              `json:"has_errors"`
}

// smEntry is a single source-map entry for JSON output.
type smEntry struct {
	PreprocLine int    `json:"preproc_line"`
	OrigFile    string `json:"orig_file"`
	OrigLine    int    `json:"orig_line"`
}

func newPpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pp [file...]",
		Short: "Preprocess ST source files",
		Long: `Preprocess IEC 61131-3 Structured Text source files by evaluating
conditional compilation directives ({IF}, {ELSIF}, {ELSE}, {END_IF},
{DEFINE}, {ERROR}).

Use --define / -D to set external symbols for conditional compilation.
Multiple defines can be specified:

  stc pp myfile.st --define VENDOR_BECKHOFF --define DEBUG`,
		Args: cobra.MinimumNArgs(1),
		RunE: runPp,
	}
	cmd.Flags().StringSliceP("define", "D", nil, "Define preprocessor symbols (can be repeated)")
	return cmd
}

func runPp(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	defineFlags, _ := cmd.Flags().GetStringSlice("define")

	// Build defines map from flags.
	defines := make(map[string]bool, len(defineFlags))
	for _, d := range defineFlags {
		defines[d] = true
	}

	hasErrors := false
	var outputs []ppOutput

	for _, filename := range args {
		content, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			hasErrors = true
			continue
		}

		result := preprocess.Preprocess(string(content), preprocess.Options{
			Filename: filename,
			Defines:  defines,
		})

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
			// Build source map entries.
			mappings := result.SourceMap.Mappings()
			entries := make([]smEntry, len(mappings))
			for i, m := range mappings {
				entries[i] = smEntry{
					PreprocLine: m.PreprocLine,
					OrigFile:    m.OrigFile,
					OrigLine:    m.OrigLine,
				}
			}
			diags := result.Diags
			if diags == nil {
				diags = []diag.Diagnostic{}
			}
			outputs = append(outputs, ppOutput{
				File:        filename,
				Output:      result.Output,
				SourceMap:   entries,
				Diagnostics: diags,
				HasErrors:   fileHasErrors,
			})
		default: // text
			// Print diagnostics to stderr.
			for _, d := range result.Diags {
				fmt.Fprintln(os.Stderr, d.String())
			}
			// Print preprocessed output to stdout.
			fmt.Fprint(os.Stdout, result.Output)
			if result.Output != "" && result.Output[len(result.Output)-1] != '\n' {
				fmt.Fprintln(os.Stdout)
			}
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
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}

	return nil
}
