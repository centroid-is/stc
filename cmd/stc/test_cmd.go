package main

import (
	"fmt"
	"os"

	stctesting "github.com/centroid-is/stc/pkg/testing"
	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test [dir]",
		Short: "Run ST unit tests",
		Long:  "Discover and run *_test.st test files in the specified directory (default: current directory).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			format, _ := cmd.Flags().GetString("format")

			result, err := stctesting.Run(dir)
			if err != nil {
				return err
			}

			switch format {
			case "junit":
				out, fmtErr := stctesting.FormatJUnit(result)
				if fmtErr != nil {
					return fmtErr
				}
				fmt.Fprint(os.Stdout, string(out))
			case "json":
				out, fmtErr := stctesting.FormatJSON(result)
				if fmtErr != nil {
					return fmtErr
				}
				fmt.Fprintln(os.Stdout, string(out))
			default: // "text"
				printTextResults(result)
			}

			if result.HasFailures() {
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				os.Exit(1)
			}
			return nil
		},
	}
	return cmd
}

// printTextResults prints human-readable test results to stdout.
func printTextResults(result *stctesting.RunResult) {
	for _, suite := range result.Suites {
		fmt.Printf("=== RUN  %s\n", suite.Name)
		for _, tr := range suite.Tests {
			if tr.Passed {
				fmt.Printf("--- PASS: %s (%.3fs)\n", tr.Name, tr.Duration.Seconds())
			} else {
				fmt.Printf("--- FAIL: %s (%.3fs)\n", tr.Name, tr.Duration.Seconds())
				for _, a := range tr.Assertions {
					if !a.Passed {
						if a.Position != "" {
							fmt.Printf("    %s: %s\n", a.Position, a.Message)
						} else {
							fmt.Printf("    %s\n", a.Message)
						}
					}
				}
				if tr.Error != "" {
					fmt.Printf("    error: %s\n", tr.Error)
				}
			}
		}
	}

	fmt.Println()
	if result.HasFailures() {
		fmt.Println("FAIL")
	} else {
		fmt.Println("ok")
	}
	fmt.Printf("%d tests, %d passed, %d failed\n", result.Total, result.Passed, result.Failed)
}
