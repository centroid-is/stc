package bind

import (
	"testing"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/symtree"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bindTypes = `TYPE E_State :
(
	Idle := 0,
	Run := 1
);
END_TYPE

{attribute 'OPC.UA.DA.StructuredType' := '1'}
TYPE ST_HMI :
STRUCT
	{attribute 'OPC.UA.DA.Access' := '1'}
	state : E_State;
	speed : REAL;
END_STRUCT
END_TYPE

{attribute 'OPC.UA.DA' := '1'}
FUNCTION_BLOCK FB_Drive
VAR
	{attribute 'OPC.UA.DA' := '0'}
	HMI : ST_HMI;
	p : POINTER TO INT;
	r : REFERENCE TO INT;
END_VAR
END_FUNCTION_BLOCK
`

const bindGVL = `VAR_GLOBAL
	{attribute 'OPC.UA.DA' := '1'}
	drive : FB_Drive;
	arr : ARRAY[2..4] OF INT;
	e : E_State := E_State.Run;
	x : BOOL;
END_VAR
VAR_GLOBAL CONSTANT
	K : INT := 3;
END_VAR
`

const bindMain = `PROGRAM MAIN
VAR
	n : DINT;
	y : DINT;
END_VAR
n := n + 1;
y := n;
GVL.x := NOT GVL.x;
END_PROGRAM
`

// bindFiles parses the shared test sources.
func bindFiles(t *testing.T) []*ast.SourceFile {
	t.Helper()
	var files []*ast.SourceFile
	for _, s := range [][2]string{{"types.st", bindTypes}, {"GVL.st", bindGVL}, {"main.st", bindMain}} {
		r := parser.Parse(s[0], s[1])
		require.Empty(t, r.Diags, s[0])
		files = append(files, r.File)
	}
	return files
}

func bindTree(t *testing.T) *symtree.Tree {
	t.Helper()
	res := analyzer.Analyze(bindFiles(t), nil)
	tree, err := symtree.Build(res)
	require.NoError(t, err)
	return tree
}

func childNamed(t *testing.T, n opcua.SymbolNode, name string) opcua.SymbolNode {
	t.Helper()
	for _, c := range n.Children() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("%q has no child %q", n.Path(), name)
	return nil
}

func TestSymtreeRoot(t *testing.T) {
	r := Root(bindTree(t))
	assert.Equal(t, opcua.KindRoot, r.Kind())
	assert.Empty(t, r.Path())
	assert.Empty(t, r.Name())
	assert.Nil(t, r.Type())
	assert.Empty(t, r.TypeName())
	assert.Nil(t, r.Attributes())
	assert.Nil(t, r.EnumStrings())
	var names []string
	for _, c := range r.Children() {
		names = append(names, c.Name())
	}
	assert.Equal(t, []string{"GVL", "MAIN"}, names)
	assert.Equal(t, opcua.KindGVL, r.Children()[0].Kind())
	assert.Equal(t, opcua.KindProgram, r.Children()[1].Kind())
	assert.Nil(t, r.Children()[0].Type())

	assert.Empty(t, Root(nil).Children())
}

func TestSymtreeNodes(t *testing.T) {
	g := Root(bindTree(t)).Children()[0]

	drive := childNamed(t, g, "drive")
	assert.Equal(t, opcua.KindFBInstance, drive.Kind())
	assert.Equal(t, "GVL.drive", drive.Path())
	assert.Equal(t, "FB_Drive", drive.TypeName())
	assert.IsType(t, &types.FunctionBlockType{}, drive.Type())
	// The FB header attribute comes first, the instance declaration last.
	da := drive.Attributes()
	require.Len(t, da, 2)
	assert.Equal(t, ast.Attribute{Name: "OPC.UA.DA", Value: "1", HasValue: true}, ast.Attribute{Name: da[0].Name, Value: da[0].Value, HasValue: da[0].HasValue})
	assert.Equal(t, "OPC.UA.DA", da[1].Name)

	hmi := childNamed(t, drive, "HMI")
	assert.Equal(t, opcua.KindStruct, hmi.Kind())
	assert.IsType(t, &types.StructType{}, hmi.Type())
	ha := hmi.Attributes()
	require.Len(t, ha, 2)
	assert.Equal(t, "OPC.UA.DA.StructuredType", ha[0].Name, "TYPE header first")
	assert.Equal(t, "0", ha[1].Value, "member declaration last")

	st := childNamed(t, hmi, "state")
	assert.Equal(t, opcua.KindEnum, st.Kind())
	assert.Equal(t, map[int64]string{0: "Idle", 1: "Run"}, st.EnumStrings())
	assert.Equal(t, "GVL.drive.HMI.state", st.Path())
	assert.Equal(t, "OPC.UA.DA.Access", st.Attributes()[0].Name)

	sp := childNamed(t, hmi, "speed")
	assert.Equal(t, opcua.KindScalar, sp.Kind())
	assert.Nil(t, sp.Attributes())
	assert.Empty(t, sp.Children())

	assert.Equal(t, opcua.KindPointer, childNamed(t, drive, "p").Kind())
	assert.Equal(t, opcua.KindReference, childNamed(t, drive, "r").Kind())

	arr := childNamed(t, g, "arr")
	assert.Equal(t, opcua.KindArray, arr.Kind())
	els := arr.Children()
	require.Len(t, els, 3)
	for i, el := range els {
		idx := []string{"2", "3", "4"}[i]
		assert.Equal(t, "arr["+idx+"]", el.Name())
		assert.Equal(t, "GVL.arr["+idx+"]", el.Path())
		assert.Equal(t, opcua.KindScalar, el.Kind())
	}
	// Elements are synthesised on each call.
	assert.Len(t, arr.Children(), 3)
}

func TestSymtreeKindMapping(t *testing.T) {
	cases := map[symtree.Kind]opcua.Kind{
		symtree.KindGVL:        opcua.KindGVL,
		symtree.KindProgram:    opcua.KindProgram,
		symtree.KindFBInstance: opcua.KindFBInstance,
		symtree.KindStruct:     opcua.KindStruct,
		symtree.KindArray:      opcua.KindArray,
		symtree.KindScalar:     opcua.KindScalar,
		symtree.KindEnum:       opcua.KindEnum,
		symtree.KindReference:  opcua.KindReference,
		symtree.KindPointer:    opcua.KindPointer,
		symtree.Kind(99):       opcua.KindScalar,
	}
	for in, want := range cases {
		assert.Equal(t, want, kindOf(in), in.String())
	}
}

func TestSymtreeNilChildrenSkipped(t *testing.T) {
	out := wrap([]*symtree.Node{nil, {Name: "a", Path: "G.a", Attributes: []*ast.Attribute{nil, {Name: "x"}}}}, nil)
	require.Len(t, out, 1)
	assert.Equal(t, []ast.Attribute{{Name: "x"}}, out[0].Attributes())
}
