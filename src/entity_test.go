package kb

import "testing"

func TestParseEntity(t *testing.T) {
	e, err := ParseEntity("Organization", " org-1 ")
	if err != nil || e.Type != EntityOrganization || e.ID != "org-1" || e.Key() != "organization/org-1" {
		t.Fatalf("parse = %+v, %v", e, err)
	}
	for _, typ := range []string{EntityProject, EntityOrganization, EntityService, EntityEnvironment, EntityTask, "team"} {
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
	e, ok, err := entityFromBind(&ContextEntity{Type: "organization", ID: "org-1"})
	if err != nil || !ok || e.Type != EntityOrganization || e.ID != "org-1" {
		t.Fatalf("bind = %+v ok=%v err=%v", e, ok, err)
	}
	if _, ok, err = entityFromBind(nil); err != nil || ok {
		t.Fatalf("nil bind = ok=%v err=%v", ok, err)
	}
	if _, ok, err = entityFromBind(&ContextEntity{}); err != nil || ok {
		t.Fatalf("empty bind = ok=%v err=%v", ok, err)
	}
}
