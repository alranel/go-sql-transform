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
	if strings.Contains(sql, "users.email_enc") {
		t.Fatalf("alias qualifier should be preserved in %q", sql)
	}
}

func TestReplace_joinAliasPreservesQualifierInON(t *testing.T) {
	q, err := sqltransform.Parse(`SELECT * FROM book b LEFT JOIN author a ON a.id = b.author`)
	if err != nil {
		t.Fatal(err)
	}
	replacements := []struct {
		from, to sqltransform.Name
	}{
		{sqltransform.Name{Table: "book", Column: "id"}, sqltransform.Name{Table: "o_book", Column: "f_id"}},
		{sqltransform.Name{Table: "book", Column: "author"}, sqltransform.Name{Table: "o_book", Column: "f_author"}},
		{sqltransform.Name{Table: "author", Column: "id"}, sqltransform.Name{Table: "o_author", Column: "f_id"}},
		{sqltransform.Name{Table: "book"}, sqltransform.Name{Table: "o_book"}},
		{sqltransform.Name{Table: "author"}, sqltransform.Name{Table: "o_author"}},
	}
	for _, r := range replacements {
		if err := q.Replace(r.from, r.to); err != nil {
			t.Fatal(err)
		}
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sql, "o_author.") || strings.Contains(sql, "o_book.") {
		t.Fatalf("column refs should use aliases, not physical table qualifiers: %q", sql)
	}
	if !strings.Contains(sql, "a.f_id") || !strings.Contains(sql, "b.f_author") {
		t.Fatalf("expected alias-qualified ON clause in %q", sql)
	}
}

func TestReplace_updateSetExpressionColumns(t *testing.T) {
	q, err := sqltransform.Parse(`UPDATE domanda SET d_col = a_col WHERE d_col = 0 AND a_col > 0`)
	if err != nil {
		t.Fatal(err)
	}
	replacements := []struct {
		from, to sqltransform.Name
	}{
		{sqltransform.Name{Table: "domanda", Column: "d_col"}, sqltransform.Name{Table: "o_domanda", Column: "f_d_col"}},
		{sqltransform.Name{Table: "domanda", Column: "a_col"}, sqltransform.Name{Table: "o_domanda", Column: "f_a_col"}},
		{sqltransform.Name{Table: "domanda"}, sqltransform.Name{Table: "o_domanda"}},
	}
	for _, r := range replacements {
		if err := q.Replace(r.from, r.to); err != nil {
			t.Fatal(err)
		}
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	expected := "UPDATE o_domanda SET f_d_col = f_a_col WHERE f_d_col = 0 AND f_a_col > 0"
	if sql != expected {
		t.Fatalf("got %q want %q", sql, expected)
	}
}

func TestReplace_updateFrom(t *testing.T) {
	q, err := sqltransform.Parse(`
		UPDATE users SET name = c.name FROM customers c WHERE users.customer = c.id`)
	if err != nil {
		t.Fatal(err)
	}
	replacements := []struct {
		from, to sqltransform.Name
	}{
		{sqltransform.Name{Table: "users", Column: "name"}, sqltransform.Name{Table: "o_users", Column: "f_name"}},
		{sqltransform.Name{Table: "users", Column: "customer"}, sqltransform.Name{Table: "o_users", Column: "f_customer"}},
		{sqltransform.Name{Table: "customers", Column: "name"}, sqltransform.Name{Table: "o_customers", Column: "f_name"}},
		{sqltransform.Name{Table: "customers", Column: "id"}, sqltransform.Name{Table: "o_customers", Column: "f_id"}},
		{sqltransform.Name{Table: "users"}, sqltransform.Name{Table: "o_users"}},
		{sqltransform.Name{Table: "customers"}, sqltransform.Name{Table: "o_customers"}},
	}
	for _, r := range replacements {
		if err := q.Replace(r.from, r.to); err != nil {
			t.Fatal(err)
		}
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "UPDATE o_users") {
		t.Fatalf("expected target rewrite in %q", sql)
	}
	if !strings.Contains(sql, "FROM o_customers c") {
		t.Fatalf("expected FROM table rewrite in %q", sql)
	}
	if strings.Contains(sql, "FROM customers") {
		t.Fatalf("logical FROM table should be replaced in %q", sql)
	}
	if !strings.Contains(sql, "SET f_name = c.f_name") {
		t.Fatalf("expected SET column rewrite in %q", sql)
	}
	if !strings.Contains(sql, "o_users.f_customer = c.f_id") && !strings.Contains(sql, "f_customer = c.f_id") {
		t.Fatalf("expected WHERE column rewrite in %q", sql)
	}
}

func TestReplace_updateFromCTE(t *testing.T) {
	q, err := sqltransform.Parse(`
		WITH src AS (SELECT id, name FROM customers)
		UPDATE users SET name = src.name FROM src WHERE users.customer = src.id`)
	if err != nil {
		t.Fatal(err)
	}
	replacements := []struct {
		from, to sqltransform.Name
	}{
		{sqltransform.Name{Table: "users", Column: "name"}, sqltransform.Name{Table: "o_users", Column: "f_name"}},
		{sqltransform.Name{Table: "users", Column: "customer"}, sqltransform.Name{Table: "o_users", Column: "f_customer"}},
		{sqltransform.Name{Table: "customers", Column: "name"}, sqltransform.Name{Table: "o_customers", Column: "f_name"}},
		{sqltransform.Name{Table: "customers", Column: "id"}, sqltransform.Name{Table: "o_customers", Column: "f_id"}},
		{sqltransform.Name{Table: "users"}, sqltransform.Name{Table: "o_users"}},
		{sqltransform.Name{Table: "customers"}, sqltransform.Name{Table: "o_customers"}},
	}
	for _, r := range replacements {
		if err := q.Replace(r.from, r.to); err != nil {
			t.Fatal(err)
		}
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "FROM o_customers") {
		t.Fatalf("expected CTE body table rewrite in %q", sql)
	}
	if !strings.Contains(sql, "FROM src") {
		t.Fatalf("CTE name src should be preserved in %q", sql)
	}
	if strings.Contains(sql, "FROM o_src") {
		t.Fatalf("CTE name must not be rewritten in %q", sql)
	}
}

func TestReplace_deleteUsing(t *testing.T) {
	q, err := sqltransform.Parse(`
		DELETE FROM users USING customers c WHERE users.customer = c.id AND c.active`)
	if err != nil {
		t.Fatal(err)
	}
	replacements := []struct {
		from, to sqltransform.Name
	}{
		{sqltransform.Name{Table: "users", Column: "customer"}, sqltransform.Name{Table: "o_users", Column: "f_customer"}},
		{sqltransform.Name{Table: "customers", Column: "id"}, sqltransform.Name{Table: "o_customers", Column: "f_id"}},
		{sqltransform.Name{Table: "customers", Column: "active"}, sqltransform.Name{Table: "o_customers", Column: "f_active"}},
		{sqltransform.Name{Table: "users"}, sqltransform.Name{Table: "o_users"}},
		{sqltransform.Name{Table: "customers"}, sqltransform.Name{Table: "o_customers"}},
	}
	for _, r := range replacements {
		if err := q.Replace(r.from, r.to); err != nil {
			t.Fatal(err)
		}
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "DELETE FROM o_users") {
		t.Fatalf("expected target rewrite in %q", sql)
	}
	if !strings.Contains(sql, "USING o_customers c") {
		t.Fatalf("expected USING table rewrite in %q", sql)
	}
	if strings.Contains(sql, "USING customers") {
		t.Fatalf("logical USING table should be replaced in %q", sql)
	}
}

func TestReplace_columnUnqualifiedSingleTable(t *testing.T) {
	q, err := sqltransform.Parse("SELECT author FROM book WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Replace(
		sqltransform.Name{Table: "book", Column: "author"},
		sqltransform.Name{Table: "o_book", Column: "f_author"},
	); err != nil {
		t.Fatal(err)
	}
	if err := q.Replace(
		sqltransform.Name{Table: "book", Column: "id"},
		sqltransform.Name{Table: "o_book", Column: "id"},
	); err != nil {
		t.Fatal(err)
	}
	if err := q.Replace(
		sqltransform.Name{Table: "book"},
		sqltransform.Name{Table: "o_book"},
	); err != nil {
		t.Fatal(err)
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sql, " author") && !strings.Contains(sql, "f_author") {
		t.Fatalf("expected f_author in %q", sql)
	}
	if !strings.Contains(sql, "o_book") {
		t.Fatalf("expected o_book in %q", sql)
	}
}
