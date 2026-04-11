package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

func TestSubscriberAPI(t *testing.T) {
	ctx := context.Background()

	// Helper: create an issue for subscriber tests
	createIssue := func(t *testing.T) string {
		t.Helper()
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
			"title": "Subscriber test issue",
		})
		testHandler.CreateIssue(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("CreateIssue: expected 201, got %d: %s", w.Code, w.Body.String())
		}
		var issue IssueResponse
		json.NewDecoder(w.Body).Decode(&issue)
		return issue.ID
	}

	// Helper: delete an issue
	deleteIssue := func(t *testing.T, issueID string) {
		t.Helper()
		w := httptest.NewRecorder()
		req := newRequest("DELETE", "/api/issues/"+issueID, nil)
		req = withURLParam(req, "id", issueID)
		testHandler.DeleteIssue(w, req)
	}

	t.Run("Subscribe", func(t *testing.T) {
		issueID := createIssue(t)
		defer deleteIssue(t, issueID)

		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues/"+issueID+"/subscribe", nil)
		req = withURLParam(req, "id", issueID)
		testHandler.SubscribeToIssue(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("SubscribeToIssue: expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]bool
		json.NewDecoder(w.Body).Decode(&resp)
		if !resp["subscribed"] {
			t.Fatal("SubscribeToIssue: expected subscribed=true")
		}

		// Verify in DB
		subscribed, err := testHandler.Queries.IsIssueSubscriber(ctx, db.IsIssueSubscriberParams{
			IssueID:  parseUUID(issueID),
			UserType: "member",
			UserID:   parseUUID(testUserID),
		})
		if err != nil {
			t.Fatalf("IsIssueSubscriber: %v", err)
		}
		if !subscribed {
			t.Fatal("expected user to be subscribed in DB")
		}
	})

	t.Run("SubscribeIdempotent", func(t *testing.T) {
		issueID := createIssue(t)
		defer deleteIssue(t, issueID)

		// Subscribe first time
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues/"+issueID+"/subscribe", nil)
		req = withURLParam(req, "id", issueID)
		testHandler.SubscribeToIssue(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("SubscribeToIssue (1st): expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Subscribe second time — should also succeed
		w = httptest.NewRecorder()
		req = newRequest("POST", "/api/issues/"+issueID+"/subscribe", nil)
		req = withURLParam(req, "id", issueID)
		testHandler.SubscribeToIssue(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("SubscribeToIssue (2nd): expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("ListSubscribers", func(t *testing.T) {
		issueID := createIssue(t)
		defer deleteIssue(t, issueID)

		// Subscribe first
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues/"+issueID+"/subscribe", nil)
		req = withURLParam(req, "id", issueID)
		testHandler.SubscribeToIssue(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("SubscribeToIssue: expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// List
		w = httptest.NewRecorder()
		req = newRequest("GET", "/api/issues/"+issueID+"/subscribers", nil)
		req = withURLParam(req, "id", issueID)
		testHandler.ListIssueSubscribers(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("ListIssueSubscribers: expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var subscribers []SubscriberResponse
		json.NewDecoder(w.Body).Decode(&subscribers)
		if len(subscribers) == 0 {
			t.Fatal("ListIssueSubscribers: expected at least 1 subscriber")
		}
		found := false
		for _, s := range subscribers {
			if s.UserID == testUserID && s.UserType == "member" && s.Reason == "manual" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("ListIssueSubscribers: expected to find test user subscriber, got %+v", subscribers)
		}
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		issueID := createIssue(t)
		defer deleteIssue(t, issueID)

		// Subscribe first
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues/"+issueID+"/subscribe", nil)
		req = withURLParam(req, "id", issueID)
		testHandler.SubscribeToIssue(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("SubscribeToIssue: expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Unsubscribe
		w = httptest.NewRecorder()
		req = newRequest("POST", "/api/issues/"+issueID+"/unsubscribe", nil)
		req = withURLParam(req, "id", issueID)
		testHandler.UnsubscribeFromIssue(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("UnsubscribeFromIssue: expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]bool
		json.NewDecoder(w.Body).Decode(&resp)
		if resp["subscribed"] {
			t.Fatal("UnsubscribeFromIssue: expected subscribed=false")
		}

		// Verify in DB
		subscribed, err := testHandler.Queries.IsIssueSubscriber(ctx, db.IsIssueSubscriberParams{
			IssueID:  parseUUID(issueID),
			UserType: "member",
			UserID:   parseUUID(testUserID),
		})
		if err != nil {
			t.Fatalf("IsIssueSubscriber: %v", err)
		}
		if subscribed {
			t.Fatal("expected user to NOT be subscribed in DB")
		}
	})

	t.Run("ListAfterUnsubscribe", func(t *testing.T) {
		issueID := createIssue(t)
		defer deleteIssue(t, issueID)

		// Subscribe
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues/"+issueID+"/subscribe", nil)
		req = withURLParam(req, "id", issueID)
		testHandler.SubscribeToIssue(w, req)

		// Unsubscribe
		w = httptest.NewRecorder()
		req = newRequest("POST", "/api/issues/"+issueID+"/unsubscribe", nil)
		req = withURLParam(req, "id", issueID)
		testHandler.UnsubscribeFromIssue(w, req)

		// List should be empty
		w = httptest.NewRecorder()
		req = newRequest("GET", "/api/issues/"+issueID+"/subscribers", nil)
		req = withURLParam(req, "id", issueID)
		testHandler.ListIssueSubscribers(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("ListIssueSubscribers: expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var subscribers []SubscriberResponse
		json.NewDecoder(w.Body).Decode(&subscribers)
		if len(subscribers) != 0 {
			t.Fatalf("ListIssueSubscribers: expected 0 subscribers after unsubscribe, got %d", len(subscribers))
		}
	})

	t.Run("SubscribeWithExplicitBody", func(t *testing.T) {
		issueID := createIssue(t)
		defer deleteIssue(t, issueID)

		agentID := makeTestUUID("agent-sub-01")

		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues/"+issueID+"/subscribe", map[string]any{
			"user_id":   uuidToString(agentID),
			"user_type": "agent",
		})
		req = withURLParam(req, "id", issueID)
		testHandler.SubscribeToIssue(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("SubscribeToIssue: expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify the agent is subscribed
		subscribed, err := testHandler.Queries.IsIssueSubscriber(ctx, db.IsIssueSubscriberParams{
			IssueID:  parseUUID(issueID),
			UserType: "agent",
			UserID:   agentID,
		})
		if err != nil {
			t.Fatalf("IsIssueSubscriber: %v", err)
		}
		if !subscribed {
			t.Fatal("expected agent to be subscribed in DB")
		}
	})

	t.Run("UnsubscribeWithExplicitBody", func(t *testing.T) {
		issueID := createIssue(t)
		defer deleteIssue(t, issueID)

		agentID := makeTestUUID("agent-unsub-01")

		// Subscribe agent first
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues/"+issueID+"/subscribe", map[string]any{
			"user_id":   uuidToString(agentID),
			"user_type": "agent",
		})
		req = withURLParam(req, "id", issueID)
		testHandler.SubscribeToIssue(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("SubscribeToIssue: expected 200, got %d", w.Code)
		}

		// Unsubscribe agent
		w = httptest.NewRecorder()
		req = newRequest("POST", "/api/issues/"+issueID+"/unsubscribe", map[string]any{
			"user_id":   uuidToString(agentID),
			"user_type": "agent",
		})
		req = withURLParam(req, "id", issueID)
		testHandler.UnsubscribeFromIssue(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("UnsubscribeFromIssue: expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify removed
		subscribed, err := testHandler.Queries.IsIssueSubscriber(ctx, db.IsIssueSubscriberParams{
			IssueID:  parseUUID(issueID),
			UserType: "agent",
			UserID:   agentID,
		})
		if err != nil {
			t.Fatalf("IsIssueSubscriber: %v", err)
		}
		if subscribed {
			t.Fatal("expected agent to NOT be subscribed in DB")
		}
	})

	t.Run("ListSubscribersEmpty", func(t *testing.T) {
		issueID := createIssue(t)
		defer deleteIssue(t, issueID)

		w := httptest.NewRecorder()
		req := newRequest("GET", "/api/issues/"+issueID+"/subscribers", nil)
		req = withURLParam(req, "id", issueID)
		testHandler.ListIssueSubscribers(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("ListIssueSubscribers: expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var subscribers []SubscriberResponse
		json.NewDecoder(w.Body).Decode(&subscribers)
		if len(subscribers) != 0 {
			t.Fatalf("expected 0 subscribers for new issue, got %d", len(subscribers))
		}
	})

	t.Run("SubscribeNonexistentIssue", func(t *testing.T) {
		fakeID := "00000000-0000-0000-0000-000000000099"
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues/"+fakeID+"/subscribe", nil)
		req = withURLParam(req, "id", fakeID)
		testHandler.SubscribeToIssue(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for nonexistent issue, got %d: %s", w.Code, w.Body.String())
		}
	})
}
