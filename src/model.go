package kb

import "time"

const (
	KindFilesystem           = "filesystem"
	KindSourceCodeRepository = "source_code_repository"
	KindCloud                = "cloud"
)

const (
	ProviderLocal    = "local"
	ProviderGitLocal = "git-local"
	ProviderGitHub   = "github"
	ProviderGitLab   = "gitlab"
	ProviderGitea    = "gitea"
	ProviderGit      = "git"
	ProviderS3       = "s3"
	ProviderGCS      = "gcs"
	ProviderAzure    = "azure"
	ProviderOSS      = "oss"
	ProviderSFTP     = "sftp"
	ProviderSSH      = "ssh"
)

const (
	ShapeFile      = "file"
	ShapeDirectory = "directory"
)

const (
	SyncIndexed   = "indexed"
	SyncUnchanged = "unchanged"
)

// Source is where a knowledge source actually lives.
type Source struct {
	Location   string `json:"location,omitempty"`
	Path       string `json:"path,omitempty"`
	Repository string `json:"repository,omitempty"`
	Ref        string `json:"ref,omitempty"`
}

// Access is reserved for permission control. V1 does not enforce it:
// entity-scoped reads only see knowledge bound to that CONTEXT entity.
type Access struct {
	// Mode is "bound" (default): only bound context entities may search.
	// "open" is reserved for a later ACL implementation.
	Mode string `json:"mode,omitempty"`
}

// Knowledge is one registered source (a knowledge_id). Documents and
// sections belong to it. CONTEXT entities bind to it; they do not own it.
type Knowledge struct {
	ID        string            `json:"id"`
	Name      string            `json:"name,omitempty"`
	Kind      string            `json:"kind"`
	Provider  string            `json:"provider"`
	Shape     string            `json:"shape"`
	Source    Source            `json:"source"`
	Suffixes  []string          `json:"suffixes,omitempty"`
	Access    Access            `json:"access,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Revision  string            `json:"revision,omitempty"`
	Checksum  string            `json:"checksum,omitempty"`
	Documents int               `json:"documents,omitempty"`
	// Files is the indexed document paths for this source (context.list).
	// Capped; see maxListedFiles. Agents use it to see whether AGENT.md exists
	// before searching headings that happen to contain the word "agent".
	Files     []string          `json:"files,omitempty"`
	Bindings  []ContextEntity   `json:"bindings,omitempty"`
	LastSync  *SyncResult       `json:"last_sync,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Document is one file (or object) inside a knowledge source.
type Document struct {
	ID          string `json:"id"`
	KnowledgeID string `json:"knowledge_id"`
	Name        string `json:"name"`
	Path        string `json:"path,omitempty"`
	Revision    string `json:"revision,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
}

// Section is one heading-delimited slice of a parsed document.
type Section struct {
	ID          string `json:"id"`
	DocumentID  string `json:"document_id"`
	KnowledgeID string `json:"knowledge_id"`
	Heading     string `json:"heading"`
	Path        string `json:"path,omitempty"`
	Content     string `json:"content"`
	Order       int    `json:"order"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	Revision    string `json:"revision,omitempty"`
	Source      Source `json:"source,omitempty"`
}

type SyncLog struct {
	At      time.Time `json:"at"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type SyncResult struct {
	KnowledgeID string    `json:"knowledge_id"`
	Status      string    `json:"status"`
	Revision    string    `json:"revision,omitempty"`
	Checksum    string    `json:"checksum,omitempty"`
	Documents   int       `json:"documents"`
	Sections    int       `json:"sections"`
	Logs        []SyncLog `json:"logs,omitempty"`
}

type ContextSummary struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Knowledge int    `json:"knowledge"`
	Documents int    `json:"documents"`
	Sections  int    `json:"sections"`
}

type BrowseSection struct {
	ID      string `json:"id"`
	Heading string `json:"heading"`
}

type BrowseFile struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Path     string          `json:"path,omitempty"`
	Sections []BrowseSection `json:"sections"`
}

type BrowseKnowledge struct {
	ID        string       `json:"id"`
	Name      string       `json:"name,omitempty"`
	Kind      string       `json:"kind"`
	Provider  string       `json:"provider"`
	Revision  string       `json:"revision,omitempty"`
	Documents int          `json:"documents"`
	Files     []BrowseFile `json:"files"`
}

type BrowseResult struct {
	Entity    ContextEntity     `json:"entity"`
	Name      string            `json:"name,omitempty"`
	Knowledge []BrowseKnowledge `json:"knowledge"`
}

type SearchRequest struct {
	Entity ContextEntity
	Query  string
	Limit  int
}

type SearchResult struct {
	KnowledgeID  string  `json:"knowledge_id"`
	DocumentID   string  `json:"document_id"`
	SectionID    string  `json:"section_id"`
	DocumentName string  `json:"document_name"`
	Heading      string  `json:"heading"`
	Snippet      string  `json:"snippet"`
	Score        float64 `json:"score"`
	Revision     string  `json:"revision,omitempty"`
}

type ListedFile struct {
	Name     string
	Location string
	Path     string
}
