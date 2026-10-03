package mcpserver

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lkarlslund/jetkvm-desktop/pkg/emulator"
	"github.com/lkarlslund/jetkvm-desktop/pkg/session"
)

func startEmulatorSession(t *testing.T) (*emulator.Server, *session.Controller, context.Context) {
	t.Helper()
	srv, err := emulator.NewServer(emulator.Config{
		ListenAddr: "127.0.0.1:0",
		AuthMode:   emulator.AuthModePassword,
		Password:   "secret",
	})
	if err != nil {
		t.Fatalf("emulator.NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(ctx)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for srv.BaseURL() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.BaseURL() == "" {
		t.Fatal("emulator did not publish base URL")
	}
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-errCh:
			if err != nil && ctx.Err() == nil {
				t.Errorf("emulator: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("emulator did not shut down")
		}
	})

	controller := session.New(session.Config{
		BaseURL:    srv.BaseURL(),
		Password:   "secret",
		RPCTimeout: 2 * time.Second,
		Reconnect:  true,
	})
	controller.Start(ctx)
	waitForPhase(t, controller, session.PhaseConnected, 5*time.Second)
	t.Cleanup(func() { controller.Stop() })
	return srv, controller, ctx
}

func waitForPhase(t *testing.T, controller *session.Controller, want session.Phase, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if controller.Snapshot().Phase == want {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("phase = %s, want %s", controller.Snapshot().Phase, want)
}

func TestMCPToolsPowerAndState(t *testing.T) {
	srv, controller, ctx := startEmulatorSession(t)
	srv.SetActiveExtension("atx-power")
	srv.SetATXState(true, false)

	server := NewInMemoryServer(controller, 5*time.Second)
	clientSession, err := ConnectInMemory(ctx, server)
	if err != nil {
		t.Fatalf("ConnectInMemory: %v", err)
	}
	defer clientSession.Close()

	stateRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "power_state"})
	if err != nil {
		t.Fatalf("power_state: %v", err)
	}
	if stateRes.IsError {
		t.Fatalf("power_state tool error: %+v", stateRes.Content)
	}

	powerRes, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "power",
		Arguments: map[string]any{"action": "reset"},
	})
	if err != nil {
		t.Fatalf("power: %v", err)
	}
	if powerRes.IsError {
		t.Fatalf("power tool error: %+v", powerRes.Content)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, input := range srv.Inputs() {
			if input.Type == "rpc.setATXPowerAction" && input.Data == "reset" {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("expected reset ATX RPC, inputs=%+v", srv.Inputs())
}
