package interp

import (
	"fmt"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
)

// CodeUndeclaredType is the warning code for a variable whose type is not
// declared anywhere in the project; the variable runs as an auto-stub.
const CodeUndeclaredType = "RUNT001"

// CodeUndeclaredMember is the warning code for a read of a struct member
// the struct does not declare; the read yields a zero value.
const CodeUndeclaredMember = "RUNT002"

// autoStubFB stands in for an instance of an undeclared type, the Phase 14
// auto-stub pattern applied at run time: calls do nothing, members written
// by the program read back, and every other member reads as zero.
type autoStubFB struct {
	vals map[string]Value
}

func newAutoStubFB() *autoStubFB { return &autoStubFB{vals: make(map[string]Value)} }

func (s *autoStubFB) Execute(time.Duration) {}

func (s *autoStubFB) SetInput(name string, v Value) { s.vals[strings.ToUpper(name)] = v }

func (s *autoStubFB) GetOutput(name string) Value { return s.vals[strings.ToUpper(name)] }

func (s *autoStubFB) GetInput(name string) Value { return s.vals[strings.ToUpper(name)] }

// autoStub returns an auto-stub instance for the undeclared type of node
// and records one warning per type name.
func (interp *Interpreter) autoStub(nt *ast.NamedType) Value {
	interp.warn(CodeUndeclaredType, nt, "type %s is not declared; its instances run as zero-output auto-stubs", nt.Name.Name)
	return MakeFBInstanceValue(nt.Name.Name, newAutoStubFB())
}

// warn records a warning once per code and message.
func (interp *Interpreter) warn(code string, node ast.Node, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	key := code + "\x00" + msg
	if interp.warned == nil {
		interp.warned = make(map[string]bool)
	}
	if interp.warned[key] {
		return
	}
	interp.warned[key] = true
	var pos source.Pos
	if node != nil {
		p := node.Span().Start
		pos = source.Pos{File: p.File, Line: p.Line, Col: p.Col, Offset: p.Offset}
	}
	interp.warnings = append(interp.warnings, diag.Diagnostic{Severity: diag.Warning, Pos: pos, Code: code, Message: msg})
}

// Warnings returns the run-time warnings recorded so far: undeclared types
// replaced by auto-stubs and reads of undeclared struct members.
func (interp *Interpreter) Warnings() []diag.Diagnostic {
	return append([]diag.Diagnostic(nil), interp.warnings...)
}
