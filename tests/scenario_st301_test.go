package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScenarioST301 runs st301_jam.toml through `stc sim --scenario` on
// the real ST301 project with its four EtherCAT exports (ECAT-10 on real
// code). ST301 is local-only and read in place, so the test needs
// STC_SILD_DIR; only the scenario file with slave names is committed.
func TestScenarioST301(t *testing.T) {
	sild := os.Getenv("STC_SILD_DIR")
	if sild == "" {
		t.Skip("STC_SILD_DIR not set; ST301 sources and EtherCAT exports are local-only")
	}
	args := []string{"sim", filepath.Join(sild, "ST301", "ST301 solution.tsproj")}
	for i := 1; i <= 4; i++ {
		args = append(args, "--io", filepath.Join(sild, "IO List from ethercat", "ST301", "Device "+string(rune('0'+i))+".xml"))
	}
	args = append(args, "--scenario", "ecat_fixtures/scenario/st301_jam.toml", "--format", "json")

	run := func() []byte {
		var out, errOut bytes.Buffer
		cmd := exec.Command(stcBinary(t), args...)
		cmd.Stdout, cmd.Stderr = &out, &errOut
		require.NoError(t, cmd.Run(), "stc sim: %s", errOut.String())
		return out.Bytes()
	}
	out1 := run()
	out2 := run()
	assert.Equal(t, string(out1), string(out2), "two runs give byte-identical JSON")

	var res struct {
		Scenario struct {
			Passed     bool `json:"passed"`
			Cycles     int  `json:"cycles"`
			Assertions []struct {
				Path  string `json:"path"`
				Cycle int    `json:"cycle"`
				Pass  bool   `json:"pass"`
			} `json:"assertions"`
		} `json:"scenario"`
		Diagnostics []struct {
			Severity string `json:"severity"`
			Code     string `json:"code"`
		} `json:"diagnostics"`
	}
	require.NoError(t, json.Unmarshal(out1, &res))
	assert.True(t, res.Scenario.Passed)
	assert.Equal(t, 3000, res.Scenario.Cycles)
	require.Len(t, res.Scenario.Assertions, 4)
	for _, a := range res.Scenario.Assertions {
		assert.True(t, a.Pass, "%s at cycle %d", a.Path, a.Cycle)
	}
	for _, d := range res.Diagnostics {
		assert.NotEqual(t, "error", d.Severity, d.Code)
	}
	// The pulled EL1008 is seen at the first state poll after the pull.
	removed := res.Scenario.Assertions[3]
	assert.Equal(t, "ECT_Diag.Device_1_Diag[4].p_stat_bOk", removed.Path)
	assert.LessOrEqual(t, removed.Cycle, 1300+600, "seen within one 500 ms poll")
	t.Logf("removed slave seen at cycle %d", removed.Cycle)
}
