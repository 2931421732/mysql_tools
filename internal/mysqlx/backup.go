package mysqlx

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type BackupRequest struct {
	Database string `json:"database"`
	Table    string `json:"table"`
	Target   string `json:"target"`
	Mode     string `json:"mode"` // full | structure | data
}

type BackupResult struct {
	Target     string `json:"target"`
	Rows       int64  `json:"rows"`
	DurationMS int64  `json:"durationMs"`
}

var backupNameRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// BackupTable copies a table inside its own schema. "full" creates the target
// with LIKE (indexes/engine preserved; foreign keys are not copied by MySQL)
// and then inserts every row.
func (s *Session) BackupTable(ctx context.Context, req BackupRequest) (*BackupResult, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	target := strings.TrimSpace(req.Target)
	if !backupNameRe.MatchString(target) {
		return nil, fmt.Errorf("invalid backup table name %q (letters, digits, underscore; max 64)", req.Target)
	}
	if target == req.Table {
		return nil, fmt.Errorf("backup target must differ from the source table")
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "full"
	}
	if mode != "full" && mode != "structure" && mode != "data" {
		return nil, fmt.Errorf("unsupported backup mode %q", req.Mode)
	}
	exists, err := s.tableExists(ctx, req.Database, req.Table)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("table %s.%s does not exist", req.Database, req.Table)
	}
	targetExists, err := s.tableExists(ctx, req.Database, target)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	created := false
	if mode == "full" || mode == "structure" {
		if targetExists {
			return nil, fmt.Errorf("backup target %s already exists", target)
		}
		sqlText := "CREATE TABLE " + QuoteIdent(req.Database) + "." + QuoteIdent(target) +
			" LIKE " + QuoteIdent(req.Database) + "." + QuoteIdent(req.Table)
		if _, err := s.conn.ExecContext(ctx, sqlText); err != nil {
			return nil, fmt.Errorf("create backup table: %w", err)
		}
		created = true
	} else if !targetExists {
		return nil, fmt.Errorf("data-only backup target %s does not exist", target)
	}

	result := &BackupResult{Target: target}
	if mode == "full" || mode == "data" {
		cols, err := s.columns(ctx, req.Database, req.Table)
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(cols))
		for _, c := range cols {
			if c.Generated {
				continue
			}
			names = append(names, QuoteIdent(c.Name))
		}
		if len(names) == 0 {
			return nil, fmt.Errorf("table has no insertable columns")
		}
		selectList := strings.Join(names, ", ")
		sqlText := "INSERT INTO " + QuoteIdent(req.Database) + "." + QuoteIdent(target) +
			" (" + selectList + ") SELECT " + selectList + " FROM " + QuoteIdent(req.Database) + "." + QuoteIdent(req.Table)
		res, err := s.conn.ExecContext(ctx, sqlText)
		if err != nil {
			if created {
				_, _ = s.conn.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+QuoteIdent(req.Database)+"."+QuoteIdent(target))
			}
			return nil, fmt.Errorf("copy rows: %w", err)
		}
		affected, _ := res.RowsAffected()
		result.Rows = affected
	}
	result.DurationMS = time.Since(start).Milliseconds()
	return result, nil
}

func (s *Session) tableExists(ctx context.Context, db, table string) (bool, error) {
	var n int
	err := s.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?`,
		db, table).Scan(&n)
	return n > 0, err
}
