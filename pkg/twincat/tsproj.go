package twincat

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
)

// TsprojInfo is what a .tsproj (plus its PLC .xti) contributes to the model.
type TsprojInfo struct {
	PlcName     string
	AmsPort     int
	PlcprojPath string // absolute
	Tasks       []Task // cycle time and priority; programs come from TcTTO
}

type tsprojXML struct {
	Project struct {
		Tasks []struct {
			Priority  int    `xml:"Priority,attr"`
			CycleTime int64  `xml:"CycleTime,attr"` // 100 ns units
			Name      string `xml:"Name"`
		} `xml:"System>Tasks>Task"`
		Plc    []plcProjectRef `xml:"Plc>Project"`
		Safety []plcProjectRef `xml:"Safety>Project"`
	} `xml:"Project"`
}

type plcProjectRef struct {
	File        string `xml:"File,attr"`
	Name        string `xml:"Name,attr"`
	PrjFilePath string `xml:"PrjFilePath,attr"`
	AmsPort     int    `xml:"AmsPort,attr"`
}

type xtiXML struct {
	Project plcProjectRef `xml:"Project"`
}

// ReadTsproj reads a TwinCAT solution project: its tasks and the first PLC
// project, following File= references to _Config/PLC/<File> where
// PrjFilePath is relative to the xti, or the inline form where it is
// relative to the tsproj. TwinSAFE and additional PLC projects are skipped
// with VEND022 info diagnostics.
func ReadTsproj(path string) (*TsprojInfo, []diag.Diagnostic, error) {
	absPath, raw, err := readAbs(path)
	if err != nil {
		return nil, nil, err
	}
	var x tsprojXML
	if err := decodeXML(absPath, raw, &x); err != nil {
		return nil, nil, err
	}
	pos := source.Pos{File: absPath}
	var ds []diag.Diagnostic
	for _, s := range x.Project.Safety {
		ds = append(ds, info(pos, "TwinSAFE project %s skipped", firstNonEmpty(s.File, s.Name, s.PrjFilePath)))
	}
	if len(x.Project.Plc) == 0 {
		return nil, ds, fmt.Errorf("%s: no PLC project in tsproj", absPath)
	}
	for _, extra := range x.Project.Plc[1:] {
		ds = append(ds, info(pos, "additional PLC project %s skipped; only the first is imported", firstNonEmpty(extra.Name, extra.File)))
	}

	ref := x.Project.Plc[0]
	baseDir := filepath.Dir(absPath)
	if ref.File != "" {
		xtiPath := filepath.Join(baseDir, "_Config", "PLC", normPath(ref.File))
		xraw, err := os.ReadFile(xtiPath)
		if err != nil {
			return nil, ds, fmt.Errorf("reading PLC project %s: %w", xtiPath, err)
		}
		var xt xtiXML
		if err := decodeXML(xtiPath, xraw, &xt); err != nil {
			return nil, ds, err
		}
		ref = xt.Project
		baseDir = filepath.Dir(xtiPath)
		if ref.PrjFilePath == "" {
			return nil, ds, fmt.Errorf("%s: PLC project has no PrjFilePath", xtiPath)
		}
	}
	plcproj := filepath.Clean(filepath.Join(baseDir, normPath(ref.PrjFilePath)))
	if _, err := os.Stat(plcproj); err != nil {
		return nil, ds, fmt.Errorf("PLC project file %s: %w", plcproj, err)
	}

	out := &TsprojInfo{PlcName: ref.Name, AmsPort: ref.AmsPort, PlcprojPath: plcproj}
	for _, t := range x.Project.Tasks {
		d := time.Duration(t.CycleTime) * 100 * time.Nanosecond
		out.Tasks = append(out.Tasks, Task{Name: t.Name, CycleTime: d, CycleNs: d.Nanoseconds(), Priority: t.Priority})
	}
	return out, ds, nil
}

func info(pos source.Pos, format string, args ...any) diag.Diagnostic {
	return diag.Diagnostic{Severity: diag.Info, Pos: pos, Code: CodeSkipped, Message: fmt.Sprintf(format, args...)}
}

func warn(pos source.Pos, code, format string, args ...any) diag.Diagnostic {
	return diag.Diagnostic{Severity: diag.Warning, Pos: pos, Code: code, Message: fmt.Sprintf(format, args...)}
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
