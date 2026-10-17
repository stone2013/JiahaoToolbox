//go:build windows && amd64

package native

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// Prefer Microsoft's public loader when supplied. The single-EXE Beta has a
// compatibility loader for the installed runtime's documented INTERNAL export.
// That export can change. This is a release blocker until Windows verification.
func (w *host) createEnvironment(handler *callback) error {
	return w.createEnvironmentWithRuntime(handler, findRuntime())
}

// Runtime path is explicit here so the missing-runtime error path can be tested offline.
func (w *host) createEnvironmentWithRuntime(handler *callback, dllPath string) error {
	p := syscall.StringToUTF16Ptr(filepath.Join(w.o.DataDir, "WebView2"))
	exe, _ := os.Executable()
	loader := filepath.Join(filepath.Dir(exe), "WebView2Loader.dll")
	if _, e := os.Stat(loader); e == nil {
		dll, e := syscall.LoadDLL(loader)
		if e != nil {
			return e
		}
		proc, e := dll.FindProc("CreateCoreWebView2EnvironmentWithOptions")
		if e != nil {
			dll.Release()
			return e
		}
		w.dll = dll
		r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(handler)))
		w.o.Log.Print("WebView2 loader: public SDK sidecar")
		return hrError("CreateCoreWebView2EnvironmentWithOptions", r)
	}
	if dllPath == "" {
		return fmt.Errorf("未找到 x64 Microsoft Edge WebView2 Runtime。请安装 Microsoft 官方 Evergreen Runtime x64 后重新运行。核心功能不需要联网；运行组件需预先安装。")
	}
	dll, e := syscall.LoadDLL(dllPath)
	if e != nil {
		return fmt.Errorf("加载 WebView2 Runtime 失败：%w", e)
	}
	proc, e := dll.FindProc("CreateWebViewEnvironmentWithOptionsInternal")
	if e != nil {
		dll.Release()
		return fmt.Errorf("当前 WebView2 Runtime 不提供兼容入口，请使用公开 SDK Loader 构建或兼容模式：%w", e)
	}
	w.dll = dll
	// Only process-local environment variables are affected; no registry changes.
	for _, key := range []string{"WEBVIEW2_PIPE_FOR_SCRIPT_DEBUGGER", "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "WEBVIEW2_BROWSER_EXECUTABLE_FOLDER", "WEBVIEW2_USER_DATA_FOLDER"} {
		os.Setenv(key, "")
	}
	w.o.Log.Print("WebView2 loader: installed runtime internal export (Beta, not Windows-validated)")
	r, _, _ := proc.Call(1, 0, uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(handler)))
	return hrError("CreateWebViewEnvironmentWithOptionsInternal", r)
}
func findRuntime() string {
	roots := []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LOCALAPPDATA")}
	paths := []string{}
	for _, base := range roots {
		if base == "" {
			continue
		}
		root := filepath.Join(base, "Microsoft", "EdgeWebView", "Application")
		dirs, _ := os.ReadDir(root)
		for _, d := range dirs {
			if !d.IsDir() {
				continue
			}
			if _, e := strconv.Atoi(strings.Split(d.Name(), ".")[0]); e != nil {
				continue
			}
			for _, suffix := range []string{filepath.Join("EBWebView", "x64", "EmbeddedBrowserWebView.dll"), filepath.Join("EBWebView", "EmbeddedBrowserWebView.dll"), "EmbeddedBrowserWebView.dll"} {
				p := filepath.Join(root, d.Name(), suffix)
				if s, e := os.Stat(p); e == nil && !s.IsDir() {
					paths = append(paths, p)
				}
			}
		}
	}
	sort.Slice(paths, func(i, j int) bool { return runtimeVersion(paths[i]) > runtimeVersion(paths[j]) })
	if len(paths) > 0 {
		return paths[0]
	}
	return ""
}
func runtimeVersion(p string) uint64 {
	parts := strings.Split(filepath.ToSlash(p), "/")
	for _, s := range parts {
		n := strings.Split(s, ".")
		if len(n) != 4 {
			continue
		}
		v := uint64(0)
		ok := true
		for _, part := range n {
			a, e := strconv.ParseUint(part, 10, 16)
			if e != nil {
				ok = false
				break
			}
			v = (v << 16) | a
		}
		if ok {
			return v
		}
	}
	return 0
}
