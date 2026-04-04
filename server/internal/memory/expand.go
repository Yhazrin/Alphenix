package memory

import (
	"strings"
	"unicode"
)

// englishStopwords is a minimal set of common English stopwords.
var englishStopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {},
	"be": {}, "but": {}, "by": {}, "for": {}, "if": {}, "in": {},
	"into": {}, "is": {}, "it": {}, "no": {}, "not": {}, "of": {},
	"on": {}, "or": {}, "such": {}, "that": {}, "the": {}, "their": {},
	"then": {}, "there": {}, "these": {}, "they": {}, "this": {},
	"to": {}, "was": {}, "will": {}, "with": {}, "from": {}, "has": {},
	"have": {}, "had": {}, "been": {}, "do": {}, "does": {}, "did": {},
}

// ExpandQuery tokenizes a query string, filters stopwords, and builds a
// PostgreSQL tsquery with OR + prefix matching: 'term1':* | 'term2':*.
// extraContext can provide additional terms (e.g. issue description).
func ExpandQuery(query string, extraContext ...string) string {
	allText := query
	for _, c := range extraContext {
		allText += " " + c
	}

	terms := extractTerms(allText)
	var filtered []string
	seen := make(map[string]struct{})
	for _, t := range terms {
		lower := strings.ToLower(t)
		if _, stop := englishStopwords[lower]; stop {
			continue
		}
		if len(lower) < 2 {
			continue
		}
		if _, dup := seen[lower]; dup {
			continue
		}
		seen[lower] = struct{}{}
		filtered = append(filtered, lower)
	}

	if len(filtered) == 0 {
		return ""
	}

	// Build tsquery: 'term1':* | 'term2':*
	parts := make([]string, len(filtered))
	for i, t := range filtered {
		parts[i] = "'" + t + "':*"
	}
	return strings.Join(parts, " | ")
}

// extractTerms splits text on non-alphanumeric characters.
func extractTerms(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// isStopword reports whether s is an English stopword.
func isStopword(s string) bool {
	_, ok := englishStopwords[strings.ToLower(s)]
	return ok
}
