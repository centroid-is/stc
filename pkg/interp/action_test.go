package interp

import (
	"os"
	"path/filepath"
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
}
