//go:build windows

package monitor

import (
	"fmt"
	"math"
	"runtime"
	"syscall"
	"unsafe"
)

var pdhDLL = syscall.NewLazyDLL("pdh.dll")
var pdhOpen = pdhDLL.NewProc("PdhOpenQueryW")
var pdhAdd = pdhDLL.NewProc("PdhAddEnglishCounterW")
var pdhCollect = pdhDLL.NewProc("PdhCollectQueryData")
var pdhValue = pdhDLL.NewProc("PdhGetFormattedCounterValue")
var pdhArray = pdhDLL.NewProc("PdhGetFormattedCounterArrayW")
var pdhClose = pdhDLL.NewProc("PdhCloseQuery")

type pdhQuery struct{ handle uintptr }
type fmtValue struct {
	Status uint32
	_      uint32
	Number float64
}
type fmtItem struct {
	Name  *uint16
	Value fmtValue
}

func newPDH() (*pdhQuery, error) {
	q := &pdhQuery{}
	r, _, _ := pdhOpen.Call(0, 0, uintptr(unsafe.Pointer(&q.handle)))
	if r != 0 {
		return q, fmt.Errorf("PdhOpenQuery 0x%X", r)
	}
	return q, nil
}
func (q *pdhQuery) add(path string) uintptr {
	if q == nil || q.handle == 0 {
		return 0
	}
	if pdhAdd.Find() != nil {
		return 0
	}
	p, e := syscall.UTF16PtrFromString(path)
	if e != nil {
		return 0
	}
	var h uintptr
	r, _, _ := pdhAdd.Call(q.handle, uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&h)))
	if r != 0 {
		return 0
	}
	return h
}
func (q *pdhQuery) collect() error {
	if q == nil || q.handle == 0 {
		return fmt.Errorf("PDH 查询不可用")
	}
	r, _, _ := pdhCollect.Call(q.handle)
	if r != 0 {
		return fmt.Errorf("PdhCollectQueryData 0x%X", r)
	}
	return nil
}
func (q *pdhQuery) close() {
	if q != nil && q.handle != 0 {
		pdhClose.Call(q.handle)
		q.handle = 0
	}
}
func counter(h uintptr, unit string) Value {
	if h == 0 {
		return Unsupported(unit, "当前系统未提供此性能计数器")
	}
	var v fmtValue
	r, _, _ := pdhValue.Call(h, 0x200, 0, uintptr(unsafe.Pointer(&v)))
	if r != 0 || v.Status > 1 {
		return Unavailable(unit, fmt.Sprintf("性能计数器等待或失败 0x%X / 0x%X", r, v.Status))
	}
	if math.IsNaN(v.Number) || math.IsInf(v.Number, 0) {
		return Unavailable(unit, "性能计数器无效")
	}
	return Valid(v.Number, unit)
}
func readUTF16(p *uint16) string {
	if p == nil {
		return ""
	}
	a := make([]uint16, 0, 64)
	for i := 0; i < 8192; i++ {
		v := *(*uint16)(unsafe.Add(unsafe.Pointer(p), uintptr(i)*2))
		if v == 0 {
			break
		}
		a = append(a, v)
	}
	return syscall.UTF16ToString(a)
}
func counterArray(h uintptr) map[string]float64 {
	out := map[string]float64{}
	if h == 0 {
		return out
	}
	var size, count uint32
	r, _, _ := pdhArray.Call(h, 0x200, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), 0)
	if r != 0x800007D2 || size == 0 || size > 32<<20 {
		return out
	}
	for attempt := 0; attempt < 3; attempt++ {
		buf := make([]byte, size)
		r, _, _ = pdhArray.Call(h, 0x200, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buf[0])))
		if r == 0x800007D2 {
			if size > 32<<20 {
				return out
			}
			continue
		}
		if r != 0 || uintptr(count)*unsafe.Sizeof(fmtItem{}) > uintptr(len(buf)) {
			return out
		}
		items := unsafe.Slice((*fmtItem)(unsafe.Pointer(&buf[0])), int(count))
		for _, item := range items {
			if item.Value.Status <= 1 && !math.IsNaN(item.Value.Number) && !math.IsInf(item.Value.Number, 0) {
				out[readUTF16(item.Name)] = item.Value.Number
			}
		}
		runtime.KeepAlive(buf)
		return out
	}
	return out
}
