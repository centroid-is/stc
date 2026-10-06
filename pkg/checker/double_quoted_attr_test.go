package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sema039(ds []diag.Diagnostic) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range ds {
		if d.Code == CodeAttrDoubleQuoted {
			out = append(out, d)
		}
	}
	return out
}

func TestDoubleQuotedAttribute(t *testing.T) {
	const msg = "attribute name in double quotes is ignored by TwinCAT; use single quotes"

	t.Run("double quoted qualified_only is ignored with one warning", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{
			{"G.st", "{attribute \"qualified_only\"}\nVAR_GLOBAL\n\tx : INT;\nEND_VAR\n"},
			progUsing("x := 1; b := b;"),
		})
		assert.Empty(t, errorsOf(ds))
		w := sema039(ds)
		require.Len(t, w, 1)
		assert.Equal(t, diag.Warning, w[0].Severity)
		assert.Equal(t, msg, w[0].Message)
		assert.Equal(t, "G.st", w[0].Pos.File)
		assert.Equal(t, 1, w[0].Pos.Line)
		assert.Equal(t, 1, w[0].Pos.Col)
	})

	t.Run("single quoted qualified_only still applies", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{{"G.st", gvlGQualified}, progUsing("x := 1; b := b;")})
		errs := errorsOf(ds)
		require.Len(t, errs, 1)
		assert.Equal(t, CodeGVLQualifiedOnly, errs[0].Code)
		assert.Empty(t, sema039(ds))
	})

	t.Run("double quoted strict and to_string on an enum are ignored", func(t *testing.T) {
		ds, table := runGVL(t, []gvlFile{
			{"types.st", "{attribute \"strict\"}\n{attribute \"to_string\"}\nTYPE E_D : (da, db); END_TYPE\n"},
			{"main.st", "PROGRAM P\nVAR\n\te : E_D;\n\tn : INT;\nEND_VAR\nCASE e OF 0: n := 1; END_CASE\nn := n;\nEND_PROGRAM\n"},
		})
		assert.Empty(t, errorsOf(ds))
		assert.Len(t, sema039(ds), 2)
		et := enumOf(t, table, "E_D")
		assert.False(t, et.Strict)
		assert.False(t, et.ToString)
	})

	t.Run("attributes on members and nested declarations warn once each", func(t *testing.T) {
		src := "{attribute \"a\"}\nFUNCTION_BLOCK FB_A\n{attribute \"b\"}\nVAR\n\t{attribute \"c\"}\n\tx : INT;\nEND_VAR\n" +
			"x := x;\n{attribute \"d\"}\nMETHOD M\n;\nEND_METHOD\n{attribute \"e\"}\nPROPERTY P : INT\nGET P := x; END_GET\nEND_PROPERTY\n" +
			"END_FUNCTION_BLOCK\n{attribute 'ok'}\nTYPE S : STRUCT\n\t{attribute \"f\"}\n\tm : INT;\nEND_STRUCT\nEND_TYPE\n"
		ds, _ := runGVL(t, []gvlFile{{"fb.st", src}})
		assert.Empty(t, errorsOf(ds))
		assert.Len(t, sema039(ds), 6, "%v", ds)
	})

	t.Run("library files never warn", func(t *testing.T) {
		lib := parseGVLFiles(t, []gvlFile{{"lib.st", "{attribute \"qualified_only\"}\nVAR_GLOBAL\n\tlx : INT;\nEND_VAR\n"}})
		ds, _ := runGVL(t, []gvlFile{progUsing("lx := 1; b := b;")}, ResolveOpts{LibraryFiles: lib})
		assert.Empty(t, errorsOf(ds))
		assert.Empty(t, sema039(ds))
	})
}
