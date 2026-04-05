package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/multica-ai/multicode/server/internal/mcpclient"
	"github.com/multica-ai/multicode/server/internal/tool"
	"github.com/multica-ai/multicode/server/internal/util"
	db "github.com/multica-ai/multicode/server/pkg/db/generated"
)

// initMCPClientManager creates the MCP Client Manager, connects to all
// servers that have status = 'connected' in the database, and returns
// the manager for use during shutdown.
//
// This function is designed to be called during server startup after
// the database queries are initialized.
func initMCPClientManager(ctx context.Context, queries *db.Queries, registry *tool.Registry) *mcpclient.Manager {
	manager := mcpclient.NewManager(registry, mcpclient.DefaultFactory())

	servers, err := queries.ListActiveMCPServers(ctx)
	if err != nil {
		slog.Error("failed to list active MCP servers", "error", err)
		return manager
	}

	if len(servers) == 0 {
		slog.Info("no active MCP servers found")
		return manager
	}

	configs := make([]mcpclient.ServerConfig, 0, len(servers))
	for _, s := range servers {
		var args []string
		if s.Args != nil {
			json.Unmarshal(s.Args, &args)
		}
		if args == nil {
			args = []string{}
		}

		var env map[string]string
		if s.Env != nil {
			json.Unmarshal(s.Env, &env)
		}
		if env == nil {
			env = map[string]string{}
		}

		configs = append(configs, mcpclient.ServerConfig{
			ID:        util.UUIDToString(s.ID),
			Name:      s.Name,
			Transport: s.Transport,
			URL:       s.Url,
			Command:   s.Command,
			Args:      args,
			Env:       env,
		})
	}

	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	connected := manager.ConnectAll(connectCtx, configs)
	slog.Info("MCP client manager initialized", "total", len(configs), "connected", connected)
	return manager
}

// shutdownMCPClientManager gracefully disconnects all MCP clients.
func shutdownMCPClientManager(manager *mcpclient.Manager) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	manager.DisconnectAll(shutdownCtx)
	slog.Info("MCP client manager shut down")
}
