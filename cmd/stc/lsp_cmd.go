package main

import (
	"github.com/centroid-is/stc/pkg/lsp"
	"github.com/spf13/cobra"
)

func newLspCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lsp",
		Short: "Start the LSP server",
		Long: `Start the STC Language Server Protocol server on stdio.

The LSP server provides real-time diagnostics, formatting, and other
language features for Structured Text files. It is designed to be
launched by editors like VS Code.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return lsp.Run()
		},
	}
}
