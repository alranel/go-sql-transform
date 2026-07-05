package extract

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// ReadRefs collects physical read references from a parse tree.
func ReadRefs(tree *pg_query.ParseResult) []Name {
	c := &collector{mode: modeRead}
	for _, stmt := range tree.Stmts {
		c.collectStmt(stmt.Stmt, nil)
	}
	return c.refs
}

// WriteRefs collects physical write references from a parse tree.
func WriteRefs(tree *pg_query.ParseResult) []Name {
	c := &collector{mode: modeWrite}
	for _, stmt := range tree.Stmts {
		c.collectStmt(stmt.Stmt, nil)
	}
	return c.refs
}

type collectMode int

const (
	modeRead collectMode = iota
	modeWrite
)

type collector struct {
	mode collectMode
	refs []Name
}

func (c *collector) add(n Name) {
	c.refs = append(c.refs, n)
}

func (c *collector) collectStmt(node *pg_query.Node, parentScope *scope) {
	if node == nil {
		return
	}
	switch {
	case node.GetSelectStmt() != nil:
		c.collectSelect(node.GetSelectStmt(), parentScope)
	case node.GetInsertStmt() != nil:
		c.collectInsert(node.GetInsertStmt(), parentScope)
	case node.GetUpdateStmt() != nil:
		c.collectUpdate(node.GetUpdateStmt(), parentScope)
	case node.GetDeleteStmt() != nil:
		c.collectDelete(node.GetDeleteStmt(), parentScope)
	}
}

func (c *collector) collectSelect(sel *pg_query.SelectStmt, parentScope *scope) {
	if sel == nil {
		return
	}
	switch sel.Op {
	case pg_query.SetOperation_SETOP_UNION, pg_query.SetOperation_SETOP_INTERSECT, pg_query.SetOperation_SETOP_EXCEPT:
		if sel.Larg != nil {
			c.collectSelect(sel.Larg, parentScope)
		}
		if sel.Rarg != nil {
			c.collectSelect(sel.Rarg, parentScope)
		}
		return
	}

	sc := newScope(parentScope)
	sc.registerCTEs(sel.WithClause)
	c.collectCTEBodies(sel.WithClause, parentScope)
	sc.registerFromClause(sel.FromClause)
	for _, n := range sel.FromClause {
		c.collectFromExprs(n, sc)
	}

	prevMode := c.mode
	if c.mode != modeWrite {
		c.mode = modeRead
	}
	for _, t := range sel.TargetList {
		c.collectNode(t, sc)
	}
	for _, t := range sel.SortClause {
		c.collectNode(t, sc)
	}
	for _, t := range sel.GroupClause {
		c.collectNode(t, sc)
	}
	c.collectNode(sel.HavingClause, sc)
	c.collectNode(sel.WhereClause, sc)
	c.mode = prevMode
}

func (c *collector) collectInsert(ins *pg_query.InsertStmt, _ *scope) {
	if ins == nil {
		return
	}
	rel := ins.Relation
	if rel != nil && c.mode == modeWrite {
		tn := physicalTableName(rel)
		if len(ins.Cols) == 0 {
			c.add(Name{Schema: tn.Schema, Table: tn.Table})
		}
		for _, col := range ins.Cols {
			rt := col.GetResTarget()
			if rt != nil && rt.Name != "" {
				c.add(Name{Schema: tn.Schema, Table: tn.Table, Column: rt.Name})
			}
		}
	}
	if ins.SelectStmt != nil && c.mode == modeRead {
		c.collectStmt(ins.SelectStmt, nil)
	}
}

func (c *collector) collectUpdate(upd *pg_query.UpdateStmt, parentScope *scope) {
	if upd == nil {
		return
	}
	rel := upd.Relation
	if rel != nil && c.mode == modeWrite {
		tn := physicalTableName(rel)
		for _, t := range upd.TargetList {
			rt := t.GetResTarget()
			if rt != nil && rt.Name != "" {
				c.add(Name{Schema: tn.Schema, Table: tn.Table, Column: rt.Name})
			}
		}
	}
	if c.mode == modeRead {
		sc := newScope(parentScope)
		sc.registerCTEs(upd.WithClause)
		c.collectCTEBodies(upd.WithClause, parentScope)
		if rel != nil {
			sc.bindRangeVar(rel)
		}
		sc.registerFromClause(upd.FromClause)
		for _, n := range upd.FromClause {
			c.collectFromExprs(n, sc)
		}
		for _, t := range upd.TargetList {
			c.collectNode(t, sc)
		}
		c.collectNode(upd.WhereClause, sc)
		for _, t := range upd.ReturningList {
			c.collectNode(t, sc)
		}
	}
}

func (c *collector) collectDelete(del *pg_query.DeleteStmt, parentScope *scope) {
	if del == nil {
		return
	}
	rel := del.Relation
	if rel != nil && c.mode == modeWrite {
		tn := physicalTableName(rel)
		c.add(Name{Schema: tn.Schema, Table: tn.Table})
	}
	if c.mode == modeRead {
		sc := newScope(parentScope)
		sc.registerCTEs(del.WithClause)
		c.collectCTEBodies(del.WithClause, parentScope)
		if rel != nil {
			sc.bindRangeVar(rel)
		}
		for _, n := range del.UsingClause {
			sc.registerFromItem(n)
			c.collectFromExprs(n, sc)
		}
		c.collectNode(del.WhereClause, sc)
		for _, t := range del.ReturningList {
			c.collectNode(t, sc)
		}
	}
}

func (c *collector) collectNode(node *pg_query.Node, sc *scope) {
	if node == nil {
		return
	}
	switch {
	case node.GetColumnRef() != nil:
		if c.mode == modeRead {
			for _, n := range sc.resolveColumnRef(node.GetColumnRef()) {
				c.add(n)
			}
		}
	case node.GetResTarget() != nil:
		c.collectNode(node.GetResTarget().Val, sc)
	case node.GetAExpr() != nil:
		ae := node.GetAExpr()
		c.collectNode(ae.Lexpr, sc)
		c.collectNode(ae.Rexpr, sc)
	case node.GetBoolExpr() != nil:
		for _, arg := range node.GetBoolExpr().Args {
			c.collectNode(arg, sc)
		}
	case node.GetSubLink() != nil:
		sl := node.GetSubLink()
		c.collectNode(sl.Testexpr, sc)
		subScope := newScope(sc)
		c.collectSelect(sl.Subselect.GetSelectStmt(), subScope)
	case node.GetJoinExpr() != nil:
		j := node.GetJoinExpr()
		c.collectNode(j.Larg, sc)
		c.collectNode(j.Rarg, sc)
		c.collectNode(j.Quals, sc)
	case node.GetRangeSubselect() != nil:
		rs := node.GetRangeSubselect()
		c.collectSelect(rs.Subquery.GetSelectStmt(), sc.parent)
	case node.GetSelectStmt() != nil:
		c.collectSelect(node.GetSelectStmt(), sc.parent)
	case node.GetFuncCall() != nil:
		for _, arg := range node.GetFuncCall().Args {
			c.collectNode(arg, sc)
		}
	case node.GetSortBy() != nil:
		c.collectNode(node.GetSortBy().Node, sc)
	case node.GetCoalesceExpr() != nil:
		for _, arg := range node.GetCoalesceExpr().Args {
			c.collectNode(arg, sc)
		}
	case node.GetRowExpr() != nil:
		for _, arg := range node.GetRowExpr().Args {
			c.collectNode(arg, sc)
		}
	case node.GetNullTest() != nil:
		c.collectNode(node.GetNullTest().Arg, sc)
	case node.GetBooleanTest() != nil:
		c.collectNode(node.GetBooleanTest().Arg, sc)
	case node.GetTypeCast() != nil:
		c.collectNode(node.GetTypeCast().Arg, sc)
	case node.GetCaseExpr() != nil:
		ce := node.GetCaseExpr()
		c.collectNode(ce.Arg, sc)
		for _, a := range ce.Args {
			c.collectNode(a, sc)
		}
		c.collectNode(ce.Defresult, sc)
	case node.GetCaseWhen() != nil:
		cw := node.GetCaseWhen()
		c.collectNode(cw.Expr, sc)
		c.collectNode(cw.Result, sc)
	case node.GetList() != nil:
		for _, item := range node.GetList().Items {
			c.collectNode(item, sc)
		}
	}
}

func (c *collector) collectCTEBodies(wc *pg_query.WithClause, parentScope *scope) {
	if wc == nil || c.mode != modeRead {
		return
	}
	for _, node := range wc.Ctes {
		cte := node.GetCommonTableExpr()
		if cte == nil {
			continue
		}
		c.collectSelect(cte.Ctequery.GetSelectStmt(), parentScope)
	}
}

func (c *collector) collectFromExprs(node *pg_query.Node, sc *scope) {
	if node == nil || c.mode != modeRead {
		return
	}
	switch {
	case node.GetJoinExpr() != nil:
		j := node.GetJoinExpr()
		c.collectFromExprs(j.Larg, sc)
		c.collectFromExprs(j.Rarg, sc)
		c.collectNode(j.Quals, sc)
		for _, u := range j.UsingClause {
			c.collectNode(u, sc)
		}
	case node.GetRangeSubselect() != nil:
		rs := node.GetRangeSubselect()
		c.collectSelect(rs.Subquery.GetSelectStmt(), sc.parent)
	}
}
