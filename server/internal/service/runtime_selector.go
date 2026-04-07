package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/alphenix/server/internal/util"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// RuntimeCandidate holds a runtime with its computed scheduling score.
type RuntimeCandidate struct {
	Runtime db.AgentRuntime
	Score   int // lower = higher priority
}

// SelectRuntime picks the best runtime for an agent based on the agent's
// RuntimeAssignmentPolicy. Priority chain:
//
//	1. Online (status='online', not paused, not draining, approved)
//		2. Tag match (required_tags ⊆ runtime.tags, forbidden_tags ∩ runtime.tags = ∅)
//		3. Preferred list first, then fallback list, then all others
//		4. Load score (pending task count + avg duration — lower is better)
//
// Returns the selected runtime ID and true, or zero value and false if no
// suitable runtime exists. Falls back to agent.RuntimeID when no policy is configured.
func (s *TaskService) SelectRuntime(ctx context.Context, workspaceID, agentID pgtype.UUID) (pgtype.UUID, error) {
	agent, err := s.Queries.GetAgent(ctx, agentID)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("load agent: %w", err)
	}

	// Try loading the agent's policy.
	policy, err := s.Queries.GetRuntimePolicyByAgent(ctx, db.GetRuntimePolicyByAgentParams{
		AgentID:     agentID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			// No policy — fall back to agent's default runtime.
			slog.Debug("runtime selection: no active policy, using agent default",
				"agent_id", util.UUIDToString(agentID))
			return agent.RuntimeID, nil
		}
		return pgtype.UUID{}, fmt.Errorf("load policy: %w", err)
	}
	if !policy.IsActive {
		// Inactive policy — fall back to agent's default runtime.
		slog.Debug("runtime selection: inactive policy, using agent default",
			"agent_id", util.UUIDToString(agentID))
		return agent.RuntimeID, nil
	}

	// Load all runtimes in the workspace.
	runtimes, err := s.Queries.ListAgentRuntimes(ctx, workspaceID)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("list runtimes: %w", err)
	}

	// Parse policy tag constraints.
	requiredTags := parseJSONStringSlice(policy.RequiredTags)
	forbiddenTags := parseJSONStringSlice(policy.ForbiddenTags)
	preferredIDs := parseJSONStringSlice(policy.PreferredRuntimeIds)
	fallbackIDs := parseJSONStringSlice(policy.FallbackRuntimeIds)

	// Filter to eligible runtimes.
	var candidates []RuntimeCandidate
	for _, rt := range runtimes {
		if !isRuntimeEligible(rt) {
			continue
		}
		if !tagsMatch(rt, requiredTags, forbiddenTags) {
			continue
		}

		// Compute priority tier: preferred=0, fallback=1, normal=2.
		tier := 2
		rtID := util.UUIDToString(rt.ID)
		for _, pid := range preferredIDs {
			if pid == rtID {
				tier = 0
				break
			}
		}
		if tier == 2 {
			for _, fid := range fallbackIDs {
				if fid == rtID {
					tier = 1
					break
				}
			}
		}

		// Load score = pending tasks * 1000 + avg_duration_ms / 1000.
		// Lower is better. Multiply tier by 1_000_000 to ensure preferred > load.
		loadScore := computeLoadScore(rt)
		candidates = append(candidates, RuntimeCandidate{
			Runtime: rt,
			Score:   tier*1_000_000 + loadScore,
		})
	}

	if len(candidates) == 0 {
		slog.Warn("runtime selection: no eligible runtime found, falling back to agent default",
			"agent_id", util.UUIDToString(agentID),
			"workspace_id", util.UUIDToString(workspaceID))
		return agent.RuntimeID, nil
	}

	// Check max_queue_depth if set.
	if policy.MaxQueueDepth > 0 {
		var filtered []RuntimeCandidate
		for _, c := range candidates {
			pending := countPendingTasks(ctx, s.Queries, c.Runtime.ID)
			if int64(pending) <= int64(policy.MaxQueueDepth) {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) > 0 {
			candidates = filtered
		}
		// If all exceeded depth, fall through and pick the least loaded anyway.
	}

	// Sort by score (lower = better).
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score < candidates[j].Score
	})

	selected := candidates[0].Runtime
	slog.Info("runtime selected",
		"agent_id", util.UUIDToString(agentID),
		"runtime_id", util.UUIDToString(selected.ID),
		"runtime_name", selected.Name,
		"score", candidates[0].Score)

	return selected.ID, nil
}

// isRuntimeEligible checks if a runtime can accept new tasks.
func isRuntimeEligible(rt db.AgentRuntime) bool {
	if rt.Status != "online" {
		return false
	}
	if rt.Paused {
		return false
	}
	if rt.DrainMode {
		return false
	}
	if rt.ApprovalStatus != "" && rt.ApprovalStatus != "approved" {
		return false
	}
	return true
}

// tagsMatch checks required ⊆ runtimeTags and forbidden ∩ runtimeTags = ∅.
func tagsMatch(rt db.AgentRuntime, required, forbidden []string) bool {
	if len(required) == 0 && len(forbidden) == 0 {
		return true
	}

	var rtTags []string
	if rt.Tags != nil {
		if err := json.Unmarshal(rt.Tags, &rtTags); err != nil {
			slog.Warn("tagsMatch: failed to unmarshal runtime tags", "runtime_id", util.UUIDToString(rt.ID), "error", err)
		}
	}

	tagSet := make(map[string]bool, len(rtTags))
	for _, t := range rtTags {
		tagSet[t] = true
	}

	for _, req := range required {
		if !tagSet[req] {
			return false
		}
	}
	for _, forb := range forbidden {
		if tagSet[forb] {
			return false
		}
	}
	return true
}

// computeLoadScore produces a numeric score: lower = less loaded.
// Factors: success rate (24h), average task duration.
func computeLoadScore(rt db.AgentRuntime) int {
	// Use avg_task_duration_ms / 1000 as base load metric.
	// Runtimes with no history get score 0 (treated as fresh/fast).
	score := int(rt.AvgTaskDurationMs / 1000)

	// Penalize runtimes with high failure rates.
	total := rt.SuccessCount24h + rt.FailureCount24h
	if total > 0 {
		failRate := int(rt.FailureCount24h * 100 / total)
		score += failRate * 10 // 0-1000 penalty
	}

	return score
}

// countPendingTasks returns the number of queued+dispatched tasks for a runtime.
func countPendingTasks(ctx context.Context, q *db.Queries, runtimeID pgtype.UUID) int {
	count, err := q.CountPendingTasksByRuntime(ctx, runtimeID)
	if err != nil {
		return 0
	}
	return int(count)
}

// parseJSONStringSlice parses a JSON array of strings from []byte.
func parseJSONStringSlice(data []byte) []string {
	if data == nil {
		return nil
	}
	var result []string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}
