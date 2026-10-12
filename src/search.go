package kb

import (
	"math"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// parsedQuery splits a search string into filename tokens and ordinary words.
// "AGENT.md" stays one filename so a heading that contains the word "agent"
// does not rank as that file. "heartbeat protocol" stays two words.
type parsedQuery struct {
	Words []string
	Files []string
}

func (q parsedQuery) empty() bool {
	return len(q.Words) == 0 && len(q.Files) == 0
}

func (q parsedQuery) terms() []string {
	out := make([]string, 0, len(q.Files)+len(q.Words))
	out = append(out, q.Files...)
	out = append(out, q.Words...)
	return out
}

func queryTerms(query string) []string {
	return parseQuery(query).terms()
}

func parseQuery(query string) parsedQuery {
	seenFile := map[string]bool{}
	seenWord := map[string]bool{}
	var q parsedQuery
	for _, field := range strings.Fields(query) {
		field = strings.Trim(field, `"'`+"`")
		if field == "" {
			continue
		}
		lower := strings.ToLower(field)
		if isFilenameToken(lower) {
			if !seenFile[lower] {
				seenFile[lower] = true
				q.Files = append(q.Files, lower)
			}
			continue
		}
		for _, t := range splitWords(field) {
			if seenWord[t] || seenFile[t] {
				continue
			}
			seenWord[t] = true
			q.Words = append(q.Words, t)
		}
	}
	return q
}

func isFilenameToken(t string) bool {
	if t == "" {
		return false
	}
	if strings.ContainsAny(t, `/\`) {
		return true
	}
	dot := strings.LastIndex(t, ".")
	if dot <= 0 || dot == len(t)-1 {
		return false
	}
	return hasAlnum(t[:dot]) && hasAlnum(t[dot+1:])
}

func hasAlnum(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func splitWords(field string) []string {
	parts := strings.FieldsFunc(field, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var out []string
	seen := map[string]bool{}
	for _, p := range parts {
		t := strings.ToLower(p)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func sectionScore(s Section, docName, docPath string, q parsedQuery) float64 {
	if q.empty() {
		return 0
	}
	var score float64
	for _, f := range q.Files {
		n := fileMatchScore(docName, docPath, f)
		if n <= 0 {
			continue
		}
		score += n
		if s.Order <= 1 {
			score += 2
		}
	}
	if len(q.Words) == 0 {
		return score
	}
	hay := strings.ToLower(s.Heading + "\n" + s.Path + "\n" + s.Content)
	heading := strings.ToLower(s.Heading)
	for _, t := range q.Words {
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

func fileMatchScore(docName, docPath, file string) float64 {
	name := strings.ToLower(strings.TrimSpace(docName))
	p := strings.ToLower(strings.TrimSpace(docPath))
	base := func(s string) string {
		s = strings.ReplaceAll(s, "\\", "/")
		return path.Base(s)
	}
	switch {
	case name == file || p == file || base(name) == file || base(p) == file:
		return 12
	case strings.HasSuffix(name, "/"+file) || strings.HasSuffix(p, "/"+file):
		return 12
	case strings.Contains(name, file) || strings.Contains(p, file):
		return 8
	}
	return 0
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
