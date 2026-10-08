package session

import (
	"errors"
	"os"
	"testing"
)

func TestHostLeaseRefusesSecondClient(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	first, err := tryAcquireHostLease(dir, "https://jetkvm-ms01.example", "jetkvm-mcp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Release() })

	_, err = tryAcquireHostLease(dir, "https://jetkvm-ms01.example", "jetkvm-desktop")
	if err == nil {
		t.Fatal("expected second lease to fail")
	}
	var held *LocalSessionHeldError
	if !errors.As(err, &held) {
		t.Fatalf("expected LocalSessionHeldError, got %v", err)
	}
	if held.Holder != "jetkvm-mcp" {
		t.Fatalf("holder = %q, want jetkvm-mcp", held.Holder)
	}
}

func TestHostLeaseDifferentHosts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	a, err := tryAcquireHostLease(dir, "host-a", "jetkvm-mcp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Release() })

	b, err := tryAcquireHostLease(dir, "host-b", "jetkvm-desktop")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Release() })
}

func TestNormalizeLeaseHostCanonicalizesSchemeAndDefaultPort(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"http://JETKVM-MS01.example":         "jetkvm-ms01.example",
		"https://jetkvm-ms01.example":        "jetkvm-ms01.example",
		"jetkvm-ms01.example":                "jetkvm-ms01.example",
		"http://jetkvm-ms01.example:80":      "jetkvm-ms01.example",
		"https://jetkvm-ms01.example:443":    "jetkvm-ms01.example",
		"http://jetkvm-ms01.example:8080":    "jetkvm-ms01.example:8080",
		"JETKVM-MS01.EXAMPLE:8080":           "jetkvm-ms01.example:8080",
		"https://jetkvm-ms01.example:8443/":  "jetkvm-ms01.example:8443",
	}
	for in, want := range cases {
		if got := normalizeLeaseHost(in); got != want {
			t.Errorf("normalizeLeaseHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHostLeaseCanonicalizesEquivalentURLs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	first, err := tryAcquireHostLease(dir, "http://jetkvm-ms01.example", "jetkvm-mcp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Release() })

	_, err = tryAcquireHostLease(dir, "JETKVM-MS01.example", "jetkvm-desktop")
	var held *LocalSessionHeldError
	if !errors.As(err, &held) {
		t.Fatalf("expected LocalSessionHeldError for equivalent URL, got %v", err)
	}
}

func TestHostLeaseReleaseKeepsPersistentLockfile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	lease, err := tryAcquireHostLease(dir, "https://release.example", "jetkvm-mcp")
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}

	// The lockfile is persistent; only the OS lock is dropped on Release.
	// Unlinking here would let a released holder delete a live successor's lock.
	if _, err := os.Stat(lease.path); err != nil {
		t.Fatalf("Release removed the lockfile: %v", err)
	}

	next, err := tryAcquireHostLease(dir, "https://release.example", "jetkvm-desktop")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = next.Release() })
}
