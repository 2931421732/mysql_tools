package ai

import (
	"strings"

	"navmini/internal/mysqlx"
)

type Kind string

const (
	KindRead    Kind = "read"
	KindModify  Kind = "modify"
	KindDelete  Kind = "delete"
	KindBlocked Kind = "blocked"
)

func modeRank(mode string) int {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "delete":
		return 3
	case "modify":
		return 2
	default:
		return 1
	}
}

func kindRank(k Kind) int {
	switch k {
	case KindRead:
		return 1
	case KindModify:
		return 2
	case KindDelete:
		return 3
	default:
		return 99
	}
}

// Allowed reports whether a statement class may be executed under a mode.
func Allowed(k Kind, mode string) bool {
	if k == KindBlocked {
		return false
	}
	return kindRank(k) <= modeRank(mode)
}

// Classify is intentionally conservative: it works on the first keyword and a
// short list of forbidden clauses. Anything it cannot recognise is blocked.
func Classify(sql string) (Kind, string) {
	stmts := mysqlx.SplitStatements(sql)
	if len(stmts) != 1 {
		return KindBlocked, "只允许执行单条 SQL 语句"
	}
	stmt := strings.TrimSpace(stmts[0])
	body := stripLeadingComments(stmt)
	upper := strings.ToUpper(body)
	word := strings.ToUpper(firstWord(body))

	switch word {
	case "SELECT", "SHOW", "DESC", "DESCRIBE", "EXPLAIN":
		if strings.Contains(upper, "INTO OUTFILE") || strings.Contains(upper, "INTO DUMPFILE") {
			return KindBlocked, "禁止导出文件（INTO OUTFILE / DUMPFILE）"
		}
		if word == "EXPLAIN" && strings.Contains(upper, "ANALYZE") {
			return KindBlocked, "禁止 EXPLAIN ANALYZE（会实际执行语句）"
		}
		if strings.Contains(upper, "FOR UPDATE") || strings.Contains(upper, "LOCK IN SHARE MODE") {
			return KindModify, "SELECT ... FOR UPDATE 会加锁，按修改权限处理"
		}
		return KindRead, ""
	case "WITH":
		if containsAnyWord(upper, "DELETE", "UPDATE", "INSERT", "REPLACE") {
			return KindBlocked, "WITH 中不允许包含写语句"
		}
		return KindRead, ""
	case "INSERT", "UPDATE", "REPLACE":
		return KindModify, ""
	case "DELETE":
		return KindDelete, ""
	case "CREATE", "ALTER", "DROP", "TRUNCATE", "RENAME":
		return KindDelete, ""
	case "LOAD", "CALL", "SET", "USE", "GRANT", "REVOKE", "LOCK", "UNLOCK", "FLUSH", "KILL", "HANDLER", "PREPARE", "EXECUTE", "DEALLOCATE":
		return KindBlocked, "该语句类型禁止由 AI 执行"
	default:
		return KindBlocked, "无法识别的语句类型"
	}
}

func containsAnyWord(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func stripLeadingComments(s string) string {
	for {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "--") || strings.HasPrefix(s, "#") {
			if idx := strings.IndexByte(s, '\n'); idx >= 0 {
				s = s[idx+1:]
				continue
			}
			return ""
		}
		if strings.HasPrefix(s, "/*") {
			idx := strings.Index(s, "*/")
			if idx < 0 {
				return ""
			}
			s = s[idx+2:]
			continue
		}
		return s
	}
}

func firstWord(s string) string {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r', '(', ';':
			return s[:i]
		}
	}
	return s
}
