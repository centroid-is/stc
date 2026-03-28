package lsp

import (
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// convertDiagnostics maps stc diagnostics to LSP protocol diagnostics.
// stc uses 1-based line/col; LSP uses 0-based line/character.
func convertDiagnostics(diags []diag.Diagnostic) []protocol.Diagnostic {
	result := make([]protocol.Diagnostic, 0, len(diags))
	source := "stc"

	for _, d := range diags {
		severity := convertSeverity(d.Severity)

		startLine := d.Pos.Line - 1
		if startLine < 0 {
			startLine = 0
		}
		startChar := d.Pos.Col - 1
		if startChar < 0 {
			startChar = 0
		}

		endLine := d.EndPos.Line - 1
		if endLine < 0 {
			endLine = startLine
		}
		endChar := d.EndPos.Col - 1
		if endChar < 0 {
			endChar = startChar
		}

		// If end position is not set, use start position
		if d.EndPos.Line == 0 && d.EndPos.Col == 0 {
			endLine = startLine
			endChar = startChar
		}

		var code *protocol.IntegerOrString
		if d.Code != "" {
			code = &protocol.IntegerOrString{Value: d.Code}
		}

		result = append(result, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(startLine),
					Character: uint32(startChar),
				},
				End: protocol.Position{
					Line:      uint32(endLine),
					Character: uint32(endChar),
				},
			},
			Severity: &severity,
			Code:     code,
			Source:   &source,
			Message:  d.Message,
		})
	}

	return result
}

// convertSeverity maps stc severity to LSP DiagnosticSeverity.
func convertSeverity(s diag.Severity) protocol.DiagnosticSeverity {
	switch s {
	case diag.Error:
		return protocol.DiagnosticSeverityError
	case diag.Warning:
		return protocol.DiagnosticSeverityWarning
	case diag.Info:
		return protocol.DiagnosticSeverityInformation
	case diag.Hint:
		return protocol.DiagnosticSeverityHint
	default:
		return protocol.DiagnosticSeverityInformation
	}
}

// publishDiagnostics sends diagnostics for a document to the client.
// It combines parse and analysis diagnostics.
func publishDiagnostics(ctx *glsp.Context, uri string, doc *Document) {
	var allDiags []diag.Diagnostic

	if doc.ParseResult != nil {
		allDiags = append(allDiags, doc.ParseResult.Diags...)
	}
	if doc.AnalysisResult != nil {
		allDiags = append(allDiags, doc.AnalysisResult.Diags...)
	}

	protocolDiags := convertDiagnostics(allDiags)

	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: protocolDiags,
	})
}
