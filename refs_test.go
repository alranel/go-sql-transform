package sqltransform_test

import (
	"sort"
	"testing"

	"github.com/alranel/go-sql-transform"
)

func refStrings(refs []sqltransform.Reference) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.Name.String()
	}
	sort.Strings(out)
	return out
}

func TestReadReferences_simple(t *testing.T) {
	q, err := sqltransform.Parse("SELECT * FROM users WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.ReadReferences())
	want := []string{"users.*", "users.id"}
	assertSetEqual(t, got, want)
}

func TestReadReferences_joinAliases(t *testing.T) {
	q, err := sqltransform.Parse("SELECT * FROM users u JOIN orders o ON u.id = o.user_id")
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.ReadReferences())
	want := []string{"orders.*", "orders.user_id", "users.*", "users.id"}
	assertSetEqual(t, got, want)
}

func TestReadReferences_cteResolution(t *testing.T) {
	q, err := sqltransform.Parse(`
		WITH active AS (SELECT id FROM users WHERE active)
		SELECT u.name FROM users u JOIN active a ON u.id = a.id`)
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.ReadReferences())
	want := []string{"users.active", "users.id", "users.name"}
	assertSetEqual(t, got, want)
}

func TestReadReferences_jsonbEachAlias(t *testing.T) {
	q, err := sqltransform.Parse(`
		WITH walk AS (SELECT '{}'::jsonb AS in_sede)
		SELECT e.k, e.v
		FROM walk AS w
		CROSS JOIN LATERAL jsonb_each(w.in_sede) AS e(k, v)`)
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.ReadReferences())
	if len(got) != 0 {
		t.Fatalf("expected no physical refs for SRF/CTE-only query, got %v", got)
	}
}

func TestReadReferences_jsonbArrayElementsWithBaseTable(t *testing.T) {
	q, err := sqltransform.Parse(`
		SELECT (elem->>'giorno')::date
		FROM users
		CROSS JOIN LATERAL jsonb_array_elements('[]'::jsonb) AS elem`)
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.ReadReferences())
	if len(got) != 0 {
		t.Fatalf("expected SRF alias elem not attributed to users, got %v", got)
	}
}

func TestReadReferences_syntheticCTEColumn(t *testing.T) {
	q, err := sqltransform.Parse(`
		WITH p AS (SELECT 1 AS x)
		SELECT p.x FROM p`)
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.ReadReferences())
	if len(got) != 0 {
		t.Fatalf("expected no physical refs for synthetic CTE column, got %v", got)
	}
}

func TestReadReferences_siblingCTESyntheticColumn(t *testing.T) {
	q, err := sqltransform.Parse(`
		WITH
		parametri AS (SELECT 1 AS id_ultimo_transito),
		candidati AS (
			SELECT t.id
			FROM users AS t
			CROSS JOIN parametri AS p
			WHERE t.id > p.id_ultimo_transito
		)
		SELECT candidati.id FROM candidati`)
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.ReadReferences())
	want := []string{"users.id"}
	assertSetEqual(t, got, want)
}

func TestReadReferences_lateralSeesOuterAlias(t *testing.T) {
	q, err := sqltransform.Parse(`
		SELECT k.chiave
		FROM users AS u
		CROSS JOIN LATERAL (SELECT u.id::text AS chiave) AS k`)
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.ReadReferences())
	want := []string{"users.id"}
	assertSetEqual(t, got, want)
}

func TestReadReferences_schemaQualified(t *testing.T) {
	q, err := sqltransform.Parse("SELECT * FROM public.users")
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.ReadReferences())
	want := []string{"public.users.*"}
	assertSetEqual(t, got, want)
}

func TestWriteReferences_insert(t *testing.T) {
	q, err := sqltransform.Parse("INSERT INTO users (name) VALUES ('test')")
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.WriteReferences())
	want := []string{"users.name"}
	assertSetEqual(t, got, want)
}

func TestWriteReferences_update(t *testing.T) {
	q, err := sqltransform.Parse("UPDATE users SET name = 'test' WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	writes := refStrings(q.WriteReferences())
	wantWrites := []string{"users.name"}
	assertSetEqual(t, writes, wantWrites)

	reads := refStrings(q.ReadReferences())
	wantReads := []string{"users.id"}
	assertSetEqual(t, reads, wantReads)
}

func TestWriteReferences_delete(t *testing.T) {
	q, err := sqltransform.Parse("DELETE FROM users WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	got := refStrings(q.WriteReferences())
	want := []string{"users"}
	assertSetEqual(t, got, want)
}

func assertSetEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
