package mysqlx

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"

	"github.com/xuri/excelize/v2"
)

// ExportTableXLSX streams a table (with current filters/order) into an xlsx
// workbook. DATETIME/TIMESTAMP/DATE/TIME values are written as text so Excel
// never reinterprets them as serial dates.
func (s *Session) ExportTableXLSX(ctx context.Context, req RowsRequest, includeSQL bool, sqlText string, w io.Writer) (int64, error) {
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
	query := "SELECT * FROM " + QuoteIdent(req.Database) + "." + QuoteIdent(req.Table) + where + order + " LIMIT 500000"
	rows, err := s.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	header, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	types, _ := rows.ColumnTypes()
	colDefs := describeColumns(header, types)

	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetSheetName("Sheet1", "Data")
	sw, err := f.NewStreamWriter("Data")
	if err != nil {
		return 0, err
	}
	headStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	head := make([]any, len(header))
	for i, name := range header {
		head[i] = excelize.Cell{StyleID: headStyle, Value: name}
	}
	if err := sw.SetRow("A1", head); err != nil {
		return 0, err
	}
	_ = sw.SetColWidth(1, len(header), 16)

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
		row := make([]any, len(header))
		for i := range values {
			row[i] = xlsxCell(EncodeCell(values[i], colDefs[i]), colDefs[i].DBType)
		}
		axis, _ := excelize.CoordinatesToCellName(1, int(count)+2)
		if err := sw.SetRow(axis, row); err != nil {
			return count, err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, err
	}
	if err := sw.Flush(); err != nil {
		return count, err
	}
	_ = f.SetPanes("Data", &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	if len(header) > 0 {
		last, _ := excelize.ColumnNumberToName(len(header))
		_ = f.AutoFilter("Data", "A1:"+last+"1", nil)
	}
	if includeSQL {
		if err := addSQLSheet(f, sqlText); err != nil {
			return count, err
		}
	}
	return count, f.Write(w)
}

func xlsxCell(v any, dbType string) any {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case time.Time:
		switch normalizeType(dbType) {
		case "DATE":
			return x.Format("2006-01-02")
		case "TIME":
			return x.Format("15:04:05")
		default:
			return x.Format("2006-01-02 15:04:05")
		}
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return fmt.Sprint(x)
	}
}

func addSQLSheet(f *excelize.File, sqlText string) error {
	if sqlText == "" {
		sqlText = "-- no SQL captured"
	}
	if _, err := f.NewSheet("SQL"); err != nil {
		return err
	}
	style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Family: "Menlo", Size: 10},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
	})
	_ = f.SetColWidth("SQL", "A", "A", 100)
	_ = f.SetCellValue("SQL", "A1", "Query SQL")
	_ = f.SetCellStyle("SQL", "A1", "A1", style)
	_ = f.SetCellValue("SQL", "A2", sqlText)
	_ = f.SetCellStyle("SQL", "A2", "A2", style)
	_ = f.SetRowHeight("SQL", 2, 300)
	return nil
}

func normalizeType(t string) string {
	out := make([]byte, 0, len(t))
	for i := 0; i < len(t); i++ {
		c := t[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		if (c >= 'A' && c <= 'Z') || c == ' ' {
			out = append(out, c)
		}
	}
	return string(out)
}
