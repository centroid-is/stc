// Package emit produces Structured Text source code from an AST,
// with vendor-specific transformations for Beckhoff, Schneider, and
// Portable targets.
package emit

import (
	"fmt"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
)

// Emit produces Structured Text source code from a parsed AST SourceFile.
// The output respects the given Options for vendor targeting, indentation,
// and keyword casing.
func Emit(file *ast.SourceFile, opts Options) string {
	if file == nil {
		return ""
	}
	if opts.Indent == "" {
		opts.Indent = "    "
	}
	e := &emitter{
		opts: opts,
	}
	e.emitSourceFile(file)
	return e.buf.String()
}

// emitter is the internal state machine for ST emission.
type emitter struct {
	buf    strings.Builder
	indent int
	opts   Options
}

// --- helpers ---

func (e *emitter) write(s string) {
	e.buf.WriteString(s)
}

func (e *emitter) writef(format string, args ...any) {
	fmt.Fprintf(&e.buf, format, args...)
}

func (e *emitter) newline() {
	e.buf.WriteByte('\n')
}

func (e *emitter) emitIndent() {
	for i := 0; i < e.indent; i++ {
		e.write(e.opts.Indent)
	}
}

func (e *emitter) kw(keyword string) string {
	if e.opts.UppercaseKeywords {
		return strings.ToUpper(keyword)
	}
	return strings.ToLower(keyword)
}

// --- trivia ---

func (e *emitter) emitTrivia(trivia []ast.Trivia) {
	for _, t := range trivia {
		e.write(t.Text)
	}
}

func (e *emitter) emitLeadingTrivia(n *ast.NodeBase) {
	e.emitTrivia(n.LeadingTrivia)
}

func (e *emitter) emitTrailingTrivia(n *ast.NodeBase) {
	e.emitTrivia(n.TrailingTrivia)
}

// --- source file ---

func (e *emitter) emitSourceFile(file *ast.SourceFile) {
	for i, decl := range file.Declarations {
		if i > 0 {
			e.newline()
		}
		e.emitDecl(decl)
	}
}

// --- declarations ---

func (e *emitter) emitDecl(decl ast.Declaration) {
	switch d := decl.(type) {
	case *ast.ProgramDecl:
		e.emitProgramDecl(d)
	case *ast.FunctionBlockDecl:
		e.emitFunctionBlockDecl(d)
	case *ast.FunctionDecl:
		e.emitFunctionDecl(d)
	case *ast.InterfaceDecl:
		e.emitInterfaceDecl(d)
	case *ast.MethodDecl:
		e.emitMethodDecl(d)
	case *ast.PropertyDecl:
		e.emitPropertyDecl(d)
	case *ast.TypeDecl:
		e.emitTypeDecl(d)
	case *ast.ActionDecl:
		e.emitActionDecl(d)
	case *ast.TestCaseDecl:
		e.emitTestCaseDecl(d)
	case *ast.ErrorNode:
		// skip error nodes
	}
}

func (e *emitter) emitProgramDecl(d *ast.ProgramDecl) {
	e.emitLeadingTrivia(&d.NodeBase)
	e.writef("%s %s", e.kw("PROGRAM"), d.Name.Name)
	e.newline()
	for _, vb := range d.VarBlocks {
		e.emitVarBlock(vb)
	}
	for _, s := range d.Body {
		e.emitIndentedStmt(s)
	}
	e.write(e.kw("END_PROGRAM"))
	e.newline()
	e.emitTrailingTrivia(&d.NodeBase)
}

func (e *emitter) emitFunctionBlockDecl(d *ast.FunctionBlockDecl) {
	e.emitLeadingTrivia(&d.NodeBase)
	e.writef("%s %s", e.kw("FUNCTION_BLOCK"), d.Name.Name)
	if d.Extends != nil {
		e.writef(" %s %s", e.kw("EXTENDS"), d.Extends.Name)
	}
	if len(d.Implements) > 0 {
		e.writef(" %s ", e.kw("IMPLEMENTS"))
		for i, impl := range d.Implements {
			if i > 0 {
				e.write(", ")
			}
			e.write(impl.Name)
		}
	}
	e.newline()
	for _, vb := range d.VarBlocks {
		e.emitVarBlock(vb)
	}
	for _, s := range d.Body {
		e.emitIndentedStmt(s)
	}

	// Methods
	if e.opts.Target.supportsOOP() {
		for _, m := range d.Methods {
			e.newline()
			e.emitMethodDecl(m)
		}
		for _, p := range d.Properties {
			e.newline()
			e.emitPropertyDecl(p)
		}
	}

	e.newline()
	e.write(e.kw("END_FUNCTION_BLOCK"))
	e.newline()
	e.emitTrailingTrivia(&d.NodeBase)
}

func (e *emitter) emitFunctionDecl(d *ast.FunctionDecl) {
	e.emitLeadingTrivia(&d.NodeBase)
	e.writef("%s %s", e.kw("FUNCTION"), d.Name.Name)
	if d.ReturnType != nil {
		e.write(" : ")
		e.emitTypeSpec(d.ReturnType)
	}
	e.newline()
	for _, vb := range d.VarBlocks {
		e.emitVarBlock(vb)
	}
	for _, s := range d.Body {
		e.emitIndentedStmt(s)
	}
	e.write(e.kw("END_FUNCTION"))
	e.newline()
	e.emitTrailingTrivia(&d.NodeBase)
}

func (e *emitter) emitInterfaceDecl(d *ast.InterfaceDecl) {
	if !e.opts.Target.supportsOOP() {
		return
	}
	e.emitLeadingTrivia(&d.NodeBase)
	e.writef("%s %s", e.kw("INTERFACE"), d.Name.Name)
	if len(d.Extends) > 0 {
		e.writef(" %s ", e.kw("EXTENDS"))
		for i, ext := range d.Extends {
			if i > 0 {
				e.write(", ")
			}
			e.write(ext.Name)
		}
	}
	e.newline()
	for _, m := range d.Methods {
		e.emitMethodSignature(m)
	}
	for _, p := range d.Properties {
		e.emitPropertySignature(p)
	}
	e.write(e.kw("END_INTERFACE"))
	e.newline()
	e.emitTrailingTrivia(&d.NodeBase)
}

func (e *emitter) emitMethodDecl(d *ast.MethodDecl) {
	if !e.opts.Target.supportsOOP() {
		return
	}
	e.emitLeadingTrivia(&d.NodeBase)
	e.write(e.kw("METHOD"))
	if d.AccessModifier != ast.AccessNone {
		e.writef(" %s", d.AccessModifier.String())
	}
	if d.IsAbstract {
		e.writef(" %s", e.kw("ABSTRACT"))
	}
	if d.IsFinal {
		e.writef(" %s", e.kw("FINAL"))
	}
	if d.IsOverride {
		e.writef(" %s", e.kw("OVERRIDE"))
	}
	e.writef(" %s", d.Name.Name)
	if d.ReturnType != nil {
		e.write(" : ")
		e.emitTypeSpec(d.ReturnType)
	}
	e.newline()
	for _, vb := range d.VarBlocks {
		e.emitVarBlock(vb)
	}
	for _, s := range d.Body {
		e.emitIndentedStmt(s)
	}
	e.write(e.kw("END_METHOD"))
	e.newline()
	e.emitTrailingTrivia(&d.NodeBase)
}

func (e *emitter) emitPropertyDecl(d *ast.PropertyDecl) {
	if !e.opts.Target.supportsOOP() {
		return
	}
	e.emitLeadingTrivia(&d.NodeBase)
	e.write(e.kw("PROPERTY"))
	if d.AccessModifier != ast.AccessNone {
		e.writef(" %s", d.AccessModifier.String())
	}
	e.writef(" %s", d.Name.Name)
	if d.Type != nil {
		e.write(" : ")
		e.emitTypeSpec(d.Type)
	}
	e.newline()
	if d.Getter != nil {
		e.emitMethodDecl(d.Getter)
	}
	if d.Setter != nil {
		e.emitMethodDecl(d.Setter)
	}
	e.write(e.kw("END_PROPERTY"))
	e.newline()
	e.emitTrailingTrivia(&d.NodeBase)
}

func (e *emitter) emitMethodSignature(d *ast.MethodSignature) {
	e.emitLeadingTrivia(&d.NodeBase)
	e.indent++
	e.emitIndent()
	e.writef("%s %s", e.kw("METHOD"), d.Name.Name)
	if d.ReturnType != nil {
		e.write(" : ")
		e.emitTypeSpec(d.ReturnType)
	}
	e.write(";")
	e.newline()
	e.indent--
	e.emitTrailingTrivia(&d.NodeBase)
}

func (e *emitter) emitPropertySignature(d *ast.PropertySignature) {
	e.emitLeadingTrivia(&d.NodeBase)
	e.indent++
	e.emitIndent()
	e.writef("%s %s", e.kw("PROPERTY"), d.Name.Name)
	if d.Type != nil {
		e.write(" : ")
		e.emitTypeSpec(d.Type)
	}
	e.write(";")
	e.newline()
	e.indent--
	e.emitTrailingTrivia(&d.NodeBase)
}

func (e *emitter) emitTypeDecl(d *ast.TypeDecl) {
	e.emitLeadingTrivia(&d.NodeBase)
	e.writef("%s %s :", e.kw("TYPE"), d.Name.Name)
	e.newline()
	e.emitTypeBody(d.Type)
	e.write(e.kw("END_TYPE"))
	e.newline()
	e.emitTrailingTrivia(&d.NodeBase)
}

func (e *emitter) emitTypeBody(ts ast.TypeSpec) {
	switch t := ts.(type) {
	case *ast.StructType:
		e.write(e.kw("STRUCT"))
		e.newline()
		e.indent++
		for _, m := range t.Members {
			e.emitIndent()
			e.write(m.Name.Name)
			e.write(" : ")
			e.emitTypeSpec(m.Type)
			if m.InitValue != nil {
				e.write(" := ")
				e.emitExpr(m.InitValue)
			}
			e.write(";")
			e.newline()
		}
		e.indent--
		e.write(e.kw("END_STRUCT"))
		e.newline()
	case *ast.EnumType:
		e.write("(")
		e.newline()
		e.indent++
		for i, v := range t.Values {
			e.emitIndent()
			e.write(v.Name.Name)
			if v.Value != nil {
				e.write(" := ")
				e.emitExpr(v.Value)
			}
			if i < len(t.Values)-1 {
				e.write(",")
			}
			e.newline()
		}
		e.indent--
		e.write(");")
		e.newline()
	default:
		// Inline type (e.g., ARRAY[0..9] OF INT)
		e.emitTypeSpec(ts)
		e.write(";")
		e.newline()
	}
}

func (e *emitter) emitActionDecl(d *ast.ActionDecl) {
	e.emitLeadingTrivia(&d.NodeBase)
	e.writef("%s %s:", e.kw("ACTION"), d.Name.Name)
	e.newline()
	e.indent++
	for _, s := range d.Body {
		e.emitIndentedStmt(s)
	}
	e.indent--
	e.write(e.kw("END_ACTION"))
	e.newline()
	e.emitTrailingTrivia(&d.NodeBase)
}

func (e *emitter) emitTestCaseDecl(d *ast.TestCaseDecl) {
	e.emitLeadingTrivia(&d.NodeBase)
	e.writef("%s '%s'", e.kw("TEST_CASE"), d.Name)
	e.newline()
	for _, vb := range d.VarBlocks {
		e.emitVarBlock(vb)
	}
	for _, s := range d.Body {
		e.emitIndentedStmt(s)
	}
	e.write(e.kw("END_TEST_CASE"))
	e.newline()
	e.emitTrailingTrivia(&d.NodeBase)
}

// --- var blocks ---

// shouldSkipVarDecl returns true if the var decl should be filtered out
// based on the current vendor target.
func (e *emitter) shouldSkipVarDecl(vd *ast.VarDecl) bool {
	if vd.Type == nil {
		return false
	}
	return e.shouldSkipTypeSpec(vd.Type)
}

// shouldSkipTypeSpec returns true if the type should be filtered out.
func (e *emitter) shouldSkipTypeSpec(ts ast.TypeSpec) bool {
	switch t := ts.(type) {
	case *ast.PointerType:
		if !e.opts.Target.supportsPointerTo() {
			return true
		}
	case *ast.ReferenceType:
		if !e.opts.Target.supportsReferenceTo() {
			return true
		}
	case *ast.NamedType:
		if t.Name != nil && !e.opts.Target.supports64Bit() && is64BitType(t.Name.Name) {
			return true
		}
	}
	return false
}

func (e *emitter) emitVarBlock(vb *ast.VarBlock) {
	// Collect non-skipped declarations
	var kept []*ast.VarDecl
	for _, vd := range vb.Declarations {
		if !e.shouldSkipVarDecl(vd) {
			kept = append(kept, vd)
		}
	}
	// If all declarations were filtered, skip the entire block
	if len(kept) == 0 {
		return
	}

	e.emitLeadingTrivia(&vb.NodeBase)
	e.write(vb.Section.String())
	if vb.IsConstant {
		e.writef(" %s", e.kw("CONSTANT"))
	}
	if vb.IsRetain {
		e.writef(" %s", e.kw("RETAIN"))
	}
	if vb.IsPersistent {
		e.writef(" %s", e.kw("PERSISTENT"))
	}
	e.newline()
	e.indent++
	for _, vd := range kept {
		e.emitVarDecl(vd)
	}
	e.indent--
	e.write(e.kw("END_VAR"))
	e.newline()
	e.emitTrailingTrivia(&vb.NodeBase)
}

func (e *emitter) emitVarDecl(vd *ast.VarDecl) {
	e.emitLeadingTrivia(&vd.NodeBase)
	e.emitIndent()
	for i, name := range vd.Names {
		if i > 0 {
			e.write(", ")
		}
		e.write(name.Name)
	}
	if vd.AtAddress != nil {
		e.writef(" %s %s", e.kw("AT"), vd.AtAddress.Name)
	}
	e.write(" : ")
	e.emitTypeSpec(vd.Type)
	if vd.InitValue != nil {
		e.write(" := ")
		e.emitExpr(vd.InitValue)
	}
	e.write(";")
	e.newline()
	e.emitTrailingTrivia(&vd.NodeBase)
}

// --- type specs ---

func (e *emitter) emitTypeSpec(ts ast.TypeSpec) {
	if ts == nil {
		return
	}
	switch t := ts.(type) {
	case *ast.NamedType:
		e.write(t.Name.Name)
	case *ast.ArrayType:
		e.write(e.kw("ARRAY") + "[")
		for i, r := range t.Ranges {
			if i > 0 {
				e.write(", ")
			}
			e.emitExpr(r.Low)
			e.write("..")
			e.emitExpr(r.High)
		}
		e.writef("] %s ", e.kw("OF"))
		e.emitTypeSpec(t.ElementType)
	case *ast.PointerType:
		e.writef("%s ", e.kw("POINTER TO"))
		e.emitTypeSpec(t.BaseType)
	case *ast.ReferenceType:
		e.writef("%s ", e.kw("REFERENCE TO"))
		e.emitTypeSpec(t.BaseType)
	case *ast.StringType:
		if t.IsWide {
			e.write(e.kw("WSTRING"))
		} else {
			e.write(e.kw("STRING"))
		}
		if t.Length != nil {
			e.write("(")
			e.emitExpr(t.Length)
			e.write(")")
		}
	case *ast.SubrangeType:
		e.emitTypeSpec(t.BaseType)
		e.write("(")
		e.emitExpr(t.Low)
		e.write("..")
		e.emitExpr(t.High)
		e.write(")")
	case *ast.EnumType:
		e.write("(")
		for i, v := range t.Values {
			if i > 0 {
				e.write(", ")
			}
			e.write(v.Name.Name)
			if v.Value != nil {
				e.write(" := ")
				e.emitExpr(v.Value)
			}
		}
		e.write(")")
	case *ast.StructType:
		e.write(e.kw("STRUCT"))
		e.newline()
		e.indent++
		for _, m := range t.Members {
			e.emitIndent()
			e.write(m.Name.Name)
			e.write(" : ")
			e.emitTypeSpec(m.Type)
			if m.InitValue != nil {
				e.write(" := ")
				e.emitExpr(m.InitValue)
			}
			e.write(";")
			e.newline()
		}
		e.indent--
		e.emitIndent()
		e.write(e.kw("END_STRUCT"))
	case *ast.ErrorNode:
		// skip
	}
}

// --- statements ---

func (e *emitter) emitIndentedStmt(s ast.Statement) {
	e.emitLeadingTrivia(e.nodeBase(s))
	e.emitIndent()
	e.emitStmt(s)
}

func (e *emitter) emitStmt(s ast.Statement) {
	switch st := s.(type) {
	case *ast.AssignStmt:
		e.emitAssignStmt(st)
	case *ast.CallStmt:
		e.emitCallStmt(st)
	case *ast.IfStmt:
		e.emitIfStmt(st)
	case *ast.CaseStmt:
		e.emitCaseStmt(st)
	case *ast.ForStmt:
		e.emitForStmt(st)
	case *ast.WhileStmt:
		e.emitWhileStmt(st)
	case *ast.RepeatStmt:
		e.emitRepeatStmt(st)
	case *ast.ReturnStmt:
		e.write(e.kw("RETURN") + ";")
		e.newline()
	case *ast.ExitStmt:
		e.write(e.kw("EXIT") + ";")
		e.newline()
	case *ast.ContinueStmt:
		e.write(e.kw("CONTINUE") + ";")
		e.newline()
	case *ast.EmptyStmt:
		e.write(";")
		e.newline()
	case *ast.ErrorNode:
		// skip
	}
}

func (e *emitter) emitAssignStmt(s *ast.AssignStmt) {
	e.emitExpr(s.Target)
	e.write(" := ")
	if s.Value != nil {
		e.emitExpr(s.Value)
	}
	e.write(";")
	e.newline()
}

func (e *emitter) emitCallStmt(s *ast.CallStmt) {
	e.emitExpr(s.Callee)
	e.write("(")
	for i, arg := range s.Args {
		if i > 0 {
			e.write(", ")
		}
		if arg.Name != nil {
			e.write(arg.Name.Name)
			if arg.IsOutput {
				e.write(" => ")
			} else {
				e.write(" := ")
			}
		}
		e.emitExpr(arg.Value)
	}
	e.write(");")
	e.newline()
}

func (e *emitter) emitIfStmt(s *ast.IfStmt) {
	e.writef("%s ", e.kw("IF"))
	e.emitExpr(s.Condition)
	e.writef(" %s", e.kw("THEN"))
	e.newline()
	e.indent++
	for _, st := range s.Then {
		e.emitIndentedStmt(st)
	}
	e.indent--
	for _, elif := range s.ElsIfs {
		e.emitIndent()
		e.writef("%s ", e.kw("ELSIF"))
		e.emitExpr(elif.Condition)
		e.writef(" %s", e.kw("THEN"))
		e.newline()
		e.indent++
		for _, st := range elif.Body {
			e.emitIndentedStmt(st)
		}
		e.indent--
	}
	if len(s.Else) > 0 {
		e.emitIndent()
		e.write(e.kw("ELSE"))
		e.newline()
		e.indent++
		for _, st := range s.Else {
			e.emitIndentedStmt(st)
		}
		e.indent--
	}
	e.emitIndent()
	e.write(e.kw("END_IF") + ";")
	e.newline()
}

func (e *emitter) emitCaseStmt(s *ast.CaseStmt) {
	e.writef("%s ", e.kw("CASE"))
	e.emitExpr(s.Expr)
	e.writef(" %s", e.kw("OF"))
	e.newline()
	e.indent++
	for _, branch := range s.Branches {
		e.emitIndent()
		for i, label := range branch.Labels {
			if i > 0 {
				e.write(", ")
			}
			e.emitCaseLabel(label)
		}
		e.write(":")
		e.newline()
		e.indent++
		for _, st := range branch.Body {
			e.emitIndentedStmt(st)
		}
		e.indent--
	}
	e.indent--
	if len(s.ElseBranch) > 0 {
		e.emitIndent()
		e.write(e.kw("ELSE"))
		e.newline()
		e.indent++
		for _, st := range s.ElseBranch {
			e.emitIndentedStmt(st)
		}
		e.indent--
	}
	e.emitIndent()
	e.write(e.kw("END_CASE") + ";")
	e.newline()
}

func (e *emitter) emitCaseLabel(label ast.CaseLabel) {
	switch l := label.(type) {
	case *ast.CaseLabelValue:
		e.emitExpr(l.Value)
	case *ast.CaseLabelRange:
		e.emitExpr(l.Low)
		e.write("..")
		e.emitExpr(l.High)
	}
}

func (e *emitter) emitForStmt(s *ast.ForStmt) {
	e.writef("%s %s := ", e.kw("FOR"), s.Variable.Name)
	e.emitExpr(s.From)
	e.writef(" %s ", e.kw("TO"))
	e.emitExpr(s.To)
	if s.By != nil {
		e.writef(" %s ", e.kw("BY"))
		e.emitExpr(s.By)
	}
	e.writef(" %s", e.kw("DO"))
	e.newline()
	e.indent++
	for _, st := range s.Body {
		e.emitIndentedStmt(st)
	}
	e.indent--
	e.emitIndent()
	e.write(e.kw("END_FOR") + ";")
	e.newline()
}

func (e *emitter) emitWhileStmt(s *ast.WhileStmt) {
	e.writef("%s ", e.kw("WHILE"))
	e.emitExpr(s.Condition)
	e.writef(" %s", e.kw("DO"))
	e.newline()
	e.indent++
	for _, st := range s.Body {
		e.emitIndentedStmt(st)
	}
	e.indent--
	e.emitIndent()
	e.write(e.kw("END_WHILE") + ";")
	e.newline()
}

func (e *emitter) emitRepeatStmt(s *ast.RepeatStmt) {
	e.write(e.kw("REPEAT"))
	e.newline()
	e.indent++
	for _, st := range s.Body {
		e.emitIndentedStmt(st)
	}
	e.indent--
	e.emitIndent()
	e.writef("%s ", e.kw("UNTIL"))
	e.emitExpr(s.Condition)
	e.newline()
	e.emitIndent()
	e.write(e.kw("END_REPEAT") + ";")
	e.newline()
}

// --- expressions ---

func (e *emitter) emitExpr(expr ast.Expr) {
	if expr == nil {
		return
	}
	switch x := expr.(type) {
	case *ast.BinaryExpr:
		e.emitExpr(x.Left)
		e.writef(" %s ", strings.ToUpper(x.Op.Text))
		e.emitExpr(x.Right)
	case *ast.UnaryExpr:
		op := strings.ToUpper(x.Op.Text)
		// Word operators (NOT) get a space; symbol operators (-) don't
		if isWordOp(op) {
			e.writef("%s ", op)
		} else {
			e.write(op)
		}
		e.emitExpr(x.Operand)
	case *ast.Literal:
		e.emitLiteral(x)
	case *ast.Ident:
		e.write(x.Name)
	case *ast.CallExpr:
		e.emitExpr(x.Callee)
		e.write("(")
		for i, arg := range x.Args {
			if i > 0 {
				e.write(", ")
			}
			e.emitExpr(arg)
		}
		e.write(")")
	case *ast.MemberAccessExpr:
		e.emitExpr(x.Object)
		e.write(".")
		e.write(x.Member.Name)
	case *ast.IndexExpr:
		e.emitExpr(x.Object)
		e.write("[")
		for i, idx := range x.Indices {
			if i > 0 {
				e.write(", ")
			}
			e.emitExpr(idx)
		}
		e.write("]")
	case *ast.DerefExpr:
		e.emitExpr(x.Operand)
		e.write("^")
	case *ast.ParenExpr:
		e.write("(")
		e.emitExpr(x.Inner)
		e.write(")")
	case *ast.ErrorNode:
		// skip
	}
}

func (e *emitter) emitLiteral(lit *ast.Literal) {
	if lit.TypePrefix != "" {
		e.writef("%s#%s", lit.TypePrefix, lit.Value)
	} else {
		e.write(lit.Value)
	}
}

// isWordOp returns true for word-based operators that need spacing.
func isWordOp(op string) bool {
	switch op {
	case "NOT", "AND", "OR", "XOR", "MOD":
		return true
	}
	return false
}

// nodeBase extracts the NodeBase from a Statement for trivia access.
func (e *emitter) nodeBase(s ast.Statement) *ast.NodeBase {
	switch st := s.(type) {
	case *ast.AssignStmt:
		return &st.NodeBase
	case *ast.CallStmt:
		return &st.NodeBase
	case *ast.IfStmt:
		return &st.NodeBase
	case *ast.CaseStmt:
		return &st.NodeBase
	case *ast.ForStmt:
		return &st.NodeBase
	case *ast.WhileStmt:
		return &st.NodeBase
	case *ast.RepeatStmt:
		return &st.NodeBase
	case *ast.ReturnStmt:
		return &st.NodeBase
	case *ast.ExitStmt:
		return &st.NodeBase
	case *ast.ContinueStmt:
		return &st.NodeBase
	case *ast.EmptyStmt:
		return &st.NodeBase
	case *ast.ErrorNode:
		return &st.NodeBase
	default:
		return &ast.NodeBase{}
	}
}
