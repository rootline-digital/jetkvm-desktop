package session

import (
	"errors"
	"testing"
)

func TestHostLeaseRefusesSecondClient(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockDirOverride = dir
	t.Cleanup(func() { lockDirOverride = "" })

	first, err := TryAcquireHostLease("https://jetkvm-ms01.example", "jetkvm-mcp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Release() })

	_, err = TryAcquireHostLease("https://jetkvm-ms01.example", "jetkvm-desktop")
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
	lockDirOverride = dir
	t.Cleanup(func() { lockDirOverride = "" })

	a, err := TryAcquireHostLease("host-a", "jetkvm-mcp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Release() })

	b, err := TryAcquireHostLease("host-b", "jetkvm-desktop")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Release() })
}
