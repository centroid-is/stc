package symtree

import (
	"testing"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const edgeSrc = `{attribute 'OPC.UA.DA' := '1'}
TYPE ST_A :
STRUCT
	x : INT;
END_STRUCT
END_TYPE

{attribute 'alias'}
TYPE T_A : ST_A; END_TYPE

FUNCTION_BLOCK FB_B
VAR
	v : INT;
	w : BOOL;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_D EXTENDS FB_B
VAR
	v : REAL;
END_VAR
END_FUNCTION_BLOCK

FUNCTION_BLOCK FB_Orphan EXTENDS FB_Missing
VAR
	o : INT;
END_VAR
END_FUNCTION_BLOCK

PROGRAM P
VAR
	s1 : STRING;
	s2 : STRING(80);
	s3 : WSTRING;
	s4 : STRING(GVL_X.N);
	al : T_A;
	d : FB_D;
	orphan : FB_Orphan;
	b : BYTE; si : SINT; i : INT; di : DINT; li : LINT;
	open : ARRAY[1..Unknown] OF INT;
	{attribute 'a'}
	{attribute 'b' := 'it''s'}
	flagged : INT;
END_VAR
VAR_TEMP
	tmp : INT;
END_VAR
END_PROGRAM
`

const extraSrc = `PROGRAM Extra
VAR
	a : ST_A;
	u : NoSuchType;
	p : POINTER TO INT;
	arr : ARRAY[0..1] OF INT;
	arr2 : ARRAY[0..Q] OF INT;
END_VAR
END_PROGRAM
`

func buildEdge(t *testing.T) *Tree {
	t.Helper()
	f := parser.Parse("p.st", edgeSrc).File
	res := analyzer.Analyze([]*ast.SourceFile{f}, nil)
	// Extra is parsed but not analysed, so it has no POU scope.
	extra := parser.Parse("extra.st", extraSrc).File
	res.Files = append(res.Files, extra)
	res.LibraryFiles = []*ast.SourceFile{nil}
	tree, err := Build(res)
	require.NoError(t, err)
	return tree
}

func lookup(t *testing.T, tree *Tree, path string) *Node {
	t.Helper()
	n, err := tree.Lookup(path)
	require.NoError(t, err, path)
	return n
}

func TestEdgeTypeNames(t *testing.T) {
	tree := buildEdge(t)
	assert.Equal(t, "STRING", lookup(t, tree, "P.s1").TypeName)
	assert.Equal(t, "STRING(80)", lookup(t, tree, "P.s2").TypeName)
	assert.Equal(t, "WSTRING", lookup(t, tree, "P.s3").TypeName)
	assert.Equal(t, "STRING", lookup(t, tree, "P.s4").TypeName)
	open := lookup(t, tree, "P.open")
	assert.Equal(t, KindArray, open.Kind)
	assert.False(t, open.Bounded)
	assert.Equal(t, "ARRAY[1..UNKNOWN] OF INT", open.TypeName)
	assert.Equal(t, "POINTER TO INT", lookup(t, tree, "Extra.p").TypeName)
	assert.Equal(t, "NoSuchType", lookup(t, tree, "Extra.u").TypeName)
	assert.Equal(t, "ARRAY[0..1] OF INT", lookup(t, tree, "Extra.arr").TypeName)
	assert.Equal(t, "ARRAY[0..?] OF INT", lookup(t, tree, "Extra.arr2").TypeName)
}

func TestEdgeAliasToStruct(t *testing.T) {
	al := lookup(t, buildEdge(t), "P.al")
	assert.Equal(t, KindStruct, al.Kind)
	assert.Equal(t, "T_A", al.TypeName)
	assert.Equal(t, []string{"OPC.UA.DA=1", "alias="}, attrNames(al))
	assert.Equal(t, []string{"x"}, childNames(al))
}

func TestEdgeExtendsOverrideAndOrphan(t *testing.T) {
	tree := buildEdge(t)
	d := lookup(t, tree, "P.d")
	assert.Equal(t, []string{"v", "w"}, childNames(d))
	assert.Equal(t, "REAL", child(t, d, "v").TypeName)
	assert.Equal(t, []string{"o"}, childNames(lookup(t, tree, "P.orphan")))
}

func TestEdgeTempSkippedAndUnanalysed(t *testing.T) {
	tree := buildEdge(t)
	_, err := tree.Lookup("P.tmp")
	assert.Error(t, err)
	a := lookup(t, tree, "Extra.a")
	assert.Equal(t, KindStruct, a.Kind)
	assert.Equal(t, []string{"x"}, childNames(a))
}

func TestEdgeStructMemberMissingFromType(t *testing.T) {
	f := parser.Parse("p.st", edgeSrc).File
	res := analyzer.Analyze([]*ast.SourceFile{f}, nil)
	// A member added after analysis is absent from the resolved struct
	// type; a member without a name is skipped.
	st := f.Declarations[0].(*ast.TypeDecl).Type.(*ast.StructType)
	added := parser.Parse("q.st", "TYPE Q : STRUCT extra : INT; END_STRUCT END_TYPE").File
	st.Members = append(st.Members, added.Declarations[0].(*ast.TypeDecl).Type.(*ast.StructType).Members[0], &ast.StructMember{})
	tree, err := Build(res)
	require.NoError(t, err)
	a := lookup(t, tree, "P.al")
	assert.Equal(t, []string{"x", "extra"}, childNames(a))
	assert.Nil(t, child(t, a, "extra").Type)
	assert.Equal(t, "INT", child(t, a, "extra").TypeName)
}

func TestEdgeBits(t *testing.T) {
	tree := buildEdge(t)
	for path, ok := range map[string]bool{
		"P.b.7": true, "P.b.8": false, "P.si.7": true, "P.i.15": true, "P.di.31": true, "P.di.32": false, "P.li.63": true,
	} {
		_, err := tree.Lookup(path)
		assert.Equal(t, ok, err == nil, path)
	}
}

func TestEdgeTextAttributes(t *testing.T) {
	txt := buildEdge(t).Text()
	assert.Contains(t, txt, "  P.flagged : INT {a, b := 'it''s'}\n")
	out, err := buildEdge(t).JSON()
	require.NoError(t, err)
	assert.Contains(t, string(out), `"type_name":"ARRAY[1..UNKNOWN] OF INT","section":"VAR","pos":{"file":"p.st","line":40,"col":2},"element":{"name":"[*]"`)
}

func TestEdgeGVLNameClash(t *testing.T) {
	src := `VAR_GLOBAL
	x : INT;
END_VAR
PROGRAM Clash
END_PROGRAM
`
	f := parser.Parse("Clash.st", src).File
	tree, err := Build(analyzer.Analyze([]*ast.SourceFile{f}, nil))
	require.NoError(t, err)
	require.Len(t, tree.Roots, 2)
	assert.Equal(t, KindGVL, tree.Roots[0].Kind)
	assert.Equal(t, "INT", child(t, tree.Roots[0], "x").TypeName)
	assert.NotNil(t, child(t, tree.Roots[0], "x").Type)
}

func TestEdgeLibraryGVLOverridden(t *testing.T) {
	lib := parser.Parse("G.st", "VAR_GLOBAL\n\tlibVar : INT;\nEND_VAR\n").File
	user := parser.Parse("G.st", "VAR_GLOBAL\n\tuserVar : INT;\nEND_VAR\n").File
	res := analyzer.Analyze([]*ast.SourceFile{user}, nil, analyzer.AnalyzeOpts{LibraryFiles: []*ast.SourceFile{lib}})
	tree, err := Build(res)
	require.NoError(t, err)
	require.Len(t, tree.Roots, 1)
	assert.Equal(t, []string{"userVar"}, childNames(tree.Roots[0]))
}
