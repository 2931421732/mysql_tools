package mysqlx

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type RowFilter struct {
	Column    string `json:"column"`
	Op        string `json:"op"`
	Value     string `json:"value"`
	Value2    string `json:"value2,omitempty"`
	Connector string `json:"connector,omitempty"` // and (default) | or
}

type RowOrder struct {
	Column string `json:"column"`
	Desc   bool   `json:"desc"`
}

type RowsRequest struct {
	Database    string      `json:"database"`
	Table       string      `json:"table"`
	Limit       int         `json:"limit"`
	Offset      int         `json:"offset"`
	Order       []RowOrder  `json:"orderBy"`
	Filters     []RowFilter `json:"filters"`
	Count       bool        `json:"count"`
	DefaultSort bool        `json:"defaultSort"`
}

type RowsResult struct {
	Columns    []Column `json:"columns"`
	Rows       [][]any  `json:"rows"`
	PrimaryKey []string `json:"primaryKey"`
	Total      *int64   `json:"total"`
	Truncated  bool     `json:"truncated"`
	DurationMS int64    `json:"durationMs"`
	Type       string   `json:"type"`
	Engine     string   `json:"engine"`
	Comment    string   `json:"comment"`
}

type UpdateRowRequest struct {
	Database string             `json:"database"`
	Table    string             `json:"table"`
	PK       map[string]*string `json:"pk"`
	Changes  map[string]*string `json:"changes"`
}

type InsertRowRequest struct {
	Database string             `json:"database"`
	Table    string             `json:"table"`
	Values   map[string]*string `json:"values"`
}

type DeleteRowRequest struct {
	Database string             `json:"database"`
	Table    string             `json:"table"`
	PK       map[string]*string `json:"pk"`
}

func (s *Session) Rows(ctx context.Context, req RowsRequest) (*RowsResult, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	if req.Limit <= 0 {
		req.Limit = 200
	}
	if req.Limit > 5000 {
		req.Limit = 5000
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	start := time.Now()

	cols, err := s.columns(ctx, req.Database, req.Table)
	if err != nil {
		return nil, err
	}
	colMap := columnMap(cols)
	where, args, err := buildWhere(colMap, req.Filters)
	if err != nil {
		return nil, err
	}
	pk, err := s.primaryKey(ctx, req.Database, req.Table)
	if err != nil {
		return nil, err
	}
	orderReq := req.Order
	if len(orderReq) == 0 && req.DefaultSort && len(pk) > 0 {
		for _, name := range pk {
			orderReq = append(orderReq, RowOrder{Column: name, Desc: true})
		}
	}
	order, err := buildOrder(colMap, pk, orderReq)
	if err != nil {
		return nil, err
	}

	base := "SELECT * FROM " + QuoteIdent(req.Database) + "." + QuoteIdent(req.Table) + where
	sqlText := base + order + " LIMIT ? OFFSET ?"
	qargs := append(append([]any{}, args...), req.Limit+1, req.Offset)

	rows, err := s.conn.QueryContext(ctx, sqlText, qargs...)
	if err != nil {
		return nil, err
	}
	resultCols, data, truncated, err := scanRows(rows, req.Limit+1)
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(data) > req.Limit {
		data = data[:req.Limit]
		truncated = true
	}

	out := &RowsResult{
		Columns:    resultCols,
		Rows:       data,
		PrimaryKey: pk,
		Truncated:  truncated,
		DurationMS: time.Since(start).Milliseconds(),
	}
	_ = s.conn.QueryRowContext(ctx,
		`SELECT TABLE_TYPE, IFNULL(ENGINE,''), IFNULL(TABLE_COMMENT,'')
		   FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?`,
		req.Database, req.Table).Scan(&out.Type, &out.Engine, &out.Comment)
	if out.Type == "VIEW" || out.Type == "SYSTEM VIEW" {
		out.Type = "VIEW"
	} else if out.Type == "BASE TABLE" {
		out.Type = "TABLE"
	}

	if req.Count {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		var total int64
		err := s.conn.QueryRowContext(cctx, "SELECT COUNT(*) FROM "+QuoteIdent(req.Database)+"."+QuoteIdent(req.Table)+where, args...).Scan(&total)
		cancel()
		if err == nil {
			out.Total = &total
		}
	}
	return out, nil
}

func (s *Session) UpdateRow(ctx context.Context, req UpdateRowRequest) (int64, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	if len(req.Changes) == 0 {
		return 0, fmt.Errorf("no changed values")
	}
	cols, err := s.columns(ctx, req.Database, req.Table)
	if err != nil {
		return 0, err
	}
	colMap := columnMap(cols)
	pk, err := s.primaryKey(ctx, req.Database, req.Table)
	if err != nil {
		return 0, err
	}
	if len(pk) == 0 {
		return 0, fmt.Errorf("table %s has no primary key; editing is read-only", req.Table)
	}
	if err := checkPK(pk, req.PK); err != nil {
		return 0, err
	}

	sets := make([]string, 0, len(req.Changes))
	args := make([]any, 0, len(req.Changes)+len(pk))
	for name, val := range req.Changes {
		if _, ok := colMap[name]; !ok {
			return 0, fmt.Errorf("unknown column %q", name)
		}
		sets = append(sets, QuoteIdent(name)+"=?")
		args = append(args, nullableString(val))
	}
	wheres := make([]string, 0, len(pk))
	for _, name := range pk {
		wheres = append(wheres, QuoteIdent(name)+" <=> ?")
		args = append(args, nullableString(req.PK[name]))
	}

	sqlText := "UPDATE " + QuoteIdent(req.Database) + "." + QuoteIdent(req.Table) +
		" SET " + strings.Join(sets, ", ") + " WHERE " + strings.Join(wheres, " AND ") + " LIMIT 1"
	res, err := s.conn.ExecContext(ctx, sqlText, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Session) InsertRow(ctx context.Context, req InsertRowRequest) (int64, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	cols, err := s.columns(ctx, req.Database, req.Table)
	if err != nil {
		return 0, err
	}
	colMap := columnMap(cols)
	names := make([]string, 0, len(req.Values))
	args := make([]any, 0, len(req.Values))
	for name, val := range req.Values {
		if _, ok := colMap[name]; !ok {
			return 0, fmt.Errorf("unknown column %q", name)
		}
		names = append(names, QuoteIdent(name))
		args = append(args, nullableString(val))
	}
	var sqlText string
	if len(names) == 0 {
		sqlText = "INSERT INTO " + QuoteIdent(req.Database) + "." + QuoteIdent(req.Table) + " () VALUES ()"
	} else {
		marks := make([]string, len(names))
		for i := range marks {
			marks[i] = "?"
		}
		sqlText = "INSERT INTO " + QuoteIdent(req.Database) + "." + QuoteIdent(req.Table) +
			" (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(marks, ", ") + ")"
	}
	res, err := s.conn.ExecContext(ctx, sqlText, args...)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *Session) DeleteRow(ctx context.Context, req DeleteRowRequest) (int64, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	if _, err := s.columns(ctx, req.Database, req.Table); err != nil {
		return 0, err
	}
	pk, err := s.primaryKey(ctx, req.Database, req.Table)
	if err != nil {
		return 0, err
	}
	if len(pk) == 0 {
		return 0, fmt.Errorf("table %s has no primary key; deleting is disabled", req.Table)
	}
	if err := checkPK(pk, req.PK); err != nil {
		return 0, err
	}
	wheres := make([]string, 0, len(pk))
	args := make([]any, 0, len(pk))
	for _, name := range pk {
		wheres = append(wheres, QuoteIdent(name)+" <=> ?")
		args = append(args, nullableString(req.PK[name]))
	}
	sqlText := "DELETE FROM " + QuoteIdent(req.Database) + "." + QuoteIdent(req.Table) +
		" WHERE " + strings.Join(wheres, " AND ") + " LIMIT 1"
	res, err := s.conn.ExecContext(ctx, sqlText, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ExportTable streams the whole table (with filters/order) as CSV.
func (s *Session) ExportTable(ctx context.Context, req RowsRequest, w io.Writer) (int64, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	cols, err := s.columns(ctx, req.Database, req.Table)
	if err != nil {
		return 0, err
	}
	colMap := columnMap(cols)
	where, args, err := buildWhere(colMap, req.Filters)
	if err != nil {
		return 0, err
	}
	pk, _ := s.primaryKey(ctx, req.Database, req.Table)
	order, err := buildOrder(colMap, pk, req.Order)
	if err != nil {
		return 0, err
	}
	sqlText := "SELECT * FROM " + QuoteIdent(req.Database) + "." + QuoteIdent(req.Table) + where + order + " LIMIT 500000"
	rows, err := s.conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	header, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return 0, err
	}
	types, _ := rows.ColumnTypes()
	colDefs := describeColumns(header, types)
	values := make([]sql.RawBytes, len(header))
	ptrs := make([]any, len(header))
	for i := range values {
		ptrs[i] = &values[i]
	}
	var count int64
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return count, err
		}
		record := make([]string, len(header))
		for i := range values {
			cell := EncodeCell(values[i], colDefs[i])
			if cell == nil {
				record[i] = ""
				continue
			}
			record[i] = fmt.Sprint(cell)
		}
		if err := cw.Write(record); err != nil {
			return count, err
		}
		count++
	}
	cw.Flush()
	return count, cw.Error()
}

func columnMap(cols []TableColumn) map[string]TableColumn {
	out := make(map[string]TableColumn, len(cols))
	for _, c := range cols {
		out[c.Name] = c
	}
	return out
}

func buildWhere(cols map[string]TableColumn, filters []RowFilter) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	args := make([]any, 0, len(filters))
	expr := ""
	for i, f := range filters {
		if _, ok := cols[f.Column]; !ok {
			return "", nil, fmt.Errorf("unknown filter column %q", f.Column)
		}
		op := strings.ToLower(strings.TrimSpace(f.Op))
		ident := QuoteIdent(f.Column)
		var cond string
		switch op {
		case "=", "!=", ">", ">=", "<", "<=":
			cond = ident + " " + op + " ?"
			args = append(args, f.Value)
		case "like", "not like":
			cond = ident + " " + strings.ToUpper(op) + " ?"
			args = append(args, f.Value)
		case "contains":
			cond = ident + " LIKE ?"
			args = append(args, "%"+f.Value+"%")
		case "starts":
			cond = ident + " LIKE ?"
			args = append(args, f.Value+"%")
		case "ends":
			cond = ident + " LIKE ?"
			args = append(args, "%"+f.Value)
		case "in", "not in":
			values := splitFilterValues(f.Value)
			if len(values) == 0 {
				return "", nil, fmt.Errorf("%s requires at least one value", op)
			}
			marks := make([]string, len(values))
			for j, v := range values {
				marks[j] = "?"
				args = append(args, v)
			}
			cond = ident + " " + strings.ToUpper(op) + " (" + strings.Join(marks, ", ") + ")"
		case "between", "not between":
			if strings.TrimSpace(f.Value) == "" || strings.TrimSpace(f.Value2) == "" {
				return "", nil, fmt.Errorf("%s requires two values", op)
			}
			cond = ident + " " + strings.ToUpper(op) + " ? AND ?"
			args = append(args, f.Value, f.Value2)
		case "is null":
			cond = ident + " IS NULL"
		case "is not null":
			cond = ident + " IS NOT NULL"
		default:
			return "", nil, fmt.Errorf("unsupported filter operator %q", f.Op)
		}
		if i == 0 {
			expr = "(" + cond + ")"
			continue
		}
		connector := strings.ToLower(strings.TrimSpace(f.Connector))
		if connector != "or" {
			connector = "and"
		}
		expr = "(" + expr + " " + strings.ToUpper(connector) + " (" + cond + "))"
	}
	return " WHERE " + expr, args, nil
}

func splitFilterValues(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func buildOrder(cols map[string]TableColumn, pk []string, order []RowOrder) (string, error) {
	parts := make([]string, 0, len(order)+len(pk))
	for _, o := range order {
		if _, ok := cols[o.Column]; !ok {
			return "", fmt.Errorf("unknown order column %q", o.Column)
		}
		dir := " ASC"
		if o.Desc {
			dir = " DESC"
		}
		parts = append(parts, QuoteIdent(o.Column)+dir)
	}
	if len(parts) == 0 {
		for _, name := range pk {
			parts = append(parts, QuoteIdent(name)+" ASC")
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " ORDER BY " + strings.Join(parts, ", "), nil
}

func checkPK(pk []string, values map[string]*string) error {
	for _, name := range pk {
		if _, ok := values[name]; !ok {
			return fmt.Errorf("primary key column %q is missing", name)
		}
	}
	return nil
}

func nullableString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

// ParseInt is a tiny helper shared by the HTTP layer.
func ParseInt(s string, fallback int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}

var _ = sql.ErrNoRows
