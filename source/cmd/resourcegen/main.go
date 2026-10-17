// resourcegen emits a deterministic AMD64 COFF .rsrc object using only Go's
// standard library. Run from the project root. No compiler SDK is required.
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sort"
)

var le = binary.LittleEndian

func put16(b []byte, n int, v uint16) { le.PutUint16(b[n:], v) }
func put32(b []byte, n int, v uint32) { le.PutUint32(b[n:], v) }

type resource struct {
	typ, id uint16
	data    []byte
	entry   int
}
type directory struct {
	ids      []uint16
	children []*directory
	leaves   []*resource
	off      int
}

func build(resources []resource) ([]byte, error) {
	if len(resources) == 0 {
		return nil, errors.New("no resources")
	}
	sort.Slice(resources, func(i, j int) bool {
		a, b := resources[i], resources[j]
		if a.typ != b.typ {
			return a.typ < b.typ
		}
		return a.id < b.id
	})
	root := &directory{}
	var typeDir *directory
	for i := range resources {
		r := &resources[i]
		if i == 0 || resources[i-1].typ != r.typ {
			typeDir = &directory{}
			root.ids = append(root.ids, r.typ)
			root.children = append(root.children, typeDir)
		}
		lang := &directory{ids: []uint16{0x409}, leaves: []*resource{r}}
		typeDir.ids = append(typeDir.ids, r.id)
		typeDir.children = append(typeDir.children, lang)
	}
	size := 0
	var assign func(*directory)
	assign = func(d *directory) {
		d.off = size
		size += 16 + 8*len(d.ids)
		for _, c := range d.children {
			assign(c)
		}
	}
	assign(root)
	for i := range resources {
		resources[i].entry = size
		size += 16
	}
	size = (size + 3) &^ 3
	data := make([]byte, size)
	var fill func(*directory)
	fill = func(d *directory) {
		put16(data, d.off+14, uint16(len(d.ids)))
		for i, id := range d.ids {
			off := d.off + 16 + 8*i
			put32(data, off, uint32(id))
			if len(d.children) > 0 {
				put32(data, off+4, uint32(d.children[i].off)|0x80000000)
				fill(d.children[i])
			} else {
				put32(data, off+4, uint32(d.leaves[i].entry))
			}
		}
	}
	fill(root)
	var reloc bytes.Buffer
	for _, r := range resources {
		off := len(data)
		put32(data, r.entry, uint32(off))
		put32(data, r.entry+4, uint32(len(r.data)))
		data = append(data, r.data...)
		for len(data)%4 != 0 {
			data = append(data, 0)
		}
		binary.Write(&reloc, le, uint32(r.entry))
		binary.Write(&reloc, le, uint32(0))
		binary.Write(&reloc, le, uint16(3)) // IMAGE_REL_AMD64_ADDR32NB
	}
	var out bytes.Buffer
	// COFF header + one 40-byte section header; timestamps intentionally zero.
	header := make([]byte, 60)
	put16(header, 0, 0x8664)
	put16(header, 2, 1)
	put32(header, 8, uint32(60+len(data)+reloc.Len()))
	put32(header, 12, 1)
	copy(header[20:], ".rsrc")
	put32(header, 36, uint32(len(data)))
	put32(header, 40, 60)
	put32(header, 44, uint32(60+len(data)))
	put16(header, 52, uint16(len(resources)))
	put32(header, 56, 0x40300040)
	out.Write(header)
	out.Write(data)
	out.Write(reloc.Bytes())
	symbol := make([]byte, 18)
	copy(symbol, ".rsrc")
	put16(symbol, 12, 1)
	symbol[16] = 3
	out.Write(symbol)
	binary.Write(&out, le, uint32(4)) // empty COFF string table
	return out.Bytes(), nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	manifest, e := os.ReadFile("resources/app.manifest")
	if e != nil {
		return e
	}
	ico, e := os.ReadFile("resources/app.ico")
	if e != nil {
		return e
	}
	if len(ico) < 6 || le.Uint16(ico[2:]) != 1 {
		return errors.New("invalid icon header")
	}
	version, e := readAppVersion()
	if e != nil {
		return fmt.Errorf("read version config: %w", e)
	}
	versionData, e := buildVersionInfo(version)
	if e != nil {
		return e
	}
	count := int(le.Uint16(ico[4:]))
	if count < 1 || count > 32 || len(ico) < 6+16*count {
		return errors.New("invalid icon directory")
	}
	group := make([]byte, 6+14*count)
	copy(group, ico[:6])
	rs := []resource{{typ: 24, id: 1, data: manifest}, {typ: 16, id: 1, data: versionData}}
	for i := 0; i < count; i++ {
		entry := ico[6+16*i:]
		n := int(le.Uint32(entry[8:]))
		off := int(le.Uint32(entry[12:]))
		if n < 1 || off < 6 || off > len(ico)-n {
			return errors.New("invalid icon range")
		}
		rs = append(rs, resource{typ: 3, id: uint16(i + 1), data: ico[off : off+n]})
		g := group[6+14*i:]
		copy(g[:12], entry[:12])
		put16(g, 12, uint16(i+1))
	}
	rs = append(rs, resource{typ: 14, id: 1, data: group})
	object, e := build(rs)
	if e != nil {
		return e
	}
	return os.WriteFile("cmd/jiahao-toolbox/app_windows_amd64.syso", object, 0644)
}
