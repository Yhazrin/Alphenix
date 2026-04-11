package handler

import (
	"testing"
	"time"
)

func TestUpdateStore_Create(t *testing.T) {
	store := NewUpdateStore()
	req, err := store.Create("rt-1", "v2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if req.RuntimeID != "rt-1" {
		t.Fatalf("expected runtime_id 'rt-1', got %q", req.RuntimeID)
	}
	if req.TargetVersion != "v2.0.0" {
		t.Fatalf("expected target_version 'v2.0.0', got %q", req.TargetVersion)
	}
	if req.Status != UpdatePending {
		t.Fatalf("expected status 'pending', got %q", req.Status)
	}
}

func TestUpdateStore_CreateDuplicate(t *testing.T) {
	store := NewUpdateStore()
	_, err := store.Create("rt-1", "v2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second create for same runtime should fail.
	_, err = store.Create("rt-1", "v2.1.0")
	if err == nil {
		t.Fatal("expected error for duplicate update")
	}
	if err != errUpdateInProgress {
		t.Fatalf("expected errUpdateInProgress, got %v", err)
	}
}

func TestUpdateStore_CreateDuplicateAfterComplete(t *testing.T) {
	store := NewUpdateStore()
	req, _ := store.Create("rt-1", "v2.0.0")
	store.Complete(req.ID, "done")

	// After completion, new create should succeed.
	req2, err := store.Create("rt-1", "v2.1.0")
	if err != nil {
		t.Fatalf("unexpected error after completing previous update: %v", err)
	}
	if req2.TargetVersion != "v2.1.0" {
		t.Fatalf("expected target_version 'v2.1.0', got %q", req2.TargetVersion)
	}
}

func TestUpdateStore_CreateDuplicateAfterFail(t *testing.T) {
	store := NewUpdateStore()
	req, _ := store.Create("rt-1", "v2.0.0")
	store.Fail(req.ID, "error")

	_, err := store.Create("rt-1", "v2.1.0")
	if err != nil {
		t.Fatalf("unexpected error after failing previous update: %v", err)
	}
}

func TestUpdateStore_CreateDifferentRuntimes(t *testing.T) {
	store := NewUpdateStore()
	_, err := store.Create("rt-1", "v2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = store.Create("rt-2", "v2.0.0")
	if err != nil {
		t.Fatalf("expected different runtime to succeed: %v", err)
	}
}

func TestUpdateStore_Get(t *testing.T) {
	store := NewUpdateStore()
	req, _ := store.Create("rt-1", "v2.0.0")

	got := store.Get(req.ID)
	if got == nil {
		t.Fatal("expected to find update")
	}
	if got.ID != req.ID {
		t.Fatalf("expected ID %q, got %q", req.ID, got.ID)
	}
}

func TestUpdateStore_GetNotFound(t *testing.T) {
	store := NewUpdateStore()
	got := store.Get("nonexistent")
	if got != nil {
		t.Fatal("expected nil for nonexistent update")
	}
}

func TestUpdateStore_GetTimeout(t *testing.T) {
	store := NewUpdateStore()
	req, _ := store.Create("rt-1", "v2.0.0")
	// Artificially age the request.
	req.CreatedAt = time.Now().Add(-121 * time.Second)

	got := store.Get(req.ID)
	if got.Status != UpdateTimeout {
		t.Fatalf("expected status 'timeout', got %q", got.Status)
	}
	if got.Error == "" {
		t.Fatal("expected timeout error message")
	}
}

func TestUpdateStore_GetTimeoutRunning(t *testing.T) {
	store := NewUpdateStore()
	req, _ := store.Create("rt-1", "v2.0.0")
	store.PopPending("rt-1") // moves to running
	// Age it.
	req.CreatedAt = time.Now().Add(-121 * time.Second)

	got := store.Get(req.ID)
	if got.Status != UpdateTimeout {
		t.Fatalf("expected status 'timeout' for running update, got %q", got.Status)
	}
}

func TestUpdateStore_GetNoTimeoutWhenCompleted(t *testing.T) {
	store := NewUpdateStore()
	req, _ := store.Create("rt-1", "v2.0.0")
	store.Complete(req.ID, "done")
	req.CreatedAt = time.Now().Add(-121 * time.Second)

	got := store.Get(req.ID)
	if got.Status != UpdateCompleted {
		t.Fatalf("expected status 'completed', got %q", got.Status)
	}
}

func TestUpdateStore_PopPending(t *testing.T) {
	store := NewUpdateStore()
	store.Create("rt-1", "v2.0.0")

	popped := store.PopPending("rt-1")
	if popped == nil {
		t.Fatal("expected to pop pending update")
	}
	if popped.Status != UpdateRunning {
		t.Fatalf("expected status 'running', got %q", popped.Status)
	}
}

func TestUpdateStore_PopPendingEmpty(t *testing.T) {
	store := NewUpdateStore()
	popped := store.PopPending("rt-1")
	if popped != nil {
		t.Fatal("expected nil when no pending updates")
	}
}

func TestUpdateStore_PopPendingWrongRuntime(t *testing.T) {
	store := NewUpdateStore()
	store.Create("rt-1", "v2.0.0")
	popped := store.PopPending("rt-2")
	if popped != nil {
		t.Fatal("expected nil for wrong runtime ID")
	}
}

func TestUpdateStore_PopPendingDoesNotPopRunning(t *testing.T) {
	store := NewUpdateStore()
	store.Create("rt-1", "v2.0.0")
	store.PopPending("rt-1")

	popped := store.PopPending("rt-1")
	if popped != nil {
		t.Fatal("should not pop running updates")
	}
}

func TestUpdateStore_Complete(t *testing.T) {
	store := NewUpdateStore()
	req, _ := store.Create("rt-1", "v2.0.0")
	store.Complete(req.ID, "update output here")

	got := store.Get(req.ID)
	if got.Status != UpdateCompleted {
		t.Fatalf("expected status 'completed', got %q", got.Status)
	}
	if got.Output != "update output here" {
		t.Fatalf("expected output 'update output here', got %q", got.Output)
	}
}

func TestUpdateStore_CompleteUnknownID(t *testing.T) {
	store := NewUpdateStore()
	// Should not panic.
	store.Complete("nonexistent", "output")
}

func TestUpdateStore_Fail(t *testing.T) {
	store := NewUpdateStore()
	req, _ := store.Create("rt-1", "v2.0.0")
	store.Fail(req.ID, "binary not found")

	got := store.Get(req.ID)
	if got.Status != UpdateFailed {
		t.Fatalf("expected status 'failed', got %q", got.Status)
	}
	if got.Error != "binary not found" {
		t.Fatalf("expected error 'binary not found', got %q", got.Error)
	}
}

func TestUpdateStore_FailUnknownID(t *testing.T) {
	store := NewUpdateStore()
	// Should not panic.
	store.Fail("nonexistent", "error")
}

func TestUpdateStore_CleanupOldRequests(t *testing.T) {
	store := NewUpdateStore()
	old, _ := store.Create("rt-1", "v1.0.0")
	old.CreatedAt = time.Now().Add(-6 * time.Minute)

	// Creating a new update triggers cleanup.
	req, err := store.Create("rt-1", "v2.0.0")
	if err != nil {
		t.Fatalf("expected create to succeed after cleanup: %v", err)
	}

	got := store.Get(old.ID)
	if got != nil {
		t.Fatal("expected old request to be cleaned up")
	}
	if req.TargetVersion != "v2.0.0" {
		t.Fatalf("expected target_version 'v2.0.0', got %q", req.TargetVersion)
	}
}

func TestUpdateStatusConstants(t *testing.T) {
	tests := []struct {
		status UpdateStatus
		want   string
	}{
		{UpdatePending, "pending"},
		{UpdateRunning, "running"},
		{UpdateCompleted, "completed"},
		{UpdateFailed, "failed"},
		{UpdateTimeout, "timeout"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("UpdateStatus %v = %q, want %q", tt.status, string(tt.status), tt.want)
		}
	}
}

func TestUpdateStore_DuplicateRunning(t *testing.T) {
	store := NewUpdateStore()
	store.Create("rt-1", "v2.0.0")
	store.PopPending("rt-1") // running

	// Should reject even when running (not just pending).
	_, err := store.Create("rt-1", "v3.0.0")
	if err == nil {
		t.Fatal("expected error when update is running")
	}
}
