//go:build !windows && !linux && !darwin && !freebsd

package config

import (
	"errors"
	"os"
)

func LockProfile(string) (*os.File, error) {
	return nil, errors.New("当前平台未实现独占配置锁")
}
