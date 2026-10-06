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
)

type deviceID struct {
	name            string
	vendor, product uint32
	factory         func() ecat.Device
}

func passive() ecat.Device   { return &Passive{} }
func digitalIO() ecat.Device { return &DigitalIO{} }

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
