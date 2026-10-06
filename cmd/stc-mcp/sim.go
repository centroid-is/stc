package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/opcua/bind"
	"github.com/centroid-is/stc/pkg/opcua/opcuatest"
	"github.com/centroid-is/stc/pkg/projectload"
	"github.com/centroid-is/stc/pkg/symtree"
)

// This file holds the long-lived simulation behind the stc_sim_* and
// stc_opcua_browse tools (Phase 29 D-14..D-16): one project per MCP server
// process, stepped (never free-running) so agent runs stay deterministic.

const (
	maxStepCycles   = 1_000_000
	defaultDepth    = 2
	maxBrowseDepth  = 10
	remoteTimeout   = 10 * time.Second
	plc1NodeID      = "ns=4;s=PLC1"
	errNoSession    = "no simulation: start stc-mcp with --project <path> [--io ...]"
	errScenarioStub = "--scenario needs the Phase 27 scenario package, which this build does not include yet"
)

// simConfig is the session configuration from the stc-mcp flags.
type simConfig struct {
	Project  string   // .tsproj/.plcproj, a .st file or a directory of .st files
	IO       []string // EtherCATConfig exports (Device N.xml), globs already expanded
	Scenario string   // reserved for the Phase 27 scenario package (not wired yet)
	OPCUA    string   // host:port of the optional OPC UA server ("" = none)
	PKIDir   string   // OPC UA certificate directory ("" = user cache dir)
}

// force is a TcLinkTo-bound input held at a value: its raw bits are written
// into the master's input image before every Tick, so the IOBinder copies
// them into the variable like a real terminal would.
type force struct {
	slot ecat.Slot
	bits uint64
}

// simSession is the loaded project with its optional EtherCAT network and
// OPC UA server. Every exported method takes mu, so tool calls serialize.
type simSession struct {
	mu       sync.Mutex
	p        *interp.Project
	rt       *interp.Runtime
	analysis analyzer.AnalysisResult
	ecat     *projectload.ECat
	src      *bind.RuntimeSource
	space    *opcua.Space
	srv      *opcua.Server
	diags    []diag.Diagnostic
	forces   map[string]force // upper-cased variable path -> force
	cycles   int
}

// newSimSession loads cfg.Project (with STC_SIM defined, like stc serve),
// attaches cfg.IO and starts the OPC UA server when cfg.OPCUA is set.
func newSimSession(cfg simConfig) (*simSession, error) {
	if cfg.Project == "" {
		return nil, errors.New(errNoSession)
	}
	if cfg.Scenario != "" {
		return nil, errors.New(errScenarioStub)
	}
	spec, res, ds, err := projectload.Load([]string{cfg.Project}, map[string]bool{"STC_SIM": true})
	if err != nil {
		return nil, err
	}
	p, err := interp.LoadProject(spec)
	if err != nil {
		return nil, fmt.Errorf("initialisation error: %w", err)
	}
	s := &simSession{p: p, rt: p.Runtime(), analysis: res, forces: map[string]force{}}
	for _, d := range ds {
		if d.Severity != diag.Error {
			s.diags = append(s.diags, d)
		}
	}
	if len(cfg.IO) > 0 {
		e, eds, err := projectload.AttachECat(p, spec, cfg.IO)
		if err != nil {
			for _, d := range eds {
				if d.Severity == diag.Error {
					err = fmt.Errorf("%w\n%s", err, d.String())
				}
			}
			return nil, err
		}
		s.ecat = e
		s.diags = append(s.diags, eds...)
	}
	s.src = bind.NewRuntimeSource(s.rt)
	if cfg.OPCUA != "" {
		if err := s.startOPCUA(cfg); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// buildSpace builds the TF6100 address space of the analysed project once.
func (s *simSession) buildSpace() (*opcua.Space, error) {
	if s.space != nil {
		return s.space, nil
	}
	tree, err := symtree.Build(s.analysis)
	if err != nil {
		return nil, fmt.Errorf("symbol tree: %w", err)
	}
	s.space, _ = opcua.Build(bind.Root(tree), s.src)
	return s.space, nil
}

// startOPCUA publishes the address space over the session runtime. Writes
// from OPC UA clients queue in s.src and are applied by Step.
func (s *simSession) startOPCUA(cfg simConfig) error {
	space, err := s.buildSpace()
	if err != nil {
		return err
	}
	oc := opcua.DefaultConfig()
	oc.Endpoint = cfg.OPCUA
	oc.PKIDir = cfg.PKIDir
	srv, err := opcua.New(oc)
	if err != nil {
		return fmt.Errorf("opc ua server: %w", err)
	}
	if err := srv.Publish(space, s.src); err != nil {
		_ = srv.Stop()
		return fmt.Errorf("publishing address space: %w", err)
	}
	if err := srv.Start(); err != nil {
		_ = srv.Stop()
		return fmt.Errorf("starting opc ua server: %w", err)
	}
	s.srv = srv
	return nil
}

// Endpoint returns the OPC UA endpoint URL, or "" without a server.
func (s *simSession) Endpoint() string {
	if s.srv == nil {
		return ""
	}
	return s.srv.Endpoint()
}

// Close stops the OPC UA server.
func (s *simSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv == nil {
		return nil
	}
	err := s.srv.Stop()
	s.srv = nil
	return err
}

// stepResult is the stc_sim_step result.
type stepResult struct {
	Stepped          int      `json:"stepped"`
	Cycles           int      `json:"cycles"` // total since the session started
	SimTimeMS        int64    `json:"sim_time_ms"`
	ScenarioFailures []string `json:"scenario_failures"`
	WriteErrors      []string `json:"write_errors,omitempty"`
}

// Step runs n Ticks. Before each Tick it drains the OPC UA write queue and
// re-applies the input forces; a Tick error stops the run and is returned
// with the cycles done so far.
func (s *simSession) Step(n int) (stepResult, error) {
	if n <= 0 || n > maxStepCycles {
		return stepResult{}, fmt.Errorf("cycles must be between 1 and %d, got %d", maxStepCycles, n)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res := stepResult{ScenarioFailures: []string{}}
	var err error
	for i := 0; i < n; i++ {
		if werr := s.src.ApplyPending(); werr != nil {
			res.WriteErrors = append(res.WriteErrors, werr.Error())
		}
		s.applyForces()
		if err = s.p.Tick(); err != nil {
			err = fmt.Errorf("cycle %d: %w", s.cycles+1, err)
			break
		}
		s.cycles++
		res.Stepped++
	}
	res.Cycles = s.cycles
	res.SimTimeMS = s.p.Clock().Milliseconds()
	return res, err
}

// applyForces writes every forced input into its input image, in path order.
func (s *simSession) applyForces() {
	if s.ecat == nil || len(s.forces) == 0 {
		return
	}
	keys := make([]string, 0, len(s.forces))
	for k := range s.forces {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	imgs := s.ecat.Network.Images()
	for _, k := range keys {
		f := s.forces[k]
		if img := imgs.Get(f.slot.Master); img != nil {
			ecat.WriteBits(img.In, f.slot.Byte, f.slot.Bit, f.slot.BitLen, f.bits)
		}
	}
}

// readEntry is one stc_sim_read result; exactly one of Value and Error is set.
type readEntry struct {
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
}

// Read returns each path's value in Phase 22 ToJSON form, in paths order.
// An unknown path yields an error entry; the other paths still read.
func (s *simSession) Read(paths []string) []readEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]readEntry, 0, len(paths))
	for _, p := range paths {
		v, err := s.rt.Get(p)
		if err != nil {
			out = append(out, readEntry{Path: p, Error: err.Error()})
			continue
		}
		out = append(out, readEntry{Path: p, Value: s.rt.ToJSON(v)})
	}
	return out
}

// writeResult is the stc_sim_write result.
type writeResult struct {
	Path  string `json:"path"`
	Route string `json:"route"` // "input_force" or "runtime_set"
	Value any    `json:"value"`
}

// Write sets path to value (a JSON-decoded bool, number, string, array or
// object, coerced like the Phase 22 Runtime.Set). A TcLinkTo-bound input
// is also forced in its input image, so the next scans keep the value
// instead of the zero the terminal would deliver; everything else goes
// through Runtime.Set only. Both are visible to the next Read.
func (s *simSession) Write(path string, value any) (writeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slot, bound := s.inputSlot(path)
	if err := s.rt.Set(path, value); err != nil {
		return writeResult{}, fmt.Errorf("%s: %w", path, err)
	}
	v, err := s.rt.Get(path)
	if err != nil {
		return writeResult{}, fmt.Errorf("%s: %w", path, err)
	}
	res := writeResult{Path: path, Route: "runtime_set", Value: s.rt.ToJSON(v)}
	if bound {
		bits, err := rawBits(v, slot.BitLen)
		if err != nil {
			return writeResult{}, fmt.Errorf("%s: %w", path, err)
		}
		s.forces[strings.ToUpper(path)] = force{slot: slot, bits: bits}
		res.Route = "input_force"
	}
	return res, nil
}

// inputSlot returns the input slot path is linked to, if any.
func (s *simSession) inputSlot(path string) (ecat.Slot, bool) {
	if s.ecat == nil {
		return ecat.Slot{}, false
	}
	for _, b := range s.ecat.Bindings {
		if b.Slot.Dir == ecat.DirIn && strings.EqualFold(b.Var.Path, path) {
			return b.Slot, true
		}
	}
	return ecat.Slot{}, false
}

// rawBits encodes an elementary value as the bits of a bitLen-wide slot.
func rawBits(v interp.Value, bitLen int) (uint64, error) {
	switch v.Kind {
	case interp.ValBool:
		if v.Bool {
			return 1, nil
		}
		return 0, nil
	case interp.ValInt:
		return uint64(v.Int), nil
	case interp.ValReal:
		if bitLen == 32 {
			return uint64(math.Float32bits(float32(v.Real))), nil
		}
		return math.Float64bits(v.Real), nil
	}
	return 0, fmt.Errorf("cannot force a linked input of this type (only BOOL, integers and reals)")
}

// browseNode is one stc_opcua_browse result node.
type browseNode struct {
	NodeID     string        `json:"node_id"`
	BrowseName string        `json:"browse_name"`
	NodeClass  string        `json:"node_class"`
	DataType   string        `json:"data_type,omitempty"`
	Access     string        `json:"access,omitempty"`
	Children   []*browseNode `json:"children,omitempty"`
}

// clampDepth applies the default (0 -> 2) and the cap of 10.
func clampDepth(depth int) (int, error) {
	switch {
	case depth < 0:
		return 0, fmt.Errorf("depth must not be negative, got %d", depth)
	case depth == 0:
		return defaultDepth, nil
	case depth > maxBrowseDepth:
		return maxBrowseDepth, nil
	}
	return depth, nil
}

// nodePath turns "ns=4;s=GVL.x", "GVL.x" or "" into a Space path ("" = PLC1).
func nodePath(node string) string {
	node = strings.TrimSpace(node)
	if i := strings.Index(node, ";s="); i >= 0 && strings.HasPrefix(node, "ns=") {
		node = node[i+3:]
	}
	if strings.EqualFold(node, "PLC1") {
		return ""
	}
	return node
}

// Browse walks the session address space from node (default PLC1) to depth
// levels below it, in declaration order. It needs no network.
func (s *simSession) Browse(node string, depth int) (*browseNode, error) {
	depth, err := clampDepth(depth)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sp, err := s.buildSpace()
	if err != nil {
		return nil, err
	}
	kids := map[string][]int{}
	for i, n := range sp.Nodes {
		kids[n.Parent] = append(kids[n.Parent], i)
	}
	var build func(path string, d int) []*browseNode
	build = func(path string, d int) []*browseNode {
		if d == 0 {
			return nil
		}
		var out []*browseNode
		for _, i := range kids[path] {
			bn := specNode(sp.Nodes[i])
			bn.Children = build(sp.Nodes[i].Path, d-1)
			out = append(out, bn)
		}
		return out
	}
	path := nodePath(node)
	if path == "" {
		return &browseNode{NodeID: plc1NodeID, BrowseName: "PLC1", NodeClass: "Object", Children: build("", depth)}, nil
	}
	for _, n := range sp.Nodes {
		if strings.EqualFold(n.Path, path) {
			bn := specNode(n)
			bn.Children = build(n.Path, depth)
			return bn, nil
		}
	}
	return nil, fmt.Errorf("node %q is not in the address space", node)
}

// specNode converts a NodeSpec without its children.
func specNode(n opcua.NodeSpec) *browseNode {
	bn := &browseNode{NodeID: "ns=4;s=" + n.Path, BrowseName: n.Name, NodeClass: "Object"}
	if n.Type != nil {
		bn.DataType = n.Type.String()
	}
	if n.Class == opcua.NodeVariable {
		bn.NodeClass = "Variable"
		bn.Access = accessText(uint8(n.Access))
	}
	return bn
}

// accessText names an AccessLevel (0 is read/write, as in NodeSpec).
func accessText(a uint8) string {
	switch a & 3 {
	case 1:
		return "read"
	case 2:
		return "write"
	}
	return "read_write"
}

// browseRemote browses endpoint read-only (SecurityPolicy None, Anonymous)
// from node (default ns=4;s=PLC1) through opcuatest.Take and cuts the
// tree at depth. The dial and browse share one 10 s deadline.
func browseRemote(ctx context.Context, endpoint, node string, depth int) (*browseNode, error) {
	depth, err := clampDepth(depth)
	if err != nil {
		return nil, err
	}
	rootText := strings.TrimSpace(node)
	if rootText == "" || strings.EqualFold(rootText, "PLC1") {
		rootText = plc1NodeID
	} else if !strings.HasPrefix(rootText, "ns=") && !strings.HasPrefix(rootText, "i=") {
		rootText = "ns=4;s=" + rootText
	}
	root := ua.ParseNodeID(rootText)
	if root == nil {
		return nil, fmt.Errorf("invalid node id %q", node)
	}
	ctx, cancel := context.WithTimeout(ctx, remoteTimeout)
	defer cancel()
	c, err := client.Dial(ctx, endpoint, client.WithInsecureSkipVerify(),
		client.WithSecurityPolicyURI(ua.SecurityPolicyURINone, ua.MessageSecurityModeNone))
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", endpoint, err)
	}
	defer func() { _ = c.Close(context.Background()) }()
	snap, err := opcuatest.Take(ctx, c, root)
	if err != nil {
		return nil, fmt.Errorf("browsing %s: %w", endpoint, err)
	}
	return snapshotTree(snap, depth), nil
}

// snapshotTree rebuilds the browse tree of snap from its parent links,
// keeping depth levels below the root, children ordered by NodeId.
func snapshotTree(snap *opcuatest.Snapshot, depth int) *browseNode {
	kids := map[string][]opcuatest.Node{}
	root := &browseNode{NodeID: snap.Root, NodeClass: "Object"}
	for _, n := range snap.Nodes {
		if n.NodeID == snap.Root {
			root = snapNode(n)
			continue
		}
		kids[n.Parent] = append(kids[n.Parent], n)
	}
	var build func(id string, d int) []*browseNode
	build = func(id string, d int) []*browseNode {
		if d == 0 {
			return nil
		}
		var out []*browseNode
		for _, n := range kids[id] {
			bn := snapNode(n)
			bn.Children = build(n.NodeID, d-1)
			out = append(out, bn)
		}
		return out
	}
	root.Children = build(snap.Root, depth)
	return root
}

func snapNode(n opcuatest.Node) *browseNode {
	bn := &browseNode{NodeID: n.NodeID, BrowseName: stripNS(n.BrowseName), NodeClass: n.NodeClass}
	if n.DataType != "" {
		bn.DataType = opcuatest.DataTypeName(n.DataType)
	}
	if n.AccessLevel != nil {
		bn.Access = accessText(*n.AccessLevel)
	}
	return bn
}

// stripNS drops a "4:" namespace prefix from a BrowseName.
func stripNS(name string) string {
	if i := strings.IndexByte(name, ':'); i > 0 && !strings.ContainsAny(name[:i], " .") {
		return name[i+1:]
	}
	return name
}

// expandGlobs expands each pattern; a pattern without matches is kept as
// given so the loader reports the missing file.
func expandGlobs(patterns []string) []string {
	var out []string
	for _, p := range patterns {
		m, err := filepath.Glob(p)
		if err != nil || len(m) == 0 {
			out = append(out, p)
			continue
		}
		sort.Strings(m)
		out = append(out, m...)
	}
	return out
}

// simHost creates the session lazily on the first sim tool call, so a load
// error is a tool error and parse/check keep working.
type simHost struct {
	mu   sync.Mutex
	cfg  simConfig
	sess *simSession
}

// session returns the session, loading it on first use. Load errors are
// returned on every call until a load succeeds.
func (h *simHost) session() (*simSession, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sess != nil {
		return h.sess, nil
	}
	if h.cfg.Project == "" {
		return nil, errors.New(errNoSession)
	}
	s, err := newSimSession(h.cfg)
	if err != nil {
		return nil, fmt.Errorf("loading simulation: %w", err)
	}
	h.sess = s
	return s, nil
}

// close closes the session if it was created.
func (h *simHost) close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sess == nil {
		return nil
	}
	err := h.sess.Close()
	h.sess = nil
	return err
}

// sim is the process-wide session host; main sets its config from flags
// and tests replace it.
var sim = &simHost{}
