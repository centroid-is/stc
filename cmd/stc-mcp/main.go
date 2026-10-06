// Package main provides the stc-mcp binary, an MCP server that exposes
// all STC toolchain operations as MCP tools over stdio transport.
//
// With --project it also hosts one long-lived, stepped simulation that the
// stc_sim_step, stc_sim_read, stc_sim_write and stc_opcua_browse tools drive.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// stringList is a repeatable string flag; each value may hold a
// comma-separated list.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }

func (l *stringList) Set(v string) error {
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			*l = append(*l, s)
		}
	}
	return nil
}

// parseFlags reads the simulation flags. --io values are glob-expanded.
func parseFlags(args []string, errOut io.Writer) (simConfig, error) {
	fs := flag.NewFlagSet("stc-mcp", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var cfg simConfig
	var io stringList
	fs.StringVar(&cfg.Project, "project", "", "Project to simulate: .tsproj/.plcproj, a .st file or a directory of .st files")
	fs.Var(&io, "io", "EtherCATConfig export (Device N.xml) attached to the project's TcLinkTo links (repeatable, globs allowed)")
	fs.StringVar(&cfg.Scenario, "scenario", "", "Scenario TOML file whose steps fire as stc_sim_step advances the scan")
	fs.StringVar(&cfg.OPCUA, "opcua", "", "Serve the simulation over OPC UA on host:port; a bare :port or port binds 127.0.0.1, give 0.0.0.0:port for all interfaces. The server starts with the simulation, on the first stc_sim_* or stc_opcua_browse call (empty = no server)")
	fs.StringVar(&cfg.Security, "security", "none", "OPC UA security mode: none (SecurityPolicy None + Anonymous) or basic256sha256 (secure only, certificate identity)")
	allowAnon := fs.Bool("allow-anonymous", true, "Accept anonymous OPC UA clients; with --security basic256sha256 it defaults to false")
	allowAnonWrite := fs.Bool("allow-anonymous-write", true, "Let anonymous OPC UA clients write (false: browse, read and subscribe only)")
	if err := fs.Parse(args); err != nil {
		return simConfig{}, err
	}
	cfg.NoAnonymous, cfg.NoAnonymousWrite = !*allowAnon, !*allowAnonWrite
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "allow-anonymous" {
			cfg.AnonymousSet = true
		}
	})
	cfg.Warn = errOut
	if cfg.OPCUA != "" {
		ep, err := loopbackDefault(cfg.OPCUA)
		if err != nil {
			return simConfig{}, err
		}
		cfg.OPCUA = ep
		// Check the security flags now rather than on the first sim call.
		oc := opcua.DefaultConfig()
		oc.AllowAnonymous = !cfg.NoAnonymous
		if err := oc.ApplySecurity(cfg.Security, cfg.AnonymousSet); err != nil {
			return simConfig{}, err
		}
	}
	if fs.NArg() > 0 {
		return simConfig{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if cfg.Project == "" && (len(io) > 0 || cfg.Scenario != "" || cfg.OPCUA != "") {
		return simConfig{}, fmt.Errorf("--io, --scenario and --opcua need --project")
	}
	cfg.IO = expandGlobs(io)
	return cfg, nil
}

// loopbackDefault binds a port-only --opcua value (":4840" or "4840") to
// 127.0.0.1, so the agent's simulation is not exposed to the network
// unless a host is given.
func loopbackDefault(ep string) (string, error) {
	host, port, err := net.SplitHostPort(ep)
	if err != nil {
		if _, perr := strconv.ParseUint(ep, 10, 16); perr != nil {
			return "", fmt.Errorf("invalid --opcua %q: want host:port, :port or port", ep)
		}
		return net.JoinHostPort("127.0.0.1", ep), nil
	}
	if host == "" {
		return net.JoinHostPort("127.0.0.1", port), nil
	}
	return ep, nil
}

// newServer builds the MCP server with every tool registered.
func newServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "stc-mcp",
		Version: "1.0.0",
	}, nil)
	registerTools(server)
	return server
}

func main() {
	cfg, err := parseFlags(os.Args[1:], os.Stderr)
	if err != nil {
		if err != flag.ErrHelp {
			fmt.Fprintln(os.Stderr, "stc-mcp:", err)
		}
		os.Exit(2)
	}
	sim = &simHost{cfg: cfg}
	defer func() { _ = sim.close() }()

	if err := newServer().Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		_ = sim.close()
		log.Fatal(err)
	}
}
