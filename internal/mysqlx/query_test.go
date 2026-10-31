package mysqlx

import (
	"testing"
)

func TestSplitStatements(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"SELECT 1", []string{"SELECT 1"}},
		{"SELECT 1; SELECT 2;", []string{"SELECT 1", "SELECT 2"}},
		{"SELECT 'a;b' AS x; SELECT 2", []string{"SELECT 'a;b' AS x", "SELECT 2"}},
		{"SELECT 1 -- comment; still comment\n; SELECT 2", []string{"SELECT 1 -- comment; still comment", "SELECT 2"}},
		{"/* a; b */ SELECT 3", []string{"/* a; b */ SELECT 3"}},
		{"SELECT `weird;name` FROM t;", []string{"SELECT `weird;name` FROM t"}},
		{"   ;  ;", nil},
	}
	for _, c := range cases {
		got := SplitStatements(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("SplitStatements(%q) = %#v, want %#v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("SplitStatements(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestIsReadStatement(t *testing.T) {
	read := []string{"SELECT 1", "  select * from t", "SHOW TABLES", "/* c */ EXPLAIN SELECT 1", "WITH x AS (SELECT 1) SELECT * FROM x", "CALL p()"}
	write := []string{"UPDATE t SET a=1", "INSERT INTO t VALUES (1)", "DELETE FROM t", "CREATE TABLE t (id int)"}
	for _, s := range read {
		if !IsReadStatement(s) {
			t.Errorf("expected read: %q", s)
		}
	}
	for _, s := range write {
		if IsReadStatement(s) {
			t.Errorf("expected write: %q", s)
		}
	}
}

func TestBuildWhereRejectsUnknownColumn(t *testing.T) {
	cols := map[string]TableColumn{"id": {Name: "id"}}
	if _, _, err := buildWhere(cols, []RowFilter{{Column: "id; DROP TABLE x", Op: "=", Value: "1"}}); err == nil {
		t.Fatal("expected unknown-column error")
	}
	where, args, err := buildWhere(cols, []RowFilter{{Column: "id", Op: ">", Value: "5"}})
	if err != nil || where != " WHERE (`id` > ?)" || len(args) != 1 {
		t.Fatalf("unexpected where: %q %#v %v", where, args, err)
	}
}

func TestSafeCharset(t *testing.T) {
	if _, ok := safeCharset("utf8mb4"); !ok {
		t.Fatal("utf8mb4 should be valid")
	}
	if _, ok := safeCharset("utf8mb4; DROP TABLE x"); ok {
		t.Fatal("injection should be rejected")
	}
}
