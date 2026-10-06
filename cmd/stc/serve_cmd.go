package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/spf13/cobra"
)

// The flag names here match the Phase 28-04 OPC UA serve command
// (--project, --cycle, --realtime, --run-for, --define) so the two merge as
// a union: 28-04 adds --opcua/--security through serveExtensions and the
// Project loop (runProjectServe) replaces its Runtime scan loop.

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve <project.tsproj|project.plcproj|file.st...>",
		Short: "Run a whole project free-running against the wall clock",
		Long: `Load a TwinCAT project or ST sources and run every task's PROGRAMs at
their task cycles, paced by the monotonic wall clock, until SIGINT/SIGTERM or
--duration. Overruns are counted per task.

--io attaches the EtherCAT network of Device N.xml exports to the project's
TcLinkTo links. --persist restores PERSISTENT/RETAIN variables from a JSON
state file at start, saves them every --persist-interval and once more on
stop. On stop the final status (tasks, runs, overruns, sim_time_ns,
diagnostics, warnings) is printed; --format json prints it as one object.`,
		RunE: runServeProject,
	}
	cmd.Flags().StringSlice("project", nil, "Project or ST sources to serve (alternative to positional arguments)")
	cmd.Flags().Duration("cycle", 0, "Override the cycle of the project's single task (default: the task cycles, else 10ms MAIN)")
	cmd.Flags().Bool("realtime", false, "Pin the scan to a dedicated OS thread")
	cmd.Flags().Duration("duration", 0, "Stop after this wall time (0 = until SIGINT/SIGTERM)")
	cmd.Flags().Duration("run-for", 0, "Alias of --duration")
	cmd.Flags().StringSliceP("define", "D", nil, "Define preprocessor symbols (can be repeated)")
	addProjectRunFlags(cmd)
	return cmd
}

// serveExtensions is the hook for services that run beside the project
// loop (Phase 28-04: --opcua). It is called after the project is loaded and
// before the loop starts; an error stops serve.
func serveExtensions(cmd *cobra.Command, p *interp.Project) error {
	return nil
}

func runServeProject(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	projFlag, _ := cmd.Flags().GetStringSlice("project")
	inputs := append(append([]string{}, projFlag...), args...)
	if len(inputs) == 0 {
		return errors.New("no project given: pass a .tsproj/.plcproj or .st files")
	}
	cycle, _ := cmd.Flags().GetDuration("cycle")
	if cmd.Flags().Changed("cycle") && cycle <= 0 {
		return fmt.Errorf("--cycle must be positive, got %s", cycle)
	}
	duration, _ := cmd.Flags().GetDuration("duration")
	if cmd.Flags().Changed("run-for") {
		if cmd.Flags().Changed("duration") {
			return errors.New("--run-for is an alias of --duration; give only one")
		}
		duration, _ = cmd.Flags().GetDuration("run-for")
	}
	if duration < 0 {
		return fmt.Errorf("--duration must not be negative, got %s", duration)
	}
	realtime, _ := cmd.Flags().GetBool("realtime")
	defineFlags, _ := cmd.Flags().GetStringSlice("define")
	defines := pipeline.ParseDefines(defineFlags)
	if defines == nil {
		defines = map[string]bool{}
	}
	defines["STC_SIM"] = true
	cmd.SilenceUsage = true

	errOut := cmd.ErrOrStderr()
	r, err := projectSetup(cmd, inputs, defines, projectSetupOpts{Cycle: cycle}, errOut)
	if err != nil {
		return err
	}
	if err := serveExtensions(cmd, r.P); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runProjectServe(ctx, r, duration, realtime); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	st, err := r.status(nil)
	if err != nil {
		return err
	}
	return writeProjectStatus(cmd.OutOrStdout(), errOut, format, st, nil)
}

// runProjectServe is the serve loop: r runs free-running until duration of
// wall time (0: until ctx is done), with --persist saved periodically and on
// stop. realtime pins the loop to one OS thread. A stop by ctx is not an
// error; a Tick or state file error is.
func runProjectServe(ctx context.Context, r *projectRunner, duration time.Duration, realtime bool) error {
	if realtime {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	return r.runFree(ctx, duration, nil)
}
