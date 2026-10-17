//go:build windows

package hardware

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unicode/utf16"
)

// The script is fixed, embedded source; no user supplied command or WQL is ever
// interpolated. CIM runs once on startup / explicit refresh, not per sample.
const inventoryScript = `
$ErrorActionPreference='Stop'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$r = @{}
function Q($key,$name,$class,$props,$ns='root/cimv2') {
 try { $items=@(Get-CimInstance -Namespace $ns -ClassName $class -OperationTimeoutSec 4 -ErrorAction Stop | Select-Object -Property $props)
  $status=if($items.Count -gt 0){'ok'}else{'unsupported'}
  $r[$key]=@{name=$name;status=$status;items=$items}
 } catch { $r[$key]=@{name=$name;status='failed';error=$_.Exception.Message;items=@()} }
}
Q 'system' '操作系统' 'Win32_OperatingSystem' @('Caption','Version','BuildNumber','OSArchitecture','LastBootUpTime')
Q 'cpu' '处理器' 'Win32_Processor' @('Name','Manufacturer','NumberOfCores','NumberOfLogicalProcessors','MaxClockSpeed','SocketDesignation')
Q 'gpu' '显卡' 'Win32_VideoController' @('Name','AdapterCompatibility','DriverVersion','DriverDate','VideoProcessor','CurrentHorizontalResolution','CurrentVerticalResolution','CurrentRefreshRate')
Q 'board' '主板' 'Win32_BaseBoard' @('Manufacturer','Product','Version')
Q 'memory' '内存条' 'Win32_PhysicalMemory' @('Manufacturer','PartNumber','Capacity','Speed','ConfiguredClockSpeed','SMBIOSMemoryType','DeviceLocator')
Q 'disk' '物理磁盘' 'Win32_DiskDrive' @('Model','Size','InterfaceType','MediaType','FirmwareRevision','Index')
Q 'storage' '存储介质' 'MSFT_PhysicalDisk' @('FriendlyName','MediaType','BusType','Size') 'root/Microsoft/Windows/Storage'
Q 'display' '显示器' 'Win32_DesktopMonitor' @('Name','MonitorManufacturer','ScreenWidth','ScreenHeight')
Q 'audio' '声卡' 'Win32_SoundDevice' @('Name','Manufacturer','Status')
Q 'network' '网卡' 'Win32_NetworkAdapter' @('Name','NetConnectionID','AdapterType','PhysicalAdapter','NetEnabled','Speed')
Q 'bios' 'BIOS' 'Win32_BIOS' @('Manufacturer','SMBIOSBIOSVersion','ReleaseDate')
# Avoid enumerating every signed driver, which can be very slow. Device-class
# driver information is already present for display adapters.
$r['drivers']=@{name='显卡驱动';status=$r['gpu'].status;items=@($r['gpu'].items | Select-Object Name,DriverVersion,DriverDate)}
try { $type=Get-ItemPropertyValue 'HKLM:\SYSTEM\CurrentControlSet\Control' 'PEFirmwareType' -ErrorAction Stop
 $r['firmware']=@{name='启动固件';status='ok';items=@(@{Mode=if($type -eq 2){'UEFI'}elseif($type -eq 1){'Legacy BIOS'}else{'Unknown'}})}
} catch { $r['firmware']=@{name='启动固件';status='unsupported';error='该只读接口未提供启动固件类型';items=@()} }
$r | ConvertTo-Json -Depth 6 -Compress
`

func Read(parent context.Context) Inventory {
	v := Base()
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	words := utf16.Encode([]rune(inventoryScript))
	b := make([]byte, len(words)*2)
	for i, w := range words {
		binary.LittleEndian.PutUint16(b[i*2:], w)
	}
	exe := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	cmd := exec.CommandContext(ctx, exe, "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(b))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr limitedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	e := cmd.Run()
	if e != nil {
		v.Status = "failed"
		reason := "CIM 查询失败：" + e.Error()
		if ctx.Err() != nil {
			reason = "CIM 查询已取消或超时"
		}
		v.Groups["system"] = Group{Name: "硬件查询", Status: "failed", Error: reason, Items: []map[string]any{}}
		return v
	}
	groups, e := Parse(bytes.TrimPrefix(stdout.Bytes(), []byte{0xef, 0xbb, 0xbf}))
	if e != nil {
		v.Status = "failed"
		v.Groups["system"] = Group{Name: "硬件查询", Status: "failed", Error: "CIM 输出解析失败：" + e.Error(), Items: []map[string]any{}}
		return v
	}
	v.Groups = groups
	v.Status = "ok"
	for _, g := range groups {
		if g.Status == "failed" {
			v.Status = "partial"
		}
	}
	v.Time = time.Now().UnixMilli()
	return v
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	room := (4 << 20) - b.Len()
	if room > 0 {
		if room > n {
			room = n
		}
		b.Buffer.Write(p[:room])
	}
	return n, nil
}
