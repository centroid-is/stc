package ecat

// Dir is the process image direction of a PDO or slot.
type Dir int

const (
	DirIn  Dir = iota // TxPdo, slave to master (%I)
	DirOut            // RxPdo, master to slave (%Q)
)

// String returns "in" or "out".
func (d Dir) String() string {
	if d == DirOut {
		return "out"
	}
	return "in"
}

// Slot is the location of one linkable item in a master's process image.
type Slot struct{}

func buildLayout(m *Master, pi *processImage) {}
