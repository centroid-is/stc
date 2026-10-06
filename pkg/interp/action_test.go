package interp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// probeEngine loads a fixture from tests/twincat_probes and returns an engine
// for its first PROGRAM, with the file's FBs and TYPEs registered.
func probeEngine(t *testing.T, name string) *ScanCycleEngine {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "tests", "twincat_probes", name))
	require.NoError(t, err)
	return gvlEngine(t, name, string(src))
}

func boolVar(t *testing.T, env *Env, name string) bool {
	t.Helper()
	v, ok := env.Get(name)
	require.True(t, ok, "no variable %s", name)
	return v.Bool
}

func intVar(t *testing.T, env *Env, name string) int64 {
	t.Helper()
	v, ok := env.Get(name)
	require.True(t, ok, "no variable %s", name)
	return v.Int
}

// fbOutput reads member of the FB instance stored in variable inst.
func fbOutput(t *testing.T, env *Env, inst, member string) Value {
	t.Helper()
	v, ok := env.Get(inst)
	require.True(t, ok, "no variable %s", inst)
	require.Equal(t, ValFBInstance, v.Kind)
	return v.FBRef.GetMember(member)
}

func TestAction(t *testing.T) {
	t.Run("action.st toggles x on every scan", func(t *testing.T) {
		eng := probeEngine(t, "action.st")
		var got []bool
		for i := 0; i < 3; i++ {
			require.NoError(t, eng.Tick(10*time.Millisecond))
			got = append(got, boolVar(t, eng.env, "x"))
		}
		assert.Equal(t, []bool{true, false, true}, got)
	})

	t.Run("action_inside.st keeps R_TRIG and TON state across scans", func(t *testing.T) {
		eng := probeEngine(t, "action_inside.st")
		eng.Initialize()
		require.True(t, eng.env.Set("x", BoolValue(true)))

		require.NoError(t, eng.Tick(400*time.Millisecond))
		assert.True(t, fbOutput(t, eng.env, "trig", "Q").Bool, "rising edge on first scan")
		assert.False(t, fbOutput(t, eng.env, "t", "Q").Bool)
		assert.Equal(t, int64(1), intVar(t, eng.env, "n"))

		require.NoError(t, eng.Tick(400*time.Millisecond))
		assert.False(t, fbOutput(t, eng.env, "trig", "Q").Bool, "no edge on second scan")
		assert.False(t, fbOutput(t, eng.env, "t", "Q").Bool, "800ms is short of PT")

		require.NoError(t, eng.Tick(400*time.Millisecond))
		assert.True(t, fbOutput(t, eng.env, "t", "Q").Bool, "TON done after 1.2s")
		assert.Equal(t, int64(3), intVar(t, eng.env, "n"))
	})

	t.Run("program action drives a TON and an R_TRIG across ticks", func(t *testing.T) {
		eng := gvlEngine(t, "timers.st", `
PROGRAM MAIN
VAR
	x : BOOL;
	done : BOOL;
	edge : BOOL;
	edges : DINT;
	elapsed : TIME;
	t : TON;
	trig : R_TRIG;
END_VAR
A_Timers();
ACTION A_Timers
t(IN := x, PT := T#100MS);
trig(CLK := x);
done := t.Q;
elapsed := t.ET;
edge := trig.Q;
IF edge THEN
	edges := edges + 1;
END_IF
END_ACTION
END_PROGRAM
`)
		eng.Initialize()
		require.NoError(t, eng.Tick(40*time.Millisecond))
		assert.False(t, boolVar(t, eng.env, "edge"), "no edge while x is FALSE")
		assert.False(t, boolVar(t, eng.env, "done"))

		require.True(t, eng.env.Set("x", BoolValue(true)))
		require.NoError(t, eng.Tick(40*time.Millisecond))
		assert.True(t, boolVar(t, eng.env, "edge"), "rising edge on the scan x goes TRUE")
		assert.False(t, boolVar(t, eng.env, "done"))

		require.NoError(t, eng.Tick(40*time.Millisecond))
		assert.False(t, boolVar(t, eng.env, "edge"), "edge lasts one scan")
		assert.False(t, boolVar(t, eng.env, "done"), "TON still short of PT")

		require.NoError(t, eng.Tick(40*time.Millisecond))
		require.NoError(t, eng.Tick(40*time.Millisecond))
		assert.True(t, boolVar(t, eng.env, "done"), "TON done once PT has elapsed")
		assert.Equal(t, int64(1), intVar(t, eng.env, "edges"))

		require.True(t, eng.env.Set("x", BoolValue(false)))
		require.NoError(t, eng.Tick(40*time.Millisecond))
		assert.False(t, boolVar(t, eng.env, "done"), "TON resets when IN drops")
		ev, ok := eng.env.Get("elapsed")
		require.True(t, ok)
		assert.Equal(t, time.Duration(0), ev.Time)
	})

	t.Run("RETURN in an action exits only the action", func(t *testing.T) {
		eng := gvlEngine(t, "ret.st", `
PROGRAM MAIN
VAR a : INT; b : INT; END_VAR
A1();
b := b + 1;
END_PROGRAM
ACTION A1
a := a + 1;
RETURN;
a := 100;
END_ACTION
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		require.NoError(t, eng.Tick(time.Millisecond))
		assert.Equal(t, int64(2), intVar(t, eng.env, "a"))
		assert.Equal(t, int64(2), intVar(t, eng.env, "b"))
	})

	fbSrc := `
FUNCTION_BLOCK FB_C
VAR_OUTPUT n : INT; END_VAR
coe();
ACTION coe
n := n + 1;
END_ACTION
END_FUNCTION_BLOCK
`
	t.Run("FB body calling its own action runs in the instance env", func(t *testing.T) {
		eng := gvlEngine(t, "fbact.st", fbSrc+`
PROGRAM MAIN
VAR inst : FB_C; END_VAR
inst();
inst();
END_PROGRAM
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		assert.Equal(t, int64(2), fbOutput(t, eng.env, "inst", "n").Int)
	})

	t.Run("external inst.coe() runs the action on the instance", func(t *testing.T) {
		eng := gvlEngine(t, "fbext.st", fbSrc+`
PROGRAM MAIN
VAR inst : FB_C; END_VAR
inst.coe();
END_PROGRAM
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		require.NoError(t, eng.Tick(time.Millisecond))
		assert.Equal(t, int64(2), fbOutput(t, eng.env, "inst", "n").Int)
	})

	t.Run("derived FB calls an action of its base FB", func(t *testing.T) {
		eng := gvlEngine(t, "ext.st", `
FUNCTION_BLOCK FB_Base
VAR_OUTPUT n : INT; END_VAR
ACTION bump
n := n + 10;
END_ACTION
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_Derived EXTENDS FB_Base
VAR_OUTPUT m : INT; END_VAR
bump();
m := m + 1;
END_FUNCTION_BLOCK
PROGRAM MAIN
VAR d : FB_Derived; END_VAR
d();
d.bump();
END_PROGRAM
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		assert.Equal(t, int64(20), fbOutput(t, eng.env, "d", "n").Int)
		assert.Equal(t, int64(1), fbOutput(t, eng.env, "d", "m").Int)
	})

	t.Run("self-recursive action hits the call depth limit", func(t *testing.T) {
		eng := gvlEngine(t, "rec.st", `
PROGRAM MAIN
VAR n : DINT; END_VAR
A1();
END_PROGRAM
ACTION A1
n := n + 1;
A1();
END_ACTION
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		var rt *RuntimeError
		require.ErrorAs(t, err, &rt)
		assert.Contains(t, rt.Msg, "maximum call depth 256 exceeded")
		assert.Equal(t, int64(MaxCallDepth), intVar(t, eng.env, "n"))
		assert.Equal(t, 0, eng.interp.callDepth, "depth unwinds after the error")
	})

	t.Run("unknown action name is still an undefined function", func(t *testing.T) {
		eng := gvlEngine(t, "undef.st", `
PROGRAM MAIN
VAR x : BOOL; END_VAR
Nope();
END_PROGRAM
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "undefined function: NOPE")
	})

	t.Run("action called with arguments is a runtime error", func(t *testing.T) {
		eng := gvlEngine(t, "args.st", `
PROGRAM MAIN
VAR x : BOOL; END_VAR
A1(1);
END_PROGRAM
ACTION A1
x := TRUE;
END_ACTION
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "action A1 takes no arguments")
	})

	t.Run("external action call with arguments is a runtime error", func(t *testing.T) {
		eng := gvlEngine(t, "extargs.st", fbSrc+`
PROGRAM MAIN
VAR inst : FB_C; END_VAR
inst.coe(1);
END_PROGRAM
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "action coe takes no arguments")
	})

	t.Run("self-recursive method hits the call depth limit", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", `
FUNCTION_BLOCK FB_R
VAR n : DINT; END_VAR
METHOD M : BOOL
n := n + 1;
M := G.r.M();
END_METHOD
END_FUNCTION_BLOCK
VAR_GLOBAL
    r : FB_R;
END_VAR
PROGRAM MAIN
VAR ok : BOOL; END_VAR
ok := G.r.M();
END_PROGRAM
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "maximum call depth 256 exceeded calling M")
		assert.Equal(t, int64(MaxCallDepth), gvlVar(t, eng, "G", "r").FBRef.GetMember("n").Int)
		assert.Equal(t, 0, eng.interp.callDepth)
	})

	// Every way of calling an FB instance runs user code, so each one must
	// be bounded: an FB calling its own GVL instance would otherwise abort
	// the Go runtime with a fatal stack overflow.
	for _, call := range []string{"s();", "G.s();", "s(x := 1);", "G.s(x := 1);"} {
		t.Run("self-calling FB instance hits the call depth limit: "+call, func(t *testing.T) {
			eng := gvlEngine(t, "G.st", `
FUNCTION_BLOCK FB_Self
VAR_INPUT x : DINT; END_VAR
VAR n : DINT; END_VAR
n := n + 1;
`+call+`
END_FUNCTION_BLOCK
VAR_GLOBAL
    s : FB_Self;
END_VAR
PROGRAM MAIN
VAR ok : BOOL; END_VAR
`+call+`
END_PROGRAM
`)
			err := eng.Tick(time.Millisecond)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "maximum call depth 256 exceeded calling FB_Self")
			assert.Equal(t, int64(MaxCallDepth), gvlVar(t, eng, "G", "s").FBRef.GetMember("n").Int)
			assert.Equal(t, 0, eng.interp.callDepth, "depth unwinds after the error")
		})
	}
}

// TestZeroArgFBCall covers FB instance calls with no arguments, which parse
// as expression statements and reach evalCall/evalMethodCall rather than
// execCallStmt.
func TestZeroArgFBCall(t *testing.T) {
	fb := `
FUNCTION_BLOCK FB_C
VAR_OUTPUT n : INT; END_VAR
n := n + 1;
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_Outer
VAR inner : FB_C; END_VAR
END_FUNCTION_BLOCK
TYPE ST_H : STRUCT a : INT; END_STRUCT END_TYPE
`
	t.Run("plain instance b();", func(t *testing.T) {
		eng := gvlEngine(t, "zb.st", fb+`
PROGRAM MAIN
VAR b : FB_C; t : TON; END_VAR
b();
t();
END_PROGRAM
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		require.NoError(t, eng.Tick(time.Millisecond))
		assert.Equal(t, int64(2), fbOutput(t, eng.env, "b", "n").Int)
	})

	t.Run("GVL instance G.f();", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", fb+`
VAR_GLOBAL
    f : FB_C;
    k : INT;
END_VAR
PROGRAM MAIN
G.f();
END_PROGRAM
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		v := gvlVar(t, eng, "G", "f")
		assert.Equal(t, int64(1), v.FBRef.GetMember("n").Int)
	})

	t.Run("GVL non-FB member call is an error", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", fb+`
VAR_GLOBAL
    k : INT;
END_VAR
PROGRAM MAIN
G.k();
END_PROGRAM
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "G.k is not callable")
	})

	t.Run("GVL unknown member call is an error", func(t *testing.T) {
		eng := gvlEngine(t, "G.st", fb+`
VAR_GLOBAL
    k : INT;
END_VAR
PROGRAM MAIN
G.nope();
END_PROGRAM
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has no variable 'nope'")
	})

	t.Run("nested instance outer.inner();", func(t *testing.T) {
		eng := gvlEngine(t, "zn.st", fb+`
PROGRAM MAIN
VAR o : FB_Outer; END_VAR
o.inner();
END_PROGRAM
`)
		require.NoError(t, eng.Tick(time.Millisecond))
		inner := fbOutput(t, eng.env, "o", "inner")
		require.Equal(t, ValFBInstance, inner.Kind)
		assert.Equal(t, int64(1), inner.FBRef.GetMember("n").Int)
	})

	t.Run("unknown member on FB is still method not found", func(t *testing.T) {
		eng := gvlEngine(t, "zu.st", fb+`
PROGRAM MAIN
VAR o : FB_Outer; END_VAR
o.missing();
END_PROGRAM
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "method 'missing' not found")
	})

	t.Run("struct non-FB member call is an error", func(t *testing.T) {
		eng := gvlEngine(t, "zs.st", fb+`
PROGRAM MAIN
VAR s : ST_H; END_VAR
s.a();
END_PROGRAM
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot call method 'a'")
	})

	t.Run("non-FB variable called is still undefined function", func(t *testing.T) {
		eng := gvlEngine(t, "zv.st", fb+`
PROGRAM MAIN
VAR x : INT; END_VAR
x();
END_PROGRAM
`)
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "undefined function: X")
	})
}

// TestThreeLevelExtends covers method, property and action lookup on an FB
// that EXTENDS an FB that itself EXTENDS a third. findMethod used to recurse
// on the same parent forever and abort the process with a stack overflow.
func TestThreeLevelExtends(t *testing.T) {
	src := `
FUNCTION_BLOCK FB_A
VAR n : DINT; END_VAR
METHOD Bump : BOOL
n := n + 1;
END_METHOD
PROPERTY Count : DINT
GET
Count := n;
END_GET
END_PROPERTY
ACTION BaseAct
n := n + 100;
END_ACTION
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_B EXTENDS FB_A
METHOD Twice : BOOL
n := n + 2;
END_METHOD
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_C EXTENDS FB_B
ACTION Act
n := n + 10;
END_ACTION
END_FUNCTION_BLOCK
PROGRAM MAIN
VAR c : FB_C; ok : BOOL; seen : DINT; END_VAR
c.Act();
ok := c.Bump();
ok := c.Twice();
c.BaseAct();
seen := c.Count;
END_PROGRAM
`
	eng := gvlEngine(t, "ext3.st", src)
	require.NoError(t, eng.Tick(time.Millisecond))
	assert.Equal(t, int64(113), fbOutput(t, eng.env, "c", "n").Int)
	assert.Equal(t, int64(113), intVar(t, eng.env, "seen"))

	t.Run("unknown member on a 3-level chain is still an error", func(t *testing.T) {
		eng := gvlEngine(t, "ext3b.st", strings.Replace(src, "c.Act();", "c.Nope();", 1))
		err := eng.Tick(time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "method 'Nope' not found on FB 'FB_C'")
	})
}
