package sqltransform_test

import (
	"sort"
	"testing"

	"github.com/alranel/go-sql-transform"
)

func funcCallNames(calls []sqltransform.FuncCall) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		if c.Schema != "" {
			out[i] = c.Schema + "." + c.Name
		} else {
			out[i] = c.Name
		}
	}
	sort.Strings(out)
	return out
}

func TestFunctionCalls(t *testing.T) {
	q, err := sqltransform.Parse("SELECT lower(name), count(*) FROM users WHERE id > abs(-1)")
	if err != nil {
		t.Fatal(err)
	}
	got := funcCallNames(q.FunctionCalls())
	want := []string{"abs", "count", "lower"}
	assertSetEqual(t, got, want)
}

func TestFunctionCalls_rejectsNoneForLiterals(t *testing.T) {
	q, err := sqltransform.Parse("SELECT 1 FROM users")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.FunctionCalls()) != 0 {
		t.Fatalf("expected no function calls, got %v", q.FunctionCalls())
	}
}

func TestReadReferences_updateWithCTE(t *testing.T) {
	q, err := sqltransform.Parse(`
		WITH active AS (SELECT id FROM users WHERE active)
		UPDATE users SET name = 'x' WHERE id IN (SELECT id FROM active)`)
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.ReadReferences())
	want := []string{"active.id", "users.active", "users.id"}
	assertSetEqual(t, got, want)
}

func TestReadReferences_updateFrom(t *testing.T) {
	q, err := sqltransform.Parse(`
		UPDATE users SET name = c.name FROM customers c WHERE users.customer = c.id`)
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.ReadReferences())
	want := []string{"customers.id", "customers.name", "users.customer"}
	assertSetEqual(t, got, want)
}

func TestExpandStar_singleTable(t *testing.T) {
	q, err := sqltransform.Parse("SELECT * FROM users")
	if err != nil {
		t.Fatal(err)
	}
	err = q.ExpandStar(func(table sqltransform.Name) ([]sqltransform.Name, error) {
		if table.Table != "users" {
			t.Fatalf("unexpected table %q", table.Table)
		}
		return []sqltransform.Name{
			{Table: "users", Column: "id"},
			{Table: "users", Column: "name"},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sql, err := q.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if refStrings(q.ReadReferences()) == nil {
		t.Fatal("expected refs")
	}
	got := refStrings(q.ReadReferences())
	want := []string{"users.id", "users.name"}
	assertSetEqual(t, got, want)
	if sql == "" {
		t.Fatal("empty sql")
	}
}
