package mysqlx

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type Database struct {
	Name      string `json:"name"`
	Charset   string `json:"charset"`
	Collation string `json:"collation"`
	System    bool   `json:"system"`
	Tables    int64  `json:"tables"`
	SizeBytes int64  `json:"sizeBytes"`
}

type Object struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Engine     string `json:"engine"`
	Rows       int64  `json:"rows"`
	DataLength int64  `json:"dataLength"`
	IndexLen   int64  `json:"indexLength"`
	Comment    string `json:"comment"`
	UpdatedAt  string `json:"updatedAt"`
}

type TableColumn struct {
	Position   int     `json:"position"`
	Name       string  `json:"name"`
	DataType   string  `json:"dataType"`
	ColumnType string  `json:"columnType"`
	Nullable   bool    `json:"nullable"`
	Default    *string `json:"default"`
	Key        string  `json:"key"`
	Extra      string  `json:"extra"`
	Comment    string  `json:"comment"`
	Charset    string  `json:"charset,omitempty"`
	Collation  string  `json:"collation,omitempty"`
	Numeric    bool    `json:"numeric"`
	Binary     bool    `json:"binary"`
	AutoIncr   bool    `json:"autoIncrement"`
	Generated  bool    `json:"generated"`
	PrimaryKey bool    `json:"primaryKey"`
	CharacterL int64   `json:"charLength,omitempty"`
	Fractional int64   `json:"fractional,omitempty"`
}

type Index struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Type    string   `json:"type"`
	Columns []string `json:"columns"`
}

type ForeignKey struct {
	Name      string `json:"name"`
	Column    string `json:"column"`
	RefSchema string `json:"refSchema"`
	RefTable  string `json:"refTable"`
	RefColumn string `json:"refColumn"`
	OnUpdate  string `json:"onUpdate"`
	OnDelete  string `json:"onDelete"`
}

type TableDetail struct {
	Database    string        `json:"database"`
	Name        string        `json:"name"`
	Type        string        `json:"type"`
	Engine      string        `json:"engine"`
	Comment     string        `json:"comment"`
	Rows        int64         `json:"rows"`
	DataLength  int64         `json:"dataLength"`
	IndexLength int64         `json:"indexLength"`
	Columns     []TableColumn `json:"columns"`
	Indexes     []Index       `json:"indexes"`
	ForeignKeys []ForeignKey  `json:"foreignKeys"`
	PrimaryKey  []string      `json:"primaryKey"`
	DDL         string        `json:"ddl"`
}

type Process struct {
	ID      int64  `json:"id"`
	User    string `json:"user"`
	Host    string `json:"host"`
	DB      string `json:"db"`
	Command string `json:"command"`
	Time    int64  `json:"time"`
	State   string `json:"state"`
	Info    string `json:"info"`
}

type KV struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ServerStatus struct {
	Status    []KV  `json:"status"`
	Variables []KV  `json:"variables"`
	Uptime    int64 `json:"uptime"`
	Queries   int64 `json:"queries"`
}

func (s *Session) Databases(ctx context.Context) ([]Database, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	rows, err := s.conn.QueryContext(ctx,
		`SELECT SCHEMA_NAME, IFNULL(DEFAULT_CHARACTER_SET_NAME,''), IFNULL(DEFAULT_COLLATION_NAME,'')
		   FROM information_schema.SCHEMATA ORDER BY SCHEMA_NAME`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Database
	for rows.Next() {
		var d Database
		if err := rows.Scan(&d.Name, &d.Charset, &d.Collation); err != nil {
			return nil, err
		}
		d.System = isSystemSchema(d.Name)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Table statistics are intentionally not aggregated here: on schemas with many
	// tables the information_schema scan dominates connection/database opening.
	// The table drawer loads a lightweight name/type list on demand instead.
	return out, nil
}

func (s *Session) Objects(ctx context.Context, db string) ([]Object, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	rows, err := s.conn.QueryContext(ctx,
		`SELECT TABLE_NAME, TABLE_TYPE, IFNULL(ENGINE,''), IFNULL(TABLE_COMMENT,'')
		   FROM information_schema.TABLES WHERE TABLE_SCHEMA=?
		  ORDER BY CASE TABLE_TYPE WHEN 'BASE TABLE' THEN 0 ELSE 1 END, TABLE_NAME`, db)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Object
	for rows.Next() {
		var o Object
		if err := rows.Scan(&o.Name, &o.Type, &o.Engine, &o.Comment); err != nil {
			return nil, err
		}
		if o.Type == "VIEW" || o.Type == "SYSTEM VIEW" {
			o.Type = "VIEW"
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Session) Columns(ctx context.Context, db, table string) ([]TableColumn, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	return s.columns(ctx, db, table)
}

func (s *Session) columns(ctx context.Context, db, table string) ([]TableColumn, error) {
	rows, err := s.conn.QueryContext(ctx,
		`SELECT ORDINAL_POSITION, COLUMN_NAME, COLUMN_TYPE, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT,
		        IFNULL(COLUMN_KEY,''), IFNULL(EXTRA,''), IFNULL(COLUMN_COMMENT,''),
		        IFNULL(CHARACTER_SET_NAME,''), IFNULL(COLLATION_NAME,''),
		        IFNULL(CHARACTER_MAXIMUM_LENGTH,0), IFNULL(NUMERIC_SCALE,0)
		   FROM information_schema.COLUMNS
		  WHERE TABLE_SCHEMA=? AND TABLE_NAME=? ORDER BY ORDINAL_POSITION`, db, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TableColumn
	for rows.Next() {
		var (
			c        TableColumn
			nullable string
			def      sql.NullString
		)
		if err := rows.Scan(&c.Position, &c.Name, &c.DataType, &c.ColumnType, &nullable, &def,
			&c.Key, &c.Extra, &c.Comment, &c.Charset, &c.Collation, &c.CharacterL, &c.Fractional); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		if def.Valid {
			v := def.String
			c.Default = &v
		}
		base := strings.ToUpper(c.ColumnType)
		c.Binary = isBinaryType(base)
		c.Numeric = isNumericType(base)
		c.PrimaryKey = c.Key == "PRI"
		c.AutoIncr = strings.Contains(strings.ToLower(c.Extra), "auto_increment")
		c.Generated = strings.Contains(strings.ToLower(c.Extra), "generated")
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Session) Indexes(ctx context.Context, db, table string) ([]Index, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	return s.indexes(ctx, db, table)
}

func (s *Session) indexes(ctx context.Context, db, table string) ([]Index, error) {
	rows, err := s.conn.QueryContext(ctx,
		`SELECT INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME, IFNULL(INDEX_TYPE,'')
		   FROM information_schema.STATISTICS
		  WHERE TABLE_SCHEMA=? AND TABLE_NAME=? ORDER BY INDEX_NAME, SEQ_IN_INDEX`, db, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var order []string
	byName := map[string]*Index{}
	for rows.Next() {
		var (
			name, col, typ string
			nonUnique      int64
			seq            int64
		)
		if err := rows.Scan(&name, &nonUnique, &seq, &col, &typ); err != nil {
			return nil, err
		}
		idx, ok := byName[name]
		if !ok {
			idx = &Index{Name: name, Unique: nonUnique == 0, Type: typ}
			byName[name] = idx
			order = append(order, name)
		}
		idx.Columns = append(idx.Columns, col)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Index, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name == "PRIMARY" && out[j].Name != "PRIMARY" })
	return out, nil
}

func (s *Session) ForeignKeys(ctx context.Context, db, table string) ([]ForeignKey, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	return s.foreignKeys(ctx, db, table)
}

func (s *Session) foreignKeys(ctx context.Context, db, table string) ([]ForeignKey, error) {
	rows, err := s.conn.QueryContext(ctx,
		`SELECT k.CONSTRAINT_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_SCHEMA, k.REFERENCED_TABLE_NAME,
		        k.REFERENCED_COLUMN_NAME, IFNULL(r.UPDATE_RULE,''), IFNULL(r.DELETE_RULE,'')
		   FROM information_schema.KEY_COLUMN_USAGE k
		   LEFT JOIN information_schema.REFERENTIAL_CONSTRAINTS r
		     ON r.CONSTRAINT_SCHEMA=k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME=k.CONSTRAINT_NAME AND r.TABLE_NAME=k.TABLE_NAME
		  WHERE k.TABLE_SCHEMA=? AND k.TABLE_NAME=? AND k.REFERENCED_TABLE_NAME IS NOT NULL
		  ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION`, db, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ForeignKey
	for rows.Next() {
		var f ForeignKey
		if err := rows.Scan(&f.Name, &f.Column, &f.RefSchema, &f.RefTable, &f.RefColumn, &f.OnUpdate, &f.OnDelete); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Session) PrimaryKey(ctx context.Context, db, table string) ([]string, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	return s.primaryKey(ctx, db, table)
}

func (s *Session) primaryKey(ctx context.Context, db, table string) ([]string, error) {
	rows, err := s.conn.QueryContext(ctx,
		`SELECT COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE
		  WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND CONSTRAINT_NAME='PRIMARY'
		  ORDER BY ORDINAL_POSITION`, db, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Session) DDL(ctx context.Context, db, table string) (string, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	return s.ddl(ctx, db, table)
}

func (s *Session) ddl(ctx context.Context, db, table string) (string, error) {
	var name, create string
	err := s.conn.QueryRowContext(ctx, "SHOW CREATE TABLE "+QuoteIdent(db)+"."+QuoteIdent(table)).Scan(&name, &create)
	if err != nil {
		return "", err
	}
	return create, nil
}

func (s *Session) TableExists(ctx context.Context, db, table string) (bool, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	var n int
	err := s.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?`,
		db, table).Scan(&n)
	return n > 0, err
}

func (s *Session) TableDetail(ctx context.Context, db, table string) (*TableDetail, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	var (
		d         TableDetail
		tableType string
	)
	err := s.conn.QueryRowContext(ctx,
		`SELECT TABLE_TYPE, IFNULL(ENGINE,''), IFNULL(TABLE_COMMENT,''), IFNULL(TABLE_ROWS,0),
		        IFNULL(DATA_LENGTH,0), IFNULL(INDEX_LENGTH,0)
		   FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?`, db, table).
		Scan(&tableType, &d.Engine, &d.Comment, &d.Rows, &d.DataLength, &d.IndexLength)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("table %s.%s not found", db, table)
	}
	if err != nil {
		return nil, err
	}
	d.Database, d.Name = db, table
	if tableType == "VIEW" || tableType == "SYSTEM VIEW" {
		d.Type = "VIEW"
	} else {
		d.Type = "TABLE"
	}
	if d.Columns, err = s.columns(ctx, db, table); err != nil {
		return nil, err
	}
	if d.Indexes, err = s.indexes(ctx, db, table); err != nil {
		return nil, err
	}
	if d.ForeignKeys, err = s.foreignKeys(ctx, db, table); err != nil {
		return nil, err
	}
	if d.PrimaryKey, err = s.primaryKey(ctx, db, table); err != nil {
		return nil, err
	}
	if d.DDL, err = s.ddl(ctx, db, table); err != nil {
		d.DDL = ""
	}
	return &d, nil
}

func (s *Session) Processlist(ctx context.Context) ([]Process, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	rows, err := s.conn.QueryContext(ctx, "SHOW FULL PROCESSLIST")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var out []Process
	for rows.Next() {
		values := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		p := Process{
			User:    values[1].String,
			Host:    values[2].String,
			DB:      values[3].String,
			Command: values[4].String,
			State:   values[6].String,
			Info:    values[7].String,
		}
		fmt.Sscanf(values[0].String, "%d", &p.ID)
		fmt.Sscanf(values[5].String, "%d", &p.Time)
		if p.State == "" {
			p.State = ""
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Session) Kill(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("invalid process id")
	}
	s.runMu.Lock()
	defer s.runMu.Unlock()
	_, err := s.conn.ExecContext(ctx, fmt.Sprintf("KILL %d", id))
	return err
}

func (s *Session) Status(ctx context.Context) (*ServerStatus, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	wanted := map[string]bool{
		"Uptime": true, "Threads_connected": true, "Threads_running": true, "Threads_cached": true,
		"Queries": true, "Questions": true, "Slow_queries": true, "Aborted_connects": true,
		"Bytes_received": true, "Bytes_sent": true, "Connections": true, "Max_used_connections": true,
		"Com_select": true, "Com_insert": true, "Com_update": true, "Com_delete": true,
		"Created_tmp_tables": true, "Created_tmp_disk_tables": true, "Table_locks_waited": true,
		"Innodb_buffer_pool_reads": true, "Innodb_buffer_pool_read_requests": true,
		"Innodb_row_lock_waits": true, "Open_tables": true, "Open_files": true,
	}
	out := &ServerStatus{}
	rows, err := s.conn.QueryContext(ctx, "SHOW GLOBAL STATUS")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			rows.Close()
			return nil, err
		}
		if wanted[name] {
			out.Status = append(out.Status, KV{Name: name, Value: value})
		}
		switch name {
		case "Uptime":
			fmt.Sscanf(value, "%d", &out.Uptime)
		case "Queries":
			fmt.Sscanf(value, "%d", &out.Queries)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out.Status, func(i, j int) bool { return out.Status[i].Name < out.Status[j].Name })

	varRows, err := s.conn.QueryContext(ctx,
		`SELECT 'version', @@version UNION ALL SELECT 'version_comment', @@version_comment
		 UNION ALL SELECT 'hostname', @@hostname UNION ALL SELECT 'port', @@port
		 UNION ALL SELECT 'max_connections', @@max_connections
		 UNION ALL SELECT 'innodb_buffer_pool_size', @@innodb_buffer_pool_size
		 UNION ALL SELECT 'character_set_server', @@character_set_server
		 UNION ALL SELECT 'collation_server', @@collation_server
		 UNION ALL SELECT 'sql_mode', @@sql_mode UNION ALL SELECT 'time_zone', @@time_zone
		 UNION ALL SELECT 'system_time_zone', @@system_time_zone
		 UNION ALL SELECT 'datadir', @@datadir UNION ALL SELECT 'socket', @@socket
		 UNION ALL SELECT 'lower_case_table_names', @@lower_case_table_names`)
	if err == nil {
		for varRows.Next() {
			var kv KV
			if err := varRows.Scan(&kv.Name, &kv.Value); err == nil {
				out.Variables = append(out.Variables, kv)
			}
		}
		varRows.Close()
	}
	return out, nil
}

func isSystemSchema(name string) bool {
	switch name {
	case "mysql", "information_schema", "performance_schema", "sys":
		return true
	}
	return false
}
