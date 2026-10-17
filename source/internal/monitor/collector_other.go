//go:build !windows && !linux

package monitor

import "errors"

func NewCollector() (Collector, error) {
	return nil, errors.New("仅支持 Windows；Linux 仅用于开发预览")
}
