package sqltransform

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	pgparser "github.com/wasilibs/go-pgquery"
)

// Query is a parsed PostgreSQL DML statement that can be inspected and transformed.
type Query struct {
	tree *pg_query.ParseResult
}

// Parse parses a single PostgreSQL DML statement.
func Parse(sql string) (*Query, error) {
	tree, err := pgparser.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("sqltransform: parse: %w", err)
	}
	if len(tree.Stmts) != 1 {
		return nil, fmt.Errorf("sqltransform: expected exactly one statement, got %d", len(tree.Stmts))
	}
	if _, err := commandOf(tree); err != nil {
		return nil, err
	}
	return &Query{tree: tree}, nil
}

// SQL returns the query text after any transformations, deparsed from the AST.
func (q *Query) SQL() (string, error) {
	out, err := pgparser.Deparse(q.tree)
	if err != nil {
		return "", fmt.Errorf("sqltransform: deparse: %w", err)
	}
	return out, nil
}

// Command returns the top-level DML command type.
func (q *Query) Command() (Command, error) {
	return commandOf(q.tree)
}

func commandOf(tree *pg_query.ParseResult) (Command, error) {
	if len(tree.Stmts) == 0 {
		return "", fmt.Errorf("sqltransform: empty statement list")
	}
	stmt := tree.Stmts[0].Stmt
	switch {
	case stmt.GetSelectStmt() != nil:
		return CommandSelect, nil
	case stmt.GetInsertStmt() != nil:
		return CommandInsert, nil
	case stmt.GetUpdateStmt() != nil:
		return CommandUpdate, nil
	case stmt.GetDeleteStmt() != nil:
		return CommandDelete, nil
	default:
		return "", ErrUnsupportedStatement
	}
}
