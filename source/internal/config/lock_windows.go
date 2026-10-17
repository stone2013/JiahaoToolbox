//go:build windows

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func LockProfile(dir string) (*os.File, error) {
	p, e := syscall.UTF16PtrFromString(filepath.Join(dir, ".instance.lock"))
	if e != nil {
		return nil, e
	}
	h, e := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		return nil, fmt.Errorf("配置目录正在使用，或无法取得独占访问；请关闭另一实例：%w", e)
	}
	return os.NewFile(uintptr(h), "profile-lock"), nil
}
