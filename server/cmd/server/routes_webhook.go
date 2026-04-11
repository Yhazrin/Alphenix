package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/alphenix/server/internal/handler"
	"github.com/multica-ai/alphenix/server/internal/middleware"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// registerWebhookRoutes registers webhook CRUD routes.
// These routes use {workspaceID} in the URL path, so workspace membership
// is resolved via RequireWorkspaceMemberFromURL rather than the header-based
// RequireWorkspaceMember used by other workspace-scoped routes.
func registerWebhookRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Route("/api/workspaces/{workspaceID}/webhooks", func(r chi.Router) {
		r.Use(middleware.RequireWorkspaceMemberFromURL(queries, "workspaceID"))

		r.Get("/", h.ListWebhooks)
		r.With(middleware.RequireWorkspaceRoleFromURL(queries, "workspaceID", "owner", "admin")).Post("/", h.CreateWebhook)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetWebhook)
			r.With(middleware.RequireWorkspaceRoleFromURL(queries, "workspaceID", "owner", "admin")).Put("/", h.UpdateWebhook)
			r.With(middleware.RequireWorkspaceRoleFromURL(queries, "workspaceID", "owner", "admin")).Delete("/", h.DeleteWebhook)
		})
	})
}
