package interp

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const autoStubSrc = `
TYPE ST_R : STRUCT a : INT; END_STRUCT END_TYPE
VAR_GLOBAL
	conv  : FB_Unknown;
	conv2 : FB_Unknown;
	arr   : ARRAY[0..1] OF FB_Other;
	r     : ST_R;
	rs    : ARRAY[0..1] OF ST_R;
END_VAR
PROGRAM MAIN
VAR
	y : BOOL;
	z : INT;
	echo : INT;
END_VAR
conv(Enable := TRUE, Speed := 7);
y := conv.Busy;
echo := conv.Speed;
conv.M_Start(1);
arr[1]();
z := r.missing;
z := rs[1].missing;
END_PROGRAM
`

func TestAutoStubUndeclaredTypes(t *testing.T) {
	p, err := LoadProject(ProjectSpec{Files: []*ast.SourceFile{parseRT(t, "G.st", autoStubSrc)}})
	require.NoError(t, err, "undeclared FB types do not abort the load")
	require.NoError(t, p.Tick())
	require.NoError(t, p.Tick())

	rt := p.Runtime()
	get := func(path string) Value {
		v, err := rt.Get(path)
		require.NoError(t, err, path)
		return v
	}
	assert.False(t, get("MAIN.y").Bool, "outputs read as zero")
	assert.Equal(t, int64(7), get("MAIN.echo").Int, "inputs written by the program read back")
	assert.Equal(t, int64(0), get("MAIN.z").Int)

	ws := rt.Interpreter().Warnings()
	var msgs []string
	for _, w := range ws {
		assert.Equal(t, diag.Warning, w.Severity)
		msgs = append(msgs, w.Code+" "+w.Message)
	}
	assert.Equal(t, []string{
		"RUNT001 type FB_Unknown is not declared; its instances run as zero-output auto-stubs",
		"RUNT001 type FB_Other is not declared; its instances run as zero-output auto-stubs",
		"RUNT002 r.missing: structure has no member 'missing'; it reads as zero",
		"RUNT002 rs[1].missing: structure has no member 'missing'; it reads as zero",
	}, msgs, "one warning per type and path, not per instance or tick")
	assert.Equal(t, "G.st", ws[0].Pos.File)
	assert.Positive(t, ws[0].Pos.Line)

	s := newAutoStubFB()
	s.SetInput("x", IntValue(3))
	assert.Equal(t, int64(3), s.GetInput("X").Int)
	s.Execute(0)
	in := New()
	in.warn("C", nil, "m")
	assert.Equal(t, 0, in.Warnings()[0].Pos.Line)
	assert.Equal(t, "a[i, 2]", exprText(&ast.IndexExpr{Object: &ast.Ident{Name: "a"}, Indices: []ast.Expr{&ast.Ident{Name: "i"}, &ast.Literal{Value: "2"}}}))
}
