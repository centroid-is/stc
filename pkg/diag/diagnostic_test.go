package diag

import (
	"encoding/json"
	"testing"

	"github.com/centroid-is/stc/pkg/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiagnosticString(t *testing.T) {
	d := Diagnostic{
		Severity: Error,
		Pos:      source.Pos{File: "test.st", Line: 3, Col: 5},
		Message:  "unexpected token",
	}
	assert.Equal(t, "test.st:3:5: error: unexpected token", d.String())
}

func TestDiagnosticStringWarning(t *testing.T) {
	d := Diagnostic{
		Severity: Warning,
		Pos:      source.Pos{File: "main.st", Line: 10, Col: 1},
		Message:  "unused variable",
	}
	assert.Equal(t, "main.st:10:1: warning: unused variable", d.String())
}

func TestSeverityString(t *testing.T) {
	assert.Equal(t, "error", Error.String())
	assert.Equal(t, "warning", Warning.String())
	assert.Equal(t, "info", Info.String())
	assert.Equal(t, "hint", Hint.String())
}

func TestCollector(t *testing.T) {
	c := NewCollector()

	pos1 := source.Pos{File: "test.st", Line: 1, Col: 1}
	pos2 := source.Pos{File: "test.st", Line: 5, Col: 10}
	pos3 := source.Pos{File: "test.st", Line: 8, Col: 3}

	c.Errorf(pos1, "E001", "unexpected token %q", ";")
	c.Warnf(pos2, "W001", "unused variable %q", "x")
	c.Add(Info, pos3, source.Pos{}, "I001", "consider using REAL instead")

	assert.Len(t, c.All(), 3)
	assert.True(t, c.HasErrors())
	assert.Len(t, c.Errors(), 1)
	assert.Equal(t, "E001", c.Errors()[0].Code)
}

func TestCollectorNoErrors(t *testing.T) {
	c := NewCollector()
	pos := source.Pos{File: "test.st", Line: 1, Col: 1}
	c.Warnf(pos, "W001", "some warning")

	assert.False(t, c.HasErrors())
	assert.Empty(t, c.Errors())
	assert.Len(t, c.All(), 1)
}

func TestDiagnosticJSON(t *testing.T) {
	d := Diagnostic{
		Severity: Error,
		Pos:      source.Pos{File: "test.st", Line: 3, Col: 5, Offset: 42},
		Code:     "E001",
		Message:  "unexpected token",
	}

	data, err := json.Marshal(d)
	require.NoError(t, err)

	var m map[string]interface{}
	err = json.Unmarshal(data, &m)
	require.NoError(t, err)

	assert.Equal(t, "error", m["severity"])
	assert.Equal(t, "E001", m["code"])
	assert.Equal(t, "unexpected token", m["message"])

	pos, ok := m["pos"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test.st", pos["file"])
	assert.Equal(t, float64(3), pos["line"])
	assert.Equal(t, float64(5), pos["col"])
}
