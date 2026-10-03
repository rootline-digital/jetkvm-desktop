package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lkarlslund/jetkvm-desktop/pkg/emulator"
	"github.com/lkarlslund/jetkvm-desktop/pkg/virtualmedia"
)

func TestControllerResetVirtualMediaRPC(t *testing.T) {
	srv, ctx, cancel := startEmulator(t)
	defer cancel()

	controller := New(Config{
		BaseURL:    srv.BaseURL(),
		Password:   "secret",
		RPCTimeout: 2 * time.Second,
		Reconnect:  true,
	})
	controller.Start(ctx)
	defer controller.Stop()

	waitForPhase(t, controller, PhaseConnected, 5*time.Second)

	tempDir := t.TempDir()
	imagePath := filepath.Join(tempDir, "reset-rpc.iso")
	if err := os.WriteFile(imagePath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := controller.UploadStorageFile(imagePath, nil); err != nil {
		t.Fatal(err)
	}
	if err := controller.MountStorageFile("reset-rpc.iso", virtualmedia.ModeCDROM); err != nil {
		t.Fatal(err)
	}

	if err := controller.ResetVirtualMedia(); err != nil {
		t.Fatalf("ResetVirtualMedia: %v", err)
	}
	state, err := controller.GetVirtualMediaState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state != nil {
		t.Fatalf("expected no mount after reset, got %+v", state)
	}
}

func TestControllerResetVirtualMediaRecoversDroppedUnmount(t *testing.T) {
	srv, ctx, cancel := startFaultedEmulator(t, emulator.FaultConfig{
		DropRPCMethod: "unmountImage",
	})
	defer cancel()

	controller := New(Config{
		BaseURL:         srv.BaseURL(),
		Password:        "secret",
		RPCTimeout:      500 * time.Millisecond,
		MutationTimeout: 500 * time.Millisecond,
		Reconnect:       true,
	})
	controller.Start(ctx)
	defer controller.Stop()

	waitForPhase(t, controller, PhaseConnected, 5*time.Second)

	tempDir := t.TempDir()
	imagePath := filepath.Join(tempDir, "wedged.iso")
	if err := os.WriteFile(imagePath, []byte("iso"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := controller.UploadStorageFile(imagePath, nil); err != nil {
		t.Fatal(err)
	}
	if err := controller.MountStorageFile("wedged.iso", virtualmedia.ModeCDROM); err != nil {
		t.Fatal(err)
	}

	// resetVirtualMedia is unknown on this path; recover reconnects and uses the RPC.
	if err := controller.recoverWedgedVirtualMedia(); err != nil {
		t.Fatalf("recoverWedgedVirtualMedia: %v", err)
	}
	state, err := controller.GetVirtualMediaState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state != nil {
		t.Fatalf("expected mount cleared after reconnect recovery, got %+v", state)
	}
}
