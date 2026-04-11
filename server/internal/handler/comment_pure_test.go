package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// --- commentToResponse tests ---

func TestCommentToResponse_Fields(t *testing.T) {
	c := db.Comment{
		ID:         makeTestUUID("comment1"),
		IssueID:    makeTestUUID("issue1"),
		AuthorType: "member",
		AuthorID:   makeTestUUID("user1"),
		Content:    "hello",
		Type:       "comment",
	}
	resp := commentToResponse(c, nil, nil)
	if resp.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if resp.Content != "hello" {
		t.Fatalf("expected content 'hello', got %q", resp.Content)
	}
	if resp.AuthorType != "member" {
		t.Fatalf("expected author_type 'member', got %q", resp.AuthorType)
	}
	// Nil reactions/attachments should become empty slices (not nil).
	if resp.Reactions == nil {
		t.Fatal("expected non-nil reactions slice")
	}
	if resp.Attachments == nil {
		t.Fatal("expected non-nil attachments slice")
	}
}

func TestCommentToResponse_WithParent(t *testing.T) {
	parentStr := uuidToString(makeTestUUID("parent1"))
	c := db.Comment{
		ID:       makeTestUUID("comment1"),
		IssueID:  makeTestUUID("issue1"),
		Content:  "reply",
		ParentID: parseUUID(parentStr),
	}
	resp := commentToResponse(c, nil, nil)
	if resp.ParentID == nil {
		t.Fatal("expected non-nil parent_id")
	}
	if *resp.ParentID != parentStr {
		t.Fatalf("expected parent_id %q, got %q", parentStr, *resp.ParentID)
	}
}

func TestCommentToResponse_NoParent(t *testing.T) {
	c := db.Comment{
		ID:      makeTestUUID("comment1"),
		IssueID: makeTestUUID("issue1"),
		Content: "top level",
	}
	resp := commentToResponse(c, nil, nil)
	if resp.ParentID != nil {
		t.Fatal("expected nil parent_id for top-level comment")
	}
}

func TestCommentToResponse_PreservesReactions(t *testing.T) {
	reactions := []ReactionResponse{
		{ID: "r1", Emoji: "+1"},
		{ID: "r2", Emoji: "heart"},
	}
	c := db.Comment{
		ID:      makeTestUUID("c1"),
		IssueID: makeTestUUID("i1"),
	}
	resp := commentToResponse(c, reactions, nil)
	if len(resp.Reactions) != 2 {
		t.Fatalf("expected 2 reactions, got %d", len(resp.Reactions))
	}
	if resp.Reactions[0].Emoji != "+1" {
		t.Fatalf("expected emoji '+1', got %q", resp.Reactions[0].Emoji)
	}
}

// --- isAgentSelfTrigger tests ---

func TestIsAgentSelfTrigger_MemberAuthor(t *testing.T) {
	h := &Handler{}
	comment := db.Comment{AuthorType: "member", AuthorID: makeTestUUID("user1")}
	issue := db.Issue{AssigneeID: makeTestUUID("agent1")}
	if h.isAgentSelfTrigger(comment, issue) {
		t.Fatal("member comment should not be agent self-trigger")
	}
}

func TestIsAgentSelfTrigger_NoAssignee(t *testing.T) {
	h := &Handler{}
	comment := db.Comment{AuthorType: "agent", AuthorID: makeTestUUID("agent1")}
	issue := db.Issue{} // AssigneeID is zero UUID (not valid)
	if h.isAgentSelfTrigger(comment, issue) {
		t.Fatal("should not trigger when issue has no assignee")
	}
}

func TestIsAgentSelfTrigger_SameAgent(t *testing.T) {
	h := &Handler{}
	agentID := makeTestUUID("agent1")
	comment := db.Comment{AuthorType: "agent", AuthorID: agentID}
	issue := db.Issue{AssigneeID: agentID}
	if !h.isAgentSelfTrigger(comment, issue) {
		t.Fatal("same agent commenting on own issue should be self-trigger")
	}
}

func TestIsAgentSelfTrigger_DifferentAgent(t *testing.T) {
	h := &Handler{}
	comment := db.Comment{AuthorType: "agent", AuthorID: makeTestUUID("agent1")}
	issue := db.Issue{AssigneeID: makeTestUUID("agent2")}
	if h.isAgentSelfTrigger(comment, issue) {
		t.Fatal("different agent should not be self-trigger")
	}
}

// --- commentMentionsOthersButNotAssignee tests ---

func makeMentionLink(label, typ, id string) string {
	return "[" + label + "](mention://" + typ + "/" + id + ")"
}

func TestCommentMentionsOthersButNotAssignee_NoMentions(t *testing.T) {
	h := &Handler{}
	issue := db.Issue{AssigneeID: makeTestUUID("agent1")}
	if h.commentMentionsOthersButNotAssignee("just a plain comment", issue) {
		t.Fatal("no mentions should return false")
	}
}

func TestCommentMentionsOthersButNotAssignee_MentionsAssignee(t *testing.T) {
	h := &Handler{}
	agentID := uuidToString(makeTestUUID("agent1"))
	content := "hey " + makeMentionLink("@Bot", "agent", agentID) + " do this"
	issue := db.Issue{AssigneeID: parseUUID(agentID)}
	if h.commentMentionsOthersButNotAssignee(content, issue) {
		t.Fatal("mentioning assignee should return false")
	}
}

func TestCommentMentionsOthersButNotAssignee_MentionsOtherAgent(t *testing.T) {
	h := &Handler{}
	assigneeID := uuidToString(makeTestUUID("agent1"))
	otherID := uuidToString(makeTestUUID("agent2"))
	content := "hey " + makeMentionLink("@Other", "agent", otherID)
	issue := db.Issue{AssigneeID: parseUUID(assigneeID)}
	if !h.commentMentionsOthersButNotAssignee(content, issue) {
		t.Fatal("mentioning other agent but not assignee should return true")
	}
}

func TestCommentMentionsOthersButNotAssignee_MentionsAll(t *testing.T) {
	h := &Handler{}
	issue := db.Issue{AssigneeID: makeTestUUID("agent1")}
	content := "[@all](mention://all/all)"
	if !h.commentMentionsOthersButNotAssignee(content, issue) {
		t.Fatal("@all mention should suppress trigger")
	}
}

func TestCommentMentionsOthersButNotAssignee_NoAssigneeWithMention(t *testing.T) {
	h := &Handler{}
	otherID := uuidToString(makeTestUUID("agent2"))
	content := "hey " + makeMentionLink("@Other", "agent", otherID)
	issue := db.Issue{} // no assignee
	if !h.commentMentionsOthersButNotAssignee(content, issue) {
		t.Fatal("no assignee with mention should return true")
	}
}

func TestCommentMentionsOthersButNotAssignee_IssueMentionsFiltered(t *testing.T) {
	h := &Handler{}
	agentID := uuidToString(makeTestUUID("agent1"))
	// Issue mentions are filtered out, so this should behave as "no mentions".
	content := "[MUL-123](mention://issue/abc-def) just referencing an issue"
	issue := db.Issue{AssigneeID: parseUUID(agentID)}
	if h.commentMentionsOthersButNotAssignee(content, issue) {
		t.Fatal("issue mentions only should not suppress trigger")
	}
}

// --- isReplyToMemberThread tests ---

func TestIsReplyToMemberThread_NotAReply(t *testing.T) {
	h := &Handler{}
	issue := db.Issue{AssigneeID: makeTestUUID("agent1")}
	if h.isReplyToMemberThread(nil, "top level", issue) {
		t.Fatal("top-level comment (nil parent) should not suppress")
	}
}

func TestIsReplyToMemberThread_AgentStartedThread(t *testing.T) {
	h := &Handler{}
	parent := &db.Comment{AuthorType: "agent", AuthorID: makeTestUUID("agent2")}
	issue := db.Issue{AssigneeID: makeTestUUID("agent1")}
	if h.isReplyToMemberThread(parent, "replying", issue) {
		t.Fatal("reply in agent-started thread should not suppress")
	}
}

func TestIsReplyToMemberThread_MemberThreadNoMention(t *testing.T) {
	h := &Handler{}
	parent := &db.Comment{AuthorType: "member", AuthorID: makeTestUUID("user1")}
	issue := db.Issue{AssigneeID: makeTestUUID("agent1")}
	if !h.isReplyToMemberThread(parent, "just chatting", issue) {
		t.Fatal("reply in member thread without mentioning assignee should suppress")
	}
}

func TestIsReplyToMemberThread_MemberThreadMentionsAssignee(t *testing.T) {
	h := &Handler{}
	agentID := uuidToString(makeTestUUID("agent1"))
	parent := &db.Comment{AuthorType: "member", AuthorID: makeTestUUID("user1")}
	content := "hey " + makeMentionLink("@Bot", "agent", agentID) + " please help"
	issue := db.Issue{AssigneeID: parseUUID(agentID)}
	if h.isReplyToMemberThread(parent, content, issue) {
		t.Fatal("reply mentioning assignee should not suppress")
	}
}

func TestIsReplyToMemberThread_ParentMentionsAssignee(t *testing.T) {
	h := &Handler{}
	agentID := uuidToString(makeTestUUID("agent1"))
	parentContent := "asking " + makeMentionLink("@Bot", "agent", agentID) + " for help"
	parent := &db.Comment{AuthorType: "member", AuthorID: makeTestUUID("user1"), Content: parentContent}
	issue := db.Issue{AssigneeID: parseUUID(agentID)}
	// Reply doesn't mention agent, but parent does — should NOT suppress.
	if h.isReplyToMemberThread(parent, "sounds good", issue) {
		t.Fatal("reply inheriting assignee mention from parent should not suppress")
	}
}

func TestIsReplyToMemberThread_NoAssignee(t *testing.T) {
	h := &Handler{}
	parent := &db.Comment{AuthorType: "member", AuthorID: makeTestUUID("user1")}
	issue := db.Issue{} // no assignee
	if !h.isReplyToMemberThread(parent, "reply", issue) {
		t.Fatal("member thread with no assignee should suppress")
	}
}

// Verify we can import pgtype (used by makeTestUUID).
var _ pgtype.UUID
