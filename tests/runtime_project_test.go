package tests

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Phase 23 gate: the four success criteria on in-repo fixtures (always on)
// and the real ST301 project with its EtherCAT network (STC_SILD_DIR).

var (
	stcBinOnce sync.Once
	stcBin     string
	stcBinErr  error
)

// stcBinary builds cmd/stc once per test binary.
func stcBinary(t *testing.T) string {
	t.Helper()
	stcBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "stc-gate-")
		if err != nil {
			stcBinErr = err
			return
		}
		stcBin = filepath.Join(dir, "stc")
		out, err := exec.Command("go", "build", "-o", stcBin, "../cmd/stc").CombinedOutput()
		if err != nil {
			stcBinErr = err
			stcBin = string(out)
		}
	})
	require.NoError(t, stcBinErr, stcBin)
	return stcBin
}

// gateStatus is the subset of the `stc sim` project JSON the gate checks.
type gateStatus struct {
	Cycles    int   `json:"cycles"`
	SimTimeNS int64 `json:"sim_time_ns"`
	Tasks     []struct {
		Name     string `json:"name"`
		CycleNS  int64  `json:"cycle_ns"`
		Runs     uint64 `json:"runs"`
		Overruns uint64 `json:"overruns"`
	} `json:"tasks"`
	Get         map[string]any `json:"get"`
	Diagnostics []struct {
		Severity string `json:"severity"`
		Code     string `json:"code"`
	} `json:"diagnostics"`
	Warnings []string `json:"warnings"`
}

// simJSON runs `stc sim args... --format json` and returns stdout and the
// decoded status; a non-zero exit fails the test.
func simJSON(t *testing.T, args ...string) (string, gateStatus) {
	t.Helper()
	cmd := exec.Command(stcBinary(t), append(append([]string{"sim"}, args...), "--format", "json")...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	require.NoError(t, err, "stc sim %v: %s", args, stderr.String())
	var st gateStatus
	require.NoError(t, json.Unmarshal(out, &st), string(out))
	return string(out), st
}

const gateDemo = "../pkg/twincat/testdata/sln/Demo/Demo solution.tsproj"

func TestProjectRuntimeGate(t *testing.T) {
	t.Run("SC1 deterministic project run", func(t *testing.T) {
		out1, st := simJSON(t, "--project", gateDemo, "--cycles", "1000")
		out2, _ := simJSON(t, "--project", gateDemo, "--cycles", "1000")
		assert.Equal(t, out1, out2, "two runs give identical JSON")
		assert.Equal(t, int64(1_000_000_000), st.SimTimeNS)
		require.Len(t, st.Tasks, 1)
		assert.Equal(t, int64(1_000_000), st.Tasks[0].CycleNS)
		assert.Equal(t, uint64(1000), st.Tasks[0].Runs)
	})

	t.Run("SC2 free-running paced by the wall clock", func(t *testing.T) {
		p, err := interp.LoadProject(interp.ProjectSpec{
			Files: []*ast.SourceFile{gateParse(t, "main.st", "PROGRAM MAIN\nVAR n : DINT; END_VAR\nn := n + 1;\nEND_PROGRAM\n")},
			Tasks: []interp.TaskSpec{{Name: "PlcTask", Cycle: time.Millisecond, Programs: []string{"MAIN"}}},
		})
		require.NoError(t, err)
		ticks := 0
		require.NoError(t, p.Run(context.Background(), interp.RunOpts{Duration: time.Second,
			OnTick: func(time.Duration) { ticks++ }}))
		assert.GreaterOrEqual(t, ticks, 800)
		assert.LessOrEqual(t, ticks, 1050)
	})

	t.Run("SC3 persistent variables survive a restart", func(t *testing.T) {
		dir := t.TempDir()
		gvl, main := filepath.Join(dir, "GVL_Cfg.st"), filepath.Join(dir, "main.st")
		require.NoError(t, os.WriteFile(gvl, []byte("VAR_GLOBAL PERSISTENT\n\tp_cfg_Speed : REAL;\nEND_VAR\n"), 0o644))
		require.NoError(t, os.WriteFile(main, []byte("PROGRAM MAIN\nEND_PROGRAM\n"), 0o644))
		state := filepath.Join(dir, "state.json")
		simJSON(t, gvl, main, "--cycles", "1", "--persist", state, "--set", "GVL_Cfg.p_cfg_Speed=42.5")
		_, st := simJSON(t, gvl, main, "--cycles", "1", "--persist", state, "--get", "GVL_Cfg.p_cfg_Speed")
		assert.Equal(t, 42.5, st.Get["GVL_Cfg.p_cfg_Speed"])
	})

	t.Run("SC4 AT variables decode by declared type", func(t *testing.T) {
		p, err := interp.LoadProject(interp.ProjectSpec{Files: []*ast.SourceFile{gateParse(t, "GVL.st", `TYPE E_Mode : (Off := 0, Auto := 2) INT; END_TYPE
TYPE ST_Card :
STRUCT
	ok  AT %I* : BOOL;
	raw AT %I* : INT;
END_STRUCT
END_TYPE
VAR_GLOBAL
	ai   AT %I* : INT;
	ar   AT %I* : REAL;
	mode AT %I* : E_Mode;
	card : ST_Card;
	ao   AT %Q* : INT;
END_VAR
PROGRAM MAIN
GVL.ao := GVL.ai;
END_PROGRAM
`)}})
		require.NoError(t, err)
		slot := func(path string) int {
			a, _, ok := p.IOSlot(path)
			require.True(t, ok, path)
			return a.ByteOffset
		}
		require.NoError(t, p.SetIOBytes('I', slot("GVL.ai"), []byte{0xFB, 0xFF}))
		f := math.Float32bits(1.25)
		require.NoError(t, p.SetIOBytes('I', slot("GVL.ar"), []byte{byte(f), byte(f >> 8), byte(f >> 16), byte(f >> 24)}))
		require.NoError(t, p.SetIOBytes('I', slot("GVL.mode"), []byte{2, 0}))
		require.NoError(t, p.SetIOBytes('I', slot("GVL.card.ok"), []byte{1}))
		require.NoError(t, p.SetIOBytes('I', slot("GVL.card.raw"), []byte{0xFE, 0xFF}))
		require.NoError(t, p.Tick())
		get := func(path string) interp.Value {
			v, err := p.Runtime().Get(path)
			require.NoError(t, err, path)
			return v
		}
		assert.Equal(t, int64(-5), get("GVL.ai").Int)
		assert.Equal(t, 1.25, get("GVL.ar").Real)
		assert.Equal(t, int64(2), get("GVL.mode").Int)
		assert.True(t, get("GVL.card.ok").Bool)
		assert.Equal(t, int64(-2), get("GVL.card.raw").Int)
		out, err := p.IOBytes('Q', slot("GVL.ao"), 2)
		require.NoError(t, err)
		assert.Equal(t, []byte{0xFB, 0xFF}, out, "INT -5 encodes back to FB FF")
	})
}

func gateParse(t *testing.T, name, src string) *ast.SourceFile {
	t.Helper()
	res := parser.Parse(name, src)
	require.Empty(t, res.Diags, name)
	return res.File
}

// TestST301Runtime runs the imported ST301 for 1000 cycles of its 1 ms
// PlcTask with the four EtherCAT exports attached. ST301 is local-only, so
// the test needs STC_SILD_DIR; it asserts counts and values only.
func TestST301Runtime(t *testing.T) {
	sild := os.Getenv("STC_SILD_DIR")
	if sild == "" {
		t.Skip("STC_SILD_DIR not set; ST301 sources and EtherCAT exports are local-only")
	}
	var exports []string
	args := []string{"--project", filepath.Join(sild, "ST301", "ST301 solution.tsproj")}
	for i := 1; i <= 4; i++ {
		x := filepath.Join(sild, "IO List from ethercat", "ST301", "Device "+string(rune('0'+i))+".xml")
		exports = append(exports, x)
		args = append(args, "--io", x)
	}
	args = append(args, "--cycles", "1000", "--get", "ECT_Diag.Device_1_SlaveCount")

	out1, st := simJSON(t, args...)
	out2, _ := simJSON(t, args...)
	assert.Equal(t, out1, out2, "two runs give identical JSON")
	assert.Equal(t, 1000, st.Cycles)
	assert.Equal(t, int64(1_000_000_000), st.SimTimeNS)
	require.Len(t, st.Tasks, 1)
	assert.Equal(t, uint64(1000), st.Tasks[0].Runs)
	for _, d := range st.Diagnostics {
		assert.NotEqual(t, "error", d.Severity, d.Code)
	}
	for _, w := range st.Warnings {
		assert.NotContains(t, w, "io:", "I/O binding errors")
	}

	// Device_1_SlaveCount is the generator's configured count, a constant
	// initialiser in ECT_Diag. The ST301 GVLs were generated from an older
	// export than the Device*.xml now on disk (22 vs 23 slaves on Device 1),
	// so the run must keep the configured value, and the network's count is
	// logged for comparison.
	gvl, err := os.ReadFile(filepath.Join(sild, "ST301", "ST301", "GVLs", "ECT_Diag.TcGVL"))
	require.NoError(t, err)
	m := regexp.MustCompile(`Device_1_SlaveCount\s*:\s*UINT\s*:=\s*(\d+)`).FindSubmatch(gvl)
	require.NotNil(t, m, "configured Device_1_SlaveCount")
	want, _ := strconv.Atoi(string(m[1]))
	assert.Equal(t, float64(want), st.Get["ECT_Diag.Device_1_SlaveCount"])
	topo, err := ecat.LoadProject(exports...)
	require.NoError(t, err)
	netCount := -1
	for _, m := range topo.Masters {
		if strings.HasPrefix(m.Name, "Device 1") {
			netCount = len(m.Slaves)
		}
	}
	require.Positive(t, netCount, "Device 1 in the topology")
	t.Logf("ST301: %d cycles, sim %d ns, Device_1_SlaveCount %d (network %d slaves), %d diagnostics (no errors)",
		st.Cycles, st.SimTimeNS, want, netCount, len(st.Diagnostics))
}
