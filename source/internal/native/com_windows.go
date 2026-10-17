//go:build windows && amd64

package native

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var ole = syscall.NewLazyDLL("ole32.dll")
var coInit = ole.NewProc("CoInitializeEx")
var coUninit = ole.NewProc("CoUninitialize")
var coFree = ole.NewProc("CoTaskMemFree")
var clsidFromString = ole.NewProc("CLSIDFromString")

type object struct{ VT *[128]uintptr }

func call(o *object, index int, args ...uintptr) uintptr {
	if o == nil {
		return 0x80004003
	}
	a := append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)
	r, _, _ := syscall.SyscallN(o.VT[index], a...)
	runtime.KeepAlive(o)
	return r
}
func hrError(where string, hr uintptr) error {
	if int32(hr) < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08X", where, uint32(hr))
	}
	return nil
}
func guid(s string) syscall.GUID {
	var g syscall.GUID
	p := syscall.StringToUTF16Ptr("{" + s + "}")
	clsidFromString.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&g)))
	return g
}

var iidUnknown = syscall.GUID{Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}

type callback struct {
	VT      *[4]uintptr
	methods [4]uintptr
	IID     syscall.GUID
	refs    atomic.Int32
	pin     runtime.Pinner
	fn      func(uintptr, *object) uintptr
}

var callbackQI = syscall.NewCallback(func(h *callback, iid *syscall.GUID, out *unsafe.Pointer) uintptr {
	if out == nil {
		return 0x80004003
	}
	*out = nil
	if iid == nil {
		return 0x80004003
	}
	if *iid != iidUnknown && *iid != h.IID {
		return 0x80004002
	}
	*out = unsafe.Pointer(h)
	h.refs.Add(1)
	return 0
})
var callbackAdd = syscall.NewCallback(func(h *callback) uintptr { return uintptr(h.refs.Add(1)) })
var callbackRelease = syscall.NewCallback(func(h *callback) uintptr {
	n := h.refs.Add(-1)
	if n < 0 {
		return 0
	}
	return uintptr(n)
})
var callbackInvoke = syscall.NewCallback(func(h *callback, a uintptr, b *object) (result uintptr) {
	defer func() {
		if recover() != nil {
			result = 0x80004005
		}
	}()
	return h.fn(a, b)
})

func newCallback(iid string, fn func(uintptr, *object) uintptr) *callback {
	h := &callback{IID: guid(iid), fn: fn}
	h.methods = [4]uintptr{callbackQI, callbackAdd, callbackRelease, callbackInvoke}
	h.VT = &h.methods
	h.refs.Store(1)
	h.pin.Pin(h)
	return h
}
func wide(p *uint16) string {
	if p == nil {
		return ""
	}
	a := make([]uint16, 0, 128)
	for i := 0; i < 65536; i++ {
		n := *(*uint16)(unsafe.Add(unsafe.Pointer(p), i*2))
		if n == 0 {
			break
		}
		a = append(a, n)
	}
	return syscall.UTF16ToString(a)
}
func getString(o *object, index int) string {
	var p *uint16
	r := call(o, index, uintptr(unsafe.Pointer(&p)))
	if p != nil {
		defer coFree.Call(uintptr(unsafe.Pointer(p)))
	}
	if int32(r) < 0 {
		return ""
	}
	return wide(p)
}
