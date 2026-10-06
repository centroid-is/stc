package symtree

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePath(t *testing.T) {
	segs, err := ParsePath("GVL.fb[2].HMI.p_stat_State")
	require.NoError(t, err)
	assert.Equal(t, []Segment{
		{Name: "GVL"}, {Name: "fb"}, {IsIndex: true, Index: 2}, {Name: "HMI"}, {Name: "p_stat_State"},
	}, segs)

	segs, err = ParsePath("x.3")
	require.NoError(t, err)
	assert.Equal(t, []Segment{{Name: "x"}, {IsBit: true, Bit: 3}}, segs)

	segs, err = ParsePath("_a[-2][0]")
	require.NoError(t, err)
	assert.Equal(t, []Segment{{Name: "_a"}, {IsIndex: true, Index: -2}, {IsIndex: true, Index: 0}}, segs)
}

func TestParsePathErrors(t *testing.T) {
	cases := map[string]string{
		"":                        "empty",
		"a..b":                    "expected",
		"a[":                      "index",
		"a[x]":                    "index",
		"a[1,2]":                  "multi-dimensional arrays not supported",
		"a[1":                     "]",
		"[1]":                     "must start",
		"1a":                      "must start",
		"a.":                      "expected",
		"a b":                     "unexpected",
		"a[99999999999999999999]": "index",
		"a.99999999999999999999":  "bit",
		"a[-]":                    "index",
		strings.Repeat("a", 1025): "longer",
	}
	for in, want := range cases {
		_, err := ParsePath(in)
		if assert.Error(t, err, in) {
			assert.Contains(t, err.Error(), want, in)
		}
	}
}

func TestLookup(t *testing.T) {
	tree := buildFixture(t)
	n, err := tree.Lookup("gvl.FB[2].hmi.P_STAT_STATE")
	require.NoError(t, err)
	assert.Equal(t, "GVL.fb[2].HMI.p_stat_State", n.Path)
	assert.Equal(t, KindEnum, n.Kind)
	assert.Equal(t, "Idle", n.EnumStrings[0])

	n, err = tree.Lookup("MAIN")
	require.NoError(t, err)
	assert.Equal(t, KindProgram, n.Kind)

	n, err = tree.Lookup("GVL.fb[3].t.Q")
	require.NoError(t, err)
	assert.Equal(t, "GVL.fb[3].t.Q", n.Path)

	n, err = tree.Lookup("MAIN.w.15")
	require.NoError(t, err)
	assert.Equal(t, "MAIN.w.15", n.Path)
	assert.Equal(t, "BOOL", n.TypeName)
	assert.Equal(t, KindScalar, n.Kind)
}

func TestLookupErrors(t *testing.T) {
	tree := buildFixture(t)
	cases := map[string]string{
		"GVL.fb[0]":             "out of bounds",
		"GVL.fb[4]":             "out of bounds",
		"GVL.fb[1].nope":        "nope",
		"Nope":                  "unknown root",
		"GVL.counter[1]":        "not an array",
		"MAIN.grid[1]":          "bounds",
		"GVL.fb.HMI":            "array",
		"MAIN.w.16":             "out of range",
		"GVL.fb[1].speed.0":     "speed",
		"GVL.fb[1].HMI.speed.0": "bit access",
		"MAIN.w.1.2":            "bit access",
		"GVL.alias.0":           "bit access",
		"a[":                    "index",
	}
	for in, want := range cases {
		_, err := tree.Lookup(in)
		if assert.Error(t, err, in) {
			assert.Contains(t, err.Error(), want, in)
		}
	}
}

func TestWalk(t *testing.T) {
	tree := buildFixture(t)
	var paths []string
	tree.Walk(func(n *Node) bool {
		paths = append(paths, n.Path)
		return n.Kind != KindFBInstance
	}, WalkOpts{})
	assert.Contains(t, paths, "GVL.fb")
	assert.NotContains(t, paths, "GVL.fb[1]")
	assert.Contains(t, paths, "MAIN.drive")
	assert.NotContains(t, paths, "MAIN.drive.HMI", "fn returning false skips the subtree")

	paths = nil
	tree.Walk(func(n *Node) bool {
		paths = append(paths, n.Path)
		return true
	}, WalkOpts{ExpandArrays: true})
	assert.Contains(t, paths, "GVL.fb[3].HMI.p_stat_State")
	assert.Contains(t, paths, "GVL.fb[1].states[1]")
	assert.Equal(t, "LibGVL", paths[0])
}

func TestJSONDeterministic(t *testing.T) {
	a, err := buildFixture(t).JSON()
	require.NoError(t, err)
	b, err := buildFixture(t).JSON()
	require.NoError(t, err)
	assert.Equal(t, a, b)

	var doc struct {
		Roots []map[string]any `json:"roots"`
	}
	require.NoError(t, json.Unmarshal(a, &doc))
	require.Len(t, doc.Roots, 3)
	s := string(a)
	assert.Contains(t, s, `"enum_strings":{"0":"Idle","1":"Run"}`)
	assert.Contains(t, s, `"attributes":[{"name":"OPC.UA.DA","value":"1"}]`)
	assert.Contains(t, s, `"low":1,"high":3,"element":{"name":"[*]","path":"GVL.fb[*]"`)
	assert.NotContains(t, s, `"GVL.fb[2]"`)
	assert.Contains(t, s, `"kind":"fb_instance"`)
	assert.Contains(t, s, `"section":"VAR_INPUT"`)
}

func TestJSONEnumOrdinalOrder(t *testing.T) {
	src := `TYPE E : (a := 10, b := 2, c := -1); END_TYPE
PROGRAM P
VAR e : E; END_VAR
END_PROGRAM
`
	f := parser.Parse("p.st", src).File
	tree, err := Build(analyzer.Analyze([]*ast.SourceFile{f}, nil))
	require.NoError(t, err)
	out, err := tree.JSON()
	require.NoError(t, err)
	assert.Contains(t, string(out), `"enum_strings":{"-1":"c","2":"b","10":"a"}`)
}

func TestText(t *testing.T) {
	txt := buildFixture(t).Text()
	assert.Contains(t, txt, "GVL\n")
	assert.Contains(t, txt, "  GVL.fb : ARRAY[1..3] OF FB_Drive\n")
	assert.Contains(t, txt, "  GVL.counter : DINT {OPC.UA.DA := '0'}\n")
	assert.Contains(t, txt, "    MAIN.drive.HMI : ST_HMI {OPC.UA.DA := '1'}\n")
	assert.Contains(t, txt, "      MAIN.drive.HMI.p_stat_State : E_State {OPC.UA.DA.Access := '1'}\n")
	assert.NotContains(t, txt, "GVL.fb[1]")
	assert.Contains(t, txt, "  MAIN.drive : FB_Drive {OPC.UA.DA := '1'}\n")
	assert.Contains(t, txt, "    MAIN.drive.enable : BOOL\n")
}

func FuzzParsePath(f *testing.F) {
	for _, s := range []string{"GVL.fb[2].HMI.p_stat_State", "x.3", "a[1,2]", "", "a..b", "a[", "a[-1]", "a.b.c[0][1].7"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		segs, err := ParsePath(s)
		if err == nil && len(segs) == 0 {
			t.Fatalf("no error and no segments for %q", s)
		}
	})
}
