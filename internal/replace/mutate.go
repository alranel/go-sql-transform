package replace

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/alranel/go-sql-transform/internal/extract"
)

// Name mirrors sqltransform.Name for replacement matching.
type Name = extract.Name

// Apply renames tables or columns in the parse tree.
func Apply(tree *pg_query.ParseResult, from, to Name) {
	for _, stmt := range tree.Stmts {
		applyStmt(stmt.Stmt, from, to)
	}
}

func applyStmt(node *pg_query.Node, from, to Name) {
	if node == nil {
		return
	}
	switch {
	case node.GetSelectStmt() != nil:
		applySelect(node.GetSelectStmt(), from, to)
	case node.GetInsertStmt() != nil:
		applyInsert(node.GetInsertStmt(), from, to)
	case node.GetUpdateStmt() != nil:
		applyUpdate(node.GetUpdateStmt(), from, to)
	case node.GetDeleteStmt() != nil:
		applyDelete(node.GetDeleteStmt(), from, to)
	}
}

func applySelect(sel *pg_query.SelectStmt, from, to Name) {
	if sel == nil {
		return
	}
	if sel.WithClause != nil {
		for _, cte := range sel.WithClause.Ctes {
			cteNode := cte.GetCommonTableExpr()
			if cteNode != nil {
				applySelect(cteNode.Ctequery.GetSelectStmt(), from, to)
			}
		}
	}
	switch sel.Op {
	case pg_query.SetOperation_SETOP_UNION, pg_query.SetOperation_SETOP_INTERSECT, pg_query.SetOperation_SETOP_EXCEPT:
		applySelect(sel.Larg, from, to)
		applySelect(sel.Rarg, from, to)
		return
	}

	sc := newReplaceScope(nil)
	sc.registerCTEs(sel.WithClause)
	sc.registerFromClause(sel.FromClause)

	for _, n := range sel.FromClause {
		applyFromItem(n, from, to, sc)
	}
	for _, n := range sel.TargetList {
		applyNodeWithScope(n, from, to, sc)
	}
	for _, n := range sel.SortClause {
		applyNodeWithScope(n, from, to, sc)
	}
	for _, n := range sel.GroupClause {
		applyNodeWithScope(n, from, to, sc)
	}
	applyNodeWithScope(sel.HavingClause, from, to, sc)
	applyNodeWithScope(sel.WhereClause, from, to, sc)
}

func applyInsert(ins *pg_query.InsertStmt, from, to Name) {
	if ins == nil {
		return
	}
	if from.Column == "" {
		applyRangeVar(ins.Relation, from, to, nil)
	} else {
		rel := ins.Relation
		tn := extract.Name{}
		if rel != nil {
			tn = extract.Name{Schema: rel.Schemaname, Table: rel.Relname}
		}
		for _, col := range ins.Cols {
			rt := col.GetResTarget()
			if rt != nil {
				applyResTargetName(rt, from, to, tn)
			}
		}
	}
	applyNode(ins.SelectStmt, from, to)
}

func applyUpdate(upd *pg_query.UpdateStmt, from, to Name) {
	if upd == nil {
		return
	}
	rel := upd.Relation
	tn := extract.Name{}
	if rel != nil {
		tn = extract.Name{Schema: rel.Schemaname, Table: rel.Relname}
	}
	if from.Column == "" {
		applyRangeVar(rel, from, to, nil)
	} else {
		for _, t := range upd.TargetList {
			rt := t.GetResTarget()
			if rt != nil {
				applyResTargetName(rt, from, to, tn)
			}
		}
	}
	sc := newReplaceScope(nil)
	if rel != nil {
		sc.bindRangeVar(rel)
	}
	applyNodeWithScope(upd.WhereClause, from, to, sc)
}

func applyDelete(del *pg_query.DeleteStmt, from, to Name) {
	if del == nil {
		return
	}
	if from.Column == "" {
		applyRangeVar(del.Relation, from, to, nil)
	}
	sc := newReplaceScope(nil)
	if rel := del.Relation; rel != nil {
		sc.bindRangeVar(rel)
	}
	applyNodeWithScope(del.WhereClause, from, to, sc)
}

func applyFromItem(node *pg_query.Node, from, to Name, sc *replaceScope) {
	if node == nil {
		return
	}
	switch {
	case node.GetRangeVar() != nil:
		applyRangeVar(node.GetRangeVar(), from, to, sc)
	case node.GetJoinExpr() != nil:
		j := node.GetJoinExpr()
		applyFromItem(j.Larg, from, to, sc)
		applyFromItem(j.Rarg, from, to, sc)
		applyNodeWithScope(j.Quals, from, to, sc)
	case node.GetRangeSubselect() != nil:
		applyNode(node.GetRangeSubselect().Subquery, from, to)
	}
}

func applyNode(node *pg_query.Node, from, to Name) {
	applyNodeWithScope(node, from, to, newReplaceScope(nil))
}

func applyNodeWithScope(node *pg_query.Node, from, to Name, sc *replaceScope) {
	if node == nil {
		return
	}
	switch {
	case node.GetColumnRef() != nil:
		if from.Column != "" {
			applyColumnRef(node.GetColumnRef(), from, to, sc)
		}
	case node.GetResTarget() != nil:
		rt := node.GetResTarget()
		applyNodeWithScope(rt.Val, from, to, sc)
	case node.GetAExpr() != nil:
		ae := node.GetAExpr()
		applyNodeWithScope(ae.Lexpr, from, to, sc)
		applyNodeWithScope(ae.Rexpr, from, to, sc)
	case node.GetBoolExpr() != nil:
		for _, arg := range node.GetBoolExpr().Args {
			applyNodeWithScope(arg, from, to, sc)
		}
	case node.GetSubLink() != nil:
		sl := node.GetSubLink()
		applyNodeWithScope(sl.Testexpr, from, to, sc)
		applyNode(sl.Subselect, from, to)
	case node.GetJoinExpr() != nil:
		j := node.GetJoinExpr()
		applyNodeWithScope(j.Larg, from, to, sc)
		applyNodeWithScope(j.Rarg, from, to, sc)
		applyNodeWithScope(j.Quals, from, to, sc)
	case node.GetRangeSubselect() != nil:
		applyNode(node.GetRangeSubselect().Subquery, from, to)
	case node.GetSelectStmt() != nil:
		applySelect(node.GetSelectStmt(), from, to)
	case node.GetFuncCall() != nil:
		for _, arg := range node.GetFuncCall().Args {
			applyNodeWithScope(arg, from, to, sc)
		}
	case node.GetSortBy() != nil:
		applyNodeWithScope(node.GetSortBy().Node, from, to, sc)
	case node.GetCoalesceExpr() != nil:
		for _, arg := range node.GetCoalesceExpr().Args {
			applyNodeWithScope(arg, from, to, sc)
		}
	case node.GetRowExpr() != nil:
		for _, arg := range node.GetRowExpr().Args {
			applyNodeWithScope(arg, from, to, sc)
		}
	case node.GetNullTest() != nil:
		applyNodeWithScope(node.GetNullTest().Arg, from, to, sc)
	case node.GetBooleanTest() != nil:
		applyNodeWithScope(node.GetBooleanTest().Arg, from, to, sc)
	case node.GetTypeCast() != nil:
		applyNodeWithScope(node.GetTypeCast().Arg, from, to, sc)
	case node.GetCaseExpr() != nil:
		ce := node.GetCaseExpr()
		applyNodeWithScope(ce.Arg, from, to, sc)
		for _, a := range ce.Args {
			applyNodeWithScope(a, from, to, sc)
		}
		applyNodeWithScope(ce.Defresult, from, to, sc)
	case node.GetCaseWhen() != nil:
		cw := node.GetCaseWhen()
		applyNodeWithScope(cw.Expr, from, to, sc)
		applyNodeWithScope(cw.Result, from, to, sc)
	case node.GetList() != nil:
		for _, item := range node.GetList().Items {
			applyNodeWithScope(item, from, to, sc)
		}
	}
}

func tableMatches(schema, table string, from Name) bool {
	if from.Table != "" && !equalFold(table, from.Table) {
		return false
	}
	if from.Schema != "" && !equalFold(schema, from.Schema) {
		return false
	}
	return from.Table != ""
}

func applyRangeVar(rv *pg_query.RangeVar, from, to Name, sc *replaceScope) {
	if rv == nil || from.Column != "" {
		return
	}
	if sc != nil && sc.isCTE(rv.Relname) {
		return
	}
	if !tableMatches(rv.Schemaname, rv.Relname, from) {
		return
	}
	if to.Schema != "" {
		rv.Schemaname = to.Schema
	}
	rv.Relname = to.Table
}

func applyResTargetName(rt *pg_query.ResTarget, from, to Name, table Name) {
	if from.Column == "" || rt.Name == "" {
		return
	}
	if !equalFold(rt.Name, from.Column) {
		return
	}
	if from.Table != "" && !equalFold(table.Table, from.Table) {
		return
	}
	if from.Schema != "" && !equalFold(table.Schema, from.Schema) {
		return
	}
	rt.Name = to.Column
}

func applyColumnRef(cr *pg_query.ColumnRef, from, to Name, sc *replaceScope) {
	parts := columnRefParts(cr)
	if len(parts) == 0 {
		return
	}
	var schema, table, column string
	switch len(parts) {
	case 1:
		column = parts[0]
	case 2:
		table, column = parts[0], parts[1]
	case 3:
		schema, table, column = parts[0], parts[1], parts[2]
	}
	if column == "*" || !equalFold(column, from.Column) {
		return
	}
	if from.Table != "" {
		if !qualifierMatchesTable(sc, schema, table, from) {
			return
		}
	}
	// rewrite
	if to.Column != "" {
		parts[len(parts)-1] = to.Column
	}
	if to.Table != "" && len(parts) >= 2 {
		parts[len(parts)-2] = to.Table
	}
	if to.Schema != "" && len(parts) == 3 {
		parts[0] = to.Schema
	}
	setColumnRefParts(cr, parts)
}

func qualifierMatchesTable(sc *replaceScope, schema, qualifier string, from Name) bool {
	if equalFold(qualifier, from.Table) {
		if from.Schema == "" || equalFold(schema, from.Schema) {
			return true
		}
	}
	if sc != nil {
		if phys := sc.physicalTableForQualifier(qualifier); phys != "" {
			if equalFold(phys, from.Table) {
				if from.Schema == "" || equalFold(schema, from.Schema) {
					return true
				}
			}
		}
	}
	return false
}

func columnRefParts(cr *pg_query.ColumnRef) []string {
	var parts []string
	for _, f := range cr.Fields {
		if str := f.GetString_(); str != nil {
			parts = append(parts, str.Sval)
		}
	}
	return parts
}

func setColumnRefParts(cr *pg_query.ColumnRef, parts []string) {
	for i, f := range cr.Fields {
		if str := f.GetString_(); str != nil && i < len(parts) {
			str.Sval = parts[i]
		}
	}
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
