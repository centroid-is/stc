// Package twincat reads TwinCAT 3 projects (tsproj, xti, plcproj, TcTTO) and
// converts TcPOU, TcGVL, TcDUT and TcIO objects into Structured Text.
//
// The converter is line preserving: in layout mode every CDATA segment is
// written at the XML line and column where its text starts, so parser and
// checker diagnostics point at the real TcPOU file position without any
// source-map remapping.
package twincat

import (
	"time"

	"github.com/centroid-is/stc/pkg/source"
)

// Kind is the kind of a converted TwinCAT object.
type Kind string

// Object kinds.
const (
	KindPOU Kind = "pou"
	KindGVL Kind = "gvl"
	KindDUT Kind = "dut"
	KindITF Kind = "itf"
)

// ResolvedFrom values for LibraryRef.ResolvedFrom.
const (
	FromProject     = "project"
	FromSibling     = "sibling"
	FromLibraryPath = "library_path"
	FromStub        = "stub"
	FromBuiltin     = "builtin"
	FromUnresolved  = "unresolved"
)

// Task is a PLC task with its cycle time, priority and called programs.
type Task struct {
	Name      string        `json:"name"`
	CycleTime time.Duration `json:"-"`
	CycleNs   int64         `json:"cycle_time_ns"`
	Priority  int           `json:"priority"`
	Programs  []string      `json:"programs"`
}

// Source is one converted TwinCAT object.
type Source struct {
	// Path is the absolute object file path; it is also the parse file name.
	Path string `json:"path"`
	// RelPath is the plcproj Compile Include, slash-normalised.
	RelPath string `json:"rel_path"`
	Kind    Kind   `json:"kind"`
	// Name is the XML Name= attribute (the GVL name for GVLs).
	Name string `json:"name"`
	// Library is the owning library name for library sources.
	Library string `json:"library,omitempty"`
	// Text is the converted ST text.
	Text string `json:"-"`
}

// LibraryRef is a plcproj PlaceholderReference and how it was resolved.
type LibraryRef struct {
	Name              string     `json:"name"`
	DefaultResolution string     `json:"default_resolution,omitempty"`
	Namespace         string     `json:"namespace,omitempty"`
	ResolvedFrom      string     `json:"resolved_from"`
	Path              string     `json:"path,omitempty"`
	Pos               source.Pos `json:"pos"`
}

// Model is an imported TwinCAT PLC project.
type Model struct {
	PlcName      string       `json:"plc_name"`
	AmsPort      int          `json:"ams_port"`
	SolutionPath string       `json:"solution_path,omitempty"`
	ProjectPath  string       `json:"project_path"`
	Tasks        []Task       `json:"tasks"`
	Sources      []Source     `json:"sources"`
	Libraries    []LibraryRef `json:"libraries"`
	// LibrarySources are ordered: sibling libraries first, then stubs.
	LibrarySources []Source `json:"-"`
}
