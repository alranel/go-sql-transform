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

func TestReplace_greatestInsideCTESubquery(t *testing.T) {
	// GREATEST/LEAST are MinMaxExpr in the Postgres AST, not FuncCall.
	q, err := sqltransform.Parse(`
		WITH parametri AS (
			SELECT COALESCE((
				SELECT t.giorno FROM permanenza AS p
				JOIN transito AS t
				  ON t.id = GREATEST(COALESCE(p.transito_entrata, 0), COALESCE(p.transito_uscita, 0))
				ORDER BY GREATEST(COALESCE(p.transito_entrata, 0), COALESCE(p.transito_uscita, 0)) DESC
				LIMIT 1
			), CURRENT_DATE) AS giorno_partenza
		)
		SELECT p.giorno_partenza FROM parametri AS p`)
	if err != nil {
		t.Fatal(err)
	}
	replacements := []struct {
		from, to sqltransform.Name
	}{
		{sqltransform.Name{Table: "permanenza", Column: "transito_entrata"}, sqltransform.Name{Table: "o_permanenza", Column: "f_transito_entrata"}},
		{sqltransform.Name{Table: "permanenza", Column: "transito_uscita"}, sqltransform.Name{Table: "o_permanenza", Column: "f_transito_uscita"}},
		{sqltransform.Name{Table: "transito", Column: "giorno"}, sqltransform.Name{Table: "o_transito", Column: "f_giorno"}},
		{sqltransform.Name{Table: "transito", Column: "id"}, sqltransform.Name{Table: "o_transito", Column: "id"}},
		{sqltransform.Name{Table: "permanenza"}, sqltransform.Name{Table: "o_permanenza"}},
		{sqltransform.Name{Table: "transito"}, sqltransform.Name{Table: "o_transito"}},
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
	if strings.Contains(sql, "p.transito_entrata") || strings.Contains(sql, "p.transito_uscita") {
		t.Fatalf("logical columns left inside GREATEST: %q", sql)
	}
	if !strings.Contains(sql, "p.f_transito_entrata") || !strings.Contains(sql, "p.f_transito_uscita") {
		t.Fatalf("expected physical columns inside GREATEST: %q", sql)
	}
}

func TestReplace_windowOverOrderBy(t *testing.T) {
	// ORDER BY / PARTITION BY inside OVER are WindowDef, not top-level SortClause.
	q, err := sqltransform.Parse(`
		SELECT t.id, row_number() OVER (ORDER BY t.giorno, t.dataoratransito, t.id) AS rn
		FROM transito AS t`)
	if err != nil {
		t.Fatal(err)
	}
	replacements := []struct {
		from, to sqltransform.Name
	}{
		{sqltransform.Name{Table: "transito", Column: "giorno"}, sqltransform.Name{Table: "o_transito", Column: "f_giorno"}},
		{sqltransform.Name{Table: "transito", Column: "dataoratransito"}, sqltransform.Name{Table: "o_transito", Column: "f_dataoratransito"}},
		{sqltransform.Name{Table: "transito", Column: "id"}, sqltransform.Name{Table: "o_transito", Column: "id"}},
		{sqltransform.Name{Table: "transito"}, sqltransform.Name{Table: "o_transito"}},
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
	if strings.Contains(sql, "t.giorno") || strings.Contains(sql, "t.dataoratransito") {
		t.Fatalf("logical columns left inside OVER ORDER BY: %q", sql)
	}
	if !strings.Contains(sql, "t.f_giorno") || !strings.Contains(sql, "t.f_dataoratransito") {
		t.Fatalf("expected physical columns inside OVER ORDER BY: %q", sql)
	}
}

func TestReplace_preservesSelectListAliasForCTE(t *testing.T) {
	// SELECT t.giorno must deparse as t.f_giorno AS giorno so CTE consumers
	// can still reference c.giorno.
	q, err := sqltransform.Parse(`
		WITH candidati AS (
			SELECT t.id, t.giorno, t.sede
			FROM transito AS t
		)
		SELECT c.giorno FROM candidati AS c`)
	if err != nil {
		t.Fatal(err)
	}
	replacements := []struct {
		from, to sqltransform.Name
	}{
		{sqltransform.Name{Table: "transito", Column: "giorno"}, sqltransform.Name{Table: "o_transito", Column: "f_giorno"}},
		{sqltransform.Name{Table: "transito", Column: "sede"}, sqltransform.Name{Table: "o_transito", Column: "f_sede"}},
		{sqltransform.Name{Table: "transito", Column: "id"}, sqltransform.Name{Table: "o_transito", Column: "id"}},
		{sqltransform.Name{Table: "transito"}, sqltransform.Name{Table: "o_transito"}},
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
	if !strings.Contains(sql, "t.f_giorno AS giorno") && !strings.Contains(sql, "t.f_giorno AS Giorno") {
		// deparser may quote or use original casing from the ColumnRef
		if !strings.Contains(strings.ToLower(sql), "t.f_giorno as giorno") {
			t.Fatalf("expected AS giorno alias after rewrite: %q", sql)
		}
	}
	if !strings.Contains(sql, "c.giorno") {
		t.Fatalf("expected CTE consumer c.giorno unchanged: %q", sql)
	}
	if strings.Contains(sql, "c.f_giorno") {
		t.Fatalf("CTE column must stay logical: %q", sql)
	}
}

func TestReplace_correlatedSubqueryOuterAlias(t *testing.T) {
	// Outer alias p must still rewrite inside NOT EXISTS.
	q, err := sqltransform.Parse(`
		SELECT p.id
		FROM permanenza AS p
		WHERE NOT EXISTS (
			SELECT 1 FROM presenza AS pr
			WHERE pr.transito_entrata IS NOT DISTINCT FROM p.transito_entrata
		)`)
	if err != nil {
		t.Fatal(err)
	}
	replacements := []struct {
		from, to sqltransform.Name
	}{
		{sqltransform.Name{Table: "permanenza", Column: "transito_entrata"}, sqltransform.Name{Table: "o_permanenza", Column: "f_transito_entrata"}},
		{sqltransform.Name{Table: "permanenza", Column: "id"}, sqltransform.Name{Table: "o_permanenza", Column: "id"}},
		{sqltransform.Name{Table: "presenza", Column: "transito_entrata"}, sqltransform.Name{Table: "o_presenza", Column: "f_transito_entrata"}},
		{sqltransform.Name{Table: "permanenza"}, sqltransform.Name{Table: "o_permanenza"}},
		{sqltransform.Name{Table: "presenza"}, sqltransform.Name{Table: "o_presenza"}},
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
	if strings.Contains(sql, "p.transito_entrata") {
		t.Fatalf("outer alias left unrewritten in subquery: %q", sql)
	}
	if !strings.Contains(sql, "p.f_transito_entrata") {
		t.Fatalf("expected p.f_transito_entrata in subquery: %q", sql)
	}
}
