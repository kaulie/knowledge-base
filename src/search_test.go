package kb

import (
	"strings"
	"testing"
)

func TestParseQueryKeepsFilenamesTogether(t *testing.T) {
	q := parseQuery("AGENT.md")
	if len(q.Files) != 1 || q.Files[0] != "agent.md" || len(q.Words) != 0 {
		t.Fatalf("AGENT.md = %+v, want one filename and no words", q)
	}
	q = parseQuery("home_brain.py")
	if len(q.Files) != 1 || q.Files[0] != "home_brain.py" || len(q.Words) != 0 {
		t.Fatalf("home_brain.py = %+v", q)
	}
	q = parseQuery("docs/AGENT.md")
	if len(q.Files) != 1 || q.Files[0] != "docs/agent.md" {
		t.Fatalf("path = %+v", q)
	}
	q = parseQuery("heartbeat protocol")
	if len(q.Files) != 0 || strings.Join(q.Words, ",") != "heartbeat,protocol" {
		t.Fatalf("words = %+v", q)
	}
	q = parseQuery("see AGENT.md for brain")
	if strings.Join(q.Files, ",") != "agent.md" || strings.Join(q.Words, ",") != "see,for,brain" {
		t.Fatalf("mixed = %+v", q)
	}
}

func TestSectionScorePrefersTheNamedFile(t *testing.T) {
	q := parseQuery("AGENT.md")
	agent := Section{Heading: "Autonomy", Content: "operating guide", Order: 1}
	if got := sectionScore(agent, "AGENT.md", "AGENT.md", q); got < 12 {
		t.Fatalf("AGENT.md score=%v, want a filename hit", got)
	}
	boundaries := Section{Heading: "agent: brain", Path: "agent: brain", Content: "can_modify server/", Order: 1}
	if got := sectionScore(boundaries, "docs/project-map/BOUNDARIES.md", "docs/project-map/BOUNDARIES.md", q); got != 0 {
		t.Fatalf("BOUNDARIES.md score=%v, want 0: heading agent is not AGENT.md", got)
	}
}

func TestSectionScoreFilenameMissHasNoWordLeak(t *testing.T) {
	q := parseQuery("home_brain.py")
	sec := Section{Heading: "LAN Brain", Content: "restart the brain process", Order: 1}
	if got := sectionScore(sec, "docs/project-map/RUNTIME.md", "docs/project-map/RUNTIME.md", q); got != 0 {
		t.Fatalf("RUNTIME.md score=%v, want 0 for a missing home_brain.py", got)
	}
}
