package memory

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multicode/server/pkg/db/generated"
)

func makeMemory(id byte, content string, similarity float64) SearchResult {
	return SearchResult{
		Memory: db.AgentMemory{
			ID:      pgtype.UUID{Bytes: [16]byte{id}, Valid: true},
			Content: content,
		},
		Score: similarity,
	}
}

func TestRankResults(t *testing.T) {
	results := []SearchResult{
		makeMemory(1, "low", 0.1),
		makeMemory(2, "high", 0.9),
		makeMemory(3, "mid", 0.5),
	}

	ranked := RankResults(results)

	if ranked[0].Rank != 1 || ranked[0].Score != 0.9 {
		t.Errorf("expected rank 1 with score 0.9, got rank %d score %f", ranked[0].Rank, ranked[0].Score)
	}
	if ranked[1].Rank != 2 || ranked[1].Score != 0.5 {
		t.Errorf("expected rank 2 with score 0.5, got rank %d score %f", ranked[1].Rank, ranked[1].Score)
	}
	if ranked[2].Rank != 3 || ranked[2].Score != 0.1 {
		t.Errorf("expected rank 3 with score 0.1, got rank %d score %f", ranked[2].Rank, ranked[2].Score)
	}
}

func TestRRFusion(t *testing.T) {
	bm25 := []SearchResult{
		makeMemory(1, "alpha", 5.0), // rank 1
		makeMemory(2, "beta", 3.0),  // rank 2
	}
	vector := []SearchResult{
		makeMemory(2, "beta", 0.95), // rank 1 (same doc as bm25 rank 2)
		makeMemory(3, "gamma", 0.8), // rank 2
	}

	fused := RRFusion(bm25, vector, 3)

	if len(fused) != 3 {
		t.Fatalf("expected 3 fused results, got %d", len(fused))
	}

	// Doc 2 appears in both lists, should have highest fused score.
	if fused[0].Memory.ID.Bytes[0] != 2 {
		t.Errorf("expected doc 2 first, got doc %d", fused[0].Memory.ID.Bytes[0])
	}

	// Doc 2 should have both scores populated.
	if fused[0].BM25Score == 0 || fused[0].VectorScore == 0 {
		t.Errorf("expected doc 2 to have both BM25 and vector scores")
	}
}

func TestRRFusionLimit(t *testing.T) {
	bm25 := []SearchResult{
		makeMemory(1, "a", 1.0),
		makeMemory(2, "b", 2.0),
		makeMemory(3, "c", 3.0),
	}

	fused := RRFusion(bm25, nil, 2)
	if len(fused) != 2 {
		t.Errorf("expected limit 2, got %d", len(fused))
	}
}

func TestExpandQuery(t *testing.T) {
	q := ExpandQuery("fix the login bug in authentication")
	if q == "" {
		t.Fatal("expected non-empty query")
	}
	// Should not contain stopwords like "the", "in".
	if contains(q, "'the':*") || contains(q, "'in':*") {
		t.Errorf("stopwords should be filtered: %s", q)
	}
	// Should contain meaningful terms.
	if !contains(q, "'login':*") || !contains(q, "'bug':*") || !contains(q, "'authentication':*") {
		t.Errorf("expected meaningful terms: %s", q)
	}
}

func TestExpandQueryWithExtraContext(t *testing.T) {
	q := ExpandQuery("fix bug", "user authentication flow")
	if !contains(q, "'authentication':*") || !contains(q, "'user':*") {
		t.Errorf("expected extra context terms: %s", q)
	}
}

func TestExpandQueryEmpty(t *testing.T) {
	q := ExpandQuery("the a an")
	if q != "" {
		t.Errorf("expected empty query for stopwords only, got %s", q)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstr(s, sub)
}

func searchSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
