package kb

import (
	"strings"
	"unicode"
)

// Well-known CONTEXT entity types. The API accepts any type that
// ParseEntity allows — new containers (beyond project) do not need a
// server change.
const (
	EntityProject      = "project"
	EntityOrganization = "organization"
	EntityService      = "service"
	EntityEnvironment  = "environment"
	EntityTask         = "task"
)

// ContextEntity is who a knowledge source is bound to. Project is one
// type; Autonomy and others pass whichever entity their world resolved.
type ContextEntity struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func (e ContextEntity) Key() string {
	return e.Type + "/" + e.ID
}

func ParseEntity(typ, id string) (ContextEntity, error) {
	typ = strings.ToLower(strings.TrimSpace(typ))
	id = strings.TrimSpace(id)
	if typ == "" || id == "" {
		return ContextEntity{}, Errorf(ErrInvalidQuery, "context entity needs type and id")
	}
	if !validEntityType(typ) {
		return ContextEntity{}, Errorf(ErrInvalidQuery, "context entity type %q is not a name (use project, organization, service, …)", typ)
	}
	return ContextEntity{Type: typ, ID: id}, nil
}

func validEntityType(typ string) bool {
	if typ == "" || !unicode.IsLetter(rune(typ[0])) {
		return false
	}
	for _, r := range typ {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			continue
		}
		return false
	}
	return true
}

func entityFromBind(bind *ContextEntity, projectID string) (ContextEntity, bool, error) {
	if bind != nil && (strings.TrimSpace(bind.Type) != "" || strings.TrimSpace(bind.ID) != "") {
		e, err := ParseEntity(bind.Type, bind.ID)
		return e, true, err
	}
	if pid := strings.TrimSpace(projectID); pid != "" {
		e, err := ParseEntity(EntityProject, pid)
		return e, true, err
	}
	return ContextEntity{}, false, nil
}
