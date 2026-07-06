package sqltransform

import "github.com/alranel/go-sql-transform/internal/extract"

// UnqualifiedColumnReferences returns column names used without object.field qualification.
func (q *Query) UnqualifiedColumnReferences() []string {
	return extract.UnqualifiedColumnRefs(q.tree)
}
