package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/lkarlslund/jetkvm-desktop/pkg/logging"
	"github.com/lkarlslund/jetkvm-desktop/pkg/mcpserver"
	"github.com/lkarlslund/jetkvm-desktop/pkg/session"
)

const defaultPasswordEnv = "JETKVM_PASSWORD"

// resolvePassword resolves the local-auth password from the environment only.
// Reading passwords from stdin is intentionally unsupported: stdio is the MCP
// JSON-RPC channel, and consuming it would block the server before it serves.
func resolvePassword(passwordEnv string, getenv func(string) string) string {
	if passwordEnv != "" {
		return getenv(passwordEnv)
	}
	return getenv(defaultPasswordEnv)
}

func main() {
	var (
		host           string
		logLevel       string
		passwordEnv    string
		rpcTimeout     time.Duration
		toolTimeout    time.Duration
		connectTimeout time.Duration
	)

	rootCmd := &cobra.Command{
		Use:   "jetkvm-mcp",
		Short: "JetKVM MCP server (stdio)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				host = args[0]
			} else if len(args) > 1 {
				return errors.New("at most one host argument")
			}
			if strings.TrimSpace(host) == "" {
				return errors.New("JetKVM host is required (--host or positional argument)")
			}

			password := resolvePassword(passwordEnv, os.Getenv)

			if err := logging.Configure(logLevel); err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return mcpserver.Run(ctx, mcpserver.Options{
				Session: session.Config{
					BaseURL:            host,
					Password:           password,
					RPCTimeout:         rpcTimeout,
					Reconnect:          true,
					LocalSessionClient: "jetkvm-mcp",
				},
				ToolTimeout:    toolTimeout,
				ConnectTimeout: connectTimeout,
			})
		},
	}

	rootCmd.Flags().StringVar(&host, "host", "", "JetKVM base URL or hostname")
	rootCmd.Flags().StringVar(&passwordEnv, "password-env", "", fmt.Sprintf("Read password from env var (default fallback: %s)", defaultPasswordEnv))
	rootCmd.Flags().StringVar(&logLevel, "log-level", "", "Log level (error, warn, info, debug, trace)")
	rootCmd.Flags().DurationVar(&rpcTimeout, "rpc-timeout", 5*time.Second, "Timeout for JetKVM JSON-RPC")
	rootCmd.Flags().DurationVar(&toolTimeout, "tool-timeout", 15*time.Second, "Default MCP tool call timeout")
	rootCmd.Flags().DurationVar(&connectTimeout, "connect-timeout", 30*time.Second, "Timeout waiting for WebRTC session")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
