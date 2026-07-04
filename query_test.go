package sqltransform_test

import (
	"testing"

	"github.com/alranel/go-sql-transform"
)

func TestCommand(t *testing.T) {
	tests := []struct {
		sql  string
		want sqltransform.Command
	}{
		{"SELECT 1", sqltransform.CommandSelect},
		{"WITH x AS (SELECT 1) SELECT * FROM x", sqltransform.CommandSelect},
		{"INSERT INTO users (name) VALUES ('a')", sqltransform.CommandInsert},
		{"INSERT INTO users SELECT name FROM other", sqltransform.CommandInsert},
		{"UPDATE users SET name = 'a'", sqltransform.CommandUpdate},
		{"DELETE FROM users", sqltransform.CommandDelete},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			q, err := sqltransform.Parse(tt.sql)
			if err != nil {
				t.Fatal(err)
			}
			cmd, err := q.Command()
			if err != nil {
				t.Fatal(err)
			}
			if cmd != tt.want {
				t.Fatalf("got %q, want %q", cmd, tt.want)
			}
		})
	}
}

func TestParse_rejectsMultiStatement(t *testing.T) {
	_, err := sqltransform.Parse("SELECT 1; SELECT 2")
	if err == nil {
		t.Fatal("expected error for multi-statement")
	}
}

func TestParse_rejectsUnsupported(t *testing.T) {
	_, err := sqltransform.Parse("CREATE TABLE users (id int)")
	if err == nil {
		t.Fatal("expected error for DDL")
	}
}

func TestParse_invalidSQL(t *testing.T) {
	_, err := sqltransform.Parse("SELECT * FROM")
	if err == nil {
		t.Fatal("expected parse error")
	}
}
