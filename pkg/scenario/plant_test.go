package scenario

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	dev1   = "Device 1 (EtherCAT)"
	linkI1 = "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 1^Input"
	linkO1 = "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 1^Output"
)

func ecatFixture(name string) string {
	return filepath.Join("..", "..", "tests", "ecat_fixtures", name)
}

func beckhoffStub(name string) string {
	return filepath.Join("..", "..", "stdlib", "vendor", "beckhoff", name)
}

// parseST parses src and fails on error diagnostics.
func parseST(t *testing.T, name, src string) *ast.SourceFile {
	t.Helper()
	res := pipeline.Parse(name, src, nil)
	for _, d := range res.Diags {
		require.NotEqual(t, diag.Error, d.Severity, "%s: %s", name, d.Message)
	}
	return res.File
}

func parseFile(t *testing.T, path string) *ast.SourceFile {
	t.Helper()
	src, err := os.ReadFile(path)
	require.NoError(t, err)
	return parseST(t, filepath.Base(path), string(src))
}

// demoSources is the Beckhoff stubs as libraries plus demo_types.st,
// demo_ect.st (GVL ECT) and extra sources (MAIN when none is given).
func demoSources(t *testing.T, extra ...string) interp.ProjectSpec {
	t.Helper()
	var src interp.ProjectSpec
	for _, n := range []string{"tc2_system.st", "tc2_ethercat.st"} {
		src.LibraryFiles = append(src.LibraryFiles, parseFile(t, beckhoffStub(n)))
	}
	types := parseFile(t, ecatFixture("demo_types.st"))
	ect := parseFile(t, ecatFixture("demo_ect.st"))
	ast.SetGVLName(ect, "ECT")
	src.Files = []*ast.SourceFile{types, ect}
	if len(extra) == 0 {
		extra = []string{"PROGRAM MAIN\nECT.CN01();\nEND_PROGRAM\n"}
	}
	for i, s := range extra {
		src.Files = append(src.Files, parseST(t, "extra"+string(rune('A'+i))+".st", s))
	}
	return src
}

var demoIO = []string{"Demo Device 1.xml", "Demo Device 2.xml"}

func ioPaths(names ...string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = ecatFixture(n)
	}
	return out
}

// demoPlant builds a Plant on Demo Device 1/2.
func demoPlant(t *testing.T, extra ...string) *Plant {
	t.Helper()
	spec, err := BuildPlantSpec(demoSources(t, extra...), ioPaths(demoIO...))
	require.NoError(t, err)
	p, err := spec.New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.Empty(t, p.IOBinder().Errors()) })
	return p
}

func TestPlantSpecBuild(t *testing.T) {
	spec, err := BuildPlantSpec(demoSources(t), ioPaths(demoIO...))
	require.NoError(t, err)
	require.NotNil(t, spec.Topology)
	assert.NotEmpty(t, spec.Bindings)
	for _, d := range spec.Diagnostics {
		assert.NotEqual(t, diag.Error, d.Severity, d.Message)
	}
	p, err := spec.New()
	require.NoError(t, err)
	assert.Equal(t, DefaultBaseTick, p.BaseTick())
	assert.NotNil(t, p.Network())
	assert.NotNil(t, p.Runtime())
	assert.Same(t, p.Network(), p.Runtime().Network())
}

func TestPlantSpecUnresolvedLink(t *testing.T) {
	src := demoSources(t)
	src.Files = append(src.Files, parseFile(t, ecatFixture("demo_bad.st")))
	spec, err := BuildPlantSpec(src, ioPaths(demoIO...))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ECAT001")
	found := false
	for _, d := range spec.Diagnostics {
		found = found || d.Code == "ECAT001"
	}
	assert.True(t, found)

	_, err = BuildPlantSpec(src, ioPaths("missing.xml"))
	assert.Error(t, err)
}

func TestPlantSpecNoIO(t *testing.T) {
	src := demoSources(t)
	src.Tasks = []interp.TaskSpec{{Name: "T", Cycle: time.Millisecond, Programs: []string{"MAIN"}}}
	spec, err := BuildPlantSpec(src, nil)
	require.NoError(t, err)
	p, err := spec.New()
	require.NoError(t, err)
	assert.Nil(t, p.Network())
	assert.Nil(t, p.IOBinder())
	assert.Equal(t, time.Millisecond, p.BaseTick())
	require.NoError(t, p.Tick())
	assert.Equal(t, time.Millisecond, p.Clock())
}

func TestPlantSpecIndependentPlants(t *testing.T) {
	spec, err := BuildPlantSpec(demoSources(t), ioPaths(demoIO...))
	require.NoError(t, err)
	a, err := spec.New()
	require.NoError(t, err)
	b, err := spec.New()
	require.NoError(t, err)
	require.NoError(t, a.Network().ForceInput(linkI1, 1))
	_, forced := b.Network().Forced(linkI1)
	assert.False(t, forced)
	require.NoError(t, a.Tick())
	require.NoError(t, b.Tick())
	va, err := a.Runtime().Get("ECT.A1_01.I1")
	require.NoError(t, err)
	vb, err := b.Runtime().Get("ECT.A1_01.I1")
	require.NoError(t, err)
	assert.True(t, va.Bool)
	assert.False(t, vb.Bool)
}

func TestPlantSpecRuntimeError(t *testing.T) {
	src := demoSources(t, "PROGRAM MAIN\nVAR a : ARRAY[1..UNKNOWN_CONST] OF INT; END_VAR\nEND_PROGRAM\n")
	spec, err := BuildPlantSpec(src, nil)
	require.NoError(t, err)
	_, err = spec.New()
	assert.Error(t, err)
}
