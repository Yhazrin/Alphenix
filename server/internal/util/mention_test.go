package util

import (
	"testing"
)

// ---------------------------------------------------------------------------
// ParseMentions
// ---------------------------------------------------------------------------

func TestParseMentions_Empty(t *testing.T) {
	got := ParseMentions("")
	if len(got) != 0 {
		t.Errorf("expected 0 mentions, got %d", len(got))
	}
}

func TestParseMentions_NoMentions(t *testing.T) {
	got := ParseMentions("just plain text with no mentions")
	if len(got) != 0 {
		t.Errorf("expected 0 mentions, got %d", len(got))
	}
}

func TestParseMentions_SingleMember(t *testing.T) {
	content := `hello [@alice](mention://member/abc-123)`
	got := ParseMentions(content)
	if len(got) != 1 {
		t.Fatalf("expected 1 mention, got %d", len(got))
	}
	if got[0].Type != "member" || got[0].ID != "abc-123" {
		t.Errorf("got %+v, want member/abc-123", got[0])
	}
}

func TestParseMentions_SingleAgent(t *testing.T) {
	content := `[@bot](mention://agent/def-456) please help`
	got := ParseMentions(content)
	if len(got) != 1 {
		t.Fatalf("expected 1 mention, got %d", len(got))
	}
	if got[0].Type != "agent" || got[0].ID != "def-456" {
		t.Errorf("got %+v, want agent/def-456", got[0])
	}
}

func TestParseMentions_IssueMention(t *testing.T) {
	content := `see [MUL-42](mention://issue/abc-def-012)`
	got := ParseMentions(content)
	if len(got) != 1 {
		t.Fatalf("expected 1 mention, got %d", len(got))
	}
	if got[0].Type != "issue" || got[0].ID != "abc-def-012" {
		t.Errorf("got %+v, want issue/abc-def-012", got[0])
	}
}

func TestParseMentions_AllMention(t *testing.T) {
	content := `[@everyone](mention://all/all)`
	got := ParseMentions(content)
	if len(got) != 1 {
		t.Fatalf("expected 1 mention, got %d", len(got))
	}
	if got[0].Type != "all" || got[0].ID != "all" {
		t.Errorf("got %+v, want all/all", got[0])
	}
}

func TestParseMentions_Deduplication(t *testing.T) {
	content := `[@a](mention://member/abc-01) and [@a](mention://member/abc-01) again`
	got := ParseMentions(content)
	if len(got) != 1 {
		t.Errorf("expected 1 deduplicated mention, got %d", len(got))
	}
}

func TestParseMentions_MultipleDifferent(t *testing.T) {
	content := `[@a](mention://member/ab-01) [@b](mention://agent/ab-02) [C](mention://issue/ab-03)`
	got := ParseMentions(content)
	if len(got) != 3 {
		t.Fatalf("expected 3 mentions, got %d", len(got))
	}
	types := map[string]bool{}
	for _, m := range got {
		types[m.Type] = true
	}
	for _, want := range []string{"member", "agent", "issue"} {
		if !types[want] {
			t.Errorf("missing mention type %q", want)
		}
	}
}

func TestParseMentions_SameIDDifferentType(t *testing.T) {
	content := `[@a](mention://member/ab-01) [B](mention://issue/ab-01)`
	got := ParseMentions(content)
	if len(got) != 2 {
		t.Errorf("expected 2 mentions (same ID, different type), got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// Mention.IsMentionAll
// ---------------------------------------------------------------------------

func TestIsMentionAll_True(t *testing.T) {
	m := Mention{Type: "all", ID: "all"}
	if !m.IsMentionAll() {
		t.Error("expected true for type=all")
	}
}

func TestIsMentionAll_False(t *testing.T) {
	m := Mention{Type: "member", ID: "abc"}
	if m.IsMentionAll() {
		t.Error("expected false for type=member")
	}
}

// ---------------------------------------------------------------------------
// HasMentionAll
// ---------------------------------------------------------------------------

func TestHasMentionAll_Empty(t *testing.T) {
	if HasMentionAll(nil) {
		t.Error("expected false for nil slice")
	}
	if HasMentionAll([]Mention{}) {
		t.Error("expected false for empty slice")
	}
}

func TestHasMentionAll_Contains(t *testing.T) {
	mentions := []Mention{
		{Type: "member", ID: "abc"},
		{Type: "all", ID: "all"},
	}
	if !HasMentionAll(mentions) {
		t.Error("expected true when @all is present")
	}
}

func TestHasMentionAll_Absent(t *testing.T) {
	mentions := []Mention{
		{Type: "member", ID: "abc"},
		{Type: "agent", ID: "def"},
	}
	if HasMentionAll(mentions) {
		t.Error("expected false when no @all present")
	}
}
