package opcua

import (
	"errors"
	"fmt"
	"strings"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/types"
)

// maxBuildDepth bounds the symbol tree depth Build descends (T-28-11).
const maxBuildDepth = 64

// Build diagnostic codes.
const (
	CodeUnknownAttrValue = "OPCUA001" // OPC.UA.DA or .Access value not understood; treated as unset
	CodePointerSkipped   = "OPCUA002" // exposed POINTER / REFERENCE symbol is not published
	CodeUnsupported      = "OPCUA003" // exposed symbol with no OPC UA mapping (e.g. non-constant array bounds)
	CodeStructuredOnFB   = "OPCUA004" // StructuredType on an FB instance is ignored
	CodeNotStructurable  = "OPCUA005" // struct cannot be a structured DataType
	CodeUnreadable       = "OPCUA006" // published Variable unknown to the NodeSource
	CodeDuplicateNodeID  = "OPCUA007" // NodeId collides case-insensitively with an earlier one or PLC1
	CodeTooDeep          = "OPCUA008" // symbol tree deeper than maxBuildDepth
)

// plc1Name is the device object the top-level nodes hang under; a GVL or
// PROGRAM with this name would collide with its NodeId.
const plc1Name = "PLC1"

type builder struct {
	diags []diag.Diagnostic
	seen  map[string]bool // upper-cased paths already visited
}

func (b *builder) report(sev diag.Severity, code, format string, args ...any) {
	b.diags = append(b.diags, diag.Diagnostic{Severity: sev, Code: code, Message: fmt.Sprintf(format, args...)})
}

// Build decides TF6100 exposure for the symbol tree under root and returns
// the Space to publish, parents before children in declaration order.
//
// A node is published when its own OPC.UA.DA is '1' or '2', when an
// enclosing node is exposed with '1' (inheritance), or when a type-level
// attribute (delivered first in Attributes) marks it. '0' prunes the node
// and its subtree; '2' publishes a struct as a single Variable. GVLs, FB
// instances, struct instances without StructuredType and arrays of
// structs/FBs are Objects, created when exposed or when any descendant is.
// GVL and PROGRAM nodes have Parent "" and attach under Objects/DeviceSet/PLC1.
//
// src, when non-nil, is only used to check that published Variables are
// readable: unknown paths produce OPCUA006 warnings. Diagnostics carry no
// source position (SymbolNode has none) and are in walk order.
func Build(root SymbolNode, src NodeSource) (*Space, []diag.Diagnostic) {
	sp := &Space{}
	if root == nil {
		return sp, nil
	}
	b := &builder{seen: map[string]bool{}}
	for _, top := range root.Children() {
		if top == nil {
			continue
		}
		if strings.EqualFold(top.Name(), plc1Name) {
			b.report(diag.Warning, CodeDuplicateNodeID, "%s: name collides with the device object ns=4;s=%s; not published", top.Path(), plc1Name)
			continue
		}
		sp.Nodes = append(sp.Nodes, b.visit(top, "", false, false, 1)...)
	}
	if src != nil {
		for _, n := range sp.Nodes {
			if n.Class != NodeVariable || n.Structured {
				continue
			}
			if _, err := src.Read(n.Path); errors.Is(err, ErrUnknownSymbol) {
				b.report(diag.Warning, CodeUnreadable, "%s: published but unknown to the value source", n.Path)
			}
		}
	}
	return sp, b.diags
}

// daValue returns the effective OPC.UA.DA mark of n: "0", "1", "2" or "".
func (b *builder) daValue(n SymbolNode) string {
	v, ok := attr(n, attrDA)
	if !ok {
		return ""
	}
	switch v = strings.TrimSpace(v); v {
	case "0", "1", "2":
		return v
	}
	b.report(diag.Warning, CodeUnknownAttrValue, "%s: %s value %q is not '0', '1' or '2'; ignored", n.Path(), attrDA, v)
	return ""
}

// access maps OPC.UA.DA.Access: '1' read, '2' write, '3' or absent
// read/write. It is never inherited.
func (b *builder) access(n SymbolNode) AccessLevel {
	v, ok := attr(n, attrAccess)
	if !ok {
		return AccessReadWrite
	}
	switch v = strings.TrimSpace(v); v {
	case "1":
		return AccessRead
	case "2":
		return AccessWrite
	case "3":
		return AccessReadWrite
	}
	b.report(diag.Warning, CodeUnknownAttrValue, "%s: %s value %q is not '1', '2' or '3'; using read/write", n.Path(), attrAccess, v)
	return AccessReadWrite
}

func isIndirect(t types.Type) bool {
	switch t.(type) {
	case *types.PointerType, *types.ReferenceType:
		return true
	}
	return false
}

// variable fills the Variable fields of spec.
func (b *builder) variable(n SymbolNode, spec NodeSpec, structured bool) []NodeSpec {
	spec.Class, spec.Type, spec.Structured, spec.Access = NodeVariable, n.Type(), structured, b.access(n)
	return []NodeSpec{spec}
}

// visit returns the specs of n's subtree (n first) or nil when nothing in
// it is published. inherited is true below a node exposed with '1';
// inStructured is true for members of a structured Variable, whose struct
// members and array-of-struct elements must be structured too.
func (b *builder) visit(n SymbolNode, parent string, inherited, inStructured bool, depth int) []NodeSpec {
	if n == nil {
		return nil
	}
	path := n.Path()
	if depth > maxBuildDepth {
		b.report(diag.Warning, CodeTooDeep, "%s: symbol tree deeper than %d levels; subtree not published", path, maxBuildDepth)
		return nil
	}
	key := strings.ToUpper(path)
	if b.seen[key] {
		b.report(diag.Warning, CodeDuplicateNodeID, "%s: NodeId collides (case-insensitively) with an earlier symbol; not published", path)
		return nil
	}
	b.seen[key] = true

	mark := b.daValue(n)
	if mark == "0" {
		return nil
	}
	exposed := inherited || mark != ""
	spec := NodeSpec{Path: path, Name: n.Name(), Parent: parent, Class: NodeObject, Type: n.Type()}
	if d, ok := attr(n, attrDescription); ok {
		spec.Description = unescapeIEC(d)
	}
	structuredAttr := func() bool {
		v, _ := attr(n, attrStructured)
		return strings.TrimSpace(v) == "1"
	}

	typ := n.Type()
	switch kind := n.Kind(); {
	case kind == KindPointer || kind == KindReference || isIndirect(typ):
		if exposed {
			b.report(diag.Info, CodePointerSkipped, "%s: %s is not published", path, typeName(typ))
		}
		return nil

	case kind == KindFBInstance:
		if structuredAttr() {
			b.report(diag.Warning, CodeStructuredOnFB, "%s: %s on a function block instance is ignored; FB instances are Objects", path, attrStructured)
		}
		return b.container(n, spec, exposed, false, depth)

	case kind == KindStruct:
		st, _ := typ.(*types.StructType)
		eligible := func() error {
			if st == nil {
				return fmt.Errorf("%s is not a struct type", typeName(typ))
			}
			return structEligible(st)
		}
		if mark == "2" {
			if err := eligible(); err != nil {
				b.report(diag.Warning, CodeNotStructurable, "%s: '2' needs a structured DataType: %v; not published", path, err)
				return nil
			}
			return b.variable(n, spec, true)
		}
		structured := inStructured
		if !structured && structuredAttr() {
			if err := eligible(); err != nil {
				b.report(diag.Warning, CodeNotStructurable, "%s: %v; published as an Object", path, err)
			} else {
				structured = true
			}
		}
		if structured {
			spec.Class, spec.Structured, spec.Access = NodeVariable, true, b.access(n)
		}
		return b.container(n, spec, exposed, structured, depth)

	case kind == KindArray:
		at, ok := typ.(*types.ArrayType)
		if !ok {
			if exposed {
				b.report(diag.Warning, CodeUnsupported, "%s: array symbol of type %s; not published", path, typeName(typ))
			}
			return nil
		}
		if isIndirect(at.ElementType) {
			if exposed {
				b.report(diag.Info, CodePointerSkipped, "%s: array of %s is not published", path, typeName(at.ElementType))
			}
			return nil
		}
		if _, err := arrayLen(at); err != nil {
			if exposed {
				b.report(diag.Warning, CodeUnsupported, "%s: %v; not published", path, err)
			}
			return nil
		}
		if err := arrayServable(at); err != nil {
			if exposed {
				b.report(diag.Warning, CodeUnsupported, "%s: %v; not published", path, err)
			}
			return nil
		}
		if _, leaf := uaGoType(at); leaf {
			if !exposed {
				return nil
			}
			if et, ok := at.ElementType.(*types.EnumType); ok {
				if err := validateEnum(et); err != nil {
					b.report(diag.Warning, CodeUnsupported, "%s: %v; not published", path, err)
					return nil
				}
			}
			return b.variable(n, spec, false)
		}
		return b.container(n, spec, exposed, inStructured, depth)

	case kind == KindScalar || kind == KindEnum:
		if !exposed {
			return nil
		}
		if et, ok := typ.(*types.EnumType); ok {
			if err := validateEnum(et); err != nil {
				b.report(diag.Warning, CodeUnsupported, "%s: %v; not published", path, err)
				return nil
			}
		} else if _, ok := uaDataType(typ); !ok {
			b.report(diag.Warning, CodeUnsupported, "%s: %s has no OPC UA DataType; not published", path, typeName(typ))
			return nil
		}
		return b.variable(n, spec, false)
	}
	// GVL, PROGRAM and any other grouping node.
	return b.container(n, spec, exposed, false, depth)
}

// container emits spec followed by its published descendants, or nothing
// when it is not exposed and no descendant is published.
func (b *builder) container(n SymbolNode, spec NodeSpec, exposed, childStructured bool, depth int) []NodeSpec {
	var kids []NodeSpec
	for _, c := range n.Children() {
		kids = append(kids, b.visit(c, spec.Path, exposed, childStructured, depth+1)...)
	}
	if !exposed && len(kids) == 0 {
		return nil
	}
	return append([]NodeSpec{spec}, kids...)
}
