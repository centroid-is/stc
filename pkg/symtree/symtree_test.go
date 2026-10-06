package symtree

import (
	"testing"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const typesSrc = `TYPE E_State :
(
	Idle := 0,
	Run := 1
);
END_TYPE

{attribute 'OPC.UA.DA' := '1'}
TYPE ST_HMI :
STRUCT
	{attribute 'OPC.UA.DA.Access' := '1'}
	p_stat_State : E_State;
	speed : REAL;
END_STRUCT
END_TYPE

TYPE ST_Rec :
STRUCT
	a : INT;
	self : ST_Rec;
END_STRUCT
END_TYPE

TYPE T_Alias : E_State; END_TYPE
`

const fbSrc = `{attribute 'fb_base'}
FUNCTION_BLOCK FB_Base
VAR_INPUT
	enable : BOOL;
END_VAR
VAR_OUTPUT
	busy : BOOL;
END_VAR
END_FUNCTION_BLOCK

{attribute 'OPC.UA.DA' := '1'}
FUNCTION_BLOCK FB_Drive EXTENDS FB_Base
VAR_IN_OUT
	io : INT;
END_VAR
VAR
	HMI : ST_HMI;
	t : TON;
	p : POINTER TO FB_Drive;
	r : REFERENCE TO INT;
	states : ARRAY[0..1] OF E_State;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Loop
VAR
	inner : FB_Loop;
END_VAR
END_FUNCTION_BLOCK
`

const gvlSrc = `VAR_GLOBAL
	fb : ARRAY[1..3] OF FB_Drive;
	{attribute 'OPC.UA.DA' := '0'}
	counter : DINT;
	alias : T_Alias;
	rec : ST_Rec;
	loop : FB_Loop;
END_VAR
VAR_GLOBAL CONSTANT
	N : INT := 3;
END_VAR
VAR_GLOBAL RETAIN
	kept : INT;
END_VAR
VAR_GLOBAL PERSISTENT
	saved : INT;
END_VAR
`

const mainSrc = `PROGRAM MAIN
VAR
	drive : FB_Drive;
	grid : ARRAY[1..2, 1..2] OF INT;
	dyn : ARRAY[1..GVL.N] OF INT;
	w : WORD;
END_VAR
END_PROGRAM
`

const libSrc = `VAR_GLOBAL
	libFlag : BOOL;
END_VAR
`

func buildFixture(t *testing.T) *Tree {
	t.Helper()
	var files []*ast.SourceFile
	for _, f := range []struct{ name, src string }{
		{"types.st", typesSrc}, {"fbs.st", fbSrc}, {"GVL.st", gvlSrc}, {"main.st", mainSrc},
	} {
		r := parser.Parse(f.name, f.src)
		require.Empty(t, r.Diags, f.name)
		files = append(files, r.File)
	}
	lib := parser.Parse("LibGVL.st", libSrc).File
	res := analyzer.Analyze(files, nil, analyzer.AnalyzeOpts{LibraryFiles: []*ast.SourceFile{lib}})
	tree, err := Build(res)
	require.NoError(t, err)
	return tree
}

func childNames(n *Node) []string {
	var out []string
	for _, c := range n.Children() {
		out = append(out, c.Name)
	}
	return out
}

func child(t *testing.T, n *Node, name string) *Node {
	t.Helper()
	for _, c := range n.Children() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("%s has no child %s (children %v)", n.Path, name, childNames(n))
	return nil
}

func attrNames(n *Node) []string {
	var out []string
	for _, a := range n.Attributes {
		out = append(out, a.Name+"="+a.Value)
	}
	return out
}

func TestBuildRootsOrder(t *testing.T) {
	tree := buildFixture(t)
	var names []string
	for _, r := range tree.Roots {
		names = append(names, r.Name)
	}
	assert.Equal(t, []string{"LibGVL", "GVL", "MAIN"}, names)
	assert.Equal(t, KindGVL, tree.Roots[1].Kind)
	assert.Equal(t, KindProgram, tree.Roots[2].Kind)
	assert.Equal(t, "GVL", tree.Roots[1].Path)
	assert.Equal(t, []string{"fb", "counter", "alias", "rec", "loop", "N", "kept", "saved"}, childNames(tree.Roots[1]))
	assert.Equal(t, "GVL.st", tree.Roots[1].Pos.File)
}

func TestBuildGVLFlags(t *testing.T) {
	gvl := buildFixture(t).Roots[1]
	n := child(t, gvl, "N")
	assert.True(t, n.Constant)
	assert.Equal(t, ast.VarGlobal, n.Section)
	assert.True(t, child(t, gvl, "kept").Retain)
	assert.True(t, child(t, gvl, "saved").Persistent)
	c := child(t, gvl, "counter")
	assert.Equal(t, KindScalar, c.Kind)
	assert.Equal(t, "DINT", c.TypeName)
	assert.Equal(t, []string{"OPC.UA.DA=0"}, attrNames(c))
	assert.Equal(t, 4, c.Pos.Line)
}

func TestBuildFBExtendsOrderAndSections(t *testing.T) {
	gvl := buildFixture(t).Roots[1]
	fb := child(t, gvl, "fb")
	require.Equal(t, KindArray, fb.Kind)
	assert.Equal(t, 1, fb.Low)
	assert.Equal(t, 3, fb.High)
	assert.Equal(t, "ARRAY[1..3] OF FB_Drive", fb.TypeName)
	elems := fb.Children()
	require.Len(t, elems, 3)
	drive := elems[1]
	assert.Equal(t, "[2]", drive.Name)
	assert.Equal(t, "GVL.fb[2]", drive.Path)
	assert.Equal(t, KindFBInstance, drive.Kind)
	assert.Equal(t, "FB_Drive", drive.TypeName)
	assert.Equal(t, []string{"OPC.UA.DA=1"}, attrNames(drive))
	assert.Equal(t, []string{"enable", "busy", "io", "HMI", "t", "p", "r", "states"}, childNames(drive))
	assert.Equal(t, ast.VarInput, child(t, drive, "enable").Section)
	assert.Equal(t, ast.VarOutput, child(t, drive, "busy").Section)
	assert.Equal(t, ast.VarInOut, child(t, drive, "io").Section)
	assert.Equal(t, ast.VarLocal, child(t, drive, "HMI").Section)
	assert.Equal(t, KindPointer, child(t, drive, "p").Kind)
	assert.Empty(t, child(t, drive, "p").Children())
	assert.Equal(t, "POINTER TO FB_Drive", child(t, drive, "p").TypeName)
	assert.Equal(t, KindReference, child(t, drive, "r").Kind)
	assert.Equal(t, "REFERENCE TO INT", child(t, drive, "r").TypeName)
}

func TestBuildStructEnumMember(t *testing.T) {
	gvl := buildFixture(t).Roots[1]
	drive := child(t, gvl, "fb").Children()[1]
	hmi := child(t, drive, "HMI")
	assert.Equal(t, KindStruct, hmi.Kind)
	assert.Equal(t, "ST_HMI", hmi.TypeName)
	assert.Equal(t, []string{"OPC.UA.DA=1"}, attrNames(hmi))
	st := child(t, hmi, "p_stat_State")
	assert.Equal(t, "GVL.fb[2].HMI.p_stat_State", st.Path)
	assert.Equal(t, KindEnum, st.Kind)
	assert.Equal(t, "E_State", st.TypeName)
	_, isEnum := st.Type.(*types.EnumType)
	assert.True(t, isEnum)
	assert.Equal(t, map[int64]string{0: "Idle", 1: "Run"}, st.EnumStrings)
	assert.Equal(t, []string{"OPC.UA.DA.Access=1"}, attrNames(st))
	assert.Equal(t, KindScalar, child(t, hmi, "speed").Kind)

	states := child(t, drive, "states")
	el := states.Children()[0]
	assert.Equal(t, KindEnum, el.Kind)
	assert.Equal(t, "E_State", el.TypeName)
	assert.Len(t, el.EnumStrings, 2)
}

func TestBuildTypeLevelThenInstanceAttributes(t *testing.T) {
	src := `{attribute 'OPC.UA.DA' := '1'}
TYPE ST_A :
STRUCT
	x : INT;
END_STRUCT
END_TYPE
PROGRAM P
VAR
	{attribute 'OPC.UA.DA.Access' := '3'}
	{attribute 'flag'}
	a : ST_A;
END_VAR
END_PROGRAM
`
	f := parser.Parse("p.st", src).File
	tree, err := Build(analyzer.Analyze([]*ast.SourceFile{f}, nil))
	require.NoError(t, err)
	a := child(t, tree.Roots[0], "a")
	assert.Equal(t, []string{"OPC.UA.DA=1", "OPC.UA.DA.Access=3", "flag="}, attrNames(a))
}

func TestBuildAliasToEnum(t *testing.T) {
	gvl := buildFixture(t).Roots[1]
	al := child(t, gvl, "alias")
	assert.Equal(t, KindEnum, al.Kind)
	assert.Equal(t, "T_Alias", al.TypeName)
	assert.Equal(t, "Run", al.EnumStrings[1])
}

func TestBuildCycleGuard(t *testing.T) {
	gvl := buildFixture(t).Roots[1]
	rec := child(t, gvl, "rec")
	assert.Equal(t, KindStruct, rec.Kind)
	self := child(t, rec, "self")
	assert.Equal(t, KindStruct, self.Kind)
	assert.Empty(t, self.Children())

	loop := child(t, gvl, "loop")
	inner := child(t, loop, "inner")
	assert.Equal(t, KindFBInstance, inner.Kind)
	assert.Empty(t, inner.Children())
}

func TestBuildStandardFB(t *testing.T) {
	gvl := buildFixture(t).Roots[1]
	drive := child(t, gvl, "fb").Children()[0]
	tn := child(t, drive, "t")
	assert.Equal(t, KindFBInstance, tn.Kind)
	assert.Equal(t, "TON", tn.TypeName)
	assert.Equal(t, []string{"IN", "PT", "Q", "ET"}, childNames(tn))
	assert.Equal(t, ast.VarInput, child(t, tn, "PT").Section)
	assert.Equal(t, ast.VarOutput, child(t, tn, "Q").Section)
	assert.Equal(t, "GVL.fb[1].t.ET", child(t, tn, "ET").Path)
}

func TestBuildProgramArrays(t *testing.T) {
	main := buildFixture(t).Roots[2]
	assert.Equal(t, []string{"drive", "grid", "dyn", "w"}, childNames(main))
	grid := child(t, main, "grid")
	assert.Equal(t, KindArray, grid.Kind)
	assert.Empty(t, grid.Children(), "multi-dimensional arrays are not expanded")
	dyn := child(t, main, "dyn")
	assert.Equal(t, KindArray, dyn.Kind)
	assert.Equal(t, 1, dyn.Low)
	assert.Equal(t, 3, dyn.High)
	assert.Len(t, dyn.Children(), 3)
	assert.Equal(t, []string{"enable", "busy", "io", "HMI", "t", "p", "r", "states"}, childNames(child(t, main, "drive")))
}

func TestBuildNilSymbols(t *testing.T) {
	_, err := Build(analyzer.AnalysisResult{})
	assert.Error(t, err)
}

func TestKindString(t *testing.T) {
	for k, want := range map[Kind]string{
		KindGVL: "gvl", KindProgram: "program", KindFBInstance: "fb_instance", KindStruct: "struct",
		KindArray: "array", KindScalar: "scalar", KindEnum: "enum", KindReference: "reference",
		KindPointer: "pointer", Kind(99): "unknown",
	} {
		assert.Equal(t, want, k.String())
	}
}
