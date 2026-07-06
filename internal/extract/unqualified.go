package extract

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// UnqualifiedColumnRefs collects column names referenced without object.field form.
// INSERT column lists and UPDATE SET target names are excluded.
func UnqualifiedColumnRefs(tree *pg_query.ParseResult) []string {
	c := &unqualifiedCollector{}
	for _, stmt := range tree.Stmts {
		c.collectStmt(stmt.Stmt)
	}
	return dedupStrings(c.names)
}

type unqualifiedCollector struct {
	names []string
}

func (c *unqualifiedCollector) add(name string) {
	if name == "" || name == "*" {
		return
	}
	c.names = append(c.names, name)
}

func (c *unqualifiedCollector) collectStmt(node *pg_query.Node) {
	if node == nil {
		return
	}
	switch {
	case node.GetSelectStmt() != nil:
		c.collectSelect(node.GetSelectStmt())
	case node.GetInsertStmt() != nil:
		c.collectInsert(node.GetInsertStmt())
	case node.GetUpdateStmt() != nil:
		c.collectUpdate(node.GetUpdateStmt())
	case node.GetDeleteStmt() != nil:
		c.collectDelete(node.GetDeleteStmt())
	}
}

func (c *unqualifiedCollector) collectSelect(sel *pg_query.SelectStmt) {
	if sel == nil {
		return
	}
	if sel.WithClause != nil {
		for _, cte := range sel.WithClause.Ctes {
			if cte.GetCommonTableExpr() != nil {
				c.collectStmt(cte.GetCommonTableExpr().Ctequery)
			}
		}
	}
	switch sel.Op {
	case pg_query.SetOperation_SETOP_UNION, pg_query.SetOperation_SETOP_INTERSECT, pg_query.SetOperation_SETOP_EXCEPT:
		c.collectSelect(sel.Larg)
		c.collectSelect(sel.Rarg)
		return
	}
	for _, n := range sel.FromClause {
		c.collectFrom(n)
	}
	for _, t := range sel.TargetList {
		c.collectExpr(t)
	}
	for _, t := range sel.SortClause {
		c.collectExpr(t)
	}
	for _, t := range sel.GroupClause {
		c.collectExpr(t)
	}
	c.collectExpr(sel.HavingClause)
	c.collectExpr(sel.WhereClause)
}

func (c *unqualifiedCollector) collectInsert(ins *pg_query.InsertStmt) {
	if ins == nil {
		return
	}
	c.collectStmt(ins.SelectStmt)
	for _, t := range ins.ReturningList {
		c.collectExpr(t)
	}
}

func (c *unqualifiedCollector) collectUpdate(upd *pg_query.UpdateStmt) {
	if upd == nil {
		return
	}
	if upd.WithClause != nil {
		for _, cte := range upd.WithClause.Ctes {
			if cte.GetCommonTableExpr() != nil {
				c.collectStmt(cte.GetCommonTableExpr().Ctequery)
			}
		}
	}
	for _, n := range upd.FromClause {
		c.collectFrom(n)
	}
	for _, t := range upd.TargetList {
		// SET target name is rt.Name; only walk the value expression.
		if rt := t.GetResTarget(); rt != nil {
			c.collectExpr(rt.Val)
		}
	}
	c.collectExpr(upd.WhereClause)
	for _, t := range upd.ReturningList {
		c.collectExpr(t)
	}
}

func (c *unqualifiedCollector) collectDelete(del *pg_query.DeleteStmt) {
	if del == nil {
		return
	}
	if del.WithClause != nil {
		for _, cte := range del.WithClause.Ctes {
			if cte.GetCommonTableExpr() != nil {
				c.collectStmt(cte.GetCommonTableExpr().Ctequery)
			}
		}
	}
	for _, n := range del.UsingClause {
		c.collectFrom(n)
	}
	c.collectExpr(del.WhereClause)
	for _, t := range del.ReturningList {
		c.collectExpr(t)
	}
}

func (c *unqualifiedCollector) collectFrom(node *pg_query.Node) {
	if node == nil {
		return
	}
	switch {
	case node.GetJoinExpr() != nil:
		j := node.GetJoinExpr()
		c.collectFrom(j.Larg)
		c.collectFrom(j.Rarg)
		c.collectExpr(j.Quals)
		for _, u := range j.UsingClause {
			c.collectExpr(u)
		}
	case node.GetRangeSubselect() != nil:
		c.collectStmt(node.GetRangeSubselect().Subquery)
	}
}

func (c *unqualifiedCollector) collectExpr(node *pg_query.Node) {
	if node == nil {
		return
	}
	if cr := node.GetColumnRef(); cr != nil {
		parts := columnRefParts(cr)
		if len(parts) == 1 && parts[0] != "*" {
			c.add(parts[0])
		}
		return
	}
	switch {
	case node.GetResTarget() != nil:
		c.collectExpr(node.GetResTarget().Val)
	case node.GetAExpr() != nil:
		ae := node.GetAExpr()
		c.collectExpr(ae.Lexpr)
		c.collectExpr(ae.Rexpr)
	case node.GetBoolExpr() != nil:
		for _, arg := range node.GetBoolExpr().Args {
			c.collectExpr(arg)
		}
	case node.GetSubLink() != nil:
		sl := node.GetSubLink()
		c.collectExpr(sl.Testexpr)
		c.collectStmt(sl.Subselect)
	case node.GetJoinExpr() != nil:
		j := node.GetJoinExpr()
		c.collectExpr(j.Larg)
		c.collectExpr(j.Rarg)
		c.collectExpr(j.Quals)
	case node.GetRangeSubselect() != nil:
		c.collectStmt(node.GetRangeSubselect().Subquery)
	case node.GetSelectStmt() != nil:
		c.collectSelect(node.GetSelectStmt())
	case node.GetFuncCall() != nil:
		for _, arg := range node.GetFuncCall().Args {
			c.collectExpr(arg)
		}
		if fc := node.GetFuncCall(); fc.AggFilter != nil {
			c.collectExpr(fc.AggFilter)
		}
	case node.GetSortBy() != nil:
		c.collectExpr(node.GetSortBy().Node)
	case node.GetCoalesceExpr() != nil:
		for _, arg := range node.GetCoalesceExpr().Args {
			c.collectExpr(arg)
		}
	case node.GetRowExpr() != nil:
		for _, arg := range node.GetRowExpr().Args {
			c.collectExpr(arg)
		}
	case node.GetNullTest() != nil:
		c.collectExpr(node.GetNullTest().Arg)
	case node.GetBooleanTest() != nil:
		c.collectExpr(node.GetBooleanTest().Arg)
	case node.GetTypeCast() != nil:
		c.collectExpr(node.GetTypeCast().Arg)
	case node.GetCaseExpr() != nil:
		ce := node.GetCaseExpr()
		c.collectExpr(ce.Arg)
		for _, a := range ce.Args {
			c.collectExpr(a)
		}
		c.collectExpr(ce.Defresult)
	case node.GetCaseWhen() != nil:
		cw := node.GetCaseWhen()
		c.collectExpr(cw.Expr)
		c.collectExpr(cw.Result)
	case node.GetList() != nil:
		for _, item := range node.GetList().Items {
			c.collectExpr(item)
		}
	}
}

func dedupStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
