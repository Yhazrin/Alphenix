package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/multica-ai/alphenix/server/internal/util"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// P7: RuntimeSelector integration tests
// ---------------------------------------------------------------------------

func TestSelectRuntime_FallbackTierWins(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	fallbackRT := makeUUID(30)
	normalRT := makeUUID(31)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: normalRT,
	}
	stub.runtimes[util.UUIDToString(fallbackRT)] = db.AgentRuntime{
		ID: fallbackRT, WorkspaceID: wsID, Status: "online",
	}
	stub.runtimes[util.UUIDToString(normalRT)] = db.AgentRuntime{
		ID: normalRT, WorkspaceID: wsID, Status: "online",
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:            agentID,
		IsActive:           true,
		FallbackRuntimeIds: []byte(fmt.Sprintf(`["%s"]`, util.UUIDToString(fallbackRT))),
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != fallbackRT {
		t.Errorf("expected fallback runtime to win over normal")
	}
}

func TestSelectRuntime_TagFilterEliminatesCandidate(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	rtGood := makeUUID(30)
	rtBad := makeUUID(31)
	agentDefault := makeUUID(32)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: agentDefault,
	}
	stub.runtimes[util.UUIDToString(rtGood)] = db.AgentRuntime{
		ID: rtGood, WorkspaceID: wsID, Status: "online",
		Tags: []byte(`["gpu","linux"]`),
	}
	stub.runtimes[util.UUIDToString(rtBad)] = db.AgentRuntime{
		ID: rtBad, WorkspaceID: wsID, Status: "online",
		Tags: []byte(`["cpu","linux"]`),
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:      agentID,
		IsActive:     true,
		RequiredTags: []byte(`["gpu"]`),
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != rtGood {
		t.Errorf("expected gpu runtime, got %v", got)
	}
}

func TestSelectRuntime_AllCandidatesFilteredByTags_Fallback(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	rtNoGpu := makeUUID(30)
	agentDefault := makeUUID(32)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: agentDefault,
	}
	stub.runtimes[util.UUIDToString(rtNoGpu)] = db.AgentRuntime{
		ID: rtNoGpu, WorkspaceID: wsID, Status: "online",
		Tags: []byte(`["cpu"]`),
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:      agentID,
		IsActive:     true,
		RequiredTags: []byte(`["tpu"]`),
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != agentDefault {
		t.Errorf("expected agent default fallback when all candidates filtered, got %v", got)
	}
}

func TestSelectRuntime_ForbiddenTagFiltersCandidate(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	rtAllowed := makeUUID(30)
	rtForbidden := makeUUID(31)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: rtForbidden,
	}
	stub.runtimes[util.UUIDToString(rtAllowed)] = db.AgentRuntime{
		ID: rtAllowed, WorkspaceID: wsID, Status: "online",
		Tags: []byte(`["linux"]`),
	}
	stub.runtimes[util.UUIDToString(rtForbidden)] = db.AgentRuntime{
		ID: rtForbidden, WorkspaceID: wsID, Status: "online",
		Tags: []byte(`["windows"]`),
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:       agentID,
		IsActive:      true,
		ForbiddenTags: []byte(`["windows"]`),
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != rtAllowed {
		t.Errorf("expected non-forbidden runtime, got %v", got)
	}
}

func TestSelectRuntime_LoadScoreTieBreaker(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	rtSlow := makeUUID(30)
	rtFast := makeUUID(31)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: rtSlow,
	}
	stub.runtimes[util.UUIDToString(rtSlow)] = db.AgentRuntime{
		ID: rtSlow, WorkspaceID: wsID, Status: "online",
		AvgTaskDurationMs: 30000, // score = 30
	}
	stub.runtimes[util.UUIDToString(rtFast)] = db.AgentRuntime{
		ID: rtFast, WorkspaceID: wsID, Status: "online",
		AvgTaskDurationMs: 1000, // score = 1
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:  agentID,
		IsActive: true,
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != rtFast {
		t.Errorf("expected faster runtime (lower load score), got %v", got)
	}
}

func TestSelectRuntime_MaxQueueDepthFilter(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	rtFull := makeUUID(30)
	rtAvailable := makeUUID(31)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: rtFull,
	}
	stub.runtimes[util.UUIDToString(rtFull)] = db.AgentRuntime{
		ID: rtFull, WorkspaceID: wsID, Status: "online",
	}
	stub.runtimes[util.UUIDToString(rtAvailable)] = db.AgentRuntime{
		ID: rtAvailable, WorkspaceID: wsID, Status: "online",
	}
	stub.tasks["t1"] = db.AgentTaskQueue{ID: makeUUID(100), RuntimeID: rtFull, Status: "queued"}
	stub.tasks["t2"] = db.AgentTaskQueue{ID: makeUUID(101), RuntimeID: rtFull, Status: "queued"}
	stub.tasks["t3"] = db.AgentTaskQueue{ID: makeUUID(102), RuntimeID: rtFull, Status: "queued"}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:       agentID,
		IsActive:      true,
		MaxQueueDepth: 2,
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != rtAvailable {
		t.Errorf("expected runtime under queue depth limit, got %v", got)
	}
}

func TestSelectRuntime_MaxQueueDepthAllExceeded_PicksLeastLoaded(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	rt1 := makeUUID(30)
	rt2 := makeUUID(31)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: rt1,
	}
	stub.runtimes[util.UUIDToString(rt1)] = db.AgentRuntime{
		ID: rt1, WorkspaceID: wsID, Status: "online",
		AvgTaskDurationMs: 5000, // load score = 5
	}
	stub.runtimes[util.UUIDToString(rt2)] = db.AgentRuntime{
		ID: rt2, WorkspaceID: wsID, Status: "online",
		AvgTaskDurationMs: 1000, // load score = 1 (lighter)
	}
	for i := byte(0); i < 5; i++ {
		stub.tasks[fmt.Sprintf("rt1-t%d", i)] = db.AgentTaskQueue{ID: makeUUID(200 + i), RuntimeID: rt1, Status: "queued"}
	}
	for i := byte(0); i < 3; i++ {
		stub.tasks[fmt.Sprintf("rt2-t%d", i)] = db.AgentTaskQueue{ID: makeUUID(210 + i), RuntimeID: rt2, Status: "queued"}
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:       agentID,
		IsActive:      true,
		MaxQueueDepth: 1,
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != rt2 {
		t.Errorf("expected rt2 (least loaded when all exceed depth), got %v", got)
	}
}

func TestSelectRuntime_GetAgentError(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	_, err := svc.SelectRuntime(context.Background(), makeUUID(10), makeUUID(99))
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
	if !strings.Contains(err.Error(), "load agent") {
		t.Errorf("expected 'load agent' in error, got %q", err.Error())
	}
}

func TestSelectRuntime_NoPolicyFallback(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	agentID := makeUUID(20)
	defaultRT := makeUUID(30)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: defaultRT,
	}
	// No policy configured — should fall back to agent default

	got, err := svc.SelectRuntime(context.Background(), makeUUID(10), agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != defaultRT {
		t.Errorf("expected fallback to agent default runtime, got %v", got)
	}
}

func TestSelectRuntime_PreferredOverFallback(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	preferredRT := makeUUID(30)
	fallbackRT := makeUUID(31)
	normalRT := makeUUID(32)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: normalRT,
	}
	stub.runtimes[util.UUIDToString(preferredRT)] = db.AgentRuntime{
		ID: preferredRT, WorkspaceID: wsID, Status: "online",
	}
	stub.runtimes[util.UUIDToString(fallbackRT)] = db.AgentRuntime{
		ID: fallbackRT, WorkspaceID: wsID, Status: "online",
	}
	stub.runtimes[util.UUIDToString(normalRT)] = db.AgentRuntime{
		ID: normalRT, WorkspaceID: wsID, Status: "online",
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:             agentID,
		IsActive:            true,
		PreferredRuntimeIds: []byte(fmt.Sprintf(`["%s"]`, util.UUIDToString(preferredRT))),
		FallbackRuntimeIds:  []byte(fmt.Sprintf(`["%s"]`, util.UUIDToString(fallbackRT))),
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != preferredRT {
		t.Errorf("preferred should beat fallback, got %v", got)
	}
}

func TestSelectRuntime_OnlyOfflineRuntimes_Fallback(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	rtOffline := makeUUID(30)
	agentDefault := makeUUID(32)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: agentDefault,
	}
	stub.runtimes[util.UUIDToString(rtOffline)] = db.AgentRuntime{
		ID: rtOffline, WorkspaceID: wsID, Status: "offline",
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:  agentID,
		IsActive: true,
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != agentDefault {
		t.Errorf("expected fallback when all runtimes offline, got %v", got)
	}
}

func TestSelectRuntime_FailureRatePenalty(t *testing.T) {
	stub := newTaskStubDBTX()
	svc := newTestTaskService(stub)

	wsID := makeUUID(10)
	agentID := makeUUID(20)
	rtReliable := makeUUID(30)
	rtUnreliable := makeUUID(31)

	stub.agents[util.UUIDToString(agentID)] = db.Agent{
		ID: agentID, RuntimeID: rtUnreliable,
	}
	stub.runtimes[util.UUIDToString(rtReliable)] = db.AgentRuntime{
		ID: rtReliable, WorkspaceID: wsID, Status: "online",
		SuccessCount24h: 100, FailureCount24h: 0,
	}
	stub.runtimes[util.UUIDToString(rtUnreliable)] = db.AgentRuntime{
		ID: rtUnreliable, WorkspaceID: wsID, Status: "online",
		SuccessCount24h: 50, FailureCount24h: 50,
	}
	stub.runtimePolicies[util.UUIDToString(agentID)] = db.RuntimeAssignmentPolicy{
		AgentID:  agentID,
		IsActive: true,
	}

	got, err := svc.SelectRuntime(context.Background(), wsID, agentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != rtReliable {
		t.Errorf("expected reliable runtime (lower failure penalty), got %v", got)
	}
}
