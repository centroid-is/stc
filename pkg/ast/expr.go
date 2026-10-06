package ast

// Token is a lightweight token reference used in AST nodes for operators.
type Token struct {
	Kind int    `json:"kind"`
	Text string `json:"text"`
	Span Span   `json:"span"`
}

// BinaryExpr represents a binary operator expression (e.g., a + b, x AND y).
type BinaryExpr struct {
	NodeBase
	Left  Expr  `json:"left"`
	Op    Token `json:"op"`
	Right Expr  `json:"right"`
}

func (n *BinaryExpr) Children() []Node {
	var nodes []Node
	if n.Left != nil {
		nodes = append(nodes, n.Left)
	}
	if n.Right != nil {
		nodes = append(nodes, n.Right)
	}
	return nodes
}
func (n *BinaryExpr) exprNode() {}

// UnaryExpr represents a unary operator expression (e.g., NOT x, -y).
type UnaryExpr struct {
	NodeBase
	Op      Token `json:"op"`
	Operand Expr  `json:"operand"`
}

func (n *UnaryExpr) Children() []Node {
	if n.Operand != nil {
		return []Node{n.Operand}
	}
	return nil
}
func (n *UnaryExpr) exprNode() {}

// LiteralKind identifies the type of a literal value.
type LiteralKind int

const (
	LitInt      LiteralKind = iota // Integer literal (e.g., 42, 16#FF)
	LitReal                        // Real literal (e.g., 3.14)
	LitString                      // String literal (e.g., 'hello')
	LitWString                     // Wide string literal (e.g., "hello")
	LitBool                        // Boolean literal (TRUE, FALSE)
	LitTime                        // Time literal (e.g., T#5s)
	LitDate                        // Date literal (e.g., D#2024-01-01)
	LitDateTime                    // Date and time literal (e.g., DT#2024-01-01-12:00:00)
	LitTod                         // Time of day literal (e.g., TOD#12:00:00)
	LitTyped                       // Typed literal (e.g., INT#5, UINT#10)
)

var literalKindNames = [...]string{
	LitInt:      "Int",
	LitReal:     "Real",
	LitString:   "String",
	LitWString:  "WString",
	LitBool:     "Bool",
	LitTime:     "Time",
	LitDate:     "Date",
	LitDateTime: "DateTime",
	LitTod:      "Tod",
	LitTyped:    "Typed",
}

// String returns the human-readable name of a LiteralKind.
func (k LiteralKind) String() string {
	if int(k) < len(literalKindNames) {
		return literalKindNames[k]
	}
	return "Unknown"
}

// Literal represents a literal value in an expression.
type Literal struct {
	NodeBase
	LitKind    LiteralKind `json:"lit_kind"`
	Value      string      `json:"value"`
	TypePrefix string      `json:"type_prefix,omitempty"`
}

func (n *Literal) Children() []Node { return nil }
func (n *Literal) exprNode()        {}

// CallExpr represents a function call in expression position.
//
// Args holds the leading positional arguments. NamedArgs holds everything
// from the first named argument (name := v or name => v) on, in source
// order; a positional argument that follows a named one is stored there
// as a CallArg with a nil Name.
type CallExpr struct {
	NodeBase
	Callee    Expr       `json:"callee"`
	Args      []Expr     `json:"args,omitempty"`
	NamedArgs []*CallArg `json:"named_args,omitempty"`
}

func (n *CallExpr) Children() []Node {
	var nodes []Node
	if n.Callee != nil {
		nodes = append(nodes, n.Callee)
	}
	for _, a := range n.Args {
		nodes = append(nodes, a)
	}
	for _, a := range n.NamedArgs {
		nodes = append(nodes, a)
	}
	return nodes
}
func (n *CallExpr) exprNode() {}

// MemberAccessExpr represents a dot-access expression (e.g., obj.field).
type MemberAccessExpr struct {
	NodeBase
	Object Expr   `json:"object"`
	Member *Ident `json:"member"`
}

func (n *MemberAccessExpr) Children() []Node {
	var nodes []Node
	if n.Object != nil {
		nodes = append(nodes, n.Object)
	}
	if n.Member != nil {
		nodes = append(nodes, n.Member)
	}
	return nodes
}
func (n *MemberAccessExpr) exprNode() {}

// IndexExpr represents an array indexing expression (e.g., arr[i, j]).
type IndexExpr struct {
	NodeBase
	Object  Expr   `json:"object"`
	Indices []Expr `json:"indices"`
}

func (n *IndexExpr) Children() []Node {
	var nodes []Node
	if n.Object != nil {
		nodes = append(nodes, n.Object)
	}
	for _, idx := range n.Indices {
		nodes = append(nodes, idx)
	}
	return nodes
}
func (n *IndexExpr) exprNode() {}

// DerefExpr represents a pointer dereference expression (e.g., ptr^).
type DerefExpr struct {
	NodeBase
	Operand Expr `json:"operand"`
}

func (n *DerefExpr) Children() []Node {
	if n.Operand != nil {
		return []Node{n.Operand}
	}
	return nil
}
func (n *DerefExpr) exprNode() {}

// ParenExpr represents a parenthesized expression.
type ParenExpr struct {
	NodeBase
	Inner Expr `json:"inner"`
}

func (n *ParenExpr) Children() []Node {
	if n.Inner != nil {
		return []Node{n.Inner}
	}
	return nil
}
func (n *ParenExpr) exprNode() {}

// BitAccessExpr represents bit access on an integer or bit-string value
// (e.g., w.3, ECT.X.q_wDigitalInputs.0). Index is normally an integer
// literal; bit 0 is the least significant bit.
type BitAccessExpr struct {
	NodeBase
	Target Expr `json:"target"`
	Index  Expr `json:"index"`
}

func (n *BitAccessExpr) Children() []Node {
	var nodes []Node
	if n.Target != nil {
		nodes = append(nodes, n.Target)
	}
	if n.Index != nil {
		nodes = append(nodes, n.Index)
	}
	return nodes
}
func (n *BitAccessExpr) exprNode() {}

// ThisExpr represents the THIS keyword (a pointer to the current FB
// instance); THIS^ is a DerefExpr wrapping it.
type ThisExpr struct {
	NodeBase
}

func (n *ThisExpr) Children() []Node { return nil }
func (n *ThisExpr) exprNode()        {}

// SuperExpr represents the SUPER keyword (a pointer to the parent FB of
// an EXTENDS chain); SUPER^ is a DerefExpr wrapping it.
type SuperExpr struct {
	NodeBase
}

func (n *SuperExpr) Children() []Node { return nil }
func (n *SuperExpr) exprNode()        {}

// StructInit represents a structure initialiser such as (a := 1, b := 'x').
// It is an Expr so it can sit in any InitValue field.
type StructInit struct {
	NodeBase
	Fields []*FieldInit `json:"fields,omitempty"`
}

func (n *StructInit) Children() []Node {
	nodes := make([]Node, 0, len(n.Fields))
	for _, f := range n.Fields {
		nodes = append(nodes, f)
	}
	return nodes
}
func (n *StructInit) exprNode() {}

// FieldInit is one name := value entry of a StructInit.
type FieldInit struct {
	NodeBase
	Name  *Ident `json:"name"`
	Value Expr   `json:"value"`
}

func (n *FieldInit) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	if n.Value != nil {
		nodes = append(nodes, n.Value)
	}
	return nodes
}

// ArrayInit represents an array initialiser such as [1, 2, 3(0)].
// It is an Expr so it can sit in any InitValue field.
type ArrayInit struct {
	NodeBase
	Elements []*ArrayInitElem `json:"elements,omitempty"`
}

func (n *ArrayInit) Children() []Node {
	nodes := make([]Node, 0, len(n.Elements))
	for _, e := range n.Elements {
		nodes = append(nodes, e)
	}
	return nodes
}
func (n *ArrayInit) exprNode() {}

// ArrayInitElem is one element of an ArrayInit. Count is nil for a plain
// element and holds N for the repetition form N(value).
type ArrayInitElem struct {
	NodeBase
	Count Expr `json:"count,omitempty"`
	Value Expr `json:"value"`
}

func (n *ArrayInitElem) Children() []Node {
	var nodes []Node
	if n.Count != nil {
		nodes = append(nodes, n.Count)
	}
	if n.Value != nil {
		nodes = append(nodes, n.Value)
	}
	return nodes
}
