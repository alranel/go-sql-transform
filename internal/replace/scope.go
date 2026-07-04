package replace

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/alranel/go-sql-transform/internal/extract"
)

type replaceScope struct {
	parent   *replaceScope
	ctes     map[string]struct{}
	bindings map[string]extract.Name // qualifier -> physical table
}

func newReplaceScope(parent *replaceScope) *replaceScope {
	return &replaceScope{
		parent:   parent,
		ctes:     make(map[string]struct{}),
		bindings: make(map[string]extract.Name),
	}
}

func (s *replaceScope) inheritCTEs() {
	if s.parent == nil {
		return
	}
	for k := range s.parent.ctes {
		s.ctes[k] = struct{}{}
	}
}

func (s *replaceScope) registerCTEs(wc *pg_query.WithClause) {
	s.inheritCTEs()
	if wc == nil {
		return
	}
	for _, node := range wc.Ctes {
		cte := node.GetCommonTableExpr()
		if cte != nil {
			s.ctes[cte.Ctename] = struct{}{}
		}
	}
}

func (s *replaceScope) registerFromClause(from []*pg_query.Node) {
	s.inheritCTEs()
	for _, n := range from {
		s.registerFromItem(n)
	}
}

func (s *replaceScope) registerFromItem(node *pg_query.Node) {
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

func (s *replaceScope) bindRangeVar(rv *pg_query.RangeVar) {
	if rv == nil {
		return
	}
	tn := extract.Name{Schema: rv.Schemaname, Table: rv.Relname}
	if s.isCTE(rv.Relname) {
		name := rv.Relname
		if rv.Alias != nil && rv.Alias.Aliasname != "" {
			name = rv.Alias.Aliasname
		}
		s.bindings[name] = extract.Name{Table: rv.Relname}
		return
	}
	name := rv.Relname
	if rv.Alias != nil && rv.Alias.Aliasname != "" {
		name = rv.Alias.Aliasname
	}
	s.bindings[name] = tn
	if rv.Alias != nil && rv.Alias.Aliasname != "" {
		s.bindings[rv.Alias.Aliasname] = tn
	}
}

func (s *replaceScope) isCTE(name string) bool {
	_, ok := s.ctes[name]
	return ok
}

func (s *replaceScope) physicalTableForQualifier(qualifier string) string {
	if tn, ok := s.bindings[qualifier]; ok {
		return tn.Table
	}
	return ""
}
