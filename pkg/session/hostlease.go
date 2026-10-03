package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	errLocalSessionHeld = errors.New("local WebRTC session already active")
	lockDirOverride     = ""
)

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
type HostLease struct {
	path   string
	file   *os.File
	client string
}

func localSessionLockDir() (string, error) {
	if lockDirOverride != "" {
		return lockDirOverride, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "jetkvm-desktop", "webrtc-sessions"), nil
}

func hostLeasePath(baseURL string) (string, error) {
	dir, err := localSessionLockDir()
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(normalizeLeaseHost(baseURL)))
	name := hex.EncodeToString(key[:8]) + ".lock"
	return filepath.Join(dir, name), nil
}

func normalizeLeaseHost(baseURL string) string {
	s := strings.TrimSpace(strings.ToLower(baseURL))
	s = strings.TrimSuffix(s, "/")
	return s
}

// TryAcquireHostLease takes an exclusive non-blocking lock for baseURL.
func TryAcquireHostLease(baseURL, clientName string) (*HostLease, error) {
	clientName = strings.TrimSpace(clientName)
	if clientName == "" {
		return nil, errors.New("client name required for local session lease")
	}
	path, err := hostLeasePath(baseURL)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		holder := readLeaseHolder(path)
		return nil, &LocalSessionHeldError{Holder: holder}
	}
	payload := fmt.Sprintf("%s=%d\n", clientName, os.Getpid())
	if _, err := f.Truncate(0); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		return nil, err
	}
	if _, err := f.WriteAt([]byte(payload), 0); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		return nil, err
	}
	return &HostLease{path: path, file: f, client: clientName}, nil
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
	if i := strings.IndexByte(line, '='); i > 0 {
		return strings.TrimSpace(line[:i])
	}
	return line
}

// Release drops the host lease.
func (h *HostLease) Release() error {
	if h == nil || h.file == nil {
		return nil
	}
	errUnlock := syscall.Flock(int(h.file.Fd()), syscall.LOCK_UN)
	errClose := h.file.Close()
	h.file = nil
	if errUnlock != nil {
		return errUnlock
	}
	return errClose
}
