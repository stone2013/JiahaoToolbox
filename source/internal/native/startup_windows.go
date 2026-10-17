//go:build windows && amd64

package native

import (
	"syscall"
	"unsafe"
)

// Use the saved preference before the first native/WebView2 paint.
func startupLight(theme string) bool {
	if theme != "system" {
		return theme == "light"
	}
	var key syscall.Handle
	name, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, name, 0, syscall.KEY_READ, &key) != nil {
		return false
	}
	defer syscall.RegCloseKey(key)
	value, _ := syscall.UTF16PtrFromString("AppsUseLightTheme")
	var kind, light uint32
	size := uint32(4)
	return syscall.RegQueryValueEx(key, value, nil, &kind, (*byte)(unsafe.Pointer(&light)), &size) == nil && kind == syscall.REG_DWORD && light != 0
}
func (w *host) FrontendReady() error {
	h := w.hwnd.Load()
	if h == 0 {
		return syscall.EINVAL
	}
	r, _, e := postMessage.Call(h, wmCommand, 6, 0)
	if r == 0 {
		return e
	}
	return nil
}
func (w *host) checkReady() {
	if w.loaded && w.mounted && !w.closed && !w.ready {
		w.ready = true
		killTimer.Call(w.hwnd.Load(), 1)
		w.o.Log.Print("native startup: navigation and frontend handshake ready")
	}
}

func StartupTheme(theme string) string {
	if startupLight(theme) {
		return "light"
	}
	return "dark"
}
func ReducedMotion() bool {
	var enabled int32 = 1
	r, _, _ := user.NewProc("SystemParametersInfoW").Call(0x1042, 0, uintptr(unsafe.Pointer(&enabled)), 0)
	return r != 0 && enabled == 0
}
