//go:build !windows

package hardware

import (
	"bufio"
	"context"
	"os"
	"strings"
)

func Read(ctx context.Context) Inventory {
	v := Base()
	v.Status = "partial"
	v.Groups["system"] = Group{Name: "操作系统", Status: "ok", Items: []map[string]any{{"Caption": "Linux · 开发预览（非 Windows 实机）", "Version": readLine("/proc/version"), "Architecture": v.Architecture}}}
	f, e := os.Open("/proc/cpuinfo")
	name := "暂不可用"
	if e == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "model name") {
				name = strings.TrimSpace(strings.SplitN(sc.Text(), ":", 2)[1])
				break
			}
		}
	}
	v.Groups["cpu"] = Group{Name: "处理器", Status: "ok", Items: []map[string]any{{"Name": name}}}
	for k, n := range map[string]string{"gpu": "显卡", "board": "主板", "memory": "内存条", "disk": "物理磁盘", "display": "显示器", "audio": "声卡", "network": "网卡", "bios": "BIOS/UEFI", "drivers": "驱动"} {
		v.Groups[k] = Group{Name: n, Status: "unsupported", Error: "Linux 开发预览不提供此 Windows CIM 信息", Items: []map[string]any{}}
	}
	return v
}
func readLine(p string) string { b, _ := os.ReadFile(p); return strings.TrimSpace(string(b)) }
