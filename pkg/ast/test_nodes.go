package ast

// TestCaseDecl represents a TEST_CASE 'name' ... END_TEST_CASE declaration.
// This is a testing-specific POU that holds assertions and test logic.
type TestCaseDecl struct {
	NodeBase
	Name      string      `json:"name"`                  // From string literal (not an Ident)
	VarBlocks []*VarBlock `json:"var_blocks,omitempty"`
	Body      []Statement `json:"body,omitempty"`
}

// Children returns all child nodes for tree traversal.
func (n *TestCaseDecl) Children() []Node {
	var nodes []Node
	for _, v := range n.VarBlocks {
		nodes = append(nodes, v)
	}
	for _, s := range n.Body {
		nodes = append(nodes, s)
	}
	return nodes
}

// declNode marks TestCaseDecl as a Declaration.
func (n *TestCaseDecl) declNode() {}
