package opcua

import "github.com/centroid-is/stc/pkg/types"

// NodeClass is the OPC UA class of a published node.
type NodeClass int

const (
	NodeObject NodeClass = iota
	NodeVariable
)

// AccessLevel is the OPC UA AccessLevel of a Variable (CurrentRead=1,
// CurrentWrite=2).
type AccessLevel uint8

const (
	AccessRead      AccessLevel = 1
	AccessWrite     AccessLevel = 2
	AccessReadWrite             = AccessRead | AccessWrite
)

// effective returns the access level with 0 meaning read/write.
func (a AccessLevel) effective() AccessLevel {
	if a == 0 {
		return AccessReadWrite
	}
	return a
}

// NodeSpec describes one node to publish. Plan 28-03's builder produces
// them from the symbol tree; Publish decides how each one reads and writes.
type NodeSpec struct {
	Path        string // NodeId string "GVL.fb[2].HMI.p_stat_State"
	Name        string // BrowseName/DisplayName text
	Parent      string // parent NodeSpec.Path; "" = attach to the Space root
	Class       NodeClass
	Type        types.Type  // Variables: IEC type; Objects: optional (FB/struct type)
	Structured  bool        // Variable of the struct's custom DataType (Type is *types.StructType)
	Access      AccessLevel // Variables only; 0 means AccessReadWrite
	Description string
}

// Space is the address space to publish.
type Space struct {
	Nodes []NodeSpec // parents before children, declaration order
}

// Find returns the NodeSpec with the given Path.
func (sp *Space) Find(path string) (NodeSpec, bool) {
	for _, n := range sp.Nodes {
		if n.Path == path {
			return n, true
		}
	}
	return NodeSpec{}, false
}
