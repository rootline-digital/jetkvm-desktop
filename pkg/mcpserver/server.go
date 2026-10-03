// Package mcpserver exposes JetKVM ATX power controls over MCP stdio.
package mcpserver

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lkarlslund/jetkvm-desktop/pkg/session"
)

const serverVersion = "0.1.0"

// Options configures the MCP stdio server and its JetKVM session.
type Options struct {
	Session        session.Config
	ToolTimeout    time.Duration
	ConnectTimeout time.Duration
}

// Run serves MCP tools on stdio until ctx is canceled. Stdout is reserved for
// JSON-RPC; diagnostics go to stderr.
func Run(ctx context.Context, opts Options) error {
	log.SetOutput(os.Stderr)

	timeout := opts.ToolTimeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	holder := newSessionHolder(ctx, opts.Session, opts.ConnectTimeout)
	defer holder.Stop()

	server := newServer(holder, timeout)
	return server.Run(ctx, &mcp.StdioTransport{})
}

func newServer(client powerDevice, timeout time.Duration) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "jetkvm-mcp",
		Version: serverVersion,
	}, nil)
	registerPowerTools(server, client, timeout)
	return server
}

// NewInMemoryServer builds an MCP server backed by an existing controller for tests.
func NewInMemoryServer(ctrl *session.Controller, timeout time.Duration) *mcp.Server {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return newServer(&controllerDevice{ctrl: ctrl}, timeout)
}

// ConnectInMemory pairs a test client session with serverTransport for tool calls.
func ConnectInMemory(ctx context.Context, server *mcp.Server) (*mcp.ClientSession, error) {
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		return nil, fmt.Errorf("server connect: %w", err)
	}
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "jetkvm-mcp-test"}, nil)
	cs, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		return nil, fmt.Errorf("client connect: %w", err)
	}
	return cs, nil
}
