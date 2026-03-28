package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// stubCommand creates a stub subcommand that prints "not yet implemented".
// Each stub respects the --format flag: JSON output returns {"error": "not yet implemented"}.
func stubCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			msg := fmt.Sprintf("stc %s: not yet implemented", use)

			switch format {
			case "json":
				out, _ := json.MarshalIndent(map[string]string{
					"error": "not yet implemented",
				}, "", "  ")
				fmt.Fprintln(os.Stdout, string(out))
			default:
				fmt.Fprintln(os.Stderr, msg)
			}
			return nil
		},
	}
}

func newEmitCmd() *cobra.Command {
	return stubCommand("emit", "Emit vendor-specific ST")
}

func newLintCmd() *cobra.Command {
	return stubCommand("lint", "Lint ST source files")
}

func newFmtCmd() *cobra.Command {
	return stubCommand("fmt", "Format ST source files")
}
