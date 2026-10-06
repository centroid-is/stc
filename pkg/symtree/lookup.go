package symtree

// Children returns the child nodes in declaration order. For a bounded
// array it synthesises the elements Low..High on each call; an unbounded
// or multi-dimensional array has no children.
func (n *Node) Children() []*Node {
	if n.Kind != KindArray {
		return n.children
	}
	if !n.Bounded || n.elem == nil || n.High < n.Low {
		return nil
	}
	out := make([]*Node, 0, n.High-n.Low+1)
	for i := n.Low; i <= n.High; i++ {
		out = append(out, n.element(i))
	}
	return out
}

// element builds array element i without checking the bounds.
func (n *Node) element(i int) *Node {
	return n.elem("[" + itoa(i) + "]")
}
