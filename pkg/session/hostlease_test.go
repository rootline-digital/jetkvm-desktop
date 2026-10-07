package session

import (
	"errors"
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
