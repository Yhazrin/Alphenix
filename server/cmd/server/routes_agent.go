package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multicode/server/internal/handler"
	"github.com/multica-ai/multicode/server/internal/middleware"
	db "github.com/multica-ai/multicode/server/pkg/db/generated"
)

// registerAgentRoutes registers agent and task API routes.
// Callers must ensure auth middleware is already applied.
func registerAgentRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	// Agents
	r.Route("/api/agents", func(r chi.Router) {
		r.Get("/", h.ListAgents)
		r.With(middleware.RequireWorkspaceRole(queries, "owner", "admin")).Post("/", h.CreateAgent)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetAgent)
			r.Put("/", h.UpdateAgent)
			r.Post("/archive", h.ArchiveAgent)
			r.Post("/restore", h.RestoreAgent)
			r.Get("/tasks", h.ListAgentTasks)
			r.Get("/skills", h.ListAgentSkills)
			r.Put("/skills", h.SetAgentSkills)
			r.Get("/prompt-preview", h.PreviewAgentPrompt)
		})
	})

	// Agent collaboration (messaging, memory)
	r.Route("/api/agents/{id}/messages", func(r chi.Router) {
		r.Post("/", h.SendAgentMessage)
		r.Get("/", h.ListAgentMessages)
		r.Post("/read", h.MarkAgentMessagesRead)
	})
	r.Route("/api/agents/{id}/memory", func(r chi.Router) {
		r.Post("/", h.StoreAgentMemory)
		r.Post("/recall", h.RecallAgentMemory)
		r.Get("/", h.ListAgentMemory)
		r.Delete("/{memoryId}", h.DeleteAgentMemory)
	})

	// Task dependencies and checkpoints
	r.Get("/api/tasks/{taskId}", h.GetTask)
	r.Get("/api/tasks/{taskId}/context-preview", h.PreviewTaskContext)
	r.Route("/api/tasks/{taskId}/dependencies", func(r chi.Router) {
		r.Post("/", h.AddTaskDependency)
		r.Delete("/", h.RemoveTaskDependency)
		r.Get("/", h.ListTaskDependencies)
	})
	r.Route("/api/tasks/{taskId}/checkpoints", func(r chi.Router) {
		r.Post("/", h.SaveTaskCheckpoint)
		r.Get("/", h.ListTaskCheckpoints)
		r.Get("/latest", h.GetLatestCheckpoint)
	})

	// Ready tasks (all dependencies satisfied)
	r.Get("/api/tasks/ready", h.GetReadyTasks)

	// Workspace memory recall
	r.Post("/api/workspace/memory/recall", h.RecallWorkspaceMemory)

	// Task report and timeline
	r.Get("/api/tasks/{taskId}/report", h.GetTaskReport)
	r.Get("/api/tasks/{taskId}/timeline", h.GetTaskTimeline)
	r.Get("/api/tasks/{taskId}/artifacts", h.GetTaskArtifacts)

	// Task review (manual)
	r.Post("/api/tasks/{taskId}/review", h.SubmitReview)

	// Task chaining
	r.Post("/api/tasks/{taskId}/chain", h.ChainTask)
}
