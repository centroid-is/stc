package twincat

// Diagnostic codes emitted by the TwinCAT importer.
const (
	// CodeUnresolvedLibrary: a PlaceholderReference resolved nowhere (warning).
	CodeUnresolvedLibrary = "VEND020"
	// CodeUnknownItem: a plcproj Compile item with an unsupported extension (warning).
	CodeUnknownItem = "VEND021"
	// CodeSkipped: a TwinSAFE project, extra PLC project or non-ST implementation was skipped (info).
	CodeSkipped = "VEND022"
	// CodeLayoutOverlap: a CDATA segment could not be placed at its XML line (warning).
	CodeLayoutOverlap = "VEND023"
	// CodeDefaultCycle: no task information; a 10 ms default task is used (warning).
	CodeDefaultCycle = "VEND024"
	// CodeCycleMismatch: tsproj and TcTTO cycle times disagree (warning).
	CodeCycleMismatch = "VEND025"
	// CodeAmbiguousSibling: more than one sibling library candidate (warning).
	CodeAmbiguousSibling = "VEND026"
	// CodeBadXML: a project or object file is not valid XML (error).
	CodeBadXML = "VEND027"
	// CodeCaseMismatch: a plcproj item path differs in case from the file on disk (warning).
	CodeCaseMismatch = "VEND028"
)
