package interp

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
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

func gvlScalars(t *testing.T, p *Project) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, path := range []string{"GCnt.calls", "GCnt.seq", "GCnt.fastAt", "GCnt.slowAt", "GData.seeded", "MAIN.sum", "MAIN.n", "SLOW.n", "MAIN.w.total"} {
		out[path] = prjInt(t, p, path)
	}
	return out
}

func TestProjectTick(t *testing.T) {
	t.Run("1000 ticks of a 1 ms task advance exactly 1 s, identically", func(t *testing.T) {
		run := func() (*Project, map[string]int64) {
			p, err := LoadProject(prjSpec(t, TaskSpec{Name: "T", Cycle: time.Millisecond, Programs: []string{"MAIN"}}))
			require.NoError(t, err)
			for i := 0; i < 1000; i++ {
				require.NoError(t, p.Tick())
			}
			return p, gvlScalars(t, p)
		}
		p1, s1 := run()
		_, s2 := run()
		assert.Equal(t, time.Second, p1.Clock())
		assert.Equal(t, uint64(1000), p1.Tasks()[0].Runs)
		assert.Equal(t, s1, s2)
		assert.Equal(t, time.Second, p1.Runtime().Interpreter().Clock())
	})

	t.Run("two tasks run at their cycles in priority order", func(t *testing.T) {
		p, err := LoadProject(prjSpec(t,
			TaskSpec{Name: "Slow", Cycle: 10 * time.Millisecond, Priority: 20, Programs: []string{"SLOW"}},
			TaskSpec{Name: "Fast", Cycle: time.Millisecond, Priority: 1, Programs: []string{"MAIN"}},
		))
		require.NoError(t, err)
		assert.Equal(t, time.Millisecond, p.BaseTick())
		require.NoError(t, p.Tick())
		assert.Equal(t, int64(1), prjInt(t, p, "GCnt.fastAt"), "prio 1 runs first")
		assert.Equal(t, int64(2), prjInt(t, p, "GCnt.slowAt"))
		for i := 1; i < 100; i++ {
			require.NoError(t, p.Tick())
		}
		ts := p.Tasks()
		assert.Equal(t, "Fast", ts[0].Name)
		assert.Equal(t, uint64(100), ts[0].Runs)
		assert.Equal(t, uint64(10), ts[1].Runs)
		assert.Equal(t, int64(10), prjInt(t, p, "SLOW.n"))
	})

	t.Run("TON in a 10 ms task advances 10 ms per run", func(t *testing.T) {
		p, err := LoadProject(prjSpec(t,
			TaskSpec{Name: "Fast", Cycle: time.Millisecond, Priority: 1, Programs: []string{"MAIN"}},
			TaskSpec{Name: "Slow", Cycle: 10 * time.Millisecond, Priority: 20, Programs: []string{"SLOW"}},
		))
		require.NoError(t, err)
		et := func(path string) time.Duration {
			v, err := p.Runtime().Get(path)
			require.NoError(t, err)
			return v.Time
		}
		require.NoError(t, p.Tick())
		assert.Equal(t, 10*time.Millisecond, et("SLOW.t.ET"))
		assert.Equal(t, time.Millisecond, et("MAIN.t.ET"))
		require.NoError(t, p.Advance(10*time.Millisecond))
		assert.Equal(t, 20*time.Millisecond, et("SLOW.t.ET"))
		assert.Equal(t, 11*time.Millisecond, et("MAIN.t.ET"))
	})

	t.Run("engine errors name task and program and do not stop other tasks", func(t *testing.T) {
		f := parseRT(t, "p.st", `
PROGRAM Bad
VAR a : ARRAY[1..2] OF INT; i : INT := 5; END_VAR
a[i] := 1;
END_PROGRAM
PROGRAM Good
VAR n : INT; END_VAR
n := n + 1;
END_PROGRAM
`)
		p, err := LoadProject(ProjectSpec{Files: []*ast.SourceFile{f}, Tasks: []TaskSpec{
			{Name: "A", Cycle: time.Millisecond, Priority: 1, Programs: []string{"Bad"}},
			{Name: "B", Cycle: time.Millisecond, Priority: 2, Programs: []string{"Good"}},
		}})
		require.NoError(t, err)
		err = p.Tick()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task A PROGRAM Bad")
		assert.Equal(t, int64(1), prjInt(t, p, "Good.n"))
	})
}

func TestProjectAdvance(t *testing.T) {
	p, err := LoadProject(prjSpec(t, TaskSpec{Name: "T", Cycle: time.Millisecond, Programs: []string{"MAIN"}}))
	require.NoError(t, err)
	require.NoError(t, p.Advance(250*time.Millisecond))
	assert.Equal(t, 250*time.Millisecond, p.Clock())
	assert.Equal(t, uint64(250), p.Tasks()[0].Runs)

	q, err := LoadProject(prjSpec(t, TaskSpec{Name: "T", Cycle: time.Millisecond, Programs: []string{"MAIN"}}))
	require.NoError(t, err)
	for i := 0; i < 250; i++ {
		require.NoError(t, q.Tick())
	}
	assert.Equal(t, gvlScalars(t, q), gvlScalars(t, p))

	assert.Error(t, p.Advance(1500*time.Microsecond))
	assert.Error(t, p.Advance(-time.Millisecond))

	bad := parseRT(t, "b.st", "PROGRAM MAIN\nVAR a : ARRAY[1..2] OF INT; i : INT := 5; END_VAR\na[i] := 1;\nEND_PROGRAM\n")
	pb, err := LoadProject(ProjectSpec{Files: []*ast.SourceFile{bad}})
	require.NoError(t, err)
	assert.Error(t, pb.Advance(20*time.Millisecond))
	assert.Equal(t, 10*time.Millisecond, pb.Clock(), "first error stops the run")
}

const prjIOSrc = `
VAR_GLOBAL
	inp : INT;
	outp : INT;
END_VAR
`

func TestProjectIOBinder(t *testing.T) {
	gvl := parseRT(t, "IO.st", prjIOSrc)
	progs := parseRT(t, "p.st", `
PROGRAM PA
IO.inp := 0;
END_PROGRAM
PROGRAM PB
VAR last : INT; END_VAR
IO.outp := IO.inp + 1;
last := IO.outp + 1;
END_PROGRAM
`)
	p, err := LoadProject(ProjectSpec{Files: []*ast.SourceFile{gvl, progs}, Tasks: []TaskSpec{
		{Name: "A", Cycle: time.Millisecond, Priority: 1, Programs: []string{"PA"}},
		{Name: "B", Cycle: time.Millisecond, Priority: 2, Programs: []string{"PB"}},
	}})
	require.NoError(t, err)
	net := ioNet()
	b := NewIOBinder([]ecat.Binding{
		bind(ecat.DirIn, 0, 16, "INT", "IO", "INP"),
		bind(ecat.DirOut, 0, 16, "INT", "IO", "OUTP"),
		bind(ecat.DirOut, 2, 16, "INT", "pb", "LAST"),
		bind(ecat.DirOut, 4, 16, "INT", "Nope", "X"),
	}, net)
	p.SetIOBinder(b)
	img := net.Images().Get(ioMaster)
	ecat.WriteBits(img.In, 0, 0, 16, 41)
	require.NoError(t, p.Tick())
	require.Len(t, b.Errors(), 1, "unknown root reported")
	assert.Equal(t, uint64(1), ecat.ReadBits(img.Out, 0, 0, 16), "inputs copied once before the first task, not again before B")
	assert.Equal(t, uint64(2), ecat.ReadBits(img.Out, 2, 0, 16), "program variables resolve across the project")

	p.SetIOBinder(nil)
	require.NoError(t, p.Tick())
}
