// Package opcuatest holds client-side helpers shared by the pkg/opcua tests
// and the stc serve exec tests. It depends only on the awcullen client and
// ua packages, never on pkg/opcua, so either side can import it.
package opcuatest

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
)

// PLCNamespace is the namespace index the snapshot follows.
const PLCNamespace = 4

// Node is one browsed node of the snapshot.
type Node struct {
	NodeID          string   `json:"nodeId"`
	Parent          string   `json:"parent,omitempty"`
	Reference       string   `json:"reference,omitempty"`
	NodeClass       string   `json:"nodeClass"`
	BrowseName      string   `json:"browseName"`
	DataType        string   `json:"dataType,omitempty"`
	ValueRank       *int32   `json:"valueRank,omitempty"`
	ArrayDimensions []uint32 `json:"arrayDimensions,omitempty"`
	AccessLevel     *uint8   `json:"accessLevel,omitempty"`
	Description     string   `json:"description,omitempty"`
}

// Field is one member of a structured DataType.
type Field struct {
	Name            string   `json:"name"`
	DataType        string   `json:"dataType"`
	ValueRank       int32    `json:"valueRank"`
	ArrayDimensions []uint32 `json:"arrayDimensions,omitempty"`
}

// EnumValue is one entry of an Enumeration DataType.
type EnumValue struct {
	Value int64  `json:"value"`
	Name  string `json:"name"`
}

// DataType is a custom DataType referenced by a snapshot node.
type DataType struct {
	NodeID string      `json:"nodeId"`
	Kind   string      `json:"kind"` // "structure" or "enumeration"
	Fields []Field     `json:"fields,omitempty"`
	Values []EnumValue `json:"values,omitempty"`
}

// Snapshot is the browse result, sorted by NodeId text.
type Snapshot struct {
	Root      string     `json:"root"`
	Nodes     []Node     `json:"nodes"`
	DataTypes []DataType `json:"dataTypes"`
}

var refNames = map[string]string{
	str(ua.ReferenceTypeIDOrganizes):    "Organizes",
	str(ua.ReferenceTypeIDHasComponent): "HasComponent",
	str(ua.ReferenceTypeIDHasProperty):  "HasProperty",
}

var classNames = map[ua.NodeClass]string{
	ua.NodeClassObject:   "Object",
	ua.NodeClassVariable: "Variable",
	ua.NodeClassMethod:   "Method",
	ua.NodeClassDataType: "DataType",
}

func nsOf(id ua.NodeID) uint16 {
	switch n := id.(type) {
	case ua.NodeIDNumeric:
		return n.NamespaceIndex
	case ua.NodeIDString:
		return n.NamespaceIndex
	case ua.NodeIDGUID:
		return n.NamespaceIndex
	case ua.NodeIDOpaque:
		return n.NamespaceIndex
	}
	return 0
}

func str(id ua.NodeID) string {
	if id == nil {
		return ""
	}
	return fmt.Sprint(id)
}

// BrowseSnapshot walks forward HierarchicalReferences from root, following
// only targets in namespace 4, and returns the indented JSON (with a
// trailing newline) of every node's NodeClass, BrowseName, DataType,
// ValueRank, ArrayDimensions, AccessLevel and Description plus the
// definitions of the namespace-4 DataTypes they use. Values and encoding
// ids are left out, so the output depends only on the address-space shape.
func BrowseSnapshot(ctx context.Context, c *client.Client, root ua.NodeID) ([]byte, error) {
	snap, err := Take(ctx, c, root)
	if err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// Take builds the Snapshot BrowseSnapshot marshals.
func Take(ctx context.Context, c *client.Client, root ua.NodeID) (*Snapshot, error) {
	snap := &Snapshot{Root: str(root), Nodes: []Node{}, DataTypes: []DataType{}}
	seen := map[string]bool{str(root): true}
	type item struct {
		id     ua.NodeID
		parent string
		ref    string
	}
	queue := []item{{id: root}}
	var dataTypes []ua.NodeID
	dtSeen := map[string]bool{}
	addDT := func(id ua.NodeID) {
		if id != nil && nsOf(id) == PLCNamespace && !dtSeen[str(id)] {
			dtSeen[str(id)] = true
			dataTypes = append(dataTypes, id)
		}
	}
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		n, err := readNode(ctx, c, it.id)
		if err != nil {
			return nil, err
		}
		n.Parent, n.Reference = it.parent, it.ref
		if n.DataType != "" {
			addDT(ua.ParseNodeID(n.DataType))
		}
		snap.Nodes = append(snap.Nodes, n)
		refs, err := browse(ctx, c, it.id)
		if err != nil {
			return nil, err
		}
		for _, r := range refs {
			target := ua.ToNodeID(r.NodeID, nil)
			key := str(target)
			if target == nil || nsOf(target) != PLCNamespace || seen[key] {
				continue
			}
			seen[key] = true
			rn := refNames[str(r.ReferenceTypeID)]
			if rn == "" {
				rn = str(r.ReferenceTypeID)
			}
			queue = append(queue, item{id: target, parent: str(it.id), ref: rn})
		}
	}
	for i := 0; i < len(dataTypes); i++ {
		dt, nested, err := readDataType(ctx, c, dataTypes[i])
		if err != nil {
			return nil, err
		}
		for _, id := range nested {
			addDT(id)
		}
		snap.DataTypes = append(snap.DataTypes, dt)
	}
	sort.Slice(snap.Nodes, func(i, j int) bool { return snap.Nodes[i].NodeID < snap.Nodes[j].NodeID })
	sort.Slice(snap.DataTypes, func(i, j int) bool { return snap.DataTypes[i].NodeID < snap.DataTypes[j].NodeID })
	return snap, nil
}

func readNode(ctx context.Context, c *client.Client, id ua.NodeID) (Node, error) {
	attrs := []uint32{ua.AttributeIDNodeClass, ua.AttributeIDBrowseName, ua.AttributeIDDataType,
		ua.AttributeIDValueRank, ua.AttributeIDArrayDimensions, ua.AttributeIDAccessLevel, ua.AttributeIDDescription}
	req := &ua.ReadRequest{}
	for _, a := range attrs {
		req.NodesToRead = append(req.NodesToRead, ua.ReadValueID{NodeID: id, AttributeID: a})
	}
	res, err := c.Read(ctx, req)
	if err != nil {
		return Node{}, fmt.Errorf("read %v: %w", id, err)
	}
	if len(res.Results) != len(attrs) || res.Results[0].StatusCode.IsBad() {
		return Node{}, fmt.Errorf("read %v: unexpected result %+v", id, res.Results)
	}
	n := Node{NodeID: str(id)}
	good := func(i int) (any, bool) {
		r := res.Results[i]
		return r.Value, r.StatusCode.IsGood() && r.Value != nil
	}
	if v, ok := good(0); ok {
		nc, _ := v.(int32)
		n.NodeClass = classNames[ua.NodeClass(nc)]
		if n.NodeClass == "" {
			n.NodeClass = fmt.Sprint(nc)
		}
	}
	if v, ok := good(1); ok {
		if qn, ok := v.(ua.QualifiedName); ok {
			n.BrowseName = fmt.Sprintf("%d:%s", qn.NamespaceIndex, qn.Name)
		}
	}
	if v, ok := good(2); ok {
		if dt, ok := v.(ua.NodeID); ok {
			n.DataType = str(dt)
		}
	}
	if v, ok := good(3); ok {
		if vr, ok := v.(int32); ok {
			n.ValueRank = &vr
		}
	}
	if v, ok := good(4); ok {
		if dims, ok := v.([]uint32); ok && len(dims) > 0 {
			n.ArrayDimensions = dims
		}
	}
	if v, ok := good(5); ok {
		if al, ok := v.(uint8); ok {
			n.AccessLevel = &al
		}
	}
	if v, ok := good(6); ok {
		if d, ok := v.(ua.LocalizedText); ok {
			n.Description = d.Text
		}
	}
	return n, nil
}

func browse(ctx context.Context, c *client.Client, id ua.NodeID) ([]ua.ReferenceDescription, error) {
	res, err := c.Browse(ctx, &ua.BrowseRequest{NodesToBrowse: []ua.BrowseDescription{{
		NodeID: id, BrowseDirection: ua.BrowseDirectionForward,
		ReferenceTypeID: ua.ReferenceTypeIDHierarchicalReferences, IncludeSubtypes: true,
		ResultMask: uint32(ua.BrowseResultMaskAll),
	}}})
	if err != nil {
		return nil, fmt.Errorf("browse %v: %w", id, err)
	}
	if len(res.Results) != 1 || res.Results[0].StatusCode.IsBad() {
		return nil, fmt.Errorf("browse %v: unexpected result %+v", id, res.Results)
	}
	refs := append([]ua.ReferenceDescription(nil), res.Results[0].References...)
	cp := res.Results[0].ContinuationPoint
	for len(cp) > 0 {
		next, err := c.BrowseNext(ctx, &ua.BrowseNextRequest{ContinuationPoints: []ua.ByteString{cp}})
		if err != nil {
			return nil, fmt.Errorf("browse next %v: %w", id, err)
		}
		if len(next.Results) != 1 || next.Results[0].StatusCode.IsBad() {
			return nil, fmt.Errorf("browse next %v: unexpected result %+v", id, next.Results)
		}
		refs = append(refs, next.Results[0].References...)
		cp = next.Results[0].ContinuationPoint
	}
	return refs, nil
}

// readDataType reads the definition of a custom DataType and returns the
// namespace-4 DataTypes its fields use.
func readDataType(ctx context.Context, c *client.Client, id ua.NodeID) (DataType, []ua.NodeID, error) {
	res, err := c.Read(ctx, &ua.ReadRequest{NodesToRead: []ua.ReadValueID{{NodeID: id, AttributeID: ua.AttributeIDDataTypeDefinition}}})
	if err != nil {
		return DataType{}, nil, fmt.Errorf("read definition %v: %w", id, err)
	}
	if len(res.Results) != 1 || res.Results[0].StatusCode.IsBad() {
		return DataType{}, nil, fmt.Errorf("read definition %v: unexpected result %+v", id, res.Results)
	}
	dt := DataType{NodeID: str(id)}
	var nested []ua.NodeID
	val := res.Results[0].Value
	switch d := val.(type) {
	case ua.StructureDefinition:
		val = &d
	case ua.EnumDefinition:
		val = &d
	}
	switch d := val.(type) {
	case *ua.StructureDefinition:
		dt.Kind = "structure"
		for _, f := range d.Fields {
			dt.Fields = append(dt.Fields, Field{Name: f.Name, DataType: str(f.DataType), ValueRank: f.ValueRank, ArrayDimensions: f.ArrayDimensions})
			nested = append(nested, f.DataType)
		}
	case *ua.EnumDefinition:
		dt.Kind = "enumeration"
		for _, f := range d.Fields {
			dt.Values = append(dt.Values, EnumValue{Value: f.Value, Name: f.Name})
		}
	default:
		return DataType{}, nil, fmt.Errorf("definition of %v has type %T", id, d)
	}
	return dt, nested, nil
}
