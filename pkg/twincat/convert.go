package twincat

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/lexer"
	"github.com/centroid-is/stc/pkg/source"
)

// Mode selects the converter renderer.
type Mode int

const (
	// ModeLayout writes every segment at its XML line and column, so ST
	// positions equal TcPOU positions. Used for checking imported projects.
	ModeLayout Mode = iota
	// ModeCompact writes segments sequentially under a "// source:" header.
	// Used for files written to disk with --out.
	ModeCompact
	// ModeDecl writes declarations only: POU and method headers with VAR
	// blocks, properties with empty GET/SET, no bodies and no actions.
	// Used by `stc vendor extract`.
	ModeDecl
)

// Converted is the ST rendering of one TwinCAT object.
type Converted struct {
	Kind Kind
	Name string
	Text string
}

type pieceKind int

const (
	pieceDecl pieceKind = iota
	pieceBody
	pieceKeyword
)

// piece is one segment of output text anchored at an XML position.
type piece struct {
	line, col int
	text      string
	kind      pieceKind
	inAction  bool // inside <Action>
	inAccess  bool // declaration or body of a property <Get>/<Set>
}

var rootKinds = map[string]Kind{"POU": KindPOU, "GVL": KindGVL, "DUT": KindDUT, "Itf": KindITF}

// ConvertFile reads and converts a TcPOU, TcGVL, TcDUT or TcIO file.
// path is used as the diagnostic file name; relPath is the plcproj
// Compile Include used in the compact header.
func ConvertFile(path, relPath string, mode Mode) (Converted, []diag.Diagnostic, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Converted{}, nil, err
	}
	return Convert(path, relPath, raw, mode)
}

// Convert converts raw TwinCAT object XML into ST text.
func Convert(path, relPath string, raw []byte, mode Mode) (Converted, []diag.Diagnostic, error) {
	var (
		out    Converted
		ds     []diag.Diagnostic
		pieces []piece
		stack  []string
		decl   strings.Builder // root declaration text, for the END keyword
	)
	has := func(name string) bool {
		for _, s := range stack {
			if s == name {
				return true
			}
		}
		return false
	}
	posAt := func(off int64) (int, int, source.Pos) {
		line, col := lineCol(raw, off)
		return line, col, source.Pos{File: path, Line: line, Col: col}
	}
	keyword := func(off int64, text string) {
		line, col, _ := posAt(off)
		pieces = append(pieces, piece{line: line, col: col, text: text, kind: pieceKeyword,
			inAction: has("Action"), inAccess: has("Get") || has("Set")})
	}

	d := xml.NewDecoder(bytes.NewReader(raw))
	for {
		off := d.InputOffset()
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Converted{}, nil, &BadXMLError{Path: path, Err: err}
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			name := tk.Name.Local
			if len(stack) == 1 {
				kind, ok := rootKinds[name]
				if !ok {
					return Converted{}, nil, fmt.Errorf("%s: unsupported TwinCAT object <%s>", path, name)
				}
				out.Kind = kind
				out.Name = attr(tk, "Name")
			}
			parent := ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, name)
			switch {
			case name == "Action":
				keyword(off, "ACTION "+attr(tk, "Name")+":")
			case name == "Get" && parent == "Property":
				keyword(off, "GET")
			case name == "Set" && parent == "Property":
				keyword(off, "SET")
			case parent == "Implementation" && name != "ST":
				_, _, pos := posAt(off)
				ds = append(ds, info(pos, "non-ST implementation (%s) in %s skipped; declaration kept", name, out.Name))
			}
		case xml.EndElement:
			// Keywords are recorded before popping so they keep the
			// action/accessor context of the element they close.
			name := tk.Name.Local
			depth := len(stack) - 1 // depth of the closing element
			parent := ""
			if depth > 0 {
				parent = stack[depth-1]
			}
			switch {
			case name == "Action":
				keyword(off, "END_ACTION")
			case name == "Method":
				keyword(off, "END_METHOD")
			case name == "Get" && parent == "Property":
				keyword(off, "END_GET")
			case name == "Set" && parent == "Property":
				keyword(off, "END_SET")
			case name == "Property":
				keyword(off, "END_PROPERTY")
			case name == "POU" && depth == 1:
				if end := pouEndKeyword(path, decl.String()); end != "" {
					keyword(off, end)
				}
			case name == "Itf" && depth == 1:
				keyword(off, "END_INTERFACE")
			}
			stack = stack[:depth]
		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			parent := stack[len(stack)-1]
			if parent != "Declaration" && parent != "ST" {
				continue
			}
			text := string(tk)
			if strings.TrimSpace(text) == "" {
				continue
			}
			line, col := lineCol(raw, off)
			if bytes.HasPrefix(raw[off:], []byte("<![CDATA[")) {
				col += len("<![CDATA[")
			}
			kind := pieceDecl
			if parent == "ST" {
				kind = pieceBody
			}
			if kind == pieceDecl && len(stack) == 3 { // TcPlcObject > POU > Declaration
				decl.WriteString(text)
			}
			pieces = append(pieces, piece{line: line, col: col, text: text, kind: kind,
				inAction: has("Action"), inAccess: has("Get") || has("Set")})
		}
	}
	if out.Kind == "" {
		return Converted{}, nil, fmt.Errorf("%s: no POU, GVL, DUT or Itf object found", path)
	}

	switch mode {
	case ModeLayout:
		var l layout
		for _, p := range pieces {
			if !l.put(p.line, p.col, p.text) {
				ds = append(ds, warn(source.Pos{File: path, Line: p.line, Col: p.col}, CodeLayoutOverlap,
					"text at line %d overlaps earlier text; placed on the next free line, positions below are shifted", p.line))
			}
		}
		out.Text = l.String()
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "// source: %s\n", relPath)
		for _, p := range pieces {
			if mode == ModeDecl && (p.kind == pieceBody || p.inAction || (p.inAccess && p.kind == pieceDecl)) {
				continue
			}
			t := strings.TrimRight(strings.TrimLeft(p.text, "\n"), " \t\n")
			b.WriteString(t)
			b.WriteString("\n")
		}
		out.Text = b.String()
	}
	return out, ds, nil
}

func attr(se xml.StartElement, name string) string {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// pouEndKeyword returns the END keyword for the first POU keyword in a
// declaration, skipping whitespace, comments and pragmas. It returns ""
// when the declaration does not start a PROGRAM, FUNCTION_BLOCK,
// FUNCTION or INTERFACE.
func pouEndKeyword(path, decl string) string {
	for _, tk := range lexer.Tokenize(path, decl) {
		switch tk.Kind {
		case lexer.Whitespace, lexer.LineComment, lexer.BlockComment, lexer.Pragma:
			continue
		case lexer.KwProgram:
			return "END_PROGRAM"
		case lexer.KwFunctionBlock:
			return "END_FUNCTION_BLOCK"
		case lexer.KwFunction:
			return "END_FUNCTION"
		case lexer.KwInterface:
			return "END_INTERFACE"
		}
		return ""
	}
	return ""
}

// layout is a line buffer that writes text at fixed line/column positions.
// Lines are bounded by the positions it is given, which come from the raw
// file, so its size never exceeds the source line count plus fallbacks.
type layout struct {
	lines []string
}

// put writes text starting at line/col (1-based). If that position lies
// before or inside text already written, the text goes to the next free
// line at column 1 and put returns false.
func (l *layout) put(line, col int, text string) bool {
	ok := true
	n := len(l.lines)
	if line < n || (line == n && col <= len(l.lines[n-1])) {
		line, col, ok = n+1, 1, false
	}
	frags := strings.Split(text, "\n")
	for len(l.lines) < line-1 {
		l.lines = append(l.lines, "")
	}
	if line == len(l.lines) {
		cur := l.lines[line-1]
		l.lines[line-1] = cur + strings.Repeat(" ", col-1-len(cur)) + frags[0]
	} else {
		l.lines = append(l.lines, strings.Repeat(" ", col-1)+frags[0])
	}
	l.lines = append(l.lines, frags[1:]...)
	return ok
}

// String returns the buffer with a trailing newline.
func (l *layout) String() string {
	return strings.Join(l.lines, "\n") + "\n"
}
