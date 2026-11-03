package mysqlx

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
)

type DatabaseExportRequest struct {
	Database   string   `json:"database"`
	Mode       string   `json:"mode"` // structure | full | custom
	DataTables []string `json:"dataTables"`
}

// ExportDatabaseSQL writes a portable SQL dump. In custom mode every table's
// structure is written, but only DataTables get INSERTs.
func (s *Session) ExportDatabaseSQL(ctx context.Context, req DatabaseExportRequest, w io.Writer) (int64, error) {
	if req.Database == "" {
		return 0, fmt.Errorf("database is required")
	}
	mode := strings.ToLower(req.Mode)
	if mode != "structure" && mode != "full" && mode != "custom" {
		mode = "full"
	}
	dataSet := map[string]bool{}
	for _, t := range req.DataTables {
		dataSet[t] = true
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()

	rows, err := s.conn.QueryContext(ctx,
		`SELECT TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES
		  WHERE TABLE_SCHEMA=? ORDER BY TABLE_TYPE, TABLE_NAME`, req.Database)
	if err != nil {
		return 0, err
	}
	type obj struct{ name, kind string }
	var objects []obj
	for rows.Next() {
		var o obj
		if err := rows.Scan(&o.name, &o.kind); err != nil {
			rows.Close()
			return 0, err
		}
		objects = append(objects, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	var count int64
	write := func(s string) error {
		_, err := io.WriteString(w, s)
		return err
	}
	if err := write("SET NAMES utf8mb4;\nSET FOREIGN_KEY_CHECKS=0;\n\n"); err != nil {
		return count, err
	}
	for _, o := range objects {
		isView := strings.Contains(o.kind, "VIEW")
		ddl, err := s.showCreate(ctx, req.Database, o.name, isView)
		if err != nil {
			return count, err
		}
		if isView {
			if err := write("DROP VIEW IF EXISTS " + QuoteIdent(req.Database) + "." + QuoteIdent(o.name) + ";\n"); err != nil {
				return count, err
			}
		} else {
			if err := write("DROP TABLE IF EXISTS " + QuoteIdent(req.Database) + "." + QuoteIdent(o.name) + ";\n"); err != nil {
				return count, err
			}
		}
		if err := write(ddl + ";\n\n"); err != nil {
			return count, err
		}
		if isView || mode == "structure" {
			continue
		}
		if mode == "custom" && !dataSet[o.name] {
			continue
		}
		n, err := s.dumpTableData(ctx, req.Database, o.name, w)
		if err != nil {
			return count, err
		}
		count += n
	}
	if err := write("SET FOREIGN_KEY_CHECKS=1;\n"); err != nil {
		return count, err
	}
	return count, nil
}

func (s *Session) showCreate(ctx context.Context, db, table string, view bool) (string, error) {
	cmd := "SHOW CREATE TABLE "
	if view {
		cmd = "SHOW CREATE VIEW "
	}
	rows, err := s.conn.QueryContext(ctx, cmd+QuoteIdent(db)+"."+QuoteIdent(table))
	if err != nil {
		return "", err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	values := make([]sql.RawBytes, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	if !rows.Next() {
		return "", fmt.Errorf("cannot read DDL for %s", table)
	}
	if err := rows.Scan(ptrs...); err != nil {
		return "", err
	}
	if len(values) < 2 {
		return "", fmt.Errorf("unexpected DDL result for %s", table)
	}
	return string(values[1]), nil
}

func (s *Session) dumpTableData(ctx context.Context, db, table string, w io.Writer) (int64, error) {
	cols, err := s.columns(ctx, db, table)
	if err != nil {
		return 0, err
	}
	rows, err := s.conn.QueryContext(ctx, "SELECT * FROM "+QuoteIdent(db)+"."+QuoteIdent(table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = QuoteIdent(c.Name)
	}
	prefix := "INSERT INTO " + QuoteIdent(db) + "." + QuoteIdent(table) + " (" + strings.Join(names, ",") + ") VALUES\n"
	values := make([]sql.RawBytes, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	var count int64
	var batch strings.Builder
	batchN := 0
	flush := func() error {
		if batchN == 0 {
			return nil
		}
		batch.WriteString(";\n")
		if _, err := io.WriteString(w, prefix+batch.String()); err != nil {
			return err
		}
		batch.Reset()
		batchN = 0
		return nil
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return count, err
		}
		if batchN > 0 {
			batch.WriteString(",\n")
		}
		batch.WriteByte('(')
		for i := range values {
			if i > 0 {
				batch.WriteString(",")
			}
			val := values[i]
			if val == nil {
				batch.WriteString("NULL")
			} else {
				v := string(val)
				batch.WriteString(sqlLiteral(cols[i], &v))
			}
		}
		batch.WriteByte(')')
		batchN++
		count++
		if batchN >= 100 {
			if err := flush(); err != nil {
				return count, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return count, err
	}
	if err := flush(); err != nil {
		return count, err
	}
	return count, nil
}
