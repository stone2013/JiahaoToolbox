//go:build !windows

package systemtools

import (
	"context"
	"errors"
	"net"
	"time"
)

func Launch(ctx context.Context, id string, elevate bool) (LaunchResult, error) {
	return LaunchResult{}, errors.New("此入口仅在 Windows 桌面版可用")
}
func StartupStatus() bool { return false }
func SetStartup(ctx context.Context, enabled bool) error {
	return errors.New("开机启动仅在 Windows 桌面版可用")
}
func OpenBrowser(url string) error {
	return errors.New("请在浏览器中打开终端给出的本地地址")
}
func OpenOfficialReleasePage() error { return errors.New("打开 GitHub Release 仅支持 Windows") }
func checkNetwork(ctx context.Context, kind, target string) (NetResult, error) {
	s := time.Now()
	r := NetResult{Kind: kind, Target: target}
	if kind == "ping" {
		return r, errors.New("Linux 开发预览不执行 Windows Ping")
	}
	a, e := net.DefaultResolver.LookupHost(ctx, target)
	r.Addresses = a
	r.Success = e == nil
	r.DurationMS = time.Since(s).Milliseconds()
	r.Message = "DNS 解析完成"
	if e != nil {
		r.Message = e.Error()
	}
	return r, nil
}
