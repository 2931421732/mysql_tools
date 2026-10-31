package ai

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		sql  string
		kind Kind
	}{
		{"SELECT * FROM t", KindRead},
		{"  /* hi */ SHOW TABLES", KindRead},
		{"EXPLAIN SELECT * FROM t", KindRead},
		{"WITH x AS (SELECT 1) SELECT * FROM x", KindRead},
		{"INSERT INTO t VALUES (1)", KindModify},
		{"UPDATE t SET a=1", KindModify},
		{"SELECT * FROM t FOR UPDATE", KindModify},
		{"DELETE FROM t", KindDelete},
		{"DROP TABLE t", KindDelete},
		{"WITH x AS (SELECT 1) DELETE FROM t", KindBlocked},
		{"LOAD DATA INFILE 'x' INTO TABLE t", KindBlocked},
		{"SELECT * INTO OUTFILE '/tmp/x' FROM t", KindBlocked},
		{"SELECT 1; SELECT 2", KindBlocked},
		{"CALL p()", KindBlocked},
		{"SET GLOBAL max_connections=1", KindBlocked},
		{"EXPLAIN ANALYZE SELECT * FROM t", KindBlocked},
	}
	for _, c := range cases {
		kind, reason := Classify(c.sql)
		if kind != c.kind {
			t.Errorf("Classify(%q) = %s (%s), want %s", c.sql, kind, reason, c.kind)
		}
	}
}

func TestAllowed(t *testing.T) {
	if !Allowed(KindRead, "readonly") || Allowed(KindModify, "readonly") || Allowed(KindDelete, "modify") {
		t.Fatal("readonly/modify permission matrix broken")
	}
	if !Allowed(KindModify, "modify") || !Allowed(KindDelete, "delete") {
		t.Fatal("higher permission modes should allow lower kinds")
	}
	if Allowed(KindBlocked, "delete") {
		t.Fatal("blocked must never be allowed")
	}
}

func TestParseReply(t *testing.T) {
	r := ParseReply("```json\n{\"reply\":\"ok\",\"sql\":\"SELECT 1\",\"risk\":\"read\"}\n```")
	if r.Reply != "ok" || r.SQL != "SELECT 1" {
		t.Fatalf("unexpected reply: %#v", r)
	}
	fallback := ParseReply("plain text")
	if fallback.Reply != "plain text" || fallback.SQL != "" {
		t.Fatalf("unexpected fallback: %#v", fallback)
	}
}

func TestExplainSummary(t *testing.T) {
	if got := ExplainSummary([]PlanRow{{Table: "t", Type: "ALL", Key: "", Rows: "10"}}); len(got) != 0 {
		t.Fatalf("small scan should not warn: %#v", got)
	}
	if got := ExplainSummary([]PlanRow{{Table: "t", Type: "ALL", Key: "", Rows: "5000"}}); len(got) != 1 {
		t.Fatalf("large scan should warn: %#v", got)
	}
}
