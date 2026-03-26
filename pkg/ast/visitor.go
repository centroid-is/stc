package ast

// Visitor defines the interface for the visitor pattern over AST nodes.
// Visit is called for each node. If it returns a non-nil Visitor, Walk
// descends into the node's children with that visitor.
type Visitor interface {
	Visit(node Node) Visitor
}

// Walk traverses the AST rooted at node using the visitor pattern.
// It calls v.Visit(node). If Visit returns a non-nil Visitor, Walk
// recursively visits each child of node with the returned visitor.
func Walk(v Visitor, node Node) {
	if node == nil {
		return
	}
	w := v.Visit(node)
	if w == nil {
		return
	}
	for _, child := range node.Children() {
		Walk(w, child)
	}
}

// Inspect traverses the AST rooted at node, calling f for each node.
// If f returns true, Inspect descends into the node's children.
func Inspect(node Node, f func(Node) bool) {
	if node == nil {
		return
	}
	if !f(node) {
		return
	}
	for _, child := range node.Children() {
		Inspect(child, f)
	}
}
