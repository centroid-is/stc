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
	Section      VarSection `json:"section"`
	IsConstant   bool       `json:"is_constant,omitempty"`
	IsRetain     bool       `json:"is_retain,omitempty"`
	IsPersistent bool       `json:"is_persistent,omitempty"`
	Declarations []*VarDecl `json:"declarations"`
}

func (n *VarBlock) Children() []Node {
	nodes := make([]Node, len(n.Declarations))
	for i, d := range n.Declarations {
		nodes[i] = d
	}
	return nodes
}

// VarDecl represents a single variable declaration (e.g., x, y : INT := 0;).
type VarDecl struct {
	NodeBase
	Names     []*Ident `json:"names"`
	Type      TypeSpec `json:"type"`
	InitValue Expr     `json:"init_value,omitempty"`
	AtAddress *Ident   `json:"at_address,omitempty"`
}

func (n *VarDecl) Children() []Node {
	var nodes []Node
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

// PragmaNode represents a {attribute '...'} pragma.
type PragmaNode struct {
	NodeBase
	Text string `json:"text"`
}

func (n *PragmaNode) Children() []Node { return nil }
