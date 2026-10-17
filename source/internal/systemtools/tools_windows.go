//go:build windows && amd64

package systemtools

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var shell = syscall.NewLazyDLL("shell32.dll")
var shellExec = shell.NewProc("ShellExecuteExW")
var k32 = syscall.NewLazyDLL("kernel32.dll")
var closeHandle = k32.NewProc("CloseHandle")

// SHELLEXECUTEINFOW, Windows x64 ABI; size 112 bytes.
type shellInfo struct {
	Size, Mask                        uint32
	Hwnd                              uintptr
	Verb, File, Parameters, Directory *uint16
	Show                              int32
	_                                 uint32
	Instance, IDList                  uintptr
	Class                             *uint16
	ClassKey                          uintptr
	HotKey                            uint32
	_2                                uint32
	Icon, Process                     uintptr
}

func shellOpen(file, params string, elevate bool) (LaunchResult, error) {
	verb := "open"
	if elevate {
		verb = "runas"
	}
	v, f := syscall.StringToUTF16Ptr(verb), syscall.StringToUTF16Ptr(file)
	s := shellInfo{Size: uint32(unsafe.Sizeof(shellInfo{})), Mask: 0x40 | 0x100 | 0x400, Verb: v, File: f, Show: 1}
	if params != "" {
		s.Parameters = syscall.StringToUTF16Ptr(params)
	}
	r, _, e := shellExec.Call(uintptr(unsafe.Pointer(&s)))
	if s.Process != 0 {
		defer closeHandle.Call(s.Process)
	}
	if r == 0 {
		if e == syscall.Errno(740) {
			return LaunchResult{NeedsElevation: true}, errors.New("Windows 要求管理员权限；需确认后以管理员身份打开")
		}
		if e == syscall.Errno(1223) {
			return LaunchResult{}, errors.New("用户取消了权限请求")
		}
		return LaunchResult{}, fmt.Errorf("Windows 未接受启动请求：%w", e)
	}
	return LaunchResult{Message: "Windows 已接受启动请求。外部工具的内部操作由 Windows 管理。"}, nil
}
func Launch(ctx context.Context, id string, elevate bool) (LaunchResult, error) {
	if ctx.Err() != nil {
		return LaunchResult{}, ctx.Err()
	}
	t, ok := Find(id)
	if !ok {
		return LaunchResult{}, errors.New("未知工具")
	}
	if elevate && !t.Admin {
		return LaunchResult{}, errors.New("此入口不需要提权")
	}
	root := filepath.Join(os.Getenv("SystemRoot"), "System32")
	exe, params := "", ""
	switch id {
	case "task-manager":
		exe = filepath.Join(root, "Taskmgr.exe")
	case "device-manager":
		exe = filepath.Join(root, "mmc.exe")
		params = `"` + filepath.Join(root, "devmgmt.msc") + `"`
	case "disk-management":
		exe = filepath.Join(root, "mmc.exe")
		params = `"` + filepath.Join(root, "diskmgmt.msc") + `"`
	case "system-information":
		exe = filepath.Join(root, "msinfo32.exe")
	case "windows-update":
		exe = "ms-settings:windowsupdate"
	case "network-settings":
		exe = "ms-settings:network-status"
	case "power-settings":
		exe = filepath.Join(root, "control.exe")
		params = "powercfg.cpl"
	case "startup-apps":
		exe = "ms-settings:startupapps"
	case "services":
		exe = filepath.Join(root, "mmc.exe")
		params = `"` + filepath.Join(root, "services.msc") + `"`
	case "event-viewer":
		exe = filepath.Join(root, "mmc.exe")
		params = `"` + filepath.Join(root, "eventvwr.msc") + `"`
	}
	return shellOpen(exe, params, elevate)
}
func OpenBrowser(url string) error {
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		return errors.New("只能打开本程序的本地地址")
	}
	_, e := shellOpen(url, "", false)
	return e
}

// OpenOfficialReleasePage uses a fixed repository URL; it never accepts caller-provided URLs.
func OpenOfficialReleasePage() error {
	_, e := shellOpen("https://github.com/stone2013/JiahaoToolbox/releases", "", false)
	return e
}
func startupPath() string {
	return filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "JiahaoToolbox.lnk")
}
func StartupStatus() bool { s, e := os.Stat(startupPath()); return e == nil && !s.IsDir() }
func SetStartup(ctx context.Context, enabled bool) error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	script := `$ErrorActionPreference='Stop'; $path=$env:JIAHAO_STARTUP_LINK; $shell=New-Object -ComObject WScript.Shell; if(Test-Path -LiteralPath $path){$old=$shell.CreateShortcut($path); if($old.Description -ne 'Jiahao Toolbox application startup'){throw 'A different shortcut already exists; not modified.'}}; `
	if enabled {
		script += `$link=$shell.CreateShortcut($path); $link.TargetPath=$env:JIAHAO_STARTUP_EXE; $link.WorkingDirectory=[IO.Path]::GetDirectoryName($env:JIAHAO_STARTUP_EXE); $link.Description='Jiahao Toolbox application startup'; $link.Save()`
	} else {
		script += `if(Test-Path -LiteralPath $path){Remove-Item -LiteralPath $path -ErrorAction Stop}`
	}
	u := utf16.Encode([]rune(script))
	b := make([]byte, len(u)*2)
	for i, n := range u {
		binary.LittleEndian.PutUint16(b[2*i:], n)
	}
	cmd := exec.CommandContext(ctx, filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(b))
	cmd.Env = append(os.Environ(), "JIAHAO_STARTUP_LINK="+startupPath(), "JIAHAO_STARTUP_EXE="+exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.WaitDelay = time.Second
	if e = cmd.Run(); e != nil {
		return fmt.Errorf("更改当前用户启动快捷方式失败：%w", e)
	}
	return nil
}
func checkNetwork(ctx context.Context, kind, target string) (NetResult, error) {
	start := time.Now()
	r := NetResult{Kind: kind, Target: target}
	if kind == "dns" {
		a, e := net.DefaultResolver.LookupHost(ctx, target)
		r.Success = e == nil
		r.Addresses = a
		r.Message = "DNS 解析完成"
		if e != nil {
			r.Message = e.Error()
		}
		r.DurationMS = time.Since(start).Milliseconds()
		return r, nil
	}
	cmd := exec.CommandContext(ctx, filepath.Join(os.Getenv("SystemRoot"), "System32", "PING.EXE"), "-n", "1", "-w", "2000", target)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.WaitDelay = time.Second
	b, e := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return r, ctx.Err()
	}
	r.Success = e == nil
	r.Message = decodeOEM(b)
	if r.Message == "" && e != nil {
		r.Message = e.Error()
	}
	r.DurationMS = time.Since(start).Milliseconds()
	return r, nil
}
func decodeOEM(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	p := k32.NewProc("MultiByteToWideChar")
	n, _, _ := p.Call(1, 0, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0, 0)
	if n == 0 {
		return string(b)
	}
	u := make([]uint16, n)
	p.Call(1, 0, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(unsafe.Pointer(&u[0])), n)
	return string(utf16.Decode(u))
}
