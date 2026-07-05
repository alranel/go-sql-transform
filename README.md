# go-sql-transform

A Go library for parsing and transforming PostgreSQL DML queries.

`go-sql-transform` parses `SELECT`, `INSERT`, `UPDATE`, and `DELETE` statements (including joins, CTEs, and subqueries) and lets you:

- Inspect which **physical** tables and columns a query reads or writes
- List **function calls** in the query
- Determine the top-level command type
- **Rename** tables and columns in the query text
- **Expand** `SELECT *` / `table.*` in projections and `RETURNING` lists to explicit columns

Typical use cases:

- **Privilege checks** — verify that tables/columns referenced by a query exist and are readable/writable for the current user
- **Function allowlists** — detect calls to built-in or extension functions before executing user SQL
- **Logical-to-physical mapping** — rewrite logical table/column names in application SQL to the real names in the underlying database (for example multi-tenant table routing)
- **Column-level enforcement** — expand `SELECT *` to the columns the caller is allowed to read

Parsing is powered by the official PostgreSQL parser ([libpg_query](https://github.com/pganalyze/libpg_query)) via [go-pgquery](https://github.com/wasilibs/go-pgquery) (WebAssembly, no cgo required).

## Installation

```bash
go get github.com/alranel/go-sql-transform
```

Requires Go 1.26 or later.

## Quick start

```go
package main

import (
	"fmt"
	"log"

	"github.com/alranel/go-sql-transform"
)

func main() {
	q, err := sqltransform.Parse(`
		WITH active AS (SELECT id FROM users WHERE active)
		SELECT u.name FROM users u JOIN active a ON u.id = a.id`)
	if err != nil {
		log.Fatal(err)
	}

	cmd, _ := q.Command()
	fmt.Println(cmd) // SELECT

	for _, ref := range q.ReadReferences() {
		fmt.Println(ref.Name.String())
	}
	// users.id
	// users.active
	// users.name

	// Map logical table name to physical table name.
	if err := q.Replace(
		sqltransform.Name{Table: "users"},
		sqltransform.Name{Table: "tenant_42_users"},
	); err != nil {
		log.Fatal(err)
	}

	sql, err := q.SQL()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(sql)
}
```

## API

### `Parse(sql string) (*Query, error)`

Parses a single DML statement. Returns an error for invalid SQL, multi-statement input, or unsupported statement types (DDL, `MERGE`, etc.).

### `(*Query) Command() (Command, error)`

Returns the top-level command: `SELECT`, `INSERT`, `UPDATE`, or `DELETE`.

`INSERT ... SELECT` reports `INSERT`. `WITH ... SELECT` reports `SELECT`.

### `(*Query) ReadReferences() []Reference`

Returns deduplicated **physical** table and column names that the query reads (from clauses, projections, filters, joins, subqueries, CTE bodies, `UPDATE`/`DELETE` `RETURNING` lists, etc.).

For `UPDATE`, read references include columns from `FROM`, `WHERE`, `RETURNING`, and CTEs. For `DELETE`, they include columns from `USING`, `WHERE`, `RETURNING`, and CTEs.

Aliases and CTE names are resolved away. For example, `u.name` with `FROM users u` is reported as `users.name`, and `a.id` through a CTE that selects `id` from `users` is reported as `users.id`.

### `(*Query) WriteReferences() []Reference`

Returns deduplicated physical table and column names that the query writes:

| Statement | Write references |
|-----------|------------------|
| `INSERT`  | target table; explicit column list if present |
| `UPDATE`  | target table; columns in `SET` |
| `DELETE`  | target table only |

`WHERE` clause columns on `UPDATE`/`DELETE` appear in `ReadReferences`, not `WriteReferences`.

### `(*Query) Replace(from, to Name) error`

Renames a table or column throughout the query AST.

**Table replacement** — set `from.Table` (and optionally `from.Schema`), leave `Column` empty:

```go
q.Replace(
	sqltransform.Name{Table: "users"},
	sqltransform.Name{Table: "real_users"},
)
```

**Column replacement** — set `from.Column`; set `from.Table` to limit the replacement to one table:

```go
q.Replace(
	sqltransform.Name{Table: "users", Column: "email"},
	sqltransform.Name{Table: "users", Column: "email_enc"},
)
```

Replacement is **alias-aware**: `u.email` is matched when replacing `{Table: "users", Column: "email"}` and `u` is an alias for `users`. Table aliases themselves are not renamed.

### `(*Query) FunctionCalls() []FuncCall`

Returns deduplicated function invocations found anywhere in the query (projections, filters, `GROUP BY`, `HAVING`, `RETURNING`, subqueries, CTEs, etc.).

Schema-qualified calls such as `pg_catalog.lower(...)` are reported with `Schema` set to the qualifier and `Name` set to the function name. Literals and operators are not included.

### `(*Query) ExpandStar(expand ExpandStarFunc) error`

Replaces `SELECT *` and `table.*` in projection and `RETURNING` lists with explicit column references.

You provide a callback that receives the physical table name for each star and returns the columns to substitute. Return an error to abort expansion, or return a filtered column list to omit unreadable columns:

```go
err := q.ExpandStar(func(table sqltransform.Name) ([]sqltransform.Name, error) {
	if table.Table != "users" {
		return nil, fmt.Errorf("unknown table %q", table.Table)
	}
	return []sqltransform.Name{
		{Table: "users", Column: "id"},
		{Table: "users", Column: "name"},
	}, nil
})
```

Expansion runs through nested `SELECT`s, set operations (`UNION`, etc.), and CTE bodies. `INSERT ... SELECT` is expanded in the source query; `UPDATE`/`DELETE` stars are expanded only in `RETURNING` lists.

After expansion, call `SQL()` to deparse the modified query.

### `(*Query) SQL() (string, error)`

Deparses the (possibly modified) AST back to a SQL string.

## `Name` and `Reference`

```go
type Name struct {
	Schema string // optional
	Table  string // optional
	Column string // optional; "*" for SELECT *
}

type Reference struct {
	Name Name
}
```

`Name.String()` formats a reference for display, for example `users.email`, `public.users`, or `users.*`.

```go
type FuncCall struct {
	Schema string // optional catalog/schema prefix
	Name   string // function name (last component)
}

type ExpandStarFunc func(table Name) ([]Name, error)
```

## Reference resolution

`ReadReferences` and `WriteReferences` always return **physical** names suitable for catalog/privilege lookups:

| Written in SQL | Reported reference |
|--------------|-------------------|
| `FROM users u` … `u.name` | `users.name` |
| `FROM users` … `WHERE active` | `users.active` |
| `FROM active a` … `a.id` (CTE `active` selects `id` from `users`) | `users.id` |
| `SELECT * FROM users` | `users.*` |

Unqualified column names (`email` with no table prefix) are resolved when exactly one table is in scope. When multiple tables are in scope and the column is ambiguous, the table field is left empty: `email`.

## Limitations

- **DML only** — `SELECT`, `INSERT`, `UPDATE`, `DELETE` (no `MERGE`, DDL, or utility statements)
- **Single statement** — multi-statement strings are rejected
- **`INSERT` without a column list** — write columns cannot be enumerated; only the target table is reported
- **`SELECT *`** — `ReadReferences` reports `table.*`, not individual columns; use `ExpandStar` to rewrite stars to explicit columns
- **`ExpandStar` scope** — only projection and `RETURNING` lists are rewritten; stars elsewhere (for example in `WHERE`) are unchanged
- **Deparse formatting** — output SQL may differ in whitespace or quoting; semantics are preserved
- **Unqualified column replacement** — when `from.Table` is set, only qualified column references match; bare column names are not disambiguated by table

## Author

Alessandro Ranellucci

## License

MIT License. See [LICENSE](LICENSE).
