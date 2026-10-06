package bind

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rtValues = `VAR_GLOBAL
	b : BOOL := TRUE;
	i : INT := -3;
	u : UINT := 1001;
	lw : LWORD := 16#FFFFFFFFFFFFFFFF;
	r : REAL := 1.5;
	s : STRING := 'hi';
	t : TIME := T#1500MS;
	tod1 : TOD;
	d : DATE;
	dt1 : DT;
	en : E_State := E_State.Run;
	arr : ARRAY[2..4] OF INT := [7, 8, 9];
	ta : ARRAY[0..1] OF TIME;
	st : ST_P;
	tm : TON;
	p : POINTER TO INT;
	ref : REFERENCE TO INT;
END_VAR
`

const rtTypes = `TYPE E_State : (Idle := 0, Run := 1); END_TYPE
TYPE ST_P : STRUCT a : INT; END_STRUCT END_TYPE
`

func newSource(t *testing.T) (*interp.Runtime, *RuntimeSource) {
	t.Helper()
	var files []*ast.SourceFile
	for _, s := range [][2]string{{"types.st", rtTypes}, {"V.st", rtValues}, {"types2.st", bindTypes}, {"GVL.st", bindGVL}, {"main.st", bindMain}} {
		r := parser.Parse(s[0], s[1])
		require.Empty(t, r.Diags, s[0])
		files = append(files, r.File)
	}
	rt, err := interp.NewRuntime(files)
	require.NoError(t, err)
	return rt, NewRuntimeSource(rt)
}

func TestRuntimeSourceRead(t *testing.T) {
	_, src := newSource(t)
	cases := map[string]any{
		"V.b":    true,
		"V.i":    int64(-3),
		"V.u":    uint64(1001),
		"V.lw":   uint64(0xFFFFFFFFFFFFFFFF),
		"V.r":    1.5,
		"V.s":    "hi",
		"V.t":    1500 * time.Millisecond,
		"V.tod1": time.Duration(0),
		"V.d":    time.Unix(0, 0).UTC(),
		"V.dt1":  time.Unix(0, 0).UTC(),
		"V.en":   int64(1),
		"V.arr":  []any{int64(7), int64(8), int64(9)},
		"V.st":   map[string]any{"A": int64(0)},
	}
	for p, want := range cases {
		got, err := src.Read(p)
		require.NoError(t, err, p)
		if m, ok := want.(map[string]any); ok {
			gm, ok := got.(map[string]any)
			require.True(t, ok, p)
			assert.Len(t, gm, len(m), p)
			continue
		}
		assert.Equal(t, want, got, p)
	}

	_, err := src.Read("V.nope")
	assert.ErrorIs(t, err, opcua.ErrUnknownSymbol)
	_, err = src.Read("V.tm")
	assert.ErrorIs(t, err, opcua.ErrTypeMismatch, "FB instances are not values")
	_, err = src.Read("V.p")
	assert.ErrorIs(t, err, opcua.ErrTypeMismatch)
	_, err = src.Snapshot([]string{"V.ta", "V.tm"})
	assert.ErrorIs(t, err, opcua.ErrTypeMismatch)
}

func TestRuntimeSourceCanonicalNested(t *testing.T) {
	_, err := canonical(interp.Value{Kind: interp.ValArray, Array: []interp.Value{{Kind: interp.ValPointer}}})
	assert.ErrorIs(t, err, opcua.ErrTypeMismatch)
	_, err = canonical(interp.Value{Kind: interp.ValStruct, Struct: map[string]interp.Value{"x": {Kind: interp.ValPointer}}})
	assert.ErrorIs(t, err, opcua.ErrTypeMismatch)
	got, err := canonical(interp.Value{Kind: interp.ValArray, ArrayLow: 5})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRuntimeSourceWriteAtCycleBoundary(t *testing.T) {
	rt, src := newSource(t)
	writes := map[string]any{
		"V.b":    false,
		"V.i":    int64(12),
		"V.u":    uint64(7),
		"V.r":    float32(2.25),
		"V.s":    "it's $5",
		"V.t":    2*time.Second + 500*time.Microsecond,
		"V.tod1": 23*time.Hour + 59*time.Minute + 1250*time.Millisecond,
		"V.d":    time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC),
		"V.dt1":  time.Date(2030, 1, 2, 3, 4, 5, 250e6, time.UTC),
		"V.en":   int64(0),
		"V.arr":  []any{int64(1), int64(2), int64(3)},
		"V.ta":   []any{time.Second, 2 * time.Second},
		"V.st.a": int64(4),
	}
	want := map[string]any{
		"V.r":   2.25,
		"V.arr": []any{int64(1), int64(2), int64(3)},
		"V.ta":  []any{time.Second, 2 * time.Second},
	}
	before := map[string]any{}
	for p, v := range writes {
		old, err := src.Read(p)
		require.NoError(t, err, p)
		before[p] = old
		require.NoError(t, src.Write(p, v), p)
	}
	assert.Equal(t, len(writes), src.Pending())
	// Not visible before the cycle boundary.
	for p := range writes {
		got, err := src.Read(p)
		require.NoError(t, err)
		assert.Equal(t, before[p], got, p)
	}
	require.NoError(t, src.ApplyPending())
	require.NoError(t, rt.Tick(10*time.Millisecond))
	assert.Zero(t, src.Pending())
	for p, v := range writes {
		got, err := src.Read(p)
		require.NoError(t, err, p)
		if w, ok := want[p]; ok {
			v = w
		}
		assert.Equal(t, v, got, p)
	}
}

func TestRuntimeSourceWriteErrors(t *testing.T) {
	_, src := newSource(t)
	cases := []struct {
		path string
		v    any
		want error
	}{
		{"V.nope", 1, opcua.ErrUnknownSymbol},
		{"GVL.K", int64(1), opcua.ErrNotWritable},
		{"V.tm.Q", true, opcua.ErrNotWritable},
		{"V.p", int64(1), opcua.ErrNotWritable},
		{"V.i", int64(70000), opcua.ErrOutOfRange},
		{"V.arr", []any{1, 2, 3, 4}, opcua.ErrOutOfRange},
		{"V.i", "abc", opcua.ErrTypeMismatch},
		{"V.b", int64(3), opcua.ErrTypeMismatch},
		{"V.en", int64(9), opcua.ErrTypeMismatch},
		{"V.arr", 5, opcua.ErrTypeMismatch},
		{"V.d", "x", opcua.ErrTypeMismatch},
	}
	for _, c := range cases {
		err := src.Write(c.path, c.v)
		assert.ErrorIs(t, err, c.want, "%s=%v: %v", c.path, c.v, err)
	}
	assert.Zero(t, src.Pending())
}

func TestRuntimeSourceApplyErrorsAndQueueBound(t *testing.T) {
	_, src := newSource(t)
	// A write that turns invalid after queueing fails at apply time
	// without stopping the others.
	src.pending = append(src.pending,
		pendingBatch{paths: []string{"V.nope"}, vals: []any{1}},
		pendingBatch{paths: []string{"V.i"}, vals: []any{5}})
	err := src.ApplyPending()
	require.Error(t, err)
	got, rerr := src.Read("V.i")
	require.NoError(t, rerr)
	assert.Equal(t, int64(5), got)

	for i := 0; i < MaxPending; i++ {
		require.NoError(t, src.Write("V.i", int64(i%100)))
	}
	assert.ErrorIs(t, src.Write("V.i", int64(1)), errQueueFull)
}

func TestRuntimeSourceSnapshotConsistent(t *testing.T) {
	rt, src := newSource(t)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				if err := rt.Tick(time.Millisecond); err != nil {
					panic(err)
				}
			}
		}
	}()
	for k := 0; k < 300; k++ {
		m, err := src.Snapshot([]string{"MAIN.n", "MAIN.y"})
		require.NoError(t, err)
		require.Equal(t, m["MAIN.n"], m["MAIN.y"], "one scan image")
	}
	close(stop)
	wg.Wait()
}

func TestRuntimeSourceClassify(t *testing.T) {
	for msg, want := range map[string]error{
		"GVL.K is a constant":             opcua.ErrNotWritable,
		"x: pointer not writable by path": opcua.ErrNotWritable,
		"x is a read-only output of TON":  opcua.ErrNotWritable,
		"70000 out of range for INT":      opcua.ErrOutOfRange,
		"cannot use bool as INT":          opcua.ErrTypeMismatch,
	} {
		assert.True(t, errors.Is(classify(fmt.Errorf("%s", msg)), want), msg)
	}
}

// TestRuntimeSourceWriteBatch is the HI-03 regression: a struct write is
// validated whole, queued as one request and applied as one unit.
func TestRuntimeSourceWriteBatch(t *testing.T) {
	rt, src := newSource(t)
	require.NoError(t, src.WriteBatch(nil))
	assert.Zero(t, src.Pending())

	// One bad leaf rejects the whole batch: nothing is queued.
	err := src.WriteBatch([]opcua.PathWrite{{Path: "V.i", Value: int64(7)}, {Path: "V.b", Value: int64(3)}})
	assert.ErrorIs(t, err, opcua.ErrTypeMismatch)
	err = src.WriteBatch([]opcua.PathWrite{{Path: "V.i", Value: int64(7)}, {Path: "V.nope", Value: 1}})
	assert.ErrorIs(t, err, opcua.ErrUnknownSymbol)
	assert.Zero(t, src.Pending())

	// A good batch is one request, applied before the next Tick.
	require.NoError(t, src.WriteBatch([]opcua.PathWrite{{Path: "V.i", Value: int64(7)}, {Path: "V.u", Value: uint64(9)}, {Path: "V.st.a", Value: int64(4)}}))
	assert.Equal(t, 1, src.Pending())
	require.NoError(t, src.ApplyPending())
	require.NoError(t, rt.Tick(10*time.Millisecond))
	for p, want := range map[string]any{"V.i": int64(7), "V.u": uint64(9), "V.st.a": int64(4)} {
		got, err := src.Read(p)
		require.NoError(t, err)
		assert.Equal(t, want, got, p)
	}

	// A batch that turns invalid after queueing is skipped whole.
	src.pending = append(src.pending, pendingBatch{paths: []string{"V.i", "V.nope"}, vals: []any{int64(1), 1}})
	require.Error(t, src.ApplyPending())
	got, err := src.Read("V.i")
	require.NoError(t, err)
	assert.Equal(t, int64(7), got, "no leaf of a rejected batch is applied")

	// A struct with more leaves than MaxPending is still one request.
	big := make([]opcua.PathWrite, MaxPending+10)
	for i := range big {
		big[i] = opcua.PathWrite{Path: "V.i", Value: int64(i % 100)}
	}
	require.NoError(t, src.WriteBatch(big))
	assert.Equal(t, 1, src.Pending())
	require.NoError(t, src.ApplyPending())
}
