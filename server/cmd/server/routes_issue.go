package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/alphenix/server/internal/handler"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// registerIssueRoutes registers issue-related API routes.
// Callers must ensure auth middleware is already applied.
func registerIssueRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Route("/api/issues", func(r chi.Router) {
		r.Get("/", h.ListIssues)
		r.Post("/", h.CreateIssue)
		r.Post("/batch-update", h.BatchUpdateIssues)
		r.Post("/batch-delete", h.BatchDeleteIssues)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetIssue)
			r.Put("/", h.UpdateIssue)
			r.Delete("/", h.DeleteIssue)
			r.Post("/comments", h.CreateComment)
			r.Get("/comments", h.ListComments)
			r.Get("/timeline", h.ListTimeline)
			r.Get("/subscribers", h.ListIssueSubscribers)
			r.Post("/subscribe", h.SubscribeToIssue)
			r.Post("/unsubscribe", h.UnsubscribeFromIssue)
			r.Get("/active-task", h.GetActiveTaskForIssue)
			r.Post("/tasks/{taskId}/cancel", h.CancelTask)
			r.Get("/task-runs", h.ListTasksByIssue)
			r.Post("/reactions", h.AddIssueReaction)
			r.Delete("/reactions", h.RemoveIssueReaction)
			r.Get("/attachments", h.ListAttachments)
			r.Post("/decompose", h.Decompose)
			r.Get("/decompose/{runId}", h.GetDecomposeResult)
			r.Post("/decompose/confirm", h.ConfirmDecompose)
		})
	})
}
