package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/symtree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadST301Shape parses the ST301-shaped fixture in sorted file order, so
// ECT_Diag.st (which needs EcDiagParam.MAX_EC_SLAVES) comes first. Each
// top-level VAR_GLOBAL is a GVL named after its file basename.
func loadST301Shape(t *testing.T) []*ast.SourceFile {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("runtime", "st301_shape", "*.st"))
	require.NoError(t, err)
	sort.Strings(paths)
	require.Len(t, paths, 4)
	var files []*ast.SourceFile
	for _, p := range paths {
		src, err := os.ReadFile(p)
		require.NoError(t, err)
		res := parser.Parse(filepath.Base(p), string(src))
		require.Empty(t, res.Diags, p)
		files = append(files, res.File)
	}
	return files
}

func TestRuntimeFixture(t *testing.T) {
	files := loadST301Shape(t)

	t.Run("SC1", func(t *testing.T) {
		res := analyzer.Analyze(files, nil)
		for _, d := range res.Diags {
			assert.NotEqual(t, diag.Error, d.Severity, "%s %s", d.Code, d.Message)
		}
		tree, err := symtree.Build(res)
		require.NoError(t, err)
		n, err := tree.Lookup("GVL.fb[2].HMI.p_stat_State")
		require.NoError(t, err)
		assert.Equal(t, "GVL.fb[2].HMI.p_stat_State", n.Path)
		assert.Equal(t, symtree.KindEnum, n.Kind)
		assert.Equal(t, "hmis_e", n.TypeName)
		assert.Equal(t, map[int64]string{0: "Idle", 1: "Running", 2: "Faulted"}, n.EnumStrings)
		var attrs []string
		for _, a := range n.Attributes {
			attrs = append(attrs, a.Name)
		}
		assert.Equal(t, []string{"to_string", "OPC.UA.DA.Access"}, attrs)
		hmi, err := tree.Lookup("gvl.FB[2].hmi")
		require.NoError(t, err)
		require.Len(t, hmi.Attributes, 1)
		assert.Equal(t, "OPC.UA.DA.StructuredType", hmi.Attributes[0].Name)
	})

	rt, err := interp.NewRuntime(files)
	require.NoError(t, err)

	t.Run("SC4", func(t *testing.T) {
		// Cycle 0: initialisers over a constant-bound array are visible
		// before the first Tick.
		v, err := rt.Get("ECT_Diag.Device_1_SlaveInfo[3].p_stat_sName")
		require.NoError(t, err)
		assert.Equal(t, "S.A1.03 (EL2008)", v.Str)
		v, err = rt.Get("ECT_Diag.Device_1_SlaveInfo[1].p_stat_nPhysAddr")
		require.NoError(t, err)
		assert.Equal(t, int64(1001), v.Int)
		v, err = rt.Get("ECT_Diag.Device_1_SlaveInfo[4].p_stat_sName")
		require.NoError(t, err)
		assert.Equal(t, "", v.Str, "element 4 has no initialiser")
		_, err = rt.Get("ECT_Diag.Device_1_SlaveInfo[5]")
		assert.Error(t, err)
		v, err = rt.Get("GVL.fb[1].HMI.p_cfg_ManualFreq")
		require.NoError(t, err)
		assert.Equal(t, 20.0, v.Real)
	})

	t.Run("SC2", func(t *testing.T) {
		require.NoError(t, rt.Set("GVL.x.HMI.p_cmd_Start", true))
		v, err := rt.Get("GVL.x.HMI.p_cmd_Start")
		require.NoError(t, err)
		assert.True(t, v.Bool)
		require.NoError(t, rt.Set("GVL.fb[2].HMI.p_stat_State", "Faulted"))
		require.NoError(t, rt.Set("GVL.fb[2].HMI.p_cfg_ManualFreq", json.Number("42.5")))
		v, err = rt.Get("GVL.fb[2].HMI")
		require.NoError(t, err)
		b, err := json.Marshal(rt.ToJSON(v))
		require.NoError(t, err)
		assert.JSONEq(t, `{"p_cfg_ManualFreq":42.5,"p_stat_xAuto":true,"p_stat_State":"Faulted","p_cmd_Start":false,"p_stat_Error":false}`, string(b))
		assert.Error(t, rt.Set("EcDiagParam.MAX_EC_SLAVES", 8), "constant")
		require.NoError(t, rt.Tick(10*time.Millisecond))
		v, err = rt.Get("GVL.x.HMI.p_stat_State")
		require.NoError(t, err)
		assert.Equal(t, int64(1), v.Int, "FB body reacted to the command")
	})

	t.Run("SC3", func(t *testing.T) {
		require.NoError(t, rt.Set("GVL.x.n", 32767))
		require.NoError(t, rt.Tick(10*time.Millisecond))
		v, err := rt.Get("GVL.x.n")
		require.NoError(t, err)
		assert.Equal(t, int64(-32768), v.Int, "INT wraps on n := n + 1")
	})

	t.Run("agreement", func(t *testing.T) {
		tree, err := symtree.Build(analyzer.Analyze(files, nil))
		require.NoError(t, err)
		leaves := 0
		tree.Walk(func(n *symtree.Node) bool {
			if n.Kind == symtree.KindScalar || n.Kind == symtree.KindEnum {
				leaves++
				_, err := rt.Get(n.Path)
				assert.NoError(t, err, n.Path)
			}
			return true
		}, symtree.WalkOpts{ExpandArrays: true})
		// 4 slaves x 3 + 3 drives x 6 + 1 constant.
		assert.Equal(t, 31, leaves)
	})
}

func TestLiteralClassPattern(t *testing.T) {
	for _, m := range []string{
		"cannot assign LREAL to REAL",
		"cannot assign DINT to INT",
		`cannot pass DINT as input parameter "nIndex" (expected BYTE)`,
		"cannot compare BYTE and DINT",
		"array index must be an integer type, got LREAL",
	} {
		assert.True(t, literalClass.MatchString(m), m)
	}
	assert.False(t, literalClass.MatchString("cannot assign WORD to UINT"))
}
