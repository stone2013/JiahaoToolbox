//go:build linux || darwin || freebsd

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func LockProfile(dir string) (*os.File, error) {
	f, e := os.OpenFile(filepath.Join(dir, ".instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, fmt.Errorf("配置目录正被另一实例使用：%w", e)
	}
	return f, nil
}
