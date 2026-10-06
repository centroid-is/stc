package opcua

import (
	"fmt"
	"time"

	"github.com/awcullen/opcua/server"
	"github.com/awcullen/opcua/ua"
)

// diNamespaceIdx is the namespace index holding the OPC UA DI DeviceSet
// (the urn:stc:filler:2 slot; the DI type model itself is not loaded).
const diNamespaceIdx = 2

// deviceSetID is the DI DeviceSet object, ns=2;i=5001 as in the DI model.
var deviceSetID = ua.NewNodeIDNumeric(diNamespaceIdx, 5001)

// plc1Properties are the DI DeviceType properties TF6100 shows on PLC1.
var plc1Properties = []struct {
	name  string
	value any
	dt    ua.NodeID
}{
	{"Model", "TwinCAT 3 PLC emulated by stc", ua.DataTypeIDString},
	{"DeviceManual", "", ua.DataTypeIDString},
	{"DeviceRevision", "3.1", ua.DataTypeIDString},
	{"SerialNumber", "stc-emulator", ua.DataTypeIDString},
	{"DeviceState", int32(0), ua.DataTypeIDInt32},
}

// plc1ID is the device object every published GVL and PROGRAM hangs under.
func (s *Server) plc1ID() ua.NodeID { return s.id(plc1Name) }

// ensureDeviceSet adds Objects/DeviceSet/PLC1 with its DI properties,
// once per Server.
func (s *Server) ensureDeviceSet() (ua.NodeID, error) {
	s.dsOnce.Do(func() {
		plc1 := s.plc1ID()
		ds := server.NewObjectNode(s.srv, deviceSetID, ua.NewQualifiedName(diNamespaceIdx, "DeviceSet"), lt("DeviceSet"), lt(""), nil,
			[]ua.Reference{
				ref(ua.ReferenceTypeIDOrganizes, true, ua.ObjectIDObjectsFolder),
				ref(ua.ReferenceTypeIDHasTypeDefinition, false, ua.ObjectTypeIDBaseObjectType),
			}, 0)
		dev := server.NewObjectNode(s.srv, plc1, ua.NewQualifiedName(s.ns, plc1Name), lt(plc1Name), lt(""), nil,
			[]ua.Reference{
				ref(ua.ReferenceTypeIDHasComponent, true, deviceSetID),
				ref(ua.ReferenceTypeIDHasTypeDefinition, false, ua.ObjectTypeIDBaseObjectType),
			}, 0)
		nodes := []server.Node{ds, dev}
		now := time.Now()
		for _, p := range plc1Properties {
			nodes = append(nodes, server.NewVariableNode(s.srv, s.id(plc1Name+"."+p.name),
				ua.NewQualifiedName(diNamespaceIdx, p.name), lt(p.name), lt(""), nil,
				[]ua.Reference{
					ref(ua.ReferenceTypeIDHasProperty, true, plc1),
					ref(ua.ReferenceTypeIDHasTypeDefinition, false, ua.VariableTypeIDPropertyType),
				},
				ua.NewDataValue(p.value, ua.Good, now, 0, now, 0), p.dt, ua.ValueRankScalar, nil,
				ua.AccessLevelsCurrentRead, 0, false, nil))
		}
		if err := s.nm.AddNodes(nodes...); err != nil {
			s.dsErr = fmt.Errorf("opcua: add DeviceSet: %w", err)
		}
	})
	return s.plc1ID(), s.dsErr
}
