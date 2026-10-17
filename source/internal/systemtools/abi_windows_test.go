//go:build windows && amd64

package systemtools

import (
	"testing"
	"unsafe"
)

func TestShellExecuteInfoABI(t *testing.T) {
	var s shellInfo
	if unsafe.Sizeof(s) != 112 || unsafe.Offsetof(s.Process) != 104 {
		t.Fatalf("SHELLEXECUTEINFOW size %d process %d", unsafe.Sizeof(s), unsafe.Offsetof(s.Process))
	}
}
