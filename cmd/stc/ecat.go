package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/spf13/cobra"
)

func newEcatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ecat",
		Short: "EtherCAT topology and TcLinkTo link tools",
	}
	validate := &cobra.Command{
		Use:   "validate --io <Device N.xml>... <file.st>...",
		Short: "Resolve TcLinkTo links in ST sources against EtherCAT exports",
		Long: `Load TwinCAT EtherCATConfig exports (Device N.xml), collect every
TcLinkTo-linked variable in the given ST files and resolve it to a process
image slot. GVL files are named after their file name. Reports unresolved
targets (ECAT001), undeclared members (ECAT002), size mismatches (ECAT003),
direction mismatches (ECAT004), duplicate bindings (ECAT005, warning) and
malformed values (ECAT006). Exit code 1 if any error is reported.`,
		Args: cobra.MinimumNArgs(1),
		RunE: runEcatValidate,
	}
	validate.Flags().StringSlice("io", nil, "EtherCATConfig export file (repeatable)")
	validate.Flags().StringSliceP("define", "D", nil, "Define preprocessor symbols (can be repeated)")
	_ = validate.MarkFlagRequired("io")
	cmd.AddCommand(validate)
	return cmd
}

// ecatBindingJSON is one resolved binding in JSON output.
type ecatBindingJSON struct {
	Var      string `json:"var"`
	Link     string `json:"link"`
	Master   string `json:"master"`
	Dir      string `json:"dir"`
	Byte     int    `json:"byte"`
	Bit      int    `json:"bit"`
	BitLen   int    `json:"bitLen"`
	TypeName string `json:"typeName"`
}

type ecatImageJSON struct {
	InBytes  int `json:"inBytes"`
	OutBytes int `json:"outBytes"`
}

type ecatValidateJSON struct {
	Bindings    []ecatBindingJSON        `json:"bindings"`
	Diagnostics []diag.Diagnostic        `json:"diagnostics"`
	Images      map[string]ecatImageJSON `json:"images"`
}

func runEcatValidate(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	ioFiles, _ := cmd.Flags().GetStringSlice("io")
	defineFlags, _ := cmd.Flags().GetStringSlice("define")
	cmd.SilenceUsage = true

	topo, err := ecat.LoadProject(ioFiles...)
	if err != nil {
		ecatFail(cmd, format, err)
	}

	var files []*ast.SourceFile
	var diags []diag.Diagnostic
	defines := pipeline.ParseDefines(defineFlags)
	for _, path := range args {
		src, err := os.ReadFile(path)
		if err != nil {
			ecatFail(cmd, format, err)
		}
		res := pipeline.Parse(path, string(src), defines)
		diags = append(diags, res.Diags...)
		base := filepath.Base(path)
		ast.SetGVLName(res.File, ast.SanitizeGVLName(strings.TrimSuffix(base, filepath.Ext(base))))
		files = append(files, res.File)
	}

	vars, collectDiags := ecat.CollectLinks(files)
	bindings, resolveDiags := ecat.Resolve(topo, vars)
	diags = append(diags, collectDiags...)
	diags = append(diags, resolveDiags...)
	ecat.SortDiagnostics(diags)

	errs, warns := 0, 0
	for _, d := range diags {
		switch d.Severity {
		case diag.Error:
			errs++
		case diag.Warning:
			warns++
		}
	}

	out := cmd.OutOrStdout()
	if format == "json" {
		res := ecatValidateJSON{
			Bindings:    []ecatBindingJSON{},
			Diagnostics: diags,
			Images:      map[string]ecatImageJSON{},
		}
		if res.Diagnostics == nil {
			res.Diagnostics = []diag.Diagnostic{}
		}
		for _, b := range bindings {
			res.Bindings = append(res.Bindings, ecatBindingJSON{
				Var: b.Var.Path, Link: b.Var.Link, Master: b.Slot.Master, Dir: b.Slot.Dir.String(),
				Byte: b.Slot.Byte, Bit: b.Slot.Bit, BitLen: b.Slot.BitLen, TypeName: b.Var.TypeName,
			})
		}
		for _, m := range topo.Masters {
			res.Images[m.Name] = ecatImageJSON{InBytes: m.InBytes, OutBytes: m.OutBytes}
		}
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(out, string(data))
	} else {
		writeEcatTable(out, bindings, diags, errs, warns)
	}
	if errs > 0 {
		os.Exit(1)
	}
	return nil
}

func writeEcatTable(out io.Writer, bindings []ecat.Binding, diags []diag.Diagnostic, errs, warns int) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "VARIABLE\tDIR\tMASTER\tBYTE.BIT\tBITS\tLINK")
	for _, b := range bindings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d.%d\t%d\t%s\n", b.Var.Path, b.Slot.Dir, b.Slot.Master, b.Slot.Byte, b.Slot.Bit, b.Slot.BitLen, b.Var.Link)
	}
	tw.Flush()
	for _, d := range diags {
		fmt.Fprintf(out, "%s: %s: %s %s\n", d.Pos, d.Severity, d.Code, d.Message)
	}
	fmt.Fprintf(out, "%d bindings, %d errors, %d warnings\n", len(bindings), errs, warns)
}

// ecatFail reports a load or read error and exits 1.
func ecatFail(cmd *cobra.Command, format string, err error) {
	if format == "json" {
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	os.Exit(1)
}
