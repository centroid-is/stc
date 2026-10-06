package twincat

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
)

// KindTask marks a TcTTO task object in PlcprojInfo.Items.
const KindTask Kind = "task"

// Item is a supported plcproj Compile item.
type Item struct {
	RelPath string // Compile Include, slash-normalised
	AbsPath string
	Ext     string // lower-case extension including the dot
	Kind    Kind   // by extension; the converter decides the final kind from the XML root
	Line    int    // plcproj line of the <Compile> start tag
}

// PlaceholderRef is a plcproj <PlaceholderReference>.
type PlaceholderRef struct {
	Name              string
	DefaultResolution string
	Namespace         string
	Pos               source.Pos // position of the start tag in the plcproj
}

// PlcprojInfo is the content of a .plcproj relevant to import.
type PlcprojInfo struct {
	Path  string // absolute
	Items []Item
	Refs  []PlaceholderRef
}

var itemKinds = map[string]Kind{
	".tcpou": KindPOU,
	".tcgvl": KindGVL,
	".tcdut": KindDUT,
	".tcio":  KindITF,
	".tctto": KindTask,
}

// ReadPlcproj reads Compile items (file order) and PlaceholderReferences
// with positions. Unsupported Compile extensions produce VEND021 warnings.
func ReadPlcproj(path string) (*PlcprojInfo, []diag.Diagnostic, error) {
	absPath, raw, err := readAbs(path)
	if err != nil {
		return nil, nil, err
	}
	out := &PlcprojInfo{Path: absPath}
	var ds []diag.Diagnostic
	dir := filepath.Dir(absPath)
	d := xml.NewDecoder(bytes.NewReader(raw))
	for {
		off := d.InputOffset()
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, &BadXMLError{Path: absPath, Err: err}
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		line, col := lineCol(raw, off)
		pos := source.Pos{File: absPath, Line: line, Col: col}
		switch se.Name.Local {
		case "Compile":
			var c struct {
				Include string `xml:"Include,attr"`
			}
			if err := d.DecodeElement(&c, &se); err != nil {
				return nil, nil, &BadXMLError{Path: absPath, Err: err}
			}
			rel := slashPath(c.Include)
			ext := strings.ToLower(filepath.Ext(rel))
			kind, known := itemKinds[ext]
			if !known {
				ds = append(ds, warn(pos, CodeUnknownItem, "unsupported plcproj item %s skipped", c.Include))
				continue
			}
			out.Items = append(out.Items, Item{RelPath: rel, AbsPath: filepath.Join(dir, filepath.FromSlash(rel)), Ext: ext, Kind: kind, Line: line})
		case "PlaceholderReference":
			var p struct {
				Include           string `xml:"Include,attr"`
				DefaultResolution string `xml:"DefaultResolution"`
				Namespace         string `xml:"Namespace"`
			}
			if err := d.DecodeElement(&p, &se); err != nil {
				return nil, nil, &BadXMLError{Path: absPath, Err: err}
			}
			out.Refs = append(out.Refs, PlaceholderRef{Name: p.Include, DefaultResolution: p.DefaultResolution, Namespace: p.Namespace, Pos: pos})
		}
	}
	return out, ds, nil
}

type tcttoXML struct {
	Task []struct {
		Name      string   `xml:"Name,attr"`
		CycleTime int64    `xml:"CycleTime"` // microseconds
		Priority  int      `xml:"Priority"`
		PouCalls  []string `xml:"PouCall>Name"`
	} `xml:"Task"`
}

// ReadTcTTO reads a TcTTO task object: name, cycle time (µs), priority and
// the programs it calls.
func ReadTcTTO(path string) (Task, error) {
	absPath, raw, err := readAbs(path)
	if err != nil {
		return Task{}, err
	}
	var x tcttoXML
	if err := decodeXML(absPath, raw, &x); err != nil {
		return Task{}, err
	}
	if len(x.Task) == 0 {
		return Task{}, fmt.Errorf("%s: no Task element", absPath)
	}
	t := x.Task[0]
	cycle := time.Duration(t.CycleTime) * time.Microsecond
	return Task{Name: t.Name, CycleTime: cycle, CycleNs: cycle.Nanoseconds(), Priority: t.Priority, Programs: t.PouCalls}, nil
}

// ttoFile is a TcTTO task with the file it came from.
type ttoFile struct {
	Path string
	Task Task
}

// defaultCycle is used for a bare plcproj without a TcTTO task.
const defaultCycle = 10 * time.Millisecond

// mergeTasks combines tsproj tasks (authoritative cycle and priority) with
// TcTTO tasks (program lists), matched by name case-insensitively. Without
// tsproj tasks the TcTTO tasks are used as-is; with neither, one 10 ms
// PlcTask is returned with a VEND024 warning at projectPath.
func mergeTasks(ts []Task, ttos []ttoFile, projectPath string) ([]Task, []diag.Diagnostic) {
	var ds []diag.Diagnostic
	if len(ts) == 0 {
		if len(ttos) == 0 {
			ds = append(ds, warn(source.Pos{File: projectPath}, CodeDefaultCycle,
				"no task configuration found; using a 10 ms PlcTask"))
			return []Task{{Name: "PlcTask", CycleTime: defaultCycle, CycleNs: defaultCycle.Nanoseconds()}}, ds
		}
		out := make([]Task, 0, len(ttos))
		for _, f := range ttos {
			out = append(out, f.Task)
		}
		return out, ds
	}
	out := make([]Task, 0, len(ts))
	for _, t := range ts {
		for _, f := range ttos {
			if !strings.EqualFold(f.Task.Name, t.Name) {
				continue
			}
			t.Programs = append([]string(nil), f.Task.Programs...)
			if f.Task.CycleTime != t.CycleTime {
				ds = append(ds, warn(source.Pos{File: f.Path}, CodeCycleMismatch,
					"task %s cycle time %v in TcTTO differs from %v in tsproj; using the tsproj value",
					t.Name, f.Task.CycleTime, t.CycleTime))
			}
			break
		}
		out = append(out, t)
	}
	return out, ds
}
