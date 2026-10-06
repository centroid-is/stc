package opcua

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/awcullen/opcua/server"
	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/types"
)

// rootID is the node top-level NodeSpecs attach to (Organizes).
func (s *Server) rootID() ua.NodeID { return ua.ObjectIDObjectsFolder }

// statusFor maps NodeSource and conversion errors to OPC UA status codes.
func statusFor(err error) ua.StatusCode {
	switch {
	case err == nil:
		return ua.Good
	case errors.Is(err, ErrNotWritable):
		return ua.BadNotWritable
	case errors.Is(err, ErrOutOfRange):
		return ua.BadOutOfRange
	case errors.Is(err, ErrUnknownSymbol):
		return ua.BadNodeIDUnknown
	case errors.Is(err, ErrTypeMismatch):
		return ua.BadTypeMismatch
	}
	return ua.BadInternalError
}

// readStatusFor maps a read failure: out-of-range values are BadOutOfRange,
// everything else BadNoData.
func readStatusFor(err error) ua.StatusCode {
	if errors.Is(err, ErrOutOfRange) {
		return ua.BadOutOfRange
	}
	return ua.BadNoData
}

func badRead(code ua.StatusCode) ua.DataValue {
	return ua.NewDataValue(nil, code, time.Time{}, 0, time.Now(), 0)
}

func goodValue(v any) ua.DataValue {
	now := time.Now()
	return ua.NewDataValue(v, ua.Good, now, 0, now, 0)
}

// validateSpace checks that every path is unique, every parent precedes
// its children and every Variable has a type.
func validateSpace(sp *Space) error {
	seen := make(map[string]bool, len(sp.Nodes))
	for i, n := range sp.Nodes {
		switch {
		case n.Path == "":
			return fmt.Errorf("opcua: node %d has an empty path", i)
		case n.Name == "":
			return fmt.Errorf("opcua: node %s has an empty name", n.Path)
		case seen[n.Path]:
			return fmt.Errorf("opcua: duplicate node path %s", n.Path)
		case n.Parent != "" && !seen[n.Parent]:
			return fmt.Errorf("opcua: node %s: parent %s is not published before it", n.Path, n.Parent)
		case n.Class != NodeObject && n.Class != NodeVariable:
			return fmt.Errorf("opcua: node %s: unknown node class %d", n.Path, n.Class)
		case n.Class == NodeVariable && n.Type == nil:
			return fmt.Errorf("opcua: variable %s has no type", n.Path)
		}
		if n.Structured {
			if _, ok := n.Type.(*types.StructType); !ok || n.Class != NodeVariable {
				return fmt.Errorf("opcua: structured node %s must be a Variable of a struct type", n.Path)
			}
		}
		seen[n.Path] = true
	}
	return nil
}

// Publish installs sp under the Objects folder: Objects, and Variables
// whose reads and writes go through src.
func (s *Server) Publish(sp *Space, src NodeSource) error {
	if sp == nil || src == nil {
		return errors.New("opcua: Publish needs a Space and a NodeSource")
	}
	if err := validateSpace(sp); err != nil {
		return err
	}
	for _, spec := range sp.Nodes {
		// awcullen's AddNodes silently replaces an existing node.
		if _, exists := s.nm.FindNode(s.id(spec.Path)); exists {
			return fmt.Errorf("opcua: node %s is already published", spec.Path)
		}
	}
	for _, spec := range sp.Nodes {
		nodes, err := s.buildNode(sp, spec, src)
		if err != nil {
			return err
		}
		if err := s.nm.AddNodes(nodes...); err != nil {
			return fmt.Errorf("opcua: add node %s: %w", spec.Path, err)
		}
	}
	return nil
}

// parentRef links a node to its parent: Organizes from the root,
// HasComponent from any other node.
func (s *Server) parentRef(spec NodeSpec) ua.Reference {
	if spec.Parent == "" {
		return ref(ua.ReferenceTypeIDOrganizes, true, s.rootID())
	}
	return ref(ua.ReferenceTypeIDHasComponent, true, s.id(spec.Parent))
}

func (s *Server) buildNode(sp *Space, spec NodeSpec, src NodeSource) ([]server.Node, error) {
	id := s.id(spec.Path)
	if spec.Class == NodeObject {
		obj := server.NewObjectNode(s.srv, id, ua.NewQualifiedName(s.ns, spec.Name), lt(spec.Name), lt(spec.Description), nil,
			[]ua.Reference{s.parentRef(spec), ref(ua.ReferenceTypeIDHasTypeDefinition, false, ua.ObjectTypeIDBaseObjectType)}, 0)
		return []server.Node{obj}, nil
	}

	var (
		dataType ua.NodeID
		rank     = ua.ValueRankScalar
		dims     []uint32
		sdt      *structDT
		enumType *types.EnumType
		err      error
	)
	switch t := spec.Type.(type) {
	case *types.StructType:
		if !spec.Structured {
			return nil, fmt.Errorf("opcua: variable %s of struct type %s must be Structured (or an Object)", spec.Path, t.Name)
		}
		if sdt, err = s.ensureStruct(t); err != nil {
			return nil, fmt.Errorf("opcua: variable %s: %w", spec.Path, err)
		}
		dataType = sdt.DTID
	case *types.EnumType:
		enumType = t
		if dataType, err = s.ensureEnum(t); err != nil {
			return nil, fmt.Errorf("opcua: variable %s: %w", spec.Path, err)
		}
	case *types.ArrayType:
		if _, ok := uaGoType(t); !ok {
			return nil, fmt.Errorf("opcua: variable %s: %w: %s has no OPC UA array mapping", spec.Path, ErrTypeMismatch, t)
		}
		n, err := arrayLen(t)
		if err != nil {
			return nil, fmt.Errorf("opcua: variable %s: %w", spec.Path, err)
		}
		// Values travel flattened (row-major) and awcullen's write check
		// rejects flat slices on rank>1 nodes, so every array is rank 1.
		rank, dims = ua.ValueRankOneDimension, []uint32{uint32(n)}
		if et, ok := t.ElementType.(*types.EnumType); ok {
			if dataType, err = s.ensureEnum(et); err != nil {
				return nil, fmt.Errorf("opcua: variable %s: %w", spec.Path, err)
			}
		} else {
			dataType, _ = uaDataType(t)
		}
	default:
		var ok bool
		if dataType, ok = uaDataType(t); !ok {
			return nil, fmt.Errorf("opcua: variable %s: %w: %s has no OPC UA DataType", spec.Path, ErrTypeMismatch, typeName(t))
		}
	}

	access := byte(spec.Access.effective())
	v := server.NewVariableNode(s.srv, id, ua.NewQualifiedName(s.ns, spec.Name), lt(spec.Name), lt(spec.Description), nil,
		[]ua.Reference{s.parentRef(spec), ref(ua.ReferenceTypeIDHasTypeDefinition, false, ua.VariableTypeIDBaseDataVariableType)},
		ua.DataValue{}, dataType, rank, dims, access, 0, false, nil)
	// Only CurrentRead/CurrentWrite are used, so UserAccessLevel equals
	// AccessLevel under the role permissions set in New.
	if sdt != nil {
		s.installStructHandlers(v, sp, spec, sdt, src)
	} else {
		installScalarHandlers(v, spec, src)
	}
	nodes := []server.Node{v}
	if enumType != nil {
		pname, pval, pdt, n := enumProperty(enumType)
		nodes = append(nodes, s.newPropertyNode(s.id(spec.Path+"."+pname), id, pname, pval, pdt, n))
	}
	return nodes, nil
}

// installScalarHandlers serves an elementary, enum or array Variable.
func installScalarHandlers(v *server.VariableNode, spec NodeSpec, src NodeSource) {
	path, typ := spec.Path, spec.Type
	v.SetReadValueHandler(func(_ *server.Session, _ ua.ReadValueID) ua.DataValue {
		val, err := src.Read(path)
		if err != nil {
			return badRead(readStatusFor(err))
		}
		u, err := toUA(typ, val)
		if err != nil {
			return badRead(readStatusFor(err))
		}
		return goodValue(u)
	})
	v.SetWriteValueHandler(func(_ *server.Session, w ua.WriteValue) (ua.DataValue, ua.StatusCode) {
		if w.IndexRange != "" {
			return ua.DataValue{}, ua.BadWriteNotSupported
		}
		iec, err := fromUA(typ, w.Value.Value)
		if err != nil {
			return ua.DataValue{}, statusFor(err)
		}
		if err := src.Write(path, iec); err != nil {
			return ua.DataValue{}, statusFor(err)
		}
		return goodValue(w.Value.Value), ua.Good
	})
}

// leafWrite is one member write produced by decomposing a struct value.
type leafWrite struct {
	path string
	val  any
}

// installStructHandlers serves a Variable of a structured DataType,
// composed from (and decomposed into) its leaf member paths.
func (s *Server) installStructHandlers(v *server.VariableNode, sp *Space, spec NodeSpec, d *structDT, src NodeSource) {
	path := spec.Path
	leaves := structLeaves(path, d)
	v.SetReadValueHandler(func(_ *server.Session, _ ua.ReadValueID) ua.DataValue {
		snap, err := src.Snapshot(leaves)
		if err != nil {
			return badRead(readStatusFor(err))
		}
		val, err := composeStruct(path, d, snap)
		if err != nil {
			return badRead(readStatusFor(err))
		}
		return goodValue(val.Interface())
	})
	v.SetWriteValueHandler(func(_ *server.Session, w ua.WriteValue) (ua.DataValue, ua.StatusCode) {
		if w.IndexRange != "" {
			return ua.DataValue{}, ua.BadWriteNotSupported
		}
		rv := reflect.ValueOf(w.Value.Value)
		if rv.Kind() == reflect.Pointer && !rv.IsNil() {
			rv = rv.Elem()
		}
		if !rv.IsValid() || rv.Type() != d.GoType {
			return ua.DataValue{}, ua.BadTypeMismatch
		}
		var writes []leafWrite
		if err := decomposeStruct(sp, path, d, rv, true, &writes); err != nil {
			return ua.DataValue{}, statusFor(err)
		}
		for _, lw := range writes {
			if err := src.Write(lw.path, lw.val); err != nil {
				return ua.DataValue{}, statusFor(err)
			}
		}
		return goodValue(w.Value.Value), ua.Good
	})
}

// elementPaths returns "<p>[i]" (or "<p>[i,j]") for every element of a in
// row-major order, starting at each dimension's Low bound.
func elementPaths(p string, a *types.ArrayType) []string {
	out := []string{""}
	for _, d := range a.Dimensions {
		next := make([]string, 0, len(out)*(d.High-d.Low+1))
		for _, prefix := range out {
			for i := d.Low; i <= d.High; i++ {
				if prefix == "" {
					next = append(next, strconv.Itoa(i))
				} else {
					next = append(next, prefix+","+strconv.Itoa(i))
				}
			}
		}
		out = next
	}
	for i, idx := range out {
		out[i] = p + "[" + idx + "]"
	}
	return out
}

// structLeaves lists the NodeSource paths a struct value is composed from:
// elementary, enum and array-of-elementary members are leaves; nested
// structs and array-of-struct elements recurse.
func structLeaves(prefix string, d *structDT) []string {
	var out []string
	for _, f := range d.Fields {
		p := prefix + "." + f.Name
		switch {
		case f.Nested != nil && f.IsArray:
			for _, ep := range elementPaths(p, f.Type.(*types.ArrayType)) {
				out = append(out, structLeaves(ep, f.Nested)...)
			}
		case f.Nested != nil:
			out = append(out, structLeaves(p, f.Nested)...)
		default:
			out = append(out, p)
		}
	}
	return out
}

// composeStruct builds the dynamic Go struct value for prefix from snap.
func composeStruct(prefix string, d *structDT, snap map[string]any) (reflect.Value, error) {
	v := reflect.New(d.GoType).Elem()
	for i, f := range d.Fields {
		p := prefix + "." + f.Name
		switch {
		case f.Nested != nil && f.IsArray:
			eps := elementPaths(p, f.Type.(*types.ArrayType))
			sl := reflect.MakeSlice(f.GoType, len(eps), len(eps))
			for j, ep := range eps {
				ev, err := composeStruct(ep, f.Nested, snap)
				if err != nil {
					return v, err
				}
				sl.Index(j).Set(ev)
			}
			v.Field(i).Set(sl)
		case f.Nested != nil:
			nv, err := composeStruct(p, f.Nested, snap)
			if err != nil {
				return v, err
			}
			v.Field(i).Set(nv)
		default:
			raw, ok := snap[p]
			if !ok {
				return v, fmt.Errorf("%w: %s", ErrUnknownSymbol, p)
			}
			u, err := toUA(f.Type, raw)
			if err != nil {
				return v, fmt.Errorf("%s: %w", p, err)
			}
			v.Field(i).Set(reflect.ValueOf(u))
		}
	}
	return v, nil
}

// memberWritable narrows writable by the member's own NodeSpec when it is
// published; an unpublished member inherits from its enclosing node.
func memberWritable(sp *Space, path string, writable bool) bool {
	if spec, ok := sp.Find(path); ok {
		return writable && spec.Access.effective()&AccessWrite != 0
	}
	return writable
}

// decomposeStruct converts a struct value into member writes, skipping
// members whose own NodeSpec is read-only (a whole-struct write never
// changes p_stat_* outputs). All conversions finish before any write.
func decomposeStruct(sp *Space, prefix string, d *structDT, v reflect.Value, writable bool, out *[]leafWrite) error {
	for i, f := range d.Fields {
		p := prefix + "." + f.Name
		w := memberWritable(sp, p, writable)
		fv := v.Field(i)
		switch {
		case f.Nested != nil && f.IsArray:
			eps := elementPaths(p, f.Type.(*types.ArrayType))
			if fv.Len() != len(eps) {
				return fmt.Errorf("%w: %s holds %d elements, got %d", ErrTypeMismatch, p, len(eps), fv.Len())
			}
			for j, ep := range eps {
				if err := decomposeStruct(sp, ep, f.Nested, fv.Index(j), memberWritable(sp, ep, w), out); err != nil {
					return err
				}
			}
		case f.Nested != nil:
			if err := decomposeStruct(sp, p, f.Nested, fv, w, out); err != nil {
				return err
			}
		default:
			if !w {
				continue
			}
			iec, err := fromUA(f.Type, fv.Interface())
			if err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			*out = append(*out, leafWrite{path: p, val: iec})
		}
	}
	return nil
}
