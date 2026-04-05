package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multicode/server/internal/util"
	db "github.com/multica-ai/multicode/server/pkg/db/generated"
)

// MatchedAgent pairs an agent ID with the policy that matched it.
type MatchedAgent struct {
	AgentID pgtype.UUID
	Policy  db.RuntimeAssignmentPolicy
}

// StartupSequenceResult is returned by RunStartupSequence.
type StartupSequenceResult struct {
	MatchedAgents []MatchedAgent
}

// RunStartupSequence loads active RuntimePolicies, matches them against the
// runtime's tags, and returns the list of agents that should be pre-warmed.
func (s *TaskService) RunStartupSequence(
	ctx context.Context,
	workspaceID, runtimeID pgtype.UUID,
	runtimeTags []string,
) (StartupSequenceResult, error) {
	slog.Info("runtime startup sequence",
		"runtime_id", util.UUIDToString(runtimeID),
		"workspace_id", util.UUIDToString(workspaceID),
		"tags", runtimeTags)

	policies, err := s.Queries.ListActiveRuntimePolicies(ctx, workspaceID)
	if err != nil {
		return StartupSequenceResult{}, fmt.Errorf("list active policies: %w", err)
	}

	tagSet := buildTagSet(runtimeTags)

	var matched []MatchedAgent
	for _, policy := range policies {
		requiredTags := parseJSONStringSlice(policy.RequiredTags)
		forbiddenTags := parseJSONStringSlice(policy.ForbiddenTags)

		if !runtimeTagsSatisfyPolicy(tagSet, requiredTags, forbiddenTags) {
			continue
		}

		matched = append(matched, MatchedAgent{
			AgentID: policy.AgentID,
			Policy:  policy,
		})
	}

	slog.Info("startup sequence complete",
		"runtime_id", util.UUIDToString(runtimeID),
		"policies_evaluated", len(policies),
		"agents_matched", len(matched))

	return StartupSequenceResult{MatchedAgents: matched}, nil
}

// runtimeTagsSatisfyPolicy checks that all required tags are present and no
// forbidden tags are present in the runtime's tag set.
func runtimeTagsSatisfyPolicy(runtimeTagSet map[string]bool, required, forbidden []string) bool {
	for _, req := range required {
		if !runtimeTagSet[req] {
			return false
		}
	}
	for _, forb := range forbidden {
		if runtimeTagSet[forb] {
			return false
		}
	}
	return true
}

// buildTagSet converts a slice of tags into a set for O(1) lookups.
func buildTagSet(tags []string) map[string]bool {
	set := make(map[string]bool, len(tags))
	for _, t := range tags {
		set[t] = true
	}
	return set
}
