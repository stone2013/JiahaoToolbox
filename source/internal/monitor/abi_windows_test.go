//go:build windows && amd64

package monitor

import (
	"testing"
	"unsafe"
)

func TestWindowsABIStructures(t *testing.T) {
	var row mibIfRow
	if unsafe.Sizeof(row) != 1352 || unsafe.Offsetof(row.InOctets) != 1208 || unsafe.Offsetof(row.OutOctets) != 1280 {
		t.Fatalf("MIB_IF_ROW2 layout: size=%d in=%d out=%d", unsafe.Sizeof(row), unsafe.Offsetof(row.InOctets), unsafe.Offsetof(row.OutOctets))
	}
	if unsafe.Sizeof(memStatus{}) != 64 || unsafe.Sizeof(fmtValue{}) != 16 || unsafe.Sizeof(fmtItem{}) != 24 || unsafe.Sizeof(adapterDesc{}) != 312 {
		t.Fatal("Windows x64 ABI layout mismatch")
	}
}
