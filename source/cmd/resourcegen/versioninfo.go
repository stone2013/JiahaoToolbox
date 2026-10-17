package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
)

type versionConfig struct {
	Version string `json:"version"`
}

func readAppVersion() (string, error) {
	b, e := os.ReadFile("version.json")
	if e != nil {
		return "", e
	}
	var c versionConfig
	if e = json.Unmarshal(b, &c); e != nil {
		return "", e
	}
	return c.Version, nil
}
func align4(b *bytes.Buffer) {
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
}
func appendU16(b *bytes.Buffer, v uint16) { _ = binary.Write(b, binary.LittleEndian, v) }
func appendU32(b *bytes.Buffer, v uint32) { _ = binary.Write(b, binary.LittleEndian, v) }
func versionBlock(key string, typ uint16, valueLength uint16, value, children []byte) []byte {
	var b bytes.Buffer
	appendU16(&b, 0)
	appendU16(&b, valueLength)
	appendU16(&b, typ)
	for _, r := range utf16.Encode([]rune(key)) {
		appendU16(&b, r)
	}
	appendU16(&b, 0)
	align4(&b)
	b.Write(value)
	align4(&b)
	b.Write(children)
	out := b.Bytes()
	binary.LittleEndian.PutUint16(out, uint16(len(out)))
	return out
}
func buildVersionInfo(version string) ([]byte, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid version %q", version)
	}
	var nums [3]uint16
	for i, p := range parts {
		var n uint64
		for _, r := range p {
			if r < '0' || r > '9' {
				return nil, errors.New("version must contain numeric MAJOR.MINOR.PATCH")
			}
			n = n*10 + uint64(r-'0')
			if n > 65535 {
				return nil, errors.New("version component exceeds Windows metadata range")
			}
		}
		nums[i] = uint16(n)
	}
	fixed := new(bytes.Buffer)
	for _, v := range []uint32{0xFEEF04BD, 0x00010000, uint32(nums[0])<<16 | uint32(nums[1]), uint32(nums[2]) << 16, uint32(nums[0])<<16 | uint32(nums[1]), uint32(nums[2]) << 16, 0x3F, 0, 0x00040004, 1, 0, 0, 0} {
		appendU32(fixed, v)
	}
	stringsMap := [][2]string{{"CompanyName", "Jiahao Toolbox"}, {"FileDescription", "Jiahao Toolbox"}, {"FileVersion", version + ".0"}, {"InternalName", "JiahaoToolbox"}, {"LegalCopyright", "Copyright (c) Jiahao Toolbox"}, {"OriginalFilename", fmt.Sprintf("JiahaoToolbox_v%s.exe", version)}, {"ProductName", "Jiahao Toolbox"}, {"ProductVersion", version + ".0"}}
	var pairs bytes.Buffer
	for _, pair := range stringsMap {
		v := append(utf16.Encode([]rune(pair[1])), 0)
		var vb bytes.Buffer
		for _, u := range v {
			appendU16(&vb, u)
		}
		pairs.Write(versionBlock(pair[0], 1, uint16(len(v)), vb.Bytes(), nil))
	}
	table := versionBlock("040904B0", 1, 0, nil, pairs.Bytes())
	sfi := versionBlock("StringFileInfo", 1, 0, nil, table)
	var trans bytes.Buffer
	appendU16(&trans, 0x0409)
	appendU16(&trans, 1200)
	var varInfo bytes.Buffer
	varInfo.Write(versionBlock("Translation", 0, 4, trans.Bytes(), nil))
	vi := versionBlock("VarFileInfo", 1, 0, nil, varInfo.Bytes())
	children := append(sfi, vi...)
	return versionBlock("VS_VERSION_INFO", 0, 52, fixed.Bytes(), children), nil
}
