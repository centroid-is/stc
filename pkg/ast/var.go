package ast

// VarSection identifies the kind of variable declaration block.
type VarSection int

const (
	VarLocal    VarSection = iota // VAR
	VarInput                      // VAR_INPUT
	VarOutput                     // VAR_OUTPUT
	VarInOut                      // VAR_IN_OUT
	VarTemp                       // VAR_TEMP
	VarGlobal                     // VAR_GLOBAL
	VarAccess                     // VAR_ACCESS
	VarExternal                   // VAR_EXTERNAL
	VarConfig                     // VAR_CONFIG
)

var varSectionNames = [...]string{
	VarLocal:    "VAR",
	VarInput:    "VAR_INPUT",
	VarOutput:   "VAR_OUTPUT",
	VarInOut:    "VAR_IN_OUT",
	VarTemp:     "VAR_TEMP",
	VarGlobal:   "VAR_GLOBAL",
	VarAccess:   "VAR_ACCESS",
	VarExternal: "VAR_EXTERNAL",
	VarConfig:   "VAR_CONFIG",
}

// String returns the IEC 61131-3 keyword for the variable section.
func (v VarSection) String() string {
	if int(v) < len(varSectionNames) {
		return varSectionNames[v]
	}
	return "VAR"
}

// VarBlock represents a variable declaration block (VAR...END_VAR).
type VarBlock struct {
	NodeBase
	Section      VarSection    `json:"section"`
	IsConstant   bool          `json:"is_constant,omitempty"`
	IsRetain     bool          `json:"is_retain,omitempty"`
	IsPersistent bool          `json:"is_persistent,omitempty"`
	Declarations []*VarDecl    `json:"declarations"`
	Attributes   []*Attribute  `json:"attributes,omitempty"`
	Pragmas      []*PragmaNode `json:"pragmas,omitempty"`
	// EndAttributes and EndPragmas sit after the last declaration, just
	// before END_VAR, and are printed there so {warning restore} or
	// {endregion} keeps covering the same lines.
	EndAttributes []*Attribute  `json:"end_attributes,omitempty"`
	EndPragmas    []*PragmaNode `json:"end_pragmas,omitempty"`
}

func (n *VarBlock) Children() []Node {
	var nodes []Node
	nodes = appendAttrs(nodes, n.Attributes, n.Pragmas)
	for _, d := range n.Declarations {
		nodes = append(nodes, d)
	}
	return appendAttrs(nodes, n.EndAttributes, n.EndPragmas)
}

// VarDecl represents a single variable declaration (e.g., x, y : INT := 0;).
type VarDecl struct {
	NodeBase
	Names      []*Ident      `json:"names"`
	Type       TypeSpec      `json:"type"`
	InitValue  Expr          `json:"init_value,omitempty"`
	AtAddress  *Ident        `json:"at_address,omitempty"`
	Attributes []*Attribute  `json:"attributes,omitempty"`
	Pragmas    []*PragmaNode `json:"pragmas,omitempty"`
}

func (n *VarDecl) Children() []Node {
	var nodes []Node
	nodes = appendAttrs(nodes, n.Attributes, n.Pragmas)
	for _, name := range n.Names {
		nodes = append(nodes, name)
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

// PragmaNode represents a non-attribute pragma such as {warning disable C0001},
// {region}, {endregion} or {text ...}, kept verbatim so fmt/emit round-trip it.
type PragmaNode struct {
	NodeBase
	Text string `json:"text"`
}

func (n *PragmaNode) Children() []Node { return nil }
