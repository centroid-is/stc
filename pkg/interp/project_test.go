package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const prjLib = `
FUNCTION_BLOCK FB_Lib
VAR_OUTPUT q : INT; END_VAR
q := q + 1;
END_FUNCTION_BLOCK
`

const prjTypes = `
TYPE E_Mode : (Off, On := 3); END_TYPE
TYPE ST_Item :
STRUCT
	v : INT := 5;
	m : E_Mode;
END_STRUCT
END_TYPE
FUNCTION F_Seed : INT
GCnt.calls := GCnt.calls + 1;
F_Seed := 42;
END_FUNCTION
FUNCTION F_Add : INT
VAR_INPUT a : INT; b : INT; END_VAR
F_Add := a + b;
END_FUNCTION
FUNCTION_BLOCK FB_Work
VAR_OUTPUT total : INT; resets : INT; END_VAR
VAR lib : FB_Lib; END_VAR
lib();
total := M_Add(lib.q);
METHOD M_Add : INT
VAR_INPUT x : INT; END_VAR
M_Add := total + x;
END_METHOD
ACTION A_Reset
total := 0;
resets := resets + 1;
END_ACTION
END_FUNCTION_BLOCK
`

const prjCnt = `
VAR_GLOBAL
	calls : INT;
	seq : INT;
	fastAt : INT;
	slowAt : INT;
END_VAR
`

const prjConst = `
VAR_GLOBAL CONSTANT
	SIZE : INT := 4;
END_VAR
`

const prjData = `
VAR_GLOBAL
	arr : ARRAY[1..GConst.SIZE] OF ST_Item;
	seeded : INT := F_Seed();
END_VAR
`

const prjProgs = `
PROGRAM MAIN
VAR
	w : FB_Work;
	sum : INT;
	t : TON;
	n : INT;
END_VAR
GCnt.seq := GCnt.seq + 1;
GCnt.fastAt := GCnt.seq;
w();
IF w.total > 100 THEN w.A_Reset(); END_IF
sum := F_Add(GData.arr[1].v, GData.seeded);
t(IN := TRUE, PT := T#1S);
n := n + 1;
END_PROGRAM
PROGRAM SLOW
VAR
	t : TON;
	n : INT;
END_VAR
GCnt.seq := GCnt.seq + 1;
GCnt.slowAt := GCnt.seq;
t(IN := TRUE, PT := T#1S);
n := n + 1;
END_PROGRAM
`

// prjSpec builds the standard project fixture with the given tasks.
func prjSpec(t *testing.T, tasks ...TaskSpec) ProjectSpec {
	t.Helper()
	return ProjectSpec{
		LibraryFiles: []*ast.SourceFile{parseRT(t, "Lib.st", prjLib)},
		Files: []*ast.SourceFile{
			parseRT(t, "types.st", prjTypes),
			parseRT(t, "GCnt.st", prjCnt),
			parseRT(t, "GConst.st", prjConst),
			parseRT(t, "GData.st", prjData),
			parseRT(t, "progs.st", prjProgs),
		},
		Tasks: tasks,
	}
}

func prjInt(t *testing.T, p *Project, path string) int64 {
	t.Helper()
	v, err := p.Runtime().Get(path)
	require.NoError(t, err, path)
	return v.Int
}

func TestLoadProject(t *testing.T) {
	t.Run("fixture loads and runs FBs, functions, methods and actions", func(t *testing.T) {
		p, err := LoadProject(prjSpec(t,
			TaskSpec{Name: "Fast", Cycle: time.Millisecond, Priority: 1, Programs: []string{"main"}},
			TaskSpec{Name: "Slow", Cycle: 10 * time.Millisecond, Priority: 20, Programs: []string{"SLOW"}},
		))
		require.NoError(t, err)
		assert.Empty(t, p.Runtime().Interpreter().InitErrors())
		assert.Equal(t, int64(5), prjInt(t, p, "GData.arr[4].v"), "bound from the other GVL's CONSTANT")
		_, err = p.Runtime().Get("GData.arr[5]")
		assert.Error(t, err)
		for i := 0; i < 120; i++ {
			require.NoError(t, p.Tick())
		}
		assert.Equal(t, int64(47), prjInt(t, p, "MAIN.sum"))
		assert.Positive(t, prjInt(t, p, "MAIN.w.resets"), "action ran when total passed 100")
		assert.LessOrEqual(t, prjInt(t, p, "MAIN.w.total"), int64(200), "method result reset by the action")
		ts := p.Tasks()
		require.Len(t, ts, 2)
		assert.Equal(t, "Fast", ts[0].Name)
		assert.Equal(t, []string{"MAIN"}, ts[0].Programs)
		assert.Equal(t, time.Millisecond, p.BaseTick())
	})

	t.Run("every GVL initialiser runs exactly once", func(t *testing.T) {
		p, err := LoadProject(prjSpec(t))
		require.NoError(t, err)
		assert.Equal(t, int64(1), prjInt(t, p, "GCnt.calls"))
		assert.Equal(t, int64(42), prjInt(t, p, "GData.seeded"))
		for i := 0; i < 10; i++ {
			require.NoError(t, p.Tick())
		}
		assert.Equal(t, int64(1), prjInt(t, p, "GCnt.calls"))
	})

	t.Run("program bound to two tasks", func(t *testing.T) {
		_, err := LoadProject(prjSpec(t,
			TaskSpec{Name: "A", Cycle: time.Millisecond, Programs: []string{"MAIN"}},
			TaskSpec{Name: "B", Cycle: time.Millisecond, Programs: []string{"Main"}},
		))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bound to tasks A and B")
	})

	t.Run("unknown programs and bad cycles are all listed", func(t *testing.T) {
		_, err := LoadProject(prjSpec(t,
			TaskSpec{Name: "A", Cycle: time.Millisecond, Programs: []string{"MAIN", "Nope"}},
			TaskSpec{Name: "B", Cycle: 0, Programs: []string{"SLOW"}},
		))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown PROGRAM Nope")
		assert.Contains(t, err.Error(), "task B: cycle time must be positive")
	})

	t.Run("default task runs MAIN every 10 ms", func(t *testing.T) {
		p, err := LoadProject(prjSpec(t))
		require.NoError(t, err)
		ts := p.Tasks()
		require.Len(t, ts, 1)
		assert.Equal(t, DefaultTaskName, ts[0].Name)
		assert.Equal(t, 10*time.Millisecond, ts[0].Cycle)
		assert.Equal(t, []string{"MAIN"}, ts[0].Programs)
	})

	t.Run("no tasks and no MAIN", func(t *testing.T) {
		f := parseRT(t, "p.st", "PROGRAM Other\nVAR x : INT; END_VAR\nx := 1;\nEND_PROGRAM\n")
		_, err := LoadProject(ProjectSpec{Files: []*ast.SourceFile{f}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no PROGRAM MAIN")
	})

	t.Run("init errors are returned", func(t *testing.T) {
		f := parseRT(t, "G.st", "VAR_GLOBAL a : ARRAY[1..Nope.X] OF INT; END_VAR\n")
		_, err := LoadProject(ProjectSpec{Files: []*ast.SourceFile{f}})
		assert.Error(t, err)
	})
}
