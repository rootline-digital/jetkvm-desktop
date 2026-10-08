package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestContenderCannotUnlinkLiveLock recreates the O_EXCL+unlink race: a
// contender sees a freshly-created lockfile whose holder record has not been
// written yet. The winner still holds the OS lock, so the contender must fail
// and must not unlink the live lockfile.
func TestContenderCannotUnlinkLiveLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := hostLeasePath(dir, "https://race.example")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	// Simulate a winner that created the lockfile but has not written its
	// holder record yet (empty file, OS lock still held).
	winner, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = winner.Close() })
	if err := lockFileForTest(winner); err != nil {
		t.Fatalf("test setup lock: %v", err)
	}

	_, err = tryAcquireHostLease(dir, "https://race.example", "jetkvm-desktop")
	var held *LocalSessionHeldError
	if !errors.As(err, &held) {
		t.Fatalf("expected LocalSessionHeldError, got %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("contender unlinked the live lockfile: %v", err)
	}
}
