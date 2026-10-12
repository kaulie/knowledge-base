package kb

import (
	"strings"
	"testing"
)

const sampleDoc = `# Runtime Architecture
## Runtime Responsibility
Runtime executes actions.

## Heartbeat Protocol
Runtime periodically sends heartbeat to the control plane.

### Interval
The interval is configurable.

## Recovery
Runtime recovers after a crash.
`

func TestParseMarkdownSections(t *testing.T) {
	sections, err := ParseMarkdown("doc-001", []byte(sampleDoc))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 4 {
		t.Fatalf("want 4 sections, got %d", len(sections))
	}
	if sections[1].Path != "Runtime Architecture / Heartbeat Protocol" {
		t.Errorf("path = %q", sections[1].Path)
	}
	if !strings.Contains(sections[1].Content, "heartbeat to the control plane") {
		t.Errorf("content = %q", sections[1].Content)
	}
}

func TestParseMarkdownRejectsNonUTF8(t *testing.T) {
	if _, err := ParseMarkdown("doc-bad", []byte{0xff, 0xfe, 0x00}); !IsCode(err, ErrParseFailed) {
		t.Fatalf("got %v", err)
	}
}
