package devices

import (
	"regexp"

	"github.com/centroid-is/stc/pkg/ecat"
)

// Vendor IDs.
const (
	VendorBeckhoff uint32 = 0x2
	VendorFesto    uint32 = 0x1d
)

// Product codes, verified against TwinCAT EtherCATConfig exports.
const (
	ProductEK1100 uint32 = 0x044c2c52
	ProductEK1110 uint32 = 0x04562c52
	ProductEK1200 uint32 = 0x04b02c52
	ProductEL6070 uint32 = 0x17b63052
	ProductEL9011 uint32 = 0x23333050
	ProductCU2508 uint32 = 0x09cc5432
	ProductEL1008 uint32 = 0x03f03052
	ProductEL1018 uint32 = 0x03fa3052 // Beckhoff numbering; not in the reference exports
	ProductEL2008 uint32 = 0x07d83052
	ProductEP2338 uint32 = 0x09224052 // EP2338-0002 and EP2338-1002 share it
	ProductCTEU   uint32 = 0x0008bc8c // Festo CTEU-EtherCAT bus node (572556)
	ProductEL3054 uint32 = 0x0bee3052
	ProductEL3064 uint32 = 0x0bf83052 // Beckhoff numbering; not in the reference exports
	ProductEL9222 uint32 = 0x24063052 // EL9222-5500
	ProductPS2001 uint32 = 0x07d14d02 // PS2001-2410
	ProductEL2912 uint32 = 0x0b603052
	ProductEP1918 uint32 = 0x077e4052 // EP1918-0002
	ProductEL1904 uint32 = 0x07703052
	ProductEL6001 uint32 = 0x17713052
)

type deviceID struct {
	name            string
	vendor, product uint32
	factory         func() ecat.Device
}

func passive() ecat.Device       { return &Passive{} }
func digitalIO() ecat.Device     { return &DigitalIO{} }
func analogIn4to20() ecat.Device { return &Analog{kind: analogCurrent} }
func analogIn0to10() ecat.Device { return &Analog{kind: analogVoltage} }
func el9222() ecat.Device        { return &EL9222{} }
func el6001() ecat.Device        { return &EL6001{} }

// knownIDs lists every exact (vendor, product) registration.
var knownIDs = []deviceID{
	{"EK1100", VendorBeckhoff, ProductEK1100, passive},
	{"EK1110", VendorBeckhoff, ProductEK1110, passive},
	{"EK1200", VendorBeckhoff, ProductEK1200, passive},
	{"EL6070", VendorBeckhoff, ProductEL6070, passive},
	{"EL9011", VendorBeckhoff, ProductEL9011, passive},
	{"CU2508", VendorBeckhoff, ProductCU2508, passive},
	{"EL1008", VendorBeckhoff, ProductEL1008, digitalIO},
	{"EL1018", VendorBeckhoff, ProductEL1018, digitalIO},
	{"EL2008", VendorBeckhoff, ProductEL2008, digitalIO},
	{"EP2338", VendorBeckhoff, ProductEP2338, digitalIO},
	{"CTEU", VendorFesto, ProductCTEU, digitalIO},
	{"EL3054", VendorBeckhoff, ProductEL3054, analogIn4to20},
	{"EL3064", VendorBeckhoff, ProductEL3064, analogIn0to10},
	{"EL9222-5500", VendorBeckhoff, ProductEL9222, el9222},
	{"PS2001-2410", VendorBeckhoff, ProductPS2001, psu},
	{"EL2912", VendorBeckhoff, ProductEL2912, safetyDiag},
	{"EP1918-0002", VendorBeckhoff, ProductEP1918, safetyDiag},
	{"EL1904", VendorBeckhoff, ProductEL1904, safetyDiag},
	{"EL6001", VendorBeckhoff, ProductEL6001, el6001},
}

type modelFallback struct {
	re      *regexp.Regexp
	factory func() ecat.Device
}

// beckhoffFallbacks match slave Model names when the product code is not
// registered (other revisions, synthetic fixtures).
var beckhoffFallbacks = []modelFallback{
	{regexp.MustCompile(`^EK1[0-2]\d\d$`), passive},
	{regexp.MustCompile(`^EL9011$`), passive},
	{regexp.MustCompile(`^EL100[48]$`), digitalIO},
	{regexp.MustCompile(`^EL101[48]$`), digitalIO},
	{regexp.MustCompile(`^EL200[48]$`), digitalIO},
	{regexp.MustCompile(`^EP2338`), digitalIO},
	{regexp.MustCompile(`^EL305\d$`), analogIn4to20},
	{regexp.MustCompile(`^EL306\d$`), analogIn0to10},
	{regexp.MustCompile(`^EL922\d`), el9222},
	{regexp.MustCompile(`^PS20\d\d`), psu},
	{regexp.MustCompile(`^EL600[12]$`), el6001},
}

// Register installs every device model into r.
func Register(r *ecat.Registry) {
	for _, id := range knownIDs {
		r.Register(id.vendor, id.product, id.factory)
	}
	for _, fb := range beckhoffFallbacks {
		r.RegisterModel(VendorBeckhoff, fb.re, fb.factory)
	}
}

func init() { Register(ecat.DefaultRegistry) }
