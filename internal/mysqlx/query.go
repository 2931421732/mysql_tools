package mysqlx

import (
	"database/sql"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

const maxCellBytes = 1 << 20 // 1 MiB per cell in JSON responses

type Column struct {
	Name     string `json:"name"`
	DBType   string `json:"dbType"`
	Nullable bool   `json:"nullable"`
	Binary   bool   `json:"binary"`
	Numeric  bool   `json:"numeric"`
	Length   int64  `json:"length,omitempty"`
	Decimals int64  `json:"decimals,omitempty"`
}

type ResultSet struct {
	Index        int      `json:"index"`
	Statement    string   `json:"statement"`
	Columns      []Column `json:"columns"`
	Rows         [][]any  `json:"rows"`
	RowCount     int      `json:"rowCount"`
	Truncated    bool     `json:"truncated"`
	Affected     int64    `json:"affected"`
	LastInsertID int64    `json:"lastInsertId"`
	Message      string   `json:"message"`
	DurationMS   int64    `json:"durationMs"`
}

func describeColumns(names []string, types []*sql.ColumnType) []Column {
	out := make([]Column, len(names))
	for i, name := range names {
		col := Column{Name: name}
		if i < len(types) && types[i] != nil {
			ct := types[i]
			col.DBType = ct.DatabaseTypeName()
			col.Nullable, _ = ct.Nullable()
			if l, ok := ct.Length(); ok {
				col.Length = l
			}
			if _, scale, ok := ct.DecimalSize(); ok {
				col.Decimals = scale
			}
		}
		base := strings.ToUpper(col.DBType)
		col.Binary = isBinaryType(base)
		col.Numeric = isNumericType(base)
		out[i] = col
	}
	return out
}

func EncodeCell(raw sql.RawBytes, col Column) any {
	if raw == nil {
		return nil
	}
	if col.Binary {
		data := raw
		suffix := ""
		if len(data) > maxCellBytes/2 {
			data = data[:maxCellBytes/2]
			suffix = "…"
		}
		return "0x" + hex.EncodeToString(data) + suffix
	}
	if len(raw) > maxCellBytes {
		raw = raw[:maxCellBytes]
		s := string(raw)
		if !utf8.ValidString(s) {
			s = strings.ToValidUTF8(s, "\uFFFD")
		}
		return s + "…"
	}
	s := string(raw)
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "\uFFFD")
	}
	return s
}

func isBinaryType(t string) bool {
	switch t {
	case "BINARY", "VARBINARY", "TINYBLOB", "BLOB", "MEDIUMBLOB", "LONGBLOB",
		"GEOMETRY", "POINT", "LINESTRING", "POLYGON", "MULTIPOINT", "MULTILINESTRING",
		"MULTIPOLYGON", "GEOMETRYCOLLECTION":
		return true
	}
	return false
}

func isNumericType(t string) bool {
	t = strings.ReplaceAll(t, "UNSIGNED", "")
	t = strings.ReplaceAll(t, "ZEROFILL", "")
	t = strings.TrimSpace(t)
	switch t {
	case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "INTEGER", "BIGINT",
		"DECIMAL", "NUMERIC", "FLOAT", "DOUBLE", "REAL", "BIT", "YEAR":
		return true
	}
	return false
}

// SplitStatements splits a SQL script on top-level semicolons. Quotes,
// backticks, line comments and block comments are respected.
func SplitStatements(script string) []string {
	var (
		out     []string
		start   int
		i       int
		n       = len(script)
		inQuote byte
	)
	flush := func(end int) {
		part := strings.TrimSpace(script[start:end])
		if part != "" && !onlyComments(part) {
			out = append(out, part)
		}
	}
	for i < n {
		c := script[i]
		switch {
		case inQuote != 0:
			if c == '\\' && inQuote != '`' && i+1 < n {
				i += 2
				continue
			}
			if c == inQuote {
				if i+1 < n && script[i+1] == inQuote {
					i += 2
					continue
				}
				inQuote = 0
			}
			i++
		case c == '\'' || c == '"' || c == '`':
			inQuote = c
			i++
		case c == '-' && i+1 < n && script[i+1] == '-' && (i+2 >= n || script[i+2] == ' ' || script[i+2] == '\t' || script[i+2] == '\n' || script[i+2] == '\r'):
			for i < n && script[i] != '\n' {
				i++
			}
		case c == '#':
			for i < n && script[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && script[i+1] == '*':
			i += 2
			for i+1 < n && !(script[i] == '*' && script[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			}
		case c == ';':
			flush(i)
			i++
			start = i
		default:
			i++
		}
	}
	flush(n)
	return out
}

func onlyComments(s string) bool {
	for {
		s = strings.TrimSpace(s)
		switch {
		case s == "":
			return true
		case strings.HasPrefix(s, "--"), strings.HasPrefix(s, "#"):
			if idx := strings.IndexByte(s, '\n'); idx >= 0 {
				s = s[idx+1:]
				continue
			}
			return true
		case strings.HasPrefix(s, "/*"):
			idx := strings.Index(s, "*/")
			if idx < 0 {
				return true
			}
			s = s[idx+2:]
			continue
		default:
			return false
		}
	}
}

// IsReadStatement reports whether a statement is expected to produce a result
// set, so it must go through QueryContext rather than ExecContext.
func IsReadStatement(stmt string) bool {
	s := stripLeadingComments(stmt)
	if s == "" {
		return false
	}
	word := strings.ToUpper(firstWord(s))
	switch word {
	case "SELECT", "SHOW", "DESC", "DESCRIBE", "EXPLAIN", "WITH", "VALUES", "TABLE", "CALL", "HELP":
		return true
	}
	return false
}

func stripLeadingComments(s string) string {
	for {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "--"), strings.HasPrefix(s, "#"):
			if idx := strings.IndexByte(s, '\n'); idx >= 0 {
				s = s[idx+1:]
				continue
			}
			return ""
		case strings.HasPrefix(s, "/*"):
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
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '(' || c == ';' {
			return s[:i]
		}
	}
	return s
}

// scanRows drains rows into JSON-friendly cells, stopping after maxRows.
func scanRows(rows *sql.Rows, maxRows int) ([]Column, [][]any, bool, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, false, err
	}
	types, _ := rows.ColumnTypes()
	columns := describeColumns(cols, types)

	values := make([]sql.RawBytes, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}

	data := make([][]any, 0, 64)
	truncated := false
	for rows.Next() {
		if len(data) >= maxRows {
			truncated = true
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, false, err
		}
		row := make([]any, len(cols))
		for i := range values {
			row[i] = EncodeCell(values[i], columns[i])
		}
		data = append(data, row)
	}
	return columns, data, truncated, rows.Err()
}
