// Package format provides ST code formatting with configurable style.
// It re-emits a parsed AST as consistently formatted Structured Text,
// preserving all comments while normalizing indentation, spacing, and
// keyword casing. Unlike pkg/emit, the formatter does NOT do vendor
// filtering -- it formats ALL constructs as-is.
package format

import (
	"fmt"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
)

// Format produces consistently formatted Structured Text from a parsed AST.
// It returns an empty string for nil input.
func Format(file *ast.SourceFile, opts FormatOptions) string {
	if file == nil {
		return ""
	}
	if opts.Indent == "" {
		opts.Indent = "    "
	}
	f := &formatter{opts: opts}
	f.emitSourceFile(file)
	return f.buf.String()
}

// formatter is the internal state machine for ST formatting.
type formatter struct {
	buf    strings.Builder
	indent int
	opts   FormatOptions
}

// --- helpers ---

func (f *formatter) write(s string) {
	f.buf.WriteString(s)
}

func (f *formatter) writef(format string, args ...any) {
	fmt.Fprintf(&f.buf, format, args...)
}

func (f *formatter) newline() {
	f.buf.WriteByte('\n')
}

func (f *formatter) emitIndent() {
	for i := 0; i < f.indent; i++ {
		f.write(f.opts.Indent)
	}
}

func (f *formatter) kw(keyword string) string {
	if f.opts.UppercaseKeywords {
		return strings.ToUpper(keyword)
	}
	return strings.ToLower(keyword)
}

// --- trivia ---

func (f *formatter) emitTrivia(trivia []ast.Trivia) {
	for _, t := range trivia {
		f.write(t.Text)
	}
}

func (f *formatter) emitLeadingTrivia(n *ast.NodeBase) {
	f.emitTrivia(n.LeadingTrivia)
}

func (f *formatter) emitTrailingTrivia(n *ast.NodeBase) {
	f.emitTrivia(n.TrailingTrivia)
}

// --- source file ---

func (f *formatter) emitSourceFile(file *ast.SourceFile) {
	for i, decl := range file.Declarations {
		if i > 0 {
			f.newline()
		}
		f.emitDecl(decl)
	}
}

// --- declarations ---

func (f *formatter) emitDecl(decl ast.Declaration) {
	switch d := decl.(type) {
	case *ast.ProgramDecl:
		f.emitProgramDecl(d)
	case *ast.FunctionBlockDecl:
		f.emitFunctionBlockDecl(d)
	case *ast.FunctionDecl:
		f.emitFunctionDecl(d)
	case *ast.InterfaceDecl:
		f.emitInterfaceDecl(d)
	case *ast.MethodDecl:
		f.emitMethodDecl(d)
	case *ast.PropertyDecl:
		f.emitPropertyDecl(d)
	case *ast.TypeDecl:
		f.emitTypeDecl(d)
	case *ast.ActionDecl:
		f.emitActionDecl(d)
	case *ast.TestCaseDecl:
		f.emitTestCaseDecl(d)
	case *ast.ErrorNode:
		// skip error nodes
	}
}

func (f *formatter) emitProgramDecl(d *ast.ProgramDecl) {
	f.emitLeadingTrivia(&d.NodeBase)
	f.writef("%s %s", f.kw("PROGRAM"), d.Name.Name)
	f.newline()
	for _, vb := range d.VarBlocks {
		f.emitVarBlock(vb)
	}
	f.indent++
	for _, s := range d.Body {
		f.emitIndentedStmt(s)
	}
	f.indent--
	f.write(f.kw("END_PROGRAM"))
	f.newline()
	f.emitTrailingTrivia(&d.NodeBase)
}

func (f *formatter) emitFunctionBlockDecl(d *ast.FunctionBlockDecl) {
	f.emitLeadingTrivia(&d.NodeBase)
	f.writef("%s %s", f.kw("FUNCTION_BLOCK"), d.Name.Name)
	if d.Extends != nil {
		f.writef(" %s %s", f.kw("EXTENDS"), d.Extends.Name)
	}
	if len(d.Implements) > 0 {
		f.writef(" %s ", f.kw("IMPLEMENTS"))
		for i, impl := range d.Implements {
			if i > 0 {
				f.write(", ")
			}
			f.write(impl.Name)
		}
	}
	f.newline()
	for _, vb := range d.VarBlocks {
		f.emitVarBlock(vb)
	}
	f.indent++
	for _, s := range d.Body {
		f.emitIndentedStmt(s)
	}
	f.indent--

	// Methods and properties - no OOP filtering in formatter
	for _, m := range d.Methods {
		f.newline()
		f.emitMethodDecl(m)
	}
	for _, p := range d.Properties {
		f.newline()
		f.emitPropertyDecl(p)
	}

	f.newline()
	f.write(f.kw("END_FUNCTION_BLOCK"))
	f.newline()
	f.emitTrailingTrivia(&d.NodeBase)
}

func (f *formatter) emitFunctionDecl(d *ast.FunctionDecl) {
	f.emitLeadingTrivia(&d.NodeBase)
	f.writef("%s %s", f.kw("FUNCTION"), d.Name.Name)
	if d.ReturnType != nil {
		f.write(" : ")
		f.emitTypeSpec(d.ReturnType)
	}
	f.newline()
	for _, vb := range d.VarBlocks {
		f.emitVarBlock(vb)
	}
	f.indent++
	for _, s := range d.Body {
		f.emitIndentedStmt(s)
	}
	f.indent--
	f.write(f.kw("END_FUNCTION"))
	f.newline()
	f.emitTrailingTrivia(&d.NodeBase)
}

func (f *formatter) emitInterfaceDecl(d *ast.InterfaceDecl) {
	f.emitLeadingTrivia(&d.NodeBase)
	f.writef("%s %s", f.kw("INTERFACE"), d.Name.Name)
	if len(d.Extends) > 0 {
		f.writef(" %s ", f.kw("EXTENDS"))
		for i, ext := range d.Extends {
			if i > 0 {
				f.write(", ")
			}
			f.write(ext.Name)
		}
	}
	f.newline()
	for _, m := range d.Methods {
		f.emitMethodSignature(m)
	}
	for _, p := range d.Properties {
		f.emitPropertySignature(p)
	}
	f.write(f.kw("END_INTERFACE"))
	f.newline()
	f.emitTrailingTrivia(&d.NodeBase)
}

func (f *formatter) emitMethodDecl(d *ast.MethodDecl) {
	f.emitLeadingTrivia(&d.NodeBase)
	f.write(f.kw("METHOD"))
	if d.AccessModifier != ast.AccessNone {
		f.writef(" %s", d.AccessModifier.String())
	}
	if d.IsAbstract {
		f.writef(" %s", f.kw("ABSTRACT"))
	}
	if d.IsFinal {
		f.writef(" %s", f.kw("FINAL"))
	}
	if d.IsOverride {
		f.writef(" %s", f.kw("OVERRIDE"))
	}
	f.writef(" %s", d.Name.Name)
	if d.ReturnType != nil {
		f.write(" : ")
		f.emitTypeSpec(d.ReturnType)
	}
	f.newline()
	for _, vb := range d.VarBlocks {
		f.emitVarBlock(vb)
	}
	f.indent++
	for _, s := range d.Body {
		f.emitIndentedStmt(s)
	}
	f.indent--
	f.write(f.kw("END_METHOD"))
	f.newline()
	f.emitTrailingTrivia(&d.NodeBase)
}

func (f *formatter) emitPropertyDecl(d *ast.PropertyDecl) {
	f.emitLeadingTrivia(&d.NodeBase)
	f.write(f.kw("PROPERTY"))
	if d.AccessModifier != ast.AccessNone {
		f.writef(" %s", d.AccessModifier.String())
	}
	f.writef(" %s", d.Name.Name)
	if d.Type != nil {
		f.write(" : ")
		f.emitTypeSpec(d.Type)
	}
	f.newline()
	if d.Getter != nil {
		f.emitMethodDecl(d.Getter)
	}
	if d.Setter != nil {
		f.emitMethodDecl(d.Setter)
	}
	f.write(f.kw("END_PROPERTY"))
	f.newline()
	f.emitTrailingTrivia(&d.NodeBase)
}

func (f *formatter) emitMethodSignature(d *ast.MethodSignature) {
	f.emitLeadingTrivia(&d.NodeBase)
	f.indent++
	f.emitIndent()
	f.writef("%s %s", f.kw("METHOD"), d.Name.Name)
	if d.ReturnType != nil {
		f.write(" : ")
		f.emitTypeSpec(d.ReturnType)
	}
	f.write(";")
	f.newline()
	f.indent--
	f.emitTrailingTrivia(&d.NodeBase)
}

func (f *formatter) emitPropertySignature(d *ast.PropertySignature) {
	f.emitLeadingTrivia(&d.NodeBase)
	f.indent++
	f.emitIndent()
	f.writef("%s %s", f.kw("PROPERTY"), d.Name.Name)
	if d.Type != nil {
		f.write(" : ")
		f.emitTypeSpec(d.Type)
	}
	f.write(";")
	f.newline()
	f.indent--
	f.emitTrailingTrivia(&d.NodeBase)
}

func (f *formatter) emitTypeDecl(d *ast.TypeDecl) {
	f.emitLeadingTrivia(&d.NodeBase)
	f.writef("%s %s :", f.kw("TYPE"), d.Name.Name)
	f.newline()
	f.emitTypeBody(d.Type)
	f.write(f.kw("END_TYPE"))
	f.newline()
	f.emitTrailingTrivia(&d.NodeBase)
}

func (f *formatter) emitTypeBody(ts ast.TypeSpec) {
	switch t := ts.(type) {
	case *ast.StructType:
		f.write(f.kw("STRUCT"))
		f.newline()
		f.indent++
		for _, m := range t.Members {
			f.emitIndent()
			f.write(m.Name.Name)
			f.write(" : ")
			f.emitTypeSpec(m.Type)
			if m.InitValue != nil {
				f.write(" := ")
				f.emitExpr(m.InitValue)
			}
			f.write(";")
			f.newline()
		}
		f.indent--
		f.write(f.kw("END_STRUCT"))
		f.newline()
	case *ast.EnumType:
		f.write("(")
		f.newline()
		f.indent++
		for i, v := range t.Values {
			f.emitIndent()
			f.write(v.Name.Name)
			if v.Value != nil {
				f.write(" := ")
				f.emitExpr(v.Value)
			}
			if i < len(t.Values)-1 {
				f.write(",")
			}
			f.newline()
		}
		f.indent--
		f.write(");")
		f.newline()
	default:
		f.emitTypeSpec(ts)
		f.write(";")
		f.newline()
	}
}

func (f *formatter) emitActionDecl(d *ast.ActionDecl) {
	f.emitLeadingTrivia(&d.NodeBase)
	f.writef("%s %s:", f.kw("ACTION"), d.Name.Name)
	f.newline()
	f.indent++
	for _, s := range d.Body {
		f.emitIndentedStmt(s)
	}
	f.indent--
	f.write(f.kw("END_ACTION"))
	f.newline()
	f.emitTrailingTrivia(&d.NodeBase)
}

func (f *formatter) emitTestCaseDecl(d *ast.TestCaseDecl) {
	f.emitLeadingTrivia(&d.NodeBase)
	f.writef("%s '%s'", f.kw("TEST_CASE"), d.Name)
	f.newline()
	for _, vb := range d.VarBlocks {
		f.emitVarBlock(vb)
	}
	f.indent++
	for _, s := range d.Body {
		f.emitIndentedStmt(s)
	}
	f.indent--
	f.write(f.kw("END_TEST_CASE"))
	f.newline()
	f.emitTrailingTrivia(&d.NodeBase)
}

// --- var blocks ---

func (f *formatter) emitVarBlock(vb *ast.VarBlock) {
	f.emitLeadingTrivia(&vb.NodeBase)
	f.write(f.kw(vb.Section.String()))
	if vb.IsConstant {
		f.writef(" %s", f.kw("CONSTANT"))
	}
	if vb.IsRetain {
		f.writef(" %s", f.kw("RETAIN"))
	}
	if vb.IsPersistent {
		f.writef(" %s", f.kw("PERSISTENT"))
	}
	f.newline()
	f.indent++
	for _, vd := range vb.Declarations {
		f.emitVarDecl(vd)
	}
	f.indent--
	f.write(f.kw("END_VAR"))
	f.newline()
	f.emitTrailingTrivia(&vb.NodeBase)
}

func (f *formatter) emitVarDecl(vd *ast.VarDecl) {
	f.emitLeadingTrivia(&vd.NodeBase)
	f.emitIndent()
	for i, name := range vd.Names {
		if i > 0 {
			f.write(", ")
		}
		f.write(name.Name)
	}
	if vd.AtAddress != nil {
		f.writef(" %s %s", f.kw("AT"), vd.AtAddress.Name)
	}
	f.write(" : ")
	f.emitTypeSpec(vd.Type)
	if vd.InitValue != nil {
		f.write(" := ")
		f.emitExpr(vd.InitValue)
	}
	f.write(";")
	f.newline()
	f.emitTrailingTrivia(&vd.NodeBase)
}

// --- type specs ---

func (f *formatter) emitTypeSpec(ts ast.TypeSpec) {
	if ts == nil {
		return
	}
	switch t := ts.(type) {
	case *ast.NamedType:
		f.write(t.Name.Name)
	case *ast.ArrayType:
		f.write(f.kw("ARRAY") + "[")
		for i, r := range t.Ranges {
			if i > 0 {
				f.write(", ")
			}
			f.emitExpr(r.Low)
			f.write("..")
			f.emitExpr(r.High)
		}
		f.writef("] %s ", f.kw("OF"))
		f.emitTypeSpec(t.ElementType)
	case *ast.PointerType:
		f.writef("%s ", f.kw("POINTER TO"))
		f.emitTypeSpec(t.BaseType)
	case *ast.ReferenceType:
		f.writef("%s ", f.kw("REFERENCE TO"))
		f.emitTypeSpec(t.BaseType)
	case *ast.StringType:
		if t.IsWide {
			f.write(f.kw("WSTRING"))
		} else {
			f.write(f.kw("STRING"))
		}
		if t.Length != nil {
			f.write("(")
			f.emitExpr(t.Length)
			f.write(")")
		}
	case *ast.SubrangeType:
		f.emitTypeSpec(t.BaseType)
		f.write("(")
		f.emitExpr(t.Low)
		f.write("..")
		f.emitExpr(t.High)
		f.write(")")
	case *ast.EnumType:
		f.write("(")
		for i, v := range t.Values {
			if i > 0 {
				f.write(", ")
			}
			f.write(v.Name.Name)
			if v.Value != nil {
				f.write(" := ")
				f.emitExpr(v.Value)
			}
		}
		f.write(")")
	case *ast.StructType:
		f.write(f.kw("STRUCT"))
		f.newline()
		f.indent++
		for _, m := range t.Members {
			f.emitIndent()
			f.write(m.Name.Name)
			f.write(" : ")
			f.emitTypeSpec(m.Type)
			if m.InitValue != nil {
				f.write(" := ")
				f.emitExpr(m.InitValue)
			}
			f.write(";")
			f.newline()
		}
		f.indent--
		f.emitIndent()
		f.write(f.kw("END_STRUCT"))
	case *ast.ErrorNode:
		// skip
	}
}

// --- statements ---

func (f *formatter) emitIndentedStmt(s ast.Statement) {
	f.emitLeadingTrivia(f.nodeBase(s))
	f.emitIndent()
	f.emitStmt(s)
}

func (f *formatter) emitStmt(s ast.Statement) {
	switch st := s.(type) {
	case *ast.AssignStmt:
		f.emitAssignStmt(st)
	case *ast.CallStmt:
		f.emitCallStmt(st)
	case *ast.IfStmt:
		f.emitIfStmt(st)
	case *ast.CaseStmt:
		f.emitCaseStmt(st)
	case *ast.ForStmt:
		f.emitForStmt(st)
	case *ast.WhileStmt:
		f.emitWhileStmt(st)
	case *ast.RepeatStmt:
		f.emitRepeatStmt(st)
	case *ast.ReturnStmt:
		f.write(f.kw("RETURN") + ";")
		f.newline()
	case *ast.ExitStmt:
		f.write(f.kw("EXIT") + ";")
		f.newline()
	case *ast.ContinueStmt:
		f.write(f.kw("CONTINUE") + ";")
		f.newline()
	case *ast.EmptyStmt:
		f.write(";")
		f.newline()
	case *ast.ErrorNode:
		// skip
	}
}

func (f *formatter) emitAssignStmt(s *ast.AssignStmt) {
	f.emitExpr(s.Target)
	f.write(" := ")
	if s.Value != nil {
		f.emitExpr(s.Value)
	}
	f.write(";")
	f.newline()
}

func (f *formatter) emitCallStmt(s *ast.CallStmt) {
	f.emitExpr(s.Callee)
	f.write("(")
	for i, arg := range s.Args {
		if i > 0 {
			f.write(", ")
		}
		if arg.Name != nil {
			f.write(arg.Name.Name)
			if arg.IsOutput {
				f.write(" => ")
			} else {
				f.write(" := ")
			}
		}
		f.emitExpr(arg.Value)
	}
	f.write(");")
	f.newline()
}

func (f *formatter) emitIfStmt(s *ast.IfStmt) {
	f.writef("%s ", f.kw("IF"))
	f.emitExpr(s.Condition)
	f.writef(" %s", f.kw("THEN"))
	f.newline()
	f.indent++
	for _, st := range s.Then {
		f.emitIndentedStmt(st)
	}
	f.indent--
	for _, elif := range s.ElsIfs {
		f.emitIndent()
		f.writef("%s ", f.kw("ELSIF"))
		f.emitExpr(elif.Condition)
		f.writef(" %s", f.kw("THEN"))
		f.newline()
		f.indent++
		for _, st := range elif.Body {
			f.emitIndentedStmt(st)
		}
		f.indent--
	}
	if len(s.Else) > 0 {
		f.emitIndent()
		f.write(f.kw("ELSE"))
		f.newline()
		f.indent++
		for _, st := range s.Else {
			f.emitIndentedStmt(st)
		}
		f.indent--
	}
	f.emitIndent()
	f.write(f.kw("END_IF") + ";")
	f.newline()
}

func (f *formatter) emitCaseStmt(s *ast.CaseStmt) {
	f.writef("%s ", f.kw("CASE"))
	f.emitExpr(s.Expr)
	f.writef(" %s", f.kw("OF"))
	f.newline()
	f.indent++
	for _, branch := range s.Branches {
		f.emitIndent()
		for i, label := range branch.Labels {
			if i > 0 {
				f.write(", ")
			}
			f.emitCaseLabel(label)
		}
		f.write(":")
		f.newline()
		f.indent++
		for _, st := range branch.Body {
			f.emitIndentedStmt(st)
		}
		f.indent--
	}
	f.indent--
	if len(s.ElseBranch) > 0 {
		f.emitIndent()
		f.write(f.kw("ELSE"))
		f.newline()
		f.indent++
		for _, st := range s.ElseBranch {
			f.emitIndentedStmt(st)
		}
		f.indent--
	}
	f.emitIndent()
	f.write(f.kw("END_CASE") + ";")
	f.newline()
}

func (f *formatter) emitCaseLabel(label ast.CaseLabel) {
	switch l := label.(type) {
	case *ast.CaseLabelValue:
		f.emitExpr(l.Value)
	case *ast.CaseLabelRange:
		f.emitExpr(l.Low)
		f.write("..")
		f.emitExpr(l.High)
	}
}

func (f *formatter) emitForStmt(s *ast.ForStmt) {
	f.writef("%s %s := ", f.kw("FOR"), s.Variable.Name)
	f.emitExpr(s.From)
	f.writef(" %s ", f.kw("TO"))
	f.emitExpr(s.To)
	if s.By != nil {
		f.writef(" %s ", f.kw("BY"))
		f.emitExpr(s.By)
	}
	f.writef(" %s", f.kw("DO"))
	f.newline()
	f.indent++
	for _, st := range s.Body {
		f.emitIndentedStmt(st)
	}
	f.indent--
	f.emitIndent()
	f.write(f.kw("END_FOR") + ";")
	f.newline()
}

func (f *formatter) emitWhileStmt(s *ast.WhileStmt) {
	f.writef("%s ", f.kw("WHILE"))
	f.emitExpr(s.Condition)
	f.writef(" %s", f.kw("DO"))
	f.newline()
	f.indent++
	for _, st := range s.Body {
		f.emitIndentedStmt(st)
	}
	f.indent--
	f.emitIndent()
	f.write(f.kw("END_WHILE") + ";")
	f.newline()
}

func (f *formatter) emitRepeatStmt(s *ast.RepeatStmt) {
	f.write(f.kw("REPEAT"))
	f.newline()
	f.indent++
	for _, st := range s.Body {
		f.emitIndentedStmt(st)
	}
	f.indent--
	f.emitIndent()
	f.writef("%s ", f.kw("UNTIL"))
	f.emitExpr(s.Condition)
	f.newline()
	f.emitIndent()
	f.write(f.kw("END_REPEAT") + ";")
	f.newline()
}

// --- expressions ---

func (f *formatter) emitExpr(expr ast.Expr) {
	if expr == nil {
		return
	}
	switch x := expr.(type) {
	case *ast.BinaryExpr:
		f.emitExpr(x.Left)
		f.writef(" %s ", f.opText(x.Op.Text))
		f.emitExpr(x.Right)
	case *ast.UnaryExpr:
		op := f.opText(x.Op.Text)
		if isWordOp(strings.ToUpper(x.Op.Text)) {
			f.writef("%s ", op)
		} else {
			f.write(op)
		}
		f.emitExpr(x.Operand)
	case *ast.Literal:
		f.emitLiteral(x)
	case *ast.Ident:
		f.write(x.Name)
	case *ast.CallExpr:
		f.emitExpr(x.Callee)
		f.write("(")
		for i, arg := range x.Args {
			if i > 0 {
				f.write(", ")
			}
			f.emitExpr(arg)
		}
		f.write(")")
	case *ast.MemberAccessExpr:
		f.emitExpr(x.Object)
		f.write(".")
		f.write(x.Member.Name)
	case *ast.IndexExpr:
		f.emitExpr(x.Object)
		f.write("[")
		for i, idx := range x.Indices {
			if i > 0 {
				f.write(", ")
			}
			f.emitExpr(idx)
		}
		f.write("]")
	case *ast.DerefExpr:
		f.emitExpr(x.Operand)
		f.write("^")
	case *ast.ParenExpr:
		f.write("(")
		f.emitExpr(x.Inner)
		f.write(")")
	case *ast.ErrorNode:
		// skip
	}
}

func (f *formatter) opText(text string) string {
	upper := strings.ToUpper(text)
	if isWordOp(upper) {
		if f.opts.UppercaseKeywords {
			return upper
		}
		return strings.ToLower(text)
	}
	return strings.ToUpper(text)
}

func (f *formatter) emitLiteral(lit *ast.Literal) {
	if lit.TypePrefix != "" {
		f.writef("%s#%s", lit.TypePrefix, lit.Value)
	} else {
		f.write(lit.Value)
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
func (f *formatter) nodeBase(s ast.Statement) *ast.NodeBase {
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
