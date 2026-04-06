package daemon

import (
	"context"
	"log/slog"

	"github.com/multica-ai/alphenix/server/internal/events"
	"github.com/multica-ai/alphenix/server/pkg/agent"
	"github.com/multica-ai/alphenix/server/pkg/protocol"
)

// HookService bridges agent-level tool hooks with the event bus, providing
// server-side observability for pre/post tool use across all agents.
//
// Inspired by Claude Code's hook system: PreToolUse can deny calls,
// PostToolUse observes results. Here we additionally publish lifecycle
// events on the bus so other services (collaboration, realtime) can react.
type HookService struct {
	bus    *events.Bus
	logger *slog.Logger
}

// NewHookService creates a HookService wired to the given event bus.
func NewHookService(bus *events.Bus, logger *slog.Logger) *HookService {
	return &HookService{bus: bus, logger: logger}
}

// BuildToolHooks returns agent.ToolHooks that publish events on the bus
// and enforce optional permission checks. The workspaceID and taskID are
// embedded in the returned hooks via closure.
func (hs *HookService) BuildToolHooks(workspaceID, taskID, agentID string) agent.ToolHooks {
	return agent.ToolHooks{
		PreToolUse: func(ctx context.Context, tool string, input map[string]any) agent.ToolHookResult {
			hs.bus.Publish(events.Event{
				Type:        protocol.EventAgentToolUse,
				WorkspaceID: workspaceID,
				ActorType:   "agent",
				ActorID:     agentID,
				Payload: map[string]any{
					"task_id": taskID,
					"tool":    tool,
					"input":   input,
				},
			})
			return agent.ToolHookResult{} // allow by default
		},
		PostToolUse: func(ctx context.Context, tool string, input map[string]any, output string) {
			hs.bus.Publish(events.Event{
				Type:        protocol.EventAgentToolResult,
				WorkspaceID: workspaceID,
				ActorType:   "agent",
				ActorID:     agentID,
				Payload: map[string]any{
					"task_id": taskID,
					"tool":    tool,
					"output":  output,
				},
			})
		},
	}
}

// PublishAgentStarted publishes an agent:started event on the bus.
func (hs *HookService) PublishAgentStarted(workspaceID, taskID, agentID, provider string) {
	hs.bus.Publish(events.Event{
		Type:        protocol.EventAgentStarted,
		WorkspaceID: workspaceID,
		ActorType:   "agent",
		ActorID:     agentID,
		Payload: map[string]any{
			"task_id":  taskID,
			"provider": provider,
		},
	})
}

// PublishAgentCompleted publishes an agent:completed event on the bus.
func (hs *HookService) PublishAgentCompleted(workspaceID, taskID, agentID string, durationMs int64) {
	hs.bus.Publish(events.Event{
		Type:        protocol.EventAgentCompleted,
		WorkspaceID: workspaceID,
		ActorType:   "agent",
		ActorID:     agentID,
		Payload: map[string]any{
			"task_id":     taskID,
			"duration_ms": durationMs,
		},
	})
}

// PublishAgentFailed publishes an agent:failed event on the bus.
func (hs *HookService) PublishAgentFailed(workspaceID, taskID, agentID, errMsg string) {
	hs.bus.Publish(events.Event{
		Type:        protocol.EventAgentFailed,
		WorkspaceID: workspaceID,
		ActorType:   "agent",
		ActorID:     agentID,
		Payload: map[string]any{
			"task_id": taskID,
			"error":   errMsg,
		},
	})
}

// PublishAgentSessionStart publishes an agent:session_start event on the bus.
func (hs *HookService) PublishAgentSessionStart(workspaceID, taskID, agentID, sessionID string) {
	hs.bus.Publish(events.Event{
		Type:        protocol.EventAgentSessionStart,
		WorkspaceID: workspaceID,
		ActorType:   "agent",
		ActorID:     agentID,
		Payload: map[string]any{
			"task_id":    taskID,
			"session_id": sessionID,
		},
	})
}

// PublishForkStarted publishes an agent:fork_started event on the bus.
func (hs *HookService) PublishForkStarted(workspaceID, taskID, agentID, forkID string) {
	hs.bus.Publish(events.Event{
		Type:        protocol.EventForkStarted,
		WorkspaceID: workspaceID,
		ActorType:   "agent",
		ActorID:     agentID,
		Payload: map[string]any{
			"task_id": taskID,
			"fork_id": forkID,
		},
	})
}

// PublishForkCompleted publishes an agent:fork_completed event on the bus.
func (hs *HookService) PublishForkCompleted(workspaceID, taskID, agentID, forkID string, durationMs int64) {
	hs.bus.Publish(events.Event{
		Type:        protocol.EventForkCompleted,
		WorkspaceID: workspaceID,
		ActorType:   "agent",
		ActorID:     agentID,
		Payload: map[string]any{
			"task_id":     taskID,
			"fork_id":     forkID,
			"duration_ms": durationMs,
		},
	})
}

// PublishForkFailed publishes an agent:fork_failed event on the bus.
func (hs *HookService) PublishForkFailed(workspaceID, taskID, agentID, forkID, errMsg string) {
	hs.bus.Publish(events.Event{
		Type:        protocol.EventForkFailed,
		WorkspaceID: workspaceID,
		ActorType:   "agent",
		ActorID:     agentID,
		Payload: map[string]any{
			"task_id": taskID,
			"fork_id": forkID,
			"error":   errMsg,
		},
	})
}

// PublishAgentStop publishes an agent:stop event on the bus.
func (hs *HookService) PublishAgentStop(workspaceID, taskID, agentID string, result agent.Result) {
	hs.bus.Publish(events.Event{
		Type:        protocol.EventAgentStop,
		WorkspaceID: workspaceID,
		ActorType:   "agent",
		ActorID:     agentID,
		Payload: map[string]any{
			"task_id":     taskID,
			"status":      result.Status,
			"duration_ms": result.DurationMs,
			"session_id":  result.SessionID,
		},
	})
}
