package main

import (
	"bytes"
	"encoding/binary"

	"testing"
)

func TestWindowsVersionResource(t *testing.T) {
	b, e := buildVersionInfo("4.4.0")
	if e != nil {
		t.Fatal(e)
	}
	if int(binary.LittleEndian.Uint16(b)) != len(b) {
		t.Fatal("resource block length mismatch")
	}
	if !bytes.Contains(b, []byte{0x34, 0x00, 0x2e, 0x00, 0x34, 0x00, 0x2e, 0x00, 0x30, 0x00}) || !bytes.Contains(b, []byte{'J', 0, 'i', 0}) {
		t.Fatal("version/product strings missing")
	}
	if _, e = buildVersionInfo("4.x.0"); e == nil {
		t.Fatal("invalid version was accepted")
	}
}
