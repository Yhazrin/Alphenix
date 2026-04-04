package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multicode/server/internal/handler"
	db "github.com/multica-ai/multicode/server/pkg/db/generated"
)

// registerTeamRoutes registers team management routes.
func registerTeamRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Route("/api/teams", func(r chi.Router) {
		r.Get("/", h.ListTeams)
		r.Post("/", h.CreateTeam)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetTeam)
			r.Patch("/", h.UpdateTeam)
			r.Post("/archive", h.ArchiveTeam)
			r.Post("/restore", h.RestoreTeam)
			r.Post("/members", h.AddTeamMember)
			r.Delete("/members/{agentId}", h.RemoveTeamMember)
			r.Post("/lead", h.SetTeamLead)
		})
	})
}
