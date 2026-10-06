package interp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const persistTypes = `
TYPE E_Mode : (Idle, Auto, Manual); END_TYPE
TYPE ST_Cfg : STRUCT
	gain : REAL;
	on : BOOL;
END_STRUCT END_TYPE
FUNCTION_BLOCK FB_Base
VAR RETAIN
	nBaseHours : UDINT;
END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_Drive EXTENDS FB_Base
VAR_INPUT
	speed : REAL;
END_VAR
VAR PERSISTENT
	p_cfg_Freq : REAL := 50.0;
END_VAR
VAR
	inner : FB_Inner;
END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_Inner
VAR PERSISTENT
	p_cfg_Ramp : TIME := T#1s;
END_VAR
END_FUNCTION_BLOCK
`

const persistGVLs = `
VAR_GLOBAL PERSISTENT
	p_cfg_Speed : REAL;
	p_cfg_Mode : E_Mode;
	p_cfg_Struct : ST_Cfg;
	p_cfg_Arr : ARRAY[1..3] OF INT;
	p_cfg_Ramp : TIME;
	p_cfg_Stamp : DT;
END_VAR
VAR_GLOBAL
	plain : INT;
	fb : FB_Drive;
	drives : ARRAY[0..1] OF FB_Drive;
	cfgs : ARRAY[1..2] OF ST_Cfg;
	big : ARRAY[1..4] OF INT;
	pTo : POINTER TO INT;
END_VAR
VAR_GLOBAL CONSTANT
	K : INT := 1;
END_VAR
`

const persistMain = `
PROGRAM MAIN
VAR RETAIN
	nRetained : DINT;
END_VAR
VAR
	n : DINT;
	d : FB_Drive;
END_VAR
n := n + 1;
END_PROGRAM
`

func persistProject(t *testing.T) *Project {
	t.Helper()
	gvl := parseRT(t, "GVL_Cfg.st", persistGVLs)
	ast.SetGVLName(gvl, "GVL_Cfg")
	p, err := LoadProject(ProjectSpec{Files: []*ast.SourceFile{
		parseRT(t, "types.st", persistTypes), gvl, parseRT(t, "main.st", persistMain),
	}})
	require.NoError(t, err)
	return p
}

func TestPersistPaths(t *testing.T) {
	p := persistProject(t)
	assert.Equal(t, []string{
		"GVL_Cfg.drives[0].inner.p_cfg_Ramp",
		"GVL_Cfg.drives[0].nBaseHours",
		"GVL_Cfg.drives[0].p_cfg_Freq",
		"GVL_Cfg.drives[1].inner.p_cfg_Ramp",
		"GVL_Cfg.drives[1].nBaseHours",
		"GVL_Cfg.drives[1].p_cfg_Freq",
		"GVL_Cfg.fb.inner.p_cfg_Ramp",
		"GVL_Cfg.fb.nBaseHours",
		"GVL_Cfg.fb.p_cfg_Freq",
		"GVL_Cfg.p_cfg_Arr",
		"GVL_Cfg.p_cfg_Mode",
		"GVL_Cfg.p_cfg_Ramp",
		"GVL_Cfg.p_cfg_Speed",
		"GVL_Cfg.p_cfg_Stamp",
		"GVL_Cfg.p_cfg_Struct",
		"MAIN.d.inner.p_cfg_Ramp",
		"MAIN.d.nBaseHours",
		"MAIN.d.p_cfg_Freq",
		"MAIN.nRetained",
	}, p.PersistPaths())
}

func TestPersistRoundTrip(t *testing.T) {
	p := persistProject(t)
	rt := p.Runtime()
	sets := map[string]any{
		"GVL_Cfg.p_cfg_Speed":                42.5,
		"GVL_Cfg.p_cfg_Mode":                 "Manual",
		"GVL_Cfg.p_cfg_Struct":               map[string]any{"gain": 1.5, "on": true},
		"GVL_Cfg.p_cfg_Arr":                  []any{7, 8, 9},
		"GVL_Cfg.p_cfg_Ramp":                 json.Number("62003.004"),
		"GVL_Cfg.p_cfg_Stamp":                "DT#2026-10-06-12:34:56.789",
		"GVL_Cfg.fb.p_cfg_Freq":              60.0,
		"GVL_Cfg.fb.nBaseHours":              123,
		"GVL_Cfg.drives[1].inner.p_cfg_Ramp": "T#250ms",
		"MAIN.nRetained":                     -5,
		"GVL_Cfg.plain":                      99,
	}
	for k, v := range sets {
		require.NoError(t, rt.Set(k, v), k)
	}
	file := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, p.SaveState(file))
	first, err := os.ReadFile(file)
	require.NoError(t, err)
	require.NoError(t, p.SaveState(file))
	second, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second), "deterministic output")
	assert.NotContains(t, string(first), "plain")
	assert.True(t, strings.HasPrefix(string(first), "{\n  \"version\": 1,"), string(first))
	tmps, _ := filepath.Glob(file + ".*.tmp")
	assert.Empty(t, tmps, "temp file removed by rename")

	q := persistProject(t)
	warns, err := q.LoadState(file)
	require.NoError(t, err)
	assert.Empty(t, warns)
	for k := range sets {
		if k == "GVL_Cfg.plain" {
			continue
		}
		want, _ := rt.Get(k)
		got, err := q.Runtime().Get(k)
		require.NoError(t, err, k)
		assert.Equal(t, rt.ToJSON(want), q.Runtime().ToJSON(got), k)
	}
	ramp, _ := q.Runtime().Get("GVL_Cfg.p_cfg_Ramp")
	assert.Equal(t, time.Minute+2*time.Second+3*time.Millisecond+4*time.Microsecond, ramp.Time)
	plain, _ := q.Runtime().Get("GVL_Cfg.plain")
	assert.Equal(t, int64(0), plain.Int)
	speed, _ := q.Runtime().Get("GVL_Cfg.p_cfg_Speed")
	assert.Equal(t, 42.5, speed.Real)
}

func writeState(t *testing.T, body string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(f, []byte(body), 0o644))
	return f
}

func TestPersistLoadWarnings(t *testing.T) {
	p := persistProject(t)
	f := writeState(t, `{"version":1,"values":{
		"GVL_Cfg.removed": 1,
		"GVL_Cfg.plain": 3,
		"gvl_cfg.P_CFG_SPEED": 7.25,
		"GVL_Cfg.p_cfg_Mode": "NoSuchMode",
		"GVL_Cfg.p_cfg_Arr": [1, 2, 3]
	}}`)
	warns, err := p.LoadState(f)
	require.NoError(t, err)
	require.Len(t, warns, 3, "%v", warns)
	joined := strings.Join(warns, "\n")
	assert.Contains(t, joined, "GVL_Cfg.removed: unknown path")
	assert.Contains(t, joined, "GVL_Cfg.plain: unknown path")
	assert.Contains(t, joined, "GVL_Cfg.p_cfg_Mode")
	speed, _ := p.Runtime().Get("GVL_Cfg.p_cfg_Speed")
	assert.Equal(t, 7.25, speed.Real, "case-insensitive path still applied")
	arr, _ := p.Runtime().Get("GVL_Cfg.p_cfg_Arr")
	assert.Equal(t, []any{int64(1), int64(2), int64(3)}, p.Runtime().ToJSON(arr))
}

func TestPersistLoadFiles(t *testing.T) {
	p := persistProject(t)
	warns, err := p.LoadState(filepath.Join(t.TempDir(), "missing.json"))
	assert.NoError(t, err, "first run")
	assert.Empty(t, warns)

	_, err = p.LoadState(writeState(t, `{"version":1,"values":{`))
	assert.ErrorContains(t, err, "malformed state file")
	_, err = p.LoadState(writeState(t, `{"version":2,"values":{}}`))
	assert.ErrorContains(t, err, "unsupported state file version 2")
	_, err = p.LoadState(writeState(t, `{"version":1,"values":{"GVL_Cfg.p_cfg_Speed":1 }} x`))
	assert.ErrorContains(t, err, "malformed state file")
	_, err = p.LoadState(t.TempDir()) // a directory cannot be read
	assert.Error(t, err)
}

func TestPersistSaveErrors(t *testing.T) {
	p := persistProject(t)
	err := p.SaveState(filepath.Join(t.TempDir(), "no", "such", "dir", "s.json"))
	assert.Error(t, err)

	// A rename onto a non-empty directory fails and leaves no temp file.
	dir := t.TempDir()
	target := filepath.Join(dir, "state.json")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "x"), 0o755))
	assert.Error(t, p.SaveState(target))
	tmps, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	assert.Empty(t, tmps)
}

// TestPersistConcurrentSaves is the LO-02 regression: two processes saving
// the same state file used to share one temp name and clobber each
// other's temp file. Each save now has its own.
func TestPersistConcurrentSaves(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state.json")
	projects := []*Project{persistProject(t), persistProject(t)}
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(p *Project) {
			defer wg.Done()
			errs <- p.SaveState(file)
		}(projects[i%2])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		assert.NoError(t, err)
	}
	tmps, _ := filepath.Glob(file + ".*.tmp")
	assert.Empty(t, tmps)
	_, err := projects[0].LoadState(file)
	assert.NoError(t, err)
}

func TestPersistStateJSONKinds(t *testing.T) {
	p := persistProject(t)
	rt := p.Runtime()
	// Special reals, unsigned 64-bit and unknown kinds round-trip through
	// stateJSON; references to other variables are not followed.
	v := rt.stateJSON(Value{Kind: ValTime, Time: 1500 * time.Microsecond}, 0)
	assert.Equal(t, json.Number("1.500000"), v)
	v = rt.stateJSON(Value{Kind: ValTime, Time: -2 * time.Millisecond}, 0)
	assert.Equal(t, json.Number("-2.000000"), v)
	assert.Nil(t, rt.stateJSON(Value{Kind: ValBool}, maxJSONDepth+1))
	assert.Nil(t, rt.stateJSON(Value{Kind: ValFBInstance}, 0))
	std := rt.stateJSON(Value{Kind: ValFBInstance, FBRef: &FBInstance{TypeName: "TON", FB: &TON{}}}, 0)
	assert.NotNil(t, std)
	s := rt.stateJSON(Value{Kind: ValStruct, Struct: map[string]Value{"B": {Kind: ValInt, Int: 2}, "A": {Kind: ValTime, Time: time.Millisecond}}}, 0)
	b, err := json.Marshal(s)
	require.NoError(t, err)
	assert.Equal(t, `{"A":1.000000,"B":2}`, string(b))
}

func TestPersistWalkerEdges(t *testing.T) {
	p := persistProject(t)
	p.files = append(p.files, nil, &ast.SourceFile{Declarations: []ast.Declaration{&ast.GVLDecl{}, &ast.ProgramDecl{}}})
	assert.Equal(t, p.PersistPaths(), p.collectPersist(), "nil files and unnamed POUs are skipped")

	w := persistWalker{rt: p.rt, seen: map[string]bool{}, visited: map[*FBInstance]bool{}}
	w.value("x", Value{Kind: ValFBInstance}, maxJSONDepth+1)
	w.value("x", Value{Kind: ValArray, ArrayLow: 2, Array: []Value{{}, {}}}, 0)
	w.value("x", Value{Kind: ValArray, Array: []Value{{Kind: ValInt}}}, 0)
	assert.Empty(t, w.out)

	assert.Equal(t, []any{}, p.rt.stateJSON(Value{Kind: ValArray, ArrayLow: 5, Array: []Value{{}}}, 0))
}
