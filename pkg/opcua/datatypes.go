package opcua

import (
	"fmt"
	"hash/fnv"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/awcullen/opcua/server"
	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/types"
)

// structField is one member of a registered structured DataType.
type structField struct {
	Name    string
	Type    types.Type   // IEC member type
	Nested  *structDT    // struct member or array-of-struct element
	IsArray bool         // ARRAY member (flattened, ValueRank 1)
	GoType  reflect.Type // Go field type (slice for arrays)
}

// structDT is a structured DataType served with a StructureDefinition and
// backed by a reflect.StructOf Go type registered with awcullen's encoder.
type structDT struct {
	Name   string
	DTID   ua.NodeID
	EncID  ua.NodeID
	GoType reflect.Type
	Fields []structField
}

// typeRegistry holds the custom DataType nodes of one Server, keyed by the
// upper-cased IEC type name (IEC identifiers are case-insensitive).
type typeRegistry struct {
	mu       sync.Mutex
	enums    map[string]ua.NodeID
	structs  map[string]*structDT
	building map[string]bool
}

// goTypeEntry is a registered dynamic Go type and its encoding NodeId text.
type goTypeEntry struct {
	goType reflect.Type
	encID  string
}

// goTypeCache is shared by all Servers in the process because awcullen's
// (Go type <-> encoding id) registry is process-global, append-only and
// panics on conflicting registrations. Keys are namespaceURI|name|layout.
var goTypeCache = struct {
	sync.Mutex
	byKey  map[string]goTypeEntry
	byName map[string]bool // namespaceURI|name already registered with some layout
}{byKey: map[string]goTypeEntry{}, byName: map[string]bool{}}

// registerGoType returns the dynamic Go type for a struct layout and its
// encoding NodeId string (namespace-less, qualified by namespaceURI). The
// first layout of a name uses "TE.<Name>.DefaultBinary"; a later different
// layout of the same name gets a layout-versioned id, since the registry
// cannot be unregistered.
func registerGoType(namespaceURI, name string, sfs []reflect.StructField) (reflect.Type, string) {
	var layout strings.Builder
	for _, f := range sfs {
		layout.WriteString(f.Name)
		layout.WriteByte(':')
		layout.WriteString(f.Type.String())
		layout.WriteByte(';')
	}
	nameKey := namespaceURI + "|" + name
	key := nameKey + "|" + layout.String()

	goTypeCache.Lock()
	defer goTypeCache.Unlock()
	if e, ok := goTypeCache.byKey[key]; ok {
		return e.goType, e.encID
	}
	encID := "TE." + name + ".DefaultBinary"
	if goTypeCache.byName[nameKey] {
		h := fnv.New32a()
		h.Write([]byte(layout.String()))
		encID = fmt.Sprintf("TE.%s.v%08x.DefaultBinary", name, h.Sum32())
	}
	gt := reflect.StructOf(sfs)
	ua.RegisterBinaryEncodingID(gt, ua.ExpandedNodeID{NamespaceURI: namespaceURI, NodeID: ua.NewNodeIDString(0, encID)})
	goTypeCache.byKey[key] = goTypeEntry{goType: gt, encID: encID}
	goTypeCache.byName[nameKey] = true
	return gt, encID
}

// goIdent replaces every rune that is not an ASCII letter, digit or '_'
// so the text can be part of an exported Go field name.
func goIdent(s string) string {
	b := []byte(s)
	for i, c := range b {
		if !(c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			b[i] = '_'
		}
	}
	return string(b)
}

func lt(s string) ua.LocalizedText { return ua.NewLocalizedText(s, "") }

func ref(t ua.NodeID, inverse bool, target ua.NodeID) ua.Reference {
	return ua.NewReference(t, inverse, ua.NewExpandedNodeID(target))
}

func (s *Server) id(text string) ua.NodeID { return ua.NewNodeIDString(s.ns, text) }

// enumContiguous reports whether the ordinals are exactly 0..n-1, the case
// EnumStrings can express.
func enumContiguous(et *types.EnumType) bool {
	for i, o := range et.Ordinals {
		if o != int64(i) {
			return false
		}
	}
	return true
}

// enumProperty returns the property an enum DataType (or Variable) carries:
// EnumStrings (LocalizedText[]) for contiguous ordinals, else EnumValues
// (EnumValueType[]).
func enumProperty(et *types.EnumType) (name string, value any, dataType ua.NodeID, n int) {
	if enumContiguous(et) {
		strs := make([]ua.LocalizedText, len(et.Values))
		for i, v := range et.Values {
			strs[i] = lt(v)
		}
		return "EnumStrings", strs, ua.DataTypeIDLocalizedText, len(strs)
	}
	vals := make([]ua.ExtensionObject, len(et.Values))
	for i, v := range et.Values {
		vals[i] = ua.EnumValueType{Value: et.Ordinals[i], DisplayName: lt(v), Description: lt("")}
	}
	return "EnumValues", vals, ua.DataTypeIDEnumValueType, len(vals)
}

// newPropertyNode builds a read-only PropertyType Variable owned by parent.
func (s *Server) newPropertyNode(id, parent ua.NodeID, name string, value any, dataType ua.NodeID, n int) *server.VariableNode {
	now := time.Now()
	return server.NewVariableNode(s.srv, id, ua.NewQualifiedName(0, name), lt(name), lt(""), nil,
		[]ua.Reference{
			ref(ua.ReferenceTypeIDHasProperty, true, parent),
			ref(ua.ReferenceTypeIDHasTypeDefinition, false, ua.VariableTypeIDPropertyType),
		},
		ua.NewDataValue(value, ua.Good, now, 0, now, 0),
		dataType, ua.ValueRankOneDimension, []uint32{uint32(n)},
		ua.AccessLevelsCurrentRead, 0, false, nil)
}

func validateEnum(et *types.EnumType) error {
	if et.Name == "" {
		return fmt.Errorf("%w: enum type without a name", ErrTypeMismatch)
	}
	if len(et.Ordinals) != len(et.Values) {
		return fmt.Errorf("%w: enum %s has %d values but %d ordinals", ErrTypeMismatch, et.Name, len(et.Values), len(et.Ordinals))
	}
	for i, o := range et.Ordinals {
		if o < math.MinInt32 || o > math.MaxInt32 {
			return fmt.Errorf("%w: enum %s value %s = %d does not fit Int32", ErrOutOfRange, et.Name, et.Values[i], o)
		}
	}
	return nil
}

// ensureEnum adds (once per Server) the Enumeration DataType DT.<Name>
// with an EnumDefinition and an EnumStrings or EnumValues property.
func (s *Server) ensureEnum(et *types.EnumType) (ua.NodeID, error) {
	s.reg.mu.Lock()
	defer s.reg.mu.Unlock()
	return s.ensureEnumLocked(et)
}

func (s *Server) ensureEnumLocked(et *types.EnumType) (ua.NodeID, error) {
	key := strings.ToUpper(et.Name)
	if id, ok := s.reg.enums[key]; ok {
		return id, nil
	}
	if err := validateEnum(et); err != nil {
		return nil, err
	}
	dtID := s.id("DT." + et.Name)
	fields := make([]ua.EnumField, len(et.Values))
	for i, v := range et.Values {
		fields[i] = ua.EnumField{Value: et.Ordinals[i], DisplayName: lt(v), Description: lt(""), Name: v}
	}
	pname, pval, pdt, n := enumProperty(et)
	propID := s.id("DT." + et.Name + "." + pname)
	dt := server.NewDataTypeNode(s.srv, dtID, ua.NewQualifiedName(s.ns, et.Name), lt(et.Name), lt(""), nil,
		[]ua.Reference{
			ref(ua.ReferenceTypeIDHasSubtype, true, ua.DataTypeIDEnumeration),
			ref(ua.ReferenceTypeIDHasProperty, false, propID),
		}, false, ua.EnumDefinition{Fields: fields})
	prop := s.newPropertyNode(propID, dtID, pname, pval, pdt, n)
	if err := s.nm.AddNodes(dt, prop); err != nil {
		return nil, fmt.Errorf("opcua: add enum %s: %w", et.Name, err)
	}
	if s.reg.enums == nil {
		s.reg.enums = map[string]ua.NodeID{}
	}
	s.reg.enums[key] = dtID
	return dtID, nil
}

// ensureStruct adds (once per Server) the structured DataType DT.<Name>
// with its StructureDefinition and Default Binary encoding object,
// registering nested structs and enums first. Members that are pointers,
// references, FBs or functions make it fail, so the builder publishes the
// struct as an Object instead.
func (s *Server) ensureStruct(st *types.StructType) (*structDT, error) {
	s.reg.mu.Lock()
	defer s.reg.mu.Unlock()
	return s.ensureStructLocked(st)
}

func (s *Server) ensureStructLocked(st *types.StructType) (*structDT, error) {
	key := strings.ToUpper(st.Name)
	if d, ok := s.reg.structs[key]; ok {
		return d, nil
	}
	if st.Name == "" {
		return nil, fmt.Errorf("%w: struct type without a name", ErrTypeMismatch)
	}
	if s.reg.building[key] {
		return nil, fmt.Errorf("%w: struct %s contains itself", ErrTypeMismatch, st.Name)
	}
	if s.reg.building == nil {
		s.reg.building = map[string]bool{}
	}
	s.reg.building[key] = true
	defer delete(s.reg.building, key)

	d := &structDT{Name: st.Name, Fields: make([]structField, len(st.Members))}
	sfs := make([]reflect.StructField, len(st.Members))
	defs := make([]ua.StructureField, len(st.Members))
	tn := goIdent(st.Name)
	for i, m := range st.Members {
		f, def, err := s.structMember(st, m)
		if err != nil {
			return nil, err
		}
		d.Fields[i] = f
		defs[i] = def
		sfs[i] = reflect.StructField{Name: fmt.Sprintf("X%s__%d_%s", tn, i, goIdent(m.Name)), Type: f.GoType}
	}
	var encText string
	d.GoType, encText = registerGoType(s.cfg.PLCNamespace, st.Name, sfs)
	d.DTID = s.id("DT." + st.Name)
	d.EncID = s.id(encText)

	dt := server.NewDataTypeNode(s.srv, d.DTID, ua.NewQualifiedName(s.ns, st.Name), lt(st.Name), lt(""), nil,
		[]ua.Reference{
			ref(ua.ReferenceTypeIDHasSubtype, true, ua.DataTypeIDStructure),
			ref(ua.ReferenceTypeIDHasEncoding, false, d.EncID),
		}, false,
		ua.StructureDefinition{
			DefaultEncodingID: d.EncID,
			BaseDataType:      ua.DataTypeIDStructure,
			StructureType:     ua.StructureTypeStructure,
			Fields:            defs,
		})
	enc := server.NewObjectNode(s.srv, d.EncID, ua.NewQualifiedName(0, "Default Binary"), lt("Default Binary"), lt(""), nil,
		[]ua.Reference{
			ref(ua.ReferenceTypeIDHasEncoding, true, d.DTID),
			ref(ua.ReferenceTypeIDHasTypeDefinition, false, ua.ObjectTypeIDDataTypeEncodingType),
		}, 0)
	if err := s.nm.AddNodes(dt, enc); err != nil {
		return nil, fmt.Errorf("opcua: add struct %s: %w", st.Name, err)
	}
	if s.reg.structs == nil {
		s.reg.structs = map[string]*structDT{}
	}
	s.reg.structs[key] = d
	return d, nil
}

// structMember maps one struct member to its field description and its
// StructureField. Arrays are flattened to ValueRank 1.
func (s *Server) structMember(st *types.StructType, m types.StructMember) (structField, ua.StructureField, error) {
	f := structField{Name: m.Name, Type: m.Type}
	def := ua.StructureField{Name: m.Name, Description: lt(""), ValueRank: ua.ValueRankScalar}
	elem := m.Type
	if a, ok := m.Type.(*types.ArrayType); ok {
		n, err := arrayLen(a)
		if err != nil {
			return f, def, fmt.Errorf("%w: %s.%s: %v", ErrTypeMismatch, st.Name, m.Name, err)
		}
		f.IsArray = true
		def.ValueRank, def.ArrayDimensions = ua.ValueRankOneDimension, []uint32{uint32(n)}
		elem = a.ElementType
	}
	var gt reflect.Type
	switch et := elem.(type) {
	case *types.StructType:
		nd, err := s.ensureStructLocked(et)
		if err != nil {
			return f, def, err
		}
		f.Nested, def.DataType, gt = nd, nd.DTID, nd.GoType
	case *types.EnumType:
		id, err := s.ensureEnumLocked(et)
		if err != nil {
			return f, def, err
		}
		def.DataType, gt = id, goInt32
	default:
		id, ok := uaDataType(elem)
		if !ok {
			return f, def, fmt.Errorf("%w: %s.%s of type %s has no OPC UA DataType", ErrTypeMismatch, st.Name, m.Name, typeName(m.Type))
		}
		def.DataType = id
		gt, _ = uaGoType(elem)
	}
	if f.IsArray {
		gt = reflect.SliceOf(gt)
	}
	f.GoType = gt
	return f, def, nil
}
