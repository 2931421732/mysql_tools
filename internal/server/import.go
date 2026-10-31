package server

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"

	"navmini/internal/mysqlx"
)

type importFile struct {
	path    string
	name    string
	created time.Time
}

type importStore struct {
	mu    sync.Mutex
	dir   string
	files map[string]importFile
}

func newImportStore() *importStore {
	dir := filepath.Join(os.TempDir(), "navmini-imports")
	_ = os.MkdirAll(dir, 0o700)
	return &importStore{dir: dir, files: map[string]importFile{}}
}

func importID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *importStore) put(data []byte, name string) (string, error) {
	return s.putExt(data, name, ".xlsx")
}

func (s *importStore) putExt(data []byte, name, ext string) (string, error) {
	id := importID()
	p := filepath.Join(s.dir, id+ext)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.files[id] = importFile{path: p, name: name, created: time.Now()}
	s.mu.Unlock()
	return id, nil
}

func (s *importStore) get(id string) (importFile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[id]
	if !ok {
		return importFile{}, false
	}
	if time.Since(f.created) > 2*time.Hour {
		_ = os.Remove(f.path)
		delete(s.files, id)
		return importFile{}, false
	}
	return f, true
}

func (s *importStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, f := range s.files {
		if time.Since(f.created) > 2*time.Hour {
			_ = os.Remove(f.path)
			delete(s.files, id)
		}
	}
}

type importSheet struct {
	Name      string `json:"name"`
	Dimension string `json:"dimension"`
	Rows      int    `json:"rows"`
	Columns   int    `json:"columns"`
}

func (s *Server) handleImportUpload(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	s.imports.cleanup()
	if err := r.ParseMultipartForm(24 << 20); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if ext != ".xlsx" && ext != ".xlsm" {
		fail(w, http.StatusBadRequest, errors.New("only .xlsx/.xlsm files are supported"))
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, 20<<20+1))
	if err != nil || len(data) > 20<<20 {
		fail(w, http.StatusBadRequest, errors.New("file is larger than 20 MB"))
		return
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("cannot read workbook: %w", err))
		return
	}
	defer f.Close()
	sheets := make([]importSheet, 0)
	for _, name := range f.GetSheetList() {
		dim, _ := f.GetSheetDimension(name)
		rows, cols := dimensionSize(dim)
		if rows <= 1 || cols <= 0 {
			rows, cols = sheetSize(f, name)
		}
		sheets = append(sheets, importSheet{Name: name, Dimension: dim, Rows: rows, Columns: cols})
	}
	id, err := s.imports.put(data, hdr.Filename)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"uploadId": id, "filename": hdr.Filename, "sheets": sheets})
}

func sheetSize(f *excelize.File, sheet string) (int, int) {
	rows, err := f.Rows(sheet)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()
	count, maxCols := 0, 0
	for rows.Next() {
		values, err := rows.Columns()
		if err != nil {
			break
		}
		count++
		if len(values) > maxCols {
			maxCols = len(values)
		}
	}
	return count, maxCols
}

func dimensionSize(dim string) (int, int) {
	dim = strings.TrimSpace(dim)
	if dim == "" {
		return 0, 0
	}
	parts := strings.Split(dim, ":")
	last := parts[len(parts)-1]
	i := 0
	for i < len(last) && ((last[i] >= 'A' && last[i] <= 'Z') || (last[i] >= 'a' && last[i] <= 'z')) {
		i++
	}
	colPart := last[:i]
	rowPart := last[i:]
	col, _ := excelize.ColumnNameToNumber(colPart)
	row, _ := strconv.Atoi(rowPart)
	return row, col
}

type importPreviewRequest struct {
	UploadID  string `json:"uploadId"`
	Sheet     string `json:"sheet"`
	HeaderRow int    `json:"headerRow"`
	Session   string `json:"session"`
	Database  string `json:"database"`
	Table     string `json:"table"`
}

type importPreviewResponse struct {
	Headers  []string             `json:"headers"`
	Preview  [][]string           `json:"preview"`
	Columns  []mysqlx.TableColumn `json:"columns"`
	Mapping  map[string]string    `json:"mapping"`
	RowCount int                  `json:"rowCount"`
	Sheet    string               `json:"sheet"`
	Header   int                  `json:"headerRow"`
}

func (s *Server) handleImportPreview(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req importPreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	if req.Database == "" || req.Table == "" {
		fail(w, http.StatusBadRequest, errors.New("database and table are required"))
		return
	}
	if req.HeaderRow < 1 {
		req.HeaderRow = 1
	}
	fi, ok := s.imports.get(req.UploadID)
	if !ok {
		fail(w, http.StatusNotFound, errors.New("upload expired or not found"))
		return
	}
	cols, err := sess.Columns(r.Context(), req.Database, req.Table)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	headers, preview, rowCount, err := readSheetPreview(fi.path, req.Sheet, req.HeaderRow, 8)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, importPreviewResponse{
		Headers: headers, Preview: preview, Columns: cols,
		Mapping: suggestMapping(headers, cols), RowCount: rowCount,
		Sheet: req.Sheet, Header: req.HeaderRow,
	})
}

type importExecuteRequest struct {
	UploadID  string            `json:"uploadId"`
	Sheet     string            `json:"sheet"`
	HeaderRow int               `json:"headerRow"`
	Session   string            `json:"session"`
	Database  string            `json:"database"`
	Table     string            `json:"table"`
	Mapping   map[string]string `json:"mapping"` // source header index -> target column
}

func (s *Server) handleImportExecute(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req importExecuteRequest
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	if req.Database == "" || req.Table == "" {
		fail(w, http.StatusBadRequest, errors.New("database and table are required"))
		return
	}
	if req.HeaderRow < 1 {
		req.HeaderRow = 1
	}
	fi, ok := s.imports.get(req.UploadID)
	if !ok {
		fail(w, http.StatusNotFound, errors.New("upload expired or not found"))
		return
	}
	cols, err := sess.Columns(r.Context(), req.Database, req.Table)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	byName := make(map[string]mysqlx.TableColumn, len(cols))
	for _, c := range cols {
		byName[c.Name] = c
	}
	targets := make([]mysqlx.TableColumn, 0, len(req.Mapping))
	sourceIdx := make([]int, 0, len(req.Mapping))
	seen := map[string]bool{}
	for src, name := range req.Mapping {
		if name == "" {
			continue
		}
		idx, err := strconv.Atoi(src)
		if err != nil || idx < 0 {
			fail(w, http.StatusBadRequest, fmt.Errorf("invalid source column %q", src))
			return
		}
		c, ok := byName[name]
		if !ok {
			fail(w, http.StatusBadRequest, fmt.Errorf("unknown target column %q", name))
			return
		}
		if c.Generated {
			fail(w, http.StatusBadRequest, fmt.Errorf("column %q is generated", name))
			return
		}
		if seen[name] {
			fail(w, http.StatusBadRequest, fmt.Errorf("column %q is mapped twice", name))
			return
		}
		seen[name] = true
		targets = append(targets, c)
		sourceIdx = append(sourceIdx, idx)
	}
	if len(targets) == 0 {
		fail(w, http.StatusBadRequest, errors.New("map at least one column"))
		return
	}
	rows, err := readSheetRows(fi.path, req.Sheet, req.HeaderRow, sourceIdx, targets, 50000)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	started := time.Now()
	names := make([]string, len(targets))
	for i, c := range targets {
		names[i] = c.Name
	}
	inserted, err := sess.InsertRows(r.Context(), req.Database, req.Table, names, rows, 250)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"inserted": inserted, "scanned": len(rows),
		"durationMs": time.Since(started).Milliseconds(),
	})
}

func readSheetPreview(path, sheet string, headerRow, limit int) ([]string, [][]string, int, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, nil, 0, err
	}
	defer f.Close()
	rows, err := f.Rows(sheet)
	if err != nil {
		return nil, nil, 0, err
	}
	defer rows.Close()
	var headers []string
	var preview [][]string
	rowNo := 0
	for rows.Next() {
		rowNo++
		values, err := rows.Columns()
		if err != nil {
			return nil, nil, 0, err
		}
		if rowNo < headerRow {
			continue
		}
		if rowNo == headerRow {
			headers = values
			continue
		}
		if len(preview) < limit {
			preview = append(preview, values)
		}
		if len(preview) >= limit {
			break
		}
	}
	if headers == nil {
		return nil, nil, 0, errors.New("header row is outside the sheet")
	}
	dim := 0
	if d, err := f.GetSheetDimension(sheet); err == nil {
		dim, _ = dimensionSize(d)
	}
	if dim <= 1 {
		dim, _ = sheetSize(f, sheet)
	}
	return headers, preview, dim, rows.Error()
}

func readSheetRows(path, sheet string, headerRow int, sourceIdx []int, targets []mysqlx.TableColumn, maxRows int) ([][]any, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := f.Rows(sheet)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([][]any, 0, 256)
	rowNo := 0
	for rows.Next() {
		rowNo++
		if rowNo <= headerRow {
			continue
		}
		values, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		row := make([]any, len(targets))
		empty := true
		for i, idx := range sourceIdx {
			raw := ""
			if idx < len(values) {
				raw = values[idx]
			}
			if strings.TrimSpace(raw) != "" {
				empty = false
			}
			v, err := importValue(targets[i], raw)
			if err != nil {
				return nil, fmt.Errorf("row %d, column %s: %w", rowNo, targets[i].Name, err)
			}
			row[i] = v
		}
		if empty {
			continue
		}
		out = append(out, row)
		if len(out) > maxRows {
			return nil, fmt.Errorf("too many rows (limit %d)", maxRows)
		}
	}
	return out, rows.Error()
}

func suggestMapping(headers []string, cols []mysqlx.TableColumn) map[string]string {
	out := map[string]string{}
	used := map[string]bool{}
	norm := func(s string) string {
		var b strings.Builder
		for _, r := range strings.ToLower(s) {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			}
		}
		return b.String()
	}
	for i, h := range headers {
		n := norm(h)
		if n == "" {
			continue
		}
		for _, c := range cols {
			if c.Generated || used[c.Name] {
				continue
			}
			if norm(c.Name) == n {
				out[strconv.Itoa(i)] = c.Name
				used[c.Name] = true
				break
			}
		}
	}
	for i, h := range headers {
		if out[strconv.Itoa(i)] != "" {
			continue
		}
		n := norm(h)
		if n == "" {
			continue
		}
		best, bestScore := "", 0
		for _, c := range cols {
			if c.Generated || used[c.Name] {
				continue
			}
			cn := norm(c.Name)
			score := 0
			if strings.Contains(n, cn) || strings.Contains(cn, n) {
				score = len(cn)
				if len(n) < score {
					score = len(n)
				}
			}
			if score > bestScore {
				best, bestScore = c.Name, score
			}
		}
		if best != "" && bestScore >= 3 {
			out[strconv.Itoa(i)] = best
			used[best] = true
		}
	}
	return out
}

func importValue(c mysqlx.TableColumn, raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	empty := raw == ""
	if empty {
		if c.Nullable || c.AutoIncr {
			return nil, nil
		}
		if c.Default != nil {
			d := strings.TrimSpace(*c.Default)
			if strings.EqualFold(d, "CURRENT_TIMESTAMP") {
				return time.Now().Format("2006-01-02 15:04:05"), nil
			}
			if strings.EqualFold(d, "NULL") {
				return nil, nil
			}
			if len(d) >= 2 && d[0] == '\'' && d[len(d)-1] == '\'' {
				return strings.ReplaceAll(d[1:len(d)-1], "''", "'"), nil
			}
			return d, nil
		}
		if c.Numeric || isNumericDataType(c.DataType) || isDateDataType(c.DataType) {
			return nil, fmt.Errorf("value is required")
		}
		return "", nil
	}
	if isDateDataType(c.DataType) {
		t, err := parseImportTime(raw)
		if err != nil {
			return nil, err
		}
		switch strings.ToUpper(c.DataType) {
		case "DATE":
			return t.Format("2006-01-02"), nil
		case "TIME":
			return t.Format("15:04:05"), nil
		default:
			return t.Format("2006-01-02 15:04:05"), nil
		}
	}
	return raw, nil
}

func isNumericDataType(t string) bool {
	switch strings.ToUpper(t) {
	case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "INTEGER", "BIGINT", "DECIMAL", "NUMERIC", "FLOAT", "DOUBLE", "REAL", "BIT", "YEAR":
		return true
	}
	return false
}

func isDateDataType(t string) bool {
	switch strings.ToUpper(t) {
	case "DATE", "DATETIME", "TIMESTAMP", "TIME":
		return true
	}
	return false
}

func parseImportTime(raw string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02",
		"2006/1/2 15:04:05", "2006/1/2 15:04", "2006/1/2",
		"1/2/2006 15:04:05", "1/2/2006", "1/2/06", "02-01-06",
		time.RFC3339,
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, raw, time.Local); err == nil {
			return t, nil
		}
	}
	if n, err := strconv.ParseFloat(raw, 64); err == nil && n > 1 && n < 200000 {
		// Excel serial date: 1899-12-30 plus days.
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.Local)
		return base.Add(time.Duration(n * 24 * float64(time.Hour))), nil
	}
	return time.Time{}, fmt.Errorf("cannot parse date/time %q", raw)
}
