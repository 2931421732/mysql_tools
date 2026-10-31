package mysqlx

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type SchemaInfo struct {
	Text      string `json:"text"`
	Tables    int    `json:"tables"`
	Truncated bool   `json:"truncated"`
}

type schemaTable struct {
	db, name, typ, engine, comment string
	rows                           int64
	cols                           []schemaColumn
	indexes                        []schemaIndex
}

type schemaColumn struct {
	name, typ, nullable, key, extra, comment string
}

type schemaIndex struct {
	name    string
	unique  bool
	columns []string
}

const (
	maxSchemaTables = 80
	maxSchemaChars  = 24000
)

// SchemaSnapshot builds a compact, index-aware description of selected databases.
func (s *Session) SchemaSnapshot(ctx context.Context, databases []string) (*SchemaInfo, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	dbs := make([]string, 0, len(databases))
	seen := map[string]bool{}
	for _, d := range databases {
		d = strings.TrimSpace(d)
		if d != "" && !seen[d] {
			seen[d] = true
			dbs = append(dbs, d)
		}
	}
	if len(dbs) == 0 {
		s.mu.Lock()
		cur := s.currentDB
		s.mu.Unlock()
		if cur == "" {
			return nil, fmt.Errorf("请先选择至少一个数据库")
		}
		dbs = []string{cur}
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(dbs)), ",")
	args := make([]any, len(dbs))
	for i, d := range dbs {
		args[i] = d
	}

	tables := make([]*schemaTable, 0, 32)
	byKey := map[string]*schemaTable{}
	trows, err := s.conn.QueryContext(ctx,
		`SELECT TABLE_SCHEMA,TABLE_NAME,TABLE_TYPE,IFNULL(ENGINE,''),IFNULL(TABLE_ROWS,0),IFNULL(TABLE_COMMENT,'')
		   FROM information_schema.TABLES
		  WHERE TABLE_SCHEMA IN (`+placeholders+`) AND TABLE_TYPE IN ('BASE TABLE','VIEW')
		  ORDER BY TABLE_SCHEMA,TABLE_NAME`, args...)
	if err != nil {
		return nil, err
	}
	for trows.Next() {
		t := &schemaTable{}
		if err := trows.Scan(&t.db, &t.name, &t.typ, &t.engine, &t.rows, &t.comment); err != nil {
			trows.Close()
			return nil, err
		}
		if len(tables) >= maxSchemaTables {
			break
		}
		tables = append(tables, t)
		byKey[t.db+"\x00"+t.name] = t
	}
	trows.Close()
	if err := trows.Err(); err != nil {
		return nil, err
	}

	crows, err := s.conn.QueryContext(ctx,
		`SELECT TABLE_SCHEMA,TABLE_NAME,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,IFNULL(COLUMN_KEY,''),IFNULL(EXTRA,''),IFNULL(COLUMN_COMMENT,'')
		   FROM information_schema.COLUMNS
		  WHERE TABLE_SCHEMA IN (`+placeholders+`)
		  ORDER BY TABLE_SCHEMA,TABLE_NAME,ORDINAL_POSITION`, args...)
	if err != nil {
		return nil, err
	}
	for crows.Next() {
		var db, table string
		var c schemaColumn
		if err := crows.Scan(&db, &table, &c.name, &c.typ, &c.nullable, &c.key, &c.extra, &c.comment); err != nil {
			crows.Close()
			return nil, err
		}
		if t := byKey[db+"\x00"+table]; t != nil {
			t.cols = append(t.cols, c)
		}
	}
	crows.Close()
	if err := crows.Err(); err != nil {
		return nil, err
	}

	irows, err := s.conn.QueryContext(ctx,
		`SELECT TABLE_SCHEMA,TABLE_NAME,INDEX_NAME,NON_UNIQUE,SEQ_IN_INDEX,COLUMN_NAME
		   FROM information_schema.STATISTICS
		  WHERE TABLE_SCHEMA IN (`+placeholders+`)
		  ORDER BY TABLE_SCHEMA,TABLE_NAME,INDEX_NAME,SEQ_IN_INDEX`, args...)
	if err != nil {
		return nil, err
	}
	idxMap := map[string]*schemaIndex{}
	idxOrder := map[string][]string{}
	for irows.Next() {
		var db, table, name, column string
		var nonUnique, seq int64
		if err := irows.Scan(&db, &table, &name, &nonUnique, &seq, &column); err != nil {
			irows.Close()
			return nil, err
		}
		if t := byKey[db+"\x00"+table]; t != nil {
			key := db + "\x00" + table + "\x00" + name
			idx := idxMap[key]
			if idx == nil {
				idx = &schemaIndex{name: name, unique: nonUnique == 0}
				idxMap[key] = idx
				idxOrder[db+"\x00"+table] = append(idxOrder[db+"\x00"+table], key)
			}
			idx.columns = append(idx.columns, column)
		}
	}
	irows.Close()
	if err := irows.Err(); err != nil {
		return nil, err
	}
	for key, t := range byKey {
		for _, ik := range idxOrder[key] {
			if idx := idxMap[ik]; idx != nil {
				t.indexes = append(t.indexes, *idx)
			}
		}
		sort.SliceStable(t.indexes, func(i, j int) bool {
			if t.indexes[i].name == "PRIMARY" {
				return true
			}
			if t.indexes[j].name == "PRIMARY" {
				return false
			}
			return t.indexes[i].name < t.indexes[j].name
		})
	}

	var b strings.Builder
	b.WriteString("-- 只包含以下数据库的 schema 摘要；rows 为 InnoDB 估算值。\n")
	truncated := false
	lastDB := ""
	for _, t := range tables {
		if t.db != lastDB {
			lastDB = t.db
			b.WriteString("\n[db] " + t.db + "\n")
		}
		typ := strings.ToLower(t.typ)
		if typ == "base table" {
			typ = "TABLE"
		} else {
			typ = "VIEW"
		}
		comment := strings.TrimSpace(t.comment)
		if len(comment) > 80 {
			comment = comment[:80]
		}
		b.WriteString(fmt.Sprintf("  %s %s (~%d rows, %s)", typ, t.name, t.rows, t.engine))
		if comment != "" {
			b.WriteString("  -- " + comment)
		}
		b.WriteString("\n")
		for _, c := range t.cols {
			b.WriteString("    " + c.name + " " + c.typ)
			if c.nullable == "NO" {
				b.WriteString(" NOT NULL")
			}
			if c.key != "" {
				b.WriteString(" KEY=" + c.key)
			}
			if c.extra != "" {
				b.WriteString(" " + c.extra)
			}
			if c.comment != "" {
				cm := c.comment
				if len(cm) > 50 {
					cm = cm[:50]
				}
				b.WriteString("  -- " + cm)
			}
			b.WriteString("\n")
		}
		if len(t.indexes) > 0 {
			parts := make([]string, 0, len(t.indexes))
			for _, idx := range t.indexes {
				s := idx.name + "(" + strings.Join(idx.columns, ",") + ")"
				if idx.unique {
					s += " UNIQUE"
				}
				parts = append(parts, s)
			}
			b.WriteString("    INDEX " + strings.Join(parts, "; ") + "\n")
		}
		if b.Len() > maxSchemaChars {
			truncated = true
			b.WriteString("\n-- schema 已截断（超过上下文预算）\n")
			break
		}
	}
	return &SchemaInfo{Text: b.String(), Tables: len(tables), Truncated: truncated}, nil
}
