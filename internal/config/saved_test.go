package config

import "testing"

func TestSavedQueryStore(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.SaveQuery(SavedQuery{Name: "top", SQL: "SELECT 1", Host: "127.0.0.1:3306", Database: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if q.ID == "" || q.CreatedAt.IsZero() {
		t.Fatalf("bad query: %#v", q)
	}
	if got := s.SavedQueries("127.0.0.1:3306"); len(got) != 1 || got[0].Name != "top" {
		t.Fatalf("list = %#v", got)
	}
	if got := s.SavedQueries("10.0.0.2:3306"); len(got) != 0 {
		t.Fatalf("host filter = %#v", got)
	}
	q.Name = "top2"
	updated, err := s.SaveQuery(q)
	if err != nil || updated.Name != "top2" || updated.CreatedAt.IsZero() {
		t.Fatalf("update = %#v %v", updated, err)
	}
	if err := s.DeleteSavedQuery(q.ID); err != nil {
		t.Fatal(err)
	}
	if got := s.SavedQueries(""); len(got) != 0 {
		t.Fatalf("after delete = %#v", got)
	}
}
