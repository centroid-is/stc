// Package preprocess implements IEC 61131-3 conditional compilation
// preprocessing with source map generation for position remapping.
package preprocess

import "github.com/centroid-is/stc/pkg/source"

// Mapping records that a given preprocessed output line originated from
// a specific line in the original source file.
type Mapping struct {
	PreprocLine int    // 1-based line in preprocessed output
	OrigFile    string // original source filename
	OrigLine    int    // 1-based line in original source
}

// SourceMap tracks the correspondence between preprocessed output positions
// and original source positions. It is built during preprocessing and used
// by downstream tools (parser, checker) to remap diagnostics.
type SourceMap struct {
	mappings []Mapping
}

// AddMapping records that preprocessed output line preprocLine corresponds
// to original source line origLine in file origFile.
func (sm *SourceMap) AddMapping(preprocLine int, origFile string, origLine int) {
	sm.mappings = append(sm.mappings, Mapping{
		PreprocLine: preprocLine,
		OrigFile:    origFile,
		OrigLine:    origLine,
	})
}

// OriginalPos translates a position in the preprocessed output back to
// the corresponding position in the original source file. If no mapping
// exists for the given line, a zero-value Pos is returned.
func (sm *SourceMap) OriginalPos(preprocLine, preprocCol int) source.Pos {
	for _, m := range sm.mappings {
		if m.PreprocLine == preprocLine {
			return source.Pos{
				File: m.OrigFile,
				Line: m.OrigLine,
				Col:  preprocCol,
			}
		}
	}
	return source.Pos{}
}

// Mappings returns a copy of all mappings in the source map.
func (sm *SourceMap) Mappings() []Mapping {
	out := make([]Mapping, len(sm.mappings))
	copy(out, sm.mappings)
	return out
}

// Len returns the number of mappings in the source map.
func (sm *SourceMap) Len() int {
	return len(sm.mappings)
}
