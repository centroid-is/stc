package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPosString(t *testing.T) {
	p := Pos{File: "test.st", Line: 10, Col: 5}
	assert.Equal(t, "test.st:10:5", p.String())
}

func TestPosStringDifferentFile(t *testing.T) {
	p := Pos{File: "main.st", Line: 1, Col: 1}
	assert.Equal(t, "main.st:1:1", p.String())
}

func TestSpanFrom(t *testing.T) {
	start := Pos{File: "test.st", Line: 1, Col: 1, Offset: 0}
	end := Pos{File: "test.st", Line: 1, Col: 10, Offset: 9}
	span := SpanFrom(start, end)
	assert.Equal(t, start, span.Start)
	assert.Equal(t, end, span.End)
}

func TestSourceFilePosFromOffset(t *testing.T) {
	content := "PROGRAM Main\n  VAR\n    x : INT;\n  END_VAR\nEND_PROGRAM\n"
	sf := New("test.st", content)

	tests := []struct {
		name   string
		offset int
		line   int
		col    int
	}{
		{"first char", 0, 1, 1},
		{"middle of line 2 (V of VAR)", 15, 2, 3},
		{"start of line 3", 19, 3, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := sf.PosFromOffset(tt.offset)
			assert.Equal(t, tt.line, pos.Line)
			assert.Equal(t, tt.col, pos.Col)
			assert.Equal(t, "test.st", pos.File)
			assert.Equal(t, tt.offset, pos.Offset)
		})
	}
}

func TestSourceFileLineContent(t *testing.T) {
	content := "PROGRAM Main\n  VAR\n    x : INT;\n  END_VAR\nEND_PROGRAM\n"
	sf := New("test.st", content)

	assert.Equal(t, "PROGRAM Main", sf.LineContent(1))
	assert.Equal(t, "  VAR", sf.LineContent(2))
	assert.Equal(t, "    x : INT;", sf.LineContent(3))
	assert.Equal(t, "  END_VAR", sf.LineContent(4))
	assert.Equal(t, "END_PROGRAM", sf.LineContent(5))
	assert.Equal(t, "", sf.LineContent(0))  // out of range
	assert.Equal(t, "", sf.LineContent(99)) // out of range
}
