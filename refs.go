package sqltransform

import (
	"fmt"
	"strings"

	"github.com/alranel/go-sql-transform/internal/extract"
)

// Name identifies a schema-qualified table and/or column.
type Name struct {
	Schema string // optional
	Table  string // optional; empty for unqualified column
	Column string // optional; empty for table-only reference; "*" for SELECT *
}

// Reference is a resolved table or column access.
type Reference struct {
	Name Name
}

func (n Name) key() string {
	return n.Schema + "\x00" + n.Table + "\x00" + n.Column
}

func (n Name) String() string {
	if n.Column == "" {
		if n.Schema != "" {
			return n.Schema + "." + n.Table
		}
		return n.Table
	}
	if n.Table == "" {
		return n.Column
	}
	parts := make([]string, 0, 3)
	if n.Schema != "" {
		parts = append(parts, n.Schema)
	}
	parts = append(parts, n.Table, n.Column)
	return strings.Join(parts, ".")
}

func dedupRefs(refs []Reference) []Reference {
	seen := make(map[string]struct{}, len(refs))
	out := make([]Reference, 0, len(refs))
	for _, r := range refs {
		k := r.Name.key()
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, r)
	}
	return out
}

func toRefs(names []extract.Name) []Reference {
	out := make([]Reference, len(names))
	for i, n := range names {
		out[i] = Reference{Name: Name(n)}
	}
	return out
}

// ReadReferences returns deduplicated physical table/column names read by the query.
// Aliases and CTE names are resolved away.
func (q *Query) ReadReferences() []Reference {
	return dedupRefs(toRefs(extract.ReadRefs(q.tree)))
}

// WriteReferences returns deduplicated physical table/column names written by the query.
func (q *Query) WriteReferences() []Reference {
	return dedupRefs(toRefs(extract.WriteRefs(q.tree)))
}

// Validate checks that a Name used for replacement has the required fields.
func (n Name) Validate(forReplace bool) error {
	if !forReplace {
		return nil
	}
	if n.Column == "" && n.Table == "" {
		return fmt.Errorf("sqltransform: table name required for table replacement")
	}
	if n.Column != "" && n.Column == "*" {
		return fmt.Errorf("sqltransform: cannot replace wildcard column")
	}
	return nil
}
