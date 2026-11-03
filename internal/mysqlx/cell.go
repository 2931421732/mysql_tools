package mysqlx

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type CellValue struct {
	Value string `json:"value"`
	Null  bool   `json:"null"`
	Bytes int    `json:"bytes"`
}

// Cell reads one full cell by primary key, without the grid's display cap.
func (s *Session) Cell(ctx context.Context, database, table, column string, pk map[string]*string) (*CellValue, error) {
	if database == "" || table == "" || column == "" || len(pk) == 0 {
		return nil, fmt.Errorf("database, table, column and primary key are required")
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()
	cols, err := s.columns(ctx, database, table)
	if err != nil {
		return nil, err
	}
	valid := columnMap(cols)
	if _, ok := valid[column]; !ok {
		return nil, fmt.Errorf("unknown column %q", column)
	}
	where := make([]string, 0, len(pk))
	args := make([]any, 0, len(pk))
	for name, value := range pk {
		if _, ok := valid[name]; !ok {
			return nil, fmt.Errorf("unknown key column %q", name)
		}
		if value == nil {
			where = append(where, QuoteIdent(name)+" IS NULL")
		} else {
			where = append(where, QuoteIdent(name)+" = ?")
			args = append(args, *value)
		}
	}
	q := "SELECT " + QuoteIdent(column) + " FROM " + QuoteIdent(database) + "." + QuoteIdent(table) +
		" WHERE " + strings.Join(where, " AND ") + " LIMIT 1"
	var raw []byte
	if err := s.conn.QueryRowContext(ctx, q, args...).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("row not found")
		}
		return nil, err
	}
	if raw == nil {
		return &CellValue{Null: true}, nil
	}
	value := string(raw)
	return &CellValue{Value: value, Bytes: len(value)}, nil
}
