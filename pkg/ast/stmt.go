package ast

// AssignStmt represents a := assignment statement.
type AssignStmt struct {
	NodeBase
	Target Expr `json:"target"`
	Value  Expr `json:"value"`
}

func (n *AssignStmt) Children() []Node {
	var nodes []Node
	if n.Target != nil {
		nodes = append(nodes, n.Target)
	}
	if n.Value != nil {
		nodes = append(nodes, n.Value)
	}
	return nodes
}
func (n *AssignStmt) stmtNode() {}

// CallStmt represents a function block call statement with named arguments.
type CallStmt struct {
	NodeBase
	Callee Expr       `json:"callee"`
	Args   []*CallArg `json:"args,omitempty"`
}

func (n *CallStmt) Children() []Node {
	var nodes []Node
	if n.Callee != nil {
		nodes = append(nodes, n.Callee)
	}
	for _, a := range n.Args {
		nodes = append(nodes, a)
	}
	return nodes
}
func (n *CallStmt) stmtNode() {}

// CallArg represents a named argument in a function block call.
// IsOutput distinguishes := (input) from => (output) binding.
type CallArg struct {
	NodeBase
	Name     *Ident `json:"name,omitempty"`
	Value    Expr   `json:"value"`
	IsOutput bool   `json:"is_output,omitempty"`
}

func (n *CallArg) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	if n.Value != nil {
		nodes = append(nodes, n.Value)
	}
	return nodes
}

// IfStmt represents an IF...THEN...ELSIF...ELSE...END_IF statement.
type IfStmt struct {
	NodeBase
	Condition Expr        `json:"condition"`
	Then      []Statement `json:"then"`
	ElsIfs    []*ElsIf    `json:"elsifs,omitempty"`
	Else      []Statement `json:"else,omitempty"`
}

func (n *IfStmt) Children() []Node {
	var nodes []Node
	if n.Condition != nil {
		nodes = append(nodes, n.Condition)
	}
	for _, s := range n.Then {
		nodes = append(nodes, s)
	}
	for _, e := range n.ElsIfs {
		nodes = append(nodes, e)
	}
	for _, s := range n.Else {
		nodes = append(nodes, s)
	}
	return nodes
}
func (n *IfStmt) stmtNode() {}

// ElsIf represents an ELSIF branch of an IF statement.
type ElsIf struct {
	NodeBase
	Condition Expr        `json:"condition"`
	Body      []Statement `json:"body"`
}

func (n *ElsIf) Children() []Node {
	var nodes []Node
	if n.Condition != nil {
		nodes = append(nodes, n.Condition)
	}
	for _, s := range n.Body {
		nodes = append(nodes, s)
	}
	return nodes
}

// CaseStmt represents a CASE...OF...END_CASE statement.
type CaseStmt struct {
	NodeBase
	Expr       Expr          `json:"expr"`
	Branches   []*CaseBranch `json:"branches"`
	ElseBranch []Statement   `json:"else_branch,omitempty"`
}

func (n *CaseStmt) Children() []Node {
	var nodes []Node
	if n.Expr != nil {
		nodes = append(nodes, n.Expr)
	}
	for _, b := range n.Branches {
		nodes = append(nodes, b)
	}
	for _, s := range n.ElseBranch {
		nodes = append(nodes, s)
	}
	return nodes
}
func (n *CaseStmt) stmtNode() {}

// CaseBranch represents a single branch in a CASE statement.
type CaseBranch struct {
	NodeBase
	Labels []CaseLabel `json:"labels"`
	Body   []Statement `json:"body"`
}

func (n *CaseBranch) Children() []Node {
	var nodes []Node
	for _, l := range n.Labels {
		nodes = append(nodes, l)
	}
	for _, s := range n.Body {
		nodes = append(nodes, s)
	}
	return nodes
}

// CaseLabel is a marker interface for case label types.
type CaseLabel interface {
	Node
	caseLabelNode()
}

// CaseLabelValue represents a single value in a CASE label.
type CaseLabelValue struct {
	NodeBase
	Value Expr `json:"value"`
}

func (n *CaseLabelValue) Children() []Node {
	if n.Value != nil {
		return []Node{n.Value}
	}
	return nil
}
func (n *CaseLabelValue) caseLabelNode() {}

// CaseLabelRange represents a range (low..high) in a CASE label.
type CaseLabelRange struct {
	NodeBase
	Low  Expr `json:"low"`
	High Expr `json:"high"`
}

func (n *CaseLabelRange) Children() []Node {
	var nodes []Node
	if n.Low != nil {
		nodes = append(nodes, n.Low)
	}
	if n.High != nil {
		nodes = append(nodes, n.High)
	}
	return nodes
}
func (n *CaseLabelRange) caseLabelNode() {}

// ForStmt represents a FOR...TO...BY...DO...END_FOR loop.
type ForStmt struct {
	NodeBase
	Variable *Ident      `json:"variable"`
	From     Expr        `json:"from"`
	To       Expr        `json:"to"`
	By       Expr        `json:"by,omitempty"`
	Body     []Statement `json:"body"`
}

func (n *ForStmt) Children() []Node {
	var nodes []Node
	if n.Variable != nil {
		nodes = append(nodes, n.Variable)
	}
	if n.From != nil {
		nodes = append(nodes, n.From)
	}
	if n.To != nil {
		nodes = append(nodes, n.To)
	}
	if n.By != nil {
		nodes = append(nodes, n.By)
	}
	for _, s := range n.Body {
		nodes = append(nodes, s)
	}
	return nodes
}
func (n *ForStmt) stmtNode() {}

// WhileStmt represents a WHILE...DO...END_WHILE loop.
type WhileStmt struct {
	NodeBase
	Condition Expr        `json:"condition"`
	Body      []Statement `json:"body"`
}

func (n *WhileStmt) Children() []Node {
	var nodes []Node
	if n.Condition != nil {
		nodes = append(nodes, n.Condition)
	}
	for _, s := range n.Body {
		nodes = append(nodes, s)
	}
	return nodes
}
func (n *WhileStmt) stmtNode() {}

// RepeatStmt represents a REPEAT...UNTIL...END_REPEAT loop.
type RepeatStmt struct {
	NodeBase
	Body      []Statement `json:"body"`
	Condition Expr        `json:"condition"`
}

func (n *RepeatStmt) Children() []Node {
	var nodes []Node
	for _, s := range n.Body {
		nodes = append(nodes, s)
	}
	if n.Condition != nil {
		nodes = append(nodes, n.Condition)
	}
	return nodes
}
func (n *RepeatStmt) stmtNode() {}

// ReturnStmt represents a RETURN statement.
type ReturnStmt struct {
	NodeBase
}

func (n *ReturnStmt) Children() []Node { return nil }
func (n *ReturnStmt) stmtNode()        {}

// ExitStmt represents an EXIT statement (breaks out of loops).
type ExitStmt struct {
	NodeBase
}

func (n *ExitStmt) Children() []Node { return nil }
func (n *ExitStmt) stmtNode()        {}

// ContinueStmt represents a CONTINUE statement.
type ContinueStmt struct {
	NodeBase
}

func (n *ContinueStmt) Children() []Node { return nil }
func (n *ContinueStmt) stmtNode()        {}

// EmptyStmt represents a bare semicolon (no-op statement).
type EmptyStmt struct {
	NodeBase
}

func (n *EmptyStmt) Children() []Node { return nil }
func (n *EmptyStmt) stmtNode()        {}
