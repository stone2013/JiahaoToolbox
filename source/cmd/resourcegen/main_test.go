package main

import (
	"bytes"
	"debug/pe"
	"testing"
)

func TestResourceObject(t *testing.T) {
	b, e := build([]resource{{typ: 24, id: 1, data: []byte("PerMonitorV2")}, {typ: 14, id: 1, data: []byte("group")}})
	if e != nil {
		t.Fatal(e)
	}
	f, e := pe.NewFile(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if f.Machine != pe.IMAGE_FILE_MACHINE_AMD64 || len(f.Sections) != 1 || len(f.Sections[0].Relocs) != 2 {
		t.Fatal("bad COFF resource")
	}
	d, e := f.Sections[0].Data()
	if e != nil || !bytes.Contains(d, []byte("PerMonitorV2")) {
		t.Fatal("resource missing", e)
	}
	for _, r := range f.Sections[0].Relocs {
		off := int(le.Uint32(d[r.VirtualAddress:]))
		if off < 16 || off >= len(d) {
			t.Fatal("bad relocation")
		}
	}
}
