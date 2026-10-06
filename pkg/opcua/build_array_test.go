package opcua

import (
	"reflect"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/types"
)

// TestBuildSkipsUnservableArrays is the ME-03 regression: the runtime
// models only the first dimension of an array and at most 10000 slots, so
// such arrays are not published (with a warning) rather than published
// with values that never read. A struct holding one is served as an
// Object whose other members stay readable.
func TestBuildSkipsUnservableArrays(t *testing.T) {
	grid := arr(types.TypeINT, [2]int{0, 1}, [2]int{0, 2})
	big := arr(types.TypeINT, [2]int{1, 10000})
	edge := arr(types.TypeINT, [2]int{0, 9999})
	stGrid := &types.StructType{Name: "ST_Grid", Members: []types.StructMember{
		{Name: "grid", Type: grid}, {Name: "n", Type: types.TypeINT}}}
	sp, ds := Build(root(gvl("G",
		node(KindArray, "grid", grid, attrs(da("1"))),
		node(KindArray, "big", big, attrs(da("1"))),
		node(KindArray, "edge", edge, attrs(da("1"))),
		node(KindStruct, "s", stGrid, attrs(da("1"), structured()),
			node(KindArray, "grid", grid, nil),
			node(KindScalar, "n", types.TypeINT, nil)),
	)), nil)
	want := []string{"G|Obj||", "G.edge|Var||RW", "G.s|Obj||", "G.s.n|Var||RW"}
	if got := shape(sp); !reflect.DeepEqual(got, want) {
		t.Errorf("nodes %q, want %q", got, want)
	}
	msgs := make([]string, 0, len(ds))
	for _, d := range ds {
		msgs = append(msgs, d.Code+" "+d.Message)
	}
	joined := strings.Join(msgs, "\n")
	for _, w := range []string{
		"G.grid: ", "multi-dimensional arrays are not served yet",
		"G.big: ", "arrays beyond index 9999 are not served",
		"G.s: ", "published as an Object",
	} {
		if !strings.Contains(joined, w) {
			t.Errorf("diagnostics lack %q:\n%s", w, joined)
		}
	}
	if err := arrayServable(arr(types.TypeINT, [2]int{0, 1})); err != nil {
		t.Errorf("1-D array: %v", err)
	}
}
