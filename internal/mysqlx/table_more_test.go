package mysqlx

import "testing"

func TestBuildWhereConnectors(t *testing.T) {
	cols := map[string]TableColumn{
		"id":     {Name: "id", Numeric: true},
		"status": {Name: "status"},
		"age":    {Name: "age", Numeric: true},
	}
	where, args, err := buildWhere(cols, []RowFilter{
		{Column: "id", Op: ">", Value: "1"},
		{Column: "status", Op: "=", Value: "2", Connector: "and"},
		{Column: "age", Op: "between", Value: "10", Value2: "20", Connector: "or"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := " WHERE (((`id` > ?) AND (`status` = ?)) OR (`age` BETWEEN ? AND ?))"
	if where != want {
		t.Fatalf("where = %q, want %q", where, want)
	}
	if len(args) != 4 || args[0] != "1" || args[3] != "20" {
		t.Fatalf("args = %#v", args)
	}

	where, args, err = buildWhere(cols, []RowFilter{{Column: "status", Op: "in", Value: "a, b ,c"}})
	if err != nil {
		t.Fatal(err)
	}
	if where != " WHERE (`status` IN (?, ?, ?))" || len(args) != 3 || args[1] != "b" {
		t.Fatalf("in where = %q args=%#v", where, args)
	}
}

func TestSQLLiteral(t *testing.T) {
	num := TableColumn{Name: "n", Numeric: true, ColumnType: "int"}
	str := TableColumn{Name: "s", ColumnType: "varchar"}
	bin := TableColumn{Name: "b", Binary: true, ColumnType: "blob"}
	v := "O'Reilly\\path\nline"
	if got := sqlLiteral(str, &v); got != `'O\'Reilly\\path\nline'` {
		t.Fatalf("string literal = %q", got)
	}
	n := "42.5"
	if got := sqlLiteral(num, &n); got != "42.5" {
		t.Fatalf("numeric literal = %q", got)
	}
	if got := sqlLiteral(num, nil); got != "NULL" {
		t.Fatalf("null literal = %q", got)
	}
	b := "0x00ff"
	if got := sqlLiteral(bin, &b); got != "0x00ff" {
		t.Fatalf("binary literal = %q", got)
	}
}
