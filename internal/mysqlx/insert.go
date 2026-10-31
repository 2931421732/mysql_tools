package mysqlx

import (
	"context"
	"fmt"
	"strings"
)

// InsertRows inserts rows in bounded multi-row batches inside one transaction.
// Columns are validated against the live schema; generated columns are rejected.
func (s *Session) InsertRows(ctx context.Context, db, table string, columns []string, rows [][]any, batch int) (int64, error) {
	if len(columns) == 0 {
		return 0, fmt.Errorf("no columns selected")
	}
	if len(rows) == 0 {
		return 0, nil
	}
	if batch <= 0 || batch > 1000 {
		batch = 250
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()

	cols, err := s.columns(ctx, db, table)
	if err != nil {
		return 0, err
	}
	byName := columnMap(cols)
	for _, name := range columns {
		c, ok := byName[name]
		if !ok {
			return 0, fmt.Errorf("unknown column %q", name)
		}
		if c.Generated {
			return 0, fmt.Errorf("column %q is generated and cannot be imported", name)
		}
	}
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	prefix := "INSERT INTO " + QuoteIdent(db) + "." + QuoteIdent(table) + " (" + quoteList(columns) + ") VALUES "
	var inserted int64
	for start := 0; start < len(rows); start += batch {
		end := start + batch
		if end > len(rows) {
			end = len(rows)
		}
		var b strings.Builder
		b.WriteString(prefix)
		args := make([]any, 0, (end-start)*len(columns))
		for i := start; i < end; i++ {
			if len(rows[i]) != len(columns) {
				return inserted, fmt.Errorf("row %d has %d values, expected %d", i+1, len(rows[i]), len(columns))
			}
			if i > start {
				b.WriteByte(',')
			}
			b.WriteByte('(')
			for j := range columns {
				if j > 0 {
					b.WriteByte(',')
				}
				b.WriteByte('?')
				args = append(args, rows[i][j])
			}
			b.WriteByte(')')
		}
		res, err := tx.ExecContext(ctx, b.String(), args...)
		if err != nil {
			return inserted, err
		}
		n, _ := res.RowsAffected()
		inserted += n
	}
	if err := tx.Commit(); err != nil {
		return inserted, err
	}
	return inserted, nil
}

func quoteList(names []string) string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = QuoteIdent(n)
	}
	return strings.Join(out, ",")
}
