package ast

// AccessModifier represents visibility modifiers for OOP members.
type AccessModifier int

const (
	AccessNone AccessModifier = iota
	AccessPublic
	AccessPrivate
	AccessProtected
	AccessInternal
)

var accessModifierNames = [...]string{
	AccessNone:      "",
	AccessPublic:    "PUBLIC",
	AccessPrivate:   "PRIVATE",
	AccessProtected: "PROTECTED",
	AccessInternal:  "INTERNAL",
}

// String returns the ST keyword for the access modifier.
func (a AccessModifier) String() string {
	if int(a) < len(accessModifierNames) {
		return accessModifierNames[a]
	}
	return ""
}

// SourceFile is the root node representing an entire ST source file.
type SourceFile struct {
	NodeBase
	Declarations []Declaration `json:"declarations"`
}

func (n *SourceFile) Children() []Node {
	nodes := make([]Node, len(n.Declarations))
	for i, d := range n.Declarations {
		nodes[i] = d
	}
	return nodes
}
func (n *SourceFile) declNode() {}

// ProgramDecl represents a PROGRAM...END_PROGRAM declaration.
type ProgramDecl struct {
	NodeBase
	Name      *Ident      `json:"name"`
	VarBlocks []*VarBlock `json:"var_blocks,omitempty"`
	Body      []Statement `json:"body,omitempty"`
}

func (n *ProgramDecl) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	for _, v := range n.VarBlocks {
		nodes = append(nodes, v)
	}
	for _, s := range n.Body {
		nodes = append(nodes, s)
	}
	return nodes
}
func (n *ProgramDecl) declNode() {}

// FunctionBlockDecl represents a FUNCTION_BLOCK...END_FUNCTION_BLOCK declaration.
type FunctionBlockDecl struct {
	NodeBase
	Name       *Ident          `json:"name"`
	Extends    *Ident          `json:"extends,omitempty"`
	Implements []*Ident        `json:"implements,omitempty"`
	VarBlocks  []*VarBlock     `json:"var_blocks,omitempty"`
	Body       []Statement     `json:"body,omitempty"`
	Methods    []*MethodDecl   `json:"methods,omitempty"`
	Properties []*PropertyDecl `json:"properties,omitempty"`
}

func (n *FunctionBlockDecl) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	if n.Extends != nil {
		nodes = append(nodes, n.Extends)
	}
	for _, impl := range n.Implements {
		nodes = append(nodes, impl)
	}
	for _, v := range n.VarBlocks {
		nodes = append(nodes, v)
	}
	for _, s := range n.Body {
		nodes = append(nodes, s)
	}
	for _, m := range n.Methods {
		nodes = append(nodes, m)
	}
	for _, p := range n.Properties {
		nodes = append(nodes, p)
	}
	return nodes
}
func (n *FunctionBlockDecl) declNode() {}

// FunctionDecl represents a FUNCTION...END_FUNCTION declaration.
type FunctionDecl struct {
	NodeBase
	Name       *Ident      `json:"name"`
	ReturnType TypeSpec     `json:"return_type,omitempty"`
	VarBlocks  []*VarBlock `json:"var_blocks,omitempty"`
	Body       []Statement `json:"body,omitempty"`
}

func (n *FunctionDecl) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	if n.ReturnType != nil {
		nodes = append(nodes, n.ReturnType)
	}
	for _, v := range n.VarBlocks {
		nodes = append(nodes, v)
	}
	for _, s := range n.Body {
		nodes = append(nodes, s)
	}
	return nodes
}
func (n *FunctionDecl) declNode() {}

// InterfaceDecl represents an INTERFACE...END_INTERFACE declaration.
type InterfaceDecl struct {
	NodeBase
	Name       *Ident               `json:"name"`
	Extends    []*Ident             `json:"extends,omitempty"`
	Methods    []*MethodSignature   `json:"methods,omitempty"`
	Properties []*PropertySignature `json:"properties,omitempty"`
}

func (n *InterfaceDecl) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	for _, e := range n.Extends {
		nodes = append(nodes, e)
	}
	for _, m := range n.Methods {
		nodes = append(nodes, m)
	}
	for _, p := range n.Properties {
		nodes = append(nodes, p)
	}
	return nodes
}
func (n *InterfaceDecl) declNode() {}

// MethodDecl represents a METHOD...END_METHOD declaration.
type MethodDecl struct {
	NodeBase
	AccessModifier AccessModifier `json:"access_modifier,omitempty"`
	Name           *Ident         `json:"name"`
	ReturnType     TypeSpec       `json:"return_type,omitempty"`
	VarBlocks      []*VarBlock    `json:"var_blocks,omitempty"`
	Body           []Statement    `json:"body,omitempty"`
	IsAbstract     bool           `json:"is_abstract,omitempty"`
	IsFinal        bool           `json:"is_final,omitempty"`
	IsOverride     bool           `json:"is_override,omitempty"`
}

func (n *MethodDecl) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	if n.ReturnType != nil {
		nodes = append(nodes, n.ReturnType)
	}
	for _, v := range n.VarBlocks {
		nodes = append(nodes, v)
	}
	for _, s := range n.Body {
		nodes = append(nodes, s)
	}
	return nodes
}
func (n *MethodDecl) declNode() {}

// PropertyDecl represents a PROPERTY...END_PROPERTY declaration.
type PropertyDecl struct {
	NodeBase
	AccessModifier AccessModifier `json:"access_modifier,omitempty"`
	Name           *Ident         `json:"name"`
	Type           TypeSpec       `json:"type,omitempty"`
	Getter         *MethodDecl    `json:"getter,omitempty"`
	Setter         *MethodDecl    `json:"setter,omitempty"`
}

func (n *PropertyDecl) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	if n.Type != nil {
		nodes = append(nodes, n.Type)
	}
	if n.Getter != nil {
		nodes = append(nodes, n.Getter)
	}
	if n.Setter != nil {
		nodes = append(nodes, n.Setter)
	}
	return nodes
}
func (n *PropertyDecl) declNode() {}

// MethodSignature represents a method declaration inside an INTERFACE (no body).
type MethodSignature struct {
	NodeBase
	Name       *Ident      `json:"name"`
	ReturnType TypeSpec     `json:"return_type,omitempty"`
	VarBlocks  []*VarBlock `json:"var_blocks,omitempty"`
}

func (n *MethodSignature) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	if n.ReturnType != nil {
		nodes = append(nodes, n.ReturnType)
	}
	for _, v := range n.VarBlocks {
		nodes = append(nodes, v)
	}
	return nodes
}
func (n *MethodSignature) declNode() {}

// PropertySignature represents a property declaration inside an INTERFACE (no body).
type PropertySignature struct {
	NodeBase
	Name *Ident   `json:"name"`
	Type TypeSpec  `json:"type,omitempty"`
}

func (n *PropertySignature) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	if n.Type != nil {
		nodes = append(nodes, n.Type)
	}
	return nodes
}
func (n *PropertySignature) declNode() {}

// TypeDecl represents a TYPE...END_TYPE declaration.
type TypeDecl struct {
	NodeBase
	Name *Ident   `json:"name"`
	Type TypeSpec  `json:"type"`
}

func (n *TypeDecl) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	if n.Type != nil {
		nodes = append(nodes, n.Type)
	}
	return nodes
}
func (n *TypeDecl) declNode() {}

// ActionDecl represents an ACTION...END_ACTION block inside a POU.
type ActionDecl struct {
	NodeBase
	Name *Ident      `json:"name"`
	Body []Statement `json:"body,omitempty"`
}

func (n *ActionDecl) Children() []Node {
	var nodes []Node
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	for _, s := range n.Body {
		nodes = append(nodes, s)
	}
	return nodes
}
func (n *ActionDecl) declNode() {}
