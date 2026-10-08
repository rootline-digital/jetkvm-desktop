//go:build !windows

package session

import (
	"os"
	"syscall"
)

func lockFileForTest(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
