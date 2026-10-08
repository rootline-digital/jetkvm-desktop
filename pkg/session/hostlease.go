package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errLocalSessionHeld = errors.New("local WebRTC session already active")

// LocalSessionHeldError is returned when another local client already holds the host lease.
type LocalSessionHeldError struct {
	Holder string
}

func (e *LocalSessionHeldError) Error() string {
	if e.Holder == "" {
		return "another local JetKVM client already holds the WebRTC session for this host"
	}
	return fmt.Sprintf("local WebRTC session held by %s", e.Holder)
}

func (e *LocalSessionHeldError) Is(target error) bool {
	return target == errLocalSessionHeld
}

// HostLease serializes WebRTC connects to one JetKVM host per machine.
//
// The lease is an OS advisory lock held on a persistent lockfile. The lockfile
// is never unlinked: the kernel releases the lock when the owning process dies,
// so stale-file recovery is unnecessary and cannot race with acquisition.
type HostLease struct {
	path string
	file *os.File
}

func localSessionLockDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "jetkvm-desktop", "webrtc-sessions"), nil
}

func hostLeasePath(dir, baseURL string) string {
	key := sha256.Sum256([]byte(normalizeLeaseHost(baseURL)))
	name := hex.EncodeToString(key[:8]) + ".lock"
	return filepath.Join(dir, name)
}

func normalizeLeaseHost(baseURL string) string {
	s := strings.TrimSpace(strings.ToLower(baseURL))
	s = strings.TrimSuffix(s, "/")
	return s
}

// TryAcquireHostLease takes an exclusive non-blocking lock for baseURL,
// using the per-user cache directory to serialize sessions on this machine.
func TryAcquireHostLease(baseURL, clientName string) (*HostLease, error) {
	dir, err := localSessionLockDir()
	if err != nil {
		return nil, err
	}
	return tryAcquireHostLease(dir, baseURL, clientName)
}

// tryAcquireHostLease is the directory-injectable core shared by
// TryAcquireHostLease and the unit tests, which pass a temp dir so they do not
// touch (or serialize against) the real per-user cache.
func tryAcquireHostLease(dir, baseURL, clientName string) (*HostLease, error) {
	clientName = strings.TrimSpace(clientName)
	if clientName == "" {
		return nil, errors.New("client name required for local session lease")
	}
	path := hostLeasePath(dir, baseURL)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}

	// Keep the lockfile in place for the lifetime of the machine model: the
	// lock is the OS flock, not the file's existence. Never O_EXCL+unlink here.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}

	if err := lockFileExclusiveNB(f); err != nil {
		_ = f.Close()
		if errors.Is(err, errLocked) {
			return nil, &LocalSessionHeldError{Holder: readLeaseHolder(path)}
		}
		return nil, err
	}

	// Record the holder for the contender's error message. Best effort only:
	// the record is a human-readable hint, not part of the mutual-exclusion
	// protocol, so a contender reading during this tiny window reports an empty
	// holder rather than a wrong one.
	if err := f.Truncate(0); err != nil {
		_ = unlockFile(f)
		_ = f.Close()
		return nil, err
	}
	if _, err := f.WriteString(clientName + "\n"); err != nil {
		_ = unlockFile(f)
		_ = f.Close()
		return nil, err
	}
	return &HostLease{path: path, file: f}, nil
}

func readLeaseHolder(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	if line == "" {
		return ""
	}
	return line
}

// Release drops the host lease. It never unlinks the lockfile: unlinking would
// let a released holder delete a lockfile that a successor has already locked.
func (h *HostLease) Release() error {
	if h == nil || h.file == nil {
		return nil
	}
	f := h.file
	h.file = nil
	if err := unlockFile(f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
