package kb

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

func queryTerms(query string) []string {
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	seen := map[string]bool{}
	var terms []string
	for _, f := range fields {
		t := strings.ToLower(f)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		terms = append(terms, t)
	}
	return terms
}

func sectionScore(s Section, terms []string) float64 {
	if len(terms) == 0 {
		return 0
	}
	hay := strings.ToLower(s.Heading + "\n" + s.Path + "\n" + s.Content)
	heading := strings.ToLower(s.Heading)
	var score float64
	for _, t := range terms {
		n := strings.Count(hay, t)
		if n == 0 {
			continue
		}
		score += 1 + math.Log(float64(n))
		if strings.Contains(heading, t) {
			score += 1
		}
	}
	return score
}

func snippet(content, heading string, terms []string, width int) string {
	flat := strings.Join(strings.Fields(strings.ReplaceAll(content, "\n", " ")), " ")
	if flat == "" {
		return heading
	}
	runes := []rune(flat)
	lower := strings.ToLower(flat)
	byteIdx := -1
	for _, t := range terms {
		if i := strings.Index(lower, t); i >= 0 && (byteIdx < 0 || i < byteIdx) {
			byteIdx = i
		}
	}
	idx := 0
	if byteIdx > 0 {
		idx = utf8.RuneCountInString(flat[:byteIdx])
	}
	if width < 40 {
		width = 160
	}
	start := idx - width/3
	if start < 0 {
		start = 0
	}
	end := start + width
	if end > len(runes) {
		end = len(runes)
		if end-width > 0 {
			start = end - width
		} else {
			start = 0
		}
	}
	out := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out += "…"
	}
	return out
}

func rankResults(results []SearchResult, limit int) []SearchResult {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].DocumentName != results[j].DocumentName {
			return results[i].DocumentName < results[j].DocumentName
		}
		return results[i].SectionID < results[j].SectionID
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}
