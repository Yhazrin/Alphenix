package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/multica-ai/multicode/server/internal/auth"
	"github.com/multica-ai/multicode/server/internal/events"
	"github.com/multica-ai/multicode/server/internal/handler"
	"github.com/multica-ai/multicode/server/internal/middleware"
	"github.com/multica-ai/multicode/server/internal/realtime"
	"github.com/multica-ai/multicode/server/internal/service"
	"github.com/multica-ai/multicode/server/internal/storage"
	db "github.com/multica-ai/multicode/server/pkg/db/generated"
)

func allowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("FRONTEND_ORIGIN"))
	}
	if raw == "" {
		return []string{"http://localhost:3000"}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return []string{"http://localhost:3000"}
	}
	return origins
}

// NewRouter creates the fully-configured Chi router with all middleware and routes.
func NewRouter(pool *pgxpool.Pool, hub *realtime.Hub, bus *events.Bus) chi.Router {
	queries := db.New(pool)
	emailSvc := service.NewEmailService()
	s3 := storage.NewS3StorageFromEnv()
	cfSigner := auth.NewCloudFrontSignerFromEnv()
	h := handler.New(queries, pool, hub, bus, emailSvc, s3, cfSigner)

	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(middleware.RequestLogger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Workspace-ID", "X-Request-ID", "X-Agent-ID", "X-Task-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// WebSocket
	mc := &membershipChecker{queries: queries}
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		realtime.HandleWebSocket(hub, mc, w, r)
	})

	// Auth (public)
	registerAuthRoutes(r, h, queries)

	// Daemon API routes
	// Public route - no auth required (uses join token instead)
	r.Post("/api/daemon/register-with-join-token", h.RegisterRuntimeWithJoinToken)
	registerDaemonRoutes(r, h, queries)

	// Protected API routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(queries))
		r.Use(middleware.RefreshCloudFrontCookies(cfSigner))

		// User-scoped routes (no workspace context required)
		r.Get("/api/me", h.GetMe)
		r.Patch("/api/me", h.UpdateMe)

		// Workspace management
		registerWorkspaceRoutes(r, h, queries)
		// Personal access tokens
		r.Route("/api/tokens", func(r chi.Router) {
			r.Get("/", h.ListPersonalAccessTokens)
			r.Post("/", h.CreatePersonalAccessToken)
			r.Delete("/{id}", h.RevokePersonalAccessToken)
		})

		// Workspace-scoped routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireWorkspaceMember(queries))

			r.Post("/api/upload-file", h.UploadFile)

			registerIssueRoutes(r, h, queries)
			registerAttachmentRoutes(r, h, queries)
			registerCommentRoutes(r, h, queries)
			registerAgentRoutes(r, h, queries)
			registerSkillRoutes(r, h, queries)
			registerRuntimeRoutes(r, h, queries)
			registerInboxRoutes(r, h, queries)
			registerRunRoutes(r, h, queries)
			registerTeamRoutes(r, h, queries)
			registerRuntimePolicyRoutes(r, h, queries)
		})
	})

	return r
}

// membershipChecker implements realtime.MembershipChecker using database queries.
type membershipChecker struct {
	queries *db.Queries
}

func (mc *membershipChecker) IsMember(ctx context.Context, userID, workspaceID string) bool {
	_, err := mc.queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID:      parseUUID(userID),
		WorkspaceID: parseUUID(workspaceID),
	})
	return err == nil
}

func parseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}
	}
	return u
}
