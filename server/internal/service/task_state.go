package service

// TaskState represents the state of a task in its lifecycle.
type TaskState string

const (
	TaskStateQueued    TaskState = "queued"
	TaskStateDispatched TaskState = "dispatched"
	TaskStateRunning   TaskState = "running"
	TaskStateInReview  TaskState = "in_review"
	TaskStateCompleted TaskState = "completed"
	TaskStateFailed    TaskState = "failed"
	TaskStateCancelled TaskState = "cancelled"
)

// allowedTransitions defines the valid state transitions.
// Key = current state, Value = set of allowed next states.
var allowedTransitions = map[TaskState][]TaskState{
	TaskStateQueued:     {TaskStateDispatched, TaskStateFailed, TaskStateCancelled},
	TaskStateDispatched:  {TaskStateRunning, TaskStateQueued, TaskStateFailed, TaskStateCancelled},
	TaskStateRunning:     {TaskStateInReview, TaskStateCompleted, TaskStateFailed, TaskStateCancelled},
	TaskStateInReview:    {TaskStateCompleted, TaskStateFailed, TaskStateQueued, TaskStateCancelled},
	TaskStateCompleted:   {},
	TaskStateFailed:      {TaskStateQueued},
	TaskStateCancelled:   {},
}

// CanTransition returns true if a transition from 'from' to 'to' is allowed.
func CanTransition(from, to TaskState) bool {
	allowed, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}
