package extract

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type bindingKind int

const (
	bindingPhysical bindingKind = iota
	bindingCTE
)

type binding struct {
	kind     bindingKind
	alias    string
	table    Name // physical table for bindingPhysical
	cteName  string
}

type cteInfo struct {
	name        string
	outputCols  map[string]Name // output column label -> physical ref
}

type scope struct {
	parent   *scope
	ctes     map[string]*cteInfo
	bindings map[string]*binding // key: alias or table name used in SQL
	physical []Name              // physical tables in FROM for unqualified resolution
}

func newScope(parent *scope) *scope {
	return &scope{
		parent:   parent,
		ctes:     make(map[string]*cteInfo),
		bindings: make(map[string]*binding),
	}
}

func (s *scope) inheritCTEs() {
	if s.parent == nil {
		return
	}
	for k, v := range s.parent.ctes {
		s.ctes[k] = v
	}
}

func (s *scope) registerCTEs(wc *pg_query.WithClause) {
	s.inheritCTEs()
	if wc == nil {
		return
	}
	for _, node := range wc.Ctes {
		cte := node.GetCommonTableExpr()
		if cte == nil {
			continue
		}
		info := &cteInfo{
			name:       cte.Ctename,
			outputCols: make(map[string]Name),
		}
		inner := newScope(s)
		inner.inheritCTEs()
		if sel := cte.Ctequery.GetSelectStmt(); sel != nil {
			inner.registerFromClause(sel.FromClause)
			info.outputCols = buildCTEOutputCols(sel, inner)
		}
		s.ctes[cte.Ctename] = info
	}
}

func buildCTEOutputCols(sel *pg_query.SelectStmt, sc *scope) map[string]Name {
	out := make(map[string]Name)
	for _, t := range sel.TargetList {
		rt := t.GetResTarget()
		if rt == nil {
			continue
		}
		label := rt.Name
		if label == "" {
			label = inferColumnLabel(rt.Val)
		}
		if label == "" {
			continue
		}
		for _, n := range sc.resolveColumnRefFromVal(rt.Val) {
			out[label] = n
			break
		}
	}
	return out
}

func inferColumnLabel(val *pg_query.Node) string {
	if val == nil {
		return ""
	}
	cr := val.GetColumnRef()
	if cr == nil {
		return ""
	}
	parts := columnRefParts(cr)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func (s *scope) registerFromClause(from []*pg_query.Node) {
	s.inheritCTEs()
	for _, n := range from {
		s.registerFromItem(n)
	}
}

func (s *scope) registerFromItem(node *pg_query.Node) {
	if node == nil {
		return
	}
	switch {
	case node.GetRangeVar() != nil:
		s.bindRangeVar(node.GetRangeVar())
	case node.GetJoinExpr() != nil:
		j := node.GetJoinExpr()
		s.registerFromItem(j.Larg)
		s.registerFromItem(j.Rarg)
	}
}

func (s *scope) bindRangeVar(rv *pg_query.RangeVar) {
	if rv == nil {
		return
	}
	tn := physicalTableName(rv)
	if s.isCTE(tn.Table) {
		name := rv.Relname
		if rv.Alias != nil && rv.Alias.Aliasname != "" {
			name = rv.Alias.Aliasname
		}
		s.bindings[name] = &binding{
			kind:    bindingCTE,
			alias:   name,
			cteName: rv.Relname,
		}
		if rv.Alias != nil && rv.Alias.Aliasname != "" && rv.Alias.Aliasname != name {
			s.bindings[rv.Alias.Aliasname] = s.bindings[name]
		}
		return
	}
	name := rv.Relname
	if rv.Alias != nil && rv.Alias.Aliasname != "" {
		name = rv.Alias.Aliasname
	}
	b := &binding{
		kind:  bindingPhysical,
		alias: name,
		table: tn,
	}
	s.bindings[name] = b
	s.physical = append(s.physical, tn)
	if rv.Alias != nil && rv.Alias.Aliasname != "" {
		s.bindings[rv.Alias.Aliasname] = b
	}
	if rv.Alias == nil || rv.Alias.Aliasname == "" {
		// also bind by physical table name
		s.bindings[rv.Relname] = b
	}
}

func (s *scope) isCTE(name string) bool {
	_, ok := s.ctes[name]
	return ok
}

func physicalTableName(rv *pg_query.RangeVar) Name {
	return Name{Schema: rv.Schemaname, Table: rv.Relname}
}

func columnRefParts(cr *pg_query.ColumnRef) []string {
	var parts []string
	for _, f := range cr.Fields {
		if str := f.GetString_(); str != nil {
			parts = append(parts, str.Sval)
		} else if f.GetAStar() != nil {
			parts = append(parts, "*")
		}
	}
	return parts
}

func (s *scope) resolveColumnRef(cr *pg_query.ColumnRef) []Name {
	return s.resolveColumnRefFromVal(&pg_query.Node{Node: &pg_query.Node_ColumnRef{ColumnRef: cr}})
}

func (s *scope) resolveColumnRefFromVal(node *pg_query.Node) []Name {
	cr := node.GetColumnRef()
	if cr == nil {
		return nil
	}
	parts := columnRefParts(cr)
	if len(parts) == 0 {
		return nil
	}
	if parts[0] == "*" || (len(parts) == 2 && parts[1] == "*") {
		return s.resolveStar(parts)
	}
	return []Name{s.resolveParts(parts)}
}

func (s *scope) resolveStar(parts []string) []Name {
	var qualifier string
	if len(parts) == 2 {
		qualifier = parts[0]
	}
	if qualifier != "" {
		if n := s.resolveQualified(qualifier, "*"); n.Table != "" || n.Column == "*" {
			n.Column = "*"
			return []Name{n}
		}
		return nil
	}
	out := make([]Name, 0, len(s.physical))
	seen := make(map[string]struct{})
	for _, t := range s.physical {
		k := t.Schema + "\x00" + t.Table
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, Name{Schema: t.Schema, Table: t.Table, Column: "*"})
	}
	return out
}

func (s *scope) resolveParts(parts []string) Name {
	switch len(parts) {
	case 1:
		return s.resolveUnqualified(parts[0])
	case 2:
		return s.resolveQualified(parts[0], parts[1])
	case 3:
		return s.resolveSchemaQualified(parts[0], parts[1], parts[2])
	default:
		return Name{}
	}
}

func (s *scope) resolveUnqualified(column string) Name {
	if len(s.physical) == 1 {
		t := s.physical[0]
		return Name{Schema: t.Schema, Table: t.Table, Column: column}
	}
	if len(s.physical) == 0 && column != "" {
		var resolved Name
		cteMatches := 0
		seenCTE := make(map[string]struct{})
		for _, b := range s.bindings {
			if b.kind != bindingCTE {
				continue
			}
			if _, ok := seenCTE[b.cteName]; ok {
				continue
			}
			seenCTE[b.cteName] = struct{}{}
			if cte, ok := s.ctes[b.cteName]; ok {
				if col, ok := cte.outputCols[column]; ok {
					cteMatches++
					resolved = col
				}
			}
		}
		if cteMatches == 1 {
			return resolved
		}
	}
	if len(s.physical) > 1 {
		return Name{Column: column}
	}
	return Name{Column: column}
}

func (s *scope) resolveQualified(qualifier, column string) Name {
	if b, ok := s.bindings[qualifier]; ok {
		switch b.kind {
		case bindingPhysical:
			return Name{Schema: b.table.Schema, Table: b.table.Table, Column: column}
		case bindingCTE:
			if cte, ok := s.ctes[b.cteName]; ok {
				if col, ok := cte.outputCols[column]; ok {
					return col
				}
			}
		}
	}
	if cte, ok := s.ctes[qualifier]; ok {
		if col, ok := cte.outputCols[column]; ok {
			return col
		}
	}
	// qualifier may be physical table name directly
	for _, t := range s.physical {
		if t.Table == qualifier {
			return Name{Schema: t.Schema, Table: t.Table, Column: column}
		}
	}
	return Name{Table: qualifier, Column: column}
}

func (s *scope) resolveSchemaQualified(schema, table, column string) Name {
	for _, t := range s.physical {
		if t.Schema == schema && t.Table == table {
			return Name{Schema: schema, Table: table, Column: column}
		}
	}
	return Name{Schema: schema, Table: table, Column: column}
}

// Scope exposes read-only scope helpers for AST transforms.
type Scope struct {
	s *scope
}

// ResolveStarFromVal returns table.* references when val is a SELECT * projection.
func (s *Scope) ResolveStarFromVal(val *pg_query.Node) []Name {
	if val == nil || s == nil || s.s == nil {
		return nil
	}
	cr := val.GetColumnRef()
	if cr == nil {
		return nil
	}
	parts := columnRefParts(cr)
	if len(parts) == 0 {
		return nil
	}
	if parts[0] == "*" || (len(parts) == 2 && parts[1] == "*") {
		return s.s.resolveStar(parts)
	}
	return nil
}

// NewScopeForSelect builds a scope for a SELECT statement target list.
func NewScopeForSelect(sel *pg_query.SelectStmt) *Scope {
	sc := newScope(nil)
	if sel == nil {
		return &Scope{s: sc}
	}
	sc.registerCTEs(sel.WithClause)
	sc.registerFromClause(sel.FromClause)
	return &Scope{s: sc}
}

// NewScopeForInsert builds a scope for INSERT RETURNING.
func NewScopeForInsert(ins *pg_query.InsertStmt) *Scope {
	sc := newScope(nil)
	if ins != nil && ins.Relation != nil {
		sc.bindRangeVar(ins.Relation)
	}
	return &Scope{s: sc}
}

// NewScopeForUpdate builds a scope for UPDATE RETURNING.
func NewScopeForUpdate(upd *pg_query.UpdateStmt) *Scope {
	sc := newScope(nil)
	if upd == nil {
		return &Scope{s: sc}
	}
	sc.registerCTEs(upd.WithClause)
	if upd.Relation != nil {
		sc.bindRangeVar(upd.Relation)
	}
	sc.registerFromClause(upd.FromClause)
	return &Scope{s: sc}
}

// NewScopeForDelete builds a scope for DELETE RETURNING.
func NewScopeForDelete(del *pg_query.DeleteStmt) *Scope {
	sc := newScope(nil)
	if del == nil {
		return &Scope{s: sc}
	}
	sc.registerCTEs(del.WithClause)
	if del.Relation != nil {
		sc.bindRangeVar(del.Relation)
	}
	for _, n := range del.UsingClause {
		sc.registerFromItem(n)
	}
	return &Scope{s: sc}
}
