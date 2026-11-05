package server

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

type aiAttachment struct {
	ID       string        `json:"id"`
	UploadID string        `json:"uploadId,omitempty"`
	Name     string        `json:"name"`
	Kind     string        `json:"kind"` // table | text
	Size     int64         `json:"size"`
	Sheets   []importSheet `json:"sheets,omitempty"`
	Sheet    string        `json:"sheet,omitempty"`
	Headers  []string      `json:"headers,omitempty"`
	Preview  [][]string    `json:"preview,omitempty"`
	RowCount int           `json:"rowCount,omitempty"`
	Text     string        `json:"text,omitempty"`
}

// handleAIAttachment stores a file for the assistant. Tabular files are
// normalized to xlsx so the existing import wizard can consume the same id.
func (s *Server) handleAIAttachment(w http.ResponseWriter, r *http.Request) {
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
	data, err := io.ReadAll(io.LimitReader(file, 20<<20+1))
	if err != nil || len(data) > 20<<20 {
		fail(w, http.StatusBadRequest, errors.New("attachment is larger than 20 MB"))
		return
	}
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	info := aiAttachment{Name: hdr.Filename, Size: int64(len(data))}
	if isArchiveName(hdr.Filename) {
		summary, err := archiveAttachmentText(data, hdr.Filename)
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		id, err := s.imports.putExt([]byte(summary), hdr.Filename+".txt", ".txt")
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		text := summary
		if len(text) > 6000 {
			text = text[:6000] + "\n…"
		}
		info.ID, info.Kind, info.Text = id, "text", text
		writeJSON(w, http.StatusOK, info)
		return
	}
	switch ext {
	case ".xlsx", ".xlsm":
		id, err := s.imports.putExt(data, hdr.Filename, ".xlsx")
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		sheets, sheet, headers, preview, rows, err := xlsxAttachmentMeta(filepath.Join(s.imports.dir, id+".xlsx"))
		if err != nil {
			fail(w, http.StatusBadRequest, fmt.Errorf("cannot parse workbook: %w", err))
			return
		}
		info.ID, info.UploadID, info.Kind = id, id, "table"
		info.Sheets, info.Sheet, info.Headers, info.Preview, info.RowCount = sheets, sheet, headers, preview, rows
	case ".csv", ".tsv":
		rows, err := parseDelimited(data, ext)
		if err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
		book, err := rowsToXLSX(rows)
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		id, err := s.imports.putExt(book, hdr.Filename, ".xlsx")
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		info.ID, info.UploadID, info.Kind = id, id, "table"
		info.Sheet = "Data"
		info.Headers = rows[0]
		if len(rows) > 1 {
			limit := len(rows)
			if limit > 21 {
				limit = 21
			}
			info.Preview = rows[1:limit]
			info.RowCount = len(rows) - 1
		}
		info.Sheets = []importSheet{{Name: "Data", Rows: len(rows), Columns: len(rows[0])}}
	case ".txt", ".text", ".log", ".out", ".err", ".trace", ".sql", ".md", ".markdown", ".json",
		".yaml", ".yml", ".toml", ".ini", ".conf", ".properties", ".xml", ".html", ".css",
		".go", ".py", ".js", ".ts", ".jsx", ".tsx", ".java", ".kt", ".c", ".h", ".cpp", ".hpp", ".cc",
		".cs", ".php", ".rb", ".rs", ".swift", ".sh", ".vue", ".svelte", ".diff", ".patch", ".dockerfile":
		id, err := s.imports.putExt(data, hdr.Filename, ".txt")
		if err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		text := strings.ToValidUTF8(string(data), "\uFFFD")
		if len(text) > 6000 {
			text = text[:6000] + "\n…"
		}
		info.ID, info.Kind, info.Text = id, "text", text
	default:
		fail(w, http.StatusBadRequest, errors.New("supported attachments: xlsx, xlsm, csv, tsv, txt, logs, source code, sql, md, json, yaml, xml and more"))
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func xlsxAttachmentMeta(path string) ([]importSheet, string, []string, [][]string, int, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, "", nil, nil, 0, err
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
	if len(sheets) == 0 {
		return nil, "", nil, nil, 0, errors.New("workbook has no sheets")
	}
	sheet := sheets[0].Name
	headers, preview, rowCount, err := readSheetPreview(path, sheet, 1, 20)
	if err != nil {
		return nil, "", nil, nil, 0, err
	}
	if rowCount > 0 {
		rowCount-- // header row
	}
	return sheets, sheet, headers, preview, rowCount, nil
}

func parseDelimited(data []byte, ext string) ([][]string, error) {
	delim := ','
	if ext == ".tsv" {
		delim = '\t'
	} else if line := firstLine(data); strings.Count(line, ";") > strings.Count(line, ",") {
		delim = ';'
	}
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("cannot parse delimited file: %w", err)
	}
	if len(rows) == 0 {
		return nil, errors.New("attachment has no rows")
	}
	if len(rows) > 50001 {
		return nil, errors.New("delimited attachment exceeds 50000 rows")
	}
	maxCols := 0
	for i := range rows {
		if len(rows[i]) > maxCols {
			maxCols = len(rows[i])
		}
	}
	if maxCols == 0 {
		return nil, errors.New("attachment has no columns")
	}
	for i := range rows {
		for len(rows[i]) < maxCols {
			rows[i] = append(rows[i], "")
		}
		rows[i][0] = strings.TrimPrefix(rows[i][0], "\ufeff")
	}
	return rows, nil
}

func firstLine(data []byte) string {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return string(data[:i])
	}
	return string(data)
}

func rowsToXLSX(rows [][]string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName("Sheet1", "Data")
	sw, err := f.NewStreamWriter("Data")
	if err != nil {
		return nil, err
	}
	for i, row := range rows {
		axis, _ := excelize.CoordinatesToCellName(1, i+1)
		vals := make([]any, len(row))
		for j, v := range row {
			vals[j] = v
		}
		if err := sw.SetRow(axis, vals); err != nil {
			return nil, err
		}
	}
	if err := sw.Flush(); err != nil {
		return nil, err
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// attachmentContext turns up to three attachments into bounded prompt data.
// The caller marks it as untrusted.
func (s *Server) attachmentContext(ids []string) (string, error) {
	var b strings.Builder
	for i, id := range ids {
		if i >= 3 {
			break
		}
		fi, ok := s.imports.get(id)
		if !ok {
			return "", errors.New("attachment expired or not found")
		}
		ext := strings.ToLower(filepath.Ext(fi.path))
		nameExt := strings.ToLower(filepath.Ext(fi.name))
		fmt.Fprintf(&b, "\n[Attachment %d: %s]\n", i+1, fi.name)
		if ext == ".xlsx" {
			sheets, sheet, headers, preview, rows, err := xlsxAttachmentMeta(fi.path)
			if err != nil {
				return "", err
			}
			names := make([]string, len(sheets))
			for j, sh := range sheets {
				names[j] = sh.Name
			}
			fmt.Fprintf(&b, "table attachment; sheets=%s; current=%s; data_rows=%d\n", strings.Join(names, ", "), sheet, rows)
			fmt.Fprintf(&b, "headers: %s\n", strings.Join(headers, " | "))
			fmt.Fprintf(&b, "sample rows:\n")
			for j, row := range preview {
				if j >= 10 {
					break
				}
				vals := make([]string, len(row))
				for k, v := range row {
					if len(v) > 80 {
						v = v[:80] + "…"
					}
					vals[k] = v
				}
				fmt.Fprintf(&b, "%s\n", strings.Join(vals, " | "))
			}
		} else {
			data, err := os.ReadFile(fi.path)
			if err != nil {
				return "", err
			}
			text := strings.ToValidUTF8(string(data), "\uFFFD")
			lines := strings.Count(text, "\n") + 1
			kind := "text"
			if isLogName(nameExt) {
				kind = "service log"
			} else if isSourceName(nameExt) {
				kind = "source code"
			}
			text = clipAttachmentText(text, kind == "service log")
			fmt.Fprintf(&b, "%s attachment; lines=%d:\n%s\n", kind, lines, text)
		}
		if b.Len() > 60000 {
			break
		}
	}
	out := b.String()
	if len(out) > 65000 {
		out = out[:65000] + "\n…truncated"
	}
	return out, nil
}

func isLogName(ext string) bool {
	switch ext {
	case ".log", ".out", ".err", ".trace":
		return true
	}
	return false
}

func isSourceName(ext string) bool {
	switch ext {
	case ".go", ".py", ".js", ".ts", ".jsx", ".tsx", ".java", ".kt", ".c", ".h", ".cpp", ".hpp", ".cc",
		".cs", ".php", ".rb", ".rs", ".swift", ".sh", ".vue", ".svelte", ".sql":
		return true
	}
	return false
}

// Logs keep both the beginning and the end; source/text keep the beginning.
func clipAttachmentText(text string, log bool) string {
	const max = 30000
	if len(text) <= max {
		return text
	}
	if log {
		head, tail := 7000, max-7000-40
		return text[:head] + "\n…[middle of log truncated]…\n" + text[len(text)-tail:]
	}
	return text[:max] + "\n…truncated"
}
