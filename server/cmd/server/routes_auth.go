package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/alphenix/server/internal/handler"
	"github.com/multica-ai/alphenix/server/internal/middleware"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// registerAuthRoutes registers auth-related public routes.
func registerAuthRoutes(r chi.Router, h *handler.Handler, queries *db.Queries) {
	r.Post("/auth/send-code", h.SendCode)
	r.Post("/auth/verify-code", h.VerifyCode)
	r.Post("/auth/logout", h.Logout)
	r.With(middleware.Auth(queries)).Post("/auth/ws-ticket", h.WsTicket)
}
