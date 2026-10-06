package analyzer

import (
	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/twincat"
)

// AnalyzeProject parses an imported TwinCAT model and analyses it with the
// model's library sources registered before the user sources. Diagnostics
// are the parse diagnostics followed by the analysis diagnostics; positions
// point at the TwinCAT XML files because the converter preserves layout.
func AnalyzeProject(m *twincat.Model, cfg *project.Config, defines map[string]bool) AnalysisResult {
	user, libs, parseDiags := twincat.ParseModel(m, defines)
	res := Analyze(user, cfg, AnalyzeOpts{LibraryFiles: libs})
	res.Diags = append(parseDiags, res.Diags...)
	return res
}
