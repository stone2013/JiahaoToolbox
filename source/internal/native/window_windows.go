//go:build windows && amd64

package native

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var user = syscall.NewLazyDLL("user32.dll")
var kernel = syscall.NewLazyDLL("kernel32.dll")
var regClass = user.NewProc("RegisterClassExW")
var createWindow = user.NewProc("CreateWindowExW")
var defWindow = user.NewProc("DefWindowProcW")
var getMessage = user.NewProc("GetMessageW")
var translate = user.NewProc("TranslateMessage")
var dispatch = user.NewProc("DispatchMessageW")
var showWindow = user.NewProc("ShowWindow")
var destroyWindow = user.NewProc("DestroyWindow")
var postMessage = user.NewProc("PostMessageW")
var sendMessage = user.NewProc("SendMessageW")
var quit = user.NewProc("PostQuitMessage")
var getClient = user.NewProc("GetClientRect")
var getWindowRect = user.NewProc("GetWindowRect")
var setPos = user.NewProc("SetWindowPos")
var isZoomed = user.NewProc("IsZoomed")
var setDPI = user.NewProc("SetProcessDpiAwarenessContext")
var getDPI = user.NewProc("GetDpiForWindow")
var getSystemDPI = user.NewProc("GetDpiForSystem")
var getMetrics = user.NewProc("GetSystemMetrics")
var monitorFrom = user.NewProc("MonitorFromWindow")
var monitorInfo = user.NewProc("GetMonitorInfoW")
var releaseCapture = user.NewProc("ReleaseCapture")
var setTimer = user.NewProc("SetTimer")
var killTimer = user.NewProc("KillTimer")
var msgBox = user.NewProc("MessageBoxW")
var loadCursor = user.NewProc("LoadCursorW")
var getPlacement = user.NewProc("GetWindowPlacement")

// Copy OS-owned message structures through the Windows API. LPARAM is not a Go pointer.
var copyNativeMemory = kernel.NewProc("RtlMoveMemory")

type rect struct{ L, T, R, B int32 }
type point struct{ X, Y int32 }
type message struct {
	Hwnd           uintptr
	Message        uint32
	_              uint32
	Wparam, Lparam uintptr
	Time           uint32
	Pt             point
	Private        uint32
}
type wndClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type placement struct {
	Length, Flags, ShowCmd uint32
	Min, Max               point
	Normal                 rect
}
type minmax struct{ Reserved, MaxSize, MaxPosition, MinTrack, MaxTrack point }
type monInfo struct {
	Size          uint32
	Monitor, Work rect
	Flags         uint32
}
type registration struct {
	index int
	token int64
}
type host struct {
	o                    Options
	hwnd                 atomic.Uintptr
	controller, web, env *object
	handlers             []*callback
	events               []registration
	dll                  *syscall.DLL
	err                  error
	closed, ready        bool
	loaded, mounted      bool
	normalW, normalH     int
}

var current *host // Exactly one GUI per process. All COM work stays on its STA.
const wmCommand = 0x8000 + 70
const wmFailure = 0x8000 + 71

func New(o Options) Window { return &host{o: o} }
func Alert(text string) {
	p := syscall.StringToUTF16Ptr(text)
	t := syscall.StringToUTF16Ptr("嘉豪工具箱")
	msgBox.Call(0, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(t)), 0x10)
}
func (w *host) Command(cmd string) error {
	id := map[string]uintptr{"minimize": 1, "maximize": 2, "close": 3, "drag": 4, "reload": 5}[cmd]
	if id == 0 {
		return errors.New("不支持的窗口操作")
	}
	h := w.hwnd.Load()
	if h == 0 {
		return errors.New("原生窗口尚未就绪")
	}
	r, _, e := postMessage.Call(h, wmCommand, id, 0)
	if r == 0 {
		return e
	}
	return nil
}
func (w *host) Run(ctx context.Context) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	current = w
	if setDPI.Find() == nil {
		r, _, e := setDPI.Call(^uintptr(3))
		if r == 0 {
			w.o.Log.Printf("SetProcessDpiAwarenessContext: %v (manifest may already set DPI)", e)
		}
	}
	r, _, _ := coInit.Call(0, 2)
	if e := hrError("CoInitializeEx", r); e != nil {
		return e
	}
	defer coUninit.Call()
	instance, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	cursor, _, _ := loadCursor.Call(0, 32512)
	name := syscall.StringToUTF16Ptr("JiahaoToolboxWindowV4")
	icon, _, _ := user.NewProc("LoadIconW").Call(instance, 1)
	color := uintptr(0x211308)
	background := "FF081321"
	if startupLight(w.o.Config.Get().Theme) {
		color = 0xf9f2ed
		background = "FFEDF2F9"
	}
	// Microsoft documents this setting to prevent the controller's initial white flash.
	if err := os.Setenv("WEBVIEW2_DEFAULT_BACKGROUND_COLOR", background); err != nil {
		return err
	}
	brush, _, _ := syscall.NewLazyDLL("gdi32.dll").NewProc("CreateSolidBrush").Call(color)
	if brush != 0 {
		defer syscall.NewLazyDLL("gdi32.dll").NewProc("DeleteObject").Call(brush)
	}
	wc := wndClass{Size: uint32(unsafe.Sizeof(wndClass{})), Style: 3, Proc: windowProc, Instance: instance, Icon: icon, SmallIcon: icon, Cursor: cursor, Background: brush, Name: name}
	r, _, e := regClass.Call(uintptr(unsafe.Pointer(&wc)))
	if r == 0 {
		return fmt.Errorf("RegisterClassExW: %w", e)
	}
	s := w.o.Config.Get()
	dpi := uintptr(96)
	if getSystemDPI.Find() == nil {
		dpi, _, _ = getSystemDPI.Call()
	}
	scale := float64(dpi) / 96
	sw, _, _ := getMetrics.Call(0)
	sh, _, _ := getMetrics.Call(1)
	width, height := int(float64(s.Width)*scale), int(float64(s.Height)*scale)
	if width > int(sw)-32 {
		width = int(sw) - 32
	}
	if height > int(sh)-56 {
		height = int(sh) - 56
	}
	w.normalW = s.Width
	w.normalH = s.Height
	title := syscall.StringToUTF16Ptr("嘉豪工具箱 v" + w.o.Version)
	style := uintptr(0x80000000 | 0x00040000 | 0x00020000 | 0x00010000 | 0x00080000 | 0x02000000 | 0x04000000)
	hwnd, _, e := createWindow.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(title)), style, uintptr((int(sw)-width)/2), uintptr((int(sh)-height)/2), uintptr(width), uintptr(height), 0, 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW: %w", e)
	}
	w.hwnd.Store(hwnd)
	show := uintptr(5)
	if s.Maximized {
		show = 3
	}
	showWindow.Call(hwnd, show)
	envHandler := w.handler("4e8a3389-c9d8-4bd2-b6b5-124fee6cc14d", func(hr uintptr, env *object) uintptr {
		if w.closed {
			return 0
		}
		if e := hrError("WebView2 environment", hr); e != nil {
			w.fail(e)
			return 0
		}
		if env == nil {
			w.fail(errors.New("WebView2 返回空环境"))
			return 0
		}
		w.env = env
		call(env, 1)
		complete := w.handler("6c4819f3-c9b7-4260-8127-c9f5bde7f68c", func(hr uintptr, c *object) uintptr {
			if w.closed {
				return 0
			}
			if e := hrError("WebView2 controller", hr); e != nil {
				w.fail(e)
				return 0
			}
			if c == nil {
				w.fail(errors.New("WebView2 返回空控制器"))
				return 0
			}
			w.controller = c
			call(c, 1)
			r := call(c, 25, uintptr(unsafe.Pointer(&w.web)))
			if e := hrError("GetCoreWebView2", r); e != nil {
				w.fail(e)
				return 0
			}
			if w.web == nil {
				w.fail(errors.New("空 WebView2"))
				return 0
			}
			if e := w.configure(); e != nil {
				w.fail(e)
				return 0
			}
			w.resize()
			call(c, 4, 1)
			u := syscall.StringToUTF16Ptr(w.o.URL)
			r = call(w.web, 5, uintptr(unsafe.Pointer(u)))
			if e := hrError("Navigate", r); e != nil {
				w.fail(e)
				return 0
			}
			return 0
		})
		r := call(env, 3, hwnd, uintptr(unsafe.Pointer(complete)))
		if e := hrError("CreateCoreWebView2Controller", r); e != nil {
			w.fail(e)
		}
		return 0
	})
	if e := w.createEnvironment(envHandler); e != nil {
		w.closed = true
		destroyWindow.Call(hwnd)
		w.hwnd.Store(0)
		return e
	}
	setTimer.Call(hwnd, 1, 20000, 0)
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			w.Command("close")
		case <-stop:
		}
	}()
	defer close(stop)
	var msg message
	for {
		r, _, e := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) == -1 {
			w.err = fmt.Errorf("GetMessage: %w", e)
			break
		}
		if r == 0 {
			break
		}
		translate.Call(uintptr(unsafe.Pointer(&msg)))
		dispatch.Call(uintptr(unsafe.Pointer(&msg)))
	}
	w.closed = true
	w.hwnd.Store(0)
	// Release registered event callbacks before releasing the WebView. The runtime
	// DLL stays loaded until process exit; in-flight COM work must not be unloaded.
	if w.web != nil {
		for _, ev := range w.events {
			call(w.web, ev.index, uintptr(ev.token))
		}
		call(w.web, 2)
		w.web = nil
	}
	if w.controller != nil {
		call(w.controller, 24)
		call(w.controller, 2)
		w.controller = nil
	}
	if w.env != nil {
		call(w.env, 2)
		w.env = nil
	}
	for _, h := range w.handlers {
		if h.refs.Load() <= 1 {
			h.pin.Unpin()
		}
	}
	runtime.KeepAlive(w)
	return w.err
}
func (w *host) handler(iid string, fn func(uintptr, *object) uintptr) *callback {
	h := newCallback(iid, fn)
	w.handlers = append(w.handlers, h)
	return h
}
func (w *host) add(index int, iid string, fn func(uintptr, *object) uintptr) error {
	h := w.handler(iid, fn)
	var t int64
	r := call(w.web, index, uintptr(unsafe.Pointer(h)), uintptr(unsafe.Pointer(&t)))
	if e := hrError("WebView2 event", r); e != nil {
		return e
	}
	w.events = append(w.events, registration{index + 1, t})
	return nil
}
func (w *host) configure() error {
	var settings *object
	if r := call(w.web, 3, uintptr(unsafe.Pointer(&settings))); int32(r) >= 0 && settings != nil {
		call(settings, 6, 0)
		call(settings, 12, 0)
		call(settings, 14, 0)
		call(settings, 16, 0)
		call(settings, 18, 0)
		call(settings, 2)
	} // no web messages, devtools, context menus or zoom controls
	if e := w.add(7, "9adbe429-f36d-432b-9ddc-f8881fbd76e3", func(_ uintptr, a *object) uintptr {
		u := getString(a, 3)
		if !strings.HasPrefix(u, w.o.Origin+"/") && u != "about:blank" && !strings.HasPrefix(u, "blob:"+w.o.Origin+"/") {
			call(a, 8, 1)
		}
		return 0
	}); e != nil {
		return e
	}
	if e := w.add(15, "d33a35bf-1c49-4f98-93ab-006e0533fe1c", func(_ uintptr, a *object) uintptr {
		var ok int32
		call(a, 3, uintptr(unsafe.Pointer(&ok)))
		if ok != 0 {
			w.loaded = true
			w.checkReady()
		} else {
			var code int32
			call(a, 4, uintptr(unsafe.Pointer(&code)))
			w.fail(fmt.Errorf("WebView2 页面加载失败，状态 %d", code))
		}
		return 0
	}); e != nil {
		return e
	}
	if e := w.add(23, "15e1c6a3-c72a-4df3-91d7-d097fbec6bfd", func(_ uintptr, a *object) uintptr { call(a, 7, 2); return 0 }); e != nil {
		return e
	}
	if e := w.add(44, "d4c185fe-c81c-4989-97af-2d3fa7ab5651", func(_ uintptr, a *object) uintptr { call(a, 6, 1); return 0 }); e != nil {
		return e
	}
	return w.add(25, "79e0aea4-990b-42d9-aa1d-0fcc2e5bc7f1", func(_ uintptr, a *object) uintptr {
		w.fail(errors.New("WebView2 进程异常退出。请关闭并重启工具箱；详情见日志。"))
		return 0
	})
}
func (w *host) fail(e error) {
	if w.err != nil {
		return
	}
	w.err = e
	w.o.Log.Print(e)
	postMessage.Call(w.hwnd.Load(), wmFailure, 0, 0)
}
func (w *host) resize() {
	if w.controller == nil || w.hwnd.Load() == 0 {
		return
	}
	var r rect
	getClient.Call(w.hwnd.Load(), uintptr(unsafe.Pointer(&r)))
	call(w.controller, 6, uintptr(unsafe.Pointer(&r)))
	call(w.controller, 23)
}

var windowProc = syscall.NewCallback(func(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	w := current
	if w == nil {
		r, _, _ := defWindow.Call(hwnd, uintptr(msg), wp, lp)
		return r
	}
	switch msg {
	case 0x83: // WM_NCCALCSIZE: custom titlebar, keep the native resize styles.
		if wp != 0 {
			return 0
		}
	case 0x05: // WM_SIZE
		w.resize()
		if w.o.OnBackground != nil {
			w.o.OnBackground(wp == 1)
		}
		return 0
	case 0x03:
		if w.controller != nil {
			call(w.controller, 23)
		}
	case 0x07:
		if w.controller != nil {
			call(w.controller, 12, 0)
		}
	case 0x02E0: // WM_DPICHANGED: apply the recommended physical rectangle.
		if lp == 0 {
			return 0
		}
		var r rect
		copyNativeMemory.Call(uintptr(unsafe.Pointer(&r)), lp, unsafe.Sizeof(r))
		setPos.Call(hwnd, 0, uintptr(r.L), uintptr(r.T), uintptr(r.R-r.L), uintptr(r.B-r.T), 0x14)
		w.resize()
		return 0
	case 0x24: // WM_GETMINMAXINFO: maximize only into the current work area.
		if lp == 0 {
			return 0
		}
		var m minmax
		copyNativeMemory.Call(uintptr(unsafe.Pointer(&m)), lp, unsafe.Sizeof(m))
		mon, _, _ := monitorFrom.Call(hwnd, 2)
		info := monInfo{Size: uint32(unsafe.Sizeof(monInfo{}))}
		if r, _, _ := monitorInfo.Call(mon, uintptr(unsafe.Pointer(&info))); r != 0 {
			m.MaxPosition = point{info.Work.L - info.Monitor.L, info.Work.T - info.Monitor.T}
			m.MaxSize = point{info.Work.R - info.Work.L, info.Work.B - info.Work.T}
			m.MaxTrack = m.MaxSize
		}
		dpi := uintptr(96)
		if getDPI.Find() == nil {
			dpi, _, _ = getDPI.Call(hwnd)
		}
		minW, minH := int32(480*dpi/96), int32(320*dpi/96)
		if m.MaxSize.X > 0 && minW > m.MaxSize.X {
			minW = m.MaxSize.X
		}
		if m.MaxSize.Y > 0 && minH > m.MaxSize.Y {
			minH = m.MaxSize.Y
		}
		m.MinTrack = point{minW, minH}
		copyNativeMemory.Call(lp, uintptr(unsafe.Pointer(&m)), unsafe.Sizeof(m))
		return 0
	case 0x84: // WM_NCHITTEST; signed coordinates support monitors left of primary.
		z, _, _ := isZoomed.Call(hwnd)
		if z == 0 {
			var r rect
			getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
			x, y := int32(int16(lp&0xffff)), int32(int16((lp>>16)&0xffff))
			dpi := uintptr(96)
			if getDPI.Find() == nil {
				dpi, _, _ = getDPI.Call(hwnd)
			}
			edge := int32(8 * dpi / 96)
			left, right, top, bottom := x < r.L+edge, x >= r.R-edge, y < r.T+edge, y >= r.B-edge
			switch {
			case left && top:
				return 13
			case right && top:
				return 14
			case left && bottom:
				return 16
			case right && bottom:
				return 17
			case left:
				return 10
			case right:
				return 11
			case top:
				return 12
			case bottom:
				return 15
			}
		}
	case 0x113:
		if wp == 1 && !w.ready {
			w.fail(errors.New("WebView2 或前端通信初始化超过 20 秒。请检查 Runtime 安装、用户数据目录权限或使用兼容模式。"))
		}
		return 0
	case wmFailure:
		destroyWindow.Call(hwnd)
		return 0
	case wmCommand:
		switch wp {
		case 1:
			showWindow.Call(hwnd, 6)
		case 2:
			z, _, _ := isZoomed.Call(hwnd)
			if z != 0 {
				showWindow.Call(hwnd, 9)
			} else {
				showWindow.Call(hwnd, 3)
			}
		case 3:
			sendMessage.Call(hwnd, 0x10, 0, 0)
		case 4:
			releaseCapture.Call()
			sendMessage.Call(hwnd, 0xA1, 2, 0)
		case 6:
			w.mounted = true
			w.checkReady()
		case 5:
			if w.web != nil {
				call(w.web, 31)
			}
		}
		return 0
	case 0x10: // WM_CLOSE
		var p placement
		p.Length = uint32(unsafe.Sizeof(p))
		if r, _, _ := getPlacement.Call(hwnd, uintptr(unsafe.Pointer(&p))); r != 0 {
			s := w.o.Config.Get()
			dpi := uintptr(96)
			if getDPI.Find() == nil {
				dpi, _, _ = getDPI.Call(hwnd)
			}
			s.Width = int(p.Normal.R-p.Normal.L) * 96 / int(dpi)
			s.Height = int(p.Normal.B-p.Normal.T) * 96 / int(dpi)
			s.Maximized = p.ShowCmd == 3
			if e := w.o.Config.Update(s); e != nil {
				w.o.Log.Printf("保存窗口配置：%v", e)
			}
		}
		w.closed = true
		destroyWindow.Call(hwnd)
		return 0
	case 0x02:
		w.closed = true
		w.hwnd.Store(0)
		killTimer.Call(hwnd, 1)
		quit.Call(0)
		return 0
	}
	r, _, _ := defWindow.Call(hwnd, uintptr(msg), wp, lp)
	return r
})
