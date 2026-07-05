package expandstar

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/alranel/go-sql-transform/internal/extract"
)

// ExpandFunc returns the columns to substitute for table.* in a SELECT target list.
// Return an error to reject expansion (e.g. unreadable columns are omitted by returning a filtered list).
type ExpandFunc func(table extract.Name) ([]extract.Name, error)

// Apply replaces SELECT * / table.* in projection and RETURNING lists with explicit columns.
func Apply(tree *pg_query.ParseResult, expand ExpandFunc) error {
	for _, stmt := range tree.Stmts {
		if err := applyStmt(stmt.Stmt, expand); err != nil {
			return err
		}
	}
	return nil
}

func applyStmt(node *pg_query.Node, expand ExpandFunc) error {
	if node == nil {
		return nil
	}
	switch {
	case node.GetSelectStmt() != nil:
		return applySelect(node.GetSelectStmt(), expand)
	case node.GetInsertStmt() != nil:
		return applyInsert(node.GetInsertStmt(), expand)
	case node.GetUpdateStmt() != nil:
		return applyUpdate(node.GetUpdateStmt(), expand)
	case node.GetDeleteStmt() != nil:
		return applyDelete(node.GetDeleteStmt(), expand)
	}
	return nil
}

func applySelect(sel *pg_query.SelectStmt, expand ExpandFunc) error {
	if sel == nil {
		return nil
	}
	if sel.WithClause != nil {
		for _, cte := range sel.WithClause.Ctes {
			if cte.GetCommonTableExpr() != nil {
				if err := applyStmt(cte.GetCommonTableExpr().Ctequery, expand); err != nil {
					return err
				}
			}
		}
	}
	switch sel.Op {
	case pg_query.SetOperation_SETOP_UNION, pg_query.SetOperation_SETOP_INTERSECT, pg_query.SetOperation_SETOP_EXCEPT:
		if err := applySelect(sel.Larg, expand); err != nil {
			return err
		}
		return applySelect(sel.Rarg, expand)
	}

	sc := extract.NewScopeForSelect(sel)
	newTargets, err := expandTargetList(sel.TargetList, sc, expand)
	if err != nil {
		return err
	}
	sel.TargetList = newTargets
	return nil
}

func applyInsert(ins *pg_query.InsertStmt, expand ExpandFunc) error {
	if ins == nil {
		return nil
	}
	if err := applyStmt(ins.SelectStmt, expand); err != nil {
		return err
	}
	sc := extract.NewScopeForInsert(ins)
	newTargets, err := expandTargetList(ins.ReturningList, sc, expand)
	if err != nil {
		return err
	}
	ins.ReturningList = newTargets
	return nil
}

func applyUpdate(upd *pg_query.UpdateStmt, expand ExpandFunc) error {
	if upd == nil {
		return nil
	}
	if upd.WithClause != nil {
		for _, cte := range upd.WithClause.Ctes {
			if cte.GetCommonTableExpr() != nil {
				if err := applyStmt(cte.GetCommonTableExpr().Ctequery, expand); err != nil {
					return err
				}
			}
		}
	}
	sc := extract.NewScopeForUpdate(upd)
	newTargets, err := expandTargetList(upd.ReturningList, sc, expand)
	if err != nil {
		return err
	}
	upd.ReturningList = newTargets
	return nil
}

func applyDelete(del *pg_query.DeleteStmt, expand ExpandFunc) error {
	if del == nil {
		return nil
	}
	if del.WithClause != nil {
		for _, cte := range del.WithClause.Ctes {
			if cte.GetCommonTableExpr() != nil {
				if err := applyStmt(cte.GetCommonTableExpr().Ctequery, expand); err != nil {
					return err
				}
			}
		}
	}
	sc := extract.NewScopeForDelete(del)
	newTargets, err := expandTargetList(del.ReturningList, sc, expand)
	if err != nil {
		return err
	}
	del.ReturningList = newTargets
	return nil
}

func expandTargetList(targets []*pg_query.Node, sc *extract.Scope, expand ExpandFunc) ([]*pg_query.Node, error) {
	if len(targets) == 0 {
		return targets, nil
	}
	out := make([]*pg_query.Node, 0, len(targets))
	for _, t := range targets {
		expanded, err := expandResTarget(t, sc, expand)
		if err != nil {
			return nil, err
		}
		out = append(out, expanded...)
	}
	return out, nil
}

func expandResTarget(node *pg_query.Node, sc *extract.Scope, expand ExpandFunc) ([]*pg_query.Node, error) {
	rt := node.GetResTarget()
	if rt == nil {
		return []*pg_query.Node{node}, nil
	}
	stars := sc.ResolveStarFromVal(rt.Val)
	if len(stars) == 0 {
		return []*pg_query.Node{node}, nil
	}
	var out []*pg_query.Node
	for _, star := range stars {
		cols, err := expand(star)
		if err != nil {
			return nil, err
		}
		if len(cols) == 0 {
			return nil, fmt.Errorf("expandstar: no columns for %s", star.Table)
		}
		for _, col := range cols {
			out = append(out, makeResTarget(col, rt.Name))
		}
	}
	return out, nil
}

func makeResTarget(col extract.Name, alias string) *pg_query.Node {
	label := alias
	if label == "" {
		label = col.Column
	}
	return &pg_query.Node{
		Node: &pg_query.Node_ResTarget{
			ResTarget: &pg_query.ResTarget{
				Name: label,
				Val:  makeColumnRef(col),
			},
		},
	}
}

func makeColumnRef(col extract.Name) *pg_query.Node {
	var fields []*pg_query.Node
	if col.Schema != "" {
		fields = append(fields, stringNode(col.Schema))
	}
	if col.Table != "" {
		fields = append(fields, stringNode(col.Table))
	}
	fields = append(fields, stringNode(col.Column))
	return &pg_query.Node{
		Node: &pg_query.Node_ColumnRef{
			ColumnRef: &pg_query.ColumnRef{Fields: fields},
		},
	}
}

func stringNode(s string) *pg_query.Node {
	return &pg_query.Node{
		Node: &pg_query.Node_String_{
			String_: &pg_query.String{Sval: s},
		},
	}
}
