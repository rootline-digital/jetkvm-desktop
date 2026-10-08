//go:build windows

package session

import "os"

func lockFileForTest(f *os.File) error {
	// Lock the same region production uses so the race test exercises the real
	// mutual-exclusion protocol rather than a stale offset.
	return lockFileExclusiveNB(f)
}
