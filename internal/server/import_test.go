package server

import (
	"testing"

	"navmini/internal/mysqlx"
)

func TestParseImportTime(t *testing.T) {
	cases := map[string]string{
		"2026-11-01 10:20:30": "2026-11-01 10:20:30",
		"2026/11/01 10:20":    "2026-11-01 10:20:00",
		"2026-11-01":          "2026-11-01 00:00:00",
	}
	for in, want := range cases {
		got, err := parseImportTime(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got.Format("2006-01-02 15:04:05") != want {
			t.Fatalf("%q => %s, want %s", in, got, want)
		}
	}
	serial, err := parseImportTime("46300")
	if err != nil || serial.Year() < 2026 {
		t.Fatalf("excel serial: %v %v", serial, err)
	}
	if _, err := parseImportTime("not a date"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestImportValue(t *testing.T) {
	d := mysqlx.TableColumn{Name: "created_at", DataType: "datetime", Nullable: true}
	v, err := importValue(d, "2026-11-01 10:20:30")
	if err != nil || v != "2026-11-01 10:20:30" {
		t.Fatalf("date = %#v, %v", v, err)
	}
	if v, err := importValue(d, ""); err != nil || v != nil {
		t.Fatalf("null = %#v, %v", v, err)
	}
	num := mysqlx.TableColumn{Name: "amount", DataType: "decimal", Nullable: false}
	if v, err := importValue(num, ""); err == nil || v != nil {
		t.Fatalf("required numeric should fail: %#v %v", v, err)
	}
}

func TestSuggestMapping(t *testing.T) {
	cols := []mysqlx.TableColumn{
		{Name: "id"}, {Name: "customer_name"}, {Name: "email"}, {Name: "created_at"},
	}
	got := suggestMapping([]string{"ID", "Customer Name", "E-mail", "Created At"}, cols)
	want := map[string]string{"0": "id", "1": "customer_name", "2": "email", "3": "created_at"}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("mapping[%s] = %q, want %q (all %#v)", k, got[k], v, got)
		}
	}
}

func TestDimensionSize(t *testing.T) {
	if r, c := dimensionSize("A1:F120"); r != 120 || c != 6 {
		t.Fatalf("dimension = %d/%d", r, c)
	}
}
