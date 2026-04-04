package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type SubmitReviewRequest struct {
	ReviewerID string `json:"reviewer_id"`
	Verdict    string `json:"verdict"`
	Feedback   string `json:"feedback"`
}

func (h *Handler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "taskId")
	taskID := parseUUID(taskIDStr)

	var req SubmitReviewRequest
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5MB
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	reviewerID := parseUUID(req.ReviewerID)

	_, err := h.ReviewService.SubmitManualReview(r.Context(), taskID, reviewerID, req.Verdict, req.Feedback)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
