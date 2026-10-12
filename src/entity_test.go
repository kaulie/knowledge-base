package kb

import "testing"

func TestParseEntity(t *testing.T) {
	e, err := ParseEntity("Project", " project-a ")
	if err != nil || e.Type != EntityProject || e.ID != "project-a" || e.Key() != "project/project-a" {
		t.Fatalf("parse project = %+v, %v", e, err)
	}
	for _, typ := range []string{EntityOrganization, EntityService, EntityEnvironment, EntityTask, "team"} {
		if _, err := ParseEntity(typ, "x"); err != nil {
			t.Fatalf("type %q: %v", typ, err)
		}
	}
	if _, err := ParseEntity("", "x"); err == nil || !IsCode(err, ErrInvalidQuery) {
		t.Fatalf("empty type = %v", err)
	}
	if _, err := ParseEntity("not-a-type", "x"); err == nil || !IsCode(err, ErrInvalidQuery) {
		t.Fatalf("hyphen type = %v", err)
	}
}

func TestEntityFromBind(t *testing.T) {
	e, ok, err := entityFromBind(&ContextEntity{Type: "organization", ID: "org-1"}, "project-a")
	if err != nil || !ok || e.Type != EntityOrganization || e.ID != "org-1" {
		t.Fatalf("bind object wins = %+v ok=%v err=%v", e, ok, err)
	}
	e, ok, err = entityFromBind(nil, "project-a")
	if err != nil || !ok || e.Type != EntityProject || e.ID != "project-a" {
		t.Fatalf("project alias = %+v ok=%v err=%v", e, ok, err)
	}
	if _, ok, err = entityFromBind(nil, ""); err != nil || ok {
		t.Fatalf("no bind = ok=%v err=%v", ok, err)
	}
}
