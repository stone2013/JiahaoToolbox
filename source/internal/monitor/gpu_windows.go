//go:build windows && amd64

package monitor

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var dxgi = syscall.NewLazyDLL("dxgi.dll")
var createFactory = dxgi.NewProc("CreateDXGIFactory1")
var factoryIID = syscall.GUID{Data1: 0x770aae78, Data2: 0xf26f, Data3: 0x4dba, Data4: [8]byte{0xa8, 0x29, 0x25, 0x3c, 0x83, 0xd1, 0xb3, 0x87}}

type dxObject struct{ VT *[16]uintptr }

func dxCall(o *dxObject, index int, args ...uintptr) uintptr {
	if o == nil {
		return 0x80004003
	}
	a := append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)
	r, _, _ := syscall.SyscallN(o.VT[index], a...)
	runtime.KeepAlive(o)
	return r
}

// DXGI_ADAPTER_DESC1 uses SIZE_T (64-bit on the supported x64 target). Do not
// replace these fields with Win32_VideoController.AdapterRAM (a 32-bit field).
type adapterDesc struct {
	Description                                   [128]uint16
	Vendor, Device, Subsystem, Revision           uint32
	DedicatedVideo, DedicatedSystem, SharedSystem uint64
	LuidLow                                       uint32
	LuidHigh                                      int32
	Flags                                         uint32
	_                                             uint32
}

func readGPUs() ([]GPU, error) {
	if e := createFactory.Find(); e != nil {
		return nil, e
	}
	var factory *dxObject
	r, _, _ := createFactory.Call(uintptr(unsafe.Pointer(&factoryIID)), uintptr(unsafe.Pointer(&factory)))
	if int32(r) < 0 || factory == nil {
		return nil, fmt.Errorf("DXGI factory HRESULT 0x%08x", uint32(r))
	}
	defer dxCall(factory, 2)
	out := []GPU{}
	for i := uintptr(0); i < 64; i++ {
		var adapter *dxObject
		r = dxCall(factory, 12, i, uintptr(unsafe.Pointer(&adapter)))
		if uint32(r) == 0x887a0002 {
			break
		}
		if int32(r) < 0 || adapter == nil {
			return out, fmt.Errorf("DXGI enumeration HRESULT 0x%08x", uint32(r))
		}
		var d adapterDesc
		r = dxCall(adapter, 10, uintptr(unsafe.Pointer(&d)))
		dxCall(adapter, 2)
		if int32(r) < 0 {
			continue
		}
		if d.Flags&2 != 0 {
			continue
		} // Exclude Microsoft software rasterizers.
		out = append(out, GPU{ID: fmt.Sprintf("%08x%08x", uint32(d.LuidHigh), d.LuidLow), Name: syscall.UTF16ToString(d.Description[:]), VendorID: fmt.Sprintf("0x%04X", d.Vendor), Usage: Unavailable("%", "等待 WDDM GPU Engine 有效计数；驱动或系统可能不提供"), Memory: Unavailable("B", "GPU Adapter Memory 计数器尚未提供"), Dedicated: Valid(float64(d.DedicatedVideo), "B"), SharedLimit: Valid(float64(d.SharedSystem), "B"), Temperature: Unsupported("°C", "未接入厂商温度接口")})
	}
	return out, nil
}
