package memory

import (
	"sort"

	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// RRFK is the standard Reciprocal Rank Fusion constant.
const RRFK = 60

// SearchResult wraps an agent memory entry with its rank and score from a
// single search channel (BM25 or vector).
type SearchResult struct {
	Memory db.AgentMemory
	Score  float64
	Rank   int
}

// FusedResult wraps an agent memory entry with fused scores from both channels.
type FusedResult struct {
	Memory     db.AgentMemory
	FusedScore float64
	BM25Score  float64
	VectorScore float64
}

// RankResults sorts results by Score descending and assigns 1-based ranks.
func RankResults(results []SearchResult) []SearchResult {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	for i := range results {
		results[i].Rank = i + 1
	}
	return results
}

// RRFusion merges ranked lists from BM25 and vector search using Reciprocal
// Rank Fusion: score(d) = Σ 1/(k + rank_d). Returns top-K fused results.
func RRFusion(bm25, vector []SearchResult, limit int) []FusedResult {
	scores := make(map[string]*FusedResult)

	for _, r := range bm25 {
		id := formatUUID(r.Memory.ID.Bytes)
		rrf := 1.0 / float64(RRFK+r.Rank)
		if existing, ok := scores[id]; ok {
			existing.FusedScore += rrf
			existing.BM25Score = r.Score
		} else {
			scores[id] = &FusedResult{
				Memory:     r.Memory,
				FusedScore: rrf,
				BM25Score:  r.Score,
			}
		}
	}

	for _, r := range vector {
		id := formatUUID(r.Memory.ID.Bytes)
		rrf := 1.0 / float64(RRFK+r.Rank)
		if existing, ok := scores[id]; ok {
			existing.FusedScore += rrf
			existing.VectorScore = r.Score
		} else {
			scores[id] = &FusedResult{
				Memory:      r.Memory,
				FusedScore:  rrf,
				VectorScore: r.Score,
			}
		}
	}

	results := make([]FusedResult, 0, len(scores))
	for _, v := range scores {
		results = append(results, *v)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].FusedScore > results[j].FusedScore
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

func formatUUID(b [16]byte) string {
	const hex = "0123456789abcdef"
	buf := make([]byte, 36)
	// 8-4-4-4-12 format
	positions := []int{0, 1, 2, 3, 4, 5, 6, 7, 9, 10, 11, 12, 14, 15, 16, 17, 19, 20, 21, 22, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35}
	for i, pos := range positions {
		buf[pos] = hex[b[i/2]>>(4*uint(1-i%2))&0x0f]
	}
	buf[8] = '-'
	buf[13] = '-'
	buf[18] = '-'
	buf[23] = '-'
	return string(buf)
}
