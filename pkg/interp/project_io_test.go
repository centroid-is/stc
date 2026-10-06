package interp

import (
	"math"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/iomap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const projWildSrc = `
TYPE ST_Card :
STRUCT
	ok  AT %I* : BOOL;
	raw AT %I* : INT;
	cmd AT %Q* : WORD;
	note : INT;
END_STRUCT
END_TYPE
VAR_GLOBAL
	fixedIn AT %IW4 : INT;
	ai      AT %I* : INT;
	ar      AT %I* : REAL;
	ao      AT %Q* : INT := -2;
	claimed AT %I* : INT;
	card    : ST_Card;
	txt     AT %I* : STRING;
	mk      AT %MW2 : INT := 9;
END_VAR
PROGRAM MAIN
VAR
	pIn AT %IB10 : SINT;
	pw  AT %I* : DINT;
	copy : INT;
END_VAR
copy := GVL.ai;
END_PROGRAM
`

func loadWild(t *testing.T) *Project {
	t.Helper()
	p, err := LoadProject(ProjectSpec{Files: []*ast.SourceFile{parseRT(t, "GVL.st", projWildSrc)}})
	require.NoError(t, err)
	return p
}

func TestProjectWildcard(t *testing.T) {
	p := loadWild(t)
	// explicit %IB10 (MAIN) and %IW4 (GVL) end at byte 11: wildcards follow
	addr, n, ok := p.IOSlot("gvl.AI")
	require.True(t, ok)
	assert.Equal(t, iomap.AreaInput, addr.Area)
	assert.Equal(t, 12, addr.ByteOffset, "INT aligned to 2 after byte 11")
	assert.Equal(t, 2, n)
	assert.Equal(t, iomap.SizeWord, addr.Size)

	ar, n, _ := p.IOSlot("GVL.ar")
	assert.Equal(t, 16, ar.ByteOffset)
	assert.Equal(t, 4, n)
	ao, _, _ := p.IOSlot("GVL.ao")
	assert.Equal(t, iomap.AreaOutput, ao.Area)
	assert.Equal(t, 0, ao.ByteOffset)

	// struct members with AT get slots in declaration order
	okS, n, ok := p.IOSlot("GVL.card.ok")
	require.True(t, ok)
	assert.Equal(t, 1, n)
	assert.Equal(t, iomap.SizeBit, okS.Size)
	raw, _, _ := p.IOSlot("GVL.card.raw")
	assert.Greater(t, raw.ByteOffset, okS.ByteOffset)
	_, _, ok = p.IOSlot("GVL.card.note")
	assert.False(t, ok)
	_, _, ok = p.IOSlot("GVL.txt")
	assert.False(t, ok, "STRING cannot be bound")
	_, _, ok = p.IOSlot("MAIN.pIn")
	assert.False(t, ok, "explicit program addresses are synced by the engine")
	pw, _, ok := p.IOSlot("MAIN.pw")
	require.True(t, ok)
	assert.Equal(t, 0, pw.ByteOffset%4)

	// the INT -5 criterion and the other directions
	require.NoError(t, p.SetIOBytes('I', addr.ByteOffset, []byte{0xFB, 0xFF}))
	require.NoError(t, p.SetIOBytes('I', ar.ByteOffset, le(4, uint64(math.Float32bits(0.5)))))
	require.NoError(t, p.SetIOBytes('I', okS.ByteOffset, []byte{1}))
	require.NoError(t, p.SetIOBytes('I', raw.ByteOffset, []byte{0xFE, 0xFF}))
	require.NoError(t, p.SetIOBytes('I', 4, []byte{0x10, 0x00}))
	require.NoError(t, p.SetIOBytes('I', 10, []byte{0xFF}))
	require.NoError(t, p.Tick())
	rt := p.Runtime()
	get := func(path string) Value {
		v, err := rt.Get(path)
		require.NoError(t, err, path)
		return v
	}
	assert.Equal(t, int64(-5), get("GVL.ai").Int)
	assert.Equal(t, int64(-5), get("MAIN.copy").Int)
	assert.Equal(t, 0.5, get("GVL.ar").Real)
	assert.True(t, get("GVL.card.ok").Bool)
	assert.Equal(t, int64(-2), get("GVL.card.raw").Int)
	assert.Equal(t, int64(16), get("GVL.fixedIn").Int)
	assert.Equal(t, int64(-1), get("MAIN.pIn").Int)
	assert.Equal(t, int64(9), get("GVL.mk").Int)
	out, err := p.IOBytes('Q', ao.ByteOffset, 2)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xFE, 0xFF}, out)
	mk, err := p.IOBytes('M', 2, 2)
	require.NoError(t, err)
	assert.Equal(t, []byte{9, 0}, mk)
}

func TestProjectWildcardBinderClaims(t *testing.T) {
	p := loadWild(t)
	_, _, ok := p.IOSlot("GVL.claimed")
	require.True(t, ok)
	bd := ecat.Binding{Var: ecat.LinkedVar{Steps: []string{"GVL", "claimed"}, TypeName: "INT"}}
	p.SetIOBinder(NewIOBinder([]ecat.Binding{bd}, nil))
	_, _, ok = p.IOSlot("GVL.claimed")
	assert.False(t, ok, "a TcLinkTo-bound variable is not double bound")
	_, _, ok = p.IOSlot("GVL.ai")
	assert.True(t, ok)
	require.NoError(t, p.Tick())
}

func TestProjectIOBytesBounds(t *testing.T) {
	p := loadWild(t)
	assert.Error(t, p.SetIOBytes('X', 0, []byte{1}))
	assert.Error(t, p.SetIOBytes('I', -1, []byte{1}))
	assert.Error(t, p.SetIOBytes('Q', 1<<20, []byte{1}))
	_, err := p.IOBytes('X', 0, 1)
	assert.Error(t, err)
	_, err = p.IOBytes('M', 0, -1)
	assert.Error(t, err)
	_, err = p.IOBytes('I', 1<<20, 1)
	assert.Error(t, err)
	_, _, ok := p.IOSlot("GVL.nope")
	assert.False(t, ok)
	assert.Equal(t, iomap.SizeByte, wildcardSize(64))
}

func TestProjectAllocIOEdges(t *testing.T) {
	f := parseRT(t, "GVL.st", projWildSrc)
	for _, d := range f.Declarations {
		if g, ok := d.(*ast.GVLDecl); ok {
			g.Blocks[0].Declarations[1].AtAddress.Name = "%bogus" // ai
		}
	}
	p, err := LoadProject(ProjectSpec{Files: []*ast.SourceFile{f}})
	require.NoError(t, err)
	_, _, ok := p.IOSlot("GVL.ai")
	assert.False(t, ok, "unparsable address is skipped")
	p.files = append(p.files, nil, &ast.SourceFile{Declarations: []ast.Declaration{
		&ast.GVLDecl{}, &ast.ProgramDecl{}, &ast.ProgramDecl{Name: &ast.Ident{Name: "NoSuch"}},
		&ast.GVLDecl{Name: &ast.Ident{Name: "NoSuchGVL"}, Blocks: f.Declarations[1].(*ast.GVLDecl).Blocks},
	}})
	p.SetIOBinder(nil)
	_, _, ok = p.IOSlot("GVL.ar")
	assert.True(t, ok)
}
