package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/opcua/bind"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/symtree"
	"github.com/centroid-is/stc/pkg/twincat"
	"github.com/centroid-is/stc/pkg/vendor"
	"github.com/spf13/cobra"
)

// defaultServeCycle is the scan cycle when neither --cycle nor a project
// task gives one.
const defaultServeCycle = 10 * time.Millisecond

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve [project.tsproj|project.plcproj|file.st|dir ...]",
		Short: "Run a project's scan and serve it over OPC UA (TF6100 address space)",
		Long: `Load a TwinCAT project or ST sources, run every PROGRAM in a free-running
scan paced by the wall clock, and serve the OPC.UA.DA-marked symbols over
OPC UA with the TwinCAT TF6100 address space (Objects/DeviceSet/PLC1,
namespace 4, NodeIds ns=4;s=<GVL>.<path>).

OPC UA reads come from one consistent scan image; writes are queued and
applied between scans. Analysis errors are reported but do not stop the
server, and a runtime error stops the scan while the address space keeps
serving the last values. The listener binds all interfaces.

Stops on SIGINT/SIGTERM, or after --run-for.`,
		RunE: runServe,
	}
	cmd.Flags().StringSlice("project", nil, "Project or ST sources to serve (alternative to positional arguments)")
	cmd.Flags().String("opcua", ":4840", "OPC UA listen address host:port (the listener binds all interfaces)")
	cmd.Flags().String("security", "none", "Security mode: none (SecurityPolicy None + Anonymous) or basic256sha256 (secure only)")
	cmd.Flags().String("cert", "", "Server certificate (DER or PEM); generated when empty")
	cmd.Flags().String("key", "", "Server private key; required with --cert")
	cmd.Flags().String("pki-dir", "", "Directory for generated certificates (default: user cache dir)")
	cmd.Flags().Duration("cycle", 0, "Scan cycle (default: the project's first task cycle, else 10ms)")
	cmd.Flags().Bool("realtime", false, "Pace the scan on a dedicated OS thread with absolute deadlines and count overruns")
	cmd.Flags().Duration("run-for", 0, "Stop after this duration (0 = until SIGINT/SIGTERM)")
	cmd.Flags().StringSliceP("define", "D", nil, "Define preprocessor symbols (can be repeated)")
	return cmd
}

// serveInfo is the --format json start-up line.
type serveInfo struct {
	Endpoint       string            `json:"endpoint"`
	NamespaceIndex uint16            `json:"namespace_index"`
	NodeCount      int               `json:"node_count"`
	Cycle          string            `json:"cycle"`
	Diagnostics    []diag.Diagnostic `json:"diagnostics"`
}

// servedProject is a loaded, analysed and instantiated project.
type servedProject struct {
	res   analyzer.AnalysisResult
	rt    *interp.Runtime
	cycle time.Duration // task cycle; 0 when unknown
	diags []diag.Diagnostic
}

func runServe(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	format, _ := cmd.Flags().GetString("format")
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

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
	runFor, _ := cmd.Flags().GetDuration("run-for")
	realtime, _ := cmd.Flags().GetBool("realtime")
	defineFlags, _ := cmd.Flags().GetStringSlice("define")
	defines := pipeline.ParseDefines(defineFlags)
	if defines == nil {
		defines = map[string]bool{}
	}
	defines["STC_SIM"] = true

	p, err := loadServeProject(inputs, defines)
	if err != nil {
		return err
	}
	if cycle == 0 {
		cycle = p.cycle
	}
	if cycle == 0 {
		cycle = defaultServeCycle
	}

	tree, err := symtree.Build(p.res)
	if err != nil {
		return fmt.Errorf("symbol tree: %w", err)
	}
	src := bind.NewRuntimeSource(p.rt)
	space, odiags := opcua.Build(bind.Root(tree), src)
	diags := append(errorsOnly(p.diags), odiags...)

	srv, err := opcua.New(cfg)
	if err != nil {
		return fmt.Errorf("opc ua server: %w", err)
	}
	if err := srv.Publish(space, src); err != nil {
		return fmt.Errorf("publishing address space: %w", err)
	}
	if err := srv.Start(); err != nil {
		return fmt.Errorf("starting opc ua server: %w", err)
	}
	defer func() { _ = srv.Stop() }()

	info := serveInfo{Endpoint: srv.Endpoint(), NamespaceIndex: srv.NamespaceIndex(),
		NodeCount: len(space.Nodes), Cycle: cycle.String(), Diagnostics: diags}
	if info.Diagnostics == nil {
		info.Diagnostics = []diag.Diagnostic{}
	}
	printServeInfo(out, errOut, format, info)

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if runFor > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, runFor)
		defer cancel()
	}

	loop := &scanLoop{rt: p.rt, src: src, cycle: cycle, realtime: realtime,
		warn: func(err error) { reportServe(errOut, format, "write_error", err) }}
	if err := loop.run(ctx); err != nil {
		reportServe(errOut, format, "scan_stopped", err)
		<-ctx.Done() // keep serving the last image
	}
	if realtime && format != "json" {
		fmt.Fprintf(errOut, "scan: %d cycles, %d overruns\n", loop.cycles, loop.overruns)
	}
	return nil
}

// serveConfig builds the OPC UA server configuration from the flags.
func serveConfig(cmd *cobra.Command) (opcua.Config, error) {
	cfg := opcua.DefaultConfig()
	cfg.Endpoint, _ = cmd.Flags().GetString("opcua")
	cfg.CertFile, _ = cmd.Flags().GetString("cert")
	cfg.KeyFile, _ = cmd.Flags().GetString("key")
	cfg.PKIDir, _ = cmd.Flags().GetString("pki-dir")
	cfg.SoftwareVersion = rootVersion(cmd)
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

// rootVersion returns the root command's version string.
func rootVersion(cmd *cobra.Command) string {
	if v := cmd.Root().Version; v != "" {
		return v
	}
	return "dev"
}

// errorsOnly keeps the error diagnostics: warnings of a large project would
// drown the start-up report.
func errorsOnly(ds []diag.Diagnostic) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range ds {
		if d.Severity == diag.Error {
			out = append(out, d)
		}
	}
	return out
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

// loadServeProject loads a TwinCAT project (a single .tsproj/.plcproj) or
// ST files and directories, analyses them and builds the runtime.
func loadServeProject(inputs []string, defines map[string]bool) (*servedProject, error) {
	if hasProjectArg(inputs) {
		if len(inputs) != 1 {
			return nil, errors.New("a project path (.tsproj or .plcproj) must be the only input")
		}
		m, ids, err := twincat.Import(inputs[0], twincat.Options{Defines: defines})
		if err != nil {
			return nil, fmt.Errorf("importing %s: %w", inputs[0], err)
		}
		var cfg *project.Config
		if cp, err := project.FindConfig(filepath.Dir(m.ProjectPath)); err == nil {
			cfg, _ = project.LoadConfig(cp)
		}
		res := analyzer.AnalyzeProject(m, cfg, defines)
		p := &servedProject{res: res, diags: append(ids, res.Diags...)}
		if len(m.Tasks) > 0 {
			p.cycle = m.Tasks[0].CycleTime
		}
		return p, p.instantiate()
	}

	files, err := expandSTInputs(inputs)
	if err != nil {
		return nil, err
	}
	var parsed []*ast.SourceFile
	var ds []diag.Diagnostic
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("cannot read file: %w", err)
		}
		r := pipeline.Parse(f, string(content), defines)
		parsed = append(parsed, r.File)
		ds = append(ds, r.Diags...)
	}
	var cfg *project.Config
	var libs []*ast.SourceFile
	if cp, err := project.FindConfig("."); err == nil {
		if cfg, err = project.LoadConfig(cp); err == nil && len(cfg.Build.LibraryPaths) > 0 {
			libs, _ = vendor.LoadLibraries(cfg, filepath.Dir(cp))
		}
	}
	res := analyzer.Analyze(parsed, cfg, analyzer.AnalyzeOpts{LibraryFiles: libs})
	p := &servedProject{res: res, diags: append(ds, res.Diags...)}
	return p, p.instantiate()
}

// instantiate builds the runtime from the analysed files.
func (p *servedProject) instantiate() error {
	rt, err := interp.NewRuntime(p.res.Files, interp.RuntimeOpts{LibraryFiles: p.res.LibraryFiles})
	if err != nil {
		return fmt.Errorf("initialisation error: %w", err)
	}
	p.rt = rt
	return nil
}

// expandSTInputs lists the .st files named by inputs, walking directories
// in lexical order.
func expandSTInputs(inputs []string) ([]string, error) {
	var out []string
	for _, in := range inputs {
		fi, err := os.Stat(in)
		if err != nil {
			return nil, err
		}
		if !fi.IsDir() {
			out = append(out, in)
			continue
		}
		var found []string
		err = filepath.WalkDir(in, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".st") {
				found = append(found, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if len(found) == 0 {
			return nil, fmt.Errorf("no .st files in %s", in)
		}
		sort.Strings(found)
		out = append(out, found...)
	}
	return out, nil
}

// scanLoop runs the free-running scan: queued OPC UA writes are applied,
// then one Tick of cycle runs, then the loop waits for the next cycle.
type scanLoop struct {
	rt       *interp.Runtime
	src      *bind.RuntimeSource
	cycle    time.Duration
	realtime bool
	warn     func(error)

	cycles, overruns int
}

// run ticks until ctx is done (nil) or a Tick fails (its error).
func (l *scanLoop) run(ctx context.Context) error {
	if l.realtime {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	ticker := time.NewTicker(l.cycle)
	defer ticker.Stop()
	next := time.Now()
	for {
		if err := l.src.ApplyPending(); err != nil {
			l.warn(err)
		}
		if err := l.rt.Tick(l.cycle); err != nil {
			return err
		}
		l.cycles++
		if l.realtime {
			next = next.Add(l.cycle)
			wait := time.Until(next)
			if wait <= 0 {
				l.overruns++
				next = time.Now()
				wait = 0
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
