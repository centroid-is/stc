package interp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStdFBAliases pins that the counters and bistables accept both the IEC
// input names and the Tc2_Standard names TwinCAT code uses.
func TestStdFBAliases(t *testing.T) {
	on := BoolValue(true)

	t.Run("Go level aliases", func(t *testing.T) {
		cases := []struct {
			fb       string
			iec, tc2 string
		}{
			{"CTU", "R", "RESET"},
			{"CTD", "LD", "LOAD"},
			{"CTUD", "R", "RESET"},
			{"CTUD", "LD", "LOAD"},
			{"SR", "S1", "SET1"},
			{"SR", "R", "RESET"},
			{"RS", "S", "SET"},
			{"RS", "R1", "RESET1"},
		}
		for _, c := range cases {
			fb := StdlibFBFactory[c.fb]()
			fb.SetInput(c.tc2, on)
			assert.True(t, fb.GetInput(c.iec).Bool, "%s.%s set via %s", c.fb, c.iec, c.tc2)
			assert.True(t, fb.GetInput(c.tc2).Bool, "%s.%s readable", c.fb, c.tc2)
			assert.Equal(t, fb.GetInput(c.iec), fb.GetInput(c.tc2))
		}
	})

	t.Run("CTU RESET resets", func(t *testing.T) {
		c := &CTU{}
		c.SetInput("PV", IntValue(3))
		c.SetInput("CU", on)
		c.Execute(0)
		require.Equal(t, int64(1), c.GetOutput("CV").Int)
		c.SetInput("RESET", on)
		c.Execute(0)
		assert.Equal(t, int64(0), c.GetOutput("CV").Int)
	})

	t.Run("CTD LOAD loads", func(t *testing.T) {
		c := &CTD{}
		c.SetInput("PV", IntValue(5))
		c.SetInput("LOAD", on)
		c.Execute(0)
		assert.Equal(t, int64(5), c.GetOutput("CV").Int)
	})

	t.Run("CTUD RESET and LOAD", func(t *testing.T) {
		c := &CTUD{}
		c.SetInput("PV", IntValue(4))
		c.SetInput("LOAD", on)
		c.Execute(0)
		require.Equal(t, int64(4), c.GetOutput("CV").Int)
		c.SetInput("LOAD", BoolValue(false))
		c.SetInput("RESET", on)
		c.Execute(0)
		assert.Equal(t, int64(0), c.GetOutput("CV").Int)
	})

	run := func(t *testing.T, src string, scans []map[string]Value, check func(i int, eng *ScanCycleEngine)) {
		t.Helper()
		eng := gvlEngine(t, "P.st", src)
		for i, in := range scans {
			if i == 0 {
				require.NoError(t, eng.Tick(10*time.Millisecond))
			}
			for k, v := range in {
				eng.env.Set(k, v)
			}
			require.NoError(t, eng.Tick(10*time.Millisecond))
			check(i, eng)
		}
	}
	off := BoolValue(false)

	t.Run("ST CTU with RESET", func(t *testing.T) {
		src := `
PROGRAM P
VAR c : CTU; x, r, q : BOOL; cv : INT; END_VAR
c(CU := x, RESET := r, PV := 3);
q := c.Q; cv := c.CV;
END_PROGRAM
`
		// CU rises on scans 0, 2 and 4; RESET on scan 5 clears the count.
		scans := []map[string]Value{{"x": on}, {"x": off}, {"x": on}, {"x": off}, {"x": on}, {"r": on}}
		want := []int64{1, 1, 2, 2, 3, 0}
		run(t, src, scans, func(i int, eng *ScanCycleEngine) {
			assert.Equal(t, want[i], progVar(t, eng, "cv").Int, "scan %d", i)
			if i == 4 {
				assert.True(t, progVar(t, eng, "q").Bool)
			}
		})
	})

	t.Run("ST CTD with LOAD", func(t *testing.T) {
		src := `
PROGRAM P
VAR c : CTD; x, l : BOOL; cv : INT; END_VAR
c(CD := x, LOAD := l, PV := 2);
cv := c.CV;
END_PROGRAM
`
		scans := []map[string]Value{{"l": on}, {"l": off, "x": on}}
		want := []int64{2, 1}
		run(t, src, scans, func(i int, eng *ScanCycleEngine) {
			assert.Equal(t, want[i], progVar(t, eng, "cv").Int, "scan %d", i)
		})
	})

	t.Run("ST CTUD with RESET and LOAD", func(t *testing.T) {
		src := `
PROGRAM P
VAR c : CTUD; u, d, r, l : BOOL; cv : INT; END_VAR
c(CU := u, CD := d, RESET := r, LOAD := l, PV := 5);
cv := c.CV;
END_PROGRAM
`
		scans := []map[string]Value{{"l": on}, {"l": off, "u": on}, {"r": on}}
		want := []int64{5, 6, 0}
		run(t, src, scans, func(i int, eng *ScanCycleEngine) {
			assert.Equal(t, want[i], progVar(t, eng, "cv").Int, "scan %d", i)
		})
	})

	t.Run("ST SR with SET1 and RESET", func(t *testing.T) {
		src := `
PROGRAM P
VAR sr : SR; a, b, q : BOOL; END_VAR
sr(SET1 := a, RESET := b);
q := sr.Q1;
END_PROGRAM
`
		scans := []map[string]Value{{"a": on}, {"a": off}, {"b": on}, {"a": on}}
		want := []bool{true, true, false, true}
		run(t, src, scans, func(i int, eng *ScanCycleEngine) {
			assert.Equal(t, want[i], progVar(t, eng, "q").Bool, "scan %d", i)
		})
	})

	t.Run("ST RS with SET and RESET1", func(t *testing.T) {
		src := `
PROGRAM P
VAR rs : RS; a, b, q : BOOL; END_VAR
rs(SET := a, RESET1 := b);
q := rs.Q1;
END_PROGRAM
`
		scans := []map[string]Value{{"a": on}, {"a": off}, {"b": on, "a": on}}
		want := []bool{true, true, false}
		run(t, src, scans, func(i int, eng *ScanCycleEngine) {
			assert.Equal(t, want[i], progVar(t, eng, "q").Bool, "scan %d", i)
		})
	})
}
