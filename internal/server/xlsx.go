package server

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/xuri/excelize/v2"

	"navmini/internal/mysqlx"
)

type xlsxExportRequest struct {
	Kind       string `json:"kind"` // table | result
	Session    string `json:"session"`
	Database   string `json:"database"`
	Table      string `json:"table"`
	SQL        string `json:"sql"`
	IncludeSQL bool   `json:"includeSQL"`
	mysqlx.RowsRequest
	Columns []mysqlx.Column `json:"columns"`
	Rows    [][]any         `json:"rows"`
}

func (s *Server) handleExportXLSX(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req xlsxExportRequest
	if err := decodeJSON(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	name := safeFileName(req.Table)
	if name == "" {
		name = "query"
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.xlsx"`)
	w.Header().Set("Cache-Control", "no-store")

	if req.Kind == "result" {
		if len(req.Rows) > 50000 {
			fail(w, http.StatusBadRequest, errors.New("result is too large to export; narrow the query"))
			return
		}
		f := excelize.NewFile()
		defer f.Close()
		_ = f.SetSheetName("Sheet1", "Data")
		headStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
		for c, col := range req.Columns {
			cell, _ := excelize.CoordinatesToCellName(c+1, 1)
			_ = f.SetCellValue("Data", cell, col.Name)
			_ = f.SetCellStyle("Data", cell, cell, headStyle)
		}
		for ri, row := range req.Rows {
			for ci := range req.Columns {
				if ci >= len(row) {
					continue
				}
				cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
				_ = f.SetCellValue("Data", cell, row[ci])
			}
		}
		_ = f.SetPanes("Data", &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
		if req.IncludeSQL {
			_ = addResultSQLSheet(f, req.SQL)
		}
		if err := f.Write(w); err != nil {
			fail(w, http.StatusInternalServerError, err)
		}
		return
	}

	if req.Database == "" || req.Table == "" {
		fail(w, http.StatusBadRequest, errors.New("database and table are required"))
		return
	}
	sess, ok := sessionByID(s, w, req.Session)
	if !ok {
		return
	}
	sqlText := strings.TrimSpace(req.SQL)
	if sqlText == "" {
		sqlText = "SELECT * FROM " + req.Database + "." + req.Table
	}
	rowsReq := req.RowsRequest
	rowsReq.Database = req.Database
	rowsReq.Table = req.Table
	if _, err := sess.ExportTableXLSX(r.Context(), rowsReq, req.IncludeSQL, sqlText, w); err != nil {
		// Headers may already be sent; log only.
		log.Printf("xlsx export: %v", err)
	}
}

func addResultSQLSheet(f *excelize.File, sqlText string) error {
	if sqlText == "" {
		return nil
	}
	if _, err := f.NewSheet("SQL"); err != nil {
		return err
	}
	_ = f.SetColWidth("SQL", "A", "A", 100)
	_ = f.SetCellValue("SQL", "A1", "Query SQL")
	_ = f.SetCellValue("SQL", "A2", sqlText)
	style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "Menlo", Size: 10},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
	})
	_ = f.SetCellStyle("SQL", "A1", "A2", style)
	return nil
}
