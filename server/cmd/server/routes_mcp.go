package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/alphenix/server/internal/handler"
	"github.com/multica-ai/alphenix/server/internal/middleware"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// registerMCPServerRoutes registers MCP server CRUD routes.
func registerMCPServerRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Route("/api/workspaces/{workspaceID}/mcp-servers", func(r chi.Router) {
		r.Get("/", h.ListMCPServers)
		r.With(middleware.RequireWorkspaceRole(queries, "owner", "admin")).Post("/", h.CreateMCPServer)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetMCPServer)
			r.With(middleware.RequireWorkspaceRole(queries, "owner", "admin")).Put("/", h.UpdateMCPServer)
			r.With(middleware.RequireWorkspaceRole(queries, "owner", "admin")).Delete("/", h.DeleteMCPServer)
		})
	})
}
