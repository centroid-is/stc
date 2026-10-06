package ast

// NamedType represents a simple type reference (e.g., BOOL, INT, FB_Motor).
// Namespace is set for a library-qualified name such as
// Tc2_EtherCAT.ST_EcSlaveState.
type NamedType struct {
	NodeBase
	Namespace *Ident `json:"namespace,omitempty"`
	Name      *Ident `json:"name"`
}

func (n *NamedType) Children() []Node {
	var nodes []Node
	if n.Namespace != nil {
		nodes = append(nodes, n.Namespace)
	}
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	return nodes
}
func (n *NamedType) typeSpecNode() {}

// ArrayType represents an ARRAY[range] OF element_type declaration.
type ArrayType struct {
	NodeBase
	Ranges      []*SubrangeSpec `json:"ranges"`
	ElementType TypeSpec        `json:"element_type"`
}

func (n *ArrayType) Children() []Node {
	var nodes []Node
	for _, r := range n.Ranges {
		nodes = append(nodes, r)
	}
	if n.ElementType != nil {
		nodes = append(nodes, n.ElementType)
	}
	return nodes
}
func (n *ArrayType) typeSpecNode() {}

// SubrangeSpec represents an array dimension range (e.g., 0..9).
type SubrangeSpec struct {
	NodeBase
	Low  Expr `json:"low"`
	High Expr `json:"high"`
}

func (n *SubrangeSpec) Children() []Node {
	var nodes []Node
	if n.Low != nil {
		nodes = append(nodes, n.Low)
	}
	if n.High != nil {
		nodes = append(nodes, n.High)
	}
	return nodes
}

// PointerType represents a POINTER TO base_type declaration.
type PointerType struct {
	NodeBase
	BaseType TypeSpec `json:"base_type"`
}

func (n *PointerType) Children() []Node {
	if n.BaseType != nil {
		return []Node{n.BaseType}
	}
	return nil
}
func (n *PointerType) typeSpecNode() {}

// ReferenceType represents a REFERENCE TO base_type declaration.
type ReferenceType struct {
	NodeBase
	BaseType TypeSpec `json:"base_type"`
}

func (n *ReferenceType) Children() []Node {
	if n.BaseType != nil {
		return []Node{n.BaseType}
	}
	return nil
}
func (n *ReferenceType) typeSpecNode() {}

// StringType represents a STRING(length) or WSTRING(length) type.
type StringType struct {
	NodeBase
	IsWide bool `json:"is_wide,omitempty"`
	Length Expr `json:"length,omitempty"`
}

func (n *StringType) Children() []Node {
	if n.Length != nil {
		return []Node{n.Length}
	}
	return nil
}
func (n *StringType) typeSpecNode() {}

// SubrangeType represents a subrange type (e.g., INT(0..100)).
type SubrangeType struct {
	NodeBase
	BaseType TypeSpec `json:"base_type"`
	Low      Expr     `json:"low"`
	High     Expr     `json:"high"`
}

func (n *SubrangeType) Children() []Node {
	var nodes []Node
	if n.BaseType != nil {
		nodes = append(nodes, n.BaseType)
	}
	if n.Low != nil {
		nodes = append(nodes, n.Low)
	}
	if n.High != nil {
		nodes = append(nodes, n.High)
	}
	return nodes
}
func (n *SubrangeType) typeSpecNode() {}

// EnumType represents an enumeration type declaration.
type EnumType struct {
	NodeBase
	BaseType TypeSpec     `json:"base_type,omitempty"`
	Values   []*EnumValue `json:"values"`
	// EndAttributes and EndPragmas sit after the last value, just before
	// the closing parenthesis.
	EndAttributes []*Attribute  `json:"end_attributes,omitempty"`
	EndPragmas    []*PragmaNode `json:"end_pragmas,omitempty"`
}

func (n *EnumType) Children() []Node {
	var nodes []Node
	if n.BaseType != nil {
		nodes = append(nodes, n.BaseType)
	}
	for _, v := range n.Values {
		nodes = append(nodes, v)
	}
	return appendAttrs(nodes, n.EndAttributes, n.EndPragmas)
}
func (n *EnumType) typeSpecNode() {}

// EnumValue represents a single member of an enumeration with optional init value.
type EnumValue struct {
	NodeBase
	Name       *Ident        `json:"name"`
	Value      Expr          `json:"value,omitempty"`
	Attributes []*Attribute  `json:"attributes,omitempty"`
	Pragmas    []*PragmaNode `json:"pragmas,omitempty"`
}

func (n *EnumValue) Children() []Node {
	var nodes []Node
	nodes = appendAttrs(nodes, n.Attributes, n.Pragmas)
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	if n.Value != nil {
		nodes = append(nodes, n.Value)
	}
	return nodes
}

// StructType represents a STRUCT...END_STRUCT type declaration.
type StructType struct {
	NodeBase
	Members []*StructMember `json:"members"`
	// EndAttributes and EndPragmas sit after the last member, just before
	// END_STRUCT.
	EndAttributes []*Attribute  `json:"end_attributes,omitempty"`
	EndPragmas    []*PragmaNode `json:"end_pragmas,omitempty"`
}

func (n *StructType) Children() []Node {
	nodes := make([]Node, 0, len(n.Members))
	for _, m := range n.Members {
		nodes = append(nodes, m)
	}
	return appendAttrs(nodes, n.EndAttributes, n.EndPragmas)
}
func (n *StructType) typeSpecNode() {}

// StructMember represents a single member of a STRUCT.
type StructMember struct {
	NodeBase
	Name      *Ident   `json:"name"`
	Type      TypeSpec `json:"type"`
	InitValue Expr     `json:"init_value,omitempty"`
	// AtAddress holds a direct address such as %I* or %QX0.0 on a member.
	AtAddress  *Ident        `json:"at_address,omitempty"`
	Attributes []*Attribute  `json:"attributes,omitempty"`
	Pragmas    []*PragmaNode `json:"pragmas,omitempty"`
}

func (n *StructMember) Children() []Node {
	var nodes []Node
	nodes = appendAttrs(nodes, n.Attributes, n.Pragmas)
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	if n.Type != nil {
		nodes = append(nodes, n.Type)
	}
	if n.InitValue != nil {
		nodes = append(nodes, n.InitValue)
	}
	if n.AtAddress != nil {
		nodes = append(nodes, n.AtAddress)
	}
	return nodes
}
