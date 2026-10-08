package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func isVirtualMediaRPCTimedOut(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

func isVirtualMediaTransientTransport(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "closed pipe") ||
		strings.Contains(msg, "rpc data channel not ready") ||
		strings.Contains(msg, "client not connected")
}

func (c *Controller) waitForRPCReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		current := c.clientIfConnected()
		if current == nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := current.Ping(ctx)
		cancel()
		if err == nil {
			return nil
		}
		if !isVirtualMediaRPCTimedOut(err) && !isVirtualMediaTransientTransport(err) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("rpc not ready after reconnect")
}

func isVirtualMediaMethodNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Method not found") || strings.Contains(msg, "-32601")
}

func (c *Controller) tryUnmountMedia(timeout time.Duration) error {
	current := c.clientIfConnected()
	if current == nil {
		return errors.New("client not connected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return current.UnmountMedia(ctx)
}

func (c *Controller) cleanupIncompleteUploads() error {
	files, err := c.ListStorageFiles(context.Background())
	if err != nil {
		return err
	}
	for _, file := range files {
		if !strings.HasSuffix(strings.ToLower(file.Filename), ".incomplete") {
			continue
		}
		if err := c.DeleteStorageFile(file.Filename); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) waitForPhase(phase Phase, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.Snapshot().Phase == phase {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for phase %v", phase)
}

func (c *Controller) recoverWedgedVirtualMedia() error {
	if err := c.tryUnmountMedia(3 * time.Second); err == nil {
		return c.cleanupIncompleteUploads()
	} else if !isVirtualMediaRPCTimedOut(err) {
		return err
	}

	c.ReconnectNow()
	if err := c.waitForPhase(PhaseConnected, 30*time.Second); err != nil {
		return err
	}
	if err := c.waitForRPCReady(10 * time.Second); err != nil {
		return err
	}
	if current := c.clientIfConnected(); current != nil {
		ctx, cancel := context.WithTimeout(context.Background(), c.cfg.MutationTimeout)
		resetErr := current.ResetVirtualMedia(ctx)
		cancel()
		if resetErr == nil {
			return c.cleanupIncompleteUploads()
		}
		if !isVirtualMediaMethodNotFound(resetErr) && !isVirtualMediaRPCTimedOut(resetErr) {
			return resetErr
		}
	}
	if err := c.tryUnmountMedia(c.cfg.MutationTimeout); err != nil {
		return fmt.Errorf("virtual media still wedged after reconnect: %w", err)
	}
	return c.cleanupIncompleteUploads()
}

// ResetVirtualMedia clears a wedged virtual-media RPC without rebooting the JetKVM device.
func (c *Controller) ResetVirtualMedia() error {
	current := c.clientIfConnected()
	if current == nil {
		return errors.New("client not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.MutationTimeout)
	err := current.ResetVirtualMedia(ctx)
	cancel()
	if err == nil {
		return c.cleanupIncompleteUploads()
	}
	if !isVirtualMediaMethodNotFound(err) && !isVirtualMediaRPCTimedOut(err) {
		return err
	}
	return c.recoverWedgedVirtualMedia()
}
