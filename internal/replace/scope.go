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
	case node.GetRangeFunction() != nil:
		s.bindRangeFunction(node.GetRangeFunction())
	case node.GetRangeSubselect() != nil:
		s.bindRangeSubselect(node.GetRangeSubselect())
	}
}

func (s *replaceScope) bindRangeFunction(rf *pg_query.RangeFunction) {
	if rf == nil || rf.Alias == nil || rf.Alias.Aliasname == "" {
		return
	}
	// Virtual relation: bind alias to itself so column rewrites skip it.
	s.bindings[rf.Alias.Aliasname] = extract.Name{Table: rf.Alias.Aliasname}
}

func (s *replaceScope) bindRangeSubselect(rs *pg_query.RangeSubselect) {
	if rs == nil || rs.Alias == nil || rs.Alias.Aliasname == "" {
		return
	}
	s.bindings[rs.Alias.Aliasname] = extract.Name{Table: rs.Alias.Aliasname}
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
	for cur := s; cur != nil; cur = cur.parent {
		if _, ok := cur.ctes[name]; ok {
			return true
		}
	}
	return false
}

func (s *replaceScope) physicalTableForQualifier(qualifier string) string {
	for cur := s; cur != nil; cur = cur.parent {
		if tn, ok := cur.bindings[qualifier]; ok {
			return tn.Table
		}
	}
	return ""
}

// aliasForTable returns the SQL alias for a logical table name in the current scope.
func (s *replaceScope) aliasForTable(table string) string {
	if s == nil {
		return ""
	}
	var found string
	for cur := s; cur != nil; cur = cur.parent {
		for qualifier, tn := range cur.bindings {
			if !equalFold(tn.Table, table) {
				continue
			}
			if equalFold(qualifier, table) {
				continue
			}
			if found != "" && !equalFold(found, qualifier) {
				return ""
			}
			found = qualifier
		}
	}
	return found
}

// unqualifiedColumnMatchesTable reports whether a bare column reference can be
// attributed to from.Table (exactly one table in scope and it matches from).
func (s *replaceScope) unqualifiedColumnMatchesTable(from Name) bool {
	if s == nil || from.Table == "" {
		return from.Table == ""
	}
	var unique []extract.Name
	seen := map[string]struct{}{}
	for _, tn := range s.bindings {
		key := tn.Schema + "\x00" + tn.Table
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, tn)
	}
	if len(unique) != 1 {
		return false
	}
	tn := unique[0]
	if !equalFold(tn.Table, from.Table) {
		return false
	}
	if from.Schema != "" && !equalFold(tn.Schema, from.Schema) {
		return false
	}
	return true
}
