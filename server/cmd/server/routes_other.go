package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/alphenix/server/internal/handler"
	"github.com/multica-ai/alphenix/server/internal/middleware"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// registerWorkspaceRoutes registers workspace management routes.
func registerWorkspaceRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Route("/api/workspaces", func(r chi.Router) {
		r.Get("/", h.ListWorkspaces)
		r.Post("/", h.CreateWorkspace)
		r.Route("/{id}", func(r chi.Router) {
			// Member-level access
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceMemberFromURL(queries, "id"))
				r.Get("/", h.GetWorkspace)
				r.Get("/members", h.ListMembersWithUser)
				r.Post("/leave", h.LeaveWorkspace)
			})
			// Admin-level access
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireWorkspaceRoleFromURL(queries, "id", "owner", "admin"))
				r.Put("/", h.UpdateWorkspace)
				r.Patch("/", h.UpdateWorkspace)
				r.Post("/members", h.CreateMember)
				r.Route("/members/{memberId}", func(r chi.Router) {
					r.Patch("/", h.UpdateMember)
					r.Delete("/", h.DeleteMember)
				})
				r.Post("/runtime-join-tokens", h.CreateRuntimeJoinToken)
				r.Get("/runtime-join-tokens", h.ListRuntimeJoinTokens)
			})
			// Owner-only access
			r.With(middleware.RequireWorkspaceRoleFromURL(queries, "id", "owner")).Delete("/", h.DeleteWorkspace)
			// Workspace repo routes
			r.Route("/repos", func(r chi.Router) {
				r.Get("/", h.ListWorkspaceRepos)
				r.Post("/", h.CreateWorkspaceRepo)
				r.Route("/{repoID}", func(r chi.Router) {
					r.Get("/", h.GetWorkspaceRepo)
					r.Patch("/", h.UpdateWorkspaceRepo)
					r.Delete("/", h.DeleteWorkspaceRepo)
					r.Get("/issues", h.ListIssuesByRepoID)
				})
			})
		})
	})
}

// registerSkillRoutes registers skill management routes.
func registerSkillRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Route("/api/skills", func(r chi.Router) {
		r.Get("/", h.ListSkills)
		r.With(middleware.RequireWorkspaceRole(queries, "owner", "admin")).Post("/", h.CreateSkill)
		r.With(middleware.RequireWorkspaceRole(queries, "owner", "admin")).Post("/import", h.ImportSkill)
		r.Get("/marketplace/search", h.SearchMarketplace)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetSkill)
			r.Put("/", h.UpdateSkill)
			r.Delete("/", h.DeleteSkill)
			r.Get("/files", h.ListSkillFiles)
			r.Put("/files", h.UpsertSkillFile)
			r.Delete("/files/{fileId}", h.DeleteSkillFile)
			r.Get("/agents", h.ListSkillAgents)
		})
	})
}

// registerInboxRoutes registers inbox notification routes.
func registerInboxRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Route("/api/inbox", func(r chi.Router) {
		r.Get("/", h.ListInbox)
		r.Get("/unread-count", h.CountUnreadInbox)
		r.Post("/mark-all-read", h.MarkAllInboxRead)
		r.Post("/archive-all", h.ArchiveAllInbox)
		r.Post("/archive-all-read", h.ArchiveAllReadInbox)
		r.Post("/archive-completed", h.ArchiveCompletedInbox)
		r.Post("/{id}/read", h.MarkInboxRead)
		r.Post("/{id}/archive", h.ArchiveInboxItem)
	})
}

// registerRuntimeRoutes registers runtime management routes.
func registerRuntimeRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Route("/api/runtimes", func(r chi.Router) {
		r.Get("/", h.ListAgentRuntimes)
		r.Get("/{runtimeId}/usage", h.GetRuntimeUsage)
		r.Get("/{runtimeId}/activity", h.GetRuntimeTaskActivity)
		r.Get("/{runtimeId}/audit-logs", h.GetRuntimeAuditLogs)
		r.Post("/{runtimeId}/ping", h.InitiatePing)
		r.Get("/{runtimeId}/ping/{pingId}", h.GetPing)
		r.Post("/{runtimeId}/update", h.InitiateUpdate)
		r.Get("/{runtimeId}/update/{updateId}", h.GetUpdate)
		r.Post("/{runtimeId}/approve", h.ApproveRuntime)
		r.Post("/{runtimeId}/reject", h.RejectRuntime)
		r.Post("/{runtimeId}/pause", h.PauseRuntime)
		r.Post("/{runtimeId}/resume", h.ResumeRuntime)
		r.Post("/{runtimeId}/revoke", h.RevokeRuntime)
		r.Post("/{runtimeId}/drain", h.DrainRuntime)
	})
}

// registerAttachmentRoutes registers attachment routes.
func registerAttachmentRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Get("/api/attachments/{id}", h.GetAttachmentByID)
	r.Delete("/api/attachments/{id}", h.DeleteAttachment)
}

// registerCommentRoutes registers comment CRUD routes.
func registerCommentRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Route("/api/comments/{commentId}", func(r chi.Router) {
		r.Put("/", h.UpdateComment)
		r.Delete("/", h.DeleteComment)
		r.Post("/reactions", h.AddReaction)
		r.Delete("/reactions", h.RemoveReaction)
	})
}

// registerRuntimePolicyRoutes registers runtime assignment policy routes.
func registerRuntimePolicyRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Route("/api/runtime-policies", func(r chi.Router) {
		r.Get("/", h.ListRuntimePolicies)
		r.Post("/", h.CreateRuntimePolicy)
		r.Get("/by-agent/{agentId}", h.GetRuntimePolicyByAgent)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.GetRuntimePolicy)
			r.Patch("/", h.UpdateRuntimePolicy)
			r.Delete("/", h.DeleteRuntimePolicy)
		})
	})
}

// registerRunRoutes registers run lifecycle routes.
func registerRunRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Route("/api/runs", func(r chi.Router) {
		r.Use(middleware.RequireWorkspaceMember(queries))
		r.Get("/", h.ListRuns)
		r.Post("/", h.CreateRun)
		r.Route("/{runId}", func(r chi.Router) {
			r.Get("/", h.GetRun)
			r.Post("/start", h.StartRun)
			r.Post("/cancel", h.CancelRun)
			r.Post("/complete", h.CompleteRun)
			r.Post("/execute", h.ExecuteRun)
			r.Post("/retry", h.RetryRun)
			r.Get("/steps", h.GetRunSteps)
			r.Post("/steps", h.RecordStep)
			r.Get("/todos", h.GetRunTodos)
			r.Post("/todos", h.CreateRunTodo)
			r.Patch("/todos/{todoId}", h.UpdateRunTodo)
			r.Get("/artifacts", h.GetRunArtifacts)
			r.Get("/events", h.ListRunEvents)
		})
		r.Get("/by-issue/{issueId}", h.ListRunsByIssue)
	})
}
