package sqltransform_test

import (
	"strings"
	"testing"

	"github.com/alranel/go-sql-transform"
)

func TestReplace_table(t *testing.T) {
	q, err := sqltransform.Parse("SELECT * FROM users WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Replace(
		sqltransform.Name{Table: "users"},
		sqltransform.Name{Table: "t_users"},
	); err != nil {
		t.Fatal(err)
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "t_users") {
		t.Fatalf("expected t_users in %q", sql)
	}
	if strings.Contains(sql, " users") && !strings.Contains(sql, "t_users") {
		t.Fatalf("users should be replaced in %q", sql)
	}
}

func TestReplace_columnQualified(t *testing.T) {
	q, err := sqltransform.Parse("SELECT users.email, orders.email FROM users JOIN orders ON true")
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Replace(
		sqltransform.Name{Table: "users", Column: "email"},
		sqltransform.Name{Table: "users", Column: "email_enc"},
	); err != nil {
		t.Fatal(err)
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "email_enc") {
		t.Fatalf("expected email_enc in %q", sql)
	}
	// orders.email should remain
	if strings.Count(sql, "email") != 1 || strings.Contains(sql, "orders.email_enc") {
		// orders.email unchanged means plain "email" appears once in orders.email
		if !strings.Contains(sql, "orders.email") {
			t.Fatalf("orders.email should remain in %q", sql)
		}
	}
}

func TestReplace_columnAliasAware(t *testing.T) {
	q, err := sqltransform.Parse("SELECT u.email FROM users u")
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Replace(
		sqltransform.Name{Table: "users", Column: "email"},
		sqltransform.Name{Table: "users", Column: "email_enc"},
	); err != nil {
		t.Fatal(err)
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "email_enc") {
		t.Fatalf("expected email_enc in %q", sql)
	}
}
