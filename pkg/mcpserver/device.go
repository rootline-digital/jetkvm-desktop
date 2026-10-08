package mcpserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lkarlslund/jetkvm-desktop/pkg/session"
)

// powerDevice is the narrow surface MCP power tools need. Tests inject a fake;
// production uses a session-backed holder that connects on first use.
type powerDevice interface {
	setATXPower(ctx context.Context, action session.ATXPowerAction) error
	getATXState(ctx context.Context) (*session.ATXState, error)
}

type sessionHolder struct {
	mu sync.Mutex

	cfg            session.Config
	connectTimeout time.Duration
	runCtx         context.Context
	ctrl           *session.Controller
	started        bool
}

func newSessionHolder(runCtx context.Context, cfg session.Config, connectTimeout time.Duration) *sessionHolder {
	if connectTimeout <= 0 {
		connectTimeout = 30 * time.Second
	}
	return &sessionHolder{
		cfg:            cfg,
		connectTimeout: connectTimeout,
		runCtx:         runCtx,
	}
}

func (h *sessionHolder) ensureStarted() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return
	}
	h.ctrl = session.New(h.cfg)
	h.ctrl.Start(h.runCtx)
	h.started = true
}

func (h *sessionHolder) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ctrl != nil {
		h.ctrl.Stop()
		h.ctrl = nil
	}
	h.started = false
}

func (h *sessionHolder) waitConnected(ctx context.Context) (*session.Controller, error) {
	h.ensureStarted()
	deadline := time.Now().Add(h.connectTimeout)
	for {
		h.mu.Lock()
		ctrl := h.ctrl
		h.mu.Unlock()
		if ctrl == nil {
			return nil, fmt.Errorf("session not started")
		}
		snap := ctrl.Snapshot()
		switch snap.Phase {
		case session.PhaseConnected:
			return ctrl, nil
		case session.PhaseAuthFailed, session.PhaseFatal:
			return nil, fmt.Errorf("jetkvm session %s", snap.Phase)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for jetkvm connection (phase %s)", snap.Phase)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (h *sessionHolder) setATXPower(ctx context.Context, action session.ATXPowerAction) error {
	ctrl, err := h.waitConnected(ctx)
	if err != nil {
		return err
	}
	return ctrl.SetATXPowerActionContext(ctx, action)
}

func (h *sessionHolder) getATXState(ctx context.Context) (*session.ATXState, error) {
	ctrl, err := h.waitConnected(ctx)
	if err != nil {
		return nil, err
	}
	return ctrl.GetATXState(ctx)
}

type controllerDevice struct {
	ctrl *session.Controller
}

func (d *controllerDevice) setATXPower(ctx context.Context, action session.ATXPowerAction) error {
	if d.ctrl == nil {
		return fmt.Errorf("controller not connected")
	}
	return d.ctrl.SetATXPowerAction(action)
}

func (d *controllerDevice) getATXState(ctx context.Context) (*session.ATXState, error) {
	if d.ctrl == nil {
		return nil, fmt.Errorf("controller not connected")
	}
	return d.ctrl.GetATXState(ctx)
}
