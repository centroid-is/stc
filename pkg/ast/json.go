package ast

import (
	"encoding/json"
	"fmt"
)

// MarshalNode serializes any Node to JSON with a "kind" discriminator field.
func MarshalNode(n Node) ([]byte, error) {
	if n == nil {
		return []byte("null"), nil
	}
	return json.Marshal(nodeToMap(n))
}

// nodeToMap converts a Node to a map for JSON serialization, ensuring the
// "kind" field is always present as a string discriminator.
func nodeToMap(n Node) map[string]interface{} {
	if n == nil {
		return nil
	}
	m := make(map[string]interface{})
	m["kind"] = n.Kind().String()

	switch v := n.(type) {
	case *SourceFile:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if len(v.Declarations) > 0 {
			decls := make([]interface{}, len(v.Declarations))
			for i, d := range v.Declarations {
				decls[i] = nodeToMap(d)
			}
			m["declarations"] = decls
		}

	case *ProgramDecl:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		marshalVarBlocks(m, v.VarBlocks)
		marshalBody(m, v.Body)

	case *FunctionBlockDecl:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		if v.Extends != nil {
			m["extends"] = nodeToMap(v.Extends)
		}
		if len(v.Implements) > 0 {
			impls := make([]interface{}, len(v.Implements))
			for i, impl := range v.Implements {
				impls[i] = nodeToMap(impl)
			}
			m["implements"] = impls
		}
		marshalVarBlocks(m, v.VarBlocks)
		marshalBody(m, v.Body)
		if len(v.Methods) > 0 {
			methods := make([]interface{}, len(v.Methods))
			for i, method := range v.Methods {
				methods[i] = nodeToMap(method)
			}
			m["methods"] = methods
		}
		if len(v.Properties) > 0 {
			props := make([]interface{}, len(v.Properties))
			for i, p := range v.Properties {
				props[i] = nodeToMap(p)
			}
			m["properties"] = props
		}

	case *FunctionDecl:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		if v.ReturnType != nil {
			m["return_type"] = nodeToMap(v.ReturnType)
		}
		marshalVarBlocks(m, v.VarBlocks)
		marshalBody(m, v.Body)

	case *InterfaceDecl:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		if len(v.Extends) > 0 {
			exts := make([]interface{}, len(v.Extends))
			for i, e := range v.Extends {
				exts[i] = nodeToMap(e)
			}
			m["extends"] = exts
		}

	case *MethodDecl:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.AccessModifier != AccessNone {
			m["access_modifier"] = v.AccessModifier.String()
		}
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		if v.ReturnType != nil {
			m["return_type"] = nodeToMap(v.ReturnType)
		}
		marshalVarBlocks(m, v.VarBlocks)
		marshalBody(m, v.Body)
		if v.IsAbstract {
			m["is_abstract"] = true
		}
		if v.IsFinal {
			m["is_final"] = true
		}
		if v.IsOverride {
			m["is_override"] = true
		}

	case *PropertyDecl:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.AccessModifier != AccessNone {
			m["access_modifier"] = v.AccessModifier.String()
		}
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		if v.Type != nil {
			m["type"] = nodeToMap(v.Type)
		}
		if v.Getter != nil {
			m["getter"] = nodeToMap(v.Getter)
		}
		if v.Setter != nil {
			m["setter"] = nodeToMap(v.Setter)
		}

	case *MethodSignature:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		if v.ReturnType != nil {
			m["return_type"] = nodeToMap(v.ReturnType)
		}
		marshalVarBlocks(m, v.VarBlocks)

	case *PropertySignature:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		if v.Type != nil {
			m["type"] = nodeToMap(v.Type)
		}

	case *TypeDecl:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		if v.Type != nil {
			m["type"] = nodeToMap(v.Type)
		}

	case *ActionDecl:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		marshalBody(m, v.Body)

	case *TestCaseDecl:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		m["name"] = v.Name
		marshalVarBlocks(m, v.VarBlocks)
		marshalBody(m, v.Body)

	// Statements
	case *AssignStmt:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Target != nil {
			m["target"] = nodeToMap(v.Target)
		}
		if v.Value != nil {
			m["value"] = nodeToMap(v.Value)
		}

	case *CallStmt:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Callee != nil {
			m["callee"] = nodeToMap(v.Callee)
		}

	case *IfStmt:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Condition != nil {
			m["condition"] = nodeToMap(v.Condition)
		}

	case *CaseStmt:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Expr != nil {
			m["expr"] = nodeToMap(v.Expr)
		}

	case *ForStmt:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Variable != nil {
			m["variable"] = nodeToMap(v.Variable)
		}

	case *WhileStmt:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Condition != nil {
			m["condition"] = nodeToMap(v.Condition)
		}

	case *RepeatStmt:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Condition != nil {
			m["condition"] = nodeToMap(v.Condition)
		}

	case *ReturnStmt:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)

	case *ExitStmt:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)

	case *ContinueStmt:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)

	case *EmptyStmt:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)

	// Expressions
	case *BinaryExpr:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Left != nil {
			m["left"] = nodeToMap(v.Left)
		}
		m["op"] = v.Op.Text
		if v.Right != nil {
			m["right"] = nodeToMap(v.Right)
		}

	case *UnaryExpr:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		m["op"] = v.Op.Text
		if v.Operand != nil {
			m["operand"] = nodeToMap(v.Operand)
		}

	case *Literal:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		m["lit_kind"] = v.LitKind.String()
		m["value"] = v.Value
		if v.TypePrefix != "" {
			m["type_prefix"] = v.TypePrefix
		}

	case *Ident:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		m["name"] = v.Name

	case *CallExpr:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Callee != nil {
			m["callee"] = nodeToMap(v.Callee)
		}
		if len(v.Args) > 0 {
			args := make([]interface{}, len(v.Args))
			for i, a := range v.Args {
				args[i] = nodeToMap(a)
			}
			m["args"] = args
		}

	case *MemberAccessExpr:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Object != nil {
			m["object"] = nodeToMap(v.Object)
		}
		if v.Member != nil {
			m["member"] = nodeToMap(v.Member)
		}

	case *IndexExpr:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Object != nil {
			m["object"] = nodeToMap(v.Object)
		}

	case *DerefExpr:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Operand != nil {
			m["operand"] = nodeToMap(v.Operand)
		}

	case *ParenExpr:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Inner != nil {
			m["inner"] = nodeToMap(v.Inner)
		}

	// Types
	case *NamedType:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}

	case *ArrayType:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if len(v.Ranges) > 0 {
			ranges := make([]interface{}, len(v.Ranges))
			for i, r := range v.Ranges {
				ranges[i] = nodeToMap(r)
			}
			m["ranges"] = ranges
		}
		if v.ElementType != nil {
			m["element_type"] = nodeToMap(v.ElementType)
		}

	case *SubrangeSpec:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Low != nil {
			m["low"] = nodeToMap(v.Low)
		}
		if v.High != nil {
			m["high"] = nodeToMap(v.High)
		}

	case *PointerType:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.BaseType != nil {
			m["base_type"] = nodeToMap(v.BaseType)
		}

	case *ReferenceType:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.BaseType != nil {
			m["base_type"] = nodeToMap(v.BaseType)
		}

	case *StringType:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.IsWide {
			m["is_wide"] = true
		}
		if v.Length != nil {
			m["length"] = nodeToMap(v.Length)
		}

	case *SubrangeType:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.BaseType != nil {
			m["base_type"] = nodeToMap(v.BaseType)
		}
		if v.Low != nil {
			m["low"] = nodeToMap(v.Low)
		}
		if v.High != nil {
			m["high"] = nodeToMap(v.High)
		}

	case *EnumType:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.BaseType != nil {
			m["base_type"] = nodeToMap(v.BaseType)
		}
		if len(v.Values) > 0 {
			vals := make([]interface{}, len(v.Values))
			for i, ev := range v.Values {
				vals[i] = nodeToMap(ev)
			}
			m["values"] = vals
		}

	case *EnumValue:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		if v.Value != nil {
			m["value"] = nodeToMap(v.Value)
		}

	case *StructType:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if len(v.Members) > 0 {
			members := make([]interface{}, len(v.Members))
			for i, sm := range v.Members {
				members[i] = nodeToMap(sm)
			}
			m["members"] = members
		}

	case *StructMember:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		if v.Type != nil {
			m["type"] = nodeToMap(v.Type)
		}
		if v.InitValue != nil {
			m["init_value"] = nodeToMap(v.InitValue)
		}

	// Var
	case *VarBlock:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		m["section"] = v.Section.String()
		if v.IsConstant {
			m["is_constant"] = true
		}
		if v.IsRetain {
			m["is_retain"] = true
		}
		if v.IsPersistent {
			m["is_persistent"] = true
		}
		if len(v.Declarations) > 0 {
			decls := make([]interface{}, len(v.Declarations))
			for i, d := range v.Declarations {
				decls[i] = nodeToMap(d)
			}
			m["declarations"] = decls
		}

	case *VarDecl:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if len(v.Names) > 0 {
			names := make([]interface{}, len(v.Names))
			for i, n := range v.Names {
				names[i] = nodeToMap(n)
			}
			m["names"] = names
		}
		if v.Type != nil {
			m["type"] = nodeToMap(v.Type)
		}
		if v.InitValue != nil {
			m["init_value"] = nodeToMap(v.InitValue)
		}
		if v.AtAddress != nil {
			m["at_address"] = nodeToMap(v.AtAddress)
		}

	case *CallArg:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Name != nil {
			m["name"] = nodeToMap(v.Name)
		}
		if v.Value != nil {
			m["value"] = nodeToMap(v.Value)
		}
		if v.IsOutput {
			m["is_output"] = true
		}

	case *ElsIf:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Condition != nil {
			m["condition"] = nodeToMap(v.Condition)
		}

	case *CaseBranch:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)

	case *CaseLabelValue:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Value != nil {
			m["value"] = nodeToMap(v.Value)
		}

	case *CaseLabelRange:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		if v.Low != nil {
			m["low"] = nodeToMap(v.Low)
		}
		if v.High != nil {
			m["high"] = nodeToMap(v.High)
		}

	case *ErrorNode:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		m["message"] = v.Message

	case *PragmaNode:
		m["span"] = v.NodeSpan
		marshalTrivia(m, &v.NodeBase)
		m["text"] = v.Text

	default:
		m["error"] = fmt.Sprintf("unknown node type: %T", n)
	}

	return m
}

func marshalTrivia(m map[string]interface{}, nb *NodeBase) {
	if len(nb.LeadingTrivia) > 0 {
		m["leading_trivia"] = nb.LeadingTrivia
	}
	if len(nb.TrailingTrivia) > 0 {
		m["trailing_trivia"] = nb.TrailingTrivia
	}
}

func marshalVarBlocks(m map[string]interface{}, blocks []*VarBlock) {
	if len(blocks) > 0 {
		vbs := make([]interface{}, len(blocks))
		for i, vb := range blocks {
			vbs[i] = nodeToMap(vb)
		}
		m["var_blocks"] = vbs
	}
}

func marshalBody(m map[string]interface{}, stmts []Statement) {
	if len(stmts) > 0 {
		body := make([]interface{}, len(stmts))
		for i, s := range stmts {
			body[i] = nodeToMap(s)
		}
		m["body"] = body
	}
}

// MarshalJSON implements json.Marshaler for NodeKind (serializes as string).
func (k NodeKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// MarshalJSON implements json.Marshaler for VarSection (serializes as keyword string).
func (v VarSection) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

// MarshalJSON implements json.Marshaler for AccessModifier.
func (a AccessModifier) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// MarshalJSON implements json.Marshaler for LiteralKind.
func (k LiteralKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// MarshalJSON implements json.Marshaler for TriviaKind.
func (k TriviaKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}
