package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/pipeline"
)

// sectionMarker matches the "// ---- <project>/<dir>/<Name>.<ext>" lines the
// probe flattener writes before every TwinCAT object.
var sectionMarker = regexp.MustCompile(`^// ---- (?:.*/)?([^/]+)\.Tc(?:GVL|DUT|POU)\s*$`)

// nameGVLs gives every GVL in a flattened probe file the object name from the
// nearest preceding section marker, so that ECT and ECT_Diag keep their own
// names instead of all GVLs collapsing onto the file basename.
func nameGVLs(t *testing.T, file *ast.SourceFile, src string) {
	t.Helper()
	markers := map[int]string{} // line -> object name
	var lines []int
	for i, l := range strings.Split(src, "\n") {
		if m := sectionMarker.FindStringSubmatch(l); m != nil {
			markers[i+1] = m[1]
			lines = append(lines, i+1)
		}
	}
	for _, d := range file.Declarations {
		g, ok := d.(*ast.GVLDecl)
		if !ok {
			continue
		}
		start := g.Span().Start.Line
		i := sort.SearchInts(lines, start+1) - 1 // last marker at or before start
		if i < 0 {
			t.Fatalf("GVL at line %d has no section marker", start)
		}
		name := markers[lines[i]]
		if g.Name == nil {
			g.Name = &ast.Ident{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent, NodeSpan: g.NodeSpan}}
		}
		g.Name.Name = name
		g.NameDerived = false
	}
}

// parseProbe parses one flattened probe file and fails on any error-level
// parse diagnostic, logging positions only (never source text).
func parseProbe(t *testing.T, dir, name string) (*ast.SourceFile, string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	src := string(data)
	res := pipeline.Parse(name, src, nil)
	errs := 0
	for _, d := range res.Diags {
		if d.Severity == diag.Error {
			errs++
			t.Errorf("%s:%d: parse error %s", name, d.Pos.Line, d.Code)
		}
	}
	if errs > 0 {
		t.Fatalf("%s: %d parse errors", name, errs)
	}
	nameGVLs(t, res.File, src)
	return res.File, src
}

// TestEcatST301Equivalence is the Phase 24 gate on real data: the ST301
// EtherCAT exports load into the same topology generate_gvl.py builds, every
// TIID^ target the generator wrote into ST301's GVLs exists in that topology,
// and every TcLinkTo resolves against the SVNCore DUTs and FB_ATV320 with zero
// errors. The exports and probe sources are proprietary and never committed,
// so the test runs only when STC_SILD_DIR and STC_PROBES_DIR are set and logs
// counts (and, on failure, at most 20 link paths), never source text.
func TestEcatST301Equivalence(t *testing.T) {
	sild := os.Getenv("STC_SILD_DIR")
	if sild == "" {
		t.Skip("STC_SILD_DIR not set; ST301 EtherCAT exports are local-only")
	}
	probes := os.Getenv("STC_PROBES_DIR")
	if probes == "" {
		t.Skip("STC_PROBES_DIR not set; ST301 probe sources are local-only")
	}

	// --- Topology: generator-equivalent load of Device 1..4 ----------------
	var exports []string
	for i := 1; i <= 4; i++ {
		exports = append(exports, filepath.Join(sild, "IO List from ethercat", "ST301", "Device "+string(rune('0'+i))+".xml"))
	}
	topo, err := ecat.LoadProject(exports...)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	wantSlaves := []int{23, 49, 34, 16}
	if len(topo.Masters) != len(wantSlaves) {
		t.Fatalf("masters = %d, want %d", len(topo.Masters), len(wantSlaves))
	}
	for i, m := range topo.Masters {
		t.Logf("%s: %d slaves, in %d bytes, out %d bytes", m.Name, len(m.Slaves), m.InBytes, m.OutBytes)
		if len(m.Slaves) != wantSlaves[i] {
			t.Errorf("%s: %d slaves, want %d", m.Name, len(m.Slaves), wantSlaves[i])
		}
	}

	// Nesting spot checks on Device 1.
	d1 := topo.Masters[0]
	byName := map[string]*ecat.Slave{}
	for _, s := range d1.Slaves {
		byName[s.Name] = s
	}
	const head = "ST301.A1.00 (EK1200)"
	if byName[head] == nil {
		t.Errorf("Device 1: %q missing", head)
	}
	for _, n := range []string{"ST301.A1.03 (EL1008)", "ST301.A1.09 (EL2912)"} {
		s := byName[n]
		switch {
		case s == nil:
			t.Errorf("Device 1: %q missing", n)
		case s.Parent == nil || s.Parent.Name != head:
			t.Errorf("Device 1: %q parent = %v, want %q", n, s.Parent, head)
		}
	}
	atv := 0
	for _, m := range topo.Masters {
		for _, s := range m.Slaves {
			if strings.HasPrefix(s.Model, "ATV320") {
				atv++
				if s.Parent != nil {
					t.Errorf("%s: ATV320 %q nested under %q, want master level", m.Name, s.Name, s.Parent.Name)
				}
			}
		}
	}
	t.Logf("ATV320 drives at master level: %d", atv)
	if atv == 0 {
		t.Errorf("no ATV320 slaves found")
	}

	// --- Generator equivalence: every TIID^ target exists ------------------
	types, _ := parseProbe(t, probes, "svncorecomponents.st")
	st301, _ := parseProbe(t, probes, "st301.st")
	paths := map[string]bool{}
	for _, p := range topo.Paths() {
		paths[p] = true
	}
	pragmas, targets := 0, 0
	var missing []string
	ast.Inspect(st301, func(n ast.Node) bool {
		a, ok := n.(*ast.Attribute)
		if !ok || !strings.EqualFold(a.Name, "TcLinkTo") {
			return true
		}
		pragmas++
		links, err := ecat.ParseTcLinkTo(a.Value)
		if err != nil {
			t.Errorf("st301.st:%d: malformed TcLinkTo: %v", a.Span().Start.Line, err)
			return true
		}
		for _, l := range links {
			targets++
			if !paths[l.Target] {
				missing = append(missing, l.Target)
			}
		}
		return true
	})
	t.Logf("st301.st: %d TcLinkTo pragmas, %d TIID^ targets, %d missing from topology (%d topology paths)", pragmas, targets, len(missing), len(paths))
	if pragmas != 114 {
		t.Errorf("TcLinkTo pragmas = %d, want 114", pragmas)
	}
	for i, p := range missing {
		if i == 20 {
			t.Errorf("... and %d more", len(missing)-20)
			break
		}
		t.Errorf("target not in topology: %q", p)
	}

	// --- Resolution gate: CollectLinks + Resolve with zero errors ----------
	files := []*ast.SourceFile{types, st301}
	vars, cdiags := ecat.CollectLinks(files)
	bindings, rdiags := ecat.Resolve(topo, vars)
	all := append(append([]diag.Diagnostic{}, cdiags...), rdiags...)
	warn := map[string]int{}
	errs := 0
	for _, d := range all {
		if d.Severity == diag.Error {
			errs++
			if errs <= 20 {
				t.Errorf("%s:%d: %s %s", filepath.Base(d.Pos.File), d.Pos.Line, d.Code, d.Message)
			}
			continue
		}
		warn[d.Code]++
	}
	codes := make([]string, 0, len(warn))
	for c := range warn {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		t.Logf("warning %s: %d", c, warn[c])
	}
	t.Logf("linked leaves %d, bindings %d, errors %d", len(vars), len(bindings), errs)
	if errs != 0 {
		t.Errorf("want 0 error diagnostics, got %d", errs)
	}

	perMaster := map[string]int{}
	var netID, adsAddr bool
	for _, b := range bindings {
		perMaster[b.Slot.Master]++
		netID = netID || strings.HasSuffix(b.Var.Link, "^InfoData^AmsNetId")
		adsAddr = adsAddr || strings.HasSuffix(b.Var.Link, "^InfoData^AdsAddr")
	}
	for _, m := range topo.Masters {
		t.Logf("%s: %d bindings", m.Name, perMaster[m.Name])
		if perMaster[m.Name] == 0 {
			t.Errorf("%s: no bindings", m.Name)
		}
	}
	if !netID {
		t.Errorf("no InfoData^AmsNetId binding")
	}
	if !adsAddr {
		t.Errorf("no InfoData^AdsAddr binding")
	}
}
