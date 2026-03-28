// Package main provides the stc CLI binary — an IEC 61131-3 Structured Text
// compiler toolchain.
package main

import (
	"fmt"
	"os"

	"github.com/centroid-is/stc/pkg/version"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "stc",
		Short: "IEC 61131-3 Structured Text compiler toolchain",
		Long:  "stc is a compiler toolchain for IEC 61131-3 Structured Text with CODESYS OOP extensions.",
		Version: version.String(),
	}

	rootCmd.PersistentFlags().StringP("format", "f", "text", "Output format: text, json")

	rootCmd.AddCommand(
		newParseCmd(),
		newCheckCmd(),
		newTestCmd(),
		newSimCmd(),
		newEmitCmd(),
		newLintCmd(),
		newFmtCmd(),
		newPpCmd(),
		newLspCmd(),
	)

	return rootCmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
