package preprocess

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSourceMap_OriginalPos(t *testing.T) {
	tests := []struct {
		name        string
		mappings    []struct{ preprocLine int; origFile string; origLine int }
		queryLine   int
		queryCol    int
		wantFile    string
		wantLine    int
		wantCol     int
	}{
		{
			name:      "no mappings returns zero Pos",
			mappings:  nil,
			queryLine: 1,
			queryCol:  5,
			wantFile:  "",
			wantLine:  0,
			wantCol:   0,
		},
		{
			name: "identity mapping",
			mappings: []struct{ preprocLine int; origFile string; origLine int }{
				{1, "test.st", 1},
				{2, "test.st", 2},
				{3, "test.st", 3},
			},
			queryLine: 2,
			queryCol:  10,
			wantFile:  "test.st",
			wantLine:  2,
			wantCol:   10,
		},
		{
			name: "shifted lines due to removed block",
			mappings: []struct{ preprocLine int; origFile string; origLine int }{
				{1, "test.st", 1},
				{2, "test.st", 5}, // lines 2-4 were directive block, removed
				{3, "test.st", 6},
			},
			queryLine: 2,
			queryCol:  3,
			wantFile:  "test.st",
			wantLine:  5,
			wantCol:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := &SourceMap{}
			for _, m := range tt.mappings {
				sm.AddMapping(m.preprocLine, m.origFile, m.origLine)
			}
			pos := sm.OriginalPos(tt.queryLine, tt.queryCol)
			assert.Equal(t, tt.wantFile, pos.File)
			assert.Equal(t, tt.wantLine, pos.Line)
			assert.Equal(t, tt.wantCol, pos.Col)
		})
	}
}

func TestParseDirective(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNil   bool
		wantKind  directiveKind
		wantCond  string
		wantName  string
		wantMsg   string
	}{
		{
			name:     "IF defined",
			input:    "{IF defined(VENDOR_BECKHOFF)}",
			wantKind: dirIF,
			wantCond: "defined(VENDOR_BECKHOFF)",
		},
		{
			name:     "ELSIF defined",
			input:    "{ELSIF defined(VENDOR_SCHNEIDER)}",
			wantKind: dirELSIF,
			wantCond: "defined(VENDOR_SCHNEIDER)",
		},
		{
			name:     "ELSE",
			input:    "{ELSE}",
			wantKind: dirELSE,
		},
		{
			name:     "END_IF",
			input:    "{END_IF}",
			wantKind: dirENDIF,
		},
		{
			name:     "DEFINE",
			input:    "{DEFINE MY_FLAG}",
			wantKind: dirDEFINE,
			wantName: "MY_FLAG",
		},
		{
			name:     "ERROR with message",
			input:    `{ERROR "Unsupported vendor"}`,
			wantKind: dirERROR,
			wantMsg:  "Unsupported vendor",
		},
		{
			name:    "attribute pragma is not a preprocessor directive",
			input:   "{attribute 'qualified_only'}",
			wantNil: true,
		},
		{
			name:    "unknown pragma returns nil",
			input:   "{pragma something}",
			wantNil: true,
		},
		{
			name:     "case insensitive IF",
			input:    "{if defined(X)}",
			wantKind: dirIF,
			wantCond: "defined(X)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := parseDirective(tt.input)
			if tt.wantNil {
				assert.Nil(t, d)
				return
			}
			if assert.NotNil(t, d) {
				assert.Equal(t, tt.wantKind, d.kind)
				if tt.wantCond != "" {
					assert.Equal(t, tt.wantCond, d.condition)
				}
				if tt.wantName != "" {
					assert.Equal(t, tt.wantName, d.name)
				}
				if tt.wantMsg != "" {
					assert.Equal(t, tt.wantMsg, d.message)
				}
			}
		})
	}
}

func TestEvalCondition(t *testing.T) {
	tests := []struct {
		name    string
		cond    string
		defines map[string]bool
		want    bool
	}{
		{
			name:    "defined X when X exists",
			cond:    "defined(X)",
			defines: map[string]bool{"X": true},
			want:    true,
		},
		{
			name:    "defined X when X not exists",
			cond:    "defined(X)",
			defines: map[string]bool{},
			want:    false,
		},
		{
			name:    "NOT defined X when X not exists",
			cond:    "NOT defined(X)",
			defines: map[string]bool{},
			want:    true,
		},
		{
			name:    "NOT defined X when X exists",
			cond:    "NOT defined(X)",
			defines: map[string]bool{"X": true},
			want:    false,
		},
		{
			name:    "AND both true",
			cond:    "defined(X) AND defined(Y)",
			defines: map[string]bool{"X": true, "Y": true},
			want:    true,
		},
		{
			name:    "AND one false",
			cond:    "defined(X) AND defined(Y)",
			defines: map[string]bool{"X": true},
			want:    false,
		},
		{
			name:    "OR one true",
			cond:    "defined(X) OR defined(Y)",
			defines: map[string]bool{"X": true},
			want:    true,
		},
		{
			name:    "OR both false",
			cond:    "defined(X) OR defined(Y)",
			defines: map[string]bool{},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evalCondition(tt.cond, tt.defines)
			assert.Equal(t, tt.want, result)
		})
	}
}
