package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/opcua/bind"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/symtree"
	"github.com/spf13/cobra"
)

// stc serve is the union of the Phase 23-03 project runtime (tasks at their
// cycles, --io, --persist, --duration) and the Phase 28-04 OPC UA server
// (--opcua, --security, --cert, --key, --pki-dir). One loop runs the scan:
// interp.Project.Run, with queued OPC UA writes applied between Ticks.

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve <project.tsproj|project.plcproj|file.st|dir ...>",
		Short: "Run a whole project free-running, optionally served over OPC UA",
		Long: `Load a TwinCAT project or ST sources (files or directories) and run every
task's PROGRAMs at their task cycles, paced by the monotonic wall clock,
until SIGINT/SIGTERM or --duration (alias --run-for). Overruns are counted
per task.

--io attaches the EtherCAT network of Device N.xml exports to the project's
TcLinkTo links. --persist restores PERSISTENT/RETAIN variables from a JSON
state file at start, saves them every --persist-interval and once more on
stop.

--opcua host:port serves the OPC.UA.DA-marked symbols over OPC UA with the
TwinCAT TF6100 address space (Objects/DeviceSet/PLC1, namespace 4, NodeIds
ns=4;s=<GVL>.<path>). Reads come from one consistent scan image; writes are
queued and applied between scans; subscriptions sample the live scan at
their sampling interval. A runtime error then stops the scan while
the address space keeps serving the last values. The listener binds all
interfaces. Without --opcua no server is started.

On stop the final status (tasks, runs, overruns, sim_time_ns, diagnostics,
warnings) is printed; --format json prints it as one object, preceded by the
OPC UA start-up object when --opcua is given.`,
		RunE: runServe,
	}
	cmd.Flags().StringSlice("project", nil, "Project or ST sources to serve (alternative to positional arguments)")
	cmd.Flags().Duration("cycle", 0, "Override the cycle of the project's single task (default: the task cycles, else 10ms MAIN)")
	cmd.Flags().Bool("realtime", false, "Pin the scan to a dedicated OS thread and report cycles and overruns")
	cmd.Flags().Duration("duration", 0, "Stop after this wall time (0 = until SIGINT/SIGTERM)")
	cmd.Flags().Duration("run-for", 0, "Alias of --duration")
	cmd.Flags().StringSliceP("define", "D", nil, "Define preprocessor symbols (can be repeated)")
	cmd.Flags().String("opcua", "", "Serve OPC UA on host:port, e.g. :4840 (the listener binds all interfaces; empty = no server)")
	cmd.Flags().String("security", "none", "OPC UA security mode: none (SecurityPolicy None + Anonymous) or basic256sha256 (secure only)")
	cmd.Flags().String("cert", "", "OPC UA server certificate (DER or PEM); generated when empty")
	cmd.Flags().String("key", "", "OPC UA server private key; required with --cert")
	cmd.Flags().String("scenario", "", "Fire a scenario TOML file's steps as Ticks elapse; failed expects are reported as warnings when serve stops")
	cmd.Flags().String("pki-dir", "", "Directory for generated OPC UA certificates (default: user cache dir)")
	addProjectRunFlags(cmd)
	return cmd
}

// serveInfo is the --format json OPC UA start-up line.
type serveInfo struct {
	Endpoint       string            `json:"endpoint"`
	NamespaceIndex uint16            `json:"namespace_index"`
	NodeCount      int               `json:"node_count"`
	Cycle          string            `json:"cycle"`
	Diagnostics    []diag.Diagnostic `json:"diagnostics"`
}

func runServe(cmd *cobra.Command, args []string) error {
	format, _ := cmd.Flags().GetString("format")
	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	projFlag, _ := cmd.Flags().GetStringSlice("project")
	inputs := append(append([]string{}, projFlag...), args...)
	if len(inputs) == 0 {
		return errors.New("no project given: pass a .tsproj/.plcproj, .st files or directories")
	}
	cfg, err := serveConfig(cmd)
	if err != nil {
		return err
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

	r, err := projectSetup(cmd, inputs, defines, projectSetupOpts{Cycle: cycle}, errOut)
	if err != nil {
		return err
	}
	var cycles int
	r.BeforeTick = append(r.BeforeTick, func() { cycles++ })
	var sc *serveScenario
	if path, _ := cmd.Flags().GetString("scenario"); path != "" {
		if sc, err = loadServeScenario(r, path, errOut); err != nil {
			return err
		}
	}

	var srv *opcua.Server
	if cfg.Endpoint != "" {
		if srv, err = startOPCUA(r, cfg, out, errOut, format); err != nil {
			return err
		}
		defer func() { _ = srv.Stop() }()
	}

	if sc != nil {
		sc.install(r)
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}
	runErr := runProjectServe(ctx, r, duration, realtime)
	if sc != nil {
		sc.report(errOut, format, runErr)
	}
	if runErr != nil {
		if srv == nil {
			return fmt.Errorf("serve: %w", runErr)
		}
		reportServe(errOut, format, "scan_stopped", runErr)
		<-ctx.Done() // keep serving the last image
	}
	st, err := r.status(nil)
	if err != nil {
		return err
	}
	if realtime && format != "json" {
		var overruns uint64
		for _, t := range st.Tasks {
			overruns += t.Overruns
		}
		fmt.Fprintf(errOut, "scan: %d cycles, %d overruns\n", cycles, overruns)
	}
	return writeProjectStatus(out, errOut, format, st, nil)
}

// startOPCUA builds the TF6100 address space of r's analysed project over
// its Runtime, starts the server and prints the start-up report. Queued
// OPC UA writes are applied between Ticks through r.BeforeTick.
func startOPCUA(r *projectRunner, cfg opcua.Config, out, errOut io.Writer, format string) (*opcua.Server, error) {
	tree, err := symtree.Build(r.Analysis)
	if err != nil {
		return nil, fmt.Errorf("symbol tree: %w", err)
	}
	src := bind.NewRuntimeSource(r.P.Runtime())
	space, odiags := opcua.Build(bind.Root(tree), src)
	srv, err := opcua.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("opc ua server: %w", err)
	}
	if err := srv.Publish(space, src); err != nil {
		return nil, fmt.Errorf("publishing address space: %w", err)
	}
	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("starting opc ua server: %w", err)
	}
	r.BeforeTick = append(r.BeforeTick, func() {
		if err := src.ApplyPending(); err != nil {
			reportServe(errOut, format, "write_error", err)
		}
	})
	diags := odiags
	if diags == nil {
		diags = []diag.Diagnostic{}
	}
	printServeInfo(out, errOut, format, serveInfo{Endpoint: srv.Endpoint(),
		NamespaceIndex: srv.NamespaceIndex(), NodeCount: len(space.Nodes),
		Cycle: r.P.BaseTick().String(), Diagnostics: diags})
	return srv, nil
}

// serveConfig builds the OPC UA server configuration from the flags. The
// security mode is validated even when --opcua is not given.
func serveConfig(cmd *cobra.Command) (opcua.Config, error) {
	cfg := opcua.DefaultConfig()
	cfg.Endpoint, _ = cmd.Flags().GetString("opcua")
	cfg.CertFile, _ = cmd.Flags().GetString("cert")
	cfg.KeyFile, _ = cmd.Flags().GetString("key")
	cfg.PKIDir, _ = cmd.Flags().GetString("pki-dir")
	cfg.SoftwareVersion = cmd.Root().Version
	sec, _ := cmd.Flags().GetString("security")
	switch strings.ToLower(sec) {
	case "none":
	case "basic256sha256":
		cfg.AllowNone = false
		cfg.EnableBasic256Sha256 = true
	default:
		return cfg, fmt.Errorf("invalid --security %q: want none or basic256sha256", sec)
	}
	return cfg, nil
}

func printServeInfo(out, errOut io.Writer, format string, info serveInfo) {
	if format == "json" {
		b, _ := json.Marshal(info)
		fmt.Fprintln(out, string(b))
		return
	}
	for _, d := range info.Diagnostics {
		fmt.Fprintln(errOut, d.String())
	}
	fmt.Fprintf(out, "OPC UA server listening on %s (all interfaces)\n", info.Endpoint)
	fmt.Fprintf(out, "namespace %d, %d nodes, scan cycle %s\n", info.NamespaceIndex, info.NodeCount, info.Cycle)
}

// reportServe prints a runtime event: a JSON object on stderr under
// --format json, else an "error:" line.
func reportServe(errOut io.Writer, format, event string, err error) {
	if format == "json" {
		b, _ := json.Marshal(map[string]string{"event": event, "error": err.Error()})
		fmt.Fprintln(errOut, string(b))
		return
	}
	fmt.Fprintf(errOut, "error: %s: %v\n", strings.ReplaceAll(event, "_", " "), err)
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
