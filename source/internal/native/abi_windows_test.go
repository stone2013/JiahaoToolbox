//go:build windows && amd64

package native

import (
	"testing"
	"unsafe"
)

func TestNativeWindowsABI(t *testing.T) {
	if unsafe.Sizeof(rect{}) != 16 || unsafe.Sizeof(message{}) != 48 || unsafe.Sizeof(wndClass{}) != 80 || unsafe.Sizeof(placement{}) != 44 || unsafe.Sizeof(monInfo{}) != 40 {
		t.Fatal("native Windows x64 ABI mismatch")
	}
}
func TestRuntimeVersionComparison(t *testing.T) {
	if runtimeVersion(`C:\a\Microsoft\EdgeWebView\Application\130.0.2849.80\x64\x.dll`) <= runtimeVersion(`C:\a\Microsoft\EdgeWebView\Application\99.0.1.2\x64\x.dll`) {
		t.Fatal("must sort numerically, not lexicographically")
	}
}
