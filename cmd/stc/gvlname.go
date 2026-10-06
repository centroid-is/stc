package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/spf13/cobra"
)

// gvlNameFlag is the flag that overrides the GVL name derived from the file
// basename. TwinCAT names a GVL after its .TcGVL object, which a flattened
// .st file loses, so the flag lets users restore the original name.
const gvlNameFlag = "gvl-name"

// errGVLNameArgs is returned when --gvl-name is combined with anything other
// than exactly one input file.
var errGVLNameArgs = errors.New("--gvl-name requires exactly one input file")

// addGVLNameFlag registers --gvl-name on cmd.
func addGVLNameFlag(cmd *cobra.Command) {
	cmd.Flags().String(gvlNameFlag, "", "Override the GVL name for a single input file (default: file basename)")
}

// validateGVLName rejects --gvl-name unless exactly one input file is given.
// It runs before any parsing so a long file list is never read. Under
// --format json the error is written to stdout as {"error": "..."} and the
// process exits 1; in text mode the error is returned for cobra to report
// together with the usage text.
func validateGVLName(cmd *cobra.Command, args []string, format string) error {
	if !cmd.Flags().Changed(gvlNameFlag) || len(args) == 1 {
		return nil
	}
	if format == "json" {
		out, _ := json.Marshal(map[string]string{"error": errGVLNameArgs.Error()})
		fmt.Fprintln(os.Stdout, string(out))
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		os.Exit(1)
	}
	return errGVLNameArgs
}

// applyGVLName renames the GVLDecls in file when --gvl-name is set. The value
// is sanitised to a valid identifier so fmt and emit output stays valid ST.
// Callers must apply it after parsing, never inside the incremental parse
// path, so a cache hit gets the same name as a cold run.
func applyGVLName(cmd *cobra.Command, file *ast.SourceFile) {
	if !cmd.Flags().Changed(gvlNameFlag) {
		return
	}
	name, _ := cmd.Flags().GetString(gvlNameFlag)
	ast.SetGVLName(file, ast.SanitizeGVLName(name))
}
