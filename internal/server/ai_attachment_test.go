package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDelimitedAndXLSX(t *testing.T) {
	rows, err := parseDelimited([]byte("name,city\nAlice,Shanghai\nBob,Beijing\n"), ".csv")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[1][1] != "Shanghai" {
		t.Fatalf("rows = %#v", rows)
	}
	book, err := rowsToXLSX(rows)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "a.xlsx")
	if err := os.WriteFile(path, book, 0o600); err != nil {
		t.Fatal(err)
	}
	_, sheet, headers, preview, count, err := xlsxAttachmentMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	if sheet != "Data" || len(headers) != 2 || len(preview) != 2 || count != 2 {
		t.Fatalf("meta sheet=%q headers=%v preview=%v count=%d", sheet, headers, preview, count)
	}
}

func TestAttachmentContext(t *testing.T) {
	s := &Server{imports: newImportStore()}
	rows, _ := parseDelimited([]byte("name,city\nAlice,Shanghai\n"), ".csv")
	book, _ := rowsToXLSX(rows)
	id, err := s.imports.putExt(book, "people.csv", ".xlsx")
	if err != nil {
		t.Fatal(err)
	}
	textID, err := s.imports.putExt([]byte("SELECT 1;\n"), "note.sql", ".txt")
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.attachmentContext([]string{id, textID})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"people.csv", "headers: name | city", "Alice | Shanghai", "note.sql", "SELECT 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("context missing %q:\n%s", want, out)
		}
	}
}
