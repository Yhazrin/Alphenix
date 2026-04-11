package memory

import (
	"strings"
	"testing"
)

func TestExpandQuery_Basic(t *testing.T) {
	q := ExpandQuery("how to build a router")
	if q == "" {
		t.Error("expected non-empty query")
	}
	// "how", "build", "router" should remain; "a" and "to" are stopwords
	if !strings.Contains(q, "build") {
		t.Error("expected 'build' in output")
	}
	if !strings.Contains(q, "router") {
		t.Error("expected 'router' in output")
	}
	if strings.Contains(q, "'to':*") {
		t.Error("stopword 'to' should be filtered")
	}
	if strings.Contains(q, "'a':*") {
		t.Error("stopword 'a' should be filtered")
	}
}

func TestExpandQuery_PrefixMatching(t *testing.T) {
	q := ExpandQuery("testing")
	if !strings.Contains(q, "'testing':*") {
		t.Errorf("expected prefix match format, got %q", q)
	}
}

func TestExpandQuery_ORSeparator(t *testing.T) {
	q := ExpandQuery("alpha beta gamma")
	parts := strings.Split(q, " | ")
	if len(parts) < 3 {
		t.Errorf("expected 3 OR-separated terms, got %d: %q", len(parts), q)
	}
}

func TestExpandQuery_ExtraContext(t *testing.T) {
	q := ExpandQuery("deploy", "server configuration")
	if !strings.Contains(q, "deploy") {
		t.Error("expected 'deploy' from query")
	}
	if !strings.Contains(q, "server") {
		t.Error("expected 'server' from extra context")
	}
	if !strings.Contains(q, "configuration") {
		t.Error("expected 'configuration' from extra context")
	}
}

func TestExpandQuery_Deduplicates(t *testing.T) {
	q := ExpandQuery("test test test")
	parts := strings.Split(q, " | ")
	if len(parts) != 1 {
		t.Errorf("expected 1 unique term, got %d: %q", len(parts), q)
	}
}

func TestExpandQuery_AllStopwords(t *testing.T) {
	q := ExpandQuery("the a an is")
	if q != "" {
		t.Errorf("expected empty string for all-stopword query, got %q", q)
	}
}

func TestExpandQuery_ShortTerms(t *testing.T) {
	q := ExpandQuery("a b cd")
	// "a" is stopword, "b" is < 2 chars, "cd" should remain
	if !strings.Contains(q, "cd") {
		t.Error("expected 'cd' in output")
	}
	if strings.Contains(q, "'b':*") {
		t.Error("single-char 'b' should be filtered")
	}
}

func TestExtractTerms_Basic(t *testing.T) {
	terms := extractTerms("hello world")
	if len(terms) != 2 {
		t.Fatalf("expected 2 terms, got %d: %v", len(terms), terms)
	}
}

func TestExtractTerms_Punctuation(t *testing.T) {
	terms := extractTerms("foo-bar.baz_qux!")
	if len(terms) != 4 {
		t.Errorf("expected 4 terms split on punctuation, got %d: %v", len(terms), terms)
	}
}

func TestExtractTerms_Empty(t *testing.T) {
	terms := extractTerms("")
	if len(terms) != 0 {
		t.Errorf("expected 0 terms for empty string, got %d", len(terms))
	}
}

func TestIsStopword(t *testing.T) {
	if !isStopword("the") {
		t.Error("'the' should be a stopword")
	}
	if !isStopword("THE") {
		t.Error("'THE' should be a stopword (case-insensitive)")
	}
	if isStopword("deploy") {
		t.Error("'deploy' should not be a stopword")
	}
}
