package sqltransform_test

import (
	"strings"
	"testing"

	"github.com/alranel/go-sql-transform"
)

func wrapCrypt(_ sqltransform.Name, expr sqltransform.Expr) (sqltransform.Expr, error) {
	return sqltransform.Call(
		"crypt",
		expr,
		sqltransform.Call("gen_salt", sqltransform.StringLiteral("bf"), sqltransform.IntLiteral(12)),
	), nil
}

func TestWrapWriteExpressions_update(t *testing.T) {
	q, err := sqltransform.Parse(`UPDATE users SET password = 'secret', name = 'a' WHERE id = 1`)
	if err != nil {
		t.Fatal(err)
	}
	err = q.WrapWriteExpressions(func(col sqltransform.Name) bool {
		return col.Column == "password"
	}, wrapCrypt)
	if err != nil {
		t.Fatal(err)
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "crypt('secret', gen_salt('bf', 12))") {
		t.Fatalf("expected crypt wrap in %q", sql)
	}
	if !strings.Contains(sql, "name = 'a'") {
		t.Fatalf("non-password assignment should stay plain in %q", sql)
	}
}

func TestWrapWriteExpressions_insertValues(t *testing.T) {
	q, err := sqltransform.Parse(`INSERT INTO users (name, password) VALUES ('a', 'secret'), ('b', 'other')`)
	if err != nil {
		t.Fatal(err)
	}
	err = q.WrapWriteExpressions(func(col sqltransform.Name) bool {
		return col.Table == "users" && col.Column == "password"
	}, wrapCrypt)
	if err != nil {
		t.Fatal(err)
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(sql, "crypt(") != 2 {
		t.Fatalf("expected two crypt wraps in %q", sql)
	}
	if !strings.Contains(sql, "'a'") || !strings.Contains(sql, "'b'") {
		t.Fatalf("name literals should remain in %q", sql)
	}
}

func TestWrapWriteExpressions_insertSelect(t *testing.T) {
	q, err := sqltransform.Parse(`INSERT INTO users (name, password) SELECT src.name, src.secret FROM src`)
	if err != nil {
		t.Fatal(err)
	}
	err = q.WrapWriteExpressions(func(col sqltransform.Name) bool {
		return col.Column == "password"
	}, wrapCrypt)
	if err != nil {
		t.Fatal(err)
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, "crypt(src.secret, gen_salt('bf', 12))") &&
		!strings.Contains(sql, "crypt(secret, gen_salt('bf', 12))") {
		t.Fatalf("expected crypt wrap around select target in %q", sql)
	}
}

func TestExpr_StringLiteralValue(t *testing.T) {
	q, err := sqltransform.Parse(`UPDATE users SET password = 'secret'`)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	err = q.WrapWriteExpressions(func(col sqltransform.Name) bool {
		return col.Column == "password"
	}, func(_ sqltransform.Name, expr sqltransform.Expr) (sqltransform.Expr, error) {
		s, ok := expr.StringLiteralValue()
		if !ok {
			t.Fatal("expected string literal")
		}
		got = s
		return expr, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "secret" {
		t.Fatalf("got %q", got)
	}
}
