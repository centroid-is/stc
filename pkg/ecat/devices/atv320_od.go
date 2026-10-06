package devices

import (
	"encoding/binary"

	"github.com/centroid-is/stc/pkg/ecat"
)

// ATV320 object dictionary keys: index<<8 | subindex.
const (
	odEEPROMSave uint32 = 0x2032<<8 | 0x01
	eepromSteps         = 5 // Steps an EEPROM save takes
)

func odKey(index uint16, sub uint8) uint32 { return uint32(index)<<8 | uint32(sub) }

// odEntry describes one object. size is in bytes; set is nil for read-only
// objects; motor marks motor-block parameters locked in Operation enabled.
type odEntry struct {
	size  int
	motor bool
	get   func(*ATV320) uint32
	set   func(*ATV320, uint32)
}

// Defaults for parameters without a model field (ATV320 units). FB_ATV320
// writes all of these through FB_Parameter as UINT.
var atv320ParamDefaults = map[uint32]uint32{
	odKey(0x2042, 0x0E): 75,   // NPR nominal motor power, 0.01 kW
	odKey(0x2042, 0x02): 400,  // UNS nominal motor voltage, V
	odKey(0x2042, 0x05): 1430, // NSP nominal motor speed, rpm
	odKey(0x2042, 0x07): 80,   // COS motor cos phi, 0.01
	odKey(0x2042, 0x17): 20,   // ITH motor thermal current, 0.1 A
	odKey(0x2042, 0x3F): 0,    // LFA
	odKey(0x2042, 0x2B): 0,    // RSA
	odKey(0x2042, 0x44): 0,    // TRA
	odKey(0x2052, 0x1F): 4,    // DCF ramp divider
	odKey(0x203E, 0x29): 0,    // SSB
	odKey(0x203E, 0x02): 30,   // CLI current limit, 0.1 A
	odEEPROMSave:        0,    // EEPROM save trigger
}

func u16Field(f func(*ATV320) *uint16, motor bool) odEntry {
	return odEntry{size: 2, motor: motor,
		get: func(d *ATV320) uint32 { return uint32(*f(d)) },
		set: func(d *ATV320, v uint32) { *f(d) = uint16(v) }}
}

func roEntry(size int, get func(*ATV320) uint32) odEntry {
	return odEntry{size: size, get: get}
}

func paramEntry(key uint32, size int, motor bool) odEntry {
	return odEntry{size: size, motor: motor,
		get: func(d *ATV320) uint32 { return d.params[key] },
		set: func(d *ATV320, v uint32) { d.params[key] = v }}
}

var atv320OD = func() map[uint32]odEntry {
	od := map[uint32]odEntry{
		// CiA402 objects.
		odKey(0x6040, 0x00): {size: 2,
			get: func(d *ATV320) uint32 { return uint32(d.cmd) },
			set: func(d *ATV320, v uint32) { d.cmd = uint16(v) }},
		odKey(0x6041, 0x00): roEntry(2, func(d *ATV320) uint32 { return uint32(d.eta) }),
		odKey(0x6060, 0x00): {size: 1,
			get: func(d *ATV320) uint32 { return uint32(d.opMode) },
			set: func(d *ATV320, v uint32) { d.opMode = uint8(v) }},
		odKey(0x6061, 0x00): roEntry(1, func(d *ATV320) uint32 { return uint32(d.opMode) }),
		// Live drive values.
		odKey(0x2002, 0x03): roEntry(2, func(d *ATV320) uint32 { return uint32(uint16(int16(d.rfr))) }),
		odKey(0x2002, 0x05): roEntry(2, func(d *ATV320) uint32 { return uint32(d.lcr) }),
		odKey(0x2002, 0x29): roEntry(2, func(d *ATV320) uint32 { return uint32(d.hmis) }),
		odKey(0x2029, 0x16): roEntry(2, func(d *ATV320) uint32 { return uint32(d.lft) }),
		odKey(0x2016, 0x03): roEntry(2, func(d *ATV320) uint32 { return uint32(d.di) }),
		odKey(0x2016, 0x0D): {size: 2,
			get: func(d *ATV320) uint32 { return uint32(d.ol1r) },
			set: func(d *ATV320, v uint32) { d.ol1r = uint16(v) }},
		odKey(0x2037, 0x03): {size: 2,
			get: func(d *ATV320) uint32 { return uint32(uint16(d.lfr)) },
			set: func(d *ATV320, v uint32) { d.lfr = int16(v) }},
		// Tunables backed by the model fields.
		odKey(0x2001, 0x05): u16Field(func(d *ATV320) *uint16 { return &d.HSP }, false),
		odKey(0x2001, 0x06): u16Field(func(d *ATV320) *uint16 { return &d.LSP }, false),
		odKey(0x2042, 0x03): u16Field(func(d *ATV320) *uint16 { return &d.FRS }, true),
		odKey(0x2042, 0x04): u16Field(func(d *ATV320) *uint16 { return &d.NCR }, true),
		odKey(0x203C, 0x02): u16Field(func(d *ATV320) *uint16 { return &d.ACC }, false),
		odKey(0x203C, 0x03): u16Field(func(d *ATV320) *uint16 { return &d.DEC }, false),
		// EEPROM save: a nonzero write starts a save that takes eepromSteps.
		odEEPROMSave: {size: 4,
			get: func(d *ATV320) uint32 { return d.params[odEEPROMSave] },
			set: func(d *ATV320, v uint32) {
				d.params[odEEPROMSave] = v
				if v != 0 {
					d.eepromLeft = eepromSteps
				} else {
					d.eepromLeft = 0
				}
			}},
	}
	for k := range atv320ParamDefaults {
		if k == odEEPROMSave {
			continue
		}
		idx := uint16(k >> 8)
		od[k] = paramEntry(k, 2, idx == 0x2042 || idx == 0x2052)
	}
	return od
}()

// atv320Indices records which indices exist, to tell a missing subindex
// (0x06090011) from a missing object (0x06020000).
var atv320Indices = func() map[uint16]bool {
	m := map[uint16]bool{}
	for k := range atv320OD {
		m[uint16(k>>8)] = true
	}
	return m
}()

func lookupOD(index uint16, sub uint8) (odEntry, uint32) {
	if e, ok := atv320OD[odKey(index, sub)]; ok {
		return e, 0
	}
	if atv320Indices[index] {
		return odEntry{}, ecat.AbortNoSubIndex
	}
	return odEntry{}, ecat.AbortNoObject
}

// SDORead reads an object as little-endian bytes of the object's size.
func (d *ATV320) SDORead(index uint16, sub uint8) ([]byte, uint32) {
	if code := d.sdoAllowed(); code != 0 {
		return nil, code
	}
	e, code := lookupOD(index, sub)
	if code != 0 {
		return nil, code
	}
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], e.get(d))
	return buf[:e.size:e.size], 0
}

// SDOWrite writes an object. Data longer than the object aborts; shorter
// data is zero-extended.
func (d *ATV320) SDOWrite(index uint16, sub uint8, data []byte) uint32 {
	if code := d.sdoAllowed(); code != 0 {
		return code
	}
	e, code := lookupOD(index, sub)
	switch {
	case code != 0:
		return code
	case e.set == nil:
		return ecat.AbortReadOnly
	case len(data) > e.size:
		return ecat.AbortLength
	case e.motor && d.sm.State() == StOperationEnabled:
		return ecat.AbortDeviceState
	}
	var buf [4]byte
	copy(buf[:], data)
	e.set(d, binary.LittleEndian.Uint32(buf[:]))
	return 0
}

// sdoAllowed reports whether the mailbox is available: not in INIT.
func (d *ATV320) sdoAllowed() uint32 {
	if d.ecState == ecat.StateInit {
		return ecat.AbortDeviceState
	}
	return 0
}

// stepEEPROM advances a pending EEPROM save.
func (d *ATV320) stepEEPROM() {
	if d.eepromLeft == 0 {
		return
	}
	d.eepromLeft--
	if d.eepromLeft == 0 {
		d.params[odEEPROMSave] = 0
		d.saveCount++
	}
}

// SaveCount returns the number of completed EEPROM saves.
func (d *ATV320) SaveCount() int { return d.saveCount }
