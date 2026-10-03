package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lkarlslund/jetkvm-desktop/pkg/session"
)

func boolPtr(b bool) *bool { return &b }

func withDefaultTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func textResult(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}},
	}
}

func errorResult(err error) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		IsError: true,
	}, nil, nil
}

func parseATXPowerAction(action string) (session.ATXPowerAction, error) {
	switch strings.TrimSpace(action) {
	case string(session.ATXPowerActionShortPress):
		return session.ATXPowerActionShortPress, nil
	case string(session.ATXPowerActionLongPress):
		return session.ATXPowerActionLongPress, nil
	case string(session.ATXPowerActionReset):
		return session.ATXPowerActionReset, nil
	default:
		return "", fmt.Errorf(
			"action must be one of %q, %q, %q",
			session.ATXPowerActionShortPress,
			session.ATXPowerActionLongPress,
			session.ATXPowerActionReset,
		)
	}
}

func registerPowerTools(server *mcp.Server, client powerDevice, timeout time.Duration) {
	destructive := &mcp.ToolAnnotations{
		ReadOnlyHint:    false,
		DestructiveHint: boolPtr(true),
		IdempotentHint:  false,
	}
	readOnly := &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: boolPtr(false),
		IdempotentHint:  true,
	}

	type powerArgs struct {
		Action string `json:"action"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name: "power",
		Description: "Send an ATX front-panel power signal through the JetKVM ATX extension. " +
			"Requires the ATX board wired to the motherboard header.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []string{
						string(session.ATXPowerActionShortPress),
						string(session.ATXPowerActionLongPress),
						string(session.ATXPowerActionReset),
					},
					"description": "power-short taps the button; power-long holds ~5s; reset pulses reset",
				},
			},
			"required":             []string{"action"},
			"additionalProperties": false,
		},
		Annotations: destructive,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args powerArgs) (*mcp.CallToolResult, any, error) {
		ctx, cancel := withDefaultTimeout(ctx, timeout)
		defer cancel()
		action, err := parseATXPowerAction(args.Action)
		if err != nil {
			return errorResult(err)
		}
		if err := client.setATXPower(ctx, action); err != nil {
			return errorResult(err)
		}
		return textResult("ok action=%s", action), map[string]any{"action": string(action)}, nil
	})

	type powerStateArgs struct{}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "power_state",
		Description: "Read ATX power and HDD LED lines from the JetKVM ATX extension.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
		},
		Annotations: readOnly,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ powerStateArgs) (*mcp.CallToolResult, any, error) {
		ctx, cancel := withDefaultTimeout(ctx, timeout)
		defer cancel()
		state, err := client.getATXState(ctx)
		if err != nil {
			return errorResult(err)
		}
		if state == nil {
			return errorResult(fmt.Errorf("ATX state unavailable"))
		}
		return textResult("power=%v hdd=%v", state.Power, state.HDD), map[string]any{
			"power": state.Power,
			"hdd":   state.HDD,
		}, nil
	})
}
