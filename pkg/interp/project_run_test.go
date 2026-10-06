package interp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const runSrc = `
VAR_GLOBAL
	n : DINT;
	slowN : DINT;
	zero : DINT;
	boom : BOOL;
END_VAR
PROGRAM MAIN
n := n + 1;
IF boom THEN
	n := n / zero;
END_IF
END_PROGRAM
PROGRAM SLOW
slowN := slowN + 1;
END_PROGRAM
`

// fakeClock is a WallClock whose time moves only through Sleep and advance.
type fakeClock struct {
	t      *testing.T
	now    time.Duration
	sleeps int
}

func (f *fakeClock) Now() time.Duration { return f.now }

func (f *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		f.t.Fatalf("Sleep(%v): Run must not sleep for a non-positive duration", d)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f.sleeps++
	f.now += d
	return nil
}

func runProject(t *testing.T) *Project {
	t.Helper()
	p, err := LoadProject(ProjectSpec{
		Files: []*ast.SourceFile{parseRT(t, "G.st", runSrc)},
		Tasks: []TaskSpec{
			{Name: "Fast", Cycle: time.Millisecond, Programs: []string{"MAIN"}},
			{Name: "Slow", Cycle: 4 * time.Millisecond, Priority: 1, Programs: []string{"SLOW"}},
		},
	})
	require.NoError(t, err)
	return p
}

func TestProjectRunFakeClock(t *testing.T) {
	p := runProject(t)
	clk := &fakeClock{t: t}
	ticks := 0
	var last time.Duration
	err := p.Run(context.Background(), RunOpts{Duration: 10 * time.Second, Clock: clk, OnTick: func(sim time.Duration) {
		ticks++
		last = sim
	}})
	require.NoError(t, err)
	assert.Equal(t, 10000, ticks)
	assert.Equal(t, 10*time.Second, last)
	assert.Equal(t, 9999, clk.sleeps, "one sleep between consecutive ticks")
	st := p.Tasks()
	assert.Equal(t, uint64(10000), st[0].Runs)
	assert.Equal(t, uint64(2500), st[1].Runs)
	assert.Zero(t, st[0].Overruns)
	assert.Zero(t, st[1].Overruns)
}

func TestProjectRunOverrun(t *testing.T) {
	p := runProject(t)
	clk := &fakeClock{t: t}
	ticks := 0
	// the first tick takes 5 ms of wall time: base ticks 1..4 are missed
	err := p.Run(context.Background(), RunOpts{Duration: 20 * time.Millisecond, Clock: clk, OnTick: func(time.Duration) {
		ticks++
		if ticks == 1 {
			clk.now += 5 * time.Millisecond
		}
	}})
	require.NoError(t, err)
	assert.Equal(t, 16, ticks, "ticks 1..4 are skipped, not burst")
	assert.Equal(t, 16*time.Millisecond, p.Clock(), "virtual time falls behind wall time")
	st := p.Tasks()
	assert.Equal(t, uint64(1), st[0].Overruns)
	assert.Equal(t, uint64(1), st[1].Overruns)

	// a late tick that misses no Slow deadline overruns only Fast
	p = runProject(t)
	clk = &fakeClock{t: t}
	ticks = 0
	require.NoError(t, p.Run(context.Background(), RunOpts{Duration: 10 * time.Millisecond, Clock: clk, OnTick: func(time.Duration) {
		ticks++
		if ticks == 5 {
			clk.now += 1500 * time.Microsecond
		}
	}}))
	st = p.Tasks()
	assert.Equal(t, uint64(1), st[0].Overruns)
	assert.Zero(t, st[1].Overruns)
}

func TestProjectRunCancelAndErrors(t *testing.T) {
	p := runProject(t)
	ctx, cancel := context.WithCancel(context.Background())
	ticks := 0
	err := p.Run(ctx, RunOpts{Clock: &fakeClock{t: t}, OnTick: func(time.Duration) {
		ticks++
		if ticks == 3 {
			cancel()
		}
	}})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 3, ticks)

	// a Tick error stops the run
	p = runProject(t)
	require.NoError(t, p.Runtime().Set("G.boom", true))
	err = p.Run(context.Background(), RunOpts{Clock: &fakeClock{t: t}, Duration: time.Second})
	require.Error(t, err)
	assert.False(t, errors.Is(err, context.Canceled))

	// the real clock's Sleep returns promptly on cancel
	p = runProject(t)
	ctx, cancel = context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx, RunOpts{}) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	clk := NewWallClock()
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, clk.Sleep(ctx, time.Hour), context.Canceled)
}

func TestProjectRunRealClock(t *testing.T) {
	if testing.Short() {
		t.Skip("real-time pacing")
	}
	p := runProject(t)
	ticks := 0
	require.NoError(t, p.Run(context.Background(), RunOpts{Duration: 200 * time.Millisecond, OnTick: func(time.Duration) { ticks++ }}))
	assert.GreaterOrEqual(t, ticks, 150)
	assert.LessOrEqual(t, ticks, 210)
}
