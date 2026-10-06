package vendor

import (
	"errors"
	"fmt"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/twincat"
)

// ExtractedStub is one TwinCAT object rendered as a declaration-only stub.
type ExtractedStub struct {
	Name    string       `json:"name"`
	RelPath string       `json:"rel_path"`
	Kind    twincat.Kind `json:"kind"`
	Text    string       `json:"text"`
}

// ExtractProject reads a .plcproj and renders every POU, GVL, DUT and
// interface it lists as a declaration-only stub (methods and properties
// keep their signatures, bodies are dropped), in plcproj order. Items that
// cannot be converted and unsupported items are reported as diagnostics;
// nothing is skipped silently. An error is returned only when the plcproj
// itself cannot be read.
func ExtractProject(plcprojPath string) ([]ExtractedStub, []diag.Diagnostic, error) {
	info, ds, err := twincat.ReadPlcproj(plcprojPath)
	if err != nil {
		return nil, ds, fmt.Errorf("reading project file: %w", err)
	}
	var stubs []ExtractedStub
	for _, it := range info.Items {
		switch it.Kind {
		case twincat.KindPOU, twincat.KindGVL, twincat.KindDUT, twincat.KindITF:
		default:
			continue
		}
		c, cds, err := twincat.ConvertFile(it.AbsPath, it.RelPath, twincat.ModeDecl)
		ds = append(ds, cds...)
		if err != nil {
			msg := err.Error()
			var bad *twincat.BadXMLError
			if !errors.As(err, &bad) {
				msg = "cannot read " + it.AbsPath + ": " + msg
			}
			ds = append(ds, diag.Diagnostic{Severity: diag.Error, Pos: source.Pos{File: it.AbsPath}, Code: twincat.CodeBadXML, Message: msg})
			continue
		}
		stubs = append(stubs, ExtractedStub{Name: c.Name, RelPath: it.RelPath, Kind: c.Kind, Text: c.Text})
	}
	return stubs, ds, nil
}
