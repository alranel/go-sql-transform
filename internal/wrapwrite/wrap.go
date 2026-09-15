package wrapwrite

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/alranel/go-sql-transform/internal/extract"
)

// MatchFunc reports whether a write-target column should have its RHS wrapped.
type MatchFunc func(column extract.Name) bool

// WrapFunc replaces an expression assigned to a matched write target.
type WrapFunc func(column extract.Name, expr *pg_query.Node) (*pg_query.Node, error)

// Apply wraps UPDATE SET and INSERT VALUES/SELECT expressions for matched columns.
func Apply(tree *pg_query.ParseResult, match MatchFunc, wrap WrapFunc) error {
	for _, stmt := range tree.Stmts {
		if err := applyStmt(stmt.Stmt, match, wrap); err != nil {
			return err
		}
	}
	return nil
}

func applyStmt(node *pg_query.Node, match MatchFunc, wrap WrapFunc) error {
	if node == nil {
		return nil
	}
	switch {
	case node.GetInsertStmt() != nil:
		return applyInsert(node.GetInsertStmt(), match, wrap)
	case node.GetUpdateStmt() != nil:
		return applyUpdate(node.GetUpdateStmt(), match, wrap)
	}
	return nil
}

func applyUpdate(upd *pg_query.UpdateStmt, match MatchFunc, wrap WrapFunc) error {
	if upd == nil {
		return nil
	}
	rel := upd.Relation
	table := extract.Name{}
	if rel != nil {
		table = extract.Name{Schema: rel.Schemaname, Table: rel.Relname}
	}
	for _, t := range upd.TargetList {
		rt := t.GetResTarget()
		if rt == nil || rt.Name == "" {
			continue
		}
		col := extract.Name{Schema: table.Schema, Table: table.Table, Column: rt.Name}
		if !match(col) {
			continue
		}
		wrapped, err := wrap(col, rt.Val)
		if err != nil {
			return err
		}
		rt.Val = wrapped
	}
	return nil
}

func applyInsert(ins *pg_query.InsertStmt, match MatchFunc, wrap WrapFunc) error {
	if ins == nil || len(ins.Cols) == 0 {
		return nil
	}
	rel := ins.Relation
	table := extract.Name{}
	if rel != nil {
		table = extract.Name{Schema: rel.Schemaname, Table: rel.Relname}
	}

	indexes := make([]int, 0, len(ins.Cols))
	columns := make([]extract.Name, 0, len(ins.Cols))
	for i, col := range ins.Cols {
		rt := col.GetResTarget()
		if rt == nil || rt.Name == "" {
			continue
		}
		name := extract.Name{Schema: table.Schema, Table: table.Table, Column: rt.Name}
		if match(name) {
			indexes = append(indexes, i)
			columns = append(columns, name)
		}
	}
	if len(indexes) == 0 {
		return nil
	}

	sel := ins.SelectStmt.GetSelectStmt()
	if sel == nil {
		return fmt.Errorf("wrapwrite: INSERT has no select/values clause")
	}

	if len(sel.ValuesLists) > 0 {
		return wrapValuesLists(sel.ValuesLists, indexes, columns, wrap)
	}
	return wrapSelectTargets(sel, indexes, columns, wrap)
}

func wrapValuesLists(lists []*pg_query.Node, indexes []int, columns []extract.Name, wrap WrapFunc) error {
	for _, listNode := range lists {
		list := listNode.GetList()
		if list == nil {
			return fmt.Errorf("wrapwrite: expected VALUES list node")
		}
		for j, idx := range indexes {
			if idx < 0 || idx >= len(list.Items) {
				return fmt.Errorf("wrapwrite: VALUES column index %d out of range", idx)
			}
			wrapped, err := wrap(columns[j], list.Items[idx])
			if err != nil {
				return err
			}
			list.Items[idx] = wrapped
		}
	}
	return nil
}

func wrapSelectTargets(sel *pg_query.SelectStmt, indexes []int, columns []extract.Name, wrap WrapFunc) error {
	if sel == nil {
		return nil
	}
	switch sel.Op {
	case pg_query.SetOperation_SETOP_UNION, pg_query.SetOperation_SETOP_INTERSECT, pg_query.SetOperation_SETOP_EXCEPT:
		if err := wrapSelectTargets(sel.Larg, indexes, columns, wrap); err != nil {
			return err
		}
		return wrapSelectTargets(sel.Rarg, indexes, columns, wrap)
	}
	if len(sel.ValuesLists) > 0 {
		return wrapValuesLists(sel.ValuesLists, indexes, columns, wrap)
	}
	for j, idx := range indexes {
		if idx < 0 || idx >= len(sel.TargetList) {
			return fmt.Errorf("wrapwrite: SELECT target index %d out of range", idx)
		}
		rt := sel.TargetList[idx].GetResTarget()
		if rt == nil {
			return fmt.Errorf("wrapwrite: SELECT target %d is not a ResTarget", idx)
		}
		wrapped, err := wrap(columns[j], rt.Val)
		if err != nil {
			return err
		}
		rt.Val = wrapped
	}
	return nil
}
