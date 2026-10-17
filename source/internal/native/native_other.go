//go:build !windows

package native

import (
	"context"
	"errors"
)

type stub struct{}

func New(o Options) Window { return &stub{} }
func (*stub) Run(ctx context.Context) error {
	return errors.New("本平台不提供 Windows 原生窗口，请使用 --serve")
}
func (*stub) Command(s string) error { return errors.New("浏览器预览没有原生窗口操作") }
func Alert(s string)                 {}

func StartupTheme(theme string) string { return theme }
func ReducedMotion() bool              { return false }
func SelectFolder(context.Context) (string, error) {
	return "", errors.New("仅 Windows 支持选择本机文件夹")
}
func LaunchClassicCMD(string, bool) error { return errors.New("仅 Windows 支持经典 CMD 工具") }
