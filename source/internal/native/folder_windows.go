//go:build windows && amd64

package native

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// SelectFolder uses a fixed PowerShell/WinForms program only to show the OS
// folder picker. No caller-provided value is interpolated into a command.
func SelectFolder(ctx context.Context) (string, error) {
	const script = `[Console]::OutputEncoding=[Text.UTF8Encoding]::UTF8; Add-Type -AssemblyName System.Windows.Forms; $d=New-Object System.Windows.Forms.FolderBrowserDialog; $d.Description='选择嘉豪工具箱测试目录'; $d.ShowNewFolderButton=$true; if($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK){[Console]::Write($d.SelectedPath)}`
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-WindowStyle", "Hidden", "-Command", script)
	b, e := cmd.Output()
	if e != nil {
		return "", e
	}
	path := strings.TrimSpace(string(b))
	if path == "" {
		return "", errors.New("已取消选择文件夹")
	}
	return path, nil
}

func LaunchClassicCMD(directory string, scan bool) error {
	if strings.TrimSpace(directory) == "" {
		return errors.New("缺少已选择的工作目录")
	}
	args := []string{"/D", "/T:0A", "/K", "title Jiahao Classic CMD"}
	if scan {
	args[3] += " & echo 扫描目录: %CD% & dir /s & echo. & echo 扫描结束；关闭窗口即可退出。"
	}
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = os.Getenv("windir")
	}
	if root == "" || !filepath.IsAbs(root) {
		return errors.New("无法确定 Windows 系统目录，未启动 CMD")
	}
	exe := filepath.Join(root, "System32", "cmd.exe")
	st, e := os.Stat(exe)
	if e != nil || st.IsDir() {
		return fmt.Errorf("找不到 Windows 命令提示符：%s", exe)
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = directory
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x10}
	if e := cmd.Start(); e != nil {
		return fmt.Errorf("无法启动独立 CMD 窗口：%w", e)
	}
	if e := cmd.Process.Release(); e != nil {
		return fmt.Errorf("CMD 已启动，但无法释放进程句柄：%w", e)
	}
	return nil
}
