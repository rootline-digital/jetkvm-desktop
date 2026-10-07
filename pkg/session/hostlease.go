package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
type HostLease struct {
	path   string
	file   *os.File
	client string
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

	const attempts = 2
	for i := 0; i < attempts; i++ {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			payload := fmt.Sprintf("%s=%d\n", clientName, os.Getpid())
			if _, err := f.WriteString(payload); err != nil {
				_ = f.Close()
				_ = os.Remove(path)
				return nil, err
			}
			return &HostLease{path: path, file: f, client: clientName}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		holder, pid := readLeaseRecord(path)
		if pid > 0 && processAlive(pid) {
			return nil, &LocalSessionHeldError{Holder: holder}
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return nil, &LocalSessionHeldError{Holder: readLeaseHolder(path)}
}

func readLeaseRecord(path string) (holder string, pid int) {
	holder = readLeaseHolder(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return holder, 0
	}
	line := strings.TrimSpace(string(data))
	if i := strings.LastIndexByte(line, '='); i > 0 {
		if p, err := strconv.Atoi(strings.TrimSpace(line[i+1:])); err == nil {
			pid = p
		}
	}
	return holder, pid
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
	errClose := h.file.Close()
	h.file = nil
	errRemove := os.Remove(h.path)
	if errClose != nil {
		return errClose
	}
	if errRemove != nil && !errors.Is(errRemove, os.ErrNotExist) {
		return errRemove
	}
	return nil
}
