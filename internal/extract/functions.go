package extract

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// FuncCall is a function invocation found in the query AST.
type FuncCall struct {
	Schema string
	Name   string
}

// FuncCalls collects all function invocations from a parse tree.
func FuncCalls(tree *pg_query.ParseResult) []FuncCall {
	var out []FuncCall
	for _, stmt := range tree.Stmts {
		collectFuncCalls(stmt.Stmt, &out)
	}
	return dedupFuncCalls(out)
}

func collectFuncCalls(node *pg_query.Node, out *[]FuncCall) {
	if node == nil {
		return
	}
	switch {
	case node.GetSelectStmt() != nil:
		collectFuncCallsInSelect(node.GetSelectStmt(), out)
	case node.GetInsertStmt() != nil:
		collectFuncCallsInInsert(node.GetInsertStmt(), out)
	case node.GetUpdateStmt() != nil:
		collectFuncCallsInUpdate(node.GetUpdateStmt(), out)
	case node.GetDeleteStmt() != nil:
		collectFuncCallsInDelete(node.GetDeleteStmt(), out)
	default:
		collectFuncCallsInExpr(node, out)
	}
}

func collectFuncCallsInSelect(sel *pg_query.SelectStmt, out *[]FuncCall) {
	if sel == nil {
		return
	}
	if sel.WithClause != nil {
		for _, cte := range sel.WithClause.Ctes {
			if cte.GetCommonTableExpr() != nil {
				collectFuncCalls(cte.GetCommonTableExpr().Ctequery, out)
			}
		}
	}
	switch sel.Op {
	case pg_query.SetOperation_SETOP_UNION, pg_query.SetOperation_SETOP_INTERSECT, pg_query.SetOperation_SETOP_EXCEPT:
		collectFuncCallsInSelect(sel.Larg, out)
		collectFuncCallsInSelect(sel.Rarg, out)
		return
	}
	for _, n := range sel.FromClause {
		collectFuncCallsInFrom(n, out)
	}
	for _, t := range sel.TargetList {
		collectFuncCallsInExpr(t, out)
	}
	for _, t := range sel.SortClause {
		collectFuncCallsInExpr(t, out)
	}
	for _, t := range sel.GroupClause {
		collectFuncCallsInExpr(t, out)
	}
	collectFuncCallsInExpr(sel.HavingClause, out)
	collectFuncCallsInExpr(sel.WhereClause, out)
}

func collectFuncCallsInInsert(ins *pg_query.InsertStmt, out *[]FuncCall) {
	if ins == nil {
		return
	}
	collectFuncCalls(ins.SelectStmt, out)
	for _, t := range ins.ReturningList {
		collectFuncCallsInExpr(t, out)
	}
}

func collectFuncCallsInUpdate(upd *pg_query.UpdateStmt, out *[]FuncCall) {
	if upd == nil {
		return
	}
	if upd.WithClause != nil {
		for _, cte := range upd.WithClause.Ctes {
			if cte.GetCommonTableExpr() != nil {
				collectFuncCalls(cte.GetCommonTableExpr().Ctequery, out)
			}
		}
	}
	for _, n := range upd.FromClause {
		collectFuncCallsInFrom(n, out)
	}
	for _, t := range upd.TargetList {
		collectFuncCallsInExpr(t, out)
	}
	collectFuncCallsInExpr(upd.WhereClause, out)
	for _, t := range upd.ReturningList {
		collectFuncCallsInExpr(t, out)
	}
}

func collectFuncCallsInDelete(del *pg_query.DeleteStmt, out *[]FuncCall) {
	if del == nil {
		return
	}
	if del.WithClause != nil {
		for _, cte := range del.WithClause.Ctes {
			if cte.GetCommonTableExpr() != nil {
				collectFuncCalls(cte.GetCommonTableExpr().Ctequery, out)
			}
		}
	}
	for _, n := range del.UsingClause {
		collectFuncCallsInFrom(n, out)
	}
	collectFuncCallsInExpr(del.WhereClause, out)
	for _, t := range del.ReturningList {
		collectFuncCallsInExpr(t, out)
	}
}

func collectFuncCallsInFrom(node *pg_query.Node, out *[]FuncCall) {
	if node == nil {
		return
	}
	switch {
	case node.GetJoinExpr() != nil:
		j := node.GetJoinExpr()
		collectFuncCallsInFrom(j.Larg, out)
		collectFuncCallsInFrom(j.Rarg, out)
		collectFuncCallsInExpr(j.Quals, out)
	case node.GetRangeSubselect() != nil:
		collectFuncCalls(node.GetRangeSubselect().Subquery, out)
	}
}

func collectFuncCallsInExpr(node *pg_query.Node, out *[]FuncCall) {
	if node == nil {
		return
	}
	if fc := node.GetFuncCall(); fc != nil {
		*out = append(*out, funcCallName(fc))
		for _, arg := range fc.Args {
			collectFuncCallsInExpr(arg, out)
		}
		if fc.AggFilter != nil {
			collectFuncCallsInExpr(fc.AggFilter, out)
		}
		return
	}
	switch {
	case node.GetResTarget() != nil:
		collectFuncCallsInExpr(node.GetResTarget().Val, out)
	case node.GetAExpr() != nil:
		ae := node.GetAExpr()
		collectFuncCallsInExpr(ae.Lexpr, out)
		collectFuncCallsInExpr(ae.Rexpr, out)
	case node.GetBoolExpr() != nil:
		for _, arg := range node.GetBoolExpr().Args {
			collectFuncCallsInExpr(arg, out)
		}
	case node.GetSubLink() != nil:
		sl := node.GetSubLink()
		collectFuncCallsInExpr(sl.Testexpr, out)
		collectFuncCalls(sl.Subselect, out)
	case node.GetJoinExpr() != nil:
		j := node.GetJoinExpr()
		collectFuncCallsInExpr(j.Larg, out)
		collectFuncCallsInExpr(j.Rarg, out)
		collectFuncCallsInExpr(j.Quals, out)
	case node.GetRangeSubselect() != nil:
		collectFuncCalls(node.GetRangeSubselect().Subquery, out)
	case node.GetSelectStmt() != nil:
		collectFuncCallsInSelect(node.GetSelectStmt(), out)
	case node.GetSortBy() != nil:
		collectFuncCallsInExpr(node.GetSortBy().Node, out)
	case node.GetCoalesceExpr() != nil:
		for _, arg := range node.GetCoalesceExpr().Args {
			collectFuncCallsInExpr(arg, out)
		}
	case node.GetRowExpr() != nil:
		for _, arg := range node.GetRowExpr().Args {
			collectFuncCallsInExpr(arg, out)
		}
	case node.GetNullTest() != nil:
		collectFuncCallsInExpr(node.GetNullTest().Arg, out)
	case node.GetBooleanTest() != nil:
		collectFuncCallsInExpr(node.GetBooleanTest().Arg, out)
	case node.GetTypeCast() != nil:
		collectFuncCallsInExpr(node.GetTypeCast().Arg, out)
	case node.GetCaseExpr() != nil:
		ce := node.GetCaseExpr()
		collectFuncCallsInExpr(ce.Arg, out)
		for _, a := range ce.Args {
			collectFuncCallsInExpr(a, out)
		}
		collectFuncCallsInExpr(ce.Defresult, out)
	case node.GetCaseWhen() != nil:
		cw := node.GetCaseWhen()
		collectFuncCallsInExpr(cw.Expr, out)
		collectFuncCallsInExpr(cw.Result, out)
	case node.GetList() != nil:
		for _, item := range node.GetList().Items {
			collectFuncCallsInExpr(item, out)
		}
	}
}

func funcCallName(fc *pg_query.FuncCall) FuncCall {
	var parts []string
	for _, n := range fc.Funcname {
		if s := n.GetString_(); s != nil {
			parts = append(parts, s.Sval)
		}
	}
	switch len(parts) {
	case 0:
		return FuncCall{}
	case 1:
		return FuncCall{Name: parts[0]}
	default:
		return FuncCall{Schema: parts[0], Name: parts[len(parts)-1]}
	}
}

func dedupFuncCalls(in []FuncCall) []FuncCall {
	seen := make(map[string]struct{}, len(in))
	out := make([]FuncCall, 0, len(in))
	for _, fc := range in {
		k := fc.Schema + "\x00" + fc.Name
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, fc)
	}
	return out
}
