package walk

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// Visitor is called for each node in the AST. Return false to prune children.
type Visitor func(node *pg_query.Node) bool

// Node walks a single node and its descendants depth-first.
func Node(root *pg_query.Node, visit Visitor) {
	if root == nil {
		return
	}
	if !visit(root) {
		return
	}
	walkChildren(root, visit)
}

func walkChildren(node *pg_query.Node, visit Visitor) {
	switch {
	case node.GetSelectStmt() != nil:
		walkSelect(node.GetSelectStmt(), visit)
	case node.GetInsertStmt() != nil:
		walkInsert(node.GetInsertStmt(), visit)
	case node.GetUpdateStmt() != nil:
		walkUpdate(node.GetUpdateStmt(), visit)
	case node.GetDeleteStmt() != nil:
		walkDelete(node.GetDeleteStmt(), visit)
	case node.GetJoinExpr() != nil:
		walkJoin(node.GetJoinExpr(), visit)
	case node.GetRangeSubselect() != nil:
		rs := node.GetRangeSubselect()
		Node(rs.Subquery, visit)
	case node.GetSubLink() != nil:
		sl := node.GetSubLink()
		Node(sl.Subselect, visit)
		Node(sl.Testexpr, visit)
	case node.GetAExpr() != nil:
		ae := node.GetAExpr()
		Node(ae.Lexpr, visit)
		Node(ae.Rexpr, visit)
	case node.GetBoolExpr() != nil:
		for _, arg := range node.GetBoolExpr().Args {
			Node(arg, visit)
		}
	case node.GetColumnRef() != nil:
		// leaf
	case node.GetFuncCall() != nil:
		for _, arg := range node.GetFuncCall().Args {
			Node(arg, visit)
		}
	case node.GetResTarget() != nil:
		Node(node.GetResTarget().Val, visit)
	case node.GetSortBy() != nil:
		Node(node.GetSortBy().Node, visit)
	case node.GetCoalesceExpr() != nil:
		for _, arg := range node.GetCoalesceExpr().Args {
			Node(arg, visit)
		}
	case node.GetRowExpr() != nil:
		for _, arg := range node.GetRowExpr().Args {
			Node(arg, visit)
		}
	case node.GetNullTest() != nil:
		Node(node.GetNullTest().Arg, visit)
	case node.GetBooleanTest() != nil:
		Node(node.GetBooleanTest().Arg, visit)
	case node.GetTypeCast() != nil:
		Node(node.GetTypeCast().Arg, visit)
	case node.GetCaseExpr() != nil:
		ce := node.GetCaseExpr()
		Node(ce.Arg, visit)
		for _, a := range ce.Args {
			Node(a, visit)
		}
		Node(ce.Defresult, visit)
	case node.GetCaseWhen() != nil:
		cw := node.GetCaseWhen()
		Node(cw.Expr, visit)
		Node(cw.Result, visit)
	case node.GetList() != nil:
		for _, item := range node.GetList().Items {
			Node(item, visit)
		}
	}
}

func walkSelect(sel *pg_query.SelectStmt, visit Visitor) {
	if sel == nil {
		return
	}
	if sel.WithClause != nil {
		for _, cte := range sel.WithClause.Ctes {
			Node(cte, visit)
		}
	}
	switch sel.Op {
	case pg_query.SetOperation_SETOP_NONE:
		for _, n := range sel.FromClause {
			Node(n, visit)
		}
		for _, n := range sel.TargetList {
			Node(n, visit)
		}
		Node(sel.WhereClause, visit)
		for _, n := range sel.GroupClause {
			Node(n, visit)
		}
		Node(sel.HavingClause, visit)
		for _, n := range sel.SortClause {
			Node(n, visit)
		}
	case pg_query.SetOperation_SETOP_UNION, pg_query.SetOperation_SETOP_INTERSECT, pg_query.SetOperation_SETOP_EXCEPT:
		walkSelect(sel.Larg, visit)
		walkSelect(sel.Rarg, visit)
	}
}

func walkInsert(ins *pg_query.InsertStmt, visit Visitor) {
	if ins.Relation != nil {
		visit(&pg_query.Node{Node: &pg_query.Node_RangeVar{RangeVar: ins.Relation}})
	}
	for _, c := range ins.Cols {
		Node(c, visit)
	}
	Node(ins.SelectStmt, visit)
}

func walkUpdate(upd *pg_query.UpdateStmt, visit Visitor) {
	if upd.Relation != nil {
		visit(&pg_query.Node{Node: &pg_query.Node_RangeVar{RangeVar: upd.Relation}})
	}
	for _, t := range upd.TargetList {
		Node(t, visit)
	}
	Node(upd.WhereClause, visit)
}

func walkDelete(del *pg_query.DeleteStmt, visit Visitor) {
	if del.Relation != nil {
		visit(&pg_query.Node{Node: &pg_query.Node_RangeVar{RangeVar: del.Relation}})
	}
	Node(del.WhereClause, visit)
}

func walkJoin(j *pg_query.JoinExpr, visit Visitor) {
	Node(j.Larg, visit)
	Node(j.Rarg, visit)
	Node(j.Quals, visit)
}

// ParseResult walks all statements in a parse result.
func ParseResult(tree *pg_query.ParseResult, visit Visitor) {
	for _, stmt := range tree.Stmts {
		Node(stmt.Stmt, visit)
	}
}
