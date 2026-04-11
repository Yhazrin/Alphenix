package service

import "testing"

// --- CanTransition tests ---

func TestCanTransition_AllAllowedTransitions(t *testing.T) {
	tests := []struct {
		from, to TaskState
		want     bool
	}{
		// Queued transitions
		{TaskStateQueued, TaskStateDispatched, true},
		{TaskStateQueued, TaskStateFailed, true},
		{TaskStateQueued, TaskStateCancelled, true},
		{TaskStateQueued, TaskStateRunning, false},

		// Dispatched transitions
		{TaskStateDispatched, TaskStateRunning, true},
		{TaskStateDispatched, TaskStateQueued, true},
		{TaskStateDispatched, TaskStateFailed, true},
		{TaskStateDispatched, TaskStateCancelled, true},
		{TaskStateDispatched, TaskStateCompleted, false},

		// Running transitions
		{TaskStateRunning, TaskStateInReview, true},
		{TaskStateRunning, TaskStateCompleted, true},
		{TaskStateRunning, TaskStateFailed, true},
		{TaskStateRunning, TaskStateCancelled, true},
		{TaskStateRunning, TaskStateQueued, false},

		// InReview transitions
		{TaskStateInReview, TaskStateCompleted, true},
		{TaskStateInReview, TaskStateFailed, true},
		{TaskStateInReview, TaskStateQueued, true},
		{TaskStateInReview, TaskStateCancelled, true},
		{TaskStateInReview, TaskStateRunning, false},

		// Completed — terminal
		{TaskStateCompleted, TaskStateQueued, false},
		{TaskStateCompleted, TaskStateFailed, false},

		// Failed — can retry
		{TaskStateFailed, TaskStateQueued, true},
		{TaskStateFailed, TaskStateRunning, false},

		// Cancelled — terminal
		{TaskStateCancelled, TaskStateQueued, false},
		{TaskStateCancelled, TaskStateRunning, false},
	}

	for _, tt := range tests {
		got := CanTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCanTransition_UnknownState(t *testing.T) {
	if CanTransition("bogus", TaskStateQueued) {
		t.Error("unknown from-state should return false")
	}
	if CanTransition(TaskStateQueued, "bogus") {
		t.Error("unknown to-state should return false")
	}
}

func TestCanTransition_SelfTransition(t *testing.T) {
	// No state allows self-transition
	allStates := []TaskState{
		TaskStateQueued, TaskStateDispatched, TaskStateRunning,
		TaskStateInReview, TaskStateCompleted, TaskStateFailed, TaskStateCancelled,
	}
	for _, s := range allStates {
		if CanTransition(s, s) {
			t.Errorf("self-transition %q -> %q should not be allowed", s, s)
		}
	}
}
