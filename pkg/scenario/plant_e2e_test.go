package scenario

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureSources is the scenario fixture project: the Beckhoff stubs as
// libraries, demo_types.st, demo_ect.st as GVL ECT, ECT_Diag.st and
// MAIN.st (no tasks: MAIN every DefaultBaseTick).
func fixtureSources(t *testing.T) interp.ProjectSpec {
	t.Helper()
	var src interp.ProjectSpec
	for _, n := range []string{"tc2_system.st", "tc2_ethercat.st"} {
		src.LibraryFiles = append(src.LibraryFiles, parseFile(t, beckhoffStub(n)))
	}
	ect := parseFile(t, ecatFixture("demo_ect.st"))
	ast.SetGVLName(ect, "ECT")
	src.Files = []*ast.SourceFile{
		parseFile(t, ecatFixture("demo_types.st")),
		ect,
		parseFile(t, ecatFixture("scenario/ECT_Diag.st")),
		parseFile(t, ecatFixture("scenario/MAIN.st")),
	}
	return src
}

func fixturePlant(t *testing.T) *Plant {
	t.Helper()
	spec, err := BuildPlantSpec(fixtureSources(t), ioPaths(demoIO...))
	require.NoError(t, err)
	for _, d := range spec.Diagnostics {
		require.NotEqual(t, diag.Error, d.Severity, d.Message)
	}
	p, err := spec.New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.Empty(t, p.IOBinder().Errors()) })
	return p
}

func runJam(t *testing.T) (*Report, *Plant) {
	t.Helper()
	p := fixturePlant(t)
	s, diags := Load(ecatFixture("scenario/jam.toml"))
	for _, d := range diags {
		require.NotEqual(t, diag.Error, d.Severity, d.Message)
	}
	require.NotNil(t, s)
	ex := NewExecutor(s, p)
	for _, d := range ex.Prepare() {
		require.NotEqual(t, diag.Error, d.Severity, d.Message)
	}
	rep, err := ex.Run(0)
	require.NoError(t, err)
	return rep, p
}

func TestPlantE2EJam(t *testing.T) {
	rep, p := runJam(t)
	for _, a := range rep.Assertions {
		assert.True(t, a.Pass, "%+v", a)
	}
	var txt strings.Builder
	rep.Text(&txt)
	assert.Zero(t, rep.Failed(), txt.String())
	for _, d := range rep.Diagnostics {
		assert.NotEqual(t, diag.Error, d.Severity, d.Message)
	}
	assert.Greater(t, len(rep.Assertions), 6)

	polls, err := p.Read("ECT_Diag.nPolls")
	require.NoError(t, err)
	assert.Greater(t, polls, int64(5))
	n, err := p.Read("ECT_Diag.nSlaves")
	require.NoError(t, err)
	assert.EqualValues(t, 10, n)
	ok, err := p.Read("ECT_Diag.Device_1_Diag[3].p_stat_bOk")
	require.NoError(t, err)
	assert.Equal(t, true, ok, "untouched slave stays OK")
}

func TestPlantE2EDeterministic(t *testing.T) {
	a, _ := runJam(t)
	b, _ := runJam(t)
	ja, err := json.Marshal(a)
	require.NoError(t, err)
	jb, err := json.Marshal(b)
	require.NoError(t, err)
	assert.Equal(t, string(ja), string(jb))
}
