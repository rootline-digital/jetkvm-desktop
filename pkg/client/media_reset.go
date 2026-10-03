package client

import "context"

// ResetVirtualMedia asks the device to force-clear a wedged virtual-media stack.
// Older firmware returns method not found; callers should fall back to reconnect + unmount.
func (c *Client) ResetVirtualMedia(ctx context.Context) error {
	return c.Call(ctx, "resetVirtualMedia", nil, nil)
}
