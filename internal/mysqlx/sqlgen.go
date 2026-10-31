package mysqlx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type RowSQLRequest struct {
	Database string             `json:"database"`
	Table    string             `json:"table"`
	Kind     string             `json:"kind"` // insert | update | delete
	Values   map[string]*string `json:"values"`
	PK       map[string]*string `json:"pk"`
}

// RowSQL renders one row as a standalone INSERT / UPDATE / DELETE statement.
// Values are converted to literals with the column type in mind so the result
// can be executed as-is.
func (s *Session) RowSQL(ctx context.Context, req RowSQLRequest) (string, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	cols, err := s.columns(ctx, req.Database, req.Table)
	if err != nil {
		return "", err
	}
	pk, err := s.primaryKey(ctx, req.Database, req.Table)
	if err != nil {
		return "", err
	}
	byName := make(map[string]TableColumn, len(cols))
	for _, c := range cols {
		byName[c.Name] = c
	}
	table := QuoteIdent(req.Database) + "." + QuoteIdent(req.Table)
	kind := strings.ToLower(strings.TrimSpace(req.Kind))

	switch kind {
	case "insert":
		names := make([]string, 0, len(cols))
		values := make([]string, 0, len(cols))
		for _, c := range cols {
			v, ok := req.Values[c.Name]
			if !ok {
				continue
			}
			names = append(names, QuoteIdent(c.Name))
			values = append(values, sqlLiteral(c, v))
		}
		if len(names) == 0 {
			return "", fmt.Errorf("no values to insert")
		}
		return "INSERT INTO " + table + " (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(values, ", ") + ");", nil
	case "update", "delete":
		if len(pk) == 0 {
			return "", fmt.Errorf("table %s has no primary key; cannot build a safe WHERE clause", req.Table)
		}
		if err := checkPK(pk, req.PK); err != nil {
			return "", err
		}
		where := make([]string, 0, len(pk))
		for _, name := range pk {
			col, ok := byName[name]
			if !ok {
				return "", fmt.Errorf("unknown primary key column %q", name)
			}
			where = append(where, QuoteIdent(name)+" <=> "+sqlLiteral(col, req.PK[name]))
		}
		whereSQL := strings.Join(where, " AND ")
		if kind == "delete" {
			return "DELETE FROM " + table + " WHERE " + whereSQL + " LIMIT 1;", nil
		}
		sets := make([]string, 0, len(req.Values))
		for _, c := range cols {
			if containsString(pk, c.Name) {
				continue
			}
			v, ok := req.Values[c.Name]
			if !ok {
				continue
			}
			sets = append(sets, QuoteIdent(c.Name)+" = "+sqlLiteral(c, v))
		}
		if len(sets) == 0 {
			return "", fmt.Errorf("no columns to update")
		}
		return "UPDATE " + table + " SET " + strings.Join(sets, ", ") + " WHERE " + whereSQL + " LIMIT 1;", nil
	default:
		return "", fmt.Errorf("unsupported SQL kind %q", req.Kind)
	}
}

func sqlLiteral(col TableColumn, value *string) string {
	if value == nil {
		return "NULL"
	}
	raw := *value
	if col.Numeric && !isBinaryType(col.ColumnType) {
		trimmed := strings.TrimSpace(raw)
		if trimmed != "" {
			if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
				return trimmed
			}
		}
	}
	if col.Binary {
		if len(raw) > 2 && (strings.HasPrefix(raw, "0x") || strings.HasPrefix(raw, "0X")) {
			if _, err := strconv.ParseUint(raw[2:], 16, 64); err == nil || isHex(raw[2:]) {
				return raw
			}
		}
		return "X'" + strings.ToLower(hexOrEmpty(raw)) + "'"
	}
	return quoteSQLString(raw)
}

func quoteSQLString(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 0:
			b.WriteString(`\0`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '"':
			b.WriteString(`\"`)
		case 0x1a:
			b.WriteString(`\Z`)
		default:
			b.WriteByte(s[i])
		}
	}
	b.WriteByte('\'')
	return b.String()
}

func isHex(s string) bool {
	if s == "" || len(s)%2 != 0 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func hexOrEmpty(s string) string {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return s[2:]
	}
	return ""
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
