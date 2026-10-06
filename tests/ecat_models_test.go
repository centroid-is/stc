package tests

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/ecat/devices"
)

// Deferred to Phase 26: Schneider ATV320 drives.
const (
	vendorSchneider uint32 = 0x0800005a
	productATV320   uint32 = 0x389
)

// TestEcatModelCoverage loads every real EtherCATConfig export of the
// sildarvinnsla stations and requires a device model for each slave, except
// the ATV320 drives deferred to Phase 26. The exports are customer data and
// stay local, so the test only runs when STC_SILD_DIR points at the checkout.
func TestEcatModelCoverage(t *testing.T) {
	root := os.Getenv("STC_SILD_DIR")
	if root == "" {
		t.Skip("STC_SILD_DIR not set; real sildarvinnsla exports are local-only")
	}
	base := filepath.Join(root, "IO List from ethercat")
	for _, station := range []string{"ST101", "ST201", "ST301", "baader"} {
		files, _ := filepath.Glob(filepath.Join(base, station, "Device*.xml"))
		files2, _ := filepath.Glob(filepath.Join(base, station, "baader.xml"))
		files = append(files, files2...)
		sort.Strings(files)
		if len(files) == 0 {
			t.Errorf("%s: no exports found under %s", station, base)
			continue
		}
		total, modelled, deferred := 0, 0, 0
		for _, path := range files {
			file := filepath.Base(path)
			topo, err := ecat.LoadProject(path)
			if err != nil {
				t.Errorf("%s/%s: load: %v", station, file, err)
				continue
			}
			reg := ecat.NewRegistry()
			devices.Register(reg)
			net := ecat.NewNetwork(topo, reg)

			unmatched := 0
			for _, m := range topo.Masters {
				for _, s := range m.Slaves {
					total++
					if _, ok := reg.Lookup(s); ok {
						modelled++
						continue
					}
					unmatched++
					if s.Vendor == vendorSchneider && s.Product == productATV320 {
						deferred++
						continue
					}
					t.Errorf("%s/%s: %s vendor %#x product %#x has no device model",
						station, file, s.Name, s.Vendor, s.Product)
				}
			}
			n010 := 0
			for _, d := range net.Diagnostics() {
				if d.Code == ecat.CodeNoModel {
					n010++
				}
			}
			if n010 != unmatched {
				t.Errorf("%s/%s: %d ECAT010 diagnostics, want %d", station, file, n010, unmatched)
			}
			for i := 0; i < 10; i++ {
				net.Step(10 * time.Millisecond)
			}
		}
		t.Logf("%s: %d files, %d slaves, %d modelled, %d deferred (ATV320, Phase 26)",
			station, len(files), total, modelled, deferred)
	}
}
