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
	"os"
	"strings"

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
	fs.StringVar(&cfg.OPCUA, "opcua", "", "Serve the simulation over OPC UA on host:port (empty = no server)")
	if err := fs.Parse(args); err != nil {
		return simConfig{}, err
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
