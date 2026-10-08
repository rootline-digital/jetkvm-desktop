//go:build windows

package session

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

var errLocked = errors.New("host lease already locked")

// leaseLockOffset is the byte locked by LockFileEx. Windows byte-range locks
// are mandatory: a contender opening its own handle cannot read a locked byte.
// The holder record is written at offset 0, so the lock lives far beyond it and
// never blocks readLeaseHolder from reporting the holder's name.
const leaseLockOffset = 1 << 12

func leaseLockOverlapped() *windows.Overlapped {
	return &windows.Overlapped{Offset: leaseLockOffset}
}

func lockFileExclusiveNB(f *os.File) error {
	ol := leaseLockOverlapped()
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		ol,
	)
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return errLocked
	}
	return err
}

func unlockFile(f *os.File) error {
	ol := leaseLockOverlapped()
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
}
